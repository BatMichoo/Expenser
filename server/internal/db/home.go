package database

import (
	"database/sql"
	"expenser/internal/models"
	"fmt"
	"time"

	"github.com/google/uuid"
)

func (db *DB) GetHouseUtilityTypes() (*[]models.HomeUtilityType, error) {
	query := `
		SELECT * FROM utility_types 
		`
	var expenseTypes []models.HomeUtilityType
	rows, err := db.conn.Query(query)

	if err != nil {
		return nil, fmt.Errorf("failed to fetch expenses: %v", err)
	}

	for rows.Next() {
		err := rows.Err()
		if err != nil {
			return nil, fmt.Errorf("failed to fetch expenses: %v", err)
		}

		var expType models.HomeUtilityType
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

// GetHouseExpenseByID returns the user's home expense with the given ID,
// or nil when no such expense belongs to the user.
func (db *DB) GetHouseExpenseByID(id int, userID uuid.UUID) (*models.HouseExpense, error) {
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
	err := db.conn.QueryRow(query,
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

	if err != nil {
		if err == sql.ErrNoRows {
			return nil, nil
		}
		return nil, fmt.Errorf("failed to get home expense: %w", err)
	}

	return &expense, nil
}

// Creates a new entry of a home expense. Automatically handles utility type FK.
func (db *DB) CreateHouseExpense(input *models.HouseExpense) error {
	query := `
		INSERT INTO home_expenses (utility_type_id, amount, discount_amount, expense_date, notes, metadata, created_by)
		VALUES ($1, $2, $3, $4, $5, $6, $7)
		RETURNING id, created_at, (SELECT name FROM utility_types WHERE id = utility_type_id);
	`

	err := db.conn.QueryRow(query,
		input.UtilityTypeID,
		input.Amount,
		input.DiscountAmount,
		input.ExpenseDate,
		input.Notes,
		input.Metadata,
		input.CreatedBy,
	).Scan(&input.ID, &input.CreatedAt, &input.UtilityType)

	if err != nil {
		return fmt.Errorf("failed to create home expense: %w", err)
	}

	return nil
}

func (db *DB) GetHouseExpensesForMonth(month time.Month, year int, userId uuid.UUID) (*[]models.HouseExpense, error) {
	query := `
		SELECT he.id, ut.name, he.amount, he.discount_amount, he.expense_date, he.notes, he.metadata, he.created_at, he.created_by
			FROM home_expenses he
		JOIN
			utility_types ut ON he.utility_type_id = ut.id
		WHERE
			EXTRACT(MONTH FROM expense_date) = $1 AND EXTRACT(YEAR FROM expense_date) = $2 AND created_by = $3
		ORDER BY
			he.expense_date DESC;
	`

	var expenses []models.HouseExpense
	rows, err := db.conn.Query(query,
		int(month),
		year,
		userId,
	)

	if err != nil {
		return nil, fmt.Errorf("failed to fetch expenses: %v", err)
	}

	for rows.Next() {
		err := rows.Err()
		if err != nil {
			return nil, fmt.Errorf("failed to fetch expenses: %v", err)
		}

		var exp models.HouseExpense
		err = rows.Scan(&exp.ID,
			&exp.UtilityType,
			&exp.Amount,
			&exp.DiscountAmount,
			&exp.ExpenseDate,
			&exp.Notes,
			&exp.Metadata,
			&exp.CreatedAt,
			&exp.CreatedBy,
		)

		if err != nil {
			return nil, fmt.Errorf("failed to scan expenses: %v", err)
		}
		expenses = append(expenses, exp)
	}

	return &expenses, nil
}

func (db *DB) GetHouseExpensesForYear(year int, userId uuid.UUID) (*[]models.HouseExpense, error) {
	query := `
		SELECT
			he.id, ut.name, he.amount, he.expense_date, he.notes, he.metadata, he.created_at, he.created_by
			FROM home_expenses he
		JOIN
			utility_types ut ON he.utility_type_id = ut.id
		WHERE
			($1 = 0 OR EXTRACT(YEAR FROM expense_date) = $1) AND created_by = $2
	`

	var expenses []models.HouseExpense
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

		var exp models.HouseExpense
		err = rows.Scan(&exp.ID,
			&exp.UtilityType,
			&exp.Amount,
			&exp.ExpenseDate,
			&exp.Notes,
			&exp.Metadata,
			&exp.CreatedAt,
			&exp.CreatedBy,
		)

		if err != nil {
			return nil, fmt.Errorf("error scanning expenses: %v", err)
		}
		expenses = append(expenses, exp)
	}

	return &expenses, nil
}

func (db *DB) GetHouseExpenseTypeForYear(utility, year int, userId uuid.UUID) (*[]models.HouseExpense, error) {
	query := `
		SELECT he.id, ut.name, he.amount, he.expense_date, he.metadata FROM home_expenses he
		JOIN utility_types ut ON he.utility_type_id = ut.id
		WHERE he.utility_type_id = $1 AND he.created_by = $3 AND ($2 = 0 OR EXTRACT(YEAR FROM he.expense_date) = $2)
		ORDER BY he.expense_date
	`

	var expenses []models.HouseExpense
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

		var exp models.HouseExpense
		err = rows.Scan(&exp.ID,
			&exp.UtilityType,
			&exp.Amount,
			&exp.ExpenseDate,
			&exp.Metadata,
		)

		if err != nil {
			return nil, fmt.Errorf("error scanning expenses: %v", err)
		}
		expenses = append(expenses, exp)
	}

	return &expenses, nil
}

// EditHouseExpense updates the user's home expense. It returns an error
// wrapping sql.ErrNoRows when no such expense belongs to the user.
func (db *DB) EditHouseExpense(editExpense *models.HouseExpense, userID uuid.UUID) error {
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
		RETURNING (SELECT name FROM utility_types WHERE id = $2);
	`
	err := db.conn.QueryRow(query,
		editExpense.ID,
		editExpense.UtilityTypeID,
		editExpense.Amount,
		editExpense.DiscountAmount,
		editExpense.ExpenseDate,
		editExpense.Notes,
		editExpense.Metadata,
		userID,
	).Scan(&editExpense.UtilityType)

	if err != nil {
		return fmt.Errorf("error editing expense: %w", err)
	}

	return nil
}

// DeleteHouseExpense deletes the user's home expense. It returns false when
// no such expense belongs to the user.
func (db *DB) DeleteHouseExpense(id int, userID uuid.UUID) (bool, error) {
	query := `
		DELETE FROM home_expenses
		WHERE id = $1 AND created_by = $2`

	res, err := db.conn.Exec(query,
		id,
		userID,
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

// GetHouseMonthlyTotals returns house spending per month, keyed "YYYY-MM", for expenses
// dated in [from, to).
func (db *DB) GetHouseMonthlyTotals(userID uuid.UUID, from, to time.Time) (map[string]float64, error) {
	return db.monthlyTotals(`
		SELECT to_char(expense_date, 'YYYY-MM'), SUM(amount)
		FROM home_expenses
		WHERE created_by = $1 AND expense_date >= $2 AND expense_date < $3
		GROUP BY 1`, userID, from, to)
}

// GetHouseTypeTotals returns house spending per utility type, keyed by name, for [from, to).
func (db *DB) GetHouseTypeTotals(userID uuid.UUID, from, to time.Time) (map[string]float64, error) {
	return db.monthlyTotals(`
		SELECT ut.name, SUM(he.amount)
		FROM home_expenses he
		JOIN utility_types ut ON ut.id = he.utility_type_id
		WHERE he.created_by = $1 AND he.expense_date >= $2 AND he.expense_date < $3
		GROUP BY ut.name`, userID, from, to)
}
