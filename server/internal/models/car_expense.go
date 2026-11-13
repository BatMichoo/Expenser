package models

import (
	"time"

	"github.com/google/uuid"
)

type CarExpense struct {
	ID             int       `form:"id"`
	ExpenseTypeID  int       `form:"typeID"`
	Type           string    `form:"type" binding:"required"`
	Amount         float64   `form:"amount"`
	DiscountAmount float64   `form:"discount_amount"`
	Date           time.Time `form:"date" binding:"required"`
	Notes          string    `form:"notes"`
	Metadata       Metadata  `form:"metadata"`
	CreatedAt      time.Time `form:"createdAt"`
	CreatedBy      uuid.UUID
}

// UnmarshalMetadata returns the metadata struct for the expense's type, or
// nil when it has none.
func (c CarExpense) UnmarshalMetadata() any {
	return decodeMetadata(c.Metadata, newCarMetadata(c.ExpenseTypeID))
}

// FuelMetadata represents detailed info for fuel purchases
type FuelMetadata struct {
	Liters        float64 `json:"liters"`
	PricePerLiter float64 `json:"price_per_liter"`
}

// MaintenanceMetadata represents detailed info for car repairs/maintenance
type MaintenanceMetadata struct {
	Parts     []string `json:"parts"`
	LaborCost float64  `json:"labor_cost"`
}

// InsuranceMetadata represents info for car insurance
type InsuranceMetadata struct {
	Provider     string    `json:"provider"`
	CoverageType string    `json:"coverage_type"`
	ValidUntil   time.Time `json:"valid_until"`
}

// ParkingTollsMetadata represents info for parking or tolls
type ParkingTollsMetadata struct {
	Location string `json:"location"`
	Duration string `json:"duration"` // e.g. "2 hours"
}

type CarExpenseType struct {
	ID   int
	Name string
}

// CarExpResponse is the data structure returned to the client
// after a new expense has been successfully created.
type CarExpResponse struct {
	Expense *CarExpense // Expense is the newly created car expense record.
	Modal   *ModalContent
	Lang    string
}

type EditCarFormData struct {
	Expense  *CarExpense
	Types    []CarExpenseType
	Metadata any
	Lang     string
}
