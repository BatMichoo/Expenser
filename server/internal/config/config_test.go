package config

import (
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
)

func TestParseReminderCheckInterval(t *testing.T) {
	tests := []struct {
		in   string
		want time.Duration
	}{
		{"", DefaultReminderCheckInterval},
		{"garbage", DefaultReminderCheckInterval},
		{"-5m", DefaultReminderCheckInterval},
		{"0s", DefaultReminderCheckInterval},
		{"1ns", MinReminderCheckInterval},
		{"1m", MinReminderCheckInterval},
		{"29m59s", MinReminderCheckInterval},
		{"30m", 30 * time.Minute},
		{"45m", 45 * time.Minute},
		{"48h", 48 * time.Hour},
	}
	for _, tt := range tests {
		t.Run(tt.in, func(t *testing.T) {
			assert.Equal(t, tt.want, parseReminderCheckInterval(tt.in))
		})
	}
}

func TestLoadSMTP(t *testing.T) {
	env := map[string]string{}
	get := func(k string) string { return env[k] }

	assert.Equal(t, SMTP{Port: "587"}, loadSMTP(get), "not configured: console reminders")

	env = map[string]string{"SMTP_HOST": "smtp.gmail.com", "SMTP_USER": "me@gmail.com", "SMTP_PASSWORD": "abcd efgh ijkl mnop"}
	s := loadSMTP(get)
	assert.Equal(t, "smtp.gmail.com", s.Host)
	assert.Equal(t, "587", s.Port, "Gmail's STARTTLS port by default")
	assert.Equal(t, "me@gmail.com", s.From, "From defaults to the login")
	assert.Equal(t, "abcdefghijklmnop", s.Password, "Google shows app passwords with spaces")

	env["SMTP_FROM"] = "Expenser <me@gmail.com>"
	env["SMTP_PORT"] = "2525"
	s = loadSMTP(get)
	assert.Equal(t, "Expenser <me@gmail.com>", s.From)
	assert.Equal(t, "2525", s.Port)
}

func TestDevModeIsOffUnlessExplicitlyOn(t *testing.T) {
	assert.False(t, parseDevMode(""))
	assert.False(t, parseDevMode("false"))
	assert.False(t, parseDevMode("yes"))
	assert.True(t, parseDevMode("true"))
	assert.True(t, parseDevMode(" TRUE "))
}

func TestAppURLFromEnv(t *testing.T) {
	t.Setenv("JWT_SECRET", "x")
	t.Setenv("APP_URL", " https://expenser.home.lan/ ")
	cfg, err := LoadConfig()
	if err != nil {
		t.Fatal(err)
	}
	assert.Equal(t, "https://expenser.home.lan", cfg.AppURL, "trimmed, no trailing slash")
}

func TestReceiptAIConfig(t *testing.T) {
	t.Setenv("JWT_SECRET", "x")
	t.Setenv("GEMINI_API_KEY", "key-from-env")
	t.Setenv("GEMINI_MODEL", "ignored")
	cfg, err := LoadConfig()
	if err != nil {
		t.Fatal(err)
	}
	assert.Equal(t, ReceiptAI{APIKey: "key-from-env", Model: ReceiptAIModel, Fallbacks: ReceiptAIFallbacks}, cfg.ReceiptAI, "the models come from code, not the environment")
	assert.Equal(t, append([]string{ReceiptAIModel}, ReceiptAIFallbacks...), cfg.ReceiptAI.Models(), "main model first")
}

func TestConnString(t *testing.T) {
	got := connString("app", "p@ss/w:rd", "db", "5432", "expenser", "", "UTC")
	assert.Equal(t, "postgres://app:p%40ss%2Fw%3Ard@db:5432/expenser?sslmode=disable&timezone=UTC", got)

	got = connString("app", "secret", "db", "5432", "expenser", "require", "Europe/Sofia")
	assert.Equal(t, "postgres://app:secret@db:5432/expenser?sslmode=require&timezone=Europe%2FSofia", got)
}

func TestRedactConnStringHidesPassword(t *testing.T) {
	conn := connString("app", "hunter2", "db", "5432", "expenser", "", "UTC")
	red := RedactConnString(conn)
	assert.NotContains(t, red, "hunter2")
	assert.Contains(t, red, "app:")
	assert.Contains(t, red, "db:5432/expenser")
}

func TestLoadLocation(t *testing.T) {
	loc, err := loadLocation("")
	assert.NoError(t, err)
	assert.Equal(t, DefaultTimezone, loc.String())

	loc, err = loadLocation(" UTC ")
	assert.NoError(t, err)
	assert.Equal(t, "UTC", loc.String())

	_, err = loadLocation("Mars/Olympus")
	assert.Error(t, err)
}
