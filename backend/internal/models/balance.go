package models

type BalanceData struct {
	UserID      string  `json:"user_id"`
	AccountType string  `json:"account_type"`
	Balance     float64 `json:"balance"`
	Timestamp   int64   `json:"timestamp"`
}
