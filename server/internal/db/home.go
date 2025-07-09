package database

import (
	"database/sql"
	"expenser/internal/models"
	"fmt"
	"time"
)

func (db *DB) GetTotalExpenseForMonth(month time.Month) (float64, error) {
	currentYear := time.Now().Year()
	query := `
		SELECT SUM(amount) FROM home_expenses
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

func (db *DB) GetHighestExpenseForMonth(month time.Month) (float64, string, error) {
	currentYear := time.Now().Year()
	query := `
		SELECT
			he.amount,
			ut.name
		FROM
			home_expenses he
		JOIN
			utility_types ut ON he.utility_type_id = ut.id
		WHERE
			EXTRACT(MONTH FROM he.expense_date) = $1 AND EXTRACT(YEAR FROM he.expense_date) = $2
		ORDER BY
			he.amount DESC
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
func (db *DB) GetHomeExpenseByID(id int) (*models.HomeExpense, error) {
	query := `
		SELECT
			he.id,
			ut.name AS utility_name,
			he.amount,
			he.expense_date,
			he.notes,
			he.created_at
		FROM
			home_expenses he
		JOIN
			utility_types ut ON he.utility_type_id = ut.id
		WHERE
			he.id = $1;
	`

	var expense models.HomeExpense
	err := db.conn.QueryRow(query,
		id,
	).Scan(
		&expense.ID,
		&expense.UtilityType,
		&expense.Amount,
		&expense.ExpenseDate,
		&expense.Notes,
		&expense.CreatedAt)

	if err != nil {
		if err == sql.ErrNoRows {
			return nil, nil
		}
		return nil, fmt.Errorf("failed to get home expense: %w", err)
	}

	return &expense, nil
}

// Creates a new entry of a home expense. Automatically handles utility type FK.
func (db *DB) CreateHomeExpense(input *models.HomeExpense) error {
	query := `
		INSERT INTO home_expenses (utility_type_id, amount, expense_date, notes)
		VALUES ((SELECT id FROM utility_types WHERE name ILIKE $1), $2, $3, $4)
		RETURNING id, created_at;
	`

	err := db.conn.QueryRow(query,
		input.UtilityType,
		input.Amount,
		input.ExpenseDate,
		input.Notes,
	).Scan(&input.ID, &input.CreatedAt)

	if err != nil {
		return fmt.Errorf("failed to create home expense: %w", err)
	}

	return nil
}

func (db *DB) GetExpensesForMonth(month time.Month, year int) (*[]models.HomeExpense, error) {
	query := `
		SELECT he.id, ut.name, he.amount, he.expense_date, he.notes, he.created_at
			FROM home_expenses he
		JOIN
			utility_types ut ON he.utility_type_id = ut.id
		WHERE
			EXTRACT(MONTH FROM expense_date) = $1 AND EXTRACT(YEAR FROM expense_date) = $2
		ORDER BY 
			he.expense_date DESC;
	`

	var expenses []models.HomeExpense
	rows, err := db.conn.Query(query,
		int(month),
		year,
	)

	if err != nil {
		return nil, fmt.Errorf("failed to fetch expenses: %v", err)
	}

	for rows.Next() {
		err := rows.Err()
		if err != nil {
			return nil, fmt.Errorf("failed to fetch expenses: %v", err)
		}

		var exp models.HomeExpense
		err = rows.Scan(&exp.ID,
			&exp.UtilityType,
			&exp.Amount,
			&exp.ExpenseDate,
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

func (db *DB) GetExpensesForYear(year int) (*[]models.HomeExpense, error) {
	query := `
		SELECT * FROM home_expenses
		WHERE EXTRACT(YEAR FROM expense_date) = $1
	`

	var expenses []models.HomeExpense
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

		var exp models.HomeExpense
		err = rows.Scan(&exp.ID,
			&exp.UtilityType,
			&exp.Amount,
			&exp.ExpenseDate,
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

func (db *DB) GetExpensesByUtilityType(utility string) (*[]models.HomeExpense, error) {
	query := `
		SELECT * FROM home_expenses
		WHERE utility_type_id = (SELECT id FROM utility_types WHERE name = $1)`

	var expenses []models.HomeExpense
	rows, err := db.conn.Query(query,
		utility,
	)

	if err != nil {
		return nil, fmt.Errorf("error fetching expenses: %v", err)
	}

	for rows.Next() {
		err := rows.Err()
		if err != nil {
			return nil, fmt.Errorf("error fetching expenses: %v", err)
		}

		var exp models.HomeExpense
		err = rows.Scan(&exp.ID,
			&exp.UtilityType,
			&exp.Amount,
			&exp.ExpenseDate,
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

func (db *DB) EditHomeExpense(editExpense *models.HomeExpense) error {
	query := `
		UPDATE home_expenses
		SET
			utility_type_id = (SELECT id FROM utility_types WHERE name = $2),
			amount = $3,
			expense_date = $4,
			notes = $5
		WHERE id = $1;
	`
	row := db.conn.QueryRow(query,
		editExpense.ID,
		editExpense.UtilityType,
		editExpense.Amount,
		editExpense.ExpenseDate,
		editExpense.Notes,
	)

	if err := row.Err(); err != nil {
		return fmt.Errorf("error editing expense: %v", err)
	}

	return nil
}

func (db *DB) DeleteExpense(id int) (bool, error) {
	query := `
		DELETE FROM home_expenses
		WHERE id = $1`

	res, err := db.conn.Exec(query,
		id,
	)

	if err != nil {
		return false, fmt.Errorf("error deleting expense: %v", err)
	}

	rowCount, err := res.RowsAffected()
	if err != nil {
		return false, fmt.Errorf("error deleting expense: %v", err)
	}

	if rowCount < 1 {
		return false, nil
	}

	return true, nil
}
