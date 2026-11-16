package config

import (
	"errors"
	"fmt"
	"log"
	"net/url"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"time"
)

// ReceiptAIModel is the Gemini model that reads receipts. It lives in code,
// not the environment: changing it means checking the prompt still works.
// A Lite model: fast, cheap and rarely overloaded, while the full Flash
// models often answered 503 or took minutes.
const ReceiptAIModel = "gemini-3.5-flash-lite"

// ReceiptAIFallbacks are tried, in order, while ReceiptAIModel is
// overloaded.
var ReceiptAIFallbacks = []string{"gemini-3.1-flash-lite"}

// ReceiptAI is the Gemini setup for reading receipts.
type ReceiptAI struct {
	APIKey    string   // GEMINI_API_KEY; empty disables receipt reading
	Model     string   // always ReceiptAIModel
	Fallbacks []string // always ReceiptAIFallbacks
}

// Models is the main model followed by the fallbacks.
func (r ReceiptAI) Models() []string {
	return append([]string{r.Model}, r.Fallbacks...)
}

// Config struct to hold application configuration.
type Config struct {
	// Port is where the app listens (APP_PORT, default 8080).
	Port string
	// LanDomain is the home server's domain, e.g. homeserver.lan: the
	// language cookie is set for it, and the auth service is auth.<it>.
	LanDomain string
	// HTTPSPort is where the home server serves HTTPS (HTTPS_PORT, 443).
	HTTPSPort int
	DB        DB
	// LinkSecret signs the snooze links in reminder emails.
	LinkSecret  string
	ReceiptAI   ReceiptAI
	SeedDevData bool
	// ReminderCheckInterval is how often the recurring payment reminder
	// scheduler runs.
	ReminderCheckInterval time.Duration
	// SMTP sends reminder emails; without a host they are only logged.
	SMTP SMTP
	// AppURL is where the app is reached, e.g. https://expenser.home.lan.
	// Reminder emails link to it; empty means no links.
	AppURL string
	// Location is the household's time zone (APP_TIMEZONE, default
	// Europe/Sofia). It decides which day and month a purchase falls in,
	// for the app and the database session alike.
	Location *time.Location
	// DevMode turns on developer-only helpers such as the "Send test email"
	// button. Off unless DEV_MODE=true.
	DevMode bool
}

// parseDevMode is true only for an explicit "true", so production never
// gets dev helpers by accident.
func parseDevMode(v string) bool {
	return strings.EqualFold(strings.TrimSpace(v), "true")
}

// SMTP holds the mail server settings (e.g. Gmail with an app password).
type SMTP struct {
	Host     string
	Port     string
	User     string
	Password string
	From     string
}

// loadSMTP reads SMTP_HOST, SMTP_PORT (default 587), SMTP_USER,
// SMTP_PASSWORD and SMTP_FROM (default: SMTP_USER).
func loadSMTP(get func(string) string) SMTP {
	s := SMTP{
		Host:     strings.TrimSpace(get("SMTP_HOST")),
		Port:     strings.TrimSpace(get("SMTP_PORT")),
		User:     strings.TrimSpace(get("SMTP_USER")),
		Password: strings.ReplaceAll(get("SMTP_PASSWORD"), " ", ""),
		From:     strings.TrimSpace(get("SMTP_FROM")),
	}
	if s.Port == "" {
		s.Port = "587"
	}
	if s.From == "" {
		s.From = s.User
	}
	return s
}

// MinLinkSecretLen is the shortest LINK_SECRET accepted: 256 bits for
// HMAC-SHA256.
const MinLinkSecretLen = 32

// AuthURL is the home server's login service, e.g.
// https://auth.homeserver.lan: the header's Account and Log out go there.
func (c *Config) AuthURL() string {
	u := "https://auth." + c.LanDomain
	if c.HTTPSPort != 443 {
		u += ":" + strconv.Itoa(c.HTTPSPort)
	}
	return u
}

// DB holds the database settings; ConnString builds the URL from them.
type DB struct {
	User, Password, Host, Port, Name, SSLMode, TimeZone string
}

// WithName is a copy pointing at another database on the same server.
func (d DB) WithName(name string) DB {
	d.Name = name
	return d
}

// HomeServerURL is the home server's dashboard, e.g.
// https://homeserver.lan/: the header's "Home ›" leads there. "" without a
// LAN domain (development).
func (c *Config) HomeServerURL() string {
	if c.LanDomain == "" {
		return ""
	}
	u := "https://" + c.LanDomain
	if c.HTTPSPort != 443 {
		u += ":" + strconv.Itoa(c.HTTPSPort)
	}
	return u + "/"
}

