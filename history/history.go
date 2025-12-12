package history

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
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

func FetchKlines(
	symbol string,
	interval string,
	start time.Time,
	end time.Time,
) ([]Kline, error) {

	startMs := start.UnixMilli()
	endMs := end.UnixMilli()

	var result []Kline

	for {
		params := url.Values{}
		params.Set("symbol", symbol)
		params.Set("interval", interval)
		params.Set("limit", "1000")
		params.Set("startTime", fmt.Sprintf("%d", startMs))
		params.Set("endTime", fmt.Sprintf("%d", endMs))

		resp, err := http.Get(baseURL + "?" + params.Encode())
		if err != nil {
			return nil, err
		}
		defer resp.Body.Close()

		if resp.StatusCode != 200 {
			b, _ := io.ReadAll(resp.Body)
			return nil, fmt.Errorf("binance error: %s", b)
		}

		var raw [][]interface{}
		if err := json.NewDecoder(resp.Body).Decode(&raw); err != nil {
			return nil, err
		}

		if len(raw) == 0 {
			break
		}

		for _, r := range raw {
			result = append(result, Kline{
				OpenTime: int64(r[0].(float64)),
				Open:     r[1].(string),
				High:     r[2].(string),
				Low:      r[3].(string),
				Close:    r[4].(string),
				Volume:   r[5].(string),
			})
		}

		lastClose := int64(raw[len(raw)-1][6].(float64))
		if lastClose >= endMs {
			break
		}
		startMs = lastClose + 1
	}

	return result, nil
}
