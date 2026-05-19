package models

type ReceiptItem struct {
	Name       string  `json:"name"`
	Quantity   float64 `json:"quantity"`
	UnitPrice  float64 `json:"unit_price"`
	TotalPrice float64 `json:"total_price"`
}

type Discount struct {
	Description string  `json:"description"`
	Amount      float64 `json:"amount"`
}

type Totals struct {
	TotalEUR     float64 `json:"total_eur"`
	TotalBGN     float64 `json:"total_bgn"`
	ExchangeRate float64 `json:"exchange_rate"`
}

type ReceiptAnalysis struct {
	Store     string        `json:"store"`
	Address   string        `json:"address"`
	VATNumber string        `json:"vat_number"`
	Items     []ReceiptItem `json:"items"`
	Discounts []Discount    `json:"discounts"`
	Totals    Totals        `json:"totals"`
}
