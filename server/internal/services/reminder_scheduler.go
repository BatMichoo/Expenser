package services

import (
	"context"
	"errors"
	"expenser/internal/models"
	"fmt"
	"log"
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
	runTimeout time.Duration // upper bound for one RunOnce, so a hung DB cannot stall the loop
	now        func() time.Time
}

// defaultRunTimeout bounds a single reminder check.
const defaultRunTimeout = 5 * time.Minute

func NewReminderScheduler(store ReminderStore, notifier Notifier, interval time.Duration) *ReminderScheduler {
	return &ReminderScheduler{
		store:      store,
		notifier:   notifier,
		interval:   interval,
		runTimeout: defaultRunTimeout,
		now:        time.Now,
	}
}

// Run checks once immediately and then on every interval until ctx is
// cancelled. It blocks, so call it in its own goroutine.
func (s *ReminderScheduler) Run(ctx context.Context) {
	log.Printf("Reminder scheduler started (interval %s).", s.interval)
	ticker := time.NewTicker(s.interval)
	defer ticker.Stop()

	for {
		if err := s.safeRunOnce(ctx); err != nil && ctx.Err() == nil {
			log.Printf("Reminder check failed: %v", err)
		}
		select {
		case <-ctx.Done():
			log.Println("Reminder scheduler stopped.")
			return
		case <-ticker.C:
		}
	}
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

// RunOnce performs a single check. "Today" is the server's local calendar
// date. Problems with one payment are logged and do not stop the others; a
// payment whose reminder failed is retried on the next check. The returned
// error joins every per-payment failure.
func (s *ReminderScheduler) RunOnce(ctx context.Context) error {
	today := models.DateOnly(s.now())

	active, err := s.store.GetActiveRecurringPayments(ctx)
	if err != nil {
		return err
	}
	due := make([]models.RecurringPayment, 0, len(active))
	for _, p := range active {
		// Nobody to send to: skip entirely (not sent, not counted).
		if p.ReminderRecipient() == "" {
			continue
		}
		if p.ShouldRemind(today) {
			due = append(due, p)
		}
	}

	var errs []error
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
			// The reminder went out but may be sent again on the next check.
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
