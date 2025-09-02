package database

import (
	"expenser/internal/config"
	"expenser/internal/models"
	"fmt"
	"log"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
)

func TestCreateCarExpense(t *testing.T) {
	cfg, err := config.LoadConfig()
	if err != nil {
		log.Panicf("Couldn't load config in test Create Car Expense %v", err)
		t.FailNow()
	}

	testDB := InitTestDB(cfg)
	defer testDB.Close()

	type testCase struct {
		name     string
		input    *models.CarExpense
		setup    func(t *testing.T)
		wantErr  bool
		validate func(t *testing.T, got *models.CarExpense)
	}

	expenseDate := time.Now()
	tests := []testCase{
		{
			name: "Valid",
			setup: func(t *testing.T) {
				testDB.CreateUser(TestUserRegisterModel)
			},
			input: &models.CarExpense{
				Amount:        250.00,
				Date:          expenseDate,
				ExpenseTypeID: 1,
				Notes:         "Test 1234",
			},
			wantErr: false,
			validate: func(t *testing.T, got *models.CarExpense) {
				assert.Equal(t, 250.00, got.Amount)
				assert.Equal(t, expenseDate.Local().Round(time.Second), got.Date.Local().Round(time.Second))
				assert.Equal(t, "Fuel", got.Type)
				assert.Equal(t, "Test 1234", got.Notes)
			},
		},
		{
			name: "Invalid expense type",
			setup: func(t *testing.T) {
				testDB.CreateUser(TestUserRegisterModel)
			},
			input: &models.CarExpense{
				Amount:        250.00,
				Date:          expenseDate,
				ExpenseTypeID: -1,
				Notes:         "Test 1234",
			},
			wantErr: true,
			validate: func(t *testing.T, got *models.CarExpense) {
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ResetTestDB(testDB)
			tt.setup(t)

			tt.input.CreatedBy = TestUserRegisterModel.ID
			err := testDB.CreateCarExpense(tt.input)
			if tt.wantErr {
				assert.Error(t, err)
				return
			}
			assert.NoError(t, err)

			got, err := testDB.GetCarExpenseByID(tt.input.ID)
			assert.NoError(t, err)
			assert.NotNil(t, got)
			tt.validate(t, got)
		})
	}
}

func TestEditCarExpense(t *testing.T) {
	cfg, err := config.LoadConfig()
	if err != nil {
		log.Panicf("Couldn't load config in test Edit Car Expense %v", err)
		t.FailNow()
	}

	testDB := InitTestDB(cfg)
	defer testDB.Close()

	type testCase struct {
		name     string
		input    *models.CarExpense
		setup    func(t *testing.T) int
		wantErr  bool
		validate func(t *testing.T, got *models.CarExpense)
	}

	expenseDate := time.Now()
	tests := []testCase{
		{
			name: "Valid",
			setup: func(t *testing.T) int {
				testDB.CreateUser(TestUserRegisterModel)
				initial := &models.CarExpense{
					Amount:        150.00,
					Date:          expenseDate.Add(time.Hour * 24),
					ExpenseTypeID: 1,
					Notes:         "Test 1234567",
					CreatedBy:     TestUserRegisterModel.ID,
				}

				err := testDB.CreateCarExpense(initial)
				if err != nil {
					fmt.Printf("Error setting up existing expense test: %v", err)
					t.FailNow()
				}

				return initial.ID
			},
			input: &models.CarExpense{
				Amount:        250.00,
				Date:          expenseDate,
				ExpenseTypeID: 3,
				Notes:         "Test 1234",
			},
			wantErr: false,
			validate: func(t *testing.T, got *models.CarExpense) {
				assert.Equal(t, 250.00, got.Amount)
				assert.Equal(t, expenseDate.Local().Round(time.Second), got.Date.Local().Round(time.Second))
				assert.Equal(t, "Insurance", got.Type)
				assert.Equal(t, "Test 1234", got.Notes)
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ResetTestDB(testDB)

			tt.input.ID = tt.setup(t)

			err := testDB.EditCarExpense(tt.input)
			if tt.wantErr {
				assert.Error(t, err)
				return
			}
			assert.NoError(t, err)

			got, err := testDB.GetCarExpenseByID(tt.input.ID)
			assert.NoError(t, err)
			assert.NotNil(t, got)
			tt.validate(t, got)
		})
	}
}

