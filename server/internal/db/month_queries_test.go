package database

import (
	"context"
	"expenser/internal/config"
	"expenser/internal/models"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func setupMonthTestDB(t *testing.T) (*DB, uuid.UUID) {
	t.Helper()
	cfg, err := config.LoadConfig()
	require.NoError(t, err)
	testDB := InitTestDB(cfg)
	t.Cleanup(func() { testDB.Close() })
	ResetTestDB(testDB)
	user := &models.User{Username: "MonthTester", PasswordHash: TestUserRegisterModel.PasswordHash}
	require.NoError(t, testDB.CreateUser(context.Background(), user))
	return testDB, user.ID
}

func day(y int, m time.Month, d int) time.Time {
	return time.Date(y, m, d, 12, 0, 0, 0, time.Local)
}

func addGrocery(t *testing.T, db *DB, userID uuid.UUID, product string, total, discount float64, at time.Time) {
	t.Helper()
	categories, err := db.GetAllCategories(context.Background())
	require.NoError(t, err)
	require.NotEmpty(t, categories)
	require.NoError(t, db.CreateGroceriesExpense(context.Background(), &models.GroceriesExpense{
		UserID: userID, CategoryID: categories[0].ID, Product: product, Quantity: 1,
		UnitPrice: total, TotalPrice: total, TotalDiscount: discount, PurchaseDate: at,
	}))
}

func TestGetGroceriesExpensesForMonth(t *testing.T) {
	db, userID := setupMonthTestDB(t)
	addGrocery(t, db, userID, "Milk", 2, 0, day(2026, 11, 3))
	addGrocery(t, db, userID, "Bread", 1.5, 0, day(2026, 11, 20))
	addGrocery(t, db, userID, "Old milk", 2, 0, day(2025, 11, 3)) // same month, other year
	addGrocery(t, db, userID, "October", 2, 0, day(2026, 10, 31))

	got, err := db.GetGroceriesExpensesForMonth(context.Background(), time.November, 2026, userID)
	require.NoError(t, err)
	require.Len(t, got, 2)
	assert.Equal(t, "Bread", got[0].Product, "newest first")
	assert.Equal(t, "Milk", got[1].Product)
	assert.NotEmpty(t, got[0].CategoryName)
}

func TestMonthlyTotals(t *testing.T) {
	db, userID := setupMonthTestDB(t)

	for _, e := range []struct {
		amount float64
		at     time.Time
	}{{10, day(2026, 9, 5)}, {15, day(2026, 9, 25)}, {40, day(2026, 11, 1)}, {99, day(2026, 6, 1)}} {
		require.NoError(t, db.CreateHouseExpense(context.Background(), &models.HouseExpense{UtilityTypeID: 2, Amount: e.amount, ExpenseDate: e.at, Metadata: []byte("{}"), CreatedBy: userID}))
		require.NoError(t, db.CreateCarExpense(context.Background(), &models.CarExpense{ExpenseTypeID: 1, Amount: e.amount, Date: e.at, Metadata: []byte("{}"), CreatedBy: userID}))
	}
	addGrocery(t, db, userID, "Milk", 10, 2, day(2026, 9, 5))

	from, to := day(2026, 7, 1), time.Date(2026, 12, 1, 0, 0, 0, 0, time.Local)

	house, err := db.GetHouseMonthlyTotals(context.Background(), userID, from, to)
	require.NoError(t, err)
	assert.Equal(t, map[string]float64{"2026-09": 25, "2026-11": 40}, house, "June is outside the range")

	car, err := db.GetCarMonthlyTotals(context.Background(), userID, from, to)
	require.NoError(t, err)
	assert.Equal(t, map[string]float64{"2026-09": 25, "2026-11": 40}, car)

	groceries, err := db.GetGroceriesMonthlyTotals(context.Background(), userID, from, to)
	require.NoError(t, err)
	assert.Equal(t, map[string]float64{"2026-09": 8}, groceries, "net of discount, like the monthly total")
}

func TestTypeTotals(t *testing.T) {
	db, userID := setupMonthTestDB(t)
	for _, e := range []struct {
		typeID int
		amount float64
		at     time.Time
	}{{2, 10, day(2026, 9, 5)}, {2, 15, day(2026, 10, 5)}, {3, 40, day(2026, 10, 9)}, {3, 99, day(2026, 11, 1)}} {
		require.NoError(t, db.CreateHouseExpense(context.Background(), &models.HouseExpense{UtilityTypeID: e.typeID, Amount: e.amount, ExpenseDate: e.at, Metadata: []byte("{}"), CreatedBy: userID}))
		require.NoError(t, db.CreateCarExpense(context.Background(), &models.CarExpense{ExpenseTypeID: e.typeID, Amount: e.amount, Date: e.at, Metadata: []byte("{}"), CreatedBy: userID}))
	}
	addGrocery(t, db, userID, "Milk", 10, 2, day(2026, 10, 5))

	from, to := day(2026, 5, 1), time.Date(2026, 11, 1, 0, 0, 0, 0, time.Local)

	house, err := db.GetHouseTypeTotals(context.Background(), userID, from, to)
	require.NoError(t, err)
	assert.Equal(t, map[string]float64{"Water": 25, "Gas": 40}, house, "November is outside the range")

	car, err := db.GetCarTypeTotals(context.Background(), userID, from, to)
	require.NoError(t, err)
	assert.Len(t, car, 2)

	groceries, err := db.GetGroceriesCategoryTotals(context.Background(), userID, from, to)
	require.NoError(t, err)
	require.Len(t, groceries, 1)
	for _, v := range groceries {
		assert.InDelta(t, 8, v, 0.001, "net of discount")
	}
}

func TestSpendHistory(t *testing.T) {
	db, userID := setupMonthTestDB(t)
	for _, e := range []struct {
		typeID int
		amount float64
		at     time.Time
	}{{2, 10, day(2026, 9, 5)}, {2, 15, day(2026, 9, 5)}, {3, 40, day(2026, 10, 9)}, {3, 99, day(2026, 12, 1)}} {
		require.NoError(t, db.CreateHouseExpense(context.Background(), &models.HouseExpense{UtilityTypeID: e.typeID, Amount: e.amount, DiscountAmount: 1, ExpenseDate: e.at, Metadata: []byte("{}"), CreatedBy: userID}))
		require.NoError(t, db.CreateCarExpense(context.Background(), &models.CarExpense{ExpenseTypeID: e.typeID, Amount: e.amount, Date: e.at, Metadata: []byte("{}"), CreatedBy: userID}))
	}
	addGrocery(t, db, userID, "Milk", 10, 2, day(2026, 10, 5))

	from, to := day(2026, 5, 1), time.Date(2026, 11, 1, 0, 0, 0, 0, time.Local)

	house, err := db.GetHouseSpendHistory(context.Background(), userID, from, to)
	require.NoError(t, err)
	require.Len(t, house, 2, "one row per type and day; December is outside the range")
	byType := map[string]models.SpendEntry{}
	for _, e := range house {
		byType[e.Type] = e
	}
	assert.InDelta(t, 23, byType["Water"].Amount, 0.001, "same-day amounts summed, net of discount")
	assert.Equal(t, 5, byType["Water"].Date.Day())
	assert.Equal(t, time.September, byType["Water"].Date.Month())
	assert.InDelta(t, 39, byType["Gas"].Amount, 0.001)

	car, err := db.GetCarSpendHistory(context.Background(), userID, from, to)
	require.NoError(t, err)
	assert.Len(t, car, 2)

	groceries, err := db.GetGroceriesSpendHistory(context.Background(), userID, from, to)
	require.NoError(t, err)
	require.Len(t, groceries, 1)
	assert.InDelta(t, 8, groceries[0].Amount, 0.001, "net of discount")
	assert.NotEmpty(t, groceries[0].Type, "keyed by category")
}

// A purchase just after local midnight on the 1st belongs to that month in
// every query, not only in the month list (whose EXTRACT uses the session
// zone): range bounds are local midnights, not UTC ones.
func TestGroceriesRangesUseLocalDays(t *testing.T) {
	db, userID := setupMonthTestDB(t)
	cfg, err := config.LoadConfig()
	require.NoError(t, err)
	addGrocery(t, db, userID, "Night snack", 3, 0, time.Date(2026, 11, 1, 0, 30, 0, 0, cfg.Location))

	ctx := context.Background()
	nov, err := db.GetGroceriesMonthlyTotals(ctx, userID, models.Date(2026, 11, 1), models.Date(2026, 12, 1))
	require.NoError(t, err)
	assert.Equal(t, map[string]float64{"2026-11": 3}, nov)

	oct, err := db.GetGroceriesMonthlyTotals(ctx, userID, models.Date(2026, 10, 1), models.Date(2026, 11, 1))
	require.NoError(t, err)
	assert.Empty(t, oct)

	stores, err := db.GetGroceriesCategoryTotals(ctx, userID, models.Date(2026, 11, 1), models.Date(2026, 12, 1))
	require.NoError(t, err)
	assert.Len(t, stores, 1)

	list, err := db.GetGroceriesExpensesForMonth(ctx, time.November, 2026, userID)
	require.NoError(t, err)
	assert.Len(t, list, 1)
}

// A receipt is saved all or nothing: an item that can't be stored leaves
// no other item of the receipt behind.
func TestCreateGroceriesExpensesIsAtomic(t *testing.T) {
	db, userID := setupMonthTestDB(t)
	ctx := context.Background()
	at := day(2026, 11, 5)

	good := &models.GroceriesExpense{UserID: userID, CategoryID: models.GroceryCategoryOther, Product: "Milk", Quantity: 1, TotalPrice: 2.456, PurchaseDate: at}
	bad := &models.GroceriesExpense{UserID: userID, CategoryID: 999999, Product: "Ghost", Quantity: 1, TotalPrice: 1, PurchaseDate: at}
	require.Error(t, db.CreateGroceriesExpenses(ctx, []*models.GroceriesExpense{good, bad}))

	list, err := db.GetGroceriesExpensesForMonth(ctx, time.November, 2026, userID)
	require.NoError(t, err)
	assert.Empty(t, list)

	good.ID = 0
	require.NoError(t, db.CreateGroceriesExpenses(ctx, []*models.GroceriesExpense{good}))
	assert.NotZero(t, good.ID)
	assert.Equal(t, "Other", good.CategoryName, "filled in for the row")
	assert.Equal(t, 2.46, good.TotalPrice, "as stored")
}

// Creating or editing an expense fills in the amounts as stored, so the
// success message shows what was saved (rounded to the cent).
func TestExpenseAmountsAsStored(t *testing.T) {
	db, userID := setupMonthTestDB(t)
	ctx := context.Background()
	h := &models.HouseExpense{UtilityTypeID: models.UtilityWater, Amount: 12.345, DiscountAmount: 0.004, ExpenseDate: models.Date(2026, 11, 2), CreatedBy: userID}
	require.NoError(t, db.CreateHouseExpense(ctx, h))
	assert.Equal(t, 12.35, h.Amount)
	assert.Equal(t, 0.0, h.DiscountAmount)

	c := &models.CarExpense{ExpenseTypeID: models.CarFuel, Amount: 40.999, Date: models.Date(2026, 11, 2), CreatedBy: userID}
	require.NoError(t, db.CreateCarExpense(ctx, c))
	assert.Equal(t, 41.0, c.Amount)
	c.Amount = 7.777
	require.NoError(t, db.EditCarExpense(ctx, c, userID))
	assert.Equal(t, 7.78, c.Amount)
}
