package handlers

import (
	"expenser/internal/models"
	"net/url"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestParseBillFormHouse(t *testing.T) {
	v := url.Values{
		"category": {"house"}, "typeID": {"4"}, "name": {" Fiber "},
		"expected_amount": {"29.90"}, "remember_last_amount": {"on"},
		"due_date": {"2026-12-01"}, "cycle_every": {"1"}, "cycle_unit": {"month"},
		"remind_days_before": {"3"}, "payment_hours": {"Mon-Fri 9-18"},
		"payment_options": {"cash", "card"}, "payment_option_other": {"ePay"},
		"notes": {"Contract 42"},
	}
	p, err := parseBillForm(v)
	require.NoError(t, err)
	assert.Equal(t, models.RecurringCategoryHouse, p.Category)
	require.NotNil(t, p.UtilityTypeID)
	assert.Equal(t, 4, *p.UtilityTypeID)
	assert.Nil(t, p.CarExpenseTypeID)
	assert.Equal(t, " Fiber ", p.Name, "trimmed later by the model")
	require.NotNil(t, p.ExpectedAmount)
	assert.InDelta(t, 29.9, *p.ExpectedAmount, 0.001)
	assert.True(t, p.RememberLastAmount)
	assert.Equal(t, time.Date(2026, 12, 1, 0, 0, 0, 0, time.UTC), p.DueDate)
	assert.Equal(t, models.CycleMonth, p.CycleUnit)
	assert.Equal(t, 1, p.CycleEvery)
	assert.Equal(t, 3, p.RemindDaysBefore)
	assert.Equal(t, []string{"cash", "card", "ePay"}, p.PaymentOptions)
	assert.True(t, p.Active, "not paused")
	assert.True(t, p.UpdatedAt.IsZero(), "no version on create")
}

func TestParseBillFormCarPausedWithVersion(t *testing.T) {
	ver := time.Date(2026, 11, 11, 9, 30, 0, 123456000, time.UTC)
	v := url.Values{
		"category": {"car"}, "typeID": {"3"}, "due_date": {"2027-03-01"},
		"cycle_every": {"6"}, "cycle_unit": {"month"}, "paused": {"on"},
		"version": {ver.Format(time.RFC3339Nano)},
	}
	p, err := parseBillForm(v)
	require.NoError(t, err)
	require.NotNil(t, p.CarExpenseTypeID)
	assert.Equal(t, 3, *p.CarExpenseTypeID)
	assert.Nil(t, p.UtilityTypeID)
	assert.Nil(t, p.ExpectedAmount, "blank amount means unknown")
	assert.False(t, p.Active)
	assert.True(t, ver.Equal(p.UpdatedAt))
	assert.Equal(t, []string{}, p.PaymentOptions, "all options unticked means none, not the cash default")
}

func TestParseBillFormErrors(t *testing.T) {
	base := func() url.Values {
		return url.Values{"category": {"house"}, "typeID": {"4"}, "due_date": {"2026-12-01"}, "cycle_every": {"1"}, "cycle_unit": {"month"}}
	}
	cases := map[string]func(url.Values){
		"bad date":   func(v url.Values) { v.Set("due_date", "tomorrow") },
		"bad amount": func(v url.Values) { v.Set("expected_amount", "ten") },
		"bad every":  func(v url.Values) { v.Set("cycle_every", "x") },
		"bad type":   func(v url.Values) { v.Set("typeID", "") },
	}
	for name, mutate := range cases {
		t.Run(name, func(t *testing.T) {
			v := base()
			mutate(v)
			_, err := parseBillForm(v)
			assert.Error(t, err)
		})
	}
}

func TestParseBillFormReminderSchedule(t *testing.T) {
	v := url.Values{
		"category": {"house"}, "typeID": {"3"}, "due_date": {"2026-12-08"}, "cycle_every": {"1"}, "cycle_unit": {"month"},
		"date_kind": {"arrives"}, "remind_days_after": {"1"},
		"keep_reminding": {"on"}, "repeat_every_days": {"3"}, "max_reminders": {"5"},
		"snoozed_until": {"2026-12-12"},
	}
	p, err := parseBillForm(v)
	require.NoError(t, err)
	assert.Equal(t, models.DateArrives, p.DateKind)
	assert.Equal(t, 1, p.RemindDaysAfter)
	require.NotNil(t, p.RepeatEveryDays)
	assert.Equal(t, 3, *p.RepeatEveryDays)
	require.NotNil(t, p.MaxReminders)
	assert.Equal(t, 5, *p.MaxReminders)
	require.NotNil(t, p.SnoozedUntil)
	assert.Equal(t, time.Date(2026, 12, 12, 0, 0, 0, 0, time.UTC), *p.SnoozedUntil)

	v.Del("keep_reminding")
	v.Set("max_reminders", "")
	v.Set("snoozed_until", "")
	p, err = parseBillForm(v)
	require.NoError(t, err)
	assert.Nil(t, p.RepeatEveryDays, "remind once")
	assert.Nil(t, p.MaxReminders, "no limit")
	assert.Nil(t, p.SnoozedUntil)

	v.Set("max_reminders", "lots")
	_, err = parseBillForm(v)
	assert.Error(t, err)
}

func TestParseBillFormDelivery(t *testing.T) {
	v := url.Values{"category": {"house"}, "typeID": {"4"}, "due_date": {"2026-12-01"}, "cycle_every": {"1"}, "cycle_unit": {"month"},
		"notify": {"email"}, "notify_email": {" partner@example.com "}}
	p, err := parseBillForm(v)
	require.NoError(t, err)
	assert.Equal(t, models.NotifyEmail, p.Notify)
	assert.Equal(t, " partner@example.com ", p.NotifyEmail, "trimmed by the model")

	v.Set("notify", "none")
	p, err = parseBillForm(v)
	require.NoError(t, err)
	assert.Equal(t, models.NotifyNone, p.Notify)
}
