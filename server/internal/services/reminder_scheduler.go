package services

import (
	"context"
	"expenser/internal/models"
	"fmt"
	"log"
	"time"

	"github.com/google/uuid"
)

// ReminderStore is the data access the scheduler needs. *database.DB
// implements it.
type ReminderStore interface {
	GetRecurringPaymentsToRemind(ctx context.Context, today time.Time) ([]models.RecurringPayment, error)
	MarkRecurringPaymentNotified(ctx context.Context, id uuid.UUID, dueDate, sentOn time.Time) error
}

// ReminderScheduler periodically sends reminders for recurring payments:
// once when the reminder window opens, then daily while the payment is
// overdue. Payments only move to their next due date when marked paid
// (see RecurringPaymentService.MarkPaid).
type ReminderScheduler struct {
	store    ReminderStore
	notifier Notifier
	interval time.Duration
	now      func() time.Time
}

func NewReminderScheduler(store ReminderStore, notifier Notifier, interval time.Duration) *ReminderScheduler {
	return &ReminderScheduler{
		store:    store,
		notifier: notifier,
		interval: interval,
		now:      time.Now,
	}
}

// Run checks once immediately and then on every interval until ctx is
// cancelled. It blocks, so call it in its own goroutine.
func (s *ReminderScheduler) Run(ctx context.Context) {
	log.Printf("Reminder scheduler started (interval %s).", s.interval)
	ticker := time.NewTicker(s.interval)
	defer ticker.Stop()

	for {
		if err := s.RunOnce(ctx); err != nil && ctx.Err() == nil {
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

// RunOnce performs a single check. "Today" is the server's local calendar
// date. A failed notification is logged and retried on the next check.
func (s *ReminderScheduler) RunOnce(ctx context.Context) error {
	today := models.DateOnly(s.now())

	due, err := s.store.GetRecurringPaymentsToRemind(ctx, today)
	if err != nil {
		return err
	}
	for _, p := range due {
		r := Reminder{
			Payment:  p,
			DaysLeft: p.DaysUntilDue(today),
		}
		if err := s.notifier.Notify(ctx, r); err != nil {
			log.Printf("Failed to send reminder for recurring payment %s: %v", p.ID, err)
			continue
		}
		if err := s.store.MarkRecurringPaymentNotified(ctx, p.ID, p.DueDate, today); err != nil {
			return fmt.Errorf("reminder sent but not recorded for %s: %w", p.ID, err)
		}
	}
	return nil
}
