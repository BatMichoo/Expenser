package database

import (
	"context"
	"fmt"
	"time"

	"expenser/internal/models"

	"github.com/google/uuid"
)

// ExpenseMonthsOfType returns the months ("YYYY-MM") in [from, to) that
// already have a house or car expense of the given type.
func (db *DB) ExpenseMonthsOfType(ctx context.Context, userID uuid.UUID, section models.Section, typeID int, from, to time.Time) (map[string]bool, error) {
	t, err := expenseTableFor(section)
	if err != nil {
		return nil, err
	}
	query := fmt.Sprintf(`SELECT DISTINCT to_char(expense_date, 'YYYY-MM') FROM %s
		WHERE created_by = $1 AND %s = $2 AND expense_date >= $3 AND expense_date < $4`, t.Table, t.TypeColumn)
	rows, err := db.conn.QueryContext(ctx, query, userID, typeID, from, to)
	if err != nil {
		return nil, fmt.Errorf("failed to get expense months: %w", err)
	}
	defer rows.Close()
	months := map[string]bool{}
	for rows.Next() {
		var m string
		if err := rows.Scan(&m); err != nil {
			return nil, fmt.Errorf("failed to read expense months: %w", err)
		}
		months[m] = true
	}
	return months, rows.Err()
}

// CreateBackfillExpenses adds the same house or car expense on each date,
// all or nothing.
func (db *DB) CreateBackfillExpenses(ctx context.Context, e models.BackfillExpense, dates []time.Time) error {
	t, err := expenseTableFor(e.Section)
	if err != nil {
		return err
	}
	query := fmt.Sprintf(`INSERT INTO %s (%s, amount, discount_amount, expense_date, notes, metadata, created_by)
		VALUES ($1, $2, $3, $4, $5, '{}', $6)`, t.Table, t.TypeColumn)
	tx, err := db.conn.BeginTx(ctx, nil)
	if err != nil {
		return fmt.Errorf("failed to backfill: %w", err)
	}
	defer tx.Rollback()
	stmt, err := tx.PrepareContext(ctx, query)
	if err != nil {
		return fmt.Errorf("failed to backfill: %w", err)
	}
	defer stmt.Close()
	for _, d := range dates {
		if _, err := stmt.ExecContext(ctx, e.TypeID, e.Amount, e.Discount, d, e.Notes, e.UserID); err != nil {
			return fmt.Errorf("failed to backfill %s: %w", d.Format("2006-01-02"), err)
		}
	}
	if err := tx.Commit(); err != nil {
		return fmt.Errorf("failed to backfill: %w", err)
	}
	return nil
}
