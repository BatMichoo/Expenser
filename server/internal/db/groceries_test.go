package database

import (
	"expenser/internal/config"
	"expenser/internal/models"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
)

func TestCreateGroceriesExpense(t *testing.T) {
	cfg, err := config.LoadConfig()
	if err != nil {
		t.Fatal(err)
	}

	testDB := InitTestDB(cfg)
	defer testDB.Close()

	type testCase struct {
		name     string
		input    *models.GroceriesExpense
		setup    func(t *testing.T)
		validate func(t *testing.T, got *models.GroceriesExpense)
	}

	tests := []testCase{
		{
			name: "Valid",
			setup: func(t *testing.T) {
				testDB.CreateUser(TestUserRegisterModel)
			},
			input: &models.GroceriesExpense{
				CategoryID:      1,
				Product:         "Milk",
				Quantity:        1.0,
				UnitPrice:       1.50,
				TotalPrice:      1.50,
				SupermarketName: "Lidl",
				PurchaseDate:    time.Now(),
			},
			validate: func(t *testing.T, got *models.GroceriesExpense) {
				assert.Equal(t, "Milk", got.Product)
				assert.Equal(t, 1.50, got.TotalPrice)
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ResetTestDB(testDB)
			tt.setup(t)

			categories, err := testDB.GetAllCategories()
			assert.NoError(t, err)
			assert.NotEmpty(t, categories)

			tt.input.UserID = TestUserRegisterModel.ID
			tt.input.CategoryID = categories[0].ID
			err = testDB.CreateGroceriesExpense(tt.input)
			assert.NoError(t, err)

			got, err := testDB.GetGroceriesExpenseByID(tt.input.ID)
			assert.NoError(t, err)
			assert.NotNil(t, got)
			tt.validate(t, got)
		})
	}
}

func TestGroceriesCRUD(t *testing.T) {
	cfg, err := config.LoadConfig()
	if err != nil {
		t.Fatal(err)
	}

	testDB := InitTestDB(cfg)
	defer testDB.Close()

	type testCase struct {
		name     string
		setup    func(t *testing.T) int
		validate func(t *testing.T, got *models.GroceriesExpense)
	}

	tests := []testCase{
		{
			name: "CRUD Workflow",
			setup: func(t *testing.T) int {
				testDB.CreateUser(TestUserRegisterModel)
				categories, err := testDB.GetAllCategories()
				assert.NoError(t, err)
				assert.NotEmpty(t, categories)
				expense := &models.GroceriesExpense{
					UserID:          TestUserRegisterModel.ID,
					CategoryID:      categories[0].ID,
					Product:         "Milk",
					Quantity:        1.0,
					UnitPrice:       1.50,
					TotalPrice:      1.50,
					SupermarketName: "Lidl",
					PurchaseDate:    time.Now(),
				}
				testDB.CreateGroceriesExpense(expense)
				return expense.ID
			},
			validate: func(t *testing.T, got *models.GroceriesExpense) {
				assert.Equal(t, "Bread", got.Product)
				assert.Equal(t, 2.00, got.TotalPrice)
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ResetTestDB(testDB)
			id := tt.setup(t)

			// Update
			expense, _ := testDB.GetGroceriesExpenseByID(id)
			expense.Product = "Bread"
			expense.TotalPrice = 2.00
			err := testDB.EditGroceriesExpense(expense)
			assert.NoError(t, err)

			updated, _ := testDB.GetGroceriesExpenseByID(id)
			tt.validate(t, updated)

			// Delete
			deleted, err := testDB.DeleteGroceriesExpense(id)
			assert.NoError(t, err)
			assert.True(t, deleted)
		})
	}
}
