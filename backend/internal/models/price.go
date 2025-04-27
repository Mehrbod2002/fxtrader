package models

type PriceData struct {
	Symbol    string  `json:"symbol"`
	Ask       float64 `json:"ask"`
	Bid       float64 `json:"bid"`
	Timestamp int64   `json:"timestamp"`
}
