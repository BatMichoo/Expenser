package database

import (
	"context"
	"database/sql"
	"expenser/internal/config"
	"fmt"
	"log"
	"os"
	"testing"

	"github.com/google/uuid"
)

// TestMain loads .env.dev (if present) once for every DB test, since
// config.LoadConfig only reads the environment.
func TestMain(m *testing.M) {
	if _, _, err := config.LoadDevEnv(); err != nil {
		fmt.Fprintf(os.Stderr, "loading .env.dev: %v\n", err)
		os.Exit(1)
	}
	os.Exit(m.Run())
}

// testUser is someone the home server sends: Expenser only knows their id
// (and, on their bills, their name).
type testUser struct {
	ID          uuid.UUID
	DisplayName string
}

var TestUserRegisterModel = &testUser{}

// AddTestUser gives u a new id. There's nothing to create: people are the
// home server's, and Expenser keeps no table of them.
func (db *DB) AddTestUser(_ context.Context, u *testUser) error {
	u.ID = uuid.New()
	return nil
}

func InitTestDB(cfg *config.Config) *DB {
	fmt.Printf("Connecting to test DB: %s\n", config.RedactConnString(cfg.DB.WithName(cfg.DB.Name+"_test").ConnString()))

	testDB, err := connectTestDB(cfg)
	if err != nil {
		panic("Failed to connect to test database: " + err.Error())
	}

	return testDB
}

func ResetTestDB(tdb *DB) {
	_, err := tdb.conn.Exec(`TRUNCATE recurring_payments, home_expenses, car_expenses, groceries_expenses RESTART IDENTITY CASCADE`)
	if err != nil {
		log.Printf("\n Failed to truncate test DB; \n err: %v \n", err)
	}
	log.Println("Successfully reset test DB!")
}

func connectTestDB(cfg *config.Config) (*DB, error) {
	test := cfg.DB.WithName(cfg.DB.Name + "_test")

	adminDB, err := sql.Open("postgres", cfg.DB.ConnString())
	if err != nil {
		return nil, fmt.Errorf("failed to connect to admin DB: %w", err)
	}
	defer adminDB.Close()

	// Check if test DB exists
	var exists bool
	checkQuery := `SELECT EXISTS(SELECT 1 FROM pg_database WHERE datname = $1)`
	err = adminDB.QueryRow(checkQuery, test.Name).Scan(&exists)
	if err != nil {
		return nil, fmt.Errorf("failed to check for test DB: %w", err)
	}

	// Create test DB if it doesn't exist
	if !exists {
		_, err := adminDB.Exec(fmt.Sprintf(`CREATE DATABASE %s OWNER %s`, test.Name, cfg.DB.User))
		if err != nil {
			return nil, fmt.Errorf("failed to create test DB: %w", err)
		}
		_, err = adminDB.Exec(fmt.Sprintf(`ALTER DATABASE %s SET TIMEZONE TO '%s'`, test.Name, cfg.Location.String()))
		if err != nil {
			return nil, fmt.Errorf("failed to set test DB timezone: %w", err)
		}
		fmt.Printf("Created test DB: %s", test.Name)
	} else {
		fmt.Println("Test DB already exists")
	}

	// Apply migrations to test DB
	testDBConn, err := sql.Open("postgres", test.ConnString())
	if err != nil {
		return nil, fmt.Errorf("failed to connect to test DB: %w", err)
	}

	if err := migrate(testDBConn); err != nil {
		return nil, fmt.Errorf("failed to run migrations: %w", err)
	}
	fmt.Println("Migrations applied successfully to test DB.")

	return &DB{conn: testDBConn, loc: cfg.Location}, nil
}
