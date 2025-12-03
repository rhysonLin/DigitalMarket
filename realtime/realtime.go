package realtime

import (
	"encoding/json"
	"fmt"
	"log"
	"net/url"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/gorilla/websocket"
)

const (
	binanceWsBaseURL = "wss://stream.binance.com:9443/ws"
)

// ================== 自动兼容 string / float 的价格结构 ==================

type StringOrFloat string

func (sf *StringOrFloat) UnmarshalJSON(b []byte) error {
	var s string
	if err := json.Unmarshal(b, &s); err == nil {
		*sf = StringOrFloat(s)
		return nil
	}

	var f float64
	if err := json.Unmarshal(b, &f); err == nil {
		*sf = StringOrFloat(strconv.FormatFloat(f, 'f', -1, 64))
		return nil
	}

	return fmt.Errorf("StringOrFloat: unsupported JSON value: %s", string(b))
}

// ticker 响应结构
type BinanceTickerMsg struct {
	Symbol string        `json:"s"`
	Price  StringOrFloat `json:"c"`
}

// ================== 实时价格缓存 ==================

type PriceStore struct {
	mu     sync.RWMutex
	price  string
	Symbol string
}

func NewPriceStore(symbol string) *PriceStore {
	return &PriceStore{
		Symbol: strings.ToUpper(symbol),
	}
}

func (ps *PriceStore) GetLatestPrice() string {
	ps.mu.RLock()
	defer ps.mu.RUnlock()
	return ps.price
}

func (ps *PriceStore) set(price string) {
	ps.mu.Lock()
	defer ps.mu.Unlock()
	ps.price = price
}

func (ps *PriceStore) Start() {
	go ps.runWs()
}

func (ps *PriceStore) runWs() {
	streamName := strings.ToLower(ps.Symbol) + "@ticker"

	u := url.URL{
		Scheme: "wss",
		Host:   "stream.binance.com:9443",
		Path:   "/ws/" + streamName,
	}

	for {
		log.Printf("尝试连接 Binance WebSocket: %s", u.String())
		conn, _, err := websocket.DefaultDialer.Dial(u.String(), nil)
		if err != nil {
			log.Println("连接失败，3秒后重试:", err)
			time.Sleep(3 * time.Second)
			continue
		}

		log.Println("Binance WS 连接成功")

		for {
			_, msg, err := conn.ReadMessage()
			if err != nil {
				log.Println("WS 读取失败，将重连:", err)
				conn.Close()
				break
			}

			var ticker BinanceTickerMsg
			if err := json.Unmarshal(msg, &ticker); err != nil {
				log.Println("JSON 解析失败:", err)
				continue
			}

			ps.set(string(ticker.Price))
		}
	}
}
