package database

import (
	"context"
	"database/sql"
	"expenser/internal/groceryunits"
	"expenser/internal/models"
	"fmt"
	"time"

	"github.com/google/uuid"
)

// sizeStatusSQL derives size_status from the category id in $2, the unit in
// $11 and the unit size in $12 (in INSERT) -- EditGroceriesExpense repeats it with its own placeholders.
const sizeStatusSQL = `CASE
			WHEN $2 = (SELECT id FROM grocery_categories WHERE name = 'Other') THEN 'not_tracked'
			WHEN $11::text IS NOT NULL AND $12::numeric > 0 THEN 'known'
			ELSE 'unknown'
		END`

// nullableUnit returns the unit to store, or NULL when the size is not one
// the unit_size column can hold.
func nullableUnit(unit string, size float64) sql.NullString {
	return sql.NullString{String: unit, Valid: storableSize(unit, size)}
}

func nullableUnitSize(unit string, size float64) sql.NullFloat64 {
	return sql.NullFloat64{Float64: size, Valid: storableSize(unit, size)}
}

func storableSize(unit string, size float64) bool {
	return groceryunits.Size{Unit: groceryunits.Unit(unit), Per: size}.Storable()
}

const insertGroceriesSQL = `
		INSERT INTO groceries_expenses (user_id, category_id, product, quantity, unit_price, discount_per_unit, total_price, total_discount, supermarket_name, purchase_date, unit, unit_size, size_status)
		VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11::text, $12::numeric, ` + sizeStatusSQL + `)
		RETURNING id, created_at, size_status, (SELECT name FROM grocery_categories WHERE id = $2),
			quantity, unit_price, discount_per_unit, total_price, total_discount, unit, unit_size::float8`

// execQuerier is a *sql.DB or *sql.Tx.
type execQuerier interface {
	QueryRowContext(ctx context.Context, query string, args ...any) *sql.Row
}

// insertGroceriesExpense stores input and fills in its ID, creation time,
// size status and category name, and the amounts and size as stored
// (rounded to the columns), so it can be shown without reading it back.
func insertGroceriesExpense(ctx context.Context, q execQuerier, input *models.GroceriesExpense) error {
	var unit sql.NullString
	var unitSize sql.NullFloat64
	err := q.QueryRowContext(ctx, insertGroceriesSQL,
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
		nullableUnit(input.Unit, input.UnitSize),
		nullableUnitSize(input.Unit, input.UnitSize),
	).Scan(&input.ID, &input.CreatedAt, &input.SizeStatus, &input.CategoryName,
		&input.Quantity, &input.UnitPrice, &input.DiscountPerUnit, &input.TotalPrice, &input.TotalDiscount,
		&unit, &unitSize)
	if err != nil {
		return err
	}
	input.Unit, input.UnitSize = unit.String, unitSize.Float64
	return nil
}

// CreateGroceriesExpense stores one purchase and fills it in as stored (see
// insertGroceriesExpense).
func (db *DB) CreateGroceriesExpense(ctx context.Context, input *models.GroceriesExpense) error {
	if err := insertGroceriesExpense(ctx, db.conn, input); err != nil {
		return fmt.Errorf("failed to create groceries expense: %w", err)
	}
	return nil
}

// CreateGroceriesExpenses stores the purchases of one receipt, all or
// nothing, filling in each one like CreateGroceriesExpense.
func (db *DB) CreateGroceriesExpenses(ctx context.Context, items []*models.GroceriesExpense) error {
	tx, err := db.conn.BeginTx(ctx, nil)
	if err != nil {
		return fmt.Errorf("failed to begin groceries batch: %w", err)
	}
	defer tx.Rollback() //nolint:errcheck // no-op after Commit

	for i, item := range items {
		if err := insertGroceriesExpense(ctx, tx, item); err != nil {
			return fmt.Errorf("failed to save item %d: %w", i, err)
		}
	}
	if err := tx.Commit(); err != nil {
		return fmt.Errorf("failed to commit groceries batch: %w", err)
	}
	return nil
}

const groceriesSelect = `
		SELECT e.id, e.user_id, e.category_id, c.name, e.product, e.quantity, e.unit_price, e.discount_per_unit, e.total_price, e.total_discount, e.supermarket_name, e.purchase_date, e.created_at, e.unit, e.unit_size::float8, e.size_status
		FROM groceries_expenses e
		JOIN grocery_categories c ON e.category_id = c.id
`

// scanGroceriesExpense reads a row produced by groceriesSelect.
func scanGroceriesExpense(r rowScanner, e *models.GroceriesExpense) error {
	var unit sql.NullString
	var unitSize sql.NullFloat64
	if err := r.Scan(&e.ID, &e.UserID, &e.CategoryID, &e.CategoryName, &e.Product, &e.Quantity,
		&e.UnitPrice, &e.DiscountPerUnit, &e.TotalPrice, &e.TotalDiscount, &e.SupermarketName,
		&e.PurchaseDate, &e.CreatedAt, &unit, &unitSize, &e.SizeStatus); err != nil {
		return err
	}
	e.Unit, e.UnitSize = unit.String, unitSize.Float64
	return nil
}

// GetGroceriesExpensesForMonth returns the user's groceries bought in the
// given month and year, newest first.
func (db *DB) GetGroceriesExpensesForMonth(ctx context.Context, month time.Month, year int, userID uuid.UUID) ([]models.GroceriesExpense, error) {
	from, to := db.localRange(monthRange(month, year))
	expenses, err := queryAll(ctx, db.conn, scanGroceriesExpense, groceriesSelect+`
		WHERE e.user_id = $1 AND e.purchase_date >= $2 AND e.purchase_date < $3
		ORDER BY e.purchase_date DESC, e.id DESC`, userID, from, to)
	if err != nil {
		return nil, fmt.Errorf("failed to fetch groceries for month: %w", err)
	}
	return expenses, nil
}

