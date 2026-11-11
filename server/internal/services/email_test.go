package services

import (
	"expenser/internal/models"
	"expenser/internal/utilities"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func gasReminder(lang string) Reminder {
	amount := 45.0
	return Reminder{
		To:       "me@example.com",
		DaysLeft: -3,
		Payment: models.RecurringPayment{
			Username: "ivan", Name: "Gas (Overgas)", TypeName: "Gas", Category: models.RecurringCategoryHouse,
			DateKind: models.DateArrives, DueDate: time.Date(2026, 11, 8, 0, 0, 0, 0, time.UTC),
			ExpectedAmount: &amount, PaymentHours: "Mon–Fri 9–18", PaymentOptions: []string{"cash", "ePay"},
			Notes: "Client no. 42", UserLang: lang,
		},
	}
}

func TestBuildReminderEmailEnglish(t *testing.T) {
	require.NoError(t, utilities.InitI18n())
	m := BuildReminderEmail(gasReminder("en"))

	assert.Equal(t, "me@example.com", m.To)
	assert.Equal(t, "Reminder: Gas (Overgas) — waiting 3 days", m.Subject)
	for _, want := range []string{"Gas (Overgas)", "45.00 €", "08.11.2026", "waiting 3 days", "Mon–Fri 9–18", "Cash, ePay", "Client no. 42"} {
		assert.Contains(t, m.Text, want)
		assert.Contains(t, m.HTML, want)
	}
	assert.NotContains(t, m.Text, "http", "no link to the app yet")
}

func TestBuildReminderEmailBulgarianPayBy(t *testing.T) {
	require.NoError(t, utilities.InitI18n())
	r := gasReminder("bg")
	r.Payment.DateKind = models.DatePayBy
	r.Payment.Name = ""
	r.Payment.TypeName = "Internet"
	r.DaysLeft = 2
	m := BuildReminderEmail(r)
	assert.Equal(t, "Напомняне: Интернет — след 2 дни", m.Subject)
	assert.Contains(t, m.Text, "В брой")
}

func TestBuildReminderEmailEscapesHTML(t *testing.T) {
	require.NoError(t, utilities.InitI18n())
	r := gasReminder("en")
	r.Payment.Notes = "<script>alert(1)</script>"
	m := BuildReminderEmail(r)
	assert.NotContains(t, m.HTML, "<script>")
	assert.Contains(t, m.HTML, "&lt;script&gt;")
}

func TestBuildReminderEmailUnknownLanguageFallsBackToEnglish(t *testing.T) {
	require.NoError(t, utilities.InitI18n())
	m := BuildReminderEmail(gasReminder(""))
	assert.Contains(t, m.Subject, "Reminder:")
}
