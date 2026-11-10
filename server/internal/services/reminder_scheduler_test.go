package services

import (
	"bytes"
	"context"
	"errors"
	"expenser/internal/models"
	"log"
	"strings"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func day(y int, m time.Month, d int) time.Time {
	return time.Date(y, m, d, 0, 0, 0, 0, time.UTC)
}

// fakeStore mimics the DB reminder queries over an in-memory slice.
type fakeStore struct {
	payments  []*models.RecurringPayment
	markErr   error
	markErrID uuid.UUID // when set, markErr applies only to this payment
	block     bool      // GetRecurringPaymentsToRemind waits for ctx
	panicMsg  string    // GetRecurringPaymentsToRemind panics
}

func (f *fakeStore) find(id uuid.UUID) *models.RecurringPayment {
	for _, p := range f.payments {
		if p.ID == id {
			return p
		}
	}
	return nil
}

func (f *fakeStore) GetRecurringPaymentsToRemind(ctx context.Context, today time.Time) ([]models.RecurringPayment, error) {
	if f.panicMsg != "" {
		panic(f.panicMsg)
	}
	if f.block {
		<-ctx.Done()
		return nil, ctx.Err()
	}
	out := []models.RecurringPayment{}
	for _, p := range f.payments {
		if !p.Active || p.ReminderDate().After(today) {
			continue
		}
		notifiedForDue := p.LastNotifiedFor != nil && p.LastNotifiedFor.Equal(p.DueDate)
		notifiedToday := p.LastNotifiedOn != nil && !p.LastNotifiedOn.Before(today)
		if (!p.DueDate.Before(today) && !notifiedForDue) || (p.DueDate.Before(today) && !notifiedToday) {
			out = append(out, *p)
		}
	}
	return out, nil
}

func (f *fakeStore) MarkRecurringPaymentNotified(_ context.Context, id uuid.UUID, due, sentOn time.Time) error {
	if f.markErr != nil && (f.markErrID == uuid.Nil || f.markErrID == id) {
		return f.markErr
	}
	p := f.find(id)
	p.LastNotifiedFor, p.LastNotifiedOn = &due, &sentOn
	return nil
}

type fakeNotifier struct {
	sent    []Reminder
	err     error
	panicID uuid.UUID // panic when notifying this payment
}

func (f *fakeNotifier) Notify(_ context.Context, r Reminder) error {
	if f.panicID != uuid.Nil && r.Payment.ID == f.panicID {
		panic("boom")
	}
	if f.err != nil {
		return f.err
	}
	f.sent = append(f.sent, r)
	return nil
}

func monthly(due time.Time, remind int) *models.RecurringPayment {
	return &models.RecurringPayment{
		ID:               uuid.New(),
		Category:         models.RecurringCategoryHouse,
		TypeName:         "Internet",
		StartDate:        due,
		DueDate:          due,
		CycleUnit:        models.CycleMonth,
		CycleEvery:       1,
		RemindDaysBefore: remind,
		Active:           true,
	}
}

func newTestScheduler(store ReminderStore, n Notifier, today time.Time) *ReminderScheduler {
	s := NewReminderScheduler(store, n, time.Hour)
	s.now = func() time.Time { return today }
	return s
}

func TestRunOnceSendsReminderOnce(t *testing.T) {
	p := monthly(day(2026, 11, 12), 3)
	store := &fakeStore{payments: []*models.RecurringPayment{p}}
	n := &fakeNotifier{}
	s := newTestScheduler(store, n, day(2026, 11, 10))

	require.NoError(t, s.RunOnce(context.Background()))
	require.Len(t, n.sent, 1)
	assert.Equal(t, 2, n.sent[0].DaysLeft)
	assert.Equal(t, day(2026, 11, 12), *p.LastNotifiedFor)

	require.NoError(t, s.RunOnce(context.Background()))
	assert.Len(t, n.sent, 1, "no duplicate reminder for same due date")
}

func TestRunOnceSkipsBeforeWindow(t *testing.T) {
	store := &fakeStore{payments: []*models.RecurringPayment{monthly(day(2026, 11, 20), 3)}}
	n := &fakeNotifier{}
	require.NoError(t, newTestScheduler(store, n, day(2026, 11, 10)).RunOnce(context.Background()))
	assert.Empty(t, n.sent)
}

func TestRunOnceRemindsOverdueDaily(t *testing.T) {
	p := monthly(day(2026, 11, 10), 2)
	store := &fakeStore{payments: []*models.RecurringPayment{p}}
	n := &fakeNotifier{}

	run := func(today time.Time) {
		t.Helper()
		require.NoError(t, newTestScheduler(store, n, today).RunOnce(context.Background()))
	}

	run(day(2026, 11, 8)) // window opens
	run(day(2026, 11, 9)) // already reminded for this due date
	run(day(2026, 11, 10))
	require.Len(t, n.sent, 1)
	assert.Equal(t, 2, n.sent[0].DaysLeft)

	run(day(2026, 11, 11)) // overdue
	run(day(2026, 11, 11)) // same day, no repeat
	run(day(2026, 11, 13))
	require.Len(t, n.sent, 3)
	assert.Equal(t, -1, n.sent[1].DaysLeft)
	assert.Equal(t, -3, n.sent[2].DaysLeft)
	assert.Equal(t, day(2026, 11, 10), p.DueDate, "scheduler never moves the due date")
}

func TestRunOnceRetriesFailedNotification(t *testing.T) {
	p := monthly(day(2026, 11, 10), 0)
	store := &fakeStore{payments: []*models.RecurringPayment{p}}
	n := &fakeNotifier{err: errors.New("smtp down")}
	s := newTestScheduler(store, n, day(2026, 11, 10))

	assert.ErrorContains(t, s.RunOnce(context.Background()), "smtp down", "failure is reported")
	assert.Nil(t, p.LastNotifiedFor, "failed send is not recorded")

	n.err = nil
	require.NoError(t, s.RunOnce(context.Background()))
	require.Len(t, n.sent, 1)
	assert.Equal(t, 0, n.sent[0].DaysLeft)
}

func TestRunOnceMarkFailureDoesNotStopOthers(t *testing.T) {
	first := monthly(day(2026, 11, 10), 0)
	second := monthly(day(2026, 11, 10), 0)
	store := &fakeStore{
		payments:  []*models.RecurringPayment{first, second},
		markErr:   errors.New("db gone"),
		markErrID: first.ID,
	}
	n := &fakeNotifier{}

	err := newTestScheduler(store, n, day(2026, 11, 10)).RunOnce(context.Background())
	assert.ErrorContains(t, err, "db gone")
	assert.Len(t, n.sent, 2, "second payment still reminded")
	assert.Nil(t, first.LastNotifiedFor)
	assert.NotNil(t, second.LastNotifiedFor)
}

func TestRunOnceNotifierPanicDoesNotStopOthers(t *testing.T) {
	bad := monthly(day(2026, 11, 10), 0)
	good := monthly(day(2026, 11, 10), 0)
	store := &fakeStore{payments: []*models.RecurringPayment{bad, good}}
	n := &fakeNotifier{panicID: bad.ID}

	err := newTestScheduler(store, n, day(2026, 11, 10)).RunOnce(context.Background())
	assert.ErrorContains(t, err, "notifier panic: boom")
	require.Len(t, n.sent, 1)
	assert.Equal(t, good.ID, n.sent[0].Payment.ID)
	assert.Nil(t, bad.LastNotifiedFor, "panicked reminder is retried next check")
}

func TestSafeRunOnceRecoversStorePanic(t *testing.T) {
	s := newTestScheduler(&fakeStore{panicMsg: "nil map"}, &fakeNotifier{}, day(2026, 11, 10))
	err := s.safeRunOnce(context.Background())
	assert.ErrorContains(t, err, "panic in reminder check: nil map")
}

func TestSafeRunOnceTimesOut(t *testing.T) {
	s := newTestScheduler(&fakeStore{block: true}, &fakeNotifier{}, day(2026, 11, 10))
	s.runTimeout = 50 * time.Millisecond

	done := make(chan error, 1)
	go func() { done <- s.safeRunOnce(context.Background()) }()
	select {
	case err := <-done:
		assert.ErrorIs(t, err, context.DeadlineExceeded)
	case <-time.After(2 * time.Second):
		t.Fatal("hung store stalled the scheduler")
	}
}

func TestRunStopsOnCancel(t *testing.T) {
	store := &fakeStore{}
	s := NewReminderScheduler(store, &fakeNotifier{}, time.Hour)
	ctx, cancel := context.WithCancel(context.Background())
	done := make(chan struct{})
	go func() {
		s.Run(ctx)
		close(done)
	}()
	cancel()
	select {
	case <-done:
	case <-time.After(2 * time.Second):
		t.Fatal("scheduler did not stop after cancel")
	}
}

func TestConsoleNotifier(t *testing.T) {
	var buf bytes.Buffer
	n := NewConsoleNotifier(log.New(&buf, "", 0))

	p := models.RecurringPayment{
		Username:       "ivan",
		Category:       models.RecurringCategoryCar,
		TypeName:       "Insurance",
		Name:           "Allianz",
		DueDate:        day(2026, 12, 1),
		PaymentHours:   "Mon-Fri 9-18",
		PaymentOptions: []string{"cash", "card"},
		Notes:          "Policy 7",
	}
	require.NoError(t, n.Notify(context.Background(), Reminder{Payment: p, DaysLeft: 3}))
	assert.Equal(t,
		`[reminder] user=ivan [car] Insurance "Allianz" due 2026-12-01 (in 3 days) | hours: Mon-Fri 9-18 | pay: cash, card | notes: Policy 7`,
		strings.TrimSpace(buf.String()))
}

func TestFormatReminderMinimal(t *testing.T) {
	p := models.RecurringPayment{Username: "ivan", Category: models.RecurringCategoryHouse, TypeName: "TV", DueDate: day(2026, 12, 1)}
	assert.Equal(t, "[reminder] user=ivan [house] TV due 2026-12-01 (today)", FormatReminder(Reminder{Payment: p}))
	assert.Contains(t, FormatReminder(Reminder{Payment: p, DaysLeft: 1}), "(tomorrow)")
	assert.Contains(t, FormatReminder(Reminder{Payment: p, DaysLeft: -1}), "(OVERDUE by 1 day)")
	assert.Contains(t, FormatReminder(Reminder{Payment: p, DaysLeft: -4}), "(OVERDUE by 4 days)")

	amount := 29.9
	p.ExpectedAmount = &amount
	assert.Contains(t, FormatReminder(Reminder{Payment: p}), "TV 29.90 EUR due 2026-12-01")
}
