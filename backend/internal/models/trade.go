package models

import (
	"time"

	"go.mongodb.org/mongo-driver/bson/primitive"
)

type TradeHistory struct {
	ID             primitive.ObjectID `bson:"_id" json:"_id"`
	UserID         primitive.ObjectID `bson:"user_id" json:"user_id"`
	Symbol         string             `bson:"symbol" json:"symbol"`
	AccountID      primitive.ObjectID `bson:"account_id" json:"account_id"`
	TradeType      TradeType          `bson:"trade_type" json:"trade_type"`
	OrderType      string             `bson:"order_type" json:"order_type"`
	Leverage       int                `bson:"leverage" json:"leverage"`
	Volume         float64            `bson:"volume" json:"volume"`
	EntryPrice     float64            `bson:"entry_price" json:"entry_price"`
	ClosePrice     float64            `bson:"close_price,omitempty" json:"close_price,omitempty"`
	StopLoss       float64            `bson:"stop_loss" json:"stop_loss"`
	TakeProfit     float64            `bson:"take_profit" json:"take_profit"`
	Profit         float64            `bson:"profit" json:"profit"`
	OpenTime       time.Time          `bson:"open_time" json:"open_time"`
	CloseTime      *time.Time         `bson:"close_time,omitempty" json:"close_time,omitempty"`
	CloseReason    string             `bson:"close_reason,omitempty" json:"close_reason,omitempty"`
	Status         string             `bson:"status" json:"Status"`
	MatchedTradeID string             `bson:"matched_trade_id,omitempty" json:"matched_trade_id,omitempty"`
	Expiration     *time.Time         `bson:"expiration,omitempty" json:"expiration,omitempty"`
	AccountType    string             `bson:"account_type" json:"account_type"`
	ExecutionType  ExecutionType      `bson:"execution_type" json:"execution_type"`
}

type ExecutionType string

const (
	ExecutionTypePlatform   ExecutionType = "platform"
	ExecutionTypeUserToUser ExecutionType = "user-to-user"
)

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
