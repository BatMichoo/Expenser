package services

import (
	"context"
	"errors"
	"expenser/internal/models"
	"fmt"
	"log"
	"sync"
	"time"

	"github.com/google/uuid"
)

// ReminderStore is the data access the scheduler needs. *database.DB
// implements it.
type ReminderStore interface {
	GetActiveRecurringPayments(ctx context.Context) ([]models.RecurringPayment, error)
	MarkRecurringPaymentNotified(ctx context.Context, id uuid.UUID, dueDate, sentOn time.Time) error
}

// ReminderScheduler periodically sends reminders for recurring payments,
// following each bill's schedule (RecurringPayment.ShouldRemind). Payments only move to their next due date when marked paid
// (see RecurringPaymentService.MarkPaid).
type ReminderScheduler struct {
	store      ReminderStore
	notifier   Notifier
	interval   time.Duration
	hours      models.TimeWindow // reminders are only sent inside these hours
	runTimeout time.Duration     // upper bound for one RunOnce, so a hung DB cannot stall the loop
	now        func() time.Time

	// unrecorded remembers reminders that went out but could not be saved
	// (the database failed after sending), so a later check the same day
	// saves them instead of sending again.
	mu         sync.Mutex
	unrecorded map[uuid.UUID]unrecordedReminder
}

// unrecordedReminder is a reminder sent on sentOn for dueDate that is not
// yet saved as sent.
type unrecordedReminder struct {
	dueDate, sentOn time.Time
}

// defaultRunTimeout bounds a single reminder check.
const defaultRunTimeout = 5 * time.Minute

func NewReminderScheduler(store ReminderStore, notifier Notifier, interval time.Duration, hours models.TimeWindow) *ReminderScheduler {
	return &ReminderScheduler{
		store:      store,
		notifier:   notifier,
		interval:   interval,
		hours:      hours,
		runTimeout: defaultRunTimeout,
		now:        time.Now,
		unrecorded: map[uuid.UUID]unrecordedReminder{},
	}
}

// Run checks once immediately and then after each nextWait until ctx is
// cancelled. It blocks, so call it in its own goroutine.
func (s *ReminderScheduler) Run(ctx context.Context) {
	log.Printf("Reminder scheduler started (interval %s, sending %s).", s.interval, s.hours)
	timer := time.NewTimer(0)
	defer timer.Stop()
	<-timer.C

	for {
		if err := s.safeRunOnce(ctx); err != nil && ctx.Err() == nil {
			log.Printf("Reminder check failed: %v", err)
		}
		timer.Reset(s.nextWait(s.now()))
		select {
		case <-ctx.Done():
			log.Println("Reminder scheduler stopped.")
			return
		case <-timer.C:
		}
	}
}

// nextWait is how long to sleep before the next check: the interval, but
// never past the next time the send hours open, so a long interval, a
// restart at night or a daylight-saving change can't skip a day.
func (s *ReminderScheduler) nextWait(now time.Time) time.Duration {
	if open := s.hours.NextOpenAfter(now).Sub(now); open < s.interval {
		return open
	}
	return s.interval
}

// safeRunOnce bounds RunOnce with runTimeout and turns a panic into an
// error, so a bug in the scheduler or a notifier cannot crash the server.
func (s *ReminderScheduler) safeRunOnce(ctx context.Context) (err error) {
	defer func() {
		if r := recover(); r != nil {
			err = fmt.Errorf("panic in reminder check: %v", r)
		}
	}()
	if s.runTimeout > 0 {
		var cancel context.CancelFunc
		ctx, cancel = context.WithTimeout(ctx, s.runTimeout)
		defer cancel()
	}
	return s.RunOnce(ctx)
}

// RunOnce performs a single check. Outside the send hours it does nothing.
// "Today" is the server's local calendar date. Problems with one payment are logged and do not stop the others; a
// payment whose reminder failed is retried on the next check, and one sent
// but not saved is saved on the next check, not sent again that day. The
// returned error joins every per-payment failure.
func (s *ReminderScheduler) RunOnce(ctx context.Context) error {
	now := s.now()
	if !s.hours.Contains(now) {
		return nil // outside the send hours; the next check in hours sends
	}
	today := models.DateOnly(now)

	s.mu.Lock()
	defer s.mu.Unlock()
	var errs []error
	for id, u := range s.unrecorded {
		if !u.sentOn.Equal(today) {
			delete(s.unrecorded, id) // a new day: send as usual
			continue
		}
		if err := s.store.MarkRecurringPaymentNotified(ctx, id, u.dueDate, u.sentOn); err != nil {
			errs = append(errs, fmt.Errorf("reminder sent but not recorded for %s: %w", id, err))
			continue
		}
		delete(s.unrecorded, id)
	}

	active, err := s.store.GetActiveRecurringPayments(ctx)
	if err != nil {
		return errors.Join(append(errs, err)...)
	}
	due := make([]models.RecurringPayment, 0, len(active))
	for _, p := range active {
		// Nobody to send to: skip entirely (not sent, not counted).
		if p.ReminderRecipient() == "" {
			continue
		}
		if _, sent := s.unrecorded[p.ID]; sent {
			continue // already sent today, only the record is missing
		}
		if p.ShouldRemind(today) {
			due = append(due, p)
		}
	}

	for _, p := range due {
		if ctx.Err() != nil {
			errs = append(errs, ctx.Err())
			break
		}
		r := Reminder{
			Payment:  p,
			DaysLeft: p.DaysUntilDue(today),
			To:       p.ReminderRecipient(),
		}
		if err := s.notify(ctx, r); err != nil {
			errs = append(errs, fmt.Errorf("send reminder for %s: %w", p.ID, err))
			continue
		}
		if err := s.store.MarkRecurringPaymentNotified(ctx, p.ID, p.DueDate, today); err != nil {
			// The reminder went out: later checks today retry saving it
			// instead of sending it again.
			s.unrecorded[p.ID] = unrecordedReminder{dueDate: p.DueDate, sentOn: today}
			errs = append(errs, fmt.Errorf("reminder sent but not recorded for %s: %w", p.ID, err))
		}
	}
	return errors.Join(errs...)
}

// notify calls the notifier, converting a panic into an error so one bad
// reminder does not abort the rest.
func (s *ReminderScheduler) notify(ctx context.Context, r Reminder) (err error) {
	defer func() {
		if rec := recover(); rec != nil {
			err = fmt.Errorf("notifier panic: %v", rec)
		}
	}()
	return s.notifier.Notify(ctx, r)
}
