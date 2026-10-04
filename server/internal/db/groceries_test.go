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
				assert.Equal(t, 0.0, got.DiscountPerUnit)
				assert.Equal(t, 0.0, got.TotalDiscount)
			},
		},
		{
			name: "WithDiscount",
			setup: func(t *testing.T) {
				testDB.CreateUser(TestUserRegisterModel)
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
			expense, _ := testDB.GetGroceriesExpenseByID(id)
			expense.Product = "Bread"
			expense.TotalPrice = 2.00
			expense.DiscountPerUnit = 0.50
			expense.TotalDiscount = 0.50
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

func TestGetGroceriesChartExpenses(t *testing.T) {
	cfg, err := config.LoadConfig()
	if err != nil {
		t.Fatal(err)
	}

	testDB := InitTestDB(cfg)
	defer testDB.Close()

	ResetTestDB(testDB)
	testDB.CreateUser(TestUserRegisterModel)

	categories, err := testDB.GetAllCategories()
	assert.NoError(t, err)
	assert.NotEmpty(t, categories)
	categoryID := categories[0].ID

	thisYear := time.Now().Year()
	lastYear := thisYear - 1

	expenses := []*models.GroceriesExpense{
		{UserID: TestUserRegisterModel.ID, CategoryID: categoryID, Product: "Milk", Quantity: 1, UnitPrice: 1.50, TotalPrice: 1.50, SupermarketName: "Lidl", PurchaseDate: time.Date(thisYear, time.January, 5, 0, 0, 0, 0, time.UTC)},
		{UserID: TestUserRegisterModel.ID, CategoryID: categoryID, Product: "Cheese", Quantity: 1, UnitPrice: 2.50, TotalPrice: 2.50, TotalDiscount: 0.50, SupermarketName: "Lidl", PurchaseDate: time.Date(thisYear, time.February, 5, 0, 0, 0, 0, time.UTC)},
		{UserID: TestUserRegisterModel.ID, CategoryID: categoryID, Product: "Bread", Quantity: 1, UnitPrice: 3.00, TotalPrice: 3.00, SupermarketName: "Lidl", PurchaseDate: time.Date(lastYear, time.March, 5, 0, 0, 0, 0, time.UTC)},
	}
	for _, e := range expenses {
		assert.NoError(t, testDB.CreateGroceriesExpense(e))
	}

	t.Run("GetGroceriesExpensesForYear with year returns only that year", func(t *testing.T) {
		got, err := testDB.GetGroceriesExpensesForYear(thisYear, TestUserRegisterModel.ID)
		assert.NoError(t, err)
		assert.Len(t, *got, 2)
	})

	t.Run("GetGroceriesExpensesForYear with year=0 returns all years", func(t *testing.T) {
		got, err := testDB.GetGroceriesExpensesForYear(0, TestUserRegisterModel.ID)
		assert.NoError(t, err)
		assert.Len(t, *got, 3)
	})

	t.Run("GetGroceriesExpenseCategoryForYear computes amount as total_price - total_discount", func(t *testing.T) {
		got, err := testDB.GetGroceriesExpenseCategoryForYear(categoryID, thisYear, TestUserRegisterModel.ID)
		assert.NoError(t, err)
		assert.Len(t, *got, 2)
		amounts := map[float64]bool{}
		for _, e := range *got {
			amounts[e.Amount] = true
			assert.Equal(t, categories[0].Name, e.CategoryName)
		}
		assert.True(t, amounts[1.50])
		assert.True(t, amounts[2.00]) // 2.50 - 0.50 discount
	})

	t.Run("GetGroceriesExpenseCategoryForYear with year=0 returns all years", func(t *testing.T) {
		got, err := testDB.GetGroceriesExpenseCategoryForYear(categoryID, 0, TestUserRegisterModel.ID)
		assert.NoError(t, err)
		assert.Len(t, *got, 3)
	})

	t.Run("GetGroceriesExpensesForYear carries TotalDiscount per row", func(t *testing.T) {
		got, err := testDB.GetGroceriesExpensesForYear(thisYear, TestUserRegisterModel.ID)
		assert.NoError(t, err)
		discounts := map[float64]bool{}
		for _, e := range *got {
			discounts[e.TotalDiscount] = true
		}
		assert.True(t, discounts[0.0])
		assert.True(t, discounts[0.50])
	})

	t.Run("GetGroceriesExpenseCategoryForYear carries TotalDiscount per row", func(t *testing.T) {
		got, err := testDB.GetGroceriesExpenseCategoryForYear(categoryID, thisYear, TestUserRegisterModel.ID)
		assert.NoError(t, err)
		discounts := map[float64]bool{}
		for _, e := range *got {
			discounts[e.TotalDiscount] = true
		}
		assert.True(t, discounts[0.0])
		assert.True(t, discounts[0.50])
	})
}
