package models

import (
	"errors"
	"fmt"
	"net/mail"
	"sort"
	"strings"
	"time"
	"unicode/utf8"

	"github.com/google/uuid"
)

// BillDateKind says what a bill's date means.
type BillDateKind string

const (
	// DatePayBy: the date is a deadline (electricity, internet, insurance).
	DatePayBy BillDateKind = "pay_by"
	// DateArrives: the date is roughly when the bill shows up; there is no
	// deadline (gas). Such bills are never overdue, only waiting.
	DateArrives BillDateKind = "arrives"
)

// NotifyType is how reminders for a bill are delivered.
type NotifyType string

const (
	NotifyEmail NotifyType = "email"
	NotifyNone  NotifyType = "none"
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
	Category         Section // house or car
	UtilityTypeID    *int    // set only for house payments
	CarExpenseTypeID *int    // set only for car payments
	TypeName         string  // read-only, populated on reads
	Name             string
	StartDate        time.Time // anchor of the cycle; defaults to DueDate
	DueDate          time.Time
	CycleUnit        CycleUnit
	CycleEvery       int
	RemindDaysBefore int
	PaymentHours     string
	PaymentOptions   []string // nil means "use defaults" on create
	Notes            string

	// Reminder schedule (see ShouldRemind).
	DateKind        BillDateKind
	RemindDaysAfter int        // arrives bills: first reminder N days after the date
	RepeatEveryDays *int       // nil: remind once; 1: daily; N: every N days until paid
	MaxReminders    *int       // nil: no limit; counted per cycle
	RemindersSent   int        // reminders sent for the current due date (read-only)
	SnoozedUntil    *time.Time // no reminders before this date; cleared when paid

	// Delivery (see ReminderRecipient). Reminders are sent in the
	// background, without a request to say who the owner is, so the bill
	// keeps what they need: the owner's name and language (as the home
	// server last sent them) and the address.
	Notify      NotifyType
	NotifyEmail string // where reminders go; empty: nowhere
	OwnerName   string // greets the owner in reminder emails; may be ""
	OwnerLang   string // the language of reminder emails (en, bg)
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
	ErrInvalidCategory     = errors.New("category must be 'house' or 'car'")
	ErrTypeMismatch        = errors.New("house payments need a utility type and car payments need a car expense type, not both")
	ErrInvalidCycleUnit    = errors.New("cycle unit must be one of day, week, month, year")
	ErrInvalidCycleEvery   = errors.New("cycle every must be at least 1")
	ErrInvalidRemindDays   = errors.New("remind days before cannot be negative")
	ErrMissingDueDate      = errors.New("due date is required")
	ErrInvalidAmount       = errors.New("amount cannot be negative")
	ErrInvalidDateKind     = errors.New("date kind must be pay_by or arrives")
	ErrInvalidNotify       = errors.New("notify must be email or none")
	ErrInvalidEmail        = errors.New("not a valid email address")
	ErrInvalidRepeat       = errors.New("reminders can repeat at most once a day")
	ErrInvalidMaxReminders = errors.New("the reminder limit must be at least 1")
	ErrNameTooLong         = fmt.Errorf("name cannot be longer than %d characters", MaxRecurringPaymentNameLength)
)

// MaxRecurringPaymentNameLength matches recurring_payments.name VARCHAR(100).
const MaxRecurringPaymentNameLength = 100

