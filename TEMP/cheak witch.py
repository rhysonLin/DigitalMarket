import time
import requests
import pandas as pd
from datetime import datetime, timezone

CUTOFF = datetime(2023, 1, 23, tzinfo=timezone.utc)  # “存在≥3年”：以 2026-01-23 往前推3年
TOP_N = 200

def cg_get(url, params=None):
    r = requests.get(url, params=params, timeout=30)
    r.raise_for_status()
    return r.json()

# 1) 取 Top200（按市值）
markets = cg_get(
    "https://api.coingecko.com/api/v3/coins/markets",
    params={
        "vs_currency": "usd",
        "order": "market_cap_desc",
        "per_page": TOP_N,
        "page": 1,
        "sparkline": "false",
        "price_change_percentage": "24h"
    }
)

rows = []
for i, c in enumerate(markets, start=1):
    coin_id = c["id"]

    # 2) 取每个币的 genesis_date（有些币可能为空）
    detail = cg_get(
        f"https://api.coingecko.com/api/v3/coins/{coin_id}",
        params={
            "localization": "false",
            "tickers": "false",
            "market_data": "false",
            "community_data": "false",
            "developer_data": "false",
            "sparkline": "false"
        }
    )
    genesis = detail.get("genesis_date")  # 格式通常是 "YYYY-MM-DD" 或 None

    # 简单的“流通性强”近似：24h成交量不为0（你也可以改成 > 10_000_000 等阈值）
    liquid = (c.get("total_volume") or 0) > 0

    # 解析日期并判断≥3年
    is_old = False
    if genesis:
        dt = datetime.strptime(genesis, "%Y-%m-%d").replace(tzinfo=timezone.utc)
        is_old = dt <= CUTOFF

    rows.append({
        "rank_market_cap": c.get("market_cap_rank", i),
        "name": c.get("name"),
        "symbol": c.get("symbol"),
        "id": coin_id,
        "price_usd": c.get("current_price"),
        "market_cap_usd": c.get("market_cap"),
        "volume_24h_usd": c.get("total_volume"),
        "genesis_date": genesis,
        "liquid_approx": liquid,
        "older_than_3y": is_old,
    })

    time.sleep(0.8)  # 降低被限频概率

df = pd.DataFrame(rows).sort_values("rank_market_cap")

# 3) 筛选：≥3年 + 流通性强
filtered = df[(df["older_than_3y"] == True) & (df["liquid_approx"] == True)].copy()

# 输出
filtered.to_csv("top200_older_than_3y_liquid.csv", index=False, encoding="utf-8-sig")
filtered.to_excel("top200_older_than_3y_liquid.xlsx", index=False)

print("Top200原始数量：", len(df))
print("符合(≥3年 & 流通性强)数量：", len(filtered))
print("文件已生成：top200_older_than_3y_liquid.csv / .xlsx")
