package services

import (
	"expenser/internal/models"
	"expenser/internal/utilities"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

type fakeMonthStore struct {
	house      []models.HouseExpense
	groceries  []models.GroceriesExpense
	totals     map[string]float64
	gotMonth   time.Month
	gotYear    int
	gotFrom    time.Time
	gotTo      time.Time
	typeTotals map[string]float64
	typeFrom   time.Time
	typeTo     time.Time
}

func (f *fakeMonthStore) GetHouseExpensesForMonth(m time.Month, y int, _ uuid.UUID) (*[]models.HouseExpense, error) {
	f.gotMonth, f.gotYear = m, y
	rows := append([]models.HouseExpense(nil), f.house...)
	return &rows, nil
}
func (f *fakeMonthStore) GetHouseMonthlyTotals(_ uuid.UUID, from, to time.Time) (map[string]float64, error) {
	f.gotFrom, f.gotTo = from, to
	return f.totals, nil
}
func (f *fakeMonthStore) GetHouseUtilityTypes() (*[]models.HomeUtilityType, error) {
	return &[]models.HomeUtilityType{{ID: 2, Name: "Water"}, {ID: 3, Name: "Gas"}}, nil
}
func (f *fakeMonthStore) GetCarExpensesForMonth(time.Month, int, uuid.UUID) (*[]models.CarExpense, error) {
	return &[]models.CarExpense{}, nil
}
func (f *fakeMonthStore) GetCarMonthlyTotals(uuid.UUID, time.Time, time.Time) (map[string]float64, error) {
	return f.totals, nil
}
func (f *fakeMonthStore) GetCarExpenseTypes() (*[]models.CarExpenseType, error) {
	return &[]models.CarExpenseType{}, nil
}
func (f *fakeMonthStore) GetGroceriesExpensesForMonth(time.Month, int, uuid.UUID) (*[]models.GroceriesExpense, error) {
	rows := append([]models.GroceriesExpense(nil), f.groceries...)
	return &rows, nil
}
func (f *fakeMonthStore) GetGroceriesMonthlyTotals(uuid.UUID, time.Time, time.Time) (map[string]float64, error) {
	return f.totals, nil
}
func (f *fakeMonthStore) GetHouseTypeTotals(_ uuid.UUID, from, to time.Time) (map[string]float64, error) {
	f.typeFrom, f.typeTo = from, to
	return f.typeTotals, nil
}
func (f *fakeMonthStore) GetCarTypeTotals(uuid.UUID, time.Time, time.Time) (map[string]float64, error) {
	return f.typeTotals, nil
}
func (f *fakeMonthStore) GetGroceriesCategoryTotals(uuid.UUID, time.Time, time.Time) (map[string]float64, error) {
	return f.typeTotals, nil
}
func (f *fakeMonthStore) GetAllCategories() ([]models.Category, error) {
	return []models.Category{{ID: 1, Name: "Meat"}, {ID: 2, Name: "Dairy"}}, nil
}

func newMonthStore() *fakeMonthStore {
	return &fakeMonthStore{
		house: []models.HouseExpense{
			{ID: 1, UtilityTypeID: 3, UtilityType: "Gas", Amount: 85, Notes: "meter"},
			{ID: 2, UtilityTypeID: 2, UtilityType: "Water", Amount: 20},
			{ID: 3, UtilityTypeID: 3, UtilityType: "Gas", Amount: 15},
		},
		totals:     map[string]float64{"2026-01": 50, "2026-10": 100, "2026-11": 120},
		typeTotals: map[string]float64{"Gas": 300},
	}
}

func TestHouseMonthView(t *testing.T) {
	require.NoError(t, utilities.InitI18n())
	store := newMonthStore()
	svc := NewMonthViewService(store)

	v, err := svc.House(uuid.New(), YearMonth{2026, time.November}, MonthFilter{}, "en", time.Date(2026, 11, 10, 0, 0, 0, 0, time.UTC))
	require.NoError(t, err)

	assert.Equal(t, time.November, store.gotMonth)
	assert.Equal(t, 2026, store.gotYear)
	assert.Equal(t, YearMonth{2026, time.January}.Start(), store.gotFrom, "totals from January for the year so far")
	assert.Equal(t, YearMonth{2026, time.November}.End(), store.gotTo)

	assert.Equal(t, "2026-11", v.Month)
	assert.Equal(t, "2026-10", v.PrevMonth)
	assert.Equal(t, "2026-12", v.NextMonth)
	assert.True(t, v.IsCurrent)
	assert.Len(t, v.House, 3)
	assert.InDelta(t, 120, v.Summary.Total, 0.001)
	assert.Equal(t, "Gas", v.Summary.TopType)
	require.True(t, v.HasChange)
	assert.InDelta(t, 20, v.ChangePct, 0.001)
	assert.Len(t, v.Bars, 6)
	assert.Equal(t, []models.TypeOption{{ID: 2, Name: "Water"}, {ID: 3, Name: "Gas"}}, v.Types)
	assert.Equal(t, "type.", v.TypePrefix)
	assert.False(t, v.Filtered)

	assert.Equal(t, YearMonth{2026, time.May}.Start(), store.typeFrom, "usual = previous 6 months")
	assert.Equal(t, YearMonth{2026, time.November}.Start(), store.typeTo, "current month excluded")
	require.Len(t, v.Breakdown, 2)
	assert.Equal(t, "Gas", v.Breakdown[0].Type)
	require.True(t, v.Breakdown[0].HasUsual)
	assert.InDelta(t, 50, v.Breakdown[0].Usual, 0.001)

	assert.InDelta(t, 270, v.YearSoFar.Total, 0.001, "Jan 50 + Oct 100 + Nov 120")
	assert.InDelta(t, 100.0/6, v.Usual, 0.001, "May-Oct average")
	require.True(t, v.HasEstimate, "current month gets an estimate")
	assert.InDelta(t, 120+(100.0/6)*20/30, v.Estimate, 0.001)
}

func TestPastMonthHasNoEstimate(t *testing.T) {
	require.NoError(t, utilities.InitI18n())
	v, err := NewMonthViewService(newMonthStore()).House(uuid.New(), YearMonth{2026, time.October}, MonthFilter{}, "en", time.Date(2026, 11, 10, 0, 0, 0, 0, time.UTC))
	require.NoError(t, err)
	assert.False(t, v.HasEstimate)
}

func TestHouseMonthViewFilters(t *testing.T) {
	require.NoError(t, utilities.InitI18n())
	now := time.Date(2026, 11, 10, 0, 0, 0, 0, time.UTC)
	svc := NewMonthViewService(newMonthStore())

	v, err := svc.House(uuid.New(), YearMonth{2026, time.November}, MonthFilter{TypeID: 3}, "en", now)
	require.NoError(t, err)
	assert.Len(t, v.House, 2, "type filter")
	assert.InDelta(t, 120, v.Summary.Total, 0.001, "KPIs ignore filters")
	assert.True(t, v.Filtered)

	v, err = svc.House(uuid.New(), YearMonth{2026, time.November}, MonthFilter{Query: "METER"}, "en", now)
	require.NoError(t, err)
	require.Len(t, v.House, 1)
	assert.Equal(t, 1, v.House[0].ID)

	v, err = svc.House(uuid.New(), YearMonth{2026, time.November}, MonthFilter{Query: "вода"}, "bg", now)
	require.NoError(t, err)
	require.Len(t, v.House, 1, "matches the Bulgarian type name")
	assert.Equal(t, 2, v.House[0].ID)
}

func TestGroceriesMonthViewUsesNetAmounts(t *testing.T) {
	require.NoError(t, utilities.InitI18n())
	store := &fakeMonthStore{
		groceries: []models.GroceriesExpense{
			{ID: 1, CategoryID: 1, CategoryName: "Meat", Product: "Chicken", TotalPrice: 10, TotalDiscount: 2, SupermarketName: "Lidl"},
			{ID: 2, CategoryID: 2, CategoryName: "Dairy", Product: "Milk", TotalPrice: 3},
		},
	}
	v, err := NewMonthViewService(store).Groceries(uuid.New(), YearMonth{2026, time.October}, MonthFilter{Query: "lidl"}, "en", time.Date(2026, 11, 10, 0, 0, 0, 0, time.UTC))
	require.NoError(t, err)
	assert.InDelta(t, 11, v.Summary.Total, 0.001, "price minus discount")
	assert.Equal(t, "Meat", v.Summary.TopType)
	assert.False(t, v.IsCurrent)
	assert.False(t, v.HasChange, "no previous month data")
	require.Len(t, v.Groceries, 1, "store name is searchable")
	assert.Equal(t, "category.", v.TypePrefix)
}
