package services

import (
	"context"
	"errors"
	"fmt"
	"time"

	"expenser/internal/models"

	"github.com/google/uuid"
)

// MaxBackfillMonths caps one backfill: enough to catch up on years of
// payments, small enough that a typo can't create hundreds.
const MaxBackfillMonths = 60

var (
	ErrBackfillRange   = errors.New("the months or the day are not valid")
	ErrBackfillFuture  = errors.New("backfill can't go past the current month")
	ErrBackfillTooLong = errors.New("backfill covers at most 60 months")
)

// BackfillRequest asks for Expense on Day of every month from From to To.
type BackfillRequest struct {
	From, To YearMonth
	Day      int // 1–31; short months use their last day
	Expense  models.BackfillExpense
}

// BackfillPlan is what a backfill adds and skips.
type BackfillPlan struct {
	Request BackfillRequest
	Dates   []time.Time // one per month that gets a payment
	Skipped []YearMonth // months that already have one of that type
	Months  int         // months in the range
}

// PlanBackfill works out the dates to add. existing holds the months
// ("YYYY-MM") that already have an expense of that type.
func PlanBackfill(req BackfillRequest, existing map[string]bool, now time.Time) (*BackfillPlan, error) {
	if req.Day < 1 || req.Day > 31 || req.To.Start().Before(req.From.Start()) {
		return nil, ErrBackfillRange
	}
	if req.To.Start().After(YearMonth{now.Year(), now.Month()}.Start()) {
		return nil, ErrBackfillFuture
	}
	plan := &BackfillPlan{Request: req}
	for m := req.From; !m.Start().After(req.To.Start()); m = m.Next() {
		plan.Months++
		if plan.Months > MaxBackfillMonths {
			return nil, ErrBackfillTooLong
		}
		if existing[m.String()] {
			plan.Skipped = append(plan.Skipped, m)
			continue
		}
		day := req.Day
		if last := m.End().AddDate(0, 0, -1).Day(); day > last {
			day = last
		}
		plan.Dates = append(plan.Dates, models.Date(m.Year, m.Month, day))
	}
	return plan, nil
}

// BackfillStore is the data access a backfill needs. *database.DB
// implements it.
type BackfillStore interface {
	ExpenseMonthsOfType(ctx context.Context, userID uuid.UUID, section models.Section, typeID int, from, to time.Time) (map[string]bool, error)
	CreateBackfillExpenses(ctx context.Context, e models.BackfillExpense, dates []time.Time) error
}

// BackfillService previews and applies backfills.
type BackfillService struct {
	store BackfillStore
}

func NewBackfillService(store BackfillStore) *BackfillService {
	return &BackfillService{store: store}
}

// Preview plans the backfill against what is recorded now.
func (s *BackfillService) Preview(ctx context.Context, req BackfillRequest, now time.Time) (*BackfillPlan, error) {
	if _, err := PlanBackfill(req, nil, now); err != nil { // validate before querying
		return nil, err
	}
	existing, err := s.store.ExpenseMonthsOfType(ctx, req.Expense.UserID, req.Expense.Section, req.Expense.TypeID, req.From.Start(), req.To.End())
	if err != nil {
		return nil, fmt.Errorf("backfill: %w", err)
	}
	return PlanBackfill(req, existing, now)
}

// Apply plans again (anything added since the preview is skipped) and
// creates the payments in one go.
func (s *BackfillService) Apply(ctx context.Context, req BackfillRequest, now time.Time) (*BackfillPlan, error) {
	plan, err := s.Preview(ctx, req, now)
	if err != nil {
		return nil, err
	}
	if len(plan.Dates) > 0 {
		if err := s.store.CreateBackfillExpenses(ctx, req.Expense, plan.Dates); err != nil {
			return nil, fmt.Errorf("backfill: %w", err)
		}
	}
	return plan, nil
}
