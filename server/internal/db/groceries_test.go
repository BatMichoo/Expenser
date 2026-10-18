package database

import (
	"database/sql"
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
				testDB.CreateUser(testCtx, TestUserRegisterModel)
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
				testDB.CreateUser(testCtx, TestUserRegisterModel)
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

			categories, err := testDB.GetAllCategories(testCtx)
			assert.NoError(t, err)
			assert.NotEmpty(t, categories)

			tt.input.UserID = TestUserRegisterModel.ID
			tt.input.CategoryID = categories[0].ID
			err = testDB.CreateGroceriesExpense(testCtx, tt.input)
			assert.NoError(t, err)

			got, err := testDB.GetGroceriesExpenseByID(testCtx, tt.input.ID, TestUserRegisterModel.ID)
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
				testDB.CreateUser(testCtx, TestUserRegisterModel)
				categories, err := testDB.GetAllCategories(testCtx)
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
				testDB.CreateGroceriesExpense(testCtx, expense)
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
			expense, _ := testDB.GetGroceriesExpenseByID(testCtx, id, TestUserRegisterModel.ID)
			expense.Product = "Bread"
			expense.TotalPrice = 2.00
			expense.DiscountPerUnit = 0.50
			expense.TotalDiscount = 0.50
			err := testDB.EditGroceriesExpense(testCtx, expense, TestUserRegisterModel.ID)
			assert.NoError(t, err)

			updated, _ := testDB.GetGroceriesExpenseByID(testCtx, id, TestUserRegisterModel.ID)
			tt.validate(t, updated)

			// Delete
			deleted, err := testDB.DeleteGroceriesExpense(testCtx, id, TestUserRegisterModel.ID)
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
	testDB.CreateUser(testCtx, TestUserRegisterModel)

	categories, err := testDB.GetAllCategories(testCtx)
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
		assert.NoError(t, testDB.CreateGroceriesExpense(testCtx, e))
	}

	t.Run("GetGroceriesExpensesForYear with year returns only that year", func(t *testing.T) {
		got, err := testDB.GetGroceriesExpensesForYear(testCtx, thisYear, TestUserRegisterModel.ID)
		assert.NoError(t, err)
		assert.Len(t, *got, 2)
	})

	t.Run("GetGroceriesExpensesForYear with year=0 returns all years", func(t *testing.T) {
		got, err := testDB.GetGroceriesExpensesForYear(testCtx, 0, TestUserRegisterModel.ID)
		assert.NoError(t, err)
		assert.Len(t, *got, 3)
	})

	t.Run("GetGroceriesExpenseCategoryForYear computes amount as total_price - total_discount", func(t *testing.T) {
		got, err := testDB.GetGroceriesExpenseCategoryForYear(testCtx, categoryID, thisYear, TestUserRegisterModel.ID)
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
		got, err := testDB.GetGroceriesExpenseCategoryForYear(testCtx, categoryID, 0, TestUserRegisterModel.ID)
		assert.NoError(t, err)
		assert.Len(t, *got, 3)
	})

	t.Run("GetGroceriesExpensesForYear carries TotalDiscount per row", func(t *testing.T) {
		got, err := testDB.GetGroceriesExpensesForYear(testCtx, thisYear, TestUserRegisterModel.ID)
		assert.NoError(t, err)
		discounts := map[float64]bool{}
		for _, e := range *got {
			discounts[e.TotalDiscount] = true
		}
		assert.True(t, discounts[0.0])
		assert.True(t, discounts[0.50])
	})

	t.Run("GetGroceriesExpenseCategoryForYear carries TotalDiscount per row", func(t *testing.T) {
		got, err := testDB.GetGroceriesExpenseCategoryForYear(testCtx, categoryID, thisYear, TestUserRegisterModel.ID)
		assert.NoError(t, err)
		discounts := map[float64]bool{}
		for _, e := range *got {
			discounts[e.TotalDiscount] = true
		}
		assert.True(t, discounts[0.0])
		assert.True(t, discounts[0.50])
	})
}

