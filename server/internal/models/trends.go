package models

import (
	"errors"
	"time"
)

// LegendItem is one entry of a trend chart legend.
type LegendItem struct {
	Name   string
	Color  int
	Dashed bool
}

// MonthTypeTotal is the net spend of one expense type (or grocery category)
// in one month.
type MonthTypeTotal struct {
	Month  string // YYYY-MM
	TypeID int
	Type   string
	Amount float64
}

// GroceryLine is one grocery purchase as the item-detail trends see it.
type GroceryLine struct {
	ID            int
	CategoryID    int
	Category      string
	Product       string
	Store         string
	Date          time.Time
	Quantity      float64
	UnitSize      float64
	TotalPrice    float64
	TotalDiscount float64
	Unit          string
	SizeStatus    string
}

// ErrInvalidSize is returned when a unit and size cannot be stored.
var ErrInvalidSize = errors.New("the unit or size is not valid")
