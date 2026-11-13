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
	// loc is the household's time zone, the one the database session uses.
	// Grocery purchase times are instants, so their day and month bounds
	// are local midnights (see localDay).
	loc *time.Location
}

// InitDatabase connects, applies the migrations and, when configured, the
// dev seed data. Every failure is returned, so the caller decides how to
// stop.
func InitDatabase(cfg *config.Config) (*DB, error) {
	log.Printf("Connecting to %s.", config.RedactConnString(cfg.DB.DBConnString))

	db, err := connect(cfg.DB.DBConnString, cfg.Location)
	if err != nil {
		return nil, fmt.Errorf("connect: %w", err)
	}
	fail := func(step string, err error) (*DB, error) {
		_ = db.Close()
		return nil, fmt.Errorf("%s: %w", step, err)
	}

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	if err := db.Ping(ctx); err != nil {
		return fail("ping", err)
	}
	log.Println("Connected to the database.")

	if err := migrate(db.conn); err != nil {
		return fail("apply migrations", err)
	}
	log.Println("Migrations applied.")

	if cfg.SeedDevData {
		if err := seedDevData(db); err != nil {
			return fail("seed dev data", err)
		}
		log.Println("Dev seed data applied.")
	}
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
func connect(dbConnString string, loc *time.Location) (*DB, error) {
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

	if loc == nil {
		loc = time.Local
	}
	return &DB{conn: conn, loc: loc}, nil
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

// localDay is the local midnight of the calendar day t stands for. Calendar
// days are midnight UTC (see models.Date); a timestamptz column holding
// instants, like groceries_expenses.purchase_date, needs the household's
// midnight instead, or purchases just after midnight land in the day before.
func (db *DB) localDay(t time.Time) time.Time {
	t = t.UTC()
	return time.Date(t.Year(), t.Month(), t.Day(), 0, 0, 0, 0, db.loc)
}

// localRange is localDay for both ends of a [from, to) range.
func (db *DB) localRange(from, to time.Time) (time.Time, time.Time) {
	return db.localDay(from), db.localDay(to)
}

// monthRange is [first day of the month, first day of the next) as calendar
// days. Comparing a column with a range, unlike EXTRACT(MONTH ...), lets
// the (owner, date) indexes serve the query.
func monthRange(month time.Month, year int) (time.Time, time.Time) {
	from := models.Date(year, month, 1)
	return from, from.AddDate(0, 1, 0)
}

// rowScanner is a *sql.Row or *sql.Rows.
type rowScanner interface {
	Scan(dest ...any) error
}

// queryAll runs query and reads every row with scan. It always closes the
// rows and reports an error that ended the iteration early.
func queryAll[T any](ctx context.Context, conn *sql.DB, scan func(rowScanner, *T) error, query string, args ...any) ([]T, error) {
	rows, err := conn.QueryContext(ctx, query, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var out []T
	for rows.Next() {
		var v T
		if err := scan(rows, &v); err != nil {
			return nil, err
		}
		out = append(out, v)
	}
	return out, rows.Err()
}
