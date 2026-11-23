package models

import (
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
)

func TestPayByDate(t *testing.T) {
	tests := []struct {
		name string
		in   string
		want string
	}{
		{"saturday to friday", "2026-12-12", "2026-12-11"},
		{"sunday to friday", "2026-12-13", "2026-12-11"},
		{"friday unchanged", "2026-12-11", "2026-12-11"},
		{"monday unchanged", "2026-12-14", "2026-12-14"},
		{"sunday into previous month", "2026-02-01", "2026-01-30"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			assert.Equal(t, mustDate(tt.want), PayByDate(mustDate(tt.in)))
		})
	}
}

func TestEffectiveDueDate(t *testing.T) {
	payBySat := RecurringPayment{DateKind: DatePayBy, DueDate: mustDate("2026-12-12")}
	assert.Equal(t, mustDate("2026-12-11"), payBySat.EffectiveDueDate())
	assert.True(t, payBySat.WeekendShifted())

	arrivesSat := RecurringPayment{DateKind: DateArrives, DueDate: mustDate("2026-12-12")}
	assert.Equal(t, mustDate("2026-12-12"), arrivesSat.EffectiveDueDate())
	assert.False(t, arrivesSat.WeekendShifted())

	payByWed := RecurringPayment{DateKind: DatePayBy, DueDate: mustDate("2026-12-09")}
	assert.Equal(t, mustDate("2026-12-09"), payByWed.EffectiveDueDate())
	assert.False(t, payByWed.WeekendShifted())
}

func TestDaysUntilDueUsesEffectiveDate(t *testing.T) {
	p := RecurringPayment{DateKind: DatePayBy, DueDate: mustDate("2026-12-13")}
	assert.Equal(t, 2, p.DaysUntilDue(mustDate("2026-12-09")))
}

func TestOverdueOnSaturdayForSundayBill(t *testing.T) {
	p := RecurringPayment{DateKind: DatePayBy, DueDate: mustDate("2026-12-13")}
	sat := mustDate("2026-12-12")
	assert.True(t, p.IsOverdue(sat))
	assert.Equal(t, -1, p.DaysUntilDue(sat))
}

func TestFirstReminderDateFromEffectiveDate(t *testing.T) {
	payBy := RecurringPayment{DateKind: DatePayBy, DueDate: mustDate("2026-12-13"), RemindDaysBefore: 3}
	assert.Equal(t, mustDate("2026-12-08"), payBy.FirstReminderDate())
	arrives := RecurringPayment{DateKind: DateArrives, DueDate: mustDate("2026-12-13"), RemindDaysAfter: 2}
	assert.Equal(t, mustDate("2026-12-15"), arrives.FirstReminderDate())
}

func TestShouldRemindStillKeysOnStoredDate(t *testing.T) {
	due := mustDate("2026-12-13")
	sent := mustDate("2026-12-10")
	p := RecurringPayment{
		Active: true, DateKind: DatePayBy, DueDate: due, RemindDaysBefore: 3,
		RepeatEveryDays: intPtr(1), LastNotifiedFor: &due, LastNotifiedOn: &sent, RemindersSent: 1,
	}
	assert.False(t, p.ShouldRemind(mustDate("2026-12-10")))
	assert.True(t, p.ShouldRemind(mustDate("2026-12-11")))
}

func TestMissedCyclesEffective(t *testing.T) {
	p := RecurringPayment{
		DateKind: DatePayBy, DueDate: mustDate("2026-11-15"), StartDate: mustDate("2026-11-15"),
		CycleUnit: CycleMonth, CycleEvery: 1,
	}
	assert.True(t, p.MissedCycles(mustDate("2026-12-15")))
	assert.False(t, p.MissedCycles(mustDate("2026-12-14")))
}

func mustDate(s string) time.Time {
	d, err := time.Parse("2006-01-02", s)
	if err != nil {
		panic(err)
	}
	return d
}
