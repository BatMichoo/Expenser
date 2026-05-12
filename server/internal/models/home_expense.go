package models

import (
	"encoding/json"
	"fmt"
	"time"

	"github.com/google/uuid"
)

type HouseExpense struct {
	ID            int             `form:"id"`
	UtilityTypeID int             `form:"typeID"`
	UtilityType   string          `form:"type" binding:"required"`
	Amount        float64         `form:"amount" binding:"required"`
	ExpenseDate   time.Time       `form:"date" binding:"required"`
	Notes         string          `form:"notes"`
	Metadata      json.RawMessage `form:"metadata"`
	CreatedAt     time.Time       `form:"createdAt"`
	CreatedBy     uuid.UUID
}

func (h HouseExpense) FormattedMetadata(lang string) string {
	if len(h.Metadata) == 0 {
		return ""
	}

	switch h.UtilityTypeID {
	case 1: // Electricity
		var m ElectricityMetadata
		json.Unmarshal(h.Metadata, &m)
		return fmt.Sprintf("Day: %.2f (%.4f/kWh), Night: %.2f (%.4f/kWh)", m.KWhDay, m.PriceDay, m.KWhNight, m.PriceNight)
	case 2: // Water
		var m WaterMetadata
		json.Unmarshal(h.Metadata, &m)
		return fmt.Sprintf("Qty: %.2f m³ (%.2f/m³)", m.Quantity, m.PricePerUnit)
	case 3: // Gas
		var m GasMetadata
		json.Unmarshal(h.Metadata, &m)
		return fmt.Sprintf("Qty: %.2f (%.2f/unit)", m.Quantity, m.PricePerUnit)
	case 4, 5: // Internet, TV
		var m InternetTVMetadata
		json.Unmarshal(h.Metadata, &m)
		return fmt.Sprintf("%s - %s", m.Provider, m.Plan)
	case 7: // Other
		var m OtherMetadata
		json.Unmarshal(h.Metadata, &m)
		return m.CustomName
	}
	return ""
}

func (h HouseExpense) UnmarshalMetadata() interface{} {
	if len(h.Metadata) == 0 {
		return nil
	}
	switch h.UtilityTypeID {
	case 1:
		var m ElectricityMetadata
		json.Unmarshal(h.Metadata, &m)
		return m
	case 2:
		var m WaterMetadata
		json.Unmarshal(h.Metadata, &m)
		return m
	case 3:
		var m GasMetadata
		json.Unmarshal(h.Metadata, &m)
		return m
	case 4, 5:
		var m InternetTVMetadata
		json.Unmarshal(h.Metadata, &m)
		return m
	case 7:
		var m OtherMetadata
		json.Unmarshal(h.Metadata, &m)
		return m
	}
	return nil
}

// ElectricityMetadata represents detailed info for electricity bills
type ElectricityMetadata struct {
	KWhDay     float64 `json:"kWh_day"`
	KWhNight    float64 `json:"kWh_night"`
	PriceDay   float64 `json:"price_day"`
	PriceNight float64 `json:"price_night"`
}

// GasMetadata represents detailed info for gas bills
type GasMetadata struct {
	Quantity     float64 `json:"quantity"`
	PricePerUnit float64 `json:"price_per_unit"`
}

// WaterMetadata represents detailed info for water bills
type WaterMetadata struct {
	Quantity     float64 `json:"quantity"` // in m3
	PricePerUnit float64 `json:"price_per_unit"`
}

// InternetTVMetadata represents info for internet or TV bills
type InternetTVMetadata struct {
	Provider string `json:"provider"`
	Plan     string `json:"plan"`
}

type HomeUtilityType struct {
	ID   int
	Name string
}

// HouseExpResponse is the data structure returned to the client
// after a new expense has been successfully created.
// It includes details of the newly created expense and updated summary data.
type HouseExpResponse struct {
	Expense        *HouseExpense   // Expense is the newly created home expense record.
	MonthlyExpense *MonthlyExpense // MonthlyExpense provides the updated total for the current month.
	HighestExpense *HighestExpense // HighestExpense provides the updated highest expense for the current month.
	Modal          *ModalContent
	Lang           string
}

type HouseData struct {
	Name           string
	MonthlyExpense *MonthlyExpense // MonthlyExpense summarizes the total spending for the current month.
	HighestExpense *HighestExpense // HighestExpense identifies the single largest expense in the current month.
	RecentExpenses *[]HouseExpense // RecentExpenses lists individual expenses for the current month.
	Lang           string
}

type EditHouseFormData struct {
	Expense  *HouseExpense
	Types    *[]HomeUtilityType
	Metadata interface{}
	Lang     string
}
