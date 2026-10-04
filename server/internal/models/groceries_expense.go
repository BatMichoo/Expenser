package models

import (
	"time"

	"github.com/google/uuid"
)

// GroceriesChartExpense is a minimal projection used by the chart endpoints.
// Field names (not JSON-tagged) are read directly by chart-core.js.
type GroceriesChartExpense struct {
	CategoryName  string
	Amount        float64
	TotalDiscount float64
	PurchaseDate  time.Time
}

type GroceriesExpense struct {
	ID              int       `json:"id"`
	UserID          uuid.UUID `json:"user_id"`
	CategoryID      int       `json:"category_id"`
	CategoryName    string    `json:"category_name"`
	Product         string    `json:"product"`
	Quantity        float64   `json:"quantity"`
	UnitPrice       float64   `json:"unit_price"`
	DiscountPerUnit float64   `json:"discount_per_unit"`
	TotalPrice      float64   `json:"total_price"`
	TotalDiscount   float64   `json:"total_discount"`
	SupermarketName string    `json:"supermarket_name"`
	PurchaseDate    time.Time `json:"purchase_date"`
	CreatedAt       time.Time `json:"created_at"`
}

type GroceriesData struct {
	Name           string
	MonthlyExpense *MonthlyExpense     // MonthlyExpense summarizes the total spending for the current month.
	HighestExpense *HighestExpense     // HighestExpense identifies the single largest expense in the current month.
	RecentExpenses *[]GroceriesExpense // RecentExpenses lists individual expenses for the current month.
	Lang           string
}
