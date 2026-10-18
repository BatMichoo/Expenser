package database

import (
	"context"
	"database/sql"
	"expenser/internal/models"
	"fmt"
	"time"

	"github.com/google/uuid"
)

// groceriesExpenseColumns is the select list scanned by scanGroceriesExpense.
const groceriesExpenseColumns = `e.id, e.user_id, e.category_id, c.name, e.product, e.quantity, e.unit_price, e.discount_per_unit, e.total_price, e.total_discount, e.supermarket_name, e.purchase_date, e.created_at`

// rowScanner is satisfied by *sql.Row and *sql.Rows.
type rowScanner interface {
	Scan(dest ...any) error
}

func scanGroceriesExpense(row rowScanner) (*models.GroceriesExpense, error) {
	var e models.GroceriesExpense
	err := row.Scan(&e.ID, &e.UserID, &e.CategoryID, &e.CategoryName, &e.Product, &e.Quantity, &e.UnitPrice, &e.DiscountPerUnit, &e.TotalPrice, &e.TotalDiscount, &e.SupermarketName, &e.PurchaseDate, &e.CreatedAt)
	if err != nil {
		return nil, err
	}
	return &e, nil
}

// queryRower is satisfied by *sql.DB and *sql.Tx.
type queryRower interface {
	QueryRowContext(ctx context.Context, query string, args ...any) *sql.Row
}

func insertGroceriesExpense(ctx context.Context, q queryRower, input *models.GroceriesExpense) error {
	query := `
		INSERT INTO groceries_expenses (user_id, category_id, product, quantity, unit_price, discount_per_unit, total_price, total_discount, supermarket_name, purchase_date)
		VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10)
		RETURNING id, created_at;
	`

	err := q.QueryRowContext(ctx, query,
		input.UserID,
		input.CategoryID,
		input.Product,
		input.Quantity,
		input.UnitPrice,
		input.DiscountPerUnit,
		input.TotalPrice,
		input.TotalDiscount,
		input.SupermarketName,
		input.PurchaseDate,
	).Scan(&input.ID, &input.CreatedAt)

	if err != nil {
		return fmt.Errorf("failed to create groceries expense: %w", err)
	}

	return nil
}

func (db *DB) CreateGroceriesExpense(ctx context.Context, input *models.GroceriesExpense) error {
	return insertGroceriesExpense(ctx, db.conn, input)
}

// CreateGroceriesExpenses saves all inputs in one transaction: either every
// expense is saved or none is.
func (db *DB) CreateGroceriesExpenses(ctx context.Context, inputs []*models.GroceriesExpense) error {
	tx, err := db.conn.BeginTx(ctx, nil)
	if err != nil {
		return fmt.Errorf("failed to begin transaction: %w", err)
	}
	defer tx.Rollback()

	for i, input := range inputs {
		if err := insertGroceriesExpense(ctx, tx, input); err != nil {
			return fmt.Errorf("item %d: %w", i, err)
		}
	}

	if err := tx.Commit(); err != nil {
		return fmt.Errorf("failed to commit groceries expenses: %w", err)
	}
	return nil
}

// GetTotalGroceriesExpenseForMonth returns the user's net groceries spending
// (total price minus discount) for the given month and year.
func (db *DB) GetTotalGroceriesExpenseForMonth(ctx context.Context, month time.Month, year int, userID uuid.UUID) (float64, error) {
	query := `
		SELECT SUM(total_price - total_discount) FROM groceries_expenses
		WHERE EXTRACT(MONTH FROM purchase_date) = $1 AND EXTRACT(YEAR FROM purchase_date) = $2 AND user_id = $3
	`

	var total sql.NullFloat64
	if err := db.conn.QueryRowContext(ctx, query, int(month), year, userID).Scan(&total); err != nil {
		return 0, fmt.Errorf("failed to get total groceries amount: %w", err)
	}

	return total.Float64, nil
}

