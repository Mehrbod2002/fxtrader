package models

import (
	"time"

	"go.mongodb.org/mongo-driver/bson/primitive"
)

type AlertType string

const (
	AlertTypePrice AlertType = "PRICE"
	AlertTypeTime  AlertType = "TIME"
)

type AlertStatus string

const (
	AlertStatusPending   AlertStatus = "PENDING"
	AlertStatusTriggered AlertStatus = "TRIGGERED"
	AlertStatusExpired   AlertStatus = "EXPIRED"
)

type Alert struct {
	ID                 primitive.ObjectID `json:"_id,omitempty" bson:"_id,omitempty"`
	UserID             string             `json:"user_id" bson:"user_id"`
	SymbolName         string             `json:"symbol_name" bson:"symbol_name"`
	AlertType          AlertType          `json:"alert_type" bson:"alert_type"`
	Condition          AlertCondition     `json:"condition" bson:"condition"`
	Status             AlertStatus        `json:"status" bson:"status"`
	CreatedAt          time.Time          `json:"created_at" bson:"created_at"`
	TriggeredAt        *time.Time         `json:"triggered_at,omitempty" bson:"triggered_at,omitempty"`
	NotificationMethod string             `json:"notification_method" bson:"notification_method"`
}

type AlertCondition struct {
	PriceTarget *float64   `json:"price_target,omitempty" bson:"price_target,omitempty"`
	Comparison  string     `json:"comparison,omitempty" bson:"comparison,omitempty"`
	TriggerTime *time.Time `json:"trigger_time,omitempty" bson:"trigger_time,omitempty"`
}
