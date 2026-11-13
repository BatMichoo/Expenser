package config

import (
	"bytes"
	"log"
	"maps"
	"os"
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// env is a valid configuration; tests change one key.
func env(overrides map[string]string) func(string) string {
	base := map[string]string{"JWT_SECRET": strings.Repeat("s", 32), "DB_HOST": "db", "DB_NAME": "expenser", "DB_USER": "app"}
	maps.Copy(base, overrides)
	return func(k string) string { return base[k] }
}

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
	cfg, err := LoadConfig(env(map[string]string{"APP_URL": " https://expenser.home.lan/ "}))
	require.NoError(t, err)
	assert.Equal(t, "https://expenser.home.lan", cfg.AppURL, "trimmed, no trailing slash")
}

func TestReceiptAIConfig(t *testing.T) {
	cfg, err := LoadConfig(env(map[string]string{"GEMINI_API_KEY": "key-from-env", "GEMINI_MODEL": "ignored"}))
	require.NoError(t, err)
	assert.Equal(t, ReceiptAI{APIKey: "key-from-env", Model: ReceiptAIModel, Fallbacks: ReceiptAIFallbacks}, cfg.ReceiptAI, "the models come from code, not the environment")
	assert.Equal(t, append([]string{ReceiptAIModel}, ReceiptAIFallbacks...), cfg.ReceiptAI.Models(), "main model first")
}

func TestLoadConfigDefaults(t *testing.T) {
	cfg, err := LoadConfig(env(nil))
	require.NoError(t, err)
	assert.Equal(t, "8080", cfg.Port)
	assert.Equal(t, "", cfg.LanDomain, "host-only cookies")
	assert.Equal(t, 24*time.Hour, cfg.JWT.TokenExpiration)
	assert.Equal(t, "5432", cfg.DB.Port)
	assert.Contains(t, cfg.DB.ConnString(), "@db:5432/expenser?")
	assert.Contains(t, cfg.DB.WithName("expenser_test").ConnString(), "/expenser_test?")
}

func TestLoadConfigRejectsMissingOrShortJWTSecret(t *testing.T) {
	_, err := LoadConfig(env(map[string]string{"JWT_SECRET": ""}))
	require.Error(t, err)
	assert.Contains(t, err.Error(), "JWT_SECRET is required")

	_, err = LoadConfig(env(map[string]string{"JWT_SECRET": "short"}))
	require.Error(t, err)
	assert.Contains(t, err.Error(), "JWT_SECRET must be at least 32 bytes (got 5)")
}

func TestJWTExpiration(t *testing.T) {
	tests := []struct {
		in      string
		want    time.Duration
		wantErr string
	}{
		{"", 24 * time.Hour, ""},
		{"48", 48 * time.Hour, ""},
		{"abc", 0, `JWT_EXPIRATION_HOURS "abc": must be a whole number of hours above 0`},
		{"0", 0, `JWT_EXPIRATION_HOURS "0": must be a whole number of hours above 0`},
		{"-3", 0, `JWT_EXPIRATION_HOURS "-3": must be a whole number of hours above 0`},
	}
	for _, tt := range tests {
		t.Run(tt.in, func(t *testing.T) {
			cfg, err := LoadConfig(env(map[string]string{"JWT_EXPIRATION_HOURS": tt.in}))
			if tt.wantErr != "" {
				require.Error(t, err)
				assert.Contains(t, err.Error(), tt.wantErr)
				return
			}
			require.NoError(t, err)
			assert.Equal(t, tt.want, cfg.JWT.TokenExpiration)
		})
	}
}

func TestLoadConfigRequiresDB(t *testing.T) {
	_, err := LoadConfig(env(map[string]string{"DB_HOST": "", "DB_USER": ""}))
	require.Error(t, err)
	assert.Contains(t, err.Error(), "DB_HOST is required")
	assert.Contains(t, err.Error(), "DB_USER is required")
	assert.NotContains(t, err.Error(), "DB_NAME is required")
}

func TestServerPortIsIgnoredWithWarning(t *testing.T) {
	var buf bytes.Buffer
	log.SetOutput(&buf)
	t.Cleanup(func() { log.SetOutput(os.Stderr) })

	cfg, err := LoadConfig(env(map[string]string{"SERVER_PORT": "9000"}))
	require.NoError(t, err)
	assert.Equal(t, "8080", cfg.Port)
	assert.Contains(t, buf.String(), "SERVER_PORT is no longer read")
}

func TestAppPort(t *testing.T) {
	cfg, err := LoadConfig(env(map[string]string{"APP_PORT": "9090"}))
	require.NoError(t, err)
	assert.Equal(t, "9090", cfg.Port)
}

func TestConnString(t *testing.T) {
	d := DB{User: "app", Password: "p@ss/w:rd", Host: "db", Port: "5432", Name: "expenser", SSLMode: "disable", TimeZone: "UTC"}
	assert.Equal(t, "postgres://app:p%40ss%2Fw%3Ard@db:5432/expenser?sslmode=disable&timezone=UTC", d.ConnString())

	d = DB{User: "app", Password: "secret", Host: "db", Port: "5432", Name: "expenser", SSLMode: "require", TimeZone: "Europe/Sofia"}
	assert.Equal(t, "postgres://app:secret@db:5432/expenser?sslmode=require&timezone=Europe%2FSofia", d.ConnString())
}

func TestRedactConnStringHidesPassword(t *testing.T) {
	conn := DB{User: "app", Password: "hunter2", Host: "db", Port: "5432", Name: "expenser", SSLMode: "disable", TimeZone: "UTC"}.ConnString()
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