// GetHighestGroceriesCategoryForMonth returns the category with the highest
// net spending for the given month of the current year, and that amount.
func (db *DB) GetHighestGroceriesCategoryForMonth(ctx context.Context, month time.Month, userID uuid.UUID) (float64, string, error) {
	query := `
		SELECT SUM(e.total_price - e.total_discount) AS amount, c.name
		FROM groceries_expenses e
		JOIN grocery_categories c ON e.category_id = c.id
		WHERE EXTRACT(MONTH FROM e.purchase_date) = $1 AND EXTRACT(YEAR FROM e.purchase_date) = $2 AND e.user_id = $3
		GROUP BY c.name
		ORDER BY amount DESC
		LIMIT 1;
	`

	var amount sql.NullFloat64
	var category string
	err := db.conn.QueryRowContext(ctx, query, int(month), time.Now().Year(), userID).Scan(&amount, &category)
	if err != nil {
		if err == sql.ErrNoRows {
			return 0, "", nil
		}
		return 0, "", fmt.Errorf("failed to get highest groceries category: %w", err)
	}

	return amount.Float64, category, nil
}

// GetGroceriesExpensesForMonth returns the user's groceries expenses for the
// given month and year, newest first.
func (db *DB) GetGroceriesExpensesForMonth(ctx context.Context, month time.Month, year int, userID uuid.UUID) ([]models.GroceriesExpense, error) {
	query := `
		SELECT ` + groceriesExpenseColumns + `
		FROM groceries_expenses e
		JOIN grocery_categories c ON e.category_id = c.id
		WHERE e.user_id = $1 AND EXTRACT(MONTH FROM e.purchase_date) = $2 AND EXTRACT(YEAR FROM e.purchase_date) = $3
		ORDER BY e.purchase_date DESC, e.id DESC;
	`
	rows, err := db.conn.QueryContext(ctx, query, userID, int(month), year)
	if err != nil {
		return nil, fmt.Errorf("failed to query expenses: %w", err)
	}
	defer rows.Close()

	var expenses []models.GroceriesExpense
	for rows.Next() {
		e, err := scanGroceriesExpense(rows)
		if err != nil {
			return nil, err
		}
		expenses = append(expenses, *e)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("failed to read expenses: %w", err)
	}
	return expenses, nil
}

// GetGroceriesExpenseByID returns the user's groceries expense. It returns an
// error wrapping sql.ErrNoRows when no such expense belongs to the user.
func (db *DB) GetGroceriesExpenseByID(ctx context.Context, id int, userID uuid.UUID) (*models.GroceriesExpense, error) {
	query := `
		SELECT ` + groceriesExpenseColumns + `
		FROM groceries_expenses e
		JOIN grocery_categories c ON e.category_id = c.id
		WHERE e.id = $1 AND e.user_id = $2;
	`
	e, err := scanGroceriesExpense(db.conn.QueryRowContext(ctx, query, id, userID))
	if err != nil {
		return nil, fmt.Errorf("failed to get expense: %w", err)
	}
	return e, nil
}

// EditGroceriesExpense updates the user's groceries expense. It returns an
// error wrapping sql.ErrNoRows when no such expense belongs to the user.
func (db *DB) EditGroceriesExpense(ctx context.Context, input *models.GroceriesExpense, userID uuid.UUID) error {
	query := `
		UPDATE groceries_expenses
		SET category_id = $1, product = $2, quantity = $3, unit_price = $4, discount_per_unit = $5, total_price = $6, total_discount = $7, supermarket_name = $8, purchase_date = $9
		WHERE id = $10 AND user_id = $11;
	`
	res, err := db.conn.ExecContext(ctx, query, input.CategoryID, input.Product, input.Quantity, input.UnitPrice, input.DiscountPerUnit, input.TotalPrice, input.TotalDiscount, input.SupermarketName, input.PurchaseDate, input.ID, userID)
	if err != nil {
		return fmt.Errorf("failed to update expense: %w", err)
	}
	rowsAffected, err := res.RowsAffected()
	if err != nil {
		return fmt.Errorf("failed to update expense: %w", err)
	}
	if rowsAffected == 0 {
		return fmt.Errorf("failed to update expense: %w", sql.ErrNoRows)
	}
	return nil
}

