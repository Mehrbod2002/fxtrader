package models

import (
	"time"

	"go.mongodb.org/mongo-driver/bson/primitive"
)

type LeaderRequest struct {
	ID          primitive.ObjectID `bson:"_id" json:"id"`
	UserID      string             `bson:"user_id" json:"user_id"`
	Reason      string             `bson:"reason" json:"reason"`
	Status      string             `bson:"status" json:"status"`
	AdminReason string             `bson:"admin_reason" json:"admin_reason"`
	CreatedAt   time.Time          `bson:"created_at" json:"created_at"`
	UpdatedAt   time.Time          `bson:"updated_at" json:"updated_at"`
}