// Normalize tidies user input in place: trims the name and cleans up
// payment options (see NormalizePaymentOptions). Call before Validate.
func (p *RecurringPayment) Normalize() {
	p.Name = strings.TrimSpace(p.Name)
	if p.DateKind == "" {
		p.DateKind = DatePayBy
	}
	if p.Notify == "" {
		p.Notify = NotifyEmail
	}
	p.NotifyEmail = strings.TrimSpace(p.NotifyEmail)
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
	case SectionHouse:
		if p.UtilityTypeID == nil || p.CarExpenseTypeID != nil {
			return ErrTypeMismatch
		}
	case SectionCar:
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
	if p.RemindDaysBefore < 0 || p.RemindDaysAfter < 0 {
		return ErrInvalidRemindDays
	}
	switch p.DateKind {
	case "", DatePayBy, DateArrives:
	default:
		return ErrInvalidDateKind
	}
	if p.RepeatEveryDays != nil && *p.RepeatEveryDays < 1 {
		return ErrInvalidRepeat
	}
	if p.MaxReminders != nil && *p.MaxReminders < 1 {
		return ErrInvalidMaxReminders
	}
	switch p.Notify {
	case "", NotifyEmail, NotifyNone:
	default:
		return ErrInvalidNotify
	}
	if p.NotifyEmail != "" && !ValidEmail(p.NotifyEmail) {
		return ErrInvalidEmail
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

// NextPayableOnOrAfter is the first cycle date whose effective date (see
// EffectiveDueDate) is on or after date: a Sunday cycle whose Friday has
// already passed is missed too, so it is skipped.
func (p *RecurringPayment) NextPayableOnOrAfter(date time.Time) (time.Time, error) {
	date = DateOnly(date)
	next, err := p.NextDueOnOrAfter(date)
	for err == nil && p.effective(next).Before(date) {
		next, err = p.NextDueAfter(next)
	}
	return next, err
}

// SortByDueDate orders payments by effective due date, then by the real due
// date (Friday, Saturday, Sunday), then oldest first, so every list of
// bills shows ties in the same order.
func SortByDueDate(ps []RecurringPayment) {
	sort.SliceStable(ps, func(i, j int) bool {
		a, b := ps[i], ps[j]
		if ea, eb := a.EffectiveDueDate(), b.EffectiveDueDate(); !ea.Equal(eb) {
			return ea.Before(eb)
		}
		if da, db := DateOnly(a.DueDate), DateOnly(b.DueDate); !da.Equal(db) {
			return da.Before(db)
		}
		return a.CreatedAt.Before(b.CreatedAt)
	})
}

// MissedCycles reports whether the payment is more than one cycle behind:
// not only is the current due date past, the following one is too (both
// compared by their effective dates, see EffectiveDueDate). Such a
// payment can be skipped forward instead of paid cycle by cycle.
func (p *RecurringPayment) MissedCycles(today time.Time) bool {
	today = DateOnly(today)
	if !p.EffectiveDueDate().Before(today) {
		return false
	}
	next, err := p.NextDueAfter(p.DueDate)
	return err == nil && !p.effective(next).After(today)
}

// PayByDate is the last working day on or before d: a Saturday or Sunday
// moves back to the Friday before (offices are closed at the weekend).
// Public holidays are not considered.
func PayByDate(d time.Time) time.Time {
	d = DateOnly(d)
	switch d.Weekday() {
	case time.Saturday:
		return d.AddDate(0, 0, -1)
	case time.Sunday:
		return d.AddDate(0, 0, -2)
	}
	return d
}

// effective is the date that counts for paying a bill due on d: PayByDate
// for pay-by bills; arrives bills have no deadline and keep their date.
func (p RecurringPayment) effective(d time.Time) time.Time {
	if p.DateKind == DateArrives {
		return DateOnly(d)
	}
	return PayByDate(d)
}

// EffectiveDueDate is the date that counts for the current due date: a
// pay-by bill due at the weekend is due the Friday before. Overdue, days
// left, the first reminder, sorting and what people see use it; the cycle,
// paying, snoozing and reminder bookkeeping keep the stored DueDate.
func (p RecurringPayment) EffectiveDueDate() time.Time {
	return p.effective(p.DueDate)
}

// WeekendShifted reports whether EffectiveDueDate differs from the stored
// due date (a pay-by bill due on a Saturday or Sunday).
func (p RecurringPayment) WeekendShifted() bool {
	return !p.EffectiveDueDate().Equal(DateOnly(p.DueDate))
}

// DaysUntilDue counts to the effective due date; negative when overdue.
func (p *RecurringPayment) DaysUntilDue(today time.Time) int {
	return int(p.EffectiveDueDate().Sub(DateOnly(today)).Hours() / 24)
}

// FirstReminderDate is the first day a reminder goes out for the current
// due date: N days before the effective deadline, or N days after an
// arrival date.
func (p *RecurringPayment) FirstReminderDate() time.Time {
	due := p.EffectiveDueDate()
	if p.DateKind == DateArrives {
		return due.AddDate(0, 0, p.RemindDaysAfter)
	}
	return due.AddDate(0, 0, -p.RemindDaysBefore)
}

// IsOverdue: a pay-by bill whose effective deadline has passed unpaid.
func (p *RecurringPayment) IsOverdue(today time.Time) bool {
	return p.DateKind != DateArrives && p.EffectiveDueDate().Before(DateOnly(today))
}

// IsWaiting: an arrives bill past its date and not yet paid.
func (p *RecurringPayment) IsWaiting(today time.Time) bool {
	return p.DateKind == DateArrives && DateOnly(p.DueDate).Before(DateOnly(today))
}

// ShouldRemind reports whether a reminder is due today:
//  1. paused or snoozed: no;
//  2. before the first reminder date: no;
//  3. not yet reminded for this due date: yes;
//  4. the per-cycle cap is reached: no;
//  5. otherwise only if repeating and the repeat interval has passed.
//
// Past a pay-by deadline the bill keeps its own rhythm.
func (p *RecurringPayment) ShouldRemind(today time.Time) bool {
	today = DateOnly(today)
	if !p.Active {
		return false
	}
	if p.SnoozedUntil != nil && today.Before(DateOnly(*p.SnoozedUntil)) {
		return false
	}
	if today.Before(p.FirstReminderDate()) {
		return false
	}
	thisCycle := p.LastNotifiedFor != nil && DateOnly(*p.LastNotifiedFor).Equal(DateOnly(p.DueDate))
	if !thisCycle || p.LastNotifiedOn == nil {
		return true
	}
	if p.MaxReminders != nil && p.RemindersSent >= *p.MaxReminders {
		return false
	}
	if p.RepeatEveryDays == nil {
		return false
	}
	return !today.Before(DateOnly(*p.LastNotifiedOn).AddDate(0, 0, *p.RepeatEveryDays))
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
	Waiting      []BillRow // arrives bills past their date, not yet paid
	DueSoon      []BillRow
	Later        []BillRow
	Paused       []BillRow
	OverdueCount int
	WaitingCount int
	DueThisMonth float64 // expected amounts of active bills due by month end
	Next         *BillRow
	Total        int
	Lang         string
}

// ReminderRecipient is where this bill's reminders go. Empty means nobody,
// and then no reminder is sent at all.
func (p RecurringPayment) ReminderRecipient() string {
	if p.Notify == NotifyNone {
		return ""
	}
	return p.NotifyEmail
}

// ValidEmail accepts a plain address like "name@example.com".
func ValidEmail(s string) bool {
	addr, err := mail.ParseAddress(s)
	return err == nil && addr.Address == s && addr.Name == ""
}

// TypeID is the bill's expense type: a utility type for house bills, a car
// expense type for car bills; nil when not set.
func (p *RecurringPayment) TypeID() *int {
	if p.Category == SectionCar {
		return p.CarExpenseTypeID
	}
	return p.UtilityTypeID
}

// SetTypeID sets the bill's expense type for its category.
func (p *RecurringPayment) SetTypeID(id int) {
	if p.Category == SectionCar {
		p.CarExpenseTypeID, p.UtilityTypeID = &id, nil
		return
	}
	p.UtilityTypeID, p.CarExpenseTypeID = &id, nil
}
