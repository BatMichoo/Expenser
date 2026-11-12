package database

import (
	"context"
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
				testDB.CreateUser(context.Background(), TestUserRegisterModel)
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
				assert.Equal(t, 0.0, got.DiscountPerUnit)
				assert.Equal(t, 0.0, got.TotalDiscount)
			},
		},
		{
			name: "WithDiscount",
			setup: func(t *testing.T) {
				testDB.CreateUser(context.Background(), TestUserRegisterModel)
			},
			input: &models.GroceriesExpense{
				CategoryID:      1,
				Product:         "Cheese",
				Quantity:        2.0,
				UnitPrice:       5.00,
				DiscountPerUnit: 1.00,
				TotalPrice:      8.00,
				TotalDiscount:   2.00,
				SupermarketName: "Lidl",
				PurchaseDate:    time.Now(),
			},
			validate: func(t *testing.T, got *models.GroceriesExpense) {
				assert.Equal(t, "Cheese", got.Product)
				assert.Equal(t, 1.00, got.DiscountPerUnit)
				assert.Equal(t, 8.00, got.TotalPrice)
				assert.Equal(t, 2.00, got.TotalDiscount)
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ResetTestDB(testDB)
			tt.setup(t)

			categories, err := testDB.GetAllCategories(context.Background())
			assert.NoError(t, err)
			assert.NotEmpty(t, categories)

			tt.input.UserID = TestUserRegisterModel.ID
			tt.input.CategoryID = categories[0].ID
			err = testDB.CreateGroceriesExpense(context.Background(), tt.input)
			assert.NoError(t, err)

			got, err := testDB.GetGroceriesExpenseByID(context.Background(), tt.input.ID, TestUserRegisterModel.ID)
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
				testDB.CreateUser(context.Background(), TestUserRegisterModel)
				categories, err := testDB.GetAllCategories(context.Background())
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
				testDB.CreateGroceriesExpense(context.Background(), expense)
				return expense.ID
			},
			validate: func(t *testing.T, got *models.GroceriesExpense) {
				assert.Equal(t, "Bread", got.Product)
				assert.Equal(t, 2.00, got.TotalPrice)
				assert.Equal(t, 0.50, got.DiscountPerUnit)
				assert.Equal(t, 0.50, got.TotalDiscount)
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ResetTestDB(testDB)
			id := tt.setup(t)

			// Update
			expense, _ := testDB.GetGroceriesExpenseByID(context.Background(), id, TestUserRegisterModel.ID)
			expense.Product = "Bread"
			expense.TotalPrice = 2.00
			expense.DiscountPerUnit = 0.50
			expense.TotalDiscount = 0.50
			err := testDB.EditGroceriesExpense(context.Background(), expense, TestUserRegisterModel.ID)
			assert.NoError(t, err)

			updated, _ := testDB.GetGroceriesExpenseByID(context.Background(), id, TestUserRegisterModel.ID)
			tt.validate(t, updated)

			// Delete
			deleted, err := testDB.DeleteGroceriesExpense(context.Background(), id, TestUserRegisterModel.ID)
			assert.NoError(t, err)
			assert.True(t, deleted)
		})
	}
}

func TestEditGroceriesKeepsSize(t *testing.T) {
	cfg, err := config.LoadConfig()
	if err != nil {
		t.Fatal(err)
	}
	testDB := InitTestDB(cfg)
	defer testDB.Close()
	ResetTestDB(testDB)
	testDB.CreateUser(context.Background(), TestUserRegisterModel)
	categories, err := testDB.GetAllCategories(context.Background())
	assert.NoError(t, err)

	e := &models.GroceriesExpense{
		UserID: TestUserRegisterModel.ID, CategoryID: categories[0].ID, Product: "Milk", Quantity: 2,
		UnitPrice: 1.5, TotalPrice: 3, Unit: "l", UnitSize: 1, PurchaseDate: time.Now(),
	}
	assert.NoError(t, testDB.CreateGroceriesExpense(context.Background(), e))

	// The edit form posts the current size back, so editing keeps it.
	e.Product = "Fresh milk"
	assert.NoError(t, testDB.EditGroceriesExpense(context.Background(), e, TestUserRegisterModel.ID))
	got, err := testDB.GetGroceriesExpenseByID(context.Background(), e.ID, TestUserRegisterModel.ID)
	assert.NoError(t, err)
	assert.Equal(t, "l", got.Unit)
	assert.Equal(t, 1.0, got.UnitSize)
	assert.Equal(t, "known", got.SizeStatus)
}
