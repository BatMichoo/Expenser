package database

import (
	"database/sql"
	"expenser/internal/models"
	"fmt"
	"time"

	"github.com/google/uuid"
)

func (db *DB) GetCarExpenseTypes() (*[]models.CarExpenseType, error) {
	query := `
		SELECT * FROM car_expense_types
		`
	var expenseTypes []models.CarExpenseType
	rows, err := db.conn.Query(query)

	if err != nil {
		return nil, fmt.Errorf("failed to fetch car expenses: %v", err)
	}

	for rows.Next() {
		err := rows.Err()
		if err != nil {
			return nil, fmt.Errorf("failed to fetch car expenses: %v", err)
		}

		var expType models.CarExpenseType
		err = rows.Scan(&expType.ID,
			&expType.Name,
		)

		if err != nil {
			return nil, fmt.Errorf("failed to scan expenses: %v", err)
		}
		expenseTypes = append(expenseTypes, expType)
	}

	return &expenseTypes, nil
}

// GetCarExpenseByID returns the user's car expense with the given ID,
// or nil when no such expense belongs to the user.
func (db *DB) GetCarExpenseByID(id int, userID uuid.UUID) (*models.CarExpense, error) {
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
	err := db.conn.QueryRow(query,
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

	if err != nil {
		if err == sql.ErrNoRows {
			return nil, nil
		}
		return nil, fmt.Errorf("failed to get car expense: %w", err)
	}

	return &expense, nil
}

// Creates a new entry of a car expense. Automatically handles utility type FK.
func (db *DB) CreateCarExpense(input *models.CarExpense) error {
	query := `
		INSERT INTO car_expenses (car_expense_type_id, amount, discount_amount, expense_date, notes, metadata, created_by)
		VALUES ($1, $2, $3, $4, $5, $6, $7)
		RETURNING id, created_at, (SELECT name FROM car_expense_types WHERE id = car_expense_type_id);
	`

	err := db.conn.QueryRow(query,
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

func (db *DB) GetCarExpensesForMonth(month time.Month, year int, userId uuid.UUID) (*[]models.CarExpense, error) {
	query := `
		SELECT ce.id, ct.name, ce.amount, ce.discount_amount, ce.expense_date, ce.notes, ce.metadata, ce.created_at
			FROM car_expenses ce
		JOIN
			car_expense_types ct ON ce.car_expense_type_id = ct.id
		WHERE
			EXTRACT(MONTH FROM expense_date) = $1 AND EXTRACT(YEAR FROM expense_date) = $2 AND ce.created_by = $3
		ORDER BY
			ce.expense_date DESC;
	`

	var expenses []models.CarExpense
	rows, err := db.conn.Query(query,
		int(month),
		year,
		userId,
	)

	if err != nil {
		return nil, fmt.Errorf("failed to fetch car expenses: %v", err)
	}

	for rows.Next() {
		err := rows.Err()
		if err != nil {
			return nil, fmt.Errorf("failed to fetch car expenses: %v", err)
		}

		var exp models.CarExpense
		err = rows.Scan(&exp.ID,
			&exp.Type,
			&exp.Amount,
			&exp.DiscountAmount,
			&exp.Date,
			&exp.Notes,
			&exp.Metadata,
			&exp.CreatedAt,
		)

		if err != nil {
			return nil, fmt.Errorf("failed to scan expenses: %v", err)
		}
		expenses = append(expenses, exp)
	}

	return &expenses, nil
}

func (db *DB) GetCarExpensesForYear(year int, userId uuid.UUID) (*[]models.CarExpense, error) {
	query := `
		SELECT ce.id, ct.name, ce.amount, ce.expense_date, ce.notes, ce.metadata, ce.created_at
			FROM car_expenses ce
		JOIN
			car_expense_types ct ON ce.car_expense_type_id = ct.id
		WHERE
			 ($1 = 0 OR EXTRACT(YEAR FROM expense_date) = $1) AND ce.created_by = $2
		ORDER BY 
			ce.expense_date DESC;
	`

	var expenses []models.CarExpense
	rows, err := db.conn.Query(query,
		year,
		userId,
	)

	if err != nil {
		return nil, fmt.Errorf("error fetching expenses: %v", err)
	}

	for rows.Next() {
		err := rows.Err()
		if err != nil {
			return nil, fmt.Errorf("error fetching expenses: %v", err)
		}

		var exp models.CarExpense
		err = rows.Scan(&exp.ID,
			&exp.Type,
			&exp.Amount,
			&exp.Date,
			&exp.Notes,
			&exp.Metadata,
			&exp.CreatedAt,
		)

		if err != nil {
			return nil, fmt.Errorf("error scanning expenses: %v", err)
		}
		expenses = append(expenses, exp)
	}

	return &expenses, nil
}

// EditCarExpense updates the user's car expense. It returns an error wrapping
// sql.ErrNoRows when no such expense belongs to the user.
func (db *DB) EditCarExpense(editExpense *models.CarExpense, userID uuid.UUID) error {
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
	err := db.conn.QueryRow(query,
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
func (db *DB) DeleteCarExpense(id int, userID uuid.UUID) (bool, error) {
	query := `
		DELETE FROM car_expenses
		WHERE id = $1 AND created_by = $2`

	res, err := db.conn.Exec(query,
		id,
		userID,
	)

	if err != nil {
		return false, fmt.Errorf("error deleting car expense: %v", err)
	}

	rowCount, err := res.RowsAffected()
	if err != nil {
		return false, fmt.Errorf("error deleting car expense: %v", err)
	}

	if rowCount < 1 {
		return false, nil
	}

	return true, nil
}

func (db *DB) GetCarExpenseTypeForYear(utility, year int, userId uuid.UUID) (*[]models.CarExpense, error) {
	query := `
		SELECT ce.id, ct.name, ce.amount, ce.expense_date, ce.metadata FROM car_expenses ce
		JOIN car_expense_types ct ON ce.car_expense_type_id = ct.id
		WHERE ce.car_expense_type_id = $1 AND ce.created_by = $3 AND ($2 = 0 OR EXTRACT(YEAR FROM ce.expense_date) = $2)
	`

	var expenses []models.CarExpense
	rows, err := db.conn.Query(query,
		utility,
		year,
		userId,
	)

	if err != nil {
		return nil, fmt.Errorf("error fetching expenses: %v", err)
	}

	for rows.Next() {
		err := rows.Err()
		if err != nil {
			return nil, fmt.Errorf("error fetching expenses: %v", err)
		}

		var exp models.CarExpense
		err = rows.Scan(&exp.ID,
			&exp.Type,
			&exp.Amount,
			&exp.Date,
			&exp.Metadata,
		)

		if err != nil {
			return nil, fmt.Errorf("error scanning expenses: %v", err)
		}
		expenses = append(expenses, exp)
	}

	return &expenses, nil
}

// GetCarMonthlyTotals returns car spending per month, keyed "YYYY-MM", for expenses
// dated in [from, to).
func (db *DB) GetCarMonthlyTotals(userID uuid.UUID, from, to time.Time) (map[string]float64, error) {
	return db.monthlyTotals(`
		SELECT to_char(expense_date, 'YYYY-MM'), SUM(amount)
		FROM car_expenses
		WHERE created_by = $1 AND expense_date >= $2 AND expense_date < $3
		GROUP BY 1`, userID, from, to)
}

// GetCarTypeTotals returns car spending per expense type, keyed by name, for [from, to).
func (db *DB) GetCarTypeTotals(userID uuid.UUID, from, to time.Time) (map[string]float64, error) {
	return db.monthlyTotals(`
		SELECT ct.name, SUM(ce.amount)
		FROM car_expenses ce
		JOIN car_expense_types ct ON ct.id = ce.car_expense_type_id
		WHERE ce.created_by = $1 AND ce.expense_date >= $2 AND ce.expense_date < $3
		GROUP BY ct.name`, userID, from, to)
}
