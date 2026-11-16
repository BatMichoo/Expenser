package models

import (
	"database/sql"
	"fmt"
	"strings"
	"time"
	"unicode"
	"unicode/utf8"

	"github.com/google/uuid"
)

// ErrUserNotFound means no user has the ID or username. It wraps
// sql.ErrNoRows, so handlers that don't check for it show a 404.
var ErrUserNotFound = fmt.Errorf("user not found: %w", sql.ErrNoRows)

// User is a person using Expenser. Logins are the home server's: the id
// is its user id, and DisplayName is the name it sends (X-User-Name).
type User struct {
	ID uuid.UUID
	// Username is the old Expenser username of accounts from before the
	// home server; "" for everyone else.
	Username          string
	PreferredLanguage string
	Email             string // optional preferred email for reminders
	// DisplayName is the name greetings and emails use; empty means the
	// username.
	DisplayName string
	// ReminderDefaults pre-fill the reminder settings of new bills.
	ReminderDefaults ReminderDefaults
	CreatedAt        time.Time
	UpdatedAt        time.Time
}

// MaxDisplayNameLength is the most characters a display name keeps.
const MaxDisplayNameLength = 60

// CleanDisplayName makes the name the home server sent safe for email
// headers: control characters (line breaks above all) removed, spaces
// trimmed, at most 60 characters.
func CleanDisplayName(s string) string {
	s = strings.Map(func(r rune) rune {
		if unicode.IsControl(r) {
			return -1
		}
		return r
	}, s)
	s = strings.TrimSpace(s)
	if utf8.RuneCountInString(s) > MaxDisplayNameLength {
		s = strings.TrimSpace(string([]rune(s)[:MaxDisplayNameLength]))
	}
	return s
}

// ReminderDefaults are a user's starting reminder settings for new bills.
// Changing them never touches existing bills.
type ReminderDefaults struct {
	Notify           NotifyType
	RemindDaysBefore int  // pay-by bills: first reminder N days before
	RemindDaysAfter  int  // arrives-around bills: first reminder N days after
	RepeatEveryDays  *int // nil: remind once
	MaxReminders     *int // nil: no limit
}

// DefaultReminderDefaults are the defaults before a user changes them:
// email, 3 days before, on the day for arrives-around bills, every 3 days,
// no limit.
func DefaultReminderDefaults() ReminderDefaults {
	every := 3
	return ReminderDefaults{Notify: NotifyEmail, RemindDaysBefore: 3, RepeatEveryDays: &every}
}

// Validate applies the same rules as a bill's reminder settings.
func (d ReminderDefaults) Validate() error {
	if d.Notify != NotifyEmail && d.Notify != NotifyNone {
		return ErrInvalidNotify
	}
	if d.RemindDaysBefore < 0 || d.RemindDaysAfter < 0 {
		return ErrInvalidRemindDays
	}
	if d.RepeatEveryDays != nil && *d.RepeatEveryDays < 1 {
		return ErrInvalidRepeat
	}
	if d.MaxReminders != nil && *d.MaxReminders < 1 {
		return ErrInvalidMaxReminders
	}
	return nil
}

// ApplyTo pre-fills a new bill's reminder settings.
func (d ReminderDefaults) ApplyTo(p *RecurringPayment) {
	p.Notify = d.Notify
	p.RemindDaysBefore = d.RemindDaysBefore
	p.RemindDaysAfter = d.RemindDaysAfter
	p.RepeatEveryDays = copyInt(d.RepeatEveryDays)
	p.MaxReminders = copyInt(d.MaxReminders)
}

func copyInt(p *int) *int {
	if p == nil {
		return nil
	}
	v := *p
	return &v
}
