package models

import (
	"time"

	"go.mongodb.org/mongo-driver/bson/primitive"
)

type TradeType string

const (
	TradeTypeBuy  TradeType = "BUY"
	TradeTypeSell TradeType = "SELL"
)

type TradeStatus string

const (
	TradeStatusOpen    TradeStatus = "OPEN"
	TradeStatusClosed  TradeStatus = "CLOSED"
	TradeStatusPending TradeStatus = "PENDING"
)

type TradeHistory struct {
	ID         primitive.ObjectID `json:"_id,omitempty" bson:"_id,omitempty"`
	UserID     primitive.ObjectID `json:"user_id" bson:"user_id"`
	SymbolName string             `json:"symbol_name" bson:"symbol_name"`
	TradeType  TradeType          `json:"trade_type" bson:"trade_type"`
	Leverage   int                `json:"leverage" bson:"leverage"`
	Volume     float64            `json:"volume" bson:"volume"`
	EntryPrice float64            `json:"entry_price" bson:"entry_price"`
	ClosePrice float64            `json:"close_price,omitempty" bson:"close_price,omitempty"`
	Status     TradeStatus        `json:"status" bson:"status"`
	ProfitLoss float64            `json:"profit_loss,omitempty" bson:"profit_loss,omitempty"`
	OpenTime   time.Time          `json:"open_time" bson:"open_time"`
	CloseTime  *time.Time         `json:"close_time,omitempty" bson:"close_time,omitempty"`
}
