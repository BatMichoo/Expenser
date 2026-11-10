package database

import (
	"context"
	"database/sql"
	"expenser/internal/config"
	"expenser/internal/models"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func rpDate(y int, m time.Month, d int) time.Time {
	return time.Date(y, m, d, 0, 0, 0, 0, time.UTC)
}

func rpIntPtr(i int) *int { return &i }

func setupRecurringTestDB(t *testing.T) (*DB, uuid.UUID) {
	t.Helper()
	cfg, err := config.LoadConfig()
	require.NoError(t, err)

	testDB := InitTestDB(cfg)
	t.Cleanup(func() { testDB.Close() })
	ResetTestDB(testDB)

	user := &models.User{Username: "RecurringTester", PasswordHash: TestUserRegisterModel.PasswordHash}
	require.NoError(t, testDB.CreateUser(user))
	return testDB, user.ID
}

func newInternetPayment(userID uuid.UUID, due time.Time) *models.RecurringPayment {
	return &models.RecurringPayment{
		UserID:           userID,
		Category:         models.RecurringCategoryHouse,
		UtilityTypeID:    rpIntPtr(4), // Internet
		Name:             "Fiber",
		DueDate:          due,
		CycleUnit:        models.CycleMonth,
		CycleEvery:       1,
		RemindDaysBefore: 3,
		PaymentHours:     "Mon-Fri 09:00-18:00",
		Notes:            "Contract 42",
	}
}

func TestCreateAndGetRecurringPayment(t *testing.T) {
	testDB, userID := setupRecurringTestDB(t)
	ctx := context.Background()

	p := newInternetPayment(userID, rpDate(2026, 12, 1))
	require.NoError(t, testDB.CreateRecurringPayment(ctx, p))
	assert.NotEqual(t, uuid.Nil, p.ID)
	assert.True(t, p.Active)

	got, err := testDB.GetRecurringPaymentByID(ctx, p.ID, userID)
	require.NoError(t, err)
	assert.Equal(t, "Internet", got.TypeName)
	assert.Equal(t, "RecurringTester", got.Username)
	assert.Equal(t, models.RecurringCategoryHouse, got.Category)
	assert.Equal(t, 4, *got.UtilityTypeID)
	assert.Nil(t, got.CarExpenseTypeID)
	assert.Equal(t, rpDate(2026, 12, 1), got.DueDate)
	assert.Equal(t, rpDate(2026, 12, 1), got.StartDate, "start date defaults to due date")
	assert.Equal(t, []string{"cash"}, got.PaymentOptions, "cash is pre-filled")
	assert.Equal(t, "Mon-Fri 09:00-18:00", got.PaymentHours)
	assert.Equal(t, "Contract 42", got.Notes)
	assert.Nil(t, got.LastNotifiedFor)
}

func TestCreateRecurringPaymentOptions(t *testing.T) {
	testDB, userID := setupRecurringTestDB(t)
	ctx := context.Background()

	withoutCash := newInternetPayment(userID, rpDate(2026, 12, 1))
	withoutCash.PaymentOptions = []string{"card", "ePay"}
	require.NoError(t, testDB.CreateRecurringPayment(ctx, withoutCash))

	none := newInternetPayment(userID, rpDate(2026, 12, 1))
	none.PaymentOptions = []string{}
	require.NoError(t, testDB.CreateRecurringPayment(ctx, none))

	got, err := testDB.GetRecurringPaymentByID(ctx, withoutCash.ID, userID)
	require.NoError(t, err)
	assert.Equal(t, []string{"card", "ePay"}, got.PaymentOptions, "cash can be removed")

	got, err = testDB.GetRecurringPaymentByID(ctx, none.ID, userID)
	require.NoError(t, err)
	assert.Equal(t, []string{}, got.PaymentOptions)
}

func TestCreateCarRecurringPayment(t *testing.T) {
	testDB, userID := setupRecurringTestDB(t)
	ctx := context.Background()

	p := &models.RecurringPayment{
		UserID:           userID,
		Category:         models.RecurringCategoryCar,
		CarExpenseTypeID: rpIntPtr(3), // Insurance
		DueDate:          rpDate(2027, 3, 1),
		CycleUnit:        models.CycleYear,
		CycleEvery:       1,
		RemindDaysBefore: 14,
	}
	require.NoError(t, testDB.CreateRecurringPayment(ctx, p))

	got, err := testDB.GetRecurringPaymentByID(ctx, p.ID, userID)
	require.NoError(t, err)
	assert.Equal(t, "Insurance", got.TypeName)
	assert.Nil(t, got.UtilityTypeID)
}

func TestCreateRecurringPaymentRejectsInvalid(t *testing.T) {
	testDB, userID := setupRecurringTestDB(t)
	ctx := context.Background()

	mismatch := newInternetPayment(userID, rpDate(2026, 12, 1))
	mismatch.Category = models.RecurringCategoryCar
	assert.ErrorIs(t, testDB.CreateRecurringPayment(ctx, mismatch), models.ErrTypeMismatch)

	unknownType := newInternetPayment(userID, rpDate(2026, 12, 1))
	unknownType.UtilityTypeID = rpIntPtr(9999)
	assert.Error(t, testDB.CreateRecurringPayment(ctx, unknownType), "foreign key rejects unknown type")
}

func TestRecurringPaymentsAreScopedToUser(t *testing.T) {
	testDB, userID := setupRecurringTestDB(t)
	ctx := context.Background()

	other := &models.User{Username: "OtherUser", PasswordHash: TestUserRegisterModel.PasswordHash}
	require.NoError(t, testDB.CreateUser(other))

	p := newInternetPayment(userID, rpDate(2026, 12, 1))
	require.NoError(t, testDB.CreateRecurringPayment(ctx, p))

	_, err := testDB.GetRecurringPaymentByID(ctx, p.ID, other.ID)
	assert.ErrorIs(t, err, sql.ErrNoRows)

	list, err := testDB.ListRecurringPayments(ctx, other.ID)
	require.NoError(t, err)
	assert.Empty(t, list)

	p.Name = "Hijacked"
	assert.ErrorIs(t, testDB.UpdateRecurringPayment(ctx, p, other.ID), sql.ErrNoRows)

	deleted, err := testDB.DeleteRecurringPayment(ctx, p.ID, other.ID)
	require.NoError(t, err)
	assert.False(t, deleted)
}

func TestListUpdateDeleteRecurringPayments(t *testing.T) {
	testDB, userID := setupRecurringTestDB(t)
	ctx := context.Background()

	later := newInternetPayment(userID, rpDate(2027, 1, 1))
	sooner := newInternetPayment(userID, rpDate(2026, 12, 1))
	require.NoError(t, testDB.CreateRecurringPayment(ctx, later))
	require.NoError(t, testDB.CreateRecurringPayment(ctx, sooner))

	list, err := testDB.ListRecurringPayments(ctx, userID)
	require.NoError(t, err)
	require.Len(t, list, 2)
	assert.Equal(t, sooner.ID, list[0].ID, "soonest due first")

	sooner.UtilityTypeID = rpIntPtr(5) // TV
	sooner.CycleUnit = models.CycleMonth
	sooner.CycleEvery = 3
	sooner.PaymentOptions = []string{"card"}
	sooner.Active = false
	require.NoError(t, testDB.UpdateRecurringPayment(ctx, sooner, userID))

	got, err := testDB.GetRecurringPaymentByID(ctx, sooner.ID, userID)
	require.NoError(t, err)
	assert.Equal(t, "TV", got.TypeName)
	assert.Equal(t, 3, got.CycleEvery)
	assert.Equal(t, []string{"card"}, got.PaymentOptions)
	assert.False(t, got.Active)

	deleted, err := testDB.DeleteRecurringPayment(ctx, later.ID, userID)
	require.NoError(t, err)
	assert.True(t, deleted)

	list, err = testDB.ListRecurringPayments(ctx, userID)
	require.NoError(t, err)
	assert.Len(t, list, 1)
}

func TestReminderQueries(t *testing.T) {
	testDB, userID := setupRecurringTestDB(t)
	ctx := context.Background()
	today := rpDate(2026, 11, 10)

	inWindow := newInternetPayment(userID, rpDate(2026, 11, 12))         // reminder from Nov 9
	windowOpensToday := newInternetPayment(userID, rpDate(2026, 11, 13)) // reminder from Nov 10
	tooEarly := newInternetPayment(userID, rpDate(2026, 11, 20))
	overdue := newInternetPayment(userID, rpDate(2026, 11, 5))
	inactive := newInternetPayment(userID, rpDate(2026, 11, 11))
	for _, p := range []*models.RecurringPayment{inWindow, windowOpensToday, tooEarly, overdue, inactive} {
		require.NoError(t, testDB.CreateRecurringPayment(ctx, p))
	}
	inactive.Active = false
	require.NoError(t, testDB.UpdateRecurringPayment(ctx, inactive, userID))

	ids := func(ps []models.RecurringPayment) []uuid.UUID {
		out := []uuid.UUID{}
		for _, p := range ps {
			out = append(out, p.ID)
		}
		return out
	}

	toRemind, err := testDB.GetRecurringPaymentsToRemind(ctx, today)
	require.NoError(t, err)
	assert.Equal(t, []uuid.UUID{inWindow.ID, windowOpensToday.ID}, ids(toRemind))

	overdueList, err := testDB.GetOverdueRecurringPayments(ctx, today)
	require.NoError(t, err)
	assert.Equal(t, []uuid.UUID{overdue.ID}, ids(overdueList))

	// Once notified for its due date, a payment is not returned again.
	require.NoError(t, testDB.MarkRecurringPaymentNotified(ctx, inWindow.ID, inWindow.DueDate))
	toRemind, err = testDB.GetRecurringPaymentsToRemind(ctx, today)
	require.NoError(t, err)
	assert.Equal(t, []uuid.UUID{windowOpensToday.ID}, ids(toRemind))

	// Moving to the next cycle makes it eligible again once its window opens.
	require.NoError(t, testDB.UpdateRecurringPaymentDueDate(ctx, inWindow.ID, rpDate(2026, 12, 12)))
	got, err := testDB.GetRecurringPaymentByID(ctx, inWindow.ID, userID)
	require.NoError(t, err)
	assert.Equal(t, rpDate(2026, 12, 12), got.DueDate)
	assert.Equal(t, rpDate(2026, 11, 12), *got.LastNotifiedFor)

	toRemind, err = testDB.GetRecurringPaymentsToRemind(ctx, rpDate(2026, 12, 10))
	require.NoError(t, err)
	assert.Contains(t, ids(toRemind), inWindow.ID)
}
