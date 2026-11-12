package models

import (
	"errors"
	"strings"
	"time"
	"unicode"
	"unicode/utf8"

	"github.com/google/uuid"
)

// User represents a user in the system
type User struct {
	ID                uuid.UUID
	Username          string `form:"username" binding:"required,min=3,max=50"`
	PasswordHash      string
	PreferredLanguage string
	// TokenVersion goes up when the password changes, which logs out every
	// login issued before.
	TokenVersion int
	Email        string // optional preferred email for reminders
	// DisplayName is the optional name emails greet the user with; empty
	// means the username.
	DisplayName string
	// ReminderDefaults pre-fill the reminder settings of new bills.
	ReminderDefaults ReminderDefaults
	CreatedAt        time.Time
	UpdatedAt        time.Time
}

// UserRegistration represents the data needed for user registration
type UserRegistration struct {
	Username        string `form:"username" binding:"required,min=3,max=50"`
	Password        string `form:"password" binding:"required,min=6"`
	ConfirmPassword string `form:"confirm_password" binding:"required,eqfield=Password"`
}

// UserLogin represents the data needed for user login
type UserLogin struct {
	Username string `form:"username" binding:"required"`
	Password string `form:"password" binding:"required"`
}

// MaxDisplayNameLength is the most characters a display name may have.
const MaxDisplayNameLength = 60

var ErrInvalidDisplayName = errors.New("the name must be at most 60 characters, on one line")

// CleanDisplayName trims a display name and checks it: at most 60
// characters, no line breaks or other control characters (it goes into
// email headers). Empty is allowed.
func CleanDisplayName(s string) (string, error) {
	s = strings.TrimSpace(s)
	if utf8.RuneCountInString(s) > MaxDisplayNameLength {
		return "", ErrInvalidDisplayName
	}
	for _, r := range s {
		if unicode.IsControl(r) {
			return "", ErrInvalidDisplayName
		}
	}
	return s, nil
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

// Password rules: at least 6 characters (as when registering) and at most
// 72 bytes, the most bcrypt can hash.
const (
	MinPasswordLength = 6
	MaxPasswordBytes  = 72
)

var (
	ErrPasswordTooShort  = errors.New("the password must be at least 6 characters")
	ErrPasswordTooLong   = errors.New("the password must be at most 72 bytes")
	ErrPasswordMismatch  = errors.New("the passwords don't match")
	ErrPasswordUnchanged = errors.New("the new password is the same as the current one")
)

// ValidateNewPassword checks a password change before the current password
// is verified.
func ValidateNewPassword(current, next, confirm string) error {
	switch {
	case utf8.RuneCountInString(next) < MinPasswordLength:
		return ErrPasswordTooShort
	case len(next) > MaxPasswordBytes:
		return ErrPasswordTooLong
	case next != confirm:
		return ErrPasswordMismatch
	case next == current:
		return ErrPasswordUnchanged
	}
	return nil
}
