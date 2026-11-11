package services

import (
	"context"
	"errors"
	"testing"
	"time"

	"expenser/internal/models"

	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func ym(y int, m time.Month) YearMonth { return YearMonth{y, m} }

func TestPlanBackfill(t *testing.T) {
	now := time.Date(2026, 11, 11, 12, 0, 0, 0, time.Local)
	plan, err := PlanBackfill(BackfillRequest{From: ym(2025, 11), To: ym(2026, 3), Day: 31}, map[string]bool{"2026-01": true}, now)
	require.NoError(t, err)

	var dates []string
	for _, d := range plan.Dates {
		dates = append(dates, d.Format("2006-01-02"))
	}
	assert.Equal(t, []string{"2025-11-30", "2025-12-31", "2026-02-28", "2026-03-31"}, dates, "short months get their last day")
	assert.Equal(t, []YearMonth{ym(2026, 1)}, plan.Skipped, "months that already have one")
	assert.Equal(t, 5, plan.Months)
}

func TestPlanBackfillRefusals(t *testing.T) {
	now := time.Date(2026, 11, 11, 12, 0, 0, 0, time.Local)
	for name, tc := range map[string]struct {
		req  BackfillRequest
		want error
	}{
		"to before from":  {BackfillRequest{From: ym(2026, 5), To: ym(2026, 4), Day: 1}, ErrBackfillRange},
		"future month":    {BackfillRequest{From: ym(2026, 10), To: ym(2026, 12), Day: 1}, ErrBackfillFuture},
		"more than 5 yrs": {BackfillRequest{From: ym(2021, 10), To: ym(2026, 10), Day: 1}, ErrBackfillTooLong},
		"bad day":         {BackfillRequest{From: ym(2026, 1), To: ym(2026, 2), Day: 0}, ErrBackfillRange},
	} {
		t.Run(name, func(t *testing.T) {
			_, err := PlanBackfill(tc.req, nil, now)
			assert.ErrorIs(t, err, tc.want)
		})
	}

	plan, err := PlanBackfill(BackfillRequest{From: ym(2021, 11), To: ym(2026, 10), Day: 1}, nil, now)
	require.NoError(t, err, "exactly 60 months is fine")
	assert.Len(t, plan.Dates, 60)
	_, err = PlanBackfill(BackfillRequest{From: ym(2026, 11), To: ym(2026, 11), Day: 1}, nil, now)
	assert.NoError(t, err, "the current month is allowed")
}

func TestPlanBackfillNothingToAdd(t *testing.T) {
	now := time.Date(2026, 11, 11, 12, 0, 0, 0, time.Local)
	plan, err := PlanBackfill(BackfillRequest{From: ym(2026, 1), To: ym(2026, 2), Day: 5}, map[string]bool{"2026-01": true, "2026-02": true}, now)
	require.NoError(t, err)
	assert.Empty(t, plan.Dates)
	assert.Len(t, plan.Skipped, 2)
}

type fakeBackfillStore struct {
	existing map[string]bool
	created  []time.Time
	got      models.BackfillExpense
	err      error
}

func (f *fakeBackfillStore) ExpenseMonthsOfType(_ context.Context, _ uuid.UUID, _ models.Section, _ int, _, _ time.Time) (map[string]bool, error) {
	return f.existing, f.err
}

func (f *fakeBackfillStore) CreateBackfillExpenses(_ context.Context, e models.BackfillExpense, dates []time.Time) error {
	f.got, f.created = e, dates
	return nil
}

func TestBackfillServiceRechecksOnConfirm(t *testing.T) {
	now := time.Date(2026, 11, 11, 12, 0, 0, 0, time.Local)
	store := &fakeBackfillStore{existing: map[string]bool{"2026-02": true}}
	svc := NewBackfillService(store)
	req := BackfillRequest{From: ym(2026, 1), To: ym(2026, 3), Day: 20,
		Expense: models.BackfillExpense{Section: models.SectionHouse, TypeID: 4, Amount: 15, UserID: uuid.New()}}

	plan, err := svc.Apply(context.Background(), req, now)
	require.NoError(t, err)
	assert.Len(t, plan.Dates, 2)
	assert.Equal(t, plan.Dates, store.created)
	assert.Equal(t, 15.0, store.got.Amount)

	store.err = errors.New("db down")
	_, err = svc.Preview(context.Background(), req, now)
	assert.Error(t, err)
}
