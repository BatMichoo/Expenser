package database

import (
	"expenser/internal/config"
	"expenser/internal/models"
	"testing"
	"time"
)

func TestCreateGroceriesExpense(t *testing.T) {
	cfg, _ := config.LoadConfig()
	db := InitTestDB(cfg)
	defer ResetTestDB(db)

	expense := &models.GroceriesExpense{
		UserID:          "03ca7c92-364c-4420-865b-052a3c1df340",
		Product:         "Milk",
		Quantity:        1.0,
		Price:           150,
		SupermarketName: "Lidl",
		PurchaseDate:    time.Now(),
	}

	err := db.CreateGroceriesExpense(expense)
	if err != nil {
		t.Fatalf("Failed to create groceries expense: %v", err)
	}

	if expense.ID == 0 {
		t.Error("Expected ID to be set")
	}
}

func TestGroceriesCRUD(t *testing.T) {
	cfg, _ := config.LoadConfig()
	db := InitTestDB(cfg)
	defer ResetTestDB(db)

	// Create
	expense := &models.GroceriesExpense{
		UserID:          "03ca7c92-364c-4420-865b-052a3c1df340",
		Product:         "Milk",
		Quantity:        1.0,
		Price:           150,
		SupermarketName: "Lidl",
		PurchaseDate:    time.Now(),
	}
	db.CreateGroceriesExpense(expense)

	// Get
	retrieved, err := db.GetGroceriesExpenseByID(expense.ID)
	if err != nil {
		t.Fatalf("Failed to get expense: %v", err)
	}
	if retrieved.Product != "Milk" {
		t.Errorf("Expected Milk, got %s", retrieved.Product)
	}

	// Update
	retrieved.Product = "Bread"
	err = db.EditGroceriesExpense(retrieved)
	if err != nil {
		t.Fatalf("Failed to update: %v", err)
	}
	updated, _ := db.GetGroceriesExpenseByID(expense.ID)
	if updated.Product != "Bread" {
		t.Errorf("Expected Bread, got %s", updated.Product)
	}

	// Delete
	deleted, err := db.DeleteGroceriesExpense(expense.ID)
	if err != nil || !deleted {
		t.Fatalf("Failed to delete: %v", err)
	}
	_, err = db.GetGroceriesExpenseByID(expense.ID)
	if err == nil {
		t.Error("Expected error getting deleted expense")
	}
}

