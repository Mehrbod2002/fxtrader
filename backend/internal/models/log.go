package models

import (
	"time"

	"go.mongodb.org/mongo-driver/bson/primitive"
)

type LogEntry struct {
	ID          primitive.ObjectID     `json:"_id,omitempty" bson:"_id,omitempty"`
	UserID      primitive.ObjectID     `json:"user_id,omitempty" bson:"user_id,omitempty"`
	Action      string                 `json:"action" bson:"action"`
	Description string                 `json:"description" bson:"description"`
	IPAddress   string                 `json:"ip_address,omitempty" bson:"ip_address,omitempty"`
	Timestamp   time.Time              `json:"timestamp" bson:"timestamp"`
	Metadata    map[string]interface{} `json:"metadata,omitempty" bson:"metadata,omitempty"`
}
