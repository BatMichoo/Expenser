package services

import (
	"context"
	"errors"
	"expenser/internal/models"
	"fmt"
	"time"

	"github.com/google/uuid"
)

// RecurringPaymentStore is the data access RecurringPaymentService needs.
// *database.DB implements it.
type RecurringPaymentStore interface {
	CreateRecurringPayment(ctx context.Context, p *models.RecurringPayment) error
	GetRecurringPaymentByID(ctx context.Context, id, userID uuid.UUID) (*models.RecurringPayment, error)
	ListRecurringPayments(ctx context.Context, userID uuid.UUID) ([]models.RecurringPayment, error)
	UpdateRecurringPayment(ctx context.Context, p *models.RecurringPayment, userID uuid.UUID) error
	DeleteRecurringPayment(ctx context.Context, id, userID uuid.UUID) (bool, error)
	MarkRecurringPaymentPaid(ctx context.Context, userID, id uuid.UUID, amount float64, paidOn time.Time) (*models.PaidRecurringPayment, error)
	GetUpcomingRecurringPayments(ctx context.Context, userID uuid.UUID, until time.Time) ([]models.RecurringPayment, error)
}

var (
	ErrAmountRequired     = errors.New("amount is required because the payment has no expected amount")
	ErrInvalidPaidAmount  = errors.New("paid amount cannot be negative")
	ErrInvalidUpcomingDay = errors.New("days must not be negative")
)

// RecurringPaymentService is the entry point for managing recurring payments
// on behalf of a user. Every method is scoped to userID.
type RecurringPaymentService struct {
	store RecurringPaymentStore
	now   func() time.Time
}

func NewRecurringPaymentService(store RecurringPaymentStore) *RecurringPaymentService {
	return &RecurringPaymentService{store: store, now: time.Now}
}

// Create stores a new payment owned by userID. Nil PaymentOptions get the
// defaults (cash).
func (s *RecurringPaymentService) Create(ctx context.Context, userID uuid.UUID, p *models.RecurringPayment) error {
	p.UserID = userID
	return s.store.CreateRecurringPayment(ctx, p)
}

func (s *RecurringPaymentService) Get(ctx context.Context, userID, id uuid.UUID) (*models.RecurringPayment, error) {
	return s.store.GetRecurringPaymentByID(ctx, id, userID)
}

func (s *RecurringPaymentService) List(ctx context.Context, userID uuid.UUID) ([]models.RecurringPayment, error) {
	return s.store.ListRecurringPayments(ctx, userID)
}

// Update overwrites the user-editable fields of an existing payment.
func (s *RecurringPaymentService) Update(ctx context.Context, userID uuid.UUID, p *models.RecurringPayment) error {
	p.UserID = userID
	return s.store.UpdateRecurringPayment(ctx, p, userID)
}

// Delete reports whether a payment was deleted.
func (s *RecurringPaymentService) Delete(ctx context.Context, userID, id uuid.UUID) (bool, error) {
	return s.store.DeleteRecurringPayment(ctx, id, userID)
}

// MarkPaid records the current due date as paid: it creates the matching
// house or car expense and moves the payment to its next due date. A nil
// amount uses the payment's expected amount; a zero paidOn means now.
func (s *RecurringPaymentService) MarkPaid(ctx context.Context, userID, id uuid.UUID, amount *float64, paidOn time.Time) (*models.PaidRecurringPayment, error) {
	if amount == nil {
		p, err := s.store.GetRecurringPaymentByID(ctx, id, userID)
		if err != nil {
			return nil, err
		}
		if p.ExpectedAmount == nil {
			return nil, ErrAmountRequired
		}
		amount = p.ExpectedAmount
	}
	if *amount < 0 {
		return nil, ErrInvalidPaidAmount
	}
	if paidOn.IsZero() {
		paidOn = s.now()
	}

	paid, err := s.store.MarkRecurringPaymentPaid(ctx, userID, id, *amount, paidOn)
	if err != nil {
		return nil, fmt.Errorf("mark paid: %w", err)
	}
	return paid, nil
}

// Upcoming returns the user's unpaid payments due within the next days
// days (0 = due today), overdue ones first.
func (s *RecurringPaymentService) Upcoming(ctx context.Context, userID uuid.UUID, days int) ([]models.UpcomingPayment, error) {
	if days < 0 {
		return nil, ErrInvalidUpcomingDay
	}
	today := models.DateOnly(s.now())
	payments, err := s.store.GetUpcomingRecurringPayments(ctx, userID, today.AddDate(0, 0, days))
	if err != nil {
		return nil, err
	}

	upcoming := make([]models.UpcomingPayment, 0, len(payments))
	for _, p := range payments {
		left := p.DaysUntilDue(today)
		upcoming = append(upcoming, models.UpcomingPayment{Payment: p, DaysLeft: left, Overdue: left < 0})
	}
	return upcoming, nil
}
