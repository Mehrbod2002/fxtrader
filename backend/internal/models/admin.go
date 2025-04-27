package models

import (
	"go.mongodb.org/mongo-driver/bson/primitive"
)

type AdminAccount struct {
	ID               primitive.ObjectID `json:"_id,omitempty" bson:"_id,omitempty"`
	Username         string             `json:"username" bson:"username"`
	Password         string             `json:"password" bson:"password"`
	AccountType      string             `json:"account_type" bson:"account_type"`
	RegistrationDate string             `json:"registration_date" bson:"registration_date"`
}
