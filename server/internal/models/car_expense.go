package models

import (
	"expenser/internal/utilities"
	"fmt"
	"strings"
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

// FormattedMetadata is the expense's metadata as one line of text, empty
// when it has none.
func (c CarExpense) FormattedMetadata(lang string) string {
	switch m := c.UnmarshalMetadata().(type) {
	case FuelMetadata:
		return fmt.Sprintf("%.2f L (%.2f/L)", m.Liters, m.PricePerLiter)
	case MaintenanceMetadata:
		translatedParts := make([]string, len(m.Parts))
		for i, part := range m.Parts {
			translatedParts[i] = utilities.T(lang, "parts."+part)
		}
		return fmt.Sprintf("Part: %s, Labor: %.2f", strings.Join(translatedParts, ", "), m.LaborCost)
	case InsuranceMetadata:
		return fmt.Sprintf("%s (%s) until %s", m.Provider, m.CoverageType, m.ValidUntil.Format(utilities.DateFormats.Input))
	case ParkingTollsMetadata:
		return fmt.Sprintf("%s (%s)", m.Location, m.Duration)
	case OtherMetadata:
		return m.CustomName
	}
	return ""
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
