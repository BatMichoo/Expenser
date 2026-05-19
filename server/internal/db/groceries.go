package database

import (
	"expenser/internal/models"
	"fmt"
	"github.com/google/uuid"
)

func (db *DB) CreateGroceriesExpense(input *models.GroceriesExpense) error {
	query := `
		INSERT INTO groceries_expenses (user_id, product, quantity, price, supermarket_name, purchase_date)
		VALUES ($1, $2, $3, $4, $5, $6)
		RETURNING id, created_at;
	`

	err := db.conn.QueryRow(query,
		input.UserID,
		input.Product,
		input.Quantity,
		input.Price,
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
		SELECT id, user_id, product, quantity, price, supermarket_name, purchase_date, created_at
		FROM groceries_expenses
		WHERE user_id = $1
		ORDER BY purchase_date DESC
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
		if err := rows.Scan(&e.ID, &e.UserID, &e.Product, &e.Quantity, &e.Price, &e.SupermarketName, &e.PurchaseDate, &e.CreatedAt); err != nil {
			return nil, err
		}
		expenses = append(expenses, e)
	}
	return expenses, nil
}

func (db *DB) GetGroceriesExpenseByID(id int) (*models.GroceriesExpense, error) {
	query := `
		SELECT id, user_id, product, quantity, price, supermarket_name, purchase_date, created_at
		FROM groceries_expenses
		WHERE id = $1;
	`
	var e models.GroceriesExpense
	err := db.conn.QueryRow(query, id).Scan(&e.ID, &e.UserID, &e.Product, &e.Quantity, &e.Price, &e.SupermarketName, &e.PurchaseDate, &e.CreatedAt)
	if err != nil {
		return nil, fmt.Errorf("failed to get expense: %w", err)
	}
	return &e, nil
}

func (db *DB) EditGroceriesExpense(input *models.GroceriesExpense) error {
	query := `
		UPDATE groceries_expenses
		SET product = $1, quantity = $2, price = $3, supermarket_name = $4, purchase_date = $5
		WHERE id = $6;
	`
	_, err := db.conn.Exec(query, input.Product, input.Quantity, input.Price, input.SupermarketName, input.PurchaseDate, input.ID)
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
