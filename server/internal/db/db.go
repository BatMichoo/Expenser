package database

import (
	"database/sql"
	"expenser/internal/config"
	_ "expenser/internal/db/migrations" // registers Go migrations with goose
	"expenser/internal/models"
	"fmt"
	"log"
	"path/filepath"
	"time"

	_ "github.com/lib/pq" // Import the PostgreSQL driver
	"github.com/pressly/goose"
)

type DB struct {
	conn *sql.DB
}

func InitDatabase(cfg *config.Config) (*DB, error) {
	log.Printf("Connecting to %s.", config.RedactConnString(cfg.DB.DBConnString))

	db, err := connect(cfg.DB.DBConnString)
	if err != nil {
		return nil, fmt.Errorf("connect error: %w", err)
	}

	dbDir := filepath.Join("internal", "db")
	if cfg.Mode == "" {
		dbDir = filepath.Join(config.GetProjectRootDir(), dbDir)
	}

	if err := goose.Up(db.conn, filepath.Join(dbDir, "migrations")); err != nil {
		log.Fatalf("Failed to apply migrations: %v.", err)
	}
	fmt.Println("Migrations applied successfully.")

	if cfg.SeedDevData {
		if err := seedDevData(db, filepath.Join(dbDir, "seeds")); err != nil {
			log.Fatalf("Failed to seed dev data: %v.", err)
		}
		fmt.Println("Dev seed data applied successfully.")
	}

	if err := db.Ping(); err != nil {
		if db.Close() != nil {
			return nil, fmt.Errorf("error: %s", err.Error())
		}
		return nil, fmt.Errorf("ping error: %w", err)
	}
	fmt.Println("Connected to the database!")

	return db, nil
}

// seedDevData applies the dev-only seed files. They are tracked in their own
// version table so they never mix with the schema migrations.
func seedDevData(db *DB, seedsDir string) error {
	defaultTable := goose.TableName()
	goose.SetTableName("goose_seed_version")
	defer goose.SetTableName(defaultTable)

	return goose.Up(db.conn, seedsDir)
}

// connect establishes a connection to the PostgreSQL database.
func connect(dbConnString string) (*DB, error) {
	conn, err := sql.Open("postgres", dbConnString)
	if err != nil {
		return nil, err
	}

	db := &DB{conn: conn}
	return db, nil
}

func (db *DB) Ping() error {
	return db.conn.Ping()
}

func (db *DB) Close() error {
	return db.conn.Close()
}

// monthlyTotals runs a query returning (key, sum) rows — a YYYY-MM month or
// a type name — and collects them into a map.
func (db *DB) monthlyTotals(query string, args ...any) (map[string]float64, error) {
	rows, err := db.conn.Query(query, args...)
	if err != nil {
		return nil, fmt.Errorf("failed to query monthly totals: %w", err)
	}
	defer rows.Close()

	totals := map[string]float64{}
	for rows.Next() {
		var month string
		var total sql.NullFloat64
		if err := rows.Scan(&month, &total); err != nil {
			return nil, fmt.Errorf("failed to read monthly totals: %w", err)
		}
		totals[month] = total.Float64
	}
	return totals, rows.Err()
}

// spendHistory runs a query returning (type, YYYY-MM-DD, sum) rows and
// collects them as SpendEntry values dated midnight UTC.
func (db *DB) spendHistory(query string, args ...any) ([]models.SpendEntry, error) {
	rows, err := db.conn.Query(query, args...)
	if err != nil {
		return nil, fmt.Errorf("failed to query spend history: %w", err)
	}
	defer rows.Close()

	var out []models.SpendEntry
	for rows.Next() {
		var e models.SpendEntry
		var date string
		if err := rows.Scan(&e.Type, &date, &e.Amount); err != nil {
			return nil, fmt.Errorf("failed to read spend history: %w", err)
		}
		if e.Date, err = time.Parse("2006-01-02", date); err != nil {
			return nil, fmt.Errorf("failed to parse spend history date %q: %w", date, err)
		}
		out = append(out, e)
	}
	return out, rows.Err()
}
