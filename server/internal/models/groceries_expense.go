package models

import (
	"time"

	"github.com/google/uuid"
)
type GroceriesExpense struct {
	ID              int       `json:"id"`
	UserID          uuid.UUID `json:"user_id"`
	CategoryID      int       `json:"category_id"`
	CategoryName    string    `json:"category_name"`
	Product         string    `json:"product"`
	Quantity        float64   `json:"quantity"`
	UnitPrice       float64   `json:"unit_price"`
	TotalPrice      float64   `json:"total_price"`
	SupermarketName string    `json:"supermarket_name"`
	PurchaseDate    time.Time `json:"purchase_date"`
	CreatedAt       time.Time `json:"created_at"`
}

