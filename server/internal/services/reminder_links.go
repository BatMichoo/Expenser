package services

import (
	"net/url"
	"strings"
	"time"

	"expenser/internal/models"
)

// snoozeChoices are the snooze buttons in a reminder email, in days; and
// snoozeLinkLife is how long those links work.
var snoozeChoices = []int{1, 3, 7}

const snoozeLinkLife = 14 * 24 * time.Hour

// ReminderLinks are the links in one reminder email. They only work where
// APP_URL is reachable, i.e. on the home network.
type ReminderLinks struct {
	BillsURL string
	Snooze   []ReminderSnooze
}

// ReminderSnooze is one "pause reminders for N days" link.
type ReminderSnooze struct {
	Days int
	URL  string
}

// ReminderLinkBuilder makes the links for reminder emails.
type ReminderLinkBuilder struct {
	base   string
	signer *SnoozeSigner
	now    func() time.Time
}

// NewReminderLinks returns nil (no links in emails) unless appURL is an
// http(s) address.
func NewReminderLinks(appURL string, signer *SnoozeSigner, now func() time.Time) *ReminderLinkBuilder {
	appURL = strings.TrimRight(strings.TrimSpace(appURL), "/")
	u, err := url.Parse(appURL)
	if appURL == "" || err != nil || (u.Scheme != "http" && u.Scheme != "https") || u.Host == "" {
		return nil
	}
	if now == nil {
		now = time.Now
	}
	return &ReminderLinkBuilder{base: appURL, signer: signer, now: now}
}

// For returns the links for a reminder about p. Snooze links are only
// offered when the bill keeps reminding: a remind-once bill has nothing to
// pause.
func (b *ReminderLinkBuilder) For(p models.RecurringPayment) *ReminderLinks {
	links := &ReminderLinks{BillsURL: b.base + "/bills"}
	if p.RepeatEveryDays == nil || b.signer == nil {
		return links
	}
	now := b.now()
	today := time.Date(now.Year(), now.Month(), now.Day(), 0, 0, 0, 0, time.UTC)
	for _, days := range snoozeChoices {
		token := b.signer.Sign(SnoozeClaim{
			BillID: p.ID, DueFor: models.DateOnly(p.DueDate),
			Until: today.AddDate(0, 0, days), Expires: now.Add(snoozeLinkLife),
		})
		links.Snooze = append(links.Snooze, ReminderSnooze{Days: days, URL: b.base + "/snooze/" + token})
	}
	return links
}
