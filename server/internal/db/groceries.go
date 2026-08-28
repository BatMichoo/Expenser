package database

import (
	"expenser/internal/models"
	"fmt"
	"github.com/google/uuid"
)

func (db *DB) CreateGroceriesExpense(input *models.GroceriesExpense) error {
	query := `
		INSERT INTO groceries_expenses (user_id, category_id, product, quantity, unit_price, discount_per_unit, total_price, total_discount, supermarket_name, purchase_date)
		VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10)
		RETURNING id, created_at;
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
	).Scan(&input.ID, &input.CreatedAt)

	if err != nil {
		return fmt.Errorf("failed to create groceries expense: %w", err)
	}

	return nil
}

func (db *DB) GetRecentGroceriesExpenses(userID uuid.UUID) ([]models.GroceriesExpense, error) {
	query := `
		SELECT e.id, e.user_id, e.category_id, c.name, e.product, e.quantity, e.unit_price, e.discount_per_unit, e.total_price, e.total_discount, e.supermarket_name, e.purchase_date, e.created_at
		FROM groceries_expenses e
		JOIN grocery_categories c ON e.category_id = c.id
		WHERE e.user_id = $1
		ORDER BY e.purchase_date DESC
		LIMIT 10;
	`
	rows, err := db.conn.Query(query, userID)
	if err != nil {
		return nil, fmt.Errorf("failed to query expenses: %w", err)
	}
	defer rows.Close()

	var expenses []models.GroceriesExpense
	for rows.Next() {
		var e models.GroceriesExpense
		var unitPriceRaw, discountPerUnitRaw, totalPriceRaw, totalDiscountRaw []byte
		if err := rows.Scan(&e.ID, &e.UserID, &e.CategoryID, &e.CategoryName, &e.Product, &e.Quantity, &unitPriceRaw, &discountPerUnitRaw, &totalPriceRaw, &totalDiscountRaw, &e.SupermarketName, &e.PurchaseDate, &e.CreatedAt); err != nil {
			return nil, err
		}
		fmt.Sscanf(string(unitPriceRaw), "%f", &e.UnitPrice)
		fmt.Sscanf(string(discountPerUnitRaw), "%f", &e.DiscountPerUnit)
		fmt.Sscanf(string(totalPriceRaw), "%f", &e.TotalPrice)
		fmt.Sscanf(string(totalDiscountRaw), "%f", &e.TotalDiscount)
		expenses = append(expenses, e)
	}
	return expenses, nil
}

func (db *DB) GetGroceriesExpenseByID(id int) (*models.GroceriesExpense, error) {
	query := `
		SELECT e.id, e.user_id, e.category_id, c.name, e.product, e.quantity, e.unit_price, e.discount_per_unit, e.total_price, e.total_discount, e.supermarket_name, e.purchase_date, e.created_at
		FROM groceries_expenses e
		JOIN grocery_categories c ON e.category_id = c.id
		WHERE e.id = $1;
	`
	var e models.GroceriesExpense
	var unitPriceRaw, discountPerUnitRaw, totalPriceRaw, totalDiscountRaw []byte
	err := db.conn.QueryRow(query, id).Scan(&e.ID, &e.UserID, &e.CategoryID, &e.CategoryName, &e.Product, &e.Quantity, &unitPriceRaw, &discountPerUnitRaw, &totalPriceRaw, &totalDiscountRaw, &e.SupermarketName, &e.PurchaseDate, &e.CreatedAt)
	if err != nil {
		return nil, fmt.Errorf("failed to get expense: %w", err)
	}
	fmt.Sscanf(string(unitPriceRaw), "%f", &e.UnitPrice)
	fmt.Sscanf(string(discountPerUnitRaw), "%f", &e.DiscountPerUnit)
	fmt.Sscanf(string(totalPriceRaw), "%f", &e.TotalPrice)
	fmt.Sscanf(string(totalDiscountRaw), "%f", &e.TotalDiscount)
	return &e, nil
}

func (db *DB) EditGroceriesExpense(input *models.GroceriesExpense) error {
	query := `
		UPDATE groceries_expenses
		SET category_id = $1, product = $2, quantity = $3, unit_price = $4, discount_per_unit = $5, total_price = $6, total_discount = $7, supermarket_name = $8, purchase_date = $9
		WHERE id = $10;
	`
	_, err := db.conn.Exec(query, input.CategoryID, input.Product, input.Quantity, input.UnitPrice, input.DiscountPerUnit, input.TotalPrice, input.TotalDiscount, input.SupermarketName, input.PurchaseDate, input.ID)
	if err != nil {
		return fmt.Errorf("failed to update expense: %w", err)
	}
	return nil
}

func (db *DB) DeleteGroceriesExpense(id int) (bool, error) {
	query := `DELETE FROM groceries_expenses WHERE id = $1`
	res, err := db.conn.Exec(query, id)
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
