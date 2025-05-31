package models

import (
	"sync"

	"github.com/gorilla/websocket"
)

type Client struct {
	ID          string
	Conn        *websocket.Conn
	Send        chan *PriceData
	SendTrade   chan *TradeHistory
	SendBalance chan *BalanceData
	Symbols     map[string]bool
	mu          sync.RWMutex
}

func NewClient(id string, conn *websocket.Conn) *Client {
	return &Client{
		ID:          id,
		Conn:        conn,
		Send:        make(chan *PriceData, 256),
		SendTrade:   make(chan *TradeHistory, 256),
		SendBalance: make(chan *BalanceData, 100),
		Symbols:     make(map[string]bool),
	}
}

func (c *Client) Subscribe(symbol string) {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.Symbols[symbol] = true
}

func (c *Client) Unsubscribe(symbol string) {
	c.mu.Lock()
	defer c.mu.Unlock()
	delete(c.Symbols, symbol)
}

func (c *Client) IsSubscribed(symbol string) bool {
	c.mu.RLock()
	defer c.mu.RUnlock()
	_, ok := c.Symbols[symbol]
	return ok
}

func (c *Client) Close() {
	c.mu.Lock()
	defer c.mu.Unlock()
	close(c.Send)
	close(c.SendTrade)
	close(c.SendBalance)
	c.Conn.Close()
}

type SocketMessage struct {
	Action string `json:"action"`
	Symbol string `json:"symbol"`
}

type SubscriptionResponse struct {
	Status  string   `json:"status"`
	Message string   `json:"message"`
	Symbols []string `json:"symbols,omitempty"`
}

type ErrorResponse struct {
	Error string `json:"error"`
}
