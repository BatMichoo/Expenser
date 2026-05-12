package models

import (
	"encoding/json"
	"fmt"
	"strings"
	"time"

	"github.com/google/uuid"
)

type CarExpense struct {
	ID            int             `form:"id"`
	ExpenseTypeID int             `form:"typeID"`
	Type          string          `form:"type" binding:"required"`
	Amount        float64         `form:"amount" binding:"required"`
	Date          time.Time       `form:"date" binding:"required"`
	Notes         string          `form:"notes"`
	Metadata      json.RawMessage `form:"metadata"`
	CreatedAt     time.Time       `form:"createdAt"`
	CreatedBy     uuid.UUID
}

func (c CarExpense) FormattedMetadata() string {
	if len(c.Metadata) == 0 {
		return ""
	}

	switch c.ExpenseTypeID {
	case 1: // Fuel
		var m FuelMetadata
		json.Unmarshal(c.Metadata, &m)
		return fmt.Sprintf("%.2f L (%.2f/L)", m.Liters, m.PricePerLiter)
	case 2: // Maintenance
		var m MaintenanceMetadata
		json.Unmarshal(c.Metadata, &m)
		return fmt.Sprintf("Parts: %s, Labor: %.2f", strings.Join(m.Parts, ", "), m.LaborCost)
	case 3: // Insurance
		var m InsuranceMetadata
		json.Unmarshal(c.Metadata, &m)
		return fmt.Sprintf("%s (%s) until %s", m.Provider, m.CoverageType, m.ValidUntil.Format("02.01.2006"))
	case 5: // Parking/Tolls
		var m ParkingTollsMetadata
		json.Unmarshal(c.Metadata, &m)
		return fmt.Sprintf("%s (%s)", m.Location, m.Duration)
	}
	return ""
}

func (c CarExpense) UnmarshalMetadata() interface{} {
	if len(c.Metadata) == 0 {
		return nil
	}
	switch c.ExpenseTypeID {
	case 1:
		var m FuelMetadata
		json.Unmarshal(c.Metadata, &m)
		return m
	case 2:
		var m MaintenanceMetadata
		json.Unmarshal(c.Metadata, &m)
		return m
	case 3:
		var m InsuranceMetadata
		json.Unmarshal(c.Metadata, &m)
		return m
	case 5:
		var m ParkingTollsMetadata
		json.Unmarshal(c.Metadata, &m)
		return m
	}
	return nil
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
// It includes details of the newly created expense and updated summary data.
type CarExpResponse struct {
	Expense        *CarExpense     // Expense is the newly created car expense record.
	MonthlyExpense *MonthlyExpense // MonthlyExpense provides the updated total for the current month.
	HighestExpense *HighestExpense // HighestExpense provides the updated highest expense for the current month.
	Modal          *ModalContent
	Lang           string
}

type CarData struct {
	Name           string
	MonthlyExpense *MonthlyExpense // MonthlyExpense summarizes the total spending for the current month.
	HighestExpense *HighestExpense // HighestExpense identifies the single largest expense in the current month.
	RecentExpenses *[]CarExpense   // RecentExpenses lists individual expenses for the current month.
	Lang           string
}

type EditCarFormData struct {
	Expense  *CarExpense
	Types    *[]CarExpenseType
	Metadata interface{}
	Lang     string
}