func TestGetCarExpense(t *testing.T) {
	cfg, err := config.LoadConfig()
	if err != nil {
		log.Panicf("Couldn't load config in test Get Car Expense %v", err)
		t.FailNow()
	}

	testDB := InitTestDB(cfg)
	defer testDB.Close()

	type testCase struct {
		name     string
		input    any
		setup    func(t *testing.T) int
		wantErr  bool
		wantNil  bool
		validate func(t *testing.T, got *models.CarExpense)
	}

	expenseDate := time.Now()
	tests := []testCase{
		{
			name: "Existing Expense",
			setup: func(t *testing.T) int {
				testDB.CreateUser(TestUserRegisterModel)
				expense := &models.CarExpense{
					Amount:        250.00,
					Date:          expenseDate,
					ExpenseTypeID: 1,
					Notes:         "Test 1234",
					CreatedBy:     TestUserRegisterModel.ID,
				}

				err := testDB.CreateCarExpense(expense)
				if err != nil {
					fmt.Printf("Error setting up existing expense test: %v", err)
					t.FailNow()
				}

				return expense.ID
			},
			input:   0,
			wantErr: false,
			wantNil: false,
			validate: func(t *testing.T, got *models.CarExpense) {
				assert.Equal(t, 250.00, got.Amount)
				assert.Equal(t, expenseDate.Local().Round(time.Second), got.Date.Local().Round(time.Second))
				assert.Equal(t, "Fuel", got.Type)
				assert.Equal(t, "Test 1234", got.Notes)
			},
		},
		{
			name: "Missing expense",
			setup: func(t *testing.T) int {
				testDB.CreateUser(TestUserRegisterModel)
				expense := &models.CarExpense{
					Amount:        250.00,
					Date:          expenseDate,
					ExpenseTypeID: 1,
					Notes:         "Test 1234",
					CreatedBy:     TestUserRegisterModel.ID,
				}

				err := testDB.CreateCarExpense(expense)
				if err != nil {
					t.Skipf("Error setting up missing expense test: %v", err)
				}

				return expense.ID + 1
			},
			input: &models.CarExpense{
				Amount:        250.00,
				Date:          expenseDate,
				ExpenseTypeID: 7,
				Notes:         "Test 1234",
			},
			wantErr: false,
			wantNil: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ResetTestDB(testDB)

			ID := tt.setup(t)

			if tt.wantErr {
				assert.Error(t, err)
				return
			}
			assert.NoError(t, err)

			got, err := testDB.GetCarExpenseByID(ID)
			assert.NoError(t, err)

			if tt.wantNil {
				assert.Nil(t, got)
				return
			}

			assert.NotNil(t, got)
			tt.validate(t, got)
		})
	}
}

