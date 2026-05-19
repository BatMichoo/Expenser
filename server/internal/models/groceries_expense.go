package models

import (
	"time"

	"github.com/google/uuid"
)

type GroceriesExpense struct {
	ID              int       `json:"id"`
	UserID          uuid.UUID `json:"user_id"`
	Product         string    `json:"product"`
	Quantity        float64   `json:"quantity"`
	Price           int64     `json:"price"` // Price in cents
	SupermarketName string    `json:"supermarket_name"`
	PurchaseDate    time.Time `json:"purchase_date"`
	CreatedAt       time.Time `json:"created_at"`
}
