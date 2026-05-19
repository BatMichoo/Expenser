package models

type ReceiptAnalysis struct {
	Product         string  `json:"product"`
	Quantity        float64 `json:"quantity"`
	Price           float64 `json:"price"`
	SupermarketName string  `json:"supermarket_name"`
}
