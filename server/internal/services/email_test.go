package services

import (
	"expenser/internal/models"
	"expenser/internal/utilities"
	"strings"
	"testing"
	"time"

	"github.com/google/uuid"
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

func TestBuildTestReminderEmail(t *testing.T) {
	require.NoError(t, utilities.InitI18n())
	today := time.Date(2026, 11, 11, 0, 0, 0, 0, time.UTC)
	m := BuildTestReminderEmail("ivan", "", "en", "me@example.com", today, nil)

	assert.Equal(t, "me@example.com", m.To)
	assert.Equal(t, "[Test] Reminder: Vivacom Fiber 500 Mbps + TV (apartment & garage) — in 3 days", m.Subject)
	for _, want := range []string{"Hi ivan,", "Internet", "1234.56 €", "14.11.2026", "in 3 days",
		"Mon–Fri 9:00–18:00", "Cash, Card, Bank transfer, Online, ePay.bg", "Client no. 1234567", "Second line"} {
		assert.Contains(t, m.Text, want)
		assert.Contains(t, m.HTML, want)
	}
	assert.Contains(t, m.HTML, "&lt;Vitosha 15&gt;", "HTML-like text is escaped")
	assert.Contains(t, m.HTML, "white-space:pre-line", "line breaks in notes survive in HTML")
	assert.Contains(t, m.HTML, "vertical-align:top", "labels line up with the first line of long values")

	bg := BuildTestReminderEmail("ivan", "", "bg", "me@example.com", today, nil)
	assert.True(t, strings.HasPrefix(bg.Subject, "[Тест] Напомняне:"), bg.Subject)
}

func TestReminderEmailUsesDisplayName(t *testing.T) {
	require.NoError(t, utilities.InitI18n())
	r := gasReminder("en")
	r.Payment.UserEmail = "me@example.com"
	r.Payment.UserDisplayName = "Miro"

	m := BuildReminderEmail(r)
	assert.Contains(t, m.Text, "Hi Miro,")
	assert.Contains(t, m.HTML, "Hi Miro,")
	assert.NotContains(t, m.Text, "ivan")
	assert.Equal(t, "Miro", m.ToName, "own address: named")

	r.To = "partner@example.com"
	m = BuildReminderEmail(r)
	assert.Contains(t, m.Text, "Hi Miro,", "still greets the owner")
	assert.Empty(t, m.ToName, "a partner's address gets no name")

	r.Payment.UserDisplayName = ""
	r.To = "me@example.com"
	m = BuildReminderEmail(r)
	assert.Contains(t, m.Text, "Hi ivan,", "no name: username")
	assert.Empty(t, m.ToName)
}

func TestBuildTestReminderEmailUsesDisplayName(t *testing.T) {
	require.NoError(t, utilities.InitI18n())
	m := BuildTestReminderEmail("ivan", "Miro", "en", "me@example.com", time.Date(2026, 11, 11, 0, 0, 0, 0, time.UTC), nil)
	assert.Contains(t, m.Text, "Hi Miro,")
	assert.Equal(t, "Miro", m.ToName)
}

func TestReminderLinks(t *testing.T) {
	signer := NewSnoozeSigner("secret")
	now := time.Date(2026, 11, 11, 9, 0, 0, 0, time.Local)
	b := NewReminderLinks("https://expenser.home.lan/", signer, func() time.Time { return now })
	r := gasReminder("en")
	r.Payment.ID, r.Payment.UserID = uuid.New(), uuid.New()
	every := 3
	r.Payment.RepeatEveryDays = &every

	links := b.For(r.Payment)
	require.NotNil(t, links)
	assert.Equal(t, "https://expenser.home.lan/bills", links.BillsURL, "trailing slash trimmed")
	require.Len(t, links.Snooze, 3)
	assert.Equal(t, []int{1, 3, 7}, []int{links.Snooze[0].Days, links.Snooze[1].Days, links.Snooze[2].Days})
	for _, s := range links.Snooze {
		token := strings.TrimPrefix(s.URL, "https://expenser.home.lan/snooze/")
		require.NotEqual(t, s.URL, token)
		c, err := signer.Verify(token, now)
		require.NoError(t, err)
		assert.Equal(t, r.Payment.ID, c.BillID)
		assert.Equal(t, "2026-11-08", c.DueFor.Format("2006-01-02"), "tied to this cycle")
		assert.Equal(t, now.AddDate(0, 0, s.Days).Format("2006-01-02"), c.Until.Format("2006-01-02"))
		_, err = signer.Verify(token, now.AddDate(0, 0, 14))
		assert.NoError(t, err, "links last 14 days")
		_, err = signer.Verify(token, now.AddDate(0, 0, 15))
		assert.ErrorIs(t, err, ErrSnoozeLinkExpired)
		assert.Less(t, len(s.URL), len("https://expenser.home.lan/snooze/")+50)
	}

	r.Payment.RepeatEveryDays = nil
	assert.Empty(t, b.For(r.Payment).Snooze, "remind-once bills get no snooze: there is nothing to pause")
	assert.Nil(t, NewReminderLinks("", signer, nil), "no APP_URL, no links")
	assert.Nil(t, NewReminderLinks("ftp://x", signer, nil), "only http(s)")
}

func TestReminderEmailWithLinks(t *testing.T) {
	require.NoError(t, utilities.InitI18n())
	r := gasReminder("en")
	r.Links = &ReminderLinks{BillsURL: "https://app.lan/bills", Snooze: []ReminderSnooze{
		{Days: 1, URL: "https://app.lan/snooze/a"}, {Days: 3, URL: "https://app.lan/snooze/b"}, {Days: 7, URL: "https://app.lan/snooze/c"},
	}}
	m := BuildReminderEmail(r)
	for _, u := range []string{"https://app.lan/bills", "https://app.lan/snooze/a", "https://app.lan/snooze/b", "https://app.lan/snooze/c"} {
		assert.Contains(t, m.HTML, `href="`+u+`"`)
		assert.Contains(t, m.Text, u)
	}
	assert.Contains(t, m.HTML, "Open bills")
	assert.Contains(t, m.HTML, "1 week")
	assert.Contains(t, m.Text, "home network", "explains where links work")

	r.Links.Snooze = nil
	m = BuildReminderEmail(r)
	assert.NotContains(t, m.HTML, "snooze")
	assert.Contains(t, m.HTML, "https://app.lan/bills")
}
