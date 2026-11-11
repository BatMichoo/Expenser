package models

import (
	"errors"
	"fmt"
	"strings"
	"time"
	"unicode/utf8"

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
	// ExpectedAmount is what the bill usually costs (nil when unknown). It
	// pre-fills MarkPaid and is shown in reminders.
	ExpectedAmount *float64
	// RememberLastAmount makes each payment update ExpectedAmount.
	RememberLastAmount bool
	LastPaidOn         *time.Time // date the user last marked it paid
	LastNotifiedFor    *time.Time // due date the last reminder was sent for
	LastNotifiedOn     *time.Time // calendar date the last reminder was sent
	Active             bool
	CreatedAt          time.Time
	UpdatedAt          time.Time
}

var (
	ErrInvalidCategory   = errors.New("category must be 'house' or 'car'")
	ErrTypeMismatch      = errors.New("house payments need a utility type and car payments need a car expense type, not both")
	ErrInvalidCycleUnit  = errors.New("cycle unit must be one of day, week, month, year")
	ErrInvalidCycleEvery = errors.New("cycle every must be at least 1")
	ErrInvalidRemindDays = errors.New("remind days before cannot be negative")
	ErrMissingDueDate    = errors.New("due date is required")
	ErrInvalidAmount     = errors.New("amount cannot be negative")
	ErrNameTooLong       = fmt.Errorf("name cannot be longer than %d characters", MaxRecurringPaymentNameLength)
)

// MaxRecurringPaymentNameLength matches recurring_payments.name VARCHAR(100).
const MaxRecurringPaymentNameLength = 100

// Normalize tidies user input in place: trims the name and cleans up
// payment options (see NormalizePaymentOptions). Call before Validate.
func (p *RecurringPayment) Normalize() {
	p.Name = strings.TrimSpace(p.Name)
	p.PaymentOptions = NormalizePaymentOptions(p.PaymentOptions)
}

// NormalizePaymentOptions trims each option, drops blanks and removes
// case-insensitive duplicates, keeping the first spelling. Nil stays nil
// (meaning "use defaults" on create); anything else returns a non-nil slice.
func NormalizePaymentOptions(options []string) []string {
	if options == nil {
		return nil
	}
	out := []string{}
	seen := map[string]bool{}
	for _, o := range options {
		o = strings.TrimSpace(o)
		key := strings.ToLower(o)
		if o == "" || seen[key] {
			continue
		}
		seen[key] = true
		out = append(out, o)
	}
	return out
}

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

	if !validCycleUnit(p.CycleUnit) {
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
	if p.ExpectedAmount != nil && *p.ExpectedAmount < 0 {
		return ErrInvalidAmount
	}
	if utf8.RuneCountInString(p.Name) > MaxRecurringPaymentNameLength {
		return ErrNameTooLong
	}
	return nil
}

func validCycleUnit(u CycleUnit) bool {
	switch u {
	case CycleDay, CycleWeek, CycleMonth, CycleYear:
		return true
	}
	return false
}

// DateOnly strips the time of day, returning midnight UTC of t's calendar
// date. All recurring payment dates are compared in this form.
func DateOnly(t time.Time) time.Time {
	y, m, d := t.Date()
	return time.Date(y, m, d, 0, 0, 0, 0, time.UTC)
}

// AddCycles returns the date n cycles after anchor. An unknown unit returns
// anchor unchanged; callers should validate the unit first. Month and year cycles
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

// NextDueAfter returns the first cycle date strictly after date, counting
// from StartDate. Used to move to the next cycle once a payment is paid.
// Returns ErrInvalidCycleUnit instead of looping forever on bad data.
func (p *RecurringPayment) NextDueAfter(date time.Time) (time.Time, error) {
	if !validCycleUnit(p.CycleUnit) {
		return time.Time{}, ErrInvalidCycleUnit
	}
	date = DateOnly(date)
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
		if d.After(date) {
			return d, nil
		}
	}
}

// NextDueOnOrAfter is the first cycle date on or after date (today counts).
func (p *RecurringPayment) NextDueOnOrAfter(date time.Time) (time.Time, error) {
	return p.NextDueAfter(DateOnly(date).AddDate(0, 0, -1))
}

// MissedCycles reports whether the payment is more than one cycle behind:
// not only is the current due date past, the following one is too. Such a
// payment can be skipped forward instead of paid cycle by cycle.
func (p *RecurringPayment) MissedCycles(today time.Time) bool {
	today = DateOnly(today)
	if !DateOnly(p.DueDate).Before(today) {
		return false
	}
	next, err := p.NextDueAfter(p.DueDate)
	return err == nil && !next.After(today)
}

// DaysUntilDue is negative when the payment is overdue.
func (p *RecurringPayment) DaysUntilDue(today time.Time) int {
	return int(DateOnly(p.DueDate).Sub(DateOnly(today)).Hours() / 24)
}

// ReminderDate is the first day a reminder should go out for the current
// due date.
func (p *RecurringPayment) ReminderDate() time.Time {
	return DateOnly(p.DueDate).AddDate(0, 0, -p.RemindDaysBefore)
}

// UpcomingPayment is an unpaid recurring payment due within a time window,
// including overdue ones.
type UpcomingPayment struct {
	Payment  RecurringPayment
	DaysLeft int // negative when overdue
	Overdue  bool
}

// PaidRecurringPayment is the outcome of marking a recurring payment paid.
type PaidRecurringPayment struct {
	Payment   *RecurringPayment // state after moving to the next cycle
	ExpenseID int               // id in home_expenses or car_expenses
	PaidDue   time.Time         // the due date that was paid
}

// BillRow is one recurring payment as shown on the Bills page.
type BillRow struct {
	Payment  RecurringPayment
	DaysLeft int  // negative when overdue
	Missed   bool // more than one cycle behind; can be skipped forward
}

// BillsView is the Bills page: payments grouped by urgency plus totals.
type BillsView struct {
	Overdue      []BillRow
	DueSoon      []BillRow
	Later        []BillRow
	Paused       []BillRow
	OverdueCount int
	DueThisMonth float64 // expected amounts of active bills due by month end
	Next         *BillRow
	Total        int
	Lang         string
}
