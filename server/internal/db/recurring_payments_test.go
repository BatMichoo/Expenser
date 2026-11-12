package database

import (
	"context"
	"database/sql"
	"expenser/internal/config"
	"expenser/internal/models"
	"expenser/internal/services"
	"strings"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// *DB must satisfy the stores the services depend on.
var (
	_ services.ReminderStore         = (*DB)(nil)
	_ services.RecurringPaymentStore = (*DB)(nil)
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
	require.NoError(t, testDB.CreateUser(context.Background(), user))
	return testDB, user.ID
}

func newInternetPayment(userID uuid.UUID, due time.Time) *models.RecurringPayment {
	return &models.RecurringPayment{
		UserID:           userID,
		Category:         models.SectionHouse,
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
	assert.Equal(t, models.SectionHouse, got.Category)
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
		Category:         models.SectionCar,
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
	mismatch.Category = models.SectionCar
	assert.ErrorIs(t, testDB.CreateRecurringPayment(ctx, mismatch), models.ErrTypeMismatch)

	unknownType := newInternetPayment(userID, rpDate(2026, 12, 1))
	unknownType.UtilityTypeID = rpIntPtr(9999)
	assert.Error(t, testDB.CreateRecurringPayment(ctx, unknownType), "foreign key rejects unknown type")
}

func TestRecurringPaymentsAreScopedToUser(t *testing.T) {
	testDB, userID := setupRecurringTestDB(t)
	ctx := context.Background()

	other := &models.User{Username: "OtherUser", PasswordHash: TestUserRegisterModel.PasswordHash}
	require.NoError(t, testDB.CreateUser(context.Background(), other))

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

func ids(ps []models.RecurringPayment) []uuid.UUID {
	out := []uuid.UUID{}
	for _, p := range ps {
		out = append(out, p.ID)
	}
	return out
}

func TestGetActiveRecurringPayments(t *testing.T) {
	testDB, userID := setupRecurringTestDB(t)
	ctx := context.Background()

	active := newInternetPayment(userID, rpDate(2026, 11, 12))
	paused := newInternetPayment(userID, rpDate(2026, 11, 11))
	for _, p := range []*models.RecurringPayment{active, paused} {
		require.NoError(t, testDB.CreateRecurringPayment(ctx, p))
	}
	paused.Active = false
	require.NoError(t, testDB.UpdateRecurringPayment(ctx, paused, userID))

	got, err := testDB.GetActiveRecurringPayments(ctx)
	require.NoError(t, err)
	assert.Equal(t, []uuid.UUID{active.ID}, ids(got), "the scheduler decides who gets reminded")
	assert.Equal(t, "RecurringTester", got[0].Username)
}

func TestReminderScheduleRoundTrip(t *testing.T) {
	testDB, userID := setupRecurringTestDB(t)
	ctx := context.Background()

	p := newInternetPayment(userID, rpDate(2026, 11, 8))
	p.DateKind = models.DateArrives
	p.RemindDaysAfter = 1
	every, max := 3, 5
	p.RepeatEveryDays, p.MaxReminders = &every, &max
	snooze := rpDate(2026, 11, 15)
	p.SnoozedUntil = &snooze
	require.NoError(t, testDB.CreateRecurringPayment(ctx, p))

	got, err := testDB.GetRecurringPaymentByID(ctx, p.ID, userID)
	require.NoError(t, err)
	assert.Equal(t, models.DateArrives, got.DateKind)
	assert.Equal(t, 1, got.RemindDaysAfter)
	require.NotNil(t, got.RepeatEveryDays)
	assert.Equal(t, 3, *got.RepeatEveryDays)
	require.NotNil(t, got.MaxReminders)
	assert.Equal(t, 5, *got.MaxReminders)
	require.NotNil(t, got.SnoozedUntil)
	assert.Equal(t, rpDate(2026, 11, 15), *got.SnoozedUntil)

	got.RepeatEveryDays, got.MaxReminders, got.SnoozedUntil = nil, nil, nil
	got.DateKind = models.DatePayBy
	require.NoError(t, testDB.UpdateRecurringPayment(ctx, got, userID))
	got, err = testDB.GetRecurringPaymentByID(ctx, p.ID, userID)
	require.NoError(t, err)
	assert.Nil(t, got.RepeatEveryDays, "remind once")
	assert.Nil(t, got.MaxReminders)
	assert.Nil(t, got.SnoozedUntil)
	assert.Equal(t, models.DatePayBy, got.DateKind)
}

func TestMarkNotifiedCountsPerCycle(t *testing.T) {
	testDB, userID := setupRecurringTestDB(t)
	ctx := context.Background()
	p := newInternetPayment(userID, rpDate(2026, 11, 20))
	require.NoError(t, testDB.CreateRecurringPayment(ctx, p))

	require.NoError(t, testDB.MarkRecurringPaymentNotified(ctx, p.ID, rpDate(2026, 11, 20), rpDate(2026, 11, 17)))
	require.NoError(t, testDB.MarkRecurringPaymentNotified(ctx, p.ID, rpDate(2026, 11, 20), rpDate(2026, 11, 20)))
	got, err := testDB.GetRecurringPaymentByID(ctx, p.ID, userID)
	require.NoError(t, err)
	assert.Equal(t, 2, got.RemindersSent)
	assert.Equal(t, rpDate(2026, 11, 20), *got.LastNotifiedOn)

	require.NoError(t, testDB.MarkRecurringPaymentNotified(ctx, p.ID, rpDate(2026, 12, 20), rpDate(2026, 12, 17)))
	got, err = testDB.GetRecurringPaymentByID(ctx, p.ID, userID)
	require.NoError(t, err)
	assert.Equal(t, 1, got.RemindersSent, "a new due date starts counting again")
}

func TestMarkPaidClearsSnooze(t *testing.T) {
	testDB, userID := setupRecurringTestDB(t)
	ctx := context.Background()
	p := newInternetPayment(userID, rpDate(2026, 11, 20))
	snooze := rpDate(2026, 11, 25)
	p.SnoozedUntil = &snooze
	require.NoError(t, testDB.CreateRecurringPayment(ctx, p))

	res, err := testDB.MarkRecurringPaymentPaid(ctx, userID, p.ID, 30, rpDate(2026, 11, 21))
	require.NoError(t, err)
	assert.Nil(t, res.Payment.SnoozedUntil)
	got, err := testDB.GetRecurringPaymentByID(ctx, p.ID, userID)
	require.NoError(t, err)
	assert.Nil(t, got.SnoozedUntil, "a snooze only covers the cycle it was set in")
}

func TestRecurringPaymentAmounts(t *testing.T) {
	testDB, userID := setupRecurringTestDB(t)
	ctx := context.Background()

	amount := 29.99
	p := newInternetPayment(userID, rpDate(2026, 12, 1))
	p.ExpectedAmount = &amount
	p.RememberLastAmount = true
	require.NoError(t, testDB.CreateRecurringPayment(ctx, p))

	got, err := testDB.GetRecurringPaymentByID(ctx, p.ID, userID)
	require.NoError(t, err)
	require.NotNil(t, got.ExpectedAmount)
	assert.Equal(t, 29.99, *got.ExpectedAmount)
	assert.True(t, got.RememberLastAmount)

	got.ExpectedAmount = nil
	got.RememberLastAmount = false
	require.NoError(t, testDB.UpdateRecurringPayment(ctx, got, userID))
	got, err = testDB.GetRecurringPaymentByID(ctx, p.ID, userID)
	require.NoError(t, err)
	assert.Nil(t, got.ExpectedAmount)
	assert.False(t, got.RememberLastAmount)

	negative := -5.0
	bad := newInternetPayment(userID, rpDate(2026, 12, 1))
	bad.ExpectedAmount = &negative
	assert.ErrorIs(t, testDB.CreateRecurringPayment(ctx, bad), models.ErrInvalidAmount)
}

func TestMarkRecurringPaymentPaidHouse(t *testing.T) {
	testDB, userID := setupRecurringTestDB(t)
	ctx := context.Background()

	expected := 30.0
	p := newInternetPayment(userID, rpDate(2026, 1, 31))
	p.ExpectedAmount = &expected
	p.RememberLastAmount = true
	require.NoError(t, testDB.CreateRecurringPayment(ctx, p))

	paidOn := time.Date(2026, 2, 2, 10, 0, 0, 0, time.Local)
	res, err := testDB.MarkRecurringPaymentPaid(ctx, userID, p.ID, 32.5, paidOn)
	require.NoError(t, err)
	assert.Equal(t, rpDate(2026, 1, 31), res.PaidDue)
	assert.Equal(t, rpDate(2026, 2, 28), res.Payment.DueDate)

	expense, err := testDB.GetHouseExpenseByID(context.Background(), res.ExpenseID, userID)
	require.NoError(t, err)
	assert.Equal(t, 32.5, expense.Amount)
	assert.Equal(t, "Internet", expense.UtilityType)
	assert.Equal(t, "Fiber", expense.Notes)
	assert.Equal(t, paidOn.Round(time.Second), expense.ExpenseDate.Local().Round(time.Second))

	got, err := testDB.GetRecurringPaymentByID(ctx, p.ID, userID)
	require.NoError(t, err)
	assert.Equal(t, rpDate(2026, 2, 28), got.DueDate)
	assert.Equal(t, rpDate(2026, 2, 2), *got.LastPaidOn)
	assert.Equal(t, 32.5, *got.ExpectedAmount, "remembered last amount")

	// Paying again moves on without drifting from the 31st.
	res, err = testDB.MarkRecurringPaymentPaid(ctx, userID, p.ID, 32.5, paidOn)
	require.NoError(t, err)
	assert.Equal(t, rpDate(2026, 3, 31), res.Payment.DueDate)
}

func TestMarkRecurringPaymentPaidCar(t *testing.T) {
	testDB, userID := setupRecurringTestDB(t)
	ctx := context.Background()

	expected := 400.0
	p := &models.RecurringPayment{
		UserID:           userID,
		Category:         models.SectionCar,
		CarExpenseTypeID: rpIntPtr(3), // Insurance
		DueDate:          rpDate(2026, 11, 20),
		CycleUnit:        models.CycleMonth,
		CycleEvery:       6,
		ExpectedAmount:   &expected,
	}
	require.NoError(t, testDB.CreateRecurringPayment(ctx, p))

	res, err := testDB.MarkRecurringPaymentPaid(ctx, userID, p.ID, 420, rpDate(2026, 11, 18))
	require.NoError(t, err)
	assert.Equal(t, rpDate(2027, 5, 20), res.Payment.DueDate)
	assert.Equal(t, 400.0, *res.Payment.ExpectedAmount, "amount not remembered when flag is off")

	expense, err := testDB.GetCarExpenseByID(context.Background(), res.ExpenseID, userID)
	require.NoError(t, err)
	assert.Equal(t, 420.0, expense.Amount)
	assert.Equal(t, "Insurance", expense.Type)
	assert.Equal(t, "Recurring payment", expense.Notes, "fallback note when unnamed")
}

func TestMarkRecurringPaymentPaidZeroAmount(t *testing.T) {
	testDB, userID := setupRecurringTestDB(t)
	ctx := context.Background()

	p := newInternetPayment(userID, rpDate(2026, 12, 1))
	require.NoError(t, testDB.CreateRecurringPayment(ctx, p))

	res, err := testDB.MarkRecurringPaymentPaid(ctx, userID, p.ID, 0, rpDate(2026, 12, 1))
	require.NoError(t, err, "a free month still records an expense")

	expense, err := testDB.GetHouseExpenseByID(context.Background(), res.ExpenseID, userID)
	require.NoError(t, err)
	assert.Equal(t, 0.0, expense.Amount)
}

func TestMarkRecurringPaymentPaidRollsBackOnFailure(t *testing.T) {
	testDB, userID := setupRecurringTestDB(t)
	ctx := context.Background()

	p := newInternetPayment(userID, rpDate(2026, 12, 1))
	p.RememberLastAmount = true
	require.NoError(t, testDB.CreateRecurringPayment(ctx, p))

	// Exceeds NUMERIC(10, 2), so the expense insert fails inside the transaction.
	_, err := testDB.MarkRecurringPaymentPaid(ctx, userID, p.ID, 1e10, rpDate(2026, 12, 1))
	require.Error(t, err)

	got, err := testDB.GetRecurringPaymentByID(ctx, p.ID, userID)
	require.NoError(t, err)
	assert.Equal(t, rpDate(2026, 12, 1), got.DueDate, "due date not moved")
	assert.Nil(t, got.LastPaidOn)
	assert.Nil(t, got.ExpectedAmount)

	expenses, err := testDB.GetHouseExpensesForMonth(context.Background(), time.December, 2026, userID)
	require.NoError(t, err)
	assert.Empty(t, *expenses, "no expense left behind")
}

func TestRecurringPaymentInputNormalized(t *testing.T) {
	testDB, userID := setupRecurringTestDB(t)
	ctx := context.Background()

	p := newInternetPayment(userID, rpDate(2026, 12, 1))
	p.Name = "  Fiber  "
	p.PaymentOptions = []string{" cash ", "", "Cash", "card"}
	require.NoError(t, testDB.CreateRecurringPayment(ctx, p))

	got, err := testDB.GetRecurringPaymentByID(ctx, p.ID, userID)
	require.NoError(t, err)
	assert.Equal(t, "Fiber", got.Name)
	assert.Equal(t, []string{"cash", "card"}, got.PaymentOptions)

	long := newInternetPayment(userID, rpDate(2026, 12, 1))
	long.Name = strings.Repeat("x", 101)
	assert.ErrorIs(t, testDB.CreateRecurringPayment(ctx, long), models.ErrNameTooLong)
}

func TestMarkRecurringPaymentPaidOtherUser(t *testing.T) {
	testDB, userID := setupRecurringTestDB(t)
	ctx := context.Background()

	other := &models.User{Username: "OtherUser", PasswordHash: TestUserRegisterModel.PasswordHash}
	require.NoError(t, testDB.CreateUser(context.Background(), other))

	p := newInternetPayment(userID, rpDate(2026, 12, 1))
	require.NoError(t, testDB.CreateRecurringPayment(ctx, p))

	_, err := testDB.MarkRecurringPaymentPaid(ctx, other.ID, p.ID, 10, rpDate(2026, 12, 1))
	assert.ErrorIs(t, err, sql.ErrNoRows)

	got, err := testDB.GetRecurringPaymentByID(ctx, p.ID, userID)
	require.NoError(t, err)
	assert.Equal(t, rpDate(2026, 12, 1), got.DueDate, "unchanged")
	assert.Nil(t, got.LastPaidOn)
}

func TestGetUpcomingRecurringPayments(t *testing.T) {
	testDB, userID := setupRecurringTestDB(t)
	ctx := context.Background()

	other := &models.User{Username: "OtherUser", PasswordHash: TestUserRegisterModel.PasswordHash}
	require.NoError(t, testDB.CreateUser(context.Background(), other))

	overdue := newInternetPayment(userID, rpDate(2026, 11, 1))
	soon := newInternetPayment(userID, rpDate(2026, 11, 15))
	edge := newInternetPayment(userID, rpDate(2026, 11, 17))
	later := newInternetPayment(userID, rpDate(2026, 11, 18))
	paused := newInternetPayment(userID, rpDate(2026, 11, 12))
	othersPayment := newInternetPayment(other.ID, rpDate(2026, 11, 12))
	for _, p := range []*models.RecurringPayment{overdue, soon, edge, later, paused, othersPayment} {
		require.NoError(t, testDB.CreateRecurringPayment(ctx, p))
	}
	paused.Active = false
	require.NoError(t, testDB.UpdateRecurringPayment(ctx, paused, userID))

	got, err := testDB.GetUpcomingRecurringPayments(ctx, userID, rpDate(2026, 11, 17))
	require.NoError(t, err)
	assert.Equal(t, []uuid.UUID{overdue.ID, soon.ID, edge.ID}, ids(got))
}

func TestUserAccountSettings(t *testing.T) {
	testDB, userID := setupRecurringTestDB(t)
	ctx := context.Background()

	u, err := testDB.GetUserByID(context.Background(), userID)
	require.NoError(t, err)
	assert.Empty(t, u.Email, "not required")
	assert.Empty(t, u.DisplayName, "not required")
	assert.Equal(t, models.DefaultReminderDefaults(), u.ReminderDefaults, "existing users start with today's defaults")

	two, five := 2, 5
	want := models.ReminderDefaults{Notify: models.NotifyNone, RemindDaysBefore: 7, RemindDaysAfter: 1, RepeatEveryDays: &two, MaxReminders: &five}
	require.NoError(t, testDB.UpdateUserAccount(ctx, userID, "Miro", "me@example.com", want))
	u, err = testDB.GetUserByID(context.Background(), userID)
	require.NoError(t, err)
	assert.Equal(t, "Miro", u.DisplayName)
	assert.Equal(t, "me@example.com", u.Email)
	assert.Equal(t, want, u.ReminderDefaults)

	off := models.DefaultReminderDefaults()
	off.RepeatEveryDays = nil
	require.NoError(t, testDB.UpdateUserAccount(ctx, userID, "", "", off))
	u, err = testDB.GetUserByID(context.Background(), userID)
	require.NoError(t, err)
	assert.Empty(t, u.DisplayName, "cleared")
	assert.Empty(t, u.Email, "cleared")
	assert.Nil(t, u.ReminderDefaults.RepeatEveryDays, "repeat off")
	assert.Nil(t, u.ReminderDefaults.MaxReminders, "no limit")

	bad := models.DefaultReminderDefaults()
	bad.Notify = "sms"
	assert.Error(t, testDB.UpdateUserAccount(ctx, userID, "", "", bad), "the database refuses invalid defaults too")
}

func TestUpdateUserPassword(t *testing.T) {
	testDB, userID := setupRecurringTestDB(t)
	require.NoError(t, testDB.UpdateUserPassword(context.Background(), userID, "new-hash"))
	u, err := testDB.GetUserByID(context.Background(), userID)
	require.NoError(t, err)
	assert.Equal(t, "new-hash", u.PasswordHash)

	assert.Error(t, testDB.UpdateUserPassword(context.Background(), uuid.New(), "x"), "unknown user")
}

func setUserEmail(t *testing.T, testDB *DB, userID uuid.UUID, email, name string) {
	t.Helper()
	require.NoError(t, testDB.UpdateUserAccount(context.Background(), userID, name, email, models.DefaultReminderDefaults()))
}

func TestBillDeliveryRoundTrip(t *testing.T) {
	testDB, userID := setupRecurringTestDB(t)
	ctx := context.Background()
	setUserEmail(t, testDB, userID, "me@example.com", "Miro")

	p := newInternetPayment(userID, rpDate(2026, 12, 1))
	p.NotifyEmail = "partner@example.com"
	require.NoError(t, testDB.CreateRecurringPayment(ctx, p))

	got, err := testDB.GetRecurringPaymentByID(ctx, p.ID, userID)
	require.NoError(t, err)
	assert.Equal(t, models.NotifyEmail, got.Notify, "email by default")
	assert.Equal(t, "partner@example.com", got.NotifyEmail)
	assert.Equal(t, "me@example.com", got.UserEmail)
	assert.Equal(t, "Miro", got.UserDisplayName)
	assert.Equal(t, "en", got.UserLang)

	got.Notify, got.NotifyEmail = models.NotifyNone, ""
	require.NoError(t, testDB.UpdateRecurringPayment(ctx, got, userID))
	got, err = testDB.GetRecurringPaymentByID(ctx, p.ID, userID)
	require.NoError(t, err)
	assert.Equal(t, models.NotifyNone, got.Notify)
	assert.Empty(t, got.NotifyEmail)

	active, err := testDB.GetActiveRecurringPayments(ctx)
	require.NoError(t, err)
	require.Len(t, active, 1)
	assert.Equal(t, "me@example.com", active[0].UserEmail, "the scheduler knows where to send")
}