func TestGetMultipleCarExpenses(t *testing.T) {
	cfg, err := config.LoadConfig()
	if err != nil {
		log.Panicf("Couldn't load config in test Get Multiple Car Expense %v", err)
		t.FailNow()
	}

	testDB := InitTestDB(cfg)
	defer testDB.Close()

	type testCase struct {
		name       string
		funcToTest func(userId uuid.UUID) (*[]models.CarExpense, error)
		expected   []models.CarExpense
		setup      func(t *testing.T)
		wantErr    bool
		validate   func(t *testing.T, exp []models.CarExpense, got *[]models.CarExpense)
	}

	expenseDate := time.Now()
	tests := []testCase{
		{
			name: "Expenses for month",
			funcToTest: func(userId uuid.UUID) (*[]models.CarExpense, error) {
				return testDB.GetCarExpensesForMonth(expenseDate.Month(), expenseDate.Year(), userId)
			},
			setup: func(t *testing.T) {
				testDB.CreateUser(TestUserRegisterModel)
				expenses := []models.CarExpense{
					{
						Amount:        250.00,
						Date:          expenseDate.Add(time.Duration(31 * 24 * time.Hour)),
						ExpenseTypeID: 1,
						Notes:         "Test 1234",
						CreatedBy:     TestUserRegisterModel.ID,
					},
					{
						Amount:        350.00,
						Date:          expenseDate,
						ExpenseTypeID: 3,
						Notes:         "Test 12345",
						CreatedBy:     TestUserRegisterModel.ID,
					},
					{
						Amount:        450.00,
						Date:          expenseDate,
						ExpenseTypeID: 6,
						Notes:         "Test 123456",
						CreatedBy:     TestUserRegisterModel.ID,
					},
				}

				for i := range expenses {
					err := testDB.CreateCarExpense(&expenses[i])
					if err != nil {
						t.Skipf("Error setting up existing expense test: %v", err)
					}
				}
			},
			expected: []models.CarExpense{
				{
					Amount: 350.00,
					Date:   expenseDate.Add(time.Duration(30 * 24 * time.Hour)),
					Type:   "Insurance",
					Notes:  "Test 12345",
				},
				{
					Amount: 450.00,
					Date:   expenseDate.Add(time.Duration(30 * 24 * time.Hour)),
					Type:   "Other",
					Notes:  "Test 123456",
				},
			},
			wantErr: false,
			validate: func(t *testing.T, exp []models.CarExpense, got *[]models.CarExpense) {
				actual := *got
				for i, act := range actual {
					assert.Equal(t, exp[i].Amount, act.Amount)
				}
			},
		},
		{
			name: "Expenses for year",
			funcToTest: func(userId uuid.UUID) (*[]models.CarExpense, error) {
				return testDB.GetCarExpensesForYear(expenseDate.Year(), userId)
			},
			setup: func(t *testing.T) {
				testDB.CreateUser(TestUserRegisterModel)
				expenses := []models.CarExpense{
					{
						Amount:        250.00,
						Date:          expenseDate,
						ExpenseTypeID: 1,
						Notes:         "Test 1234",
						CreatedBy:     TestUserRegisterModel.ID,
					},
					{
						Amount:        350.00,
						Date:          expenseDate.Add(time.Duration(365 * 31 * 24 * time.Hour)),
						ExpenseTypeID: 3,
						Notes:         "Test 12345",
						CreatedBy:     TestUserRegisterModel.ID,
					},
					{
						Amount:        450.00,
						Date:          expenseDate.Add(time.Duration(365 * 31 * 24 * time.Hour)),
						ExpenseTypeID: 6,
						Notes:         "Test 123456",
						CreatedBy:     TestUserRegisterModel.ID,
					},
				}

				for _, exp := range expenses {
					err := testDB.CreateCarExpense(&exp)
					if err != nil {
						t.Skipf("Error setting up existing expense test: %v", err)
					}
				}
			},
			expected: []models.CarExpense{
				{
					Amount: 250.00,
					Date:   expenseDate,
					Type:   "Fuel",
					Notes:  "Test 1234",
				},
			},
			wantErr: false,
			validate: func(t *testing.T, exp []models.CarExpense, got *[]models.CarExpense) {
				actual := *got
				assert.Equal(t, len(exp), len(actual))

				for i, act := range actual {
					assert.Equal(t, exp[i].Amount, act.Amount)
				}
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ResetTestDB(testDB)

			tt.setup(t)

			if tt.wantErr {
				assert.Error(t, err)
				return
			}
			assert.NoError(t, err)

			got, err := tt.funcToTest(TestUserRegisterModel.ID)
			assert.NoError(t, err)

			assert.NotNil(t, got)
			tt.validate(t, tt.expected, got)
		})
	}
}

