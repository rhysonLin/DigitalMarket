import csv
import time
import requests
from datetime import datetime, timezone, timedelta

BASE_URL = "https://api.binance.com/api/v3/klines"

def to_ms(dt: datetime) -> int:
    return int(dt.replace(tzinfo=timezone.utc).timestamp() * 1000)

def fetch_klines(symbol: str, interval: str, start_ms: int, end_ms: int, limit: int = 1000, sleep: float = 0.2):
    """
    Paginate Binance klines from start_ms to end_ms.
    Returns list of rows (each row is the Binance kline array).
    """
    all_rows = []
    cur = start_ms

    while True:
        params = {
            "symbol": symbol,
            "interval": interval,
            "startTime": cur,
            "endTime": end_ms,
            "limit": limit,
        }
        r = requests.get(BASE_URL, params=params, timeout=30)
        r.raise_for_status()
        rows = r.json()

        if not rows:
            break

        all_rows.extend(rows)

        # next page starts after last candle close_time
        last_close_time = rows[-1][6]
        next_start = last_close_time + 1

        if next_start <= cur:
            break

        cur = next_start

        if cur >= end_ms:
            break

        # be polite to API
        time.sleep(sleep)

        # if returned less than limit, likely done
        if len(rows) < limit:
            break

    return all_rows

def save_csv(rows, out_path: str):
    header = [
        "open_time","open","high","low","close","volume",
        "close_time","quote_asset_volume","num_trades",
        "taker_buy_base","taker_buy_quote","ignore"
    ]
    with open(out_path, "w", newline="", encoding="utf-8") as f:
        w = csv.writer(f)
        w.writerow(header)
        w.writerows(rows)

def main():
    symbol = "BTCUSDT"
    interval = "1h"
    years = 2

    end_dt = datetime.now(timezone.utc)
    start_dt = end_dt - timedelta(days=365 * years)

    start_ms = to_ms(start_dt)
    end_ms = to_ms(end_dt)

    print(f"Fetching {symbol} {interval} (UTC) from {start_dt.isoformat()} to {end_dt.isoformat()}")

    rows = fetch_klines(symbol, interval, start_ms, end_ms, limit=1000, sleep=0.2)

    # de-dup by open_time
    seen = set()
    dedup = []
    for r in rows:
        if r[0] not in seen:
            seen.add(r[0])
            dedup.append(r)

    dedup.sort(key=lambda x: x[0])

    out = f"{symbol}_1h_last{years}y_binance.csv"
    save_csv(dedup, out)

    print(f"Saved: {out} | rows={len(dedup)}")
    if dedup:
        first = datetime.fromtimestamp(dedup[0][0]/1000, tz=timezone.utc)
        last = datetime.fromtimestamp(dedup[-1][0]/1000, tz=timezone.utc)
        print(f"Range in file (UTC): {first.isoformat()} -> {last.isoformat()}")

if __name__ == "__main__":
    main()
