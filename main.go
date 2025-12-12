package main

import (
	"net/http"
	"time"

	"DigitalMarket/history"
	"DigitalMarket/realtime"

	"github.com/gin-gonic/gin"
)

func main() {
	r := gin.Default()

	priceMgr := realtime.NewManager()

	// 实时价格
	r.GET("/price", func(c *gin.Context) {
		symbol := c.DefaultQuery("symbol", "BTCUSDT")
		price := priceMgr.GetPrice(symbol)

		if price == "" {
			c.JSON(http.StatusServiceUnavailable, gin.H{
				"error": "price not ready",
			})
			return
		}

		c.JSON(http.StatusOK, gin.H{
			"symbol": symbol,
			"price":  price,
		})
	})

	// 历史 K 线
	r.GET("/klines", func(c *gin.Context) {
		symbol := c.DefaultQuery("symbol", "BTCUSDT")
		interval := c.DefaultQuery("interval", "1h")

		end := time.Now().UTC()
		start := end.Add(-30 * 24 * time.Hour)

		if s := c.Query("start"); s != "" {
			start, _ = time.Parse("2006-01-02", s)
		}
		if e := c.Query("end"); e != "" {
			end, _ = time.Parse("2006-01-02", e)
		}

		klines, err := history.FetchKlines(symbol, interval, start, end)
		if err != nil {
			c.JSON(500, gin.H{"error": err.Error()})
			return
		}

		c.JSON(200, gin.H{
			"symbol":   symbol,
			"interval": interval,
			"count":    len(klines),
			"data":     klines,
		})
	})

	r.Run(":8080")
}
