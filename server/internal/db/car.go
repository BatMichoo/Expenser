package database

import (
	"context"
	"database/sql"
	"errors"
	"expenser/internal/models"
	"fmt"
	"time"

	"github.com/google/uuid"
)

// GetCarExpenseTypes lists the car expense types in ID order.
func (db *DB) GetCarExpenseTypes(ctx context.Context) ([]models.CarExpenseType, error) {
	types, err := queryAll(ctx, db.conn, func(r rowScanner, t *models.CarExpenseType) error {
		return r.Scan(&t.ID, &t.Name)
	}, `SELECT id, name FROM car_expense_types ORDER BY id`)
	if err != nil {
		return nil, fmt.Errorf("failed to fetch car expense types: %w", err)
	}
	return types, nil
}

// GetCarExpenseByID returns the user's car expense with the given ID,
// or nil when no such expense belongs to the user.
func (db *DB) GetCarExpenseByID(ctx context.Context, id int, userID uuid.UUID) (*models.CarExpense, error) {
	query := `
		SELECT
			ce.id,
			ce.car_expense_type_id,
			ct.name AS type,
			ce.amount,
			ce.discount_amount,
			ce.expense_date,
			ce.notes,
			ce.metadata,
			ce.created_at,
			ce.created_by
		FROM
			car_expenses ce
		JOIN
			car_expense_types ct ON ce.car_expense_type_id = ct.id
		WHERE
			ce.id = $1 AND ce.created_by = $2;
	`

	var expense models.CarExpense
	err := db.conn.QueryRowContext(ctx, query,
		id,
		userID,
	).Scan(
		&expense.ID,
		&expense.ExpenseTypeID,
		&expense.Type,
		&expense.Amount,
		&expense.DiscountAmount,
		&expense.Date,
		&expense.Notes,
		&expense.Metadata,
		&expense.CreatedAt,
		&expense.CreatedBy)

	if errors.Is(err, sql.ErrNoRows) {
		return nil, nil
	}
	if err != nil {
		return nil, fmt.Errorf("failed to get car expense: %w", err)
	}

	return &expense, nil
}

// Creates a new entry of a car expense. Automatically handles utility type FK.
func (db *DB) CreateCarExpense(ctx context.Context, input *models.CarExpense) error {
	query := `
		INSERT INTO car_expenses (car_expense_type_id, amount, discount_amount, expense_date, notes, metadata, created_by)
		VALUES ($1, $2, $3, $4, $5, $6, $7)
		RETURNING id, created_at, (SELECT name FROM car_expense_types WHERE id = car_expense_type_id);
	`

	err := db.conn.QueryRowContext(ctx, query,
		input.ExpenseTypeID,
		input.Amount,
		input.DiscountAmount,
		input.Date,
		input.Notes,
		input.Metadata,
		input.CreatedBy,
	).Scan(&input.ID, &input.CreatedAt, &input.Type)

	if err != nil {
		return fmt.Errorf("failed to create car expense: %w", err)
	}

	return nil
}

// GetCarExpensesForMonth returns the user's car expenses dated in the
// month, newest first.
func (db *DB) GetCarExpensesForMonth(ctx context.Context, month time.Month, year int, userID uuid.UUID) ([]models.CarExpense, error) {
	from, to := monthRange(month, year)
	expenses, err := queryAll(ctx, db.conn, func(r rowScanner, e *models.CarExpense) error {
		return r.Scan(&e.ID, &e.ExpenseTypeID, &e.Type, &e.Amount, &e.DiscountAmount,
			&e.Date, &e.Notes, &e.Metadata, &e.CreatedAt)
	}, `
		SELECT ce.id, ce.car_expense_type_id, ct.name, ce.amount, ce.discount_amount, ce.expense_date, ce.notes, ce.metadata, ce.created_at
		FROM car_expenses ce
		JOIN car_expense_types ct ON ce.car_expense_type_id = ct.id
		WHERE ce.created_by = $1 AND ce.expense_date >= $2 AND ce.expense_date < $3
		ORDER BY ce.expense_date DESC, ce.id DESC`, userID, from, to)
	if err != nil {
		return nil, fmt.Errorf("failed to fetch car expenses: %w", err)
	}
	return expenses, nil
}

