package services

import (
	"context"
	"database/sql"
	"expenser/internal/models"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// fakePaymentStore records calls; only the methods the tests exercise do
// real work.
type fakePaymentStore struct {
	payment  *models.RecurringPayment
	upcoming []models.RecurringPayment

	createdFor  uuid.UUID
	paidAmount  float64
	paidOn      time.Time
	upcomingTil time.Time
}

func (f *fakePaymentStore) CreateRecurringPayment(_ context.Context, p *models.RecurringPayment) error {
	f.createdFor = p.UserID
	return nil
}

func (f *fakePaymentStore) GetRecurringPaymentByID(_ context.Context, id, userID uuid.UUID) (*models.RecurringPayment, error) {
	if f.payment == nil || f.payment.ID != id || f.payment.UserID != userID {
		return nil, sql.ErrNoRows
	}
	cp := *f.payment
	return &cp, nil
}

func (f *fakePaymentStore) ListRecurringPayments(context.Context, uuid.UUID) ([]models.RecurringPayment, error) {
	return nil, nil
}

func (f *fakePaymentStore) UpdateRecurringPayment(context.Context, *models.RecurringPayment, uuid.UUID) error {
	return nil
}

func (f *fakePaymentStore) DeleteRecurringPayment(context.Context, uuid.UUID, uuid.UUID) (bool, error) {
	return true, nil
}

func (f *fakePaymentStore) MarkRecurringPaymentPaid(_ context.Context, _, _ uuid.UUID, amount float64, paidOn time.Time) (*models.PaidRecurringPayment, error) {
	f.paidAmount, f.paidOn = amount, paidOn
	return &models.PaidRecurringPayment{Payment: f.payment, ExpenseID: 1}, nil
}

func (f *fakePaymentStore) GetUpcomingRecurringPayments(_ context.Context, _ uuid.UUID, until time.Time) ([]models.RecurringPayment, error) {
	f.upcomingTil = until
	return f.upcoming, nil
}

func newTestPaymentService(store RecurringPaymentStore, today time.Time) *RecurringPaymentService {
	s := NewRecurringPaymentService(store)
	s.now = func() time.Time { return today }
	return s
}

func TestCreateSetsOwner(t *testing.T) {
	store := &fakePaymentStore{}
	userID := uuid.New()
	p := &models.RecurringPayment{UserID: uuid.New()}
	require.NoError(t, NewRecurringPaymentService(store).Create(context.Background(), userID, p))
	assert.Equal(t, userID, store.createdFor, "owner comes from the caller, not the input")
}

func TestMarkPaidAmounts(t *testing.T) {
	userID := uuid.New()
	expected := 30.0
	payment := &models.RecurringPayment{ID: uuid.New(), UserID: userID, ExpectedAmount: &expected}
	today := time.Date(2026, 11, 10, 15, 0, 0, 0, time.UTC)
	ctx := context.Background()

	t.Run("uses expected amount and now by default", func(t *testing.T) {
		store := &fakePaymentStore{payment: payment}
		_, err := newTestPaymentService(store, today).MarkPaid(ctx, userID, payment.ID, nil, time.Time{})
		require.NoError(t, err)
		assert.Equal(t, 30.0, store.paidAmount)
		assert.Equal(t, today, store.paidOn)
	})

	t.Run("explicit amount and date win", func(t *testing.T) {
		store := &fakePaymentStore{payment: payment}
		amount := 35.5
		paidOn := time.Date(2026, 11, 8, 0, 0, 0, 0, time.UTC)
		_, err := newTestPaymentService(store, today).MarkPaid(ctx, userID, payment.ID, &amount, paidOn)
		require.NoError(t, err)
		assert.Equal(t, 35.5, store.paidAmount)
		assert.Equal(t, paidOn, store.paidOn)
	})

	t.Run("no amount anywhere", func(t *testing.T) {
		noExpected := &models.RecurringPayment{ID: uuid.New(), UserID: userID}
		_, err := newTestPaymentService(&fakePaymentStore{payment: noExpected}, today).
			MarkPaid(ctx, userID, noExpected.ID, nil, time.Time{})
		assert.ErrorIs(t, err, ErrAmountRequired)
	})

	t.Run("non-positive amount", func(t *testing.T) {
		zero := 0.0
		_, err := newTestPaymentService(&fakePaymentStore{payment: payment}, today).
			MarkPaid(ctx, userID, payment.ID, &zero, time.Time{})
		assert.ErrorIs(t, err, ErrInvalidPaidAmount)
	})

	t.Run("other user's payment", func(t *testing.T) {
		_, err := newTestPaymentService(&fakePaymentStore{payment: payment}, today).
			MarkPaid(ctx, uuid.New(), payment.ID, nil, time.Time{})
		assert.ErrorIs(t, err, sql.ErrNoRows)
	})
}

func TestUpcoming(t *testing.T) {
	today := time.Date(2026, 11, 10, 18, 30, 0, 0, time.UTC)
	store := &fakePaymentStore{upcoming: []models.RecurringPayment{
		{DueDate: day(2026, 11, 7)},
		{DueDate: day(2026, 11, 10)},
		{DueDate: day(2026, 11, 15)},
	}}
	svc := newTestPaymentService(store, today)

	got, err := svc.Upcoming(context.Background(), uuid.New(), 7)
	require.NoError(t, err)
	assert.Equal(t, day(2026, 11, 17), store.upcomingTil)
	require.Len(t, got, 3)
	assert.Equal(t, -3, got[0].DaysLeft)
	assert.True(t, got[0].Overdue)
	assert.Equal(t, 0, got[1].DaysLeft)
	assert.False(t, got[1].Overdue)
	assert.Equal(t, 5, got[2].DaysLeft)

	_, err = svc.Upcoming(context.Background(), uuid.New(), -1)
	assert.ErrorIs(t, err, ErrInvalidUpcomingDay)
}
