package database

import (
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

// scanUnit applies the nullable unit columns to e.
func scanUnit(e *models.GroceriesExpense, unit sql.NullString, size sql.NullFloat64) {
	e.Unit = unit.String
	e.UnitSize = size.Float64
}

func (db *DB) CreateGroceriesExpense(input *models.GroceriesExpense) error {
	query := `
		INSERT INTO groceries_expenses (user_id, category_id, product, quantity, unit_price, discount_per_unit, total_price, total_discount, supermarket_name, purchase_date, unit, unit_size, size_status)
		VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11::text, $12::numeric, ` + sizeStatusSQL + `)
		RETURNING id, created_at, size_status;
	`

	err := db.conn.QueryRow(query,
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
	).Scan(&input.ID, &input.CreatedAt, &input.SizeStatus)

	if err != nil {
		return fmt.Errorf("failed to create groceries expense: %w", err)
	}

	return nil
}

const groceriesSelect = `
		SELECT e.id, e.user_id, e.category_id, c.name, e.product, e.quantity, e.unit_price, e.discount_per_unit, e.total_price, e.total_discount, e.supermarket_name, e.purchase_date, e.created_at, e.unit, e.unit_size::float8, e.size_status
		FROM groceries_expenses e
		JOIN grocery_categories c ON e.category_id = c.id
`

// scanGroceries reads rows produced by groceriesSelect.
func scanGroceries(rows *sql.Rows) ([]models.GroceriesExpense, error) {
	defer rows.Close()
	var expenses []models.GroceriesExpense
	for rows.Next() {
		var e models.GroceriesExpense
		var unitPriceRaw, discountPerUnitRaw, totalPriceRaw, totalDiscountRaw []byte
		var unit sql.NullString
		var unitSize sql.NullFloat64
		if err := rows.Scan(&e.ID, &e.UserID, &e.CategoryID, &e.CategoryName, &e.Product, &e.Quantity, &unitPriceRaw, &discountPerUnitRaw, &totalPriceRaw, &totalDiscountRaw, &e.SupermarketName, &e.PurchaseDate, &e.CreatedAt, &unit, &unitSize, &e.SizeStatus); err != nil {
			return nil, err
		}
		scanUnit(&e, unit, unitSize)
		fmt.Sscanf(string(unitPriceRaw), "%f", &e.UnitPrice)
		fmt.Sscanf(string(discountPerUnitRaw), "%f", &e.DiscountPerUnit)
		fmt.Sscanf(string(totalPriceRaw), "%f", &e.TotalPrice)
		fmt.Sscanf(string(totalDiscountRaw), "%f", &e.TotalDiscount)
		expenses = append(expenses, e)
	}
	return expenses, rows.Err()
}

// GetGroceriesExpensesForMonth returns the user's groceries bought in the
// given month and year, newest first.
func (db *DB) GetGroceriesExpensesForMonth(month time.Month, year int, userID uuid.UUID) (*[]models.GroceriesExpense, error) {
	rows, err := db.conn.Query(groceriesSelect+`
		WHERE e.user_id = $1
			AND EXTRACT(MONTH FROM e.purchase_date) = $2 AND EXTRACT(YEAR FROM e.purchase_date) = $3
		ORDER BY e.purchase_date DESC, e.id DESC;`, userID, int(month), year)
	if err != nil {
		return nil, fmt.Errorf("failed to query groceries for month: %w", err)
	}
	expenses, err := scanGroceries(rows)
	if err != nil {
		return nil, fmt.Errorf("failed to read groceries for month: %w", err)
	}
	return &expenses, nil
}

// GetGroceriesMonthlyTotals returns net spending (price minus discount) per
// month, keyed "YYYY-MM", for purchases in [from, to).
func (db *DB) GetGroceriesMonthlyTotals(userID uuid.UUID, from, to time.Time) (map[string]float64, error) {
	return db.monthlyTotals(`
		SELECT to_char(purchase_date, 'YYYY-MM'), SUM(total_price - total_discount)
		FROM groceries_expenses
		WHERE user_id = $1 AND purchase_date >= $2 AND purchase_date < $3
		GROUP BY 1`, userID, from, to)
}

// GetGroceriesExpenseByID returns the user's groceries expense. It returns an
// error wrapping sql.ErrNoRows when no such expense belongs to the user.
func (db *DB) GetGroceriesExpenseByID(id int, userID uuid.UUID) (*models.GroceriesExpense, error) {
	query := `
		SELECT e.id, e.user_id, e.category_id, c.name, e.product, e.quantity, e.unit_price, e.discount_per_unit, e.total_price, e.total_discount, e.supermarket_name, e.purchase_date, e.created_at, e.unit, e.unit_size::float8, e.size_status
		FROM groceries_expenses e
		JOIN grocery_categories c ON e.category_id = c.id
		WHERE e.id = $1 AND e.user_id = $2;
	`
	var e models.GroceriesExpense
	var unitPriceRaw, discountPerUnitRaw, totalPriceRaw, totalDiscountRaw []byte
	var unit sql.NullString
	var unitSize sql.NullFloat64
	err := db.conn.QueryRow(query, id, userID).Scan(&e.ID, &e.UserID, &e.CategoryID, &e.CategoryName, &e.Product, &e.Quantity, &unitPriceRaw, &discountPerUnitRaw, &totalPriceRaw, &totalDiscountRaw, &e.SupermarketName, &e.PurchaseDate, &e.CreatedAt, &unit, &unitSize, &e.SizeStatus)
	if err != nil {
		return nil, fmt.Errorf("failed to get expense: %w", err)
	}
	scanUnit(&e, unit, unitSize)
	fmt.Sscanf(string(unitPriceRaw), "%f", &e.UnitPrice)
	fmt.Sscanf(string(discountPerUnitRaw), "%f", &e.DiscountPerUnit)
	fmt.Sscanf(string(totalPriceRaw), "%f", &e.TotalPrice)
	fmt.Sscanf(string(totalDiscountRaw), "%f", &e.TotalDiscount)
	return &e, nil
}

// EditGroceriesExpense updates the user's groceries expense. It returns an
// error wrapping sql.ErrNoRows when no such expense belongs to the user.
func (db *DB) EditGroceriesExpense(input *models.GroceriesExpense, userID uuid.UUID) error {
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
	res, err := db.conn.Exec(query, input.CategoryID, input.Product, input.Quantity, input.UnitPrice, input.DiscountPerUnit, input.TotalPrice, input.TotalDiscount, input.SupermarketName, input.PurchaseDate, input.ID, userID, nullableUnit(input.Unit, input.UnitSize), nullableUnitSize(input.Unit, input.UnitSize))
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
func (db *DB) DeleteGroceriesExpense(id int, userID uuid.UUID) (bool, error) {
	query := `DELETE FROM groceries_expenses WHERE id = $1 AND user_id = $2`
	res, err := db.conn.Exec(query, id, userID)
	if err != nil {
		return false, fmt.Errorf("failed to delete expense: %w", err)
	}
	rowsAffected, _ := res.RowsAffected()
	return rowsAffected > 0, nil
}

func (db *DB) GetAllCategories() ([]models.Category, error) {
	query := `SELECT id, name FROM grocery_categories ORDER BY name ASC`
	rows, err := db.conn.Query(query)
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

// GetGroceriesCategoryTotals returns net groceries spending per category, keyed by name, for [from, to).
func (db *DB) GetGroceriesCategoryTotals(userID uuid.UUID, from, to time.Time) (map[string]float64, error) {
	return db.monthlyTotals(`
		SELECT c.name, SUM(e.total_price - e.total_discount)
		FROM groceries_expenses e
		JOIN grocery_categories c ON c.id = e.category_id
		WHERE e.user_id = $1 AND e.purchase_date >= $2 AND e.purchase_date < $3
		GROUP BY c.name`, userID, from, to)
}

// GetGroceriesSpendHistory returns net groceries spending per category and day for [from, to).
func (db *DB) GetGroceriesSpendHistory(userID uuid.UUID, from, to time.Time) ([]models.SpendEntry, error) {
	return db.spendHistory(`
		SELECT gc.name, to_char(g.purchase_date, 'YYYY-MM-DD'), SUM(g.total_price - g.total_discount)
		FROM groceries_expenses g
		JOIN grocery_categories gc ON gc.id = g.category_id
		WHERE g.user_id = $1 AND g.purchase_date >= $2 AND g.purchase_date < $3
		GROUP BY 1, 2`, userID, from, to)
}

// GetGroceriesStoreTotals returns net groceries spending per supermarket,
// keyed by its name, for [from, to). Purchases without a store name are
// left out.
func (db *DB) GetGroceriesStoreTotals(userID uuid.UUID, from, to time.Time) (map[string]float64, error) {
	return db.monthlyTotals(`
		SELECT TRIM(supermarket_name), SUM(total_price - total_discount)
		FROM groceries_expenses
		WHERE user_id = $1 AND purchase_date >= $2 AND purchase_date < $3
			AND TRIM(COALESCE(supermarket_name, '')) <> ''
		GROUP BY 1`, userID, from, to)
}
