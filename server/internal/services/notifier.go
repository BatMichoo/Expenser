package services

import (
	"context"
	"expenser/internal/models"
	"expenser/internal/utilities"
	"fmt"
	"log"
	"strings"
)

// Reminder is a single notification about an upcoming recurring payment.
type Reminder struct {
	Payment  models.RecurringPayment
	DaysLeft int // negative when overdue
}

// Notifier delivers reminders to users. Implementations can send email,
// push notifications, etc.; ConsoleNotifier just logs.
type Notifier interface {
	Notify(ctx context.Context, r Reminder) error
}

// ConsoleNotifier writes reminders to a logger. Used until real delivery
// (e.g. email) is implemented.
type ConsoleNotifier struct {
	logger *log.Logger
}

// NewConsoleNotifier logs to logger, or to the standard logger when nil.
func NewConsoleNotifier(logger *log.Logger) *ConsoleNotifier {
	if logger == nil {
		logger = log.Default()
	}
	return &ConsoleNotifier{logger: logger}
}

func (n *ConsoleNotifier) Notify(_ context.Context, r Reminder) error {
	n.logger.Print(FormatReminder(r))
	return nil
}

// FormatReminder renders a reminder as a single human-readable line.
func FormatReminder(r Reminder) string {
	p := r.Payment
	var b strings.Builder

	fmt.Fprintf(&b, "[reminder] user=%s [%s] %s", p.Username, p.Category, p.TypeName)
	if p.Name != "" {
		fmt.Fprintf(&b, " %q", p.Name)
	}
	if p.ExpectedAmount != nil {
		fmt.Fprintf(&b, " %.2f %s", *p.ExpectedAmount, utilities.Currency)
	}
	if p.DateKind == models.DateArrives {
		fmt.Fprintf(&b, " arrives around %s (%s)", p.DueDate.Format("2006-01-02"), waitingText(r.DaysLeft))
	} else {
		fmt.Fprintf(&b, " due %s (%s)", p.DueDate.Format("2006-01-02"), daysLeftText(r.DaysLeft))
	}
	if p.PaymentHours != "" {
		fmt.Fprintf(&b, " | hours: %s", p.PaymentHours)
	}
	if len(p.PaymentOptions) > 0 {
		fmt.Fprintf(&b, " | pay: %s", strings.Join(p.PaymentOptions, ", "))
	}
	if p.Notes != "" {
		fmt.Fprintf(&b, " | notes: %s", p.Notes)
	}
	return b.String()
}

func daysLeftText(days int) string {
	switch {
	case days == -1:
		return "OVERDUE by 1 day"
	case days < 0:
		return fmt.Sprintf("OVERDUE by %d days", -days)
	}
	switch days {
	case 0:
		return "today"
	case 1:
		return "tomorrow"
	default:
		return fmt.Sprintf("in %d days", days)
	}
}

// waitingText describes an arrives bill: there is no deadline, so past its
// date it is waiting to be paid rather than overdue.
func waitingText(days int) string {
	switch {
	case days == -1:
		return "waiting 1 day"
	case days < 0:
		return fmt.Sprintf("waiting %d days", -days)
	}
	return daysLeftText(days)
}
