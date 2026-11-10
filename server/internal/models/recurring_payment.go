package models

import (
	"errors"
	"time"

	"github.com/google/uuid"
)

type RecurringPaymentCategory string

const (
	RecurringCategoryHouse RecurringPaymentCategory = "house"
	RecurringCategoryCar   RecurringPaymentCategory = "car"
)

type CycleUnit string

const (
	CycleDay   CycleUnit = "day"
	CycleWeek  CycleUnit = "week"
	CycleMonth CycleUnit = "month"
	CycleYear  CycleUnit = "year"
)

// Common payment options. PaymentOptions is free-form, so users may store
// values outside this list.
const (
	PaymentOptionCash         = "cash"
	PaymentOptionCard         = "card"
	PaymentOptionBankTransfer = "bank_transfer"
	PaymentOptionOnline       = "online"
)

// DefaultPaymentOptions is applied when a payment is created without any
// options. Cash is pre-filled but users may remove it.
func DefaultPaymentOptions() []string {
	return []string{PaymentOptionCash}
}

// RecurringPayment is a bill that repeats on a cycle and that the user wants
// to be reminded about before it is due.
type RecurringPayment struct {
	ID               uuid.UUID
	UserID           uuid.UUID
	Username         string // read-only, populated on reads
	Category         RecurringPaymentCategory
	UtilityTypeID    *int   // set only for house payments
	CarExpenseTypeID *int   // set only for car payments
	TypeName         string // read-only, populated on reads
	Name             string
	StartDate        time.Time // anchor of the cycle; defaults to DueDate
	DueDate          time.Time
	CycleUnit        CycleUnit
	CycleEvery       int
	RemindDaysBefore int
	PaymentHours     string
	PaymentOptions   []string // nil means "use defaults" on create
	Notes            string
	LastNotifiedFor  *time.Time // due date the last reminder was sent for
	Active           bool
	CreatedAt        time.Time
	UpdatedAt        time.Time
}

var (
	ErrInvalidCategory   = errors.New("category must be 'house' or 'car'")
	ErrTypeMismatch      = errors.New("house payments need a utility type and car payments need a car expense type, not both")
	ErrInvalidCycleUnit  = errors.New("cycle unit must be one of day, week, month, year")
	ErrInvalidCycleEvery = errors.New("cycle every must be at least 1")
	ErrInvalidRemindDays = errors.New("remind days before cannot be negative")
	ErrMissingDueDate    = errors.New("due date is required")
)

// Validate checks the fields a user controls.
func (p *RecurringPayment) Validate() error {
	switch p.Category {
	case RecurringCategoryHouse:
		if p.UtilityTypeID == nil || p.CarExpenseTypeID != nil {
			return ErrTypeMismatch
		}
	case RecurringCategoryCar:
		if p.CarExpenseTypeID == nil || p.UtilityTypeID != nil {
			return ErrTypeMismatch
		}
	default:
		return ErrInvalidCategory
	}

	switch p.CycleUnit {
	case CycleDay, CycleWeek, CycleMonth, CycleYear:
	default:
		return ErrInvalidCycleUnit
	}

	if p.CycleEvery < 1 {
		return ErrInvalidCycleEvery
	}
	if p.RemindDaysBefore < 0 {
		return ErrInvalidRemindDays
	}
	if p.DueDate.IsZero() {
		return ErrMissingDueDate
	}
	return nil
}

// DateOnly strips the time of day, returning midnight UTC of t's calendar
// date. All recurring payment dates are compared in this form.
func DateOnly(t time.Time) time.Time {
	y, m, d := t.Date()
	return time.Date(y, m, d, 0, 0, 0, 0, time.UTC)
}

// AddCycles returns the date n cycles after anchor. Month and year cycles
// clamp to the last day of shorter months (Jan 31 + 1 month = Feb 28/29),
// always measured from the anchor so the day does not drift.
func AddCycles(anchor time.Time, unit CycleUnit, every, n int) time.Time {
	anchor = DateOnly(anchor)
	steps := every * n
	switch unit {
	case CycleDay:
		return anchor.AddDate(0, 0, steps)
	case CycleWeek:
		return anchor.AddDate(0, 0, 7*steps)
	case CycleMonth:
		return addMonthsClamped(anchor, steps)
	case CycleYear:
		return addMonthsClamped(anchor, 12*steps)
	}
	return anchor
}

func addMonthsClamped(t time.Time, months int) time.Time {
	total := int(t.Month()) - 1 + months
	year := t.Year() + total/12
	month := total % 12
	if month < 0 {
		month += 12
		year--
	}
	firstOfMonth := time.Date(year, time.Month(month+1), 1, 0, 0, 0, 0, time.UTC)
	lastDay := firstOfMonth.AddDate(0, 1, -1).Day()
	day := t.Day()
	if day > lastDay {
		day = lastDay
	}
	return time.Date(year, time.Month(month+1), day, 0, 0, 0, 0, time.UTC)
}

// NextDueDate returns the first cycle date on or after today, counting from
// StartDate.
func (p *RecurringPayment) NextDueDate(today time.Time) time.Time {
	today = DateOnly(today)
	start := p.StartDate
	if start.IsZero() {
		start = p.DueDate
	}
	every := p.CycleEvery
	if every < 1 {
		every = 1
	}
	for n := 0; ; n++ {
		d := AddCycles(start, p.CycleUnit, every, n)
		if !d.Before(today) {
			return d
		}
	}
}

// ReminderDate is the first day a reminder should go out for the current
// due date.
func (p *RecurringPayment) ReminderDate() time.Time {
	return DateOnly(p.DueDate).AddDate(0, 0, -p.RemindDaysBefore)
}