func TestGetTotalCarExpenseMonth(t *testing.T) {
	cfg, err := config.LoadConfig()
	if err != nil {
		log.Panicf("Couldn't load config in test Get Total Car Expense Month %v", err)
		t.FailNow()
	}

	testDB := InitTestDB(cfg)
	defer testDB.Close()

	type testCase struct {
		name       string
		funcToTest func(userId uuid.UUID) (float64, error)
		expected   float64
		setup      func(t *testing.T)
		wantErr    bool
		validate   func(t *testing.T, exp float64, got float64)
	}

	expenseDate := time.Now()
	tests := []testCase{
		{
			name: "One Expense for month",
			funcToTest: func(userId uuid.UUID) (float64, error) {
				return testDB.GetTotalCarExpenseForMonth(expenseDate.Month(), userId)
			},
			setup: func(t *testing.T) {
				testDB.CreateUser(TestUserRegisterModel)
				expenses := []models.CarExpense{
					{
						Amount:        250.00,
						Date:          expenseDate,
						ExpenseTypeID: 1,
						Notes:         "Test 1234",
						CreatedBy:     TestUserRegisterModel.ID,
					},
					{
						Amount:        350.00,
						Date:          expenseDate.Add(time.Duration(31 * 24 * time.Hour)),
						ExpenseTypeID: 3,
						Notes:         "Test 12345",
						CreatedBy:     TestUserRegisterModel.ID,
					},
					{
						Amount:        450.00,
						Date:          expenseDate.Add(time.Duration(31 * 24 * time.Hour)),
						ExpenseTypeID: 6,
						Notes:         "Test 123456",
						CreatedBy:     TestUserRegisterModel.ID,
					},
				}

				for _, exp := range expenses {
					err := testDB.CreateCarExpense(&exp)
					if err != nil {
						t.Skipf("Error setting up existing car expense test: %v", err)
					}
				}
			},
			expected: 250.00,
			wantErr:  false,
			validate: func(t *testing.T, exp float64, got float64) {
				assert.Equal(t, exp, got)
			},
		},
		{
			name: "No Expenses for month",
			funcToTest: func(userId uuid.UUID) (float64, error) {
				return testDB.GetTotalCarExpenseForMonth(expenseDate.Month(), userId)
			},
			setup: func(t *testing.T) {
				testDB.CreateUser(TestUserRegisterModel)
				expenses := []models.CarExpense{
					{
						Amount:        250.00,
						Date:          expenseDate.AddDate(0, -1, 0),
						ExpenseTypeID: 1,
						Notes:         "Test 1234",
						CreatedBy:     TestUserRegisterModel.ID,
					},
					{
						Amount:        350.00,
						Date:          expenseDate.Add(time.Duration(31 * 24 * time.Hour)),
						ExpenseTypeID: 3,
						Notes:         "Test 12345",
						CreatedBy:     TestUserRegisterModel.ID,
					},
					{
						Amount:        450.00,
						Date:          expenseDate.Add(time.Duration(31 * 24 * time.Hour)),
						ExpenseTypeID: 6,
						Notes:         "Test 123456",
						CreatedBy:     TestUserRegisterModel.ID,
					},
				}

				for _, exp := range expenses {
					err := testDB.CreateCarExpense(&exp)
					if err != nil {
						t.Skipf("Error setting up existing car expense test: %v", err)
					}
				}
			},
			expected: 0.00,
			wantErr:  false,
			validate: func(t *testing.T, exp float64, got float64) {
				assert.Equal(t, exp, got)
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ResetTestDB(testDB)

			tt.setup(t)

			if tt.wantErr {
				assert.Error(t, err)
				return
			}
			assert.NoError(t, err)

			got, err := tt.funcToTest(TestUserRegisterModel.ID)
			assert.NoError(t, err)

			assert.NotNil(t, got)
			tt.validate(t, tt.expected, got)
		})
	}
}

