package models

import (
	"time"

	"go.mongodb.org/mongo-driver/bson/primitive"
)

type Symbol struct {
	ID             primitive.ObjectID `json:"_id,omitempty" bson:"_id,omitempty"`
	SymbolName     string             `json:"symbol_name" bson:"symbol_name"`
	DisplayName    string             `json:"display_name" bson:"display_name"`
	Category       string             `json:"category" bson:"category"`
	DeniedAccounts []string           `json:"denied_accounts" bson:"denied_accounts"`
	Leverage       int                `json:"leverage" bson:"leverage"`
	MinLot         float64            `json:"min_lot" bson:"min_lot"`
	MaxLot         float64            `json:"max_lot" bson:"max_lot"`
	Spread         float64            `json:"spread" bson:"spread"`
	Commission     float64            `json:"commission" bson:"commission"`
	TradingHours   TradingHours       `json:"trading_hours" bson:"trading_hours"`
	IsTradingOpen  bool               `json:"is_trading_open" bson:"is_trading_open"`
	CreatedAt      time.Time          `json:"created_at" bson:"created_at"`
	UpdatedAt      time.Time          `json:"updated_at" bson:"updated_at"`
}

type TradingHours struct {
	Unlimited bool   `json:"unlimited" bson:"unlimited"`
	OpenTime  string `json:"open_time,omitempty" bson:"open_time,omitempty"`
	CloseTime string `json:"close_time,omitempty" bson:"close_time,omitempty"`
}
