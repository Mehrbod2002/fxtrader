package models

import (
	"time"

	"go.mongodb.org/mongo-driver/bson/primitive"
)

type TransactionStatus string

const (
	TransactionStatusPending  TransactionStatus = "PENDING"
	TransactionStatusApproved TransactionStatus = "APPROVED"
	TransactionStatusRejected TransactionStatus = "REJECTED"
)

type TransactionType string

const (
	TransactionTypeDeposit    TransactionType = "DEPOSIT"
	TransactionTypeWithdrawal TransactionType = "WITHDRAWAL"
)

type PaymentMethod string

const (
	PaymentMethodCardToCard     PaymentMethod = "CARD_TO_CARD"
	PaymentMethodDepositReceipt PaymentMethod = "DEPOSIT_RECEIPT"
)

type Transaction struct {
	ID              primitive.ObjectID `bson:"_id" json:"_id"`
	UserID          string             `bson:"user_id" json:"user_id"`
	TelegramID      string             `bson:"telegram_id" json:"telegram_id"`
	TransactionType TransactionType    `bson:"transaction_type" json:"transaction_type"`
	PaymentMethod   PaymentMethod      `bson:"payment_method" json:"payment_method"`
	Amount          float64            `bson:"amount" json:"amount"`
	ReceiptImage    string             `bson:"receipt_image,omitempty" json:"receipt_image"`
	Status          TransactionStatus  `bson:"status" json:"status"`
	RequestTime     time.Time          `bson:"request_time" json:"request_time"`
	ResponseTime    *time.Time         `bson:"response_time,omitempty" json:"response_time"`
	Reason          string             `bson:"reason,omitempty" json:"reason"`
	AdminComment    string             `bson:"admin_comment,omitempty" json:"admin_comment"`
}