// LoadConfig loads the configuration through getenv (normally os.Getenv), so
// tests can pass a map. It does not read .env.dev; see LoadDevEnv. Bad or
// missing required settings are an error, so the app fails at start.
func LoadConfig(getenv func(string) string) (*Config, error) {
	if getenv("SERVER_PORT") != "" {
		log.Printf("SERVER_PORT is no longer read; the app listens on APP_PORT (%s).\n", portOrDefault(getenv("APP_PORT")))
	}

	linkSecret := getenv("LINK_SECRET")
	if linkSecret == "" {
		return nil, fmt.Errorf("LINK_SECRET is required")
	}
	if len(linkSecret) < MinLinkSecretLen {
		return nil, fmt.Errorf("LINK_SECRET must be at least %d bytes (got %d)", MinLinkSecretLen, len(linkSecret))
	}

	httpsPort := 443
	if v := strings.TrimSpace(getenv("HTTPS_PORT")); v != "" {
		p, err := strconv.Atoi(v)
		if err != nil || p < 1 || p > 65535 {
			return nil, fmt.Errorf("HTTPS_PORT %q: must be a port number", v)
		}
		httpsPort = p
	}

	var errs []error
	for _, k := range []string{"DB_HOST", "DB_NAME", "DB_USER"} {
		if getenv(k) == "" {
			errs = append(errs, fmt.Errorf("%s is required", k))
		}
	}
	if err := errors.Join(errs...); err != nil {
		return nil, err
	}

	loc, err := loadLocation(getenv("APP_TIMEZONE"))
	if err != nil {
		return nil, err
	}

	db := DB{
		User:     getenv("DB_USER"),
		Password: getenv("DB_PASS"),
		Host:     getenv("DB_HOST"),
		Port:     getenv("DB_PORT"),
		Name:     getenv("DB_NAME"),
		SSLMode:  getenv("DB_SSLMODE"),
		TimeZone: loc.String(),
	}
	if db.Port == "" {
		db.Port = "5432"
	}
	if db.SSLMode == "" {
		// The database is only reached over the Docker network.
		db.SSLMode = "disable"
	}

	return &Config{
		Port:        portOrDefault(getenv("APP_PORT")),
		LanDomain:   strings.TrimSpace(getenv("LAN_DOMAIN")),
		HTTPSPort:   httpsPort,
		DB:          db,
		LinkSecret:  linkSecret,
		DevMode:     parseDevMode(getenv("DEV_MODE")),
		Location:    loc,
		AppURL:      strings.TrimRight(strings.TrimSpace(getenv("APP_URL")), "/"),
		ReceiptAI:   ReceiptAI{APIKey: getenv("GEMINI_API_KEY"), Model: ReceiptAIModel, Fallbacks: ReceiptAIFallbacks},
		SeedDevData: getenv("SEED_DEV_DATA") == "true",

		ReminderCheckInterval: parseReminderCheckInterval(getenv("REMINDER_CHECK_INTERVAL")),
		SMTP:                  loadSMTP(getenv),
	}, nil
}

// portOrDefault is the APP_PORT value, or 8080 when unset.
func portOrDefault(v string) string {
	if v == "" {
		return "8080"
	}
	return v
}

// DefaultTimezone is the time zone when APP_TIMEZONE is not set.
const DefaultTimezone = "Europe/Sofia"

// loadLocation reads APP_TIMEZONE; an unknown zone is an error rather than
// a silent fallback, since it would move purchases between days.
func loadLocation(name string) (*time.Location, error) {
	name = strings.TrimSpace(name)
	if name == "" {
		name = DefaultTimezone
	}
	loc, err := time.LoadLocation(name)
	if err != nil {
		return nil, fmt.Errorf("APP_TIMEZONE %q: %w", name, err)
	}
	return loc, nil
}

// ConnString builds a postgres URL. The user and password are escaped, so a
// password with characters such as @ or / still works. TimeZone is the
// session time zone, used by EXTRACT and to_char on timestamps.
func (d DB) ConnString() string {
	u := url.URL{
		Scheme:   "postgres",
		User:     url.UserPassword(d.User, d.Password),
		Host:     d.Host + ":" + d.Port,
		Path:     "/" + d.Name,
		RawQuery: url.Values{"sslmode": {d.SSLMode}, "timezone": {d.TimeZone}}.Encode(),
	}
	return u.String()
}

// RedactConnString is the connection string with the password hidden, safe
// to write to the log.
func RedactConnString(conn string) string {
	u, err := url.Parse(conn)
	if err != nil {
		return "(unparsable connection string)"
	}
	return u.Redacted()
}

const (
	DefaultReminderCheckInterval = 24 * time.Hour
	MinReminderCheckInterval     = 30 * time.Minute
)

// parseReminderCheckInterval never fails: empty or invalid values use the
// default, and values below MinReminderCheckInterval are raised to it so a
// typo like "1ns" cannot hammer the database.
func parseReminderCheckInterval(v string) time.Duration {
	if v == "" {
		return DefaultReminderCheckInterval
	}
	d, err := time.ParseDuration(v)
	if err != nil || d <= 0 {
		log.Printf("Invalid REMINDER_CHECK_INTERVAL %q, using default %s\n", v, DefaultReminderCheckInterval)
		return DefaultReminderCheckInterval
	}
	if d < MinReminderCheckInterval {
		log.Printf("REMINDER_CHECK_INTERVAL %q is below the minimum, using %s\n", v, MinReminderCheckInterval)
		return MinReminderCheckInterval
	}
	return d
}

// GetProjectRootDir is used to have consistant way to get the root of our project
// from any other package. Current use cases are when reading files for seeding.
func GetProjectRootDir() string {
	wd, err := os.Getwd()
	if err != nil {
		return ""
	}

	for {
		// Check if the file exists in the current directory
		if _, err := os.Stat(filepath.Join(wd, "go.mod")); err == nil {
			return wd
		}

		// Traverse up the directory tree
		parent := filepath.Dir(wd)
		if parent == wd {
			// We have reached the root directory
			return ""
		}
		wd = parent
	}
}