func TestGroceriesMonthSummary(t *testing.T) {
	cfg, err := config.LoadConfig()
	if err != nil {
		t.Fatal(err)
	}

	testDB := InitTestDB(cfg)
	defer testDB.Close()

	ResetTestDB(testDB)
	testDB.CreateUser(testCtx, TestUserRegisterModel)

	categories, err := testDB.GetAllCategories(testCtx)
	assert.NoError(t, err)
	assert.GreaterOrEqual(t, len(categories), 2)

	now := time.Now()
	month := now.Month()

	t.Run("Empty month returns zero", func(t *testing.T) {
		total, err := testDB.GetTotalGroceriesExpenseForMonth(testCtx, month, now.Year(), TestUserRegisterModel.ID)
		assert.NoError(t, err)
		assert.Equal(t, 0.0, total)

		amount, category, err := testDB.GetHighestGroceriesCategoryForMonth(testCtx, month, TestUserRegisterModel.ID)
		assert.NoError(t, err)
		assert.Equal(t, 0.0, amount)
		assert.Equal(t, "", category)
	})

	expenses := []*models.GroceriesExpense{
		{UserID: TestUserRegisterModel.ID, CategoryID: categories[0].ID, Product: "Milk", Quantity: 1, UnitPrice: 1.50, TotalPrice: 1.50, SupermarketName: "Lidl", PurchaseDate: now},
		{UserID: TestUserRegisterModel.ID, CategoryID: categories[1].ID, Product: "Steak", Quantity: 1, UnitPrice: 10.00, TotalPrice: 10.00, TotalDiscount: 2.00, SupermarketName: "Lidl", PurchaseDate: now},
		{UserID: TestUserRegisterModel.ID, CategoryID: categories[0].ID, Product: "Bread", Quantity: 1, UnitPrice: 3.00, TotalPrice: 3.00, SupermarketName: "Lidl", PurchaseDate: now.AddDate(-1, 0, 0)},
	}
	for _, e := range expenses {
		assert.NoError(t, testDB.CreateGroceriesExpense(testCtx, e))
	}

	t.Run("Total is net of discounts and limited to current month", func(t *testing.T) {
		total, err := testDB.GetTotalGroceriesExpenseForMonth(testCtx, month, now.Year(), TestUserRegisterModel.ID)
		assert.NoError(t, err)
		assert.InDelta(t, 9.50, total, 0.001) // 1.50 + (10.00 - 2.00)
	})

	t.Run("Highest returns the top category by net amount", func(t *testing.T) {
		amount, category, err := testDB.GetHighestGroceriesCategoryForMonth(testCtx, month, TestUserRegisterModel.ID)
		assert.NoError(t, err)
		assert.InDelta(t, 8.00, amount, 0.001)
		assert.Equal(t, categories[1].Name, category)
	})
}

func TestCreateGroceriesExpensesIsAtomic(t *testing.T) {
	cfg, err := config.LoadConfig()
	if err != nil {
		t.Fatal(err)
	}

	testDB := InitTestDB(cfg)
	defer testDB.Close()

	ResetTestDB(testDB)
	testDB.CreateUser(testCtx, TestUserRegisterModel)

	otherID, err := testDB.GetCategoryIDByName(testCtx, "Other")
	assert.NoError(t, err)

	now := time.Now()
	item := func(product string, categoryID int) *models.GroceriesExpense {
		return &models.GroceriesExpense{UserID: TestUserRegisterModel.ID, CategoryID: categoryID, Product: product, Quantity: 1, UnitPrice: 1, TotalPrice: 1, PurchaseDate: now}
	}

	t.Run("A failing item rolls back the whole batch", func(t *testing.T) {
		err := testDB.CreateGroceriesExpenses(testCtx, []*models.GroceriesExpense{item("Good", otherID), item("Bad", 999999)})
		assert.Error(t, err)

		got, err := testDB.GetGroceriesExpensesForMonth(testCtx, now.Month(), now.Year(), TestUserRegisterModel.ID)
		assert.NoError(t, err)
		assert.Empty(t, got)
	})

	t.Run("A valid batch saves every item", func(t *testing.T) {
		batch := []*models.GroceriesExpense{item("One", otherID), item("Two", otherID)}
		assert.NoError(t, testDB.CreateGroceriesExpenses(testCtx, batch))
		for _, e := range batch {
			assert.NotZero(t, e.ID)
		}

		got, err := testDB.GetGroceriesExpensesForMonth(testCtx, now.Month(), now.Year(), TestUserRegisterModel.ID)
		assert.NoError(t, err)
		assert.Len(t, got, 2)
		assert.Equal(t, "Other", got[0].CategoryName)
	})

	t.Run("Unknown category name is not found", func(t *testing.T) {
		_, err := testDB.GetCategoryIDByName(testCtx, "Caviar")
		assert.ErrorIs(t, err, sql.ErrNoRows)
	})
}
