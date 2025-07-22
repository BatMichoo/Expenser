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
	ServerPort string
	DB         DB
	JWT        JWT
	Mode       string
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

	envPath := filepath.Join(GetProjectRootDir(), ".env.development")
	if err := godotenv.Load(envPath); err != nil {
		log.Printf("No .env.development file found at %s, using environment variables\n", envPath)
	}

	dbUser := os.Getenv("DB_USER")
	dbPass := os.Getenv("DB_PASS")
	dbHost := os.Getenv("DB_HOST")
	dbPort := os.Getenv("DB_PORT")
	dbName := os.Getenv("DB_NAME")
	serverPort := os.Getenv("SERVER_PORT")

	testDBName := os.Getenv("TEST_DB_NAME")

	// JWT configuration
	jwtSecret := os.Getenv("JWT_SECRET")
	if jwtSecret == "" {
		jwtSecret = "your-secret-key-change-this-in-production"
		log.Println("Warning: Using default JWT secret. Set JWT_SECRET environment variable in production.")
	}

	jwtExpirationStr := os.Getenv("JWT_EXPIRATION_HOURS")
	jwtExpiration := 24 * time.Hour // Default to 24 hours
	if jwtExpirationStr != "" {
		if hours, err := strconv.Atoi(jwtExpirationStr); err == nil {
			jwtExpiration = time.Duration(hours) * time.Hour
		}
	}

	// Construct the database connection string.
	dbConnString := fmt.Sprintf("postgres://%s:%s@%s:%s/%s?sslmode=disable",
		dbUser, dbPass, dbHost, dbPort, dbName)

	testDBConnString := fmt.Sprintf("postgres://%s:%s@%s:%s/%s?sslmode=disable",
		dbUser, dbPass, dbHost, dbPort, testDBName)

	DB := DB{
		DBConnString:     dbConnString,
		TestDBConnString: testDBConnString,
		TestDBName:       testDBName,
	}

	return &Config{
		ServerPort: serverPort,
		DB:         DB,
		JWT: JWT{
			SecretKey:       jwtSecret,
			TokenExpiration: jwtExpiration,
		},
		Mode: mode,
	}, nil
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
