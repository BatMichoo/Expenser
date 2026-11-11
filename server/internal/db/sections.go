package database

import (
	"fmt"

	"expenser/internal/models"
)

// expenseTable is where a section's typed expenses are stored. The names
// are fixed identifiers, never user input, so they are safe to put into
// SQL text.
type expenseTable struct {
	Table      string
	TypeColumn string
}

// expenseTableFor returns the house or car expense table.
func expenseTableFor(s models.Section) (expenseTable, error) {
	switch s {
	case models.SectionHouse:
		return expenseTable{Table: "home_expenses", TypeColumn: "utility_type_id"}, nil
	case models.SectionCar:
		return expenseTable{Table: "car_expenses", TypeColumn: "car_expense_type_id"}, nil
	}
	return expenseTable{}, fmt.Errorf("section %q: %w", s, models.ErrInvalidCategory)
}

// GetSectionTypes lists the expense types of the house or car section.
func (db *DB) GetSectionTypes(s models.Section) ([]models.TypeOption, error) {
	var opts []models.TypeOption
	switch s {
	case models.SectionHouse:
		types, err := db.GetHouseUtilityTypes()
		if err != nil {
			return nil, err
		}
		for _, t := range *types {
			opts = append(opts, models.TypeOption{ID: t.ID, Name: t.Name})
		}
	case models.SectionCar:
		types, err := db.GetCarExpenseTypes()
		if err != nil {
			return nil, err
		}
		for _, t := range *types {
			opts = append(opts, models.TypeOption{ID: t.ID, Name: t.Name})
		}
	default:
		return nil, fmt.Errorf("section %q: %w", s, models.ErrInvalidCategory)
	}
	return opts, nil
}
