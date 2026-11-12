package database

import (
	"testing"

	"expenser/internal/models"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestExpenseTableFor(t *testing.T) {
	house, err := expenseTableFor(models.SectionHouse)
	require.NoError(t, err)
	assert.Equal(t, expenseTable{Table: "home_expenses", TypeColumn: "utility_type_id", TypeTable: "utility_types", DateColumn: "expense_date", OwnerColumn: "created_by"}, house)
	car, err := expenseTableFor(models.SectionCar)
	require.NoError(t, err)
	assert.Equal(t, expenseTable{Table: "car_expenses", TypeColumn: "car_expense_type_id", TypeTable: "car_expense_types", DateColumn: "expense_date", OwnerColumn: "created_by"}, car)
	_, err = expenseTableFor(models.SectionGroceries)
	assert.ErrorIs(t, err, models.ErrInvalidCategory, "groceries have no types of this kind")
}

func TestGetSectionTypes(t *testing.T) {
	db, _ := setupMonthTestDB(t)
	house, err := db.GetSectionTypes(models.SectionHouse)
	require.NoError(t, err)
	car, err := db.GetSectionTypes(models.SectionCar)
	require.NoError(t, err)
	assert.NotEmpty(t, house)
	assert.NotEmpty(t, car)
	assert.Contains(t, house, models.TypeOption{ID: 4, Name: "Internet"})
	_, err = db.GetSectionTypes(models.SectionGroceries)
	assert.ErrorIs(t, err, models.ErrInvalidCategory)
}
