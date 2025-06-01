package models

import (
	"sync"

	"github.com/gorilla/websocket"
	"go.mongodb.org/mongo-driver/bson/primitive"
)

type Client struct {
	ID           string
	Conn         *websocket.Conn
	Send         chan *PriceData
	SendTrade    chan *TradeHistory
	SendBalance  chan *BalanceData
	SendOrders   chan OrderStreamResponse
	Symbols      map[string]bool
	SymbolsMu    sync.RWMutex
	CloseHandler func()
}

type OrderStreamResponse struct {
	Type        string             `json:"type"`
	UserID      primitive.ObjectID `json:"user_id"`
	AccountType string             `json:"account_type"`
	Trades      []TradeStream      `json:"trades"`
}

type TradeStream struct {
	ID          primitive.ObjectID `json:"id"`
	Symbol      string             `json:"symbol"`
	TradeType   string             `json:"trade_type"`
	OrderType   string             `json:"order_type"`
	Volume      float64            `json:"volume"`
	EntryPrice  float64            `json:"entry_price"`
	StopLoss    float64            `json:"stop_loss"`
	TakeProfit  float64            `json:"take_profit"`
	Profit      float64            `json:"profit"`
	OpenTime    int64              `json:"open_time"`
	Status      string             `json:"status"`
	AccountType string             `json:"account_type"`
}

func NewClient(id string, conn *websocket.Conn) *Client {
	return &Client{
		ID:          id,
		Conn:        conn,
		Send:        make(chan *PriceData, 256),
		SendTrade:   make(chan *TradeHistory, 256),
		SendBalance: make(chan *BalanceData, 256),
		SendOrders:  make(chan OrderStreamResponse, 256),
		Symbols:     make(map[string]bool),
	}
}

func (c *Client) Subscribe(symbol string) {
	c.SymbolsMu.Lock()
	c.Symbols[symbol] = true
	c.SymbolsMu.Unlock()
}

func (c *Client) Unsubscribe(symbol string) {
	c.SymbolsMu.Lock()
	delete(c.Symbols, symbol)
	c.SymbolsMu.Unlock()
}

func (c *Client) IsSubscribed(symbol string) bool {
	c.SymbolsMu.RLock()
	defer c.SymbolsMu.RUnlock()
	return c.Symbols[symbol]
}

func (c *Client) Close() {
	if c.CloseHandler != nil {
		c.CloseHandler()
	}
	c.Conn.Close()
}

type SocketMessage struct {
	Action string `json:"action"`
	Symbol string `json:"symbol"`
}

type SubscriptionResponse struct {
	Status      string   `json:"status"`
	Message     string   `json:"message"`
	UserID      string   `json:"user_id"`
	AccountType string   `json:"account_type"`
	Symbols     []string `json:"symbols,omitempty"`
}

type ErrorResponse struct {
	Error string `json:"error"`
}
