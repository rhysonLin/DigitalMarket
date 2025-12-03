package main

import (
	"log"
	"net/http"

	"DigitalMarket/history"
	"DigitalMarket/realtime"

	"github.com/gin-gonic/gin"
)

func main() {
	r := gin.Default()

	// ========== 实时价格 ==========
	priceStore := realtime.NewPriceStore("BTCUSDT")
	priceStore.Start()

	// GET /price
	r.GET("/price", func(c *gin.Context) {
		price := priceStore.GetLatestPrice()
		if price == "" {
			c.JSON(http.StatusServiceUnavailable, gin.H{
				"error": "实时价格暂未获取到",
			})
			return
		}

		c.JSON(http.StatusOK, gin.H{
			"symbol": "BTCUSDT",
			"price":  price,
		})
	})

	// ========== 最近一个月 K 线 ==========
	// GET /klines?symbol=BTCUSDT
	r.GET("/klines", func(c *gin.Context) {
		symbol := c.Query("symbol")
		if symbol == "" {
			symbol = "BTCUSDT"
		}

		klines, err := history.GetLastMonthHourlyKlines(symbol)
		if err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
			return
		}

		c.JSON(http.StatusOK, gin.H{
			"symbol":  symbol,
			"count":   len(klines),
			"interval": "1h",
			"data":    klines,
		})
	})

	if err := r.Run(":8080"); err != nil {
		log.Fatal(err)
	}
}
