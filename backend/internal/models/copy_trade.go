package models

import (
	"time"

	"go.mongodb.org/mongo-driver/bson/primitive"
)

type ActivceStatus string

const (
	Active   ActivceStatus = "active"
	Inactive ActivceStatus = "inactive"
)

type CopyTradeSubscription struct {
	ID                 primitive.ObjectID `json:"_id,omitempty" bson:"_id,omitempty"`
	FollowerID         string             `json:"follower_id" bson:"follower_id"`
	LeaderID           string             `json:"leader_id" bson:"leader_id"`
	FollowerIDTelegram string             `json:"follower_id_telegram" bson:"follower_id_telegram"`
	LeaderIDTelegram   string             `json:"leader_id_telegram" bson:"leader_id_telegram"`
	AllocatedAmount    float64            `json:"allocated_amount" bson:"allocated_amount"`
	Status             ActivceStatus      `json:"status" bson:"status"`
	CreatedAt          time.Time          `json:"created_at" bson:"created_at"`
}

type CopyTrade struct {
	ID                      primitive.ObjectID `json:"_id,omitempty" bson:"_id,omitempty"`
	SubscriptionID          primitive.ObjectID `json:"subscription_id" bson:"subscription_id"`
	LeaderTradeID           primitive.ObjectID `json:"leader_trade_id" bson:"leader_trade_id"`
	FollowerTradeID         primitive.ObjectID `json:"follower_trade_id" bson:"follower_trade_id"`
	LeaderTradeIDTelegram   primitive.ObjectID `json:"leader_trade_id_telegram" bson:"leader_trade_id_telegram"`
	FollowerTradeIDTelegran primitive.ObjectID `json:"follower_trade_id_telegram" bson:"follower_trade_id_telegram"`
	CreatedAt               time.Time          `json:"created_at" bson:"created_at"`
}
