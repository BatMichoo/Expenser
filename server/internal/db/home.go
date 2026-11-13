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

// GetHouseUtilityTypes lists the house utility types in ID order.
func (db *DB) GetHouseUtilityTypes(ctx context.Context) ([]models.HomeUtilityType, error) {
	types, err := queryAll(ctx, db.conn, func(r rowScanner, t *models.HomeUtilityType) error {
		return r.Scan(&t.ID, &t.Name)
	}, `SELECT id, name FROM utility_types ORDER BY id`)
	if err != nil {
		return nil, fmt.Errorf("failed to fetch utility types: %w", err)
	}
	return types, nil
}

// GetHouseExpenseByID returns the user's home expense with the given ID,
// or nil when no such expense belongs to the user.
func (db *DB) GetHouseExpenseByID(ctx context.Context, id int, userID uuid.UUID) (*models.HouseExpense, error) {
	query := `
		SELECT
			he.id,
			he.utility_type_id,
			ut.name AS utility_name,
			he.amount,
			he.discount_amount,
			he.expense_date,
			he.notes,
			he.metadata,
			he.created_at
		FROM
			home_expenses he
		JOIN
			utility_types ut ON he.utility_type_id = ut.id
		WHERE
			he.id = $1 AND he.created_by = $2;
	`

	var expense models.HouseExpense
	err := db.conn.QueryRowContext(ctx, query,
		id,
		userID,
	).Scan(
		&expense.ID,
		&expense.UtilityTypeID,
		&expense.UtilityType,
		&expense.Amount,
		&expense.DiscountAmount,
		&expense.ExpenseDate,
		&expense.Notes,
		&expense.Metadata,
		&expense.CreatedAt)

	if errors.Is(err, sql.ErrNoRows) {
		return nil, nil
	}
	if err != nil {
		return nil, fmt.Errorf("failed to get home expense: %w", err)
	}

	return &expense, nil
}

// CreateHouseExpense stores a house expense and fills in its ID, creation
// time, type name and the amounts as stored (rounded to the cent).
func (db *DB) CreateHouseExpense(ctx context.Context, input *models.HouseExpense) error {
	query := `
		INSERT INTO home_expenses (utility_type_id, amount, discount_amount, expense_date, notes, metadata, created_by)
		VALUES ($1, $2, $3, $4, $5, $6, $7)
		RETURNING id, created_at, (SELECT name FROM utility_types WHERE id = utility_type_id), amount::float8, discount_amount::float8;
	`

	err := db.conn.QueryRowContext(ctx, query,
		input.UtilityTypeID,
		input.Amount,
		input.DiscountAmount,
		input.ExpenseDate,
		input.Notes,
		input.Metadata,
		input.CreatedBy,
	).Scan(&input.ID, &input.CreatedAt, &input.UtilityType, &input.Amount, &input.DiscountAmount)

	if err != nil {
		return fmt.Errorf("failed to create home expense: %w", err)
	}

	return nil
}

// GetHouseExpensesForMonth returns the user's house expenses dated in the
// month, newest first.
func (db *DB) GetHouseExpensesForMonth(ctx context.Context, month time.Month, year int, userID uuid.UUID) ([]models.HouseExpense, error) {
	from, to := monthRange(month, year)
	expenses, err := queryAll(ctx, db.conn, func(r rowScanner, e *models.HouseExpense) error {
		return r.Scan(&e.ID, &e.UtilityTypeID, &e.UtilityType, &e.Amount, &e.DiscountAmount,
			&e.ExpenseDate, &e.Notes, &e.Metadata, &e.CreatedAt, &e.CreatedBy)
	}, `
		SELECT he.id, he.utility_type_id, ut.name, he.amount, he.discount_amount, he.expense_date, he.notes, he.metadata, he.created_at, he.created_by
		FROM home_expenses he
		JOIN utility_types ut ON he.utility_type_id = ut.id
		WHERE he.created_by = $1 AND he.expense_date >= $2 AND he.expense_date < $3
		ORDER BY he.expense_date DESC, he.id DESC`, userID, from, to)
	if err != nil {
		return nil, fmt.Errorf("failed to fetch house expenses: %w", err)
	}
	return expenses, nil
}