func TestGetHighestCarExpenseMonth(t *testing.T) {
	cfg, err := config.LoadConfig()
	if err != nil {
		log.Panicf("Couldn't load config in test Get Highest Car Expense Month %v", err)
		t.FailNow()
	}

	testDB := InitTestDB(cfg)
	defer testDB.Close()

	type testCase struct {
		name       string
		funcToTest func(userId uuid.UUID) (float64, string, error)
		setup      func(t *testing.T) (float64, string)
		wantErr    bool
		validate   func(t *testing.T, expA float64, expT string, gotA float64, gotT string)
	}

	expenseDate := time.Now()
	tests := []testCase{
		{
			name: "Highest Expense for month",
			funcToTest: func(userId uuid.UUID) (float64, string, error) {
				return testDB.GetHighestCarExpenseForMonth(expenseDate.Month(), userId)
			},
			setup: func(t *testing.T) (float64, string) {
				testDB.CreateUser(TestUserRegisterModel)
				expenses := []models.CarExpense{
					{
						Amount:        250.00,
						Date:          expenseDate.Add(time.Duration(30 * 24 * time.Hour)),
						ExpenseTypeID: 1,
						Notes:         "Test 1234",
						CreatedBy:     TestUserRegisterModel.ID,
					},
					{
						Amount:        350.00,
						Date:          expenseDate,
						ExpenseTypeID: 3,
						Notes:         "Test 12345",
						CreatedBy:     TestUserRegisterModel.ID,
					},
					{
						Amount:        450.00,
						Date:          expenseDate,
						ExpenseTypeID: 6,
						Notes:         "Test 123456",
						CreatedBy:     TestUserRegisterModel.ID,
					},
				}

				for i := range expenses {
					err := testDB.CreateCarExpense(&expenses[i])
					if err != nil {
						t.Skipf("Error setting up existing expense test: %v", err)
					}
				}

				return expenses[2].Amount, expenses[2].Type
			},
			wantErr: false,
			validate: func(t *testing.T, expA float64, expT string, gotA float64, gotT string) {
				assert.Equal(t, expA, gotA)
				assert.Equal(t, expT, gotT)
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ResetTestDB(testDB)

			expA, expT := tt.setup(t)

			if tt.wantErr {
				assert.Error(t, err)
				return
			}
			assert.NoError(t, err)

			gotA, gotT, err := tt.funcToTest(TestUserRegisterModel.ID)
			assert.NoError(t, err)

			assert.NotNil(t, gotT)
			tt.validate(t, expA, expT, gotA, gotT)
		})
	}
}

func TestDeleteCarExpense(t *testing.T) {
	cfg, err := config.LoadConfig()
	if err != nil {
		log.Panicf("Couldn't load config in test Delete Car Expense %v", err)
		t.FailNow()
	}

	testDB := InitTestDB(cfg)
	defer testDB.Close()

	type testCase struct {
		name     string
		input    *models.CarExpense
		setup    func(t *testing.T, he *models.CarExpense)
		wantErr  bool
		validate func(t *testing.T, got bool)
	}

	expenseDate := time.Now()
	tests := []testCase{
		{
			name: "Has existing expense",
			input: &models.CarExpense{
				Amount:        250.00,
				Date:          expenseDate,
				ExpenseTypeID: 1,
				Notes:         "Test 1234",
			},
			setup: func(t *testing.T, he *models.CarExpense) {
				testDB.CreateUser(TestUserRegisterModel)
				he.CreatedBy = TestUserRegisterModel.ID
				err := testDB.CreateCarExpense(he)
				if err != nil {
					t.Skipf("Error setting up deleting existing car expense test: %v", err)
				}
			},
			wantErr: false,
			validate: func(t *testing.T, got bool) {
				assert.True(t, got)
			},
		},
		{
			name: "No existing expense",
			setup: func(t *testing.T, he *models.CarExpense) {
				testDB.CreateUser(TestUserRegisterModel)
				he.CreatedBy = TestUserRegisterModel.ID
				he.ID = 15000
			},
			input: &models.CarExpense{
				Amount:        250.00,
				Date:          expenseDate,
				ExpenseTypeID: 7,
				Notes:         "Test 1234",
			},
			wantErr: false,
			validate: func(t *testing.T, got bool) {
				assert.False(t, got)
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ResetTestDB(testDB)
			tt.setup(t, tt.input)

			got, err := testDB.DeleteCarExpense(tt.input.ID)

			if tt.wantErr {
				assert.Error(t, err)
				return
			}

			assert.NoError(t, err)
			assert.NotNil(t, got)

			tt.validate(t, got)
		})
	}
}
