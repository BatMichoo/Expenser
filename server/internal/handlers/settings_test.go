package handlers

import (
	"expenser/internal/models"
	"net/url"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestParseAccountForm(t *testing.T) {
	a, err := parseAccountForm(url.Values{
		"name": {"  Miro "}, "email": {" me@example.com "}, "notify": {"none"},
		"remind_days_before": {"7"}, "remind_days_after": {"1"},
		"keep_reminding": {"on"}, "repeat_every_days": {"2"}, "max_reminders": {"5"},
	})
	require.NoError(t, err)
	assert.Equal(t, "Miro", a.DisplayName)
	assert.Equal(t, "me@example.com", a.Email)
	d := a.Defaults
	assert.Equal(t, models.NotifyNone, d.Notify)
	assert.Equal(t, 7, d.RemindDaysBefore)
	assert.Equal(t, 1, d.RemindDaysAfter)
	require.NotNil(t, d.RepeatEveryDays)
	assert.Equal(t, 2, *d.RepeatEveryDays)
	require.NotNil(t, d.MaxReminders)
	assert.Equal(t, 5, *d.MaxReminders)
}

func TestParseAccountFormOptionalParts(t *testing.T) {
	a, err := parseAccountForm(url.Values{
		"name": {""}, "email": {""}, "notify": {"email"},
		"remind_days_before": {"3"}, "remind_days_after": {"0"},
		"repeat_every_days": {"3"}, "max_reminders": {""},
	})
	require.NoError(t, err)
	assert.Empty(t, a.DisplayName)
	assert.Empty(t, a.Email)
	assert.Nil(t, a.Defaults.RepeatEveryDays, "unchecked: remind once")
	assert.Nil(t, a.Defaults.MaxReminders, "empty: no limit")
}

func TestParseAccountFormErrors(t *testing.T) {
	base := func() url.Values {
		return url.Values{"notify": {"email"}, "remind_days_before": {"3"}, "remind_days_after": {"0"}}
	}
	for name, tc := range map[string]struct {
		key, val string
		want     error
	}{
		"bad email":  {"email", "not-an-email", models.ErrInvalidEmail},
		"long name":  {"name", "123456789012345678901234567890123456789012345678901234567890X", models.ErrInvalidDisplayName},
		"bad notify": {"notify", "sms", models.ErrInvalidNotify},
		"neg before": {"remind_days_before", "-1", models.ErrInvalidRemindDays},
		"text after": {"remind_days_after", "x", models.ErrInvalidRemindDays},
		"max zero":   {"max_reminders", "0", models.ErrInvalidMaxReminders},
	} {
		t.Run(name, func(t *testing.T) {
			v := base()
			v.Set(tc.key, tc.val)
			_, err := parseAccountForm(v)
			assert.ErrorIs(t, err, tc.want)
		})
	}
	v := base()
	v.Set("keep_reminding", "on")
	v.Set("repeat_every_days", "0")
	_, err := parseAccountForm(v)
	assert.ErrorIs(t, err, models.ErrInvalidRepeat)
}

func TestNewBillUsesReminderDefaults(t *testing.T) {
	two := 2
	d := models.ReminderDefaults{Notify: models.NotifyNone, RemindDaysBefore: 7, RemindDaysAfter: 1, RepeatEveryDays: &two}
	now := time.Date(2026, 11, 12, 0, 0, 0, 0, time.UTC)
	p := newBill("car", d, now)
	assert.Equal(t, models.RecurringCategoryCar, p.Category)
	assert.Equal(t, models.NotifyNone, p.Notify)
	assert.Equal(t, 7, p.RemindDaysBefore)
	assert.Equal(t, 1, p.RemindDaysAfter)
	assert.Equal(t, 2, *p.RepeatEveryDays)
	assert.Nil(t, p.MaxReminders)
	assert.Equal(t, models.DatePayBy, p.DateKind)
	assert.Equal(t, time.Date(2026, 12, 12, 0, 0, 0, 0, time.UTC), p.DueDate)
	assert.Equal(t, models.DefaultPaymentOptions(), p.PaymentOptions)
	assert.True(t, p.Active)
}
