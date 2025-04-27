package models

import (
	"time"

	"go.mongodb.org/mongo-driver/bson/primitive"
)

type Rule struct {
	ID        primitive.ObjectID `json:"_id,omitempty" bson:"_id,omitempty"`
	Content   string             `json:"content" bson:"content"`
	CreatedAt time.Time          `json:"created_at" bson:"created_at"`
}