// EditCarExpense updates the user's car expense. It returns an error wrapping
// sql.ErrNoRows when no such expense belongs to the user.
func (db *DB) EditCarExpense(ctx context.Context, editExpense *models.CarExpense, userID uuid.UUID) error {
	query := `
		UPDATE car_expenses
		SET
			car_expense_type_id = $2,
			amount = $3,
			discount_amount = $4,
			expense_date = $5,
			notes = $6,
			metadata = $7
		WHERE id = $1 AND created_by = $8
		RETURNING (SELECT name FROM car_expense_types WHERE id = $2);
	`
	err := db.conn.QueryRowContext(ctx, query,
		editExpense.ID,
		editExpense.ExpenseTypeID,
		editExpense.Amount,
		editExpense.DiscountAmount,
		editExpense.Date,
		editExpense.Notes,
		editExpense.Metadata,
		userID,
	).Scan(&editExpense.Type)

	if err != nil {
		return fmt.Errorf("error editing car expense: %w", err)
	}

	return nil
}

// DeleteCarExpense deletes the user's car expense. It returns false when no
// such expense belongs to the user.
func (db *DB) DeleteCarExpense(ctx context.Context, id int, userID uuid.UUID) (bool, error) {
	query := `
		DELETE FROM car_expenses
		WHERE id = $1 AND created_by = $2`

	res, err := db.conn.ExecContext(ctx, query,
		id,
		userID,
	)

	if err != nil {
		return false, fmt.Errorf("error deleting car expense: %w", err)
	}
	n, err := res.RowsAffected()
	if err != nil {
		return false, fmt.Errorf("error deleting car expense: %w", err)
	}
	return n > 0, nil
}

// GetCarMonthlyTotals returns net car spending (amount minus discount) per month, keyed "YYYY-MM", for expenses
// dated in [from, to).
func (db *DB) GetCarMonthlyTotals(ctx context.Context, userID uuid.UUID, from, to time.Time) (map[string]float64, error) {
	return db.monthlyTotals(ctx, `
		SELECT to_char(expense_date, 'YYYY-MM'), SUM(amount - discount_amount)
		FROM car_expenses
		WHERE created_by = $1 AND expense_date >= $2 AND expense_date < $3
		GROUP BY 1`, userID, from, to)
}

// GetCarTypeTotals returns net car spending per expense type, keyed by name, for [from, to).
func (db *DB) GetCarTypeTotals(ctx context.Context, userID uuid.UUID, from, to time.Time) (map[string]float64, error) {
	return db.monthlyTotals(ctx, `
		SELECT ct.name, SUM(ce.amount - ce.discount_amount)
		FROM car_expenses ce
		JOIN car_expense_types ct ON ct.id = ce.car_expense_type_id
		WHERE ce.created_by = $1 AND ce.expense_date >= $2 AND ce.expense_date < $3
		GROUP BY ct.name`, userID, from, to)
}

// GetCarSpendHistory returns net car spending per expense type and day for [from, to).
func (db *DB) GetCarSpendHistory(ctx context.Context, userID uuid.UUID, from, to time.Time) ([]models.SpendEntry, error) {
	return db.spendHistory(ctx, `
		SELECT ct.name, to_char(ce.expense_date, 'YYYY-MM-DD'), SUM(ce.amount - ce.discount_amount)
		FROM car_expenses ce
		JOIN car_expense_types ct ON ct.id = ce.car_expense_type_id
		WHERE ce.created_by = $1 AND ce.expense_date >= $2 AND ce.expense_date < $3
		GROUP BY 1, 2`, userID, from, to)
}
