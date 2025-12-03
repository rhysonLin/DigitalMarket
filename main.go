package main

import (
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/gorilla/websocket"
)

const (
	binanceWsBaseURL = "wss://stream.binance.com:9443/ws"
	symbol           = "BTCUSDT" // 你要订阅的交易对
)

// ================== 自定义类型：既能收字符串，又能收数字 ==================

type StringOrFloat string

func (sf *StringOrFloat) UnmarshalJSON(b []byte) error {
	// 尝试按 string 解析
	var s string
	if err := json.Unmarshal(b, &s); err == nil {
		*sf = StringOrFloat(s)
		return nil
	}

	// 尝试按 float64 解析
	var f float64
	if err := json.Unmarshal(b, &f); err == nil {
		*sf = StringOrFloat(strconv.FormatFloat(f, 'f', -1, 64))
		return nil
	}

	return fmt.Errorf("StringOrFloat: unsupported JSON value: %s", string(b))
}

// ================== 价格缓存 ==================

type PriceStore struct {
	mu     sync.RWMutex
	price  string
	symbol string
}

func (ps *PriceStore) Set(price string) {
	ps.mu.Lock()
	defer ps.mu.Unlock()
	ps.price = price
}

func (ps *PriceStore) Get() string {
	ps.mu.RLock()
	defer ps.mu.RUnlock()
	return ps.price
}

// ================== WS 消息结构 ==================
// 文档：c = last price, s = symbol

type BinanceTickerMsg struct {
	Symbol string        `json:"s"`
	Price  StringOrFloat `json:"c"`
}

// ================== WS 订阅逻辑 ==================

func startBinanceWsTicker(ps *PriceStore, symbol string) {
	streamName := strings.ToLower(symbol) + "@ticker"
	url := binanceWsBaseURL + "/" + streamName

	for {
		log.Printf("尝试连接 Binance WS: %s", url)
		conn, _, err := websocket.DefaultDialer.Dial(url, nil)
		if err != nil {
			log.Println("连接失败，稍后重试:", err)
			time.Sleep(5 * time.Second)
			continue
		}

		log.Println("Binance WS 连接成功")

		// 读循环
		for {
			_, message, err := conn.ReadMessage()
			if err != nil {
				log.Println("读取 WS 消息失败，将重连:", err)
				conn.Close()
				break
			}

			// 调试建议：可以先打印原始消息看看
			// log.Println("原始 WS 消息:", string(message))

			var ticker BinanceTickerMsg
			if err := json.Unmarshal(message, &ticker); err != nil {
				log.Println("解析 WS 消息失败:", err)
				continue
			}

			// 更新价格
			ps.Set(string(ticker.Price))
			// 可以观察：
			// log.Printf("收到 %s 最新价格: %s\n", ticker.Symbol, ticker.Price)
		}

		time.Sleep(3 * time.Second)
	}
}

// ================== Gin HTTP 服务 ==================

func main() {
	r := gin.Default()

	priceStore := &PriceStore{
		symbol: symbol,
	}

	go startBinanceWsTicker(priceStore, symbol)

	r.GET("/health", func(c *gin.Context) {
		c.JSON(http.StatusOK, gin.H{"status": "ok"})
	})

	// GET /price 返回最新价格
	r.GET("/price", func(c *gin.Context) {
		price := priceStore.Get()
		if price == "" {
			c.JSON(http.StatusServiceUnavailable, gin.H{
				"error": "价格暂时不可用，WS 还没收到数据",
			})
			return
		}
		c.JSON(http.StatusOK, gin.H{
			"symbol": symbol,
			"price":  price,
		})
	})

	if err := r.Run(":8080"); err != nil {
		log.Fatal("Gin 启动失败:", err)
	}
}