// GetGroceriesMonthlyTotals returns net spending (price minus discount) per
// month, keyed "YYYY-MM", for purchases in [from, to).
func (db *DB) GetGroceriesMonthlyTotals(ctx context.Context, userID uuid.UUID, from, to time.Time) (map[string]float64, error) {
	from, to = db.localRange(from, to) // purchase times are instants
	return db.monthlyTotals(ctx, `
		SELECT to_char(purchase_date, 'YYYY-MM'), SUM(total_price - total_discount)
		FROM groceries_expenses
		WHERE user_id = $1 AND purchase_date >= $2 AND purchase_date < $3
		GROUP BY 1`, userID, from, to)
}

// GetGroceriesExpenseByID returns the user's groceries expense. It returns an
// error wrapping sql.ErrNoRows when no such expense belongs to the user.
func (db *DB) GetGroceriesExpenseByID(ctx context.Context, id int, userID uuid.UUID) (*models.GroceriesExpense, error) {
	var e models.GroceriesExpense
	row := db.conn.QueryRowContext(ctx, groceriesSelect+`WHERE e.id = $1 AND e.user_id = $2`, id, userID)
	if err := scanGroceriesExpense(row, &e); err != nil {
		return nil, fmt.Errorf("failed to get expense: %w", err)
	}
	return &e, nil
}

// EditGroceriesExpense updates the user's groceries expense. It returns an
// error wrapping sql.ErrNoRows when no such expense belongs to the user.
func (db *DB) EditGroceriesExpense(ctx context.Context, input *models.GroceriesExpense, userID uuid.UUID) error {
	query := `
		UPDATE groceries_expenses
		SET category_id = $1, product = $2, quantity = $3, unit_price = $4, discount_per_unit = $5, total_price = $6, total_discount = $7, supermarket_name = $8, purchase_date = $9,
			unit = $12::text, unit_size = $13::numeric,
			size_status = CASE
				WHEN $1 = (SELECT id FROM grocery_categories WHERE name = 'Other') THEN 'not_tracked'
				WHEN $12::text IS NOT NULL AND $13::numeric > 0 THEN 'known'
				ELSE 'unknown'
			END
		WHERE id = $10 AND user_id = $11;
	`
	res, err := db.conn.ExecContext(ctx, query, input.CategoryID, input.Product, input.Quantity, input.UnitPrice, input.DiscountPerUnit, input.TotalPrice, input.TotalDiscount, input.SupermarketName, input.PurchaseDate, input.ID, userID, nullableUnit(input.Unit, input.UnitSize), nullableUnitSize(input.Unit, input.UnitSize))
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
	n, err := res.RowsAffected()
	if err != nil {
		return false, fmt.Errorf("failed to delete expense: %w", err)
	}
	return n > 0, nil
}

func (db *DB) GetAllCategories(ctx context.Context) ([]models.Category, error) {
	categories, err := queryAll(ctx, db.conn, func(r rowScanner, c *models.Category) error {
		return r.Scan(&c.ID, &c.Name)
	}, `SELECT id, name FROM grocery_categories ORDER BY name ASC`)
	if err != nil {
		return nil, fmt.Errorf("failed to fetch categories: %w", err)
	}
	return categories, nil
}

// GetGroceriesCategoryTotals returns net groceries spending per category, keyed by name, for [from, to).
func (db *DB) GetGroceriesCategoryTotals(ctx context.Context, userID uuid.UUID, from, to time.Time) (map[string]float64, error) {
	from, to = db.localRange(from, to) // purchase times are instants
	return db.monthlyTotals(ctx, `
		SELECT c.name, SUM(e.total_price - e.total_discount)
		FROM groceries_expenses e
		JOIN grocery_categories c ON c.id = e.category_id
		WHERE e.user_id = $1 AND e.purchase_date >= $2 AND e.purchase_date < $3
		GROUP BY c.name`, userID, from, to)
}

// GetGroceriesSpendHistory returns net groceries spending per category and day for [from, to).
func (db *DB) GetGroceriesSpendHistory(ctx context.Context, userID uuid.UUID, from, to time.Time) ([]models.SpendEntry, error) {
	from, to = db.localRange(from, to) // purchase times are instants
	return db.spendHistory(ctx, `
		SELECT gc.name, to_char(g.purchase_date, 'YYYY-MM-DD'), SUM(g.total_price - g.total_discount)
		FROM groceries_expenses g
		JOIN grocery_categories gc ON gc.id = g.category_id
		WHERE g.user_id = $1 AND g.purchase_date >= $2 AND g.purchase_date < $3
		GROUP BY 1, 2`, userID, from, to)
}

// GetGroceriesStoreTotals returns net groceries spending per supermarket,
// keyed by its name, for [from, to). Purchases without a store name are
// left out.
func (db *DB) GetGroceriesStoreTotals(ctx context.Context, userID uuid.UUID, from, to time.Time) (map[string]float64, error) {
	from, to = db.localRange(from, to) // purchase times are instants
	return db.monthlyTotals(ctx, `
		SELECT TRIM(supermarket_name), SUM(total_price - total_discount)
		FROM groceries_expenses
		WHERE user_id = $1 AND purchase_date >= $2 AND purchase_date < $3
			AND TRIM(COALESCE(supermarket_name, '')) <> ''
		GROUP BY 1`, userID, from, to)
}
