package database

import (
	"database/sql"
	"expenser/internal/models"
	"fmt"
	"time"
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

func (db *DB) GetTotalCarExpenseForMonth(month time.Month) (float64, error) {
	currentYear := time.Now().Year()
	query := `
		SELECT SUM(amount) FROM car_expenses
		WHERE EXTRACT(MONTH FROM expense_date) = $1 AND EXTRACT(YEAR FROM expense_date) = $2
		`

	var totalAmount sql.NullFloat64
	err := db.conn.QueryRow(query,
		int(month),
		currentYear,
	).Scan(&totalAmount)

	if err != nil {
		if err == sql.ErrNoRows {
			return 0.00, nil
		}
		return 0.00, fmt.Errorf("failed to get total amount: %w", err)
	}

	return totalAmount.Float64, nil
}

func (db *DB) GetHighestCarExpenseForMonth(month time.Month) (float64, string, error) {
	currentYear := time.Now().Year()
	query := `
		SELECT
			ce.amount,
			ct.name
		FROM
			car_expenses ce
		JOIN
			car_expense_types ct ON ce.car_expense_type_id = ct.id
		WHERE
			EXTRACT(MONTH FROM ce.expense_date) = $1 AND EXTRACT(YEAR FROM ce.expense_date) = $2
		ORDER BY
			ce.amount DESC
		LIMIT 1;
	`

	var highestExpense sql.NullFloat64
	var utilType string

	err := db.conn.QueryRow(query,
		int(month),
		currentYear,
	).Scan(&highestExpense, &utilType)

	if err != nil {
		if err == sql.ErrNoRows {
			return 0.00, "", nil
		}
		return 0.00, "", fmt.Errorf("failed to get total amount: %w", err)
	}

	return highestExpense.Float64, utilType, nil
}

// Retrieves home expense by Id, returns error upon failure.
func (db *DB) GetCarExpenseByID(id int) (*models.CarExpense, error) {
	query := `
		SELECT
			ce.id,
			ct.name AS type,
			ce.amount,
			ce.expense_date,
			ce.notes,
			ce.created_at
		FROM
			car_expenses ce
		JOIN
			car_expense_types ct ON ce.car_expense_type_id = ct.id
		WHERE
			ce.id = $1;
	`

	var expense models.CarExpense
	err := db.conn.QueryRow(query,
		id,
	).Scan(
		&expense.ID,
		&expense.Type,
		&expense.Amount,
		&expense.Date,
		&expense.Notes,
		&expense.CreatedAt)

	if err != nil {
		if err == sql.ErrNoRows {
			return nil, nil
		}
		return nil, fmt.Errorf("failed to get car expense: %w\n", err)
	}

	return &expense, nil
}

// Creates a new entry of a home expense. Automatically handles utility type FK.
func (db *DB) CreateCarExpense(input *models.CarExpense) error {
	query := `
		INSERT INTO car_expenses (car_expense_type_id, amount, expense_date, notes)
		VALUES ($1, $2, $3, $4)
		RETURNING id, created_at, (SELECT name FROM car_expense_types WHERE id = car_expense_type_id);
	`

	err := db.conn.QueryRow(query,
		input.ExpenseTypeID,
		input.Amount,
		input.Date,
		input.Notes,
	).Scan(&input.ID, &input.CreatedAt, &input.Type)

	if err != nil {
		return fmt.Errorf("failed to create car expense: %w\n", err)
	}

	return nil
}

func (db *DB) GetCarExpensesForMonth(month time.Month, year int) (*[]models.CarExpense, error) {
	query := `
		SELECT ce.id, ct.name, ce.amount, ce.expense_date, ce.notes, ce.created_at
			FROM car_expenses ce
		JOIN
			car_expense_types ct ON ce.car_expense_type_id = ct.id
		WHERE
			EXTRACT(MONTH FROM expense_date) = $1 AND EXTRACT(YEAR FROM expense_date) = $2
		ORDER BY 
			ce.expense_date DESC;
	`

	var expenses []models.CarExpense
	rows, err := db.conn.Query(query,
		int(month),
		year,
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
			&exp.Date,
			&exp.Notes,
			&exp.CreatedAt,
		)

		if err != nil {
			return nil, fmt.Errorf("failed to scan expenses: %v", err)
		}
		expenses = append(expenses, exp)
	}

	return &expenses, nil
}

func (db *DB) GetCarExpensesForYear(year int) (*[]models.CarExpense, error) {
	query := `
		SELECT * FROM car_expenses
		WHERE EXTRACT(YEAR FROM expense_date) = $1
	`

	var expenses []models.CarExpense
	rows, err := db.conn.Query(query,
		year,
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
			&exp.CreatedAt,
		)

		if err != nil {
			return nil, fmt.Errorf("error scanning expenses: %v", err)
		}
		expenses = append(expenses, exp)
	}

	return &expenses, nil
}

func (db *DB) GetCarExpensesByType(utility string) (*[]models.CarExpense, error) {
	query := `
		SELECT * FROM car_expenses
		WHERE car_expense_type_id = (SELECT id FROM car_expense_types WHERE name = $1)`

	var expenses []models.CarExpense
	rows, err := db.conn.Query(query,
		utility,
	)

	if err != nil {
		return nil, fmt.Errorf("error fetching car expenses: %v", err)
	}

	for rows.Next() {
		err := rows.Err()
		if err != nil {
			return nil, fmt.Errorf("error fetching car expenses: %v", err)
		}

		var exp models.CarExpense
		err = rows.Scan(&exp.ID,
			&exp.Type,
			&exp.Amount,
			&exp.Date,
			&exp.Notes,
			&exp.CreatedAt,
		)

		if err != nil {
			return nil, fmt.Errorf("error scanning car expenses: %v", err)
		}
		expenses = append(expenses, exp)
	}

	return &expenses, nil
}

func (db *DB) EditCarExpense(editExpense *models.CarExpense) error {
	query := `
		UPDATE car_expenses
		SET
			car_expense_type_id = $2,
			amount = $3,
			expense_date = $4,
			notes = $5
		WHERE id = $1
		RETURNING (SELECT name FROM car_expense_types WHERE id = $2);
	`
	err := db.conn.QueryRow(query,
		editExpense.ID,
		editExpense.ExpenseTypeID,
		editExpense.Amount,
		editExpense.Date,
		editExpense.Notes,
	).Scan(&editExpense.Type)

	if err != nil {
		return fmt.Errorf("error editing car expense: %v", err)
	}

	return nil
}

func (db *DB) DeleteCarExpense(id int) (bool, error) {
	query := `
		DELETE FROM car_expenses
		WHERE id = $1`

	res, err := db.conn.Exec(query,
		id,
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
