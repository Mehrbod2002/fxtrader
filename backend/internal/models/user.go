package models

import (
	"go.mongodb.org/mongo-driver/bson/primitive"
)

type Account struct {
	ID               primitive.ObjectID `bson:"_id" json:"id"`
	UserID           primitive.ObjectID `bson:"user_id" json:"user_id"`
	AccountName      string             `bson:"account_name" json:"account_name"`
	AccountType      string             `bson:"account_type" json:"account_type"`
	WalletID         string             `bson:"wallet_id" json:"wallet_id"`
	Balance          float64            `bson:"balance" json:"balance"`
	RegistrationDate string             `bson:"registration_date" json:"registration_date"`
	IsActive         bool               `bson:"is_active" json:"is_active"`
}

type User struct {
	ID                       primitive.ObjectID `bson:"_id" json:"id"`
	FullName                 string             `bson:"full_name" json:"full_name"`
	PhoneNumber              string             `bson:"phone_number" json:"phone_number"`
	TelegramID               string             `bson:"telegram_id" json:"telegram_id"`
	Username                 string             `bson:"username" json:"username"`
	CardNumber               string             `bson:"card_number" json:"card_number"`
	Citizenship              string             `bson:"citizenship" json:"citizenship"`
	NationalID               string             `bson:"national_id" json:"national_id"`
	Residence                string             `bson:"residence" json:"residence"`
	BirthDay                 string             `bson:"birthday" json:"birthday"`
	RegistrationDate         string             `bson:"registration_date" json:"registration_date"`
	IsActive                 bool               `bson:"is_active" json:"is_active"`
	IsCopyTradeLeader        bool               `bson:"is_copy_trade_leader" json:"is_copy_trade_leader"`
	AccountType              string             `bson:"account_type" json:"account_type"`
	IsCopyPendingTradeLeader bool               `bson:"is_copy_pending_trade_leader" json:"is_copy_pending_trade_leader"`
	Balance                  float64            `bson:"balance" json:"balance"` // Main account balance
	Bonus                    float64            `bson:"bonus" json:"bonus"`
	Leverage                 int                `bson:"leverage" json:"leverage"`
	TradeType                string             `bson:"trade_type" json:"trade_type"`
	WalletAddress            string             `bson:"wallet_address" json:"wallet_address"`
	ReferralCode             string             `bson:"referral_code" json:"referral_code"`
	ReferredBy               primitive.ObjectID `bson:"referred_by" json:"referred_by"`
	AccountTypes             []string           `bson:"account_types" json:"account_types"`
}
