package models

import "go.mongodb.org/mongo-driver/bson/primitive"

type UserAccount struct {
	ID               primitive.ObjectID `json:"_id,omitempty" bson:"_id,omitempty"`
	Username         string             `json:"username" bson:"username"`
	FullName         string             `json:"full_name" bson:"full_name"`
	PhoneNumber      string             `json:"phone_number" bson:"phone_number"`
	CardNumber       string             `json:"card_number" bson:"card_number"`
	NationalID       string             `json:"national_id" bson:"national_id"`
	Citizenship      string             `json:"citizenship" bson:"citizenship"`
	AccountType      string             `json:"account_type" bson:"account_type"`
	AccountName      string             `json:"account_name" bson:"account_name"`
	Balance          float64            `json:"balance" bson:"balance"`
	Bonus            float64            `json:"bonus" bson:"bonus"`
	Leverage         int                `json:"leverage" bson:"leverage"`
	TradeType        string             `json:"trade_type" bson:"trade_type"`
	RegistrationDate string             `json:"registration_date" bson:"registration_date"`
	WalletAddress    string             `json:"wallet_address" bson:"wallet_address"`
	TelegramID       string             `json:"telegram_id" bson:"telegram_id"`
	Password         string             `json:"password" bson:"password"`
}
