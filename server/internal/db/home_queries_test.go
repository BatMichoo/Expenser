package database

import (
	"context"
	"testing"
	"time"

	"expenser/internal/models"

	"github.com/google/uuid"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func addGroceryAt(t *testing.T, db *DB, userID uuid.UUID, store string, total, discount float64, at time.Time) {
	t.Helper()
	categories, err := db.GetAllCategories(context.Background())
	require.NoError(t, err)
	require.NoError(t, db.CreateGroceriesExpense(context.Background(), &models.GroceriesExpense{
		UserID: userID, CategoryID: categories[0].ID, Product: "x", Quantity: 1,
		UnitPrice: total, TotalPrice: total, TotalDiscount: discount, SupermarketName: store, PurchaseDate: at,
	}))
}

func TestGroceriesStoreTotals(t *testing.T) {
	db, userID := setupMonthTestDB(t)
	addGroceryAt(t, db, userID, "Lidl", 10, 1, day(2026, 3, 1))
	addGroceryAt(t, db, userID, "Lidl", 5, 0, day(2026, 4, 1))
	addGroceryAt(t, db, userID, "Billa", 7, 0, day(2026, 4, 2))
	addGroceryAt(t, db, userID, "  ", 3, 0, day(2026, 4, 3))
	addGroceryAt(t, db, userID, "Lidl", 50, 0, day(2025, 4, 1)) // outside the range

	got, err := db.GetGroceriesStoreTotals(context.Background(), userID, day(2026, 1, 1), day(2026, 12, 31))
	require.NoError(t, err)
	assert.Equal(t, map[string]float64{"Lidl": 14, "Billa": 7}, got, "net of discount; blank store names left out")
}

func TestBillTypeTotal(t *testing.T) {
	db, userID := setupMonthTestDB(t)
	ctx := context.Background()
	for _, e := range []struct {
		typeID int
		amount float64
		at     time.Time
	}{{2, 58, day(2025, 11, 10)}, {2, 2, day(2025, 11, 28)}, {3, 99, day(2025, 11, 10)}, {2, 70, day(2025, 12, 1)}} {
		require.NoError(t, db.CreateHouseExpense(context.Background(), &models.HouseExpense{UtilityTypeID: e.typeID, Amount: e.amount, ExpenseDate: e.at, Metadata: []byte("{}"), CreatedBy: userID}))
	}
	require.NoError(t, db.CreateCarExpense(context.Background(), &models.CarExpense{ExpenseTypeID: 2, Amount: 300, Date: day(2025, 11, 5), Metadata: []byte("{}"), CreatedBy: userID}))

	nov := time.Date(2025, 11, 1, 0, 0, 0, 0, time.Local)
	dec := time.Date(2025, 12, 1, 0, 0, 0, 0, time.Local)

	total, found, err := db.GetBillTypeTotal(ctx, userID, models.SectionHouse, 2, nov, dec)
	require.NoError(t, err)
	assert.True(t, found)
	assert.Equal(t, 60.0, total, "only that type, only that month")

	total, found, err = db.GetBillTypeTotal(ctx, userID, models.SectionCar, 2, nov, dec)
	require.NoError(t, err)
	assert.True(t, found)
	assert.Equal(t, 300.0, total, "car bills look at car expenses")

	_, found, err = db.GetBillTypeTotal(ctx, userID, models.SectionHouse, 4, nov, dec)
	require.NoError(t, err)
	assert.False(t, found, "nothing paid for that type that month")
}

func TestHouseAndCarTotalsSubtractDiscounts(t *testing.T) {
	db, userID := setupMonthTestDB(t)
	ctx := context.Background()
	at := day(2026, 9, 5)
	require.NoError(t, db.CreateHouseExpense(context.Background(), &models.HouseExpense{UtilityTypeID: 2, Amount: 100, DiscountAmount: 10, ExpenseDate: at, Metadata: []byte("{}"), CreatedBy: userID}))
	require.NoError(t, db.CreateHouseExpense(context.Background(), &models.HouseExpense{UtilityTypeID: 2, Amount: 20, ExpenseDate: at, Metadata: []byte("{}"), CreatedBy: userID}))
	require.NoError(t, db.CreateCarExpense(context.Background(), &models.CarExpense{ExpenseTypeID: 1, Amount: 50, DiscountAmount: 5, Date: at, Metadata: []byte("{}"), CreatedBy: userID}))
	from, to := day(2026, 1, 1), day(2026, 12, 31)

	house, err := db.GetHouseMonthlyTotals(context.Background(), userID, from, to)
	require.NoError(t, err)
	assert.Equal(t, 110.0, house["2026-09"])
	houseTypes, err := db.GetHouseTypeTotals(context.Background(), userID, from, to)
	require.NoError(t, err)
	assert.Equal(t, 110.0, sumMap(houseTypes))
	car, err := db.GetCarMonthlyTotals(context.Background(), userID, from, to)
	require.NoError(t, err)
	assert.Equal(t, 45.0, car["2026-09"])
	carTypes, err := db.GetCarTypeTotals(context.Background(), userID, from, to)
	require.NoError(t, err)
	assert.Equal(t, 45.0, sumMap(carTypes))

	billHouse, _, err := db.GetBillTypeTotal(ctx, userID, models.SectionHouse, 2, from, to)
	require.NoError(t, err)
	assert.Equal(t, 110.0, billHouse)
	billCar, _, err := db.GetBillTypeTotal(ctx, userID, models.SectionCar, 1, from, to)
	require.NoError(t, err)
	assert.Equal(t, 45.0, billCar)
}

func sumMap(m map[string]float64) float64 {
	t := 0.0
	for _, v := range m {
		t += v
	}
	return t
}
