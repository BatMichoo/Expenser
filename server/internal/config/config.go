package config

import (
	"fmt"
	"log"
	"os"
	"path/filepath"
	"strconv"
	"time"

	"github.com/joho/godotenv"
)

// Config struct to hold application configuration.
type Config struct {
	ServerPort   string
	LanDomain    string
	DB           DB
	JWT          JWT
	Mode         string
	GeminiAPIKey string
	SeedDevData  bool
	// ReminderCheckInterval is how often the recurring payment reminder
	// scheduler runs.
	ReminderCheckInterval time.Duration
}

// JWT holds JWT-related configuration
type JWT struct {
	SecretKey       string
	TokenExpiration time.Duration
}

type DB struct {
	DBConnString     string
	TestDBConnString string
	TestDBName       string
}

// LoadConfig loads the configuration from environment variables.
func LoadConfig() (*Config, error) {
	mode := os.Getenv("MODE")

	envPath := filepath.Join(GetProjectRootDir(), ".env.dev")
	if err := godotenv.Load(envPath); err != nil {
		log.Printf("No .env.development file found at %s, using environment variables\n", envPath)
	}

	lanDomain := os.Getenv("LAN_DOMAIN")
	if lanDomain == "" {
		lanDomain = "localhost"
	}

	dbUser := os.Getenv("DB_USER")
	dbPass := os.Getenv("DB_PASS")
	dbHost := os.Getenv("DB_HOST")
	dbPort := os.Getenv("DB_PORT")
	dbName := os.Getenv("DB_NAME")
	serverPort := os.Getenv("SERVER_PORT")
	geminiAPIKey := os.Getenv("GEMINI_API_KEY")
	seedDevData := os.Getenv("SEED_DEV_DATA") == "true"

	var testDBName string
	if dbName != "" {
		testDBName = dbName + "_test"
	}

	// JWT configuration
	jwtSecret := os.Getenv("JWT_SECRET")
	if jwtSecret == "" {
		return nil, fmt.Errorf("JWT_SECRET environment variable is required")
	}

	jwtExpirationStr := os.Getenv("JWT_EXPIRATION_HOURS")
	jwtExpiration := 24 * time.Hour // Default to 24 hours
	if jwtExpirationStr != "" {
		if hours, err := strconv.Atoi(jwtExpirationStr); err == nil {
			jwtExpiration = time.Duration(hours) * time.Hour
		}
	}

	reminderCheckInterval := parseReminderCheckInterval(os.Getenv("REMINDER_CHECK_INTERVAL"))

	dbConnTemplate := "postgres://%s:%s@%s:%s/%s?sslmode=disable"

	// Construct the database connection string.
	dbConnString := fmt.Sprintf(dbConnTemplate,
		dbUser, dbPass, dbHost, dbPort, dbName)

	testDBConnString := fmt.Sprintf(dbConnTemplate,
		dbUser, dbPass, dbHost, dbPort, testDBName)

	DB := DB{
		DBConnString:     dbConnString,
		TestDBConnString: testDBConnString,
		TestDBName:       testDBName,
	}

	return &Config{
		ServerPort: serverPort,
		LanDomain:  lanDomain,
		DB:         DB,
		JWT: JWT{
			SecretKey:       jwtSecret,
			TokenExpiration: jwtExpiration,
		},
		Mode:         mode,
		GeminiAPIKey: geminiAPIKey,
		SeedDevData:  seedDevData,

		ReminderCheckInterval: reminderCheckInterval,
	}, nil
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
