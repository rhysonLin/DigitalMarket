# DigitalMarket API 文档

> 当前版本基于 **DigitalMarket 行情服务 v1**  
> 功能覆盖：  
> - 实时价格（WebSocket 驱动，支持自定义币种）  
> - 历史 K 线（REST，支持自定义时间段与周期）

服务默认地址：

```
http://localhost:8080
```

---

## 目录

- [1. 健康检查](#1-健康检查)
- [2. 实时价格接口](#2-实时价格接口)
- [3. 历史 K 线接口](#3-历史-k-线接口)
- [4. 通用说明](#4-通用说明)

---

## 1. 健康检查

### GET `/health`

**描述**  
用于检测服务是否正常运行。

**请求参数**  
无

**响应示例（200）**

```json
{
  "status": "ok"
}
```

---

## 2. 实时价格接口

### GET `/price`

**描述**  
获取指定交易对的最新实时价格。

- 后端会在**第一次请求某个 symbol 时自动建立 Binance WebSocket 订阅**
- 后续请求直接从内存缓存返回
- 一个 `symbol` 对应一个 WS 连接（当前实现）

---

### 请求参数（Query）

| 参数名 | 是否必填 | 示例 | 说明 |
|-----|--------|------|------|
| symbol | 否 | BTCUSDT | 交易对，默认 `BTCUSDT` |

---

### 请求示例

```bash
curl "http://localhost:8080/price?symbol=BTCUSDT"
curl "http://localhost:8080/price?symbol=ETHUSDT"
```

---

### 成功响应（200）

```json
{
  "symbol": "BTCUSDT",
  "price": "95123.17"
}
```

---

## 3. 历史 K 线接口

### GET `/klines`

**描述**  
获取指定交易对的历史 K 线数据。

- 数据来源：Binance REST API
- 自动处理分页（每次最多 1000 根 K 线）
- 支持自定义时间范围与周期

---

### 请求参数（Query）

| 参数名 | 是否必填 | 示例 | 说明 |
|-----|--------|------|------|
| symbol | 否 | BTCUSDT | 交易对，默认 `BTCUSDT` |
| interval | 否 | 1m / 5m / 15m / 1h / 4h / 1d | K 线周期，默认 `1h` |
| start | 否 | 2025-11-01 | 开始日期（UTC，格式：YYYY-MM-DD） |
| end | 否 | 2025-12-01 | 结束日期（UTC，格式：YYYY-MM-DD） |

---

### 成功响应（200）

```json
{
  "symbol": "BTCUSDT",
  "interval": "1h",
  "count": 720,
  "data": []
}
```

---

## 4. 通用说明

- 所有时间使用 UTC
- 所有价格字段使用 string 表示，避免浮点精度问题
