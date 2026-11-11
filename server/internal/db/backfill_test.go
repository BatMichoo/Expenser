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

func TestExpenseMonthsOfType(t *testing.T) {
	db, userID := setupMonthTestDB(t)
	ctx := context.Background()
	for _, e := range []struct {
		typeID int
		at     time.Time
	}{{4, day(2026, 1, 20)}, {4, day(2026, 1, 25)}, {4, day(2026, 3, 2)}, {5, day(2026, 2, 20)}, {4, day(2025, 12, 20)}} {
		require.NoError(t, db.CreateHouseExpense(&models.HouseExpense{UtilityTypeID: e.typeID, Amount: 15, ExpenseDate: e.at, Metadata: []byte("{}"), CreatedBy: userID}))
	}
	require.NoError(t, db.CreateCarExpense(&models.CarExpense{ExpenseTypeID: 4, Amount: 9, Date: day(2026, 2, 3), Metadata: []byte("{}"), CreatedBy: userID}))

	from, to := time.Date(2026, 1, 1, 0, 0, 0, 0, time.Local), time.Date(2026, 4, 1, 0, 0, 0, 0, time.Local)
	got, err := db.ExpenseMonthsOfType(ctx, userID, models.SectionHouse, 4, from, to)
	require.NoError(t, err)
	assert.Equal(t, map[string]bool{"2026-01": true, "2026-03": true}, got, "only that type, only in range")

	car, err := db.ExpenseMonthsOfType(ctx, userID, models.SectionCar, 4, from, to)
	require.NoError(t, err)
	assert.Equal(t, map[string]bool{"2026-02": true}, car, "car looks at car expenses")
}

func TestCreateBackfillExpenses(t *testing.T) {
	db, userID := setupMonthTestDB(t)
	ctx := context.Background()
	dates := []time.Time{day(2026, 1, 20), day(2026, 2, 20)}
	e := models.BackfillExpense{UserID: userID, Section: models.SectionHouse, TypeID: 4, Amount: 15, Discount: 1, Notes: "fiber"}
	require.NoError(t, db.CreateBackfillExpenses(ctx, e, dates))

	jan, err := db.GetHouseExpensesForMonth(time.January, 2026, userID)
	require.NoError(t, err)
	require.Len(t, *jan, 1)
	got := (*jan)[0]
	assert.Equal(t, "Internet", got.UtilityType)
	assert.Equal(t, 15.0, got.Amount)
	assert.Equal(t, 1.0, got.DiscountAmount)
	assert.Equal(t, "fiber", got.Notes)
	assert.Equal(t, 20, got.ExpenseDate.Day())

	carE := models.BackfillExpense{UserID: userID, Section: models.SectionCar, TypeID: 3, Amount: 50}
	require.NoError(t, db.CreateBackfillExpenses(ctx, carE, []time.Time{day(2026, 5, 1)}))
	may, err := db.GetCarExpensesForMonth(time.May, 2026, userID)
	require.NoError(t, err)
	assert.Len(t, *may, 1)

	bad := models.BackfillExpense{UserID: userID, Section: models.SectionHouse, TypeID: 999, Amount: 1}
	assert.Error(t, db.CreateBackfillExpenses(ctx, bad, []time.Time{day(2026, 6, 1), day(2026, 7, 1)}))
	jun, err := db.GetHouseExpensesForMonth(time.June, 2026, userID)
	require.NoError(t, err)
	assert.Empty(t, *jun, "all or nothing")
}

func TestSnoozeRecurringPayment(t *testing.T) {
	testDB, userID := setupRecurringTestDB(t)
	ctx := context.Background()
	p := newInternetPayment(userID, day(2026, 12, 1))
	require.NoError(t, testDB.CreateRecurringPayment(ctx, p))

	until := time.Date(2026, 11, 20, 0, 0, 0, 0, time.UTC)
	require.NoError(t, testDB.SnoozeRecurringPayment(ctx, p.ID, userID, until))
	got, err := testDB.GetRecurringPaymentByID(ctx, p.ID, userID)
	require.NoError(t, err)
	require.NotNil(t, got.SnoozedUntil)
	assert.Equal(t, "2026-11-20", got.SnoozedUntil.Format("2006-01-02"))

	require.NoError(t, testDB.SnoozeRecurringPayment(ctx, p.ID, userID, until.AddDate(0, 0, -5)))
	got, _ = testDB.GetRecurringPaymentByID(ctx, p.ID, userID)
	assert.Equal(t, "2026-11-20", got.SnoozedUntil.Format("2006-01-02"), "a shorter snooze never cuts a longer one")

	assert.Error(t, testDB.SnoozeRecurringPayment(ctx, p.ID, uuid.New(), until), "someone else's bill")

	byID, err := testDB.GetRecurringPaymentByIDForSnooze(ctx, p.ID)
	require.NoError(t, err)
	assert.Equal(t, userID, byID.UserID, "carries its owner")
	_, err = testDB.GetRecurringPaymentByIDForSnooze(ctx, uuid.New())
	assert.Error(t, err)
}
