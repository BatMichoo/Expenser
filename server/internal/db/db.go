package database

import (
	"context"
	"database/sql"
	"expenser/internal/config"
	"expenser/internal/db/migrations" // also registers the Go migrations with goose
	"expenser/internal/db/seeds"
	"expenser/internal/models"
	"fmt"
	"log"
	"time"

	_ "github.com/lib/pq" // Import the PostgreSQL driver
	"github.com/pressly/goose/v3"
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

	if err := migrate(db.conn); err != nil {
		log.Fatalf("Failed to apply migrations: %v.", err)
	}
	fmt.Println("Migrations applied successfully.")

	if cfg.SeedDevData {
		if err := seedDevData(db); err != nil {
			log.Fatalf("Failed to seed dev data: %v.", err)
		}
		fmt.Println("Dev seed data applied successfully.")
	}

	if err := db.Ping(context.Background()); err != nil {
		if db.Close() != nil {
			return nil, fmt.Errorf("error: %s", err.Error())
		}
		return nil, fmt.Errorf("ping error: %w", err)
	}
	fmt.Println("Connected to the database!")

	return db, nil
}

// migrate applies the schema migrations built into the binary. Branches
// merge in any order, so a migration can arrive after a newer one has run;
// it is applied then rather than refused (goose v2 skipped it silently).
func migrate(conn *sql.DB) error {
	goose.SetBaseFS(migrations.SQL)
	defer goose.SetBaseFS(nil)
	return goose.Up(conn, ".", goose.WithAllowMissing())
}

// seedDevData applies the dev-only seed files. They are tracked in their own
// version table so they never mix with the schema migrations.
func seedDevData(db *DB) error {
	defaultTable := goose.TableName()
	goose.SetTableName("goose_seed_version")
	defer goose.SetTableName(defaultTable)
	goose.SetBaseFS(seeds.FS)
	defer goose.SetBaseFS(nil)

	return goose.Up(db.conn, ".")
}

// connect establishes a connection to the PostgreSQL database.
func connect(dbConnString string) (*DB, error) {
	conn, err := sql.Open("postgres", dbConnString)
	if err != nil {
		return nil, err
	}

	// A household app needs few connections; capping them keeps a burst of
	// requests from exhausting Postgres, and recycling them picks up
	// server-side changes (and drops connections a restart left broken).
	conn.SetMaxOpenConns(10)
	conn.SetMaxIdleConns(5)
	conn.SetConnMaxLifetime(30 * time.Minute)
	conn.SetConnMaxIdleTime(5 * time.Minute)

	db := &DB{conn: conn}
	return db, nil
}

func (db *DB) Ping(ctx context.Context) error {
	return db.conn.PingContext(ctx)
}

func (db *DB) Close() error {
	return db.conn.Close()
}

// monthlyTotals runs a query returning (key, sum) rows — a YYYY-MM month or
// a type name — and collects them into a map.
func (db *DB) monthlyTotals(ctx context.Context, query string, args ...any) (map[string]float64, error) {
	rows, err := db.conn.QueryContext(ctx, query, args...)
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
func (db *DB) spendHistory(ctx context.Context, query string, args ...any) ([]models.SpendEntry, error) {
	rows, err := db.conn.QueryContext(ctx, query, args...)
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
