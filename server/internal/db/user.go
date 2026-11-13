package database

import (
	"context"
	"database/sql"
	"errors"
	"expenser/internal/models"
	"fmt"
	"time"

	"github.com/google/uuid"
	"github.com/lib/pq"
)

// ErrUsernameTaken means another user already has the username.
var ErrUsernameTaken = errors.New("username is taken")

// ErrUserNotFound means no user has the ID or username. It wraps
// sql.ErrNoRows, so handlers that don't check for it show a 404.
var ErrUserNotFound = fmt.Errorf("user not found: %w", sql.ErrNoRows)

// uniqueViolation is PostgreSQL's error code for a broken UNIQUE constraint.
const uniqueViolation = "23505"

// CreateUser creates a new user in the database. It returns ErrUsernameTaken
// when the username is in use, even if another registration got there first.
func (db *DB) CreateUser(ctx context.Context, user *models.User) error {
	query := `
		INSERT INTO users (username, password_hash, preferred_language, created_at, updated_at)
		VALUES ($1, $2, $3, $4, $5)
		RETURNING id, created_at, updated_at`

	now := time.Now()
	if user.PreferredLanguage == "" {
		user.PreferredLanguage = "en"
	}
	err := db.conn.QueryRowContext(ctx,
		query,
		user.Username,
		user.PasswordHash,
		user.PreferredLanguage,
		now,
		now,
	).Scan(&user.ID, &user.CreatedAt, &user.UpdatedAt)

	var pqErr *pq.Error
	if errors.As(err, &pqErr) && pqErr.Code == uniqueViolation {
		return ErrUsernameTaken
	}
	if err != nil {
		return fmt.Errorf("failed to create user: %w", err)
	}

	return nil
}

// GetUserByID retrieves a user by their ID, with their account settings.
func (db *DB) GetUserByID(ctx context.Context, id uuid.UUID) (*models.User, error) {
	query := `
		SELECT id, username, password_hash, preferred_language, token_version, COALESCE(email, ''),
			COALESCE(display_name, ''), default_notify, default_remind_days_before,
			default_remind_days_after, default_repeat_every_days, default_max_reminders,
			created_at, updated_at
		FROM users
		WHERE id = $1`

	user := &models.User{}
	var repeatEvery, maxReminders sql.NullInt64
	d := &user.ReminderDefaults
	err := db.conn.QueryRowContext(ctx, query, id).Scan(
		&user.ID,
		&user.Username,
		&user.PasswordHash,
		&user.PreferredLanguage,
		&user.TokenVersion,
		&user.Email,
		&user.DisplayName,
		&d.Notify,
		&d.RemindDaysBefore,
		&d.RemindDaysAfter,
		&repeatEvery,
		&maxReminders,
		&user.CreatedAt,
		&user.UpdatedAt,
	)

	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, ErrUserNotFound
		}
		return nil, fmt.Errorf("failed to get user by ID: %w", err)
	}
	d.RepeatEveryDays = nullInt(repeatEvery)
	d.MaxReminders = nullInt(maxReminders)

	return user, nil
}

// GetUserByUsername retrieves a user by their username
func (db *DB) GetUserByUsername(ctx context.Context, username string) (*models.User, error) {
	query := `
		SELECT id, username, password_hash, preferred_language, token_version, created_at, updated_at
		FROM users
		WHERE username = $1`

	user := &models.User{}
	err := db.conn.QueryRowContext(ctx, query, username).Scan(
		&user.ID,
		&user.Username,
		&user.PasswordHash,
		&user.PreferredLanguage,
		&user.TokenVersion,
		&user.CreatedAt,
		&user.UpdatedAt,
	)

	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, ErrUserNotFound
		}
		return nil, fmt.Errorf("failed to get user by username: %w", err)
	}

	return user, nil
}

// UpdateUserLanguage updates the preferred language for a user
func (db *DB) UpdateUserLanguage(ctx context.Context, userID uuid.UUID, lang string) error {
	query := `UPDATE users SET preferred_language = $1, updated_at = $2 WHERE id = $3`
	_, err := db.conn.ExecContext(ctx, query, lang, time.Now(), userID)
	return err
}

// UserExists checks if a user exists by username or email
func (db *DB) UserExists(ctx context.Context, username string) (bool, error) {
	query := `SELECT EXISTS(SELECT 1 FROM users WHERE username = $1)`

	var exists bool
	err := db.conn.QueryRowContext(ctx, query, username).Scan(&exists)
	if err != nil {
		return false, fmt.Errorf("failed to check if user exists: %w", err)
	}

	return exists, nil
}

// UpdateUserAccount saves the account settings: display name and reminder
// email (empty clears either) and the reminder defaults for new bills.
func (db *DB) UpdateUserAccount(ctx context.Context, id uuid.UUID, displayName, email string, d models.ReminderDefaults) error {
	_, err := db.conn.ExecContext(ctx, `
		UPDATE users SET
			display_name = NULLIF($1, ''), email = NULLIF($2, ''),
			default_notify = $3, default_remind_days_before = $4, default_remind_days_after = $5,
			default_repeat_every_days = $6, default_max_reminders = $7,
			updated_at = CURRENT_TIMESTAMP
		WHERE id = $8`,
		displayName, email, d.Notify, d.RemindDaysBefore, d.RemindDaysAfter,
		d.RepeatEveryDays, d.MaxReminders, id)
	if err != nil {
		return fmt.Errorf("failed to update account: %w", err)
	}
	return nil
}

// TokenVersion is the user's login version: tokens issued for an older one
// are no longer accepted.
func (db *DB) TokenVersion(ctx context.Context, id uuid.UUID) (int, error) {
	var v int
	err := db.conn.QueryRowContext(ctx, `SELECT token_version FROM users WHERE id = $1`, id).Scan(&v)
	return v, err
}

// UpdateUserPassword stores a new password hash and raises the token
// version, so logins issued before stop working.
func (db *DB) UpdateUserPassword(ctx context.Context, id uuid.UUID, hash string) error {
	res, err := db.conn.ExecContext(ctx,
		`UPDATE users SET password_hash = $1, token_version = token_version + 1, updated_at = CURRENT_TIMESTAMP WHERE id = $2`, hash, id)
	if err != nil {
		return fmt.Errorf("failed to update password: %w", err)
	}
	if n, err := res.RowsAffected(); err == nil && n == 0 {
		return ErrUserNotFound
	}
	return nil
}
