package history

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"time"
)

const binanceRestBaseURL = "https://api.binance.com"

type Kline struct {
	OpenTime int64  `json:"openTime"`
	Open     string `json:"open"`
	High     string `json:"high"`
	Low      string `json:"low"`
	Close    string `json:"close"`
	Volume   string `json:"volume"`
}

func fetchKlines(symbol, interval string, limit int, startTime, endTime *int64) ([]Kline, error) {
	params := url.Values{}
	params.Set("symbol", symbol)
	params.Set("interval", interval)
	params.Set("limit", fmt.Sprintf("%d", limit))
	if startTime != nil {
		params.Set("startTime", fmt.Sprintf("%d", *startTime))
	}
	if endTime != nil {
		params.Set("endTime", fmt.Sprintf("%d", *endTime))
	}

	finalURL := binanceRestBaseURL + "/api/v3/klines?" + params.Encode()

	client := &http.Client{Timeout: 10 * time.Second}
	resp, err := client.Get(finalURL)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		b, _ := io.ReadAll(resp.Body)
		return nil, fmt.Errorf("binance error: %s", string(b))
	}

	var raw [][]interface{}
	if err := json.NewDecoder(resp.Body).Decode(&raw); err != nil {
		return nil, err
	}

	result := make([]Kline, 0, len(raw))

	for _, item := range raw {
		result = append(result, Kline{
			OpenTime: int64(item[0].(float64)),
			Open:     item[1].(string),
			High:     item[2].(string),
			Low:      item[3].(string),
			Close:    item[4].(string),
			Volume:   item[5].(string),
		})
	}

	return result, nil
}

func GetLastMonthHourlyKlines(symbol string) ([]Kline, error) {
	now := time.Now().UTC()
	start := now.Add(-30 * 24 * time.Hour)

	startMs := start.UnixMilli()
	endMs := now.UnixMilli()

	// 30天的 1h K 线只有 720 根，不超过 1000 根限制
	return fetchKlines(symbol, "1h", 1000, &startMs, &endMs)
}
