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
func (db *DB) ExpenseMonthsOfType(ctx context.Context, userID uuid.UUID, category models.RecurringPaymentCategory, typeID int, from, to time.Time) (map[string]bool, error) {
	query := `SELECT DISTINCT to_char(expense_date, 'YYYY-MM') FROM home_expenses
		WHERE created_by = $1 AND utility_type_id = $2 AND expense_date >= $3 AND expense_date < $4`
	if category == models.RecurringCategoryCar {
		query = `SELECT DISTINCT to_char(expense_date, 'YYYY-MM') FROM car_expenses
		WHERE created_by = $1 AND car_expense_type_id = $2 AND expense_date >= $3 AND expense_date < $4`
	}
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
	query := `INSERT INTO home_expenses (utility_type_id, amount, discount_amount, expense_date, notes, metadata, created_by)
		VALUES ($1, $2, $3, $4, $5, '{}', $6)`
	if e.Category == models.RecurringCategoryCar {
		query = `INSERT INTO car_expenses (car_expense_type_id, amount, discount_amount, expense_date, notes, metadata, created_by)
		VALUES ($1, $2, $3, $4, $5, '{}', $6)`
	}
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
