import requests
import pandas as pd
import time
import json
import os
from datetime import datetime

BINANCE_API = "https://api.binance.com"
CG_API = "https://api.coingecko.com/api/v3"

# >5年 기준：genesis_date <= 2021-01-21
CUTOFF = datetime(2021, 1, 21).date()

MAJOR_QUOTES = {"USDT", "USDC", "FDUSD", "TUSD", "BUSD", "DAI", "EUR", "TRY", "BRL", "GBP", "JPY", "AUD"}

CACHE_FILE = "coingecko_genesis_cache.json"


def request_with_retry(url, params=None, timeout=60, max_retry=8):
    """
    自动处理 CoinGecko 429 限流：指数退避重试
    """
    wait = 2
    for i in range(max_retry):
        r = requests.get(url, params=params, timeout=timeout)
        if r.status_code == 429:
            print(f"[429] Too Many Requests -> 等待 {wait}s 后重试 ({i+1}/{max_retry})")
            time.sleep(wait)
            wait = min(wait * 2, 60)
            continue
        r.raise_for_status()
        return r
    raise RuntimeError(f"请求多次失败：{url}")


def get_exchange_info():
    r = requests.get(f"{BINANCE_API}/api/v3/exchangeInfo", timeout=30)
    r.raise_for_status()
    return r.json()


def get_24hr_tickers():
    r = requests.get(f"{BINANCE_API}/api/v3/ticker/24hr", timeout=30)
    r.raise_for_status()
    return r.json()


def build_symbol_map(exchange_info):
    rows = []
    for s in exchange_info["symbols"]:
        if s.get("status") != "TRADING":
            continue
        rows.append({
            "symbol": s["symbol"],
            "base": s["baseAsset"],
            "quote": s["quoteAsset"],
        })
    return pd.DataFrame(rows)


def load_cache():
    if os.path.exists(CACHE_FILE):
        try:
            with open(CACHE_FILE, "r", encoding="utf-8") as f:
                return json.load(f)
        except:
            return {}
    return {}


def save_cache(cache):
    with open(CACHE_FILE, "w", encoding="utf-8") as f:
        json.dump(cache, f, ensure_ascii=False, indent=2)


def coingecko_id_map():
    # /coins/list 也会限流，所以加 retry
    r = request_with_retry(f"{CG_API}/coins/list", timeout=60)
    data = r.json()
    df = pd.DataFrame(data)
    df["symbol"] = df["symbol"].str.upper()
    return df


def pick_best_cg_id(symbol, cg_map_df):
    cand = cg_map_df[cg_map_df["symbol"] == symbol].copy()
    if cand.empty:
        return None
    cand["id_len"] = cand["id"].str.len()
    cand = cand.sort_values(["id_len", "id"])
    return cand.iloc[0]["id"]


def get_genesis_date_cached(cg_id, cache):
    if cg_id in cache:
        return cache[cg_id]

    # 限流友好：每次查询 coin detail 之间停一下
    time.sleep(1.2)

    r = request_with_retry(
        f"{CG_API}/coins/{cg_id}",
        params={
            "localization": "false",
            "tickers": "false",
            "market_data": "false",
            "community_data": "false",
            "developer_data": "false",
            "sparkline": "false",
        },
        timeout=60
    )

    j = r.json()
    gd = j.get("genesis_date")  # "YYYY-MM-DD" or None
    cache[cg_id] = gd
    return gd


def main():
    print("1) 拉取 Binance 交易对信息...")
    ex = get_exchange_info()
    symmap = build_symbol_map(ex)

    print("2) 拉取 Binance 24h ticker...")
    tickers = pd.DataFrame(get_24hr_tickers())
    tickers = tickers[["symbol", "quoteVolume", "count"]].copy()
    tickers["quoteVolume"] = pd.to_numeric(tickers["quoteVolume"], errors="coerce").fillna(0.0)
    tickers["count"] = pd.to_numeric(tickers["count"], errors="coerce").fillna(0).astype(int)

    df = symmap.merge(tickers, on="symbol", how="inner")
    df = df[df["quote"].isin(MAJOR_QUOTES)].copy()

    print("3) 按 base 聚合流动性（24h 成交额）并取 Top200...")
    agg = df.groupby("base", as_index=False).agg(
        volume_24h_quote=("quoteVolume", "sum"),
        trades_24h=("count", "sum"),
        pairs=("symbol", "nunique")
    )
    agg = agg.sort_values("volume_24h_quote", ascending=False).head(200).reset_index(drop=True)
    agg["rank_liquidity"] = agg.index + 1

    print("4) 获取 CoinGecko symbol->id 映射（可能会限流，已自动重试）...")
    cg_map = coingecko_id_map()

    print("5) 查询 genesis_date（带缓存，避免重复请求）...")
    cache = load_cache()

    cg_ids = []
    genesis_dates = []
    over_5y = []

    for sym in agg["base"].tolist():
        cg_id = pick_best_cg_id(sym, cg_map)
        cg_ids.append(cg_id)

        if not cg_id:
            genesis_dates.append(None)
            over_5y.append(None)
            continue

        gd = get_genesis_date_cached(cg_id, cache)
        genesis_dates.append(gd)

        if gd:
            d = datetime.strptime(gd, "%Y-%m-%d").date()
            over_5y.append(d <= CUTOFF)
        else:
            over_5y.append(None)

    save_cache(cache)

    agg["coingecko_id"] = cg_ids
    agg["genesis_date"] = genesis_dates
    agg["over_5y"] = over_5y

    agg.to_csv("binance_top200_by_liquidity.csv", index=False, encoding="utf-8-sig")
    over5 = agg[agg["over_5y"] == True].copy()
    over5.to_csv("binance_top200_over5y_only.csv", index=False, encoding="utf-8-sig")

    print("\n✅ 已生成：binance_top200_by_liquidity.csv")
    print("✅ 已生成：binance_top200_over5y_only.csv")
    print(f"Top200 中判定为 >5年 的数量：{len(over5)}")
    unknown = agg["over_5y"].isna().sum()
    print(f"Top200 中 genesis_date 缺失/无法匹配 的数量：{unknown}")
    print(f"缓存文件：{CACHE_FILE}")


if __name__ == "__main__":
    main()
