package models

import (
	"go.mongodb.org/mongo-driver/bson/primitive"
)

type TelegramMessage struct {
	ID        primitive.ObjectID `bson:"_id,omitempty"`
	UserID    string             `bson:"user_id,omitempty"`
	ChatID    int64              `bson:"chat_id"`
	Message   string             `bson:"message"`
	Timestamp primitive.DateTime `bson:"timestamp"`
}
