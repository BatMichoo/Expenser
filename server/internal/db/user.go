package database

import (
	"context"
	"database/sql"
	"errors"
	"expenser/internal/models"
	"fmt"

	"github.com/google/uuid"
)

// EnsureUser makes sure the person the home server sent has a row, and
// keeps their name (for greetings and emails) and language (for emails)
// up to date. It writes only when something changed.
func (db *DB) EnsureUser(ctx context.Context, id uuid.UUID, name, lang string) error {
	_, err := db.conn.ExecContext(ctx, `
		INSERT INTO users (id, display_name, preferred_language)
		VALUES ($1, NULLIF($2, ''), $3)
		ON CONFLICT (id) DO UPDATE SET
			display_name = EXCLUDED.display_name,
			preferred_language = EXCLUDED.preferred_language,
			updated_at = CURRENT_TIMESTAMP
		WHERE users.display_name IS DISTINCT FROM EXCLUDED.display_name
			OR users.preferred_language IS DISTINCT FROM EXCLUDED.preferred_language`,
		id, name, lang)
	if err != nil {
		return fmt.Errorf("failed to ensure user: %w", err)
	}
	return nil
}

// GetUserByID retrieves a user by their ID, with their account settings.
func (db *DB) GetUserByID(ctx context.Context, id uuid.UUID) (*models.User, error) {
	query := `
		SELECT id, COALESCE(username, ''), COALESCE(preferred_language, 'en'), COALESCE(email, ''),
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
		&user.PreferredLanguage,
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
			return nil, models.ErrUserNotFound
		}
		return nil, fmt.Errorf("failed to get user by ID: %w", err)
	}
	d.RepeatEveryDays = nullInt(repeatEvery)
	d.MaxReminders = nullInt(maxReminders)

	return user, nil
}

// UpdateReminderSettings saves the reminder email (empty clears it) and
// the reminder defaults for new bills.
func (db *DB) UpdateReminderSettings(ctx context.Context, id uuid.UUID, email string, d models.ReminderDefaults) error {
	_, err := db.conn.ExecContext(ctx, `
		UPDATE users SET
			email = NULLIF($1, ''),
			default_notify = $2, default_remind_days_before = $3, default_remind_days_after = $4,
			default_repeat_every_days = $5, default_max_reminders = $6,
			updated_at = CURRENT_TIMESTAMP
		WHERE id = $7`,
		email, d.Notify, d.RemindDaysBefore, d.RemindDaysAfter,
		d.RepeatEveryDays, d.MaxReminders, id)
	if err != nil {
		return fmt.Errorf("failed to update reminder settings: %w", err)
	}
	return nil
}
