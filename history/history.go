package history

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strconv"
	"strings"
	"sync"
	"time"
)

type Kline struct {
	OpenTime int64  `json:"openTime"`
	Open     string `json:"open"`
	High     string `json:"high"`
	Low      string `json:"low"`
	Close    string `json:"close"`
	Volume   string `json:"volume"`
}

const baseURL = "https://api.binance.com/api/v3/klines"

// --------- 缓存（避免重复打 Binance）---------
type cacheKey struct {
	Symbol   string
	Interval string
	StartMs  int64
	EndMs    int64
}
type cacheVal struct {
	Data   []Kline
	Expire time.Time
}

var (
	cacheMu  sync.RWMutex
	cacheMap = make(map[cacheKey]cacheVal)
	cacheTTL = 30 * time.Second // ✅ 可调：比如 10s/60s
)

func FetchKlines(symbol, interval string, start, end time.Time) ([]Kline, error) {
	symbol = strings.ToUpper(strings.TrimSpace(symbol))
	interval = strings.TrimSpace(interval)

	startMs := start.UnixMilli()
	endMs := end.UnixMilli()
	if startMs > endMs {
		return nil, fmt.Errorf("start must be <= end")
	}

	// 命中缓存
	ck := cacheKey{Symbol: symbol, Interval: interval, StartMs: startMs, EndMs: endMs}
	cacheMu.RLock()
	if v, ok := cacheMap[ck]; ok && time.Now().Before(v.Expire) {
		cacheMu.RUnlock()
		return v.Data, nil
	}
	cacheMu.RUnlock()

	client := &http.Client{Timeout: 12 * time.Second}

	var result []Kline
	curStart := startMs

	for {
		params := url.Values{}
		params.Set("symbol", symbol)
		params.Set("interval", interval)
		params.Set("limit", "1000")
		params.Set("startTime", fmt.Sprintf("%d", curStart))
		params.Set("endTime", fmt.Sprintf("%d", endMs))

		reqURL := baseURL + "?" + params.Encode()
		resp, err := client.Get(reqURL)
		if err != nil {
			return nil, err
		}

		body, _ := io.ReadAll(resp.Body)
		resp.Body.Close() // ✅ 不要 defer 在 loop 里

		if resp.StatusCode != 200 {
			return nil, fmt.Errorf("binance error: %s", string(body))
		}

		// Binance 返回：二维数组
		var raw [][]interface{}
		if err := json.Unmarshal(body, &raw); err != nil {
			return nil, err
		}
		if len(raw) == 0 {
			break
		}

		for _, r := range raw {
			// [0]=openTime(ms) [1]=open [2]=high [3]=low [4]=close [5]=volume [6]=closeTime(ms)
			openTime, ok := asInt64(r, 0)
			if !ok {
				continue
			}
			open, _ := asString(r, 1)
			high, _ := asString(r, 2)
			low, _ := asString(r, 3)
			closep, _ := asString(r, 4)
			vol, _ := asString(r, 5)

			result = append(result, Kline{
				OpenTime: openTime,
				Open:     open,
				High:     high,
				Low:      low,
				Close:    closep,
				Volume:   vol,
			})
		}

		lastClose, ok := asInt64(raw[len(raw)-1], 6)
		if !ok || lastClose >= endMs {
			break
		}
		curStart = lastClose + 1
	}

	// 写入缓存
	cacheMu.Lock()
	cacheMap[ck] = cacheVal{Data: result, Expire: time.Now().Add(cacheTTL)}
	// 顺手清理少量过期（简单做法）
	for k, v := range cacheMap {
		if time.Now().After(v.Expire) {
			delete(cacheMap, k)
		}
	}
	cacheMu.Unlock()

	return result, nil
}

// --------- 时间解析：支持 YYYY-MM-DD / RFC3339 / "2006-01-02 15:04:05" ---------
func ParseTimeFlexible(s string) (time.Time, bool) {
	s = strings.TrimSpace(s)
	if s == "" {
		return time.Time{}, false
	}
	if t, err := time.Parse("2006-01-02", s); err == nil {
		return t, true
	}
	if t, err := time.Parse(time.RFC3339, s); err == nil {
		return t, true
	}
	if t, err := time.Parse("2006-01-02 15:04:05", s); err == nil {
		return time.Date(t.Year(), t.Month(), t.Day(), t.Hour(), t.Minute(), t.Second(), 0, time.UTC), true
	}
	return time.Time{}, false
}

// --------- 辅助：更稳的类型转换（避免 panic）---------
func asInt64(arr []interface{}, idx int) (int64, bool) {
	if idx < 0 || idx >= len(arr) {
		return 0, false
	}
	switch v := arr[idx].(type) {
	case float64:
		return int64(v), true
	case int64:
		return v, true
	case json.Number:
		x, err := v.Int64()
		return x, err == nil
	case string:
		// 有时会是字符串数字
		x, err := strconv.ParseInt(v, 10, 64)
		return x, err == nil
	default:
		return 0, false
	}
}

func asString(arr []interface{}, idx int) (string, bool) {
	if idx < 0 || idx >= len(arr) {
		return "", false
	}
	switch v := arr[idx].(type) {
	case string:
		return v, true
	case float64:
		// 转成不丢精度的字符串显示（够用）
		return strconv.FormatFloat(v, 'f', -1, 64), true
	case json.Number:
		return v.String(), true
	default:
		return fmt.Sprintf("%v", v), true
	}
}
