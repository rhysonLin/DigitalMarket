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
	return fmt.Errorf("invalid number")
}

type tickerMsg struct {
	Symbol string        `json:"s"`
	Price  StringOrFloat `json:"c"`
}

type priceStore struct {
	mu    sync.RWMutex
	price string
}

type Manager struct {
	mu     sync.Mutex
	stores map[string]*priceStore
}

func NewManager() *Manager {
	return &Manager{
		stores: make(map[string]*priceStore),
	}
}

func (m *Manager) GetPrice(symbol string) string {
	symbol = strings.ToUpper(symbol)

	m.mu.Lock()
	ps, ok := m.stores[symbol]
	if !ok {
		ps = &priceStore{}
		m.stores[symbol] = ps
		go startWs(symbol, ps)
	}
	m.mu.Unlock()

	ps.mu.RLock()
	defer ps.mu.RUnlock()
	return ps.price
}

func startWs(symbol string, ps *priceStore) {
	stream := strings.ToLower(symbol) + "@ticker"
	u := url.URL{
		Scheme: "wss",
		Host:   "stream.binance.com:9443",
		Path:   "/ws/" + stream,
	}

	for {
		log.Println("WS connect:", symbol)
		conn, _, err := websocket.DefaultDialer.Dial(u.String(), nil)
		if err != nil {
			time.Sleep(3 * time.Second)
			continue
		}

		for {
			_, msg, err := conn.ReadMessage()
			if err != nil {
				conn.Close()
				break
			}

			var t tickerMsg
			if err := json.Unmarshal(msg, &t); err != nil {
				continue
			}

			ps.mu.Lock()
			ps.price = string(t.Price)
			ps.mu.Unlock()
		}
	}
}
