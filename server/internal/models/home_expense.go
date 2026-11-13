package models

import (
	"time"

	"github.com/google/uuid"
)

type HouseExpense struct {
	ID             int       `form:"id"`
	UtilityTypeID  int       `form:"typeID"`
	UtilityType    string    `form:"type" binding:"required"`
	Amount         float64   `form:"amount"`
	DiscountAmount float64   `form:"discount_amount"`
	ExpenseDate    time.Time `form:"date" binding:"required"`
	Notes          string    `form:"notes"`
	Metadata       Metadata  `form:"metadata"`
	CreatedAt      time.Time `form:"createdAt"`
	CreatedBy      uuid.UUID
}

// UnmarshalMetadata returns the metadata struct for the expense's utility
// type, or nil when it has none.
func (h HouseExpense) UnmarshalMetadata() any {
	return decodeMetadata(h.Metadata, newHouseMetadata(h.UtilityTypeID))
}

// ElectricityMetadata represents detailed info for electricity bills
type ElectricityMetadata struct {
	KWhDay     float64 `json:"kWh_day"`
	KWhNight   float64 `json:"kWh_night"`
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
type HouseExpResponse struct {
	Expense *HouseExpense // Expense is the newly created home expense record.
	Modal   *ModalContent
	Lang    string
}

type EditHouseFormData struct {
	Expense  *HouseExpense
	Types    []HomeUtilityType
	Metadata any
	Lang     string
}
