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
	ErrAmountRequired = errors.New("amount is required because the payment has no expected amount")
	// ErrStaleUpdate means the payment changed (e.g. was marked paid) after
	// the edit form was opened; saving would undo that change.
	ErrStaleUpdate        = errors.New("recurring payment changed since it was opened")
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
//
// The cycle anchor (StartDate) is managed here, not taken from input: if the
// due date or cycle changed, the new due date becomes the anchor so later
// cycles follow the user's edit; otherwise the stored anchor is kept so
// month-end dates do not drift.
func (s *RecurringPaymentService) Update(ctx context.Context, userID uuid.UUID, p *models.RecurringPayment) error {
	existing, err := s.store.GetRecurringPaymentByID(ctx, p.ID, userID)
	if err != nil {
		return err
	}

	if !p.UpdatedAt.IsZero() && !p.UpdatedAt.Equal(existing.UpdatedAt) {
		return ErrStaleUpdate
	}

	p.UserID = userID
	cycleChanged := !models.DateOnly(p.DueDate).Equal(existing.DueDate) ||
		p.CycleUnit != existing.CycleUnit ||
		p.CycleEvery != existing.CycleEvery
	if cycleChanged {
		p.StartDate = models.DateOnly(p.DueDate)
	} else {
		p.StartDate = existing.StartDate
	}

	// Resuming a paused bill: the cycles it missed while paused were not
	// paid here, so it continues from the next due date on or after today.
	today := models.DateOnly(s.now())
	if !existing.Active && p.Active && models.DateOnly(p.DueDate).Before(today) {
		next, err := p.NextDueOnOrAfter(today)
		if err != nil {
			return err
		}
		p.DueDate = next
	}
	return s.store.UpdateRecurringPayment(ctx, p, userID)
}

// SkipMissed moves an overdue payment to its next due date on or after
// today without recording expenses for the cycles in between.
func (s *RecurringPaymentService) SkipMissed(ctx context.Context, userID, id uuid.UUID) (*models.RecurringPayment, error) {
	p, err := s.store.GetRecurringPaymentByID(ctx, id, userID)
	if err != nil {
		return nil, err
	}
	next, err := p.NextDueOnOrAfter(s.now())
	if err != nil {
		return nil, err
	}
	if !next.After(models.DateOnly(p.DueDate)) {
		return p, nil // not behind
	}
	p.DueDate = next
	if err := s.store.UpdateRecurringPayment(ctx, p, userID); err != nil {
		return nil, err
	}
	return p, nil
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