// DeleteGroceriesExpense deletes the user's groceries expense. It returns
// false when no such expense belongs to the user.
func (db *DB) DeleteGroceriesExpense(ctx context.Context, id int, userID uuid.UUID) (bool, error) {
	query := `DELETE FROM groceries_expenses WHERE id = $1 AND user_id = $2`
	res, err := db.conn.ExecContext(ctx, query, id, userID)
	if err != nil {
		return false, fmt.Errorf("failed to delete expense: %w", err)
	}
	rowsAffected, _ := res.RowsAffected()
	return rowsAffected > 0, nil
}

func (db *DB) GetGroceriesExpensesForYear(ctx context.Context, year int, userID uuid.UUID) (*[]models.GroceriesChartExpense, error) {
	query := `
		SELECT c.name, (e.total_price - e.total_discount) AS amount, e.total_discount, e.purchase_date
		FROM groceries_expenses e
		JOIN grocery_categories c ON e.category_id = c.id
		WHERE e.user_id = $2 AND ($1 = 0 OR EXTRACT(YEAR FROM e.purchase_date) = $1)
		ORDER BY e.purchase_date;
	`
	rows, err := db.conn.QueryContext(ctx, query, year, userID)
	if err != nil {
		return nil, fmt.Errorf("error fetching expenses: %v", err)
	}
	defer rows.Close()

	return scanGroceriesChartExpenses(rows)
}

func (db *DB) GetGroceriesExpenseCategoryForYear(ctx context.Context, categoryID, year int, userID uuid.UUID) (*[]models.GroceriesChartExpense, error) {
	query := `
		SELECT c.name, (e.total_price - e.total_discount) AS amount, e.total_discount, e.purchase_date
		FROM groceries_expenses e
		JOIN grocery_categories c ON e.category_id = c.id
		WHERE e.category_id = $1 AND e.user_id = $3 AND ($2 = 0 OR EXTRACT(YEAR FROM e.purchase_date) = $2)
		ORDER BY e.purchase_date;
	`
	rows, err := db.conn.QueryContext(ctx, query, categoryID, year, userID)
	if err != nil {
		return nil, fmt.Errorf("error fetching expenses: %v", err)
	}
	defer rows.Close()

	return scanGroceriesChartExpenses(rows)
}

func scanGroceriesChartExpenses(rows *sql.Rows) (*[]models.GroceriesChartExpense, error) {
	var expenses []models.GroceriesChartExpense
	for rows.Next() {
		var e models.GroceriesChartExpense
		if err := rows.Scan(&e.CategoryName, &e.Amount, &e.TotalDiscount, &e.PurchaseDate); err != nil {
			return nil, err
		}
		expenses = append(expenses, e)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("failed to read expenses: %w", err)
	}
	return &expenses, nil
}

func (db *DB) GetAllCategories(ctx context.Context) ([]models.Category, error) {
	query := `SELECT id, name FROM grocery_categories ORDER BY name ASC`
	rows, err := db.conn.QueryContext(ctx, query)
	if err != nil {
		return nil, fmt.Errorf("failed to fetch categories: %w", err)
	}
	defer rows.Close()

	var categories []models.Category
	for rows.Next() {
		var c models.Category
		if err := rows.Scan(&c.ID, &c.Name); err != nil {
			return nil, err
		}
		categories = append(categories, c)
	}
	return categories, nil
}

// GetCategoryIDByName returns the ID of the grocery category with the given
// name. It returns an error wrapping sql.ErrNoRows when there is none.
func (db *DB) GetCategoryIDByName(ctx context.Context, name string) (int, error) {
	var id int
	err := db.conn.QueryRowContext(ctx, `SELECT id FROM grocery_categories WHERE name = $1`, name).Scan(&id)
	if err != nil {
		return 0, fmt.Errorf("failed to get category %q: %w", name, err)
	}
	return id, nil
}
