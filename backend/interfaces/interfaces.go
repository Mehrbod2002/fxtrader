package interfaces

import (
	"context"
	"time"

	"github.com/gorilla/websocket"
	"github.com/mehrbod2002/fxtrader/internal/models"
)

type TradeService interface {
	PlaceTrade(userID, accountID, symbol, accountType string, tradeType models.TradeType, orderType string, leverage int, volume, entryPrice, stopLoss, takeProfit float64, expiration *time.Time) (*models.TradeHistory, TradeResponse, error)
	CloseTrade(tradeID, userID, accountType, accountID string) (TradeResponse, error)
	StreamTrades(userID, accountType string) (chan models.OrderStreamResponse, error)
	GetTrade(id string) (*models.TradeHistory, error)
	GetTradesByUserID(userID string) ([]*models.TradeHistory, error)
	GetAllTrades() ([]*models.TradeHistory, error)
	HandleTradeResponse(response TradeResponse) error
	HandleCloseTradeResponse(response TradeResponse) error
	HandleOrderStreamResponse(response models.OrderStreamResponse) error
	HandleTradeRequest(request map[string]interface{}) error
	HandleBalanceRequest(request map[string]interface{}) error
	HandleBalanceResponse(request BalanceResponse) error
	RequestBalance(userID, accountID, accountType string) (float64, error)
	RegisterMT5Connection(conn *websocket.Conn)
	ModifyTrade(ctx context.Context, userID, tradeID, accountType, accountID string, entryPrice, volume float64) (TradeResponse, error)
	RegisterWallet(userID, accountID, walletID string) error // New method for wallet registration
}

type TradeResponse struct {
	TradeID        string  `json:"trade_id"`
	UserID         string  `json:"user_id"`
	MatchedTradeID string  `json:"matched_trade_id"`
	TradeRetcode   int     `json:"trade_retcode"`
	Timestamp      float64 `json:"timestamp"`
	MatchedVolume  float64 `json:"matched_volume"`
	AccountType    string  `json:"account_type"`
	AccountID      string  `json:"account_id"`
	Status         string  `json:"status"`
	ClosePrice     float64 `json:"close_price"`
	CloseReason    string  `json:"close_reason"`
}

type BalanceResponse struct {
	Type        string  `json:"type"`
	UserID      string  `json:"user_id"`
	AccountID   string  `json:"account_id"`
	AccountType string  `json:"account_type"`
	Balance     float64 `json:"balance"`
	Error       string  `json:"error,omitempty"`
	Timestamp   float64 `json:"timestamp"`
}
