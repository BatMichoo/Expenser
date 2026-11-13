package database

import (
	"context"
	"fmt"

	"expenser/internal/models"
)

// expenseTable is where a section's typed expenses are stored. The names
// are fixed identifiers, never user input, so they are safe to put into
// SQL text.
type expenseTable struct {
	Table       string
	TypeColumn  string
	TypeTable   string
	DateColumn  string
	OwnerColumn string
}

// expenseTableFor returns the house or car expense table.
func expenseTableFor(s models.Section) (expenseTable, error) {
	switch s {
	case models.SectionHouse:
		return expenseTable{Table: "home_expenses", TypeColumn: "utility_type_id", TypeTable: "utility_types", DateColumn: "expense_date", OwnerColumn: "created_by"}, nil
	case models.SectionCar:
		return expenseTable{Table: "car_expenses", TypeColumn: "car_expense_type_id", TypeTable: "car_expense_types", DateColumn: "expense_date", OwnerColumn: "created_by"}, nil
	}
	return expenseTable{}, fmt.Errorf("section %q: %w", s, models.ErrInvalidCategory)
}

// GetSectionTypes lists the expense types of the house or car section.
func (db *DB) GetSectionTypes(ctx context.Context, s models.Section) ([]models.TypeOption, error) {
	var opts []models.TypeOption
	switch s {
	case models.SectionHouse:
		types, err := db.GetHouseUtilityTypes(ctx)
		if err != nil {
			return nil, err
		}
		for _, t := range types {
			opts = append(opts, models.TypeOption{ID: t.ID, Name: t.Name})
		}
	case models.SectionCar:
		types, err := db.GetCarExpenseTypes(ctx)
		if err != nil {
			return nil, err
		}
		for _, t := range types {
			opts = append(opts, models.TypeOption{ID: t.ID, Name: t.Name})
		}
	default:
		return nil, fmt.Errorf("section %q: %w", s, models.ErrInvalidCategory)
	}
	return opts, nil
}
