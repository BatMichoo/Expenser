package models

import (
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
)

func date(y int, m time.Month, d int) time.Time {
	return time.Date(y, m, d, 0, 0, 0, 0, time.UTC)
}

func intPtr(i int) *int { return &i }

func TestAddCycles(t *testing.T) {
	tests := []struct {
		name   string
		anchor time.Time
		unit   CycleUnit
		every  int
		n      int
		want   time.Time
	}{
		{"zero cycles", date(2026, 1, 15), CycleMonth, 1, 0, date(2026, 1, 15)},
		{"days", date(2026, 1, 30), CycleDay, 10, 1, date(2026, 2, 9)},
		{"weeks", date(2026, 1, 1), CycleWeek, 2, 2, date(2026, 1, 29)},
		{"month simple", date(2026, 1, 15), CycleMonth, 1, 1, date(2026, 2, 15)},
		{"month clamps to Feb end", date(2026, 1, 31), CycleMonth, 1, 1, date(2026, 2, 28)},
		{"month clamp does not drift", date(2026, 1, 31), CycleMonth, 1, 2, date(2026, 3, 31)},
		{"month clamps leap year", date(2028, 1, 31), CycleMonth, 1, 1, date(2028, 2, 29)},
		{"every 6 months across year", date(2026, 9, 30), CycleMonth, 6, 1, date(2027, 3, 30)},
		{"year from leap day", date(2028, 2, 29), CycleYear, 1, 1, date(2029, 2, 28)},
		{"year back to leap day", date(2028, 2, 29), CycleYear, 1, 4, date(2032, 2, 29)},
		{"strips time of day", time.Date(2026, 5, 5, 23, 59, 0, 0, time.UTC), CycleDay, 1, 1, date(2026, 5, 6)},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			assert.Equal(t, tt.want, AddCycles(tt.anchor, tt.unit, tt.every, tt.n))
		})
	}
}

func TestNextDueAfter(t *testing.T) {
	p := &RecurringPayment{StartDate: date(2026, 1, 31), DueDate: date(2026, 1, 31), CycleUnit: CycleMonth, CycleEvery: 1}

	assert.Equal(t, date(2026, 2, 28), p.NextDueAfter(date(2026, 1, 31)), "strictly after the current due date")
	assert.Equal(t, date(2026, 1, 31), p.NextDueAfter(date(2026, 1, 10)), "paid before the first due date")
	assert.Equal(t, date(2026, 3, 31), p.NextDueAfter(date(2026, 2, 28)), "no drift after clamping")

	noStart := &RecurringPayment{DueDate: date(2026, 1, 10), CycleUnit: CycleWeek, CycleEvery: 2}
	assert.Equal(t, date(2026, 1, 24), noStart.NextDueAfter(date(2026, 1, 10)), "falls back to due date as anchor")
}

func TestDaysUntilDue(t *testing.T) {
	p := &RecurringPayment{DueDate: date(2026, 11, 12)}
	assert.Equal(t, 2, p.DaysUntilDue(date(2026, 11, 10)))
	assert.Equal(t, 0, p.DaysUntilDue(time.Date(2026, 11, 12, 23, 0, 0, 0, time.UTC)))
	assert.Equal(t, -3, p.DaysUntilDue(date(2026, 11, 15)))
}

func TestReminderDate(t *testing.T) {
	p := &RecurringPayment{DueDate: date(2026, 3, 2), RemindDaysBefore: 3}
	assert.Equal(t, date(2026, 2, 27), p.ReminderDate())
}

func TestValidate(t *testing.T) {
	valid := func() RecurringPayment {
		return RecurringPayment{
			Category:      RecurringCategoryHouse,
			UtilityTypeID: intPtr(4),
			DueDate:       date(2026, 12, 1),
			CycleUnit:     CycleMonth,
			CycleEvery:    1,
		}
	}

	tests := []struct {
		name   string
		mutate func(p *RecurringPayment)
		want   error
	}{
		{"valid house", func(p *RecurringPayment) {}, nil},
		{"valid car", func(p *RecurringPayment) {
			p.Category, p.UtilityTypeID, p.CarExpenseTypeID = RecurringCategoryCar, nil, intPtr(3)
		}, nil},
		{"bad category", func(p *RecurringPayment) { p.Category = "groceries" }, ErrInvalidCategory},
		{"house without type", func(p *RecurringPayment) { p.UtilityTypeID = nil }, ErrTypeMismatch},
		{"house with car type", func(p *RecurringPayment) { p.CarExpenseTypeID = intPtr(1) }, ErrTypeMismatch},
		{"car with utility type", func(p *RecurringPayment) {
			p.Category, p.CarExpenseTypeID = RecurringCategoryCar, intPtr(1)
		}, ErrTypeMismatch},
		{"bad unit", func(p *RecurringPayment) { p.CycleUnit = "fortnight" }, ErrInvalidCycleUnit},
		{"zero every", func(p *RecurringPayment) { p.CycleEvery = 0 }, ErrInvalidCycleEvery},
		{"negative remind", func(p *RecurringPayment) { p.RemindDaysBefore = -1 }, ErrInvalidRemindDays},
		{"no due date", func(p *RecurringPayment) { p.DueDate = time.Time{} }, ErrMissingDueDate},
		{"negative amount", func(p *RecurringPayment) { v := -1.0; p.ExpectedAmount = &v }, ErrInvalidAmount},
		{"zero amount ok", func(p *RecurringPayment) { v := 0.0; p.ExpectedAmount = &v }, nil},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			p := valid()
			tt.mutate(&p)
			assert.Equal(t, tt.want, p.Validate())
		})
	}
}