// EditHouseExpense updates the user's home expense. It returns an error
// wrapping sql.ErrNoRows when no such expense belongs to the user.
func (db *DB) EditHouseExpense(ctx context.Context, editExpense *models.HouseExpense, userID uuid.UUID) error {
	query := `
		UPDATE home_expenses
		SET
			utility_type_id = $2,
			amount = $3,
			discount_amount = $4,
			expense_date = $5,
			notes = $6,
			metadata = $7
		WHERE id = $1 AND created_by = $8
		RETURNING (SELECT name FROM utility_types WHERE id = $2), amount::float8, discount_amount::float8;
	`
	err := db.conn.QueryRowContext(ctx, query,
		editExpense.ID,
		editExpense.UtilityTypeID,
		editExpense.Amount,
		editExpense.DiscountAmount,
		editExpense.ExpenseDate,
		editExpense.Notes,
		editExpense.Metadata,
		userID,
	).Scan(&editExpense.UtilityType, &editExpense.Amount, &editExpense.DiscountAmount)

	if err != nil {
		return fmt.Errorf("error editing expense: %w", err)
	}

	return nil
}

// DeleteHouseExpense deletes the user's home expense. It returns false when
// no such expense belongs to the user.
func (db *DB) DeleteHouseExpense(ctx context.Context, id int, userID uuid.UUID) (bool, error) {
	query := `
		DELETE FROM home_expenses
		WHERE id = $1 AND created_by = $2`

	res, err := db.conn.ExecContext(ctx, query,
		id,
		userID,
	)

	if err != nil {
		return false, fmt.Errorf("error deleting expense: %w", err)
	}
	n, err := res.RowsAffected()
	if err != nil {
		return false, fmt.Errorf("error deleting expense: %w", err)
	}
	return n > 0, nil
}

// GetHouseMonthlyTotals returns net house spending (amount minus discount) per month, keyed "YYYY-MM", for expenses
// dated in [from, to).
func (db *DB) GetHouseMonthlyTotals(ctx context.Context, userID uuid.UUID, from, to time.Time) (map[string]float64, error) {
	return db.monthlyTotals(ctx, `
		SELECT to_char(expense_date, 'YYYY-MM'), SUM(amount - discount_amount)
		FROM home_expenses
		WHERE created_by = $1 AND expense_date >= $2 AND expense_date < $3
		GROUP BY 1`, userID, from, to)
}

// GetHouseTypeTotals returns net house spending per utility type, keyed by name, for [from, to).
func (db *DB) GetHouseTypeTotals(ctx context.Context, userID uuid.UUID, from, to time.Time) (map[string]float64, error) {
	return db.monthlyTotals(ctx, `
		SELECT ut.name, SUM(he.amount - he.discount_amount)
		FROM home_expenses he
		JOIN utility_types ut ON ut.id = he.utility_type_id
		WHERE he.created_by = $1 AND he.expense_date >= $2 AND he.expense_date < $3
		GROUP BY ut.name`, userID, from, to)
}

// GetHouseSpendHistory returns net house spending per utility type and day for [from, to).
func (db *DB) GetHouseSpendHistory(ctx context.Context, userID uuid.UUID, from, to time.Time) ([]models.SpendEntry, error) {
	return db.spendHistory(ctx, `
		SELECT ut.name, to_char(he.expense_date, 'YYYY-MM-DD'), SUM(he.amount - he.discount_amount)
		FROM home_expenses he
		JOIN utility_types ut ON ut.id = he.utility_type_id
		WHERE he.created_by = $1 AND he.expense_date >= $2 AND he.expense_date < $3
		GROUP BY 1, 2`, userID, from, to)
}
