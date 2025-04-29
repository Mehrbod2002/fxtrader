package models

import (
	"time"

	"go.mongodb.org/mongo-driver/bson/primitive"
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

type TransactionStatus string

const (
	TransactionStatusPending  TransactionStatus = "PENDING"
	TransactionStatusApproved TransactionStatus = "APPROVED"
	TransactionStatusRejected TransactionStatus = "REJECTED"
)

type Transaction struct {
	ID              primitive.ObjectID `json:"_id,omitempty" bson:"_id,omitempty"`
	UserID          string             `json:"user_id" bson:"user_id"`
	TransactionType TransactionType    `json:"transaction_type" bson:"transaction_type"`
	PaymentMethod   PaymentMethod      `json:"payment_method" bson:"payment_method"`
	Amount          float64            `json:"amount" bson:"amount"`
	Status          TransactionStatus  `json:"status" bson:"status"`
	ReceiptImage    string             `json:"receipt_image,omitempty" bson:"receipt_image,omitempty"`
	RequestTime     time.Time          `json:"request_time" bson:"request_time"`
	ResponseTime    *time.Time         `json:"response_time,omitempty" bson:"response_time,omitempty"`
	AdminNote       string             `json:"admin_note,omitempty" bson:"admin_note,omitempty"`
}
