package services

import (
	"context"
	"expenser/internal/models"
	"expenser/internal/utilities"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

type fakeMonthStore struct {
	house         []models.HouseExpense
	groceries     []models.GroceriesExpense
	totals        map[string]float64
	gotMonth      time.Month
	gotYear       int
	gotFrom       time.Time
	gotTo         time.Time
	typeTotals    map[string]float64
	typeFrom      time.Time
	typeTo        time.Time
	bills         []models.RecurringPayment
	upcomingUntil time.Time

	history        []models.SpendEntry
	historySection string
	historyFrom    time.Time
	historyTo      time.Time
}

func (f *fakeMonthStore) spendHistory(section string, from, to time.Time) ([]models.SpendEntry, error) {
	f.historySection, f.historyFrom, f.historyTo = section, from, to
	return f.history, nil
}
func (f *fakeMonthStore) GetHouseSpendHistory(_ uuid.UUID, from, to time.Time) ([]models.SpendEntry, error) {
	return f.spendHistory("house", from, to)
}
func (f *fakeMonthStore) GetCarSpendHistory(_ uuid.UUID, from, to time.Time) ([]models.SpendEntry, error) {
	return f.spendHistory("car", from, to)
}
func (f *fakeMonthStore) GetGroceriesSpendHistory(_ uuid.UUID, from, to time.Time) ([]models.SpendEntry, error) {
	return f.spendHistory("groceries", from, to)
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
func (f *fakeMonthStore) GetUpcomingRecurringPayments(_ context.Context, _ uuid.UUID, until time.Time) ([]models.RecurringPayment, error) {
	f.upcomingUntil = until
	return f.bills, nil
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
		// Electricity (50 on the 15th) and water (20 on the 5th) for six
		// months; this month only the water is in.
		history: append(append(everyMonth("Electricity", 6, 15, 50), everyMonth("Water", 6, 5, 20)...),
			spend("Water", 0, 4, 20)),
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
	assert.InDelta(t, 220, v.BarsTotal, 0.001, "Jun-Nov summed, January outside the chart")
	assert.Equal(t, []models.TypeOption{{ID: 2, Name: "Water"}, {ID: 3, Name: "Gas"}}, v.Types)
	assert.Equal(t, "type.", v.TypePrefix)
	assert.False(t, v.Filtered)

	assert.Equal(t, YearMonth{2026, time.May}.Start(), store.typeFrom, "usual = previous 6 months")
	assert.Equal(t, YearMonth{2026, time.November}.Start(), store.typeTo, "current month excluded")
	require.Len(t, v.Breakdown, 2)
	assert.Equal(t, "Gas", v.Breakdown[0].Type)
	require.True(t, v.Breakdown[0].HasUsual)
	assert.InDelta(t, 50, v.Breakdown[0].Usual, 0.001)
	assert.Equal(t, 3, v.Breakdown[0].Color, "colour follows the type ID (Gas = 3), not its rank")
	assert.Equal(t, 2, v.Breakdown[1].Color, "Water = 2")

	assert.InDelta(t, 270, v.YearSoFar.Total, 0.001, "Jan 50 + Oct 100 + Nov 120")
	from, to := EstimateHistoryWindow(YearMonth{2026, time.November})
	assert.Equal(t, from, store.historyFrom, "estimate history from a year back")
	assert.Equal(t, to, store.historyTo)
	require.True(t, v.HasEstimate, "current month gets an estimate")
	assert.InDelta(t, 20+50, v.Estimate, 0.001, "water paid, electricity still due at its usual 50")
	assert.False(t, v.EstimateFromPace)
}

func TestMonthViewEstimatePerSection(t *testing.T) {
	require.NoError(t, utilities.InitI18n())
	now := time.Date(2026, 11, 10, 0, 0, 0, 0, time.UTC)
	ym := YearMonth{2026, time.November}
	store := newMonthStore()
	svc := NewMonthViewService(store)

	car, err := svc.Car(uuid.New(), ym, MonthFilter{}, "en", now)
	require.NoError(t, err)
	assert.Equal(t, "car", store.historySection)
	assert.True(t, car.HasEstimate)

	groceries, err := svc.Groceries(uuid.New(), ym, MonthFilter{}, "en", now)
	require.NoError(t, err)
	assert.Equal(t, "groceries", store.historySection)
	assert.True(t, groceries.HasEstimate)
}

func TestPastMonthHasNoEstimate(t *testing.T) {
	require.NoError(t, utilities.InitI18n())
	store := newMonthStore()
	v, err := NewMonthViewService(store).House(uuid.New(), YearMonth{2026, time.October}, MonthFilter{}, "en", time.Date(2026, 11, 10, 0, 0, 0, 0, time.UTC))
	require.NoError(t, err)
	assert.False(t, v.HasEstimate)
	assert.True(t, store.historyFrom.IsZero(), "no history query for a past month")
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

func TestHouseMonthViewUpcomingBills(t *testing.T) {
	require.NoError(t, utilities.InitI18n())
	store := newMonthStore()
	store.bills = []models.RecurringPayment{
		{Name: "Fiber", Category: models.SectionHouse, DueDate: time.Date(2026, 11, 14, 0, 0, 0, 0, time.UTC), Active: true},
		{Name: "Insurance", Category: models.SectionCar, DueDate: time.Date(2026, 11, 20, 0, 0, 0, 0, time.UTC), Active: true},
		{Name: "TV", Category: models.SectionHouse, DueDate: time.Date(2026, 11, 2, 0, 0, 0, 0, time.UTC), Active: true},
	}
	now := time.Date(2026, 11, 10, 15, 0, 0, 0, time.UTC)
	svc := NewMonthViewService(store)

	v, err := svc.House(uuid.New(), YearMonth{2026, time.November}, MonthFilter{}, "en", now)
	require.NoError(t, err)
	assert.Equal(t, time.Date(2026, 12, 10, 0, 0, 0, 0, time.UTC), store.upcomingUntil, "next 30 days")
	require.Len(t, v.UpcomingBills, 2, "house bills only")
	assert.Equal(t, "TV", v.UpcomingBills[0].Payment.Name, "overdue first")
	assert.Equal(t, -8, v.UpcomingBills[0].DaysLeft)
	assert.Equal(t, "Fiber", v.UpcomingBills[1].Payment.Name)
	assert.Equal(t, 4, v.UpcomingBills[1].DaysLeft)

	past, err := svc.House(uuid.New(), YearMonth{2026, time.October}, MonthFilter{}, "en", now)
	require.NoError(t, err)
	assert.Empty(t, past.UpcomingBills, "only on the current month")
}

func TestHouseMonthViewSubtractsDiscounts(t *testing.T) {
	require.NoError(t, utilities.InitI18n())
	store := newMonthStore()
	store.house = []models.HouseExpense{
		{ID: 1, UtilityTypeID: 3, UtilityType: "Gas", Amount: 85, DiscountAmount: 5},
		{ID: 2, UtilityTypeID: 2, UtilityType: "Water", Amount: 20},
	}
	v, err := NewMonthViewService(store).House(uuid.New(), YearMonth{2026, time.November}, MonthFilter{}, "en", time.Date(2026, 11, 10, 0, 0, 0, 0, time.UTC))
	require.NoError(t, err)
	assert.InDelta(t, 100, v.Summary.Total, 0.001, "85 - 5 + 20")
}

func TestGroceriesEstimateIgnoresCategories(t *testing.T) {
	require.NoError(t, utilities.InitI18n())
	// One shop a month, each time filed under a different category: every
	// category looks like a one-off, but the groceries themselves are not.
	store := &fakeMonthStore{}
	for back, cat := range []string{"", "Meat", "Dairy", "Fruit", "Bread", "Fish", "Wine"} {
		if back > 0 {
			store.history = append(store.history, spend(cat, back, 20, 100))
		}
	}
	v, err := NewMonthViewService(store).Groceries(uuid.New(), YearMonth{2026, time.November}, MonthFilter{}, "en", time.Date(2026, 11, 10, 0, 0, 0, 0, time.UTC))
	require.NoError(t, err)
	require.True(t, v.HasEstimate)
	assert.InDelta(t, 100, v.Estimate, 0.001)
}

// Groceries have far more categories than colours, so like the trends chart
// the six largest (not counting Other) get slots 1..6 by rank and the rest,
// and Other, get colour 0.
func TestGroceriesBreakdownColoursByRank(t *testing.T) {
	require.NoError(t, utilities.InitI18n())
	store := &fakeMonthStore{}
	names := []string{"A", "B", "C", "D", "E", "F", "G", "H"}
	for i, n := range names {
		store.groceries = append(store.groceries, models.GroceriesExpense{
			ID: i + 1, CategoryID: 10 + i, CategoryName: n, Product: n, TotalPrice: float64(100 - i*10)})
	}
	store.groceries = append(store.groceries, models.GroceriesExpense{ID: 99, CategoryID: 99, CategoryName: "Other", Product: "x", TotalPrice: 95})
	v, err := NewMonthViewService(store).Groceries(uuid.New(), YearMonth{2026, time.October}, MonthFilter{}, "en", time.Date(2026, 11, 10, 0, 0, 0, 0, time.UTC))
	require.NoError(t, err)
	got := map[string]int{}
	for _, b := range v.Breakdown {
		got[b.Type] = b.Color
	}
	assert.Equal(t, map[string]int{"A": 1, "Other": 0, "B": 2, "C": 3, "D": 4, "E": 5, "F": 6, "G": 0, "H": 0}, got)
}

func TestHouseBreakdownKeepsTypeColours(t *testing.T) {
	require.NoError(t, utilities.InitI18n())
	store := newMonthStore()
	store.house = []models.HouseExpense{{ID: 1, UtilityTypeID: 3, UtilityType: "Gas", Amount: 85}}
	v, err := NewMonthViewService(store).House(uuid.New(), YearMonth{2026, time.November}, MonthFilter{}, "en", time.Date(2026, 11, 10, 0, 0, 0, 0, time.UTC))
	require.NoError(t, err)
	require.NotEmpty(t, v.Breakdown)
	assert.Equal(t, 3, v.Breakdown[0].Color)
}
