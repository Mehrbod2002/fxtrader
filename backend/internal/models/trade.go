package models

import (
	"time"

	"go.mongodb.org/mongo-driver/bson/primitive"
)

type TradeHistory struct {
	ID             primitive.ObjectID `bson:"_id"`
	UserID         primitive.ObjectID `bson:"user_id"`
	Symbol         string             `bson:"symbol"`
	TradeType      TradeType          `bson:"trade_type"`
	OrderType      string             `bson:"order_type"`
	Leverage       int                `bson:"leverage"`
	Volume         float64            `bson:"volume"`
	EntryPrice     float64            `bson:"entry_price"`
	ClosePrice     float64            `bson:"close_price,omitempty"`
	StopLoss       float64            `bson:"stop_loss"`
	TakeProfit     float64            `bson:"take_profit"`
	OpenTime       time.Time          `bson:"open_time"`
	CloseTime      *time.Time         `bson:"close_time,omitempty"`
	CloseReason    string             `bson:"close_reason,omitempty"`
	Status         string             `bson:"status"`
	MatchedTradeID string             `bson:"matched_trade_id,omitempty"`
	Expiration     *time.Time         `bson:"expiration,omitempty"`
	AccountType    string             `bson:"account_type"`
}

type TradeType string

const (
	TradeTypeBuy  TradeType = "BUY"
	TradeTypeSell TradeType = "SELL"
)

type TradeStatus string

const (
	TradeStatusPending TradeStatus = "PENDING"
	TradeStatusOpen    TradeStatus = "OPEN"
	TradeStatusClosed  TradeStatus = "CLOSED"
)
