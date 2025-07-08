package database

import (
	"database/sql"
	"expenser/internal/config"
	"fmt"
	"log"
	"path/filepath"

	_ "github.com/lib/pq" // Import the PostgreSQL driver
	"github.com/pressly/goose"
)

type DB struct {
	conn *sql.DB
}

func NewDB(conn *sql.DB) *DB {
	db := &DB{conn: conn}
	return db
}

func InitDatabase(cfg *config.Config) (*DB, error) {
	log.Printf("Connecting to %s.", cfg.DB.DBConnString)

	db, err := connect(cfg.DB.DBConnString)
	if err != nil {
		return nil, fmt.Errorf("connect error: %w", err)
	}

	var migrationsDir string
	if cfg.Mode == "" {
		migrationsDir = filepath.Join(config.GetProjectRootDir(), "internal", "db", "migrations")
	} else {
		migrationsDir = filepath.Join("internal", "db", "migrations")
	}

	if err := goose.Up(db.conn, migrationsDir); err != nil {
		log.Fatalf("Failed to apply migrations: %v.", err)
	}
	fmt.Println("Migrations applied successfully.")

	if err := db.Ping(); err != nil {
		db.Close()
		return nil, fmt.Errorf("ping error: %w", err)
	}
	fmt.Println("Connected to the database!")

	return db, nil
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
