package models

import (
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestDefaultReminderDefaultsMatchNewBills(t *testing.T) {
	d := DefaultReminderDefaults()
	assert.Equal(t, NotifyEmail, d.Notify)
	assert.Equal(t, 3, d.RemindDaysBefore)
	assert.Equal(t, 0, d.RemindDaysAfter)
	if assert.NotNil(t, d.RepeatEveryDays) {
		assert.Equal(t, 3, *d.RepeatEveryDays)
	}
	assert.Nil(t, d.MaxReminders)
	assert.NoError(t, d.Validate())
}

func TestReminderDefaultsValidate(t *testing.T) {
	zero, neg := 0, -1
	for name, tc := range map[string]struct {
		edit func(*ReminderDefaults)
		want error
	}{
		"bad notify":      {func(d *ReminderDefaults) { d.Notify = "sms" }, ErrInvalidNotify},
		"negative before": {func(d *ReminderDefaults) { d.RemindDaysBefore = -1 }, ErrInvalidRemindDays},
		"negative after":  {func(d *ReminderDefaults) { d.RemindDaysAfter = -1 }, ErrInvalidRemindDays},
		"repeat zero":     {func(d *ReminderDefaults) { d.RepeatEveryDays = &zero }, ErrInvalidRepeat},
		"max negative":    {func(d *ReminderDefaults) { d.MaxReminders = &neg }, ErrInvalidMaxReminders},
		"repeat off ok":   {func(d *ReminderDefaults) { d.RepeatEveryDays = nil }, nil},
	} {
		t.Run(name, func(t *testing.T) {
			d := DefaultReminderDefaults()
			tc.edit(&d)
			assert.ErrorIs(t, d.Validate(), tc.want)
			if tc.want == nil {
				assert.NoError(t, d.Validate())
			}
		})
	}
}

func TestReminderDefaultsApplyTo(t *testing.T) {
	two, five := 2, 5
	d := ReminderDefaults{Notify: NotifyNone, RemindDaysBefore: 7, RemindDaysAfter: 1, RepeatEveryDays: &two, MaxReminders: &five}
	p := &RecurringPayment{Name: "Fiber"}
	d.ApplyTo(p)
	assert.Equal(t, NotifyNone, p.Notify)
	assert.Equal(t, 7, p.RemindDaysBefore)
	assert.Equal(t, 1, p.RemindDaysAfter)
	assert.Equal(t, 2, *p.RepeatEveryDays)
	assert.Equal(t, 5, *p.MaxReminders)
	assert.Equal(t, "Fiber", p.Name, "other fields untouched")

	two = 9
	assert.Equal(t, 2, *p.RepeatEveryDays, "bill gets its own copy")
}

func TestCleanDisplayName(t *testing.T) {
	assert.Equal(t, "Miro", CleanDisplayName("  Miro  "))
	assert.Empty(t, CleanDisplayName(""))
	assert.Equal(t, strings.Repeat("я", 60), CleanDisplayName(strings.Repeat("я", 61)), "cut to 60 characters, not bytes")
	assert.Equal(t, "MiroBcc: x@example.com", CleanDisplayName("Miro\r\nBcc: x@example.com"),
		"no line breaks or control characters (email headers)")
}
