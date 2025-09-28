package database

import (
	"expenser/internal/config"
	"expenser/internal/models"
	"log"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
)

func TestCreateHomeExpense(t *testing.T) {
	cfg, err := config.LoadConfig()
	if err != nil {
		log.Panicf("Couldn't load config in test Create Home Expense %v", err)
		t.FailNow()
	}

	testDB := InitTestDB(cfg)
	defer testDB.Close()

	type testCase struct {
		name     string
		input    *models.HouseExpense
		setup    func(t *testing.T) uuid.UUID
		wantErr  bool
		validate func(t *testing.T, got *models.HouseExpense)
	}

	expenseDate := time.Now()
	tests := []testCase{
		{
			name: "Valid",
			setup: func(t *testing.T) uuid.UUID {
				testDB.CreateUser(TestUserRegisterModel)
				return TestUserRegisterModel.ID
			},
			input: &models.HouseExpense{
				Amount:        250.00,
				ExpenseDate:   expenseDate,
				UtilityTypeID: 3,
				Notes:         "Test 1234",
			},
			wantErr: false,
			validate: func(t *testing.T, got *models.HouseExpense) {
				assert.Equal(t, 250.00, got.Amount)
				assert.Equal(t, expenseDate.Local().Round(time.Second), got.ExpenseDate.Local().Round(time.Second))
				assert.Equal(t, "Gas", got.UtilityType)
				assert.Equal(t, "Test 1234", got.Notes)
			},
		},
		{
			name: "Invalid utility type",
			setup: func(t *testing.T) uuid.UUID {
				testDB.CreateUser(TestUserRegisterModel)
				return TestUserRegisterModel.ID
			},
			input: &models.HouseExpense{
				Amount:        250.00,
				ExpenseDate:   expenseDate,
				UtilityTypeID: 0,
				Notes:         "Test 1234",
			},
			wantErr: true,
			validate: func(t *testing.T, got *models.HouseExpense) {
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ResetTestDB(testDB)

			userId := tt.setup(t)
			tt.input.CreatedBy = userId

			err := testDB.CreateHouseExpense(tt.input)
			if tt.wantErr {
				assert.Error(t, err)
				return
			}
			assert.NoError(t, err)

			got, err := testDB.GetHouseExpenseByID(tt.input.ID)
			assert.NoError(t, err)
			assert.NotNil(t, got)
			tt.validate(t, got)
		})
	}
}

func TestEditHomeExpense(t *testing.T) {
	cfg, err := config.LoadConfig()
	if err != nil {
		log.Panicf("Couldn't load config in test Edit Home Expense %v", err)
		t.FailNow()
	}

	testDB := InitTestDB(cfg)
	defer testDB.Close()

	type testCase struct {
		name     string
		input    *models.HouseExpense
		setup    func(t *testing.T) int
		wantErr  bool
		validate func(t *testing.T, got *models.HouseExpense)
	}

	expenseDate := time.Now()
	tests := []testCase{
		{
			name: "Valid",
			setup: func(t *testing.T) int {
				testDB.CreateUser(TestUserRegisterModel)
				initial := &models.HouseExpense{
					Amount:        150.00,
					ExpenseDate:   expenseDate.Add(time.Hour * 24),
					UtilityTypeID: 1,
					Notes:         "Test 1234567",
					CreatedBy:     TestUserRegisterModel.ID,
				}

				err := testDB.CreateHouseExpense(initial)
				if err != nil {
					t.Skipf("Error setting up existing expense test: %v", err)
				}

				return initial.ID
			},
			input: &models.HouseExpense{
				Amount:        250.00,
				ExpenseDate:   expenseDate,
				UtilityTypeID: 3,
				Notes:         "Test 1234",
			},
			wantErr: false,
			validate: func(t *testing.T, got *models.HouseExpense) {
				assert.Equal(t, 250.00, got.Amount)
				assert.Equal(t, expenseDate.Local().Round(time.Second), got.ExpenseDate.Local().Round(time.Second))
				assert.Equal(t, "Gas", got.UtilityType)
				assert.Equal(t, "Test 1234", got.Notes)
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ResetTestDB(testDB)

			tt.input.ID = tt.setup(t)

			err := testDB.EditHouseExpense(tt.input)
			if tt.wantErr {
				assert.Error(t, err)
				return
			}
			assert.NoError(t, err)

			got, err := testDB.GetHouseExpenseByID(tt.input.ID)
			assert.NoError(t, err)
			assert.NotNil(t, got)
			tt.validate(t, got)
		})
	}
}

func TestGetHomeExpense(t *testing.T) {
	cfg, err := config.LoadConfig()
	if err != nil {
		log.Panicf("Couldn't load config in test Get Home Expense %v", err)
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
		validate func(t *testing.T, got *models.HouseExpense)
	}

	expenseDate := time.Now()
	tests := []testCase{
		{
			name: "Existing Expense",
			setup: func(t *testing.T) int {
				testDB.CreateUser(TestUserRegisterModel)
				expense := &models.HouseExpense{
					Amount:        250.00,
					ExpenseDate:   expenseDate,
					UtilityTypeID: 3,
					Notes:         "Test 1234",
					CreatedBy:     TestUserRegisterModel.ID,
				}

				err := testDB.CreateHouseExpense(expense)
				if err != nil {
					t.Skipf("Error setting up existing expense test: %v", err)
				}

				return expense.ID
			},
			input:   0,
			wantErr: false,
			wantNil: false,
			validate: func(t *testing.T, got *models.HouseExpense) {
				assert.Equal(t, 250.00, got.Amount)
				assert.Equal(t, expenseDate.Local().Round(time.Second), got.ExpenseDate.Local().Round(time.Second))
				assert.Equal(t, "Gas", got.UtilityType)
				assert.Equal(t, "Test 1234", got.Notes)
			},
		},
		{
			name: "Missing expense",
			setup: func(t *testing.T) int {
				testDB.CreateUser(TestUserRegisterModel)
				expense := &models.HouseExpense{
					Amount:        250.00,
					ExpenseDate:   expenseDate,
					UtilityTypeID: 3,
					Notes:         "Test 1234",
					CreatedBy:     TestUserRegisterModel.ID,
				}

				err := testDB.CreateHouseExpense(expense)
				if err != nil {
					t.Skipf("Error setting up missing expense test: %v", err)
				}

				return expense.ID + 1
			},
			input: &models.HouseExpense{
				Amount:        250.00,
				ExpenseDate:   expenseDate,
				UtilityTypeID: 0,
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

			got, err := testDB.GetHouseExpenseByID(ID)
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

func TestGetMultipleHomeExpenses(t *testing.T) {
	cfg, err := config.LoadConfig()
	if err != nil {
		log.Panicf("Couldn't load config in test Get Multiple Home Expense %v", err)
		t.FailNow()
	}

	testDB := InitTestDB(cfg)
	defer testDB.Close()

	type testCase struct {
		name       string
		funcToTest func(userId uuid.UUID) (*[]models.HouseExpense, error)
		expected   []models.HouseExpense
		setup      func(t *testing.T)
		wantErr    bool
		validate   func(t *testing.T, exp []models.HouseExpense, got *[]models.HouseExpense)
	}

	expenseDate := time.Now()
	tests := []testCase{
		{
			name: "Expenses for month",
			funcToTest: func(userId uuid.UUID) (*[]models.HouseExpense, error) {
				return testDB.GetHouseExpensesForMonth(expenseDate.Month(), expenseDate.Year(), userId)
			},
			setup: func(t *testing.T) {
				testDB.CreateUser(TestUserRegisterModel)
				expenses := []models.HouseExpense{
					{
						Amount:        250.00,
						ExpenseDate:   expenseDate.Add(time.Duration(31 * 24 * time.Hour)),
						UtilityTypeID: 3,
						Notes:         "Test 1234",
						CreatedBy:     TestUserRegisterModel.ID,
					},
					{
						Amount:        350.00,
						ExpenseDate:   expenseDate,
						UtilityTypeID: 2,
						Notes:         "Test 12345",
						CreatedBy:     TestUserRegisterModel.ID,
					},
					{
						Amount:        450.00,
						ExpenseDate:   expenseDate,
						UtilityTypeID: 5,
						Notes:         "Test 123456",
						CreatedBy:     TestUserRegisterModel.ID,
					},
				}

				for _, exp := range expenses {
					err := testDB.CreateHouseExpense(&exp)
					if err != nil {
						t.Skipf("Error setting up existing expense test: %v", err)
					}
				}
			},
			expected: []models.HouseExpense{
				{
					Amount:      350.00,
					ExpenseDate: expenseDate.Add(time.Duration(30 * 24 * time.Hour)),
					UtilityType: "Water",
					Notes:       "Test 12345",
				},
				{
					Amount:      450.00,
					ExpenseDate: expenseDate.Add(time.Duration(30 * 24 * time.Hour)),
					UtilityType: "TV",
					Notes:       "Test 123456",
				},
			},
			wantErr: false,
			validate: func(t *testing.T, exp []models.HouseExpense, got *[]models.HouseExpense) {
				actual := *got
				for i, act := range actual {
					assert.Equal(t, exp[i].Amount, act.Amount)
				}
			},
		},
		{
			name: "Expenses for year",
			funcToTest: func(userId uuid.UUID) (*[]models.HouseExpense, error) {
				return testDB.GetHouseExpensesForYear(expenseDate.Year(), userId)
			},
			setup: func(t *testing.T) {
				testDB.CreateUser(TestUserRegisterModel)
				expenses := []models.HouseExpense{
					{
						Amount:        250.00,
						ExpenseDate:   expenseDate,
						UtilityTypeID: 3,
						Notes:         "Test 1234",
						CreatedBy:     TestUserRegisterModel.ID,
					},
					{
						Amount:        350.00,
						ExpenseDate:   expenseDate.Add(time.Duration(365 * 31 * 24 * time.Hour)),
						UtilityTypeID: 2,
						Notes:         "Test 12345",
						CreatedBy:     TestUserRegisterModel.ID,
					},
					{
						Amount:        450.00,
						ExpenseDate:   expenseDate.Add(time.Duration(365 * 31 * 24 * time.Hour)),
						UtilityTypeID: 5,
						Notes:         "Test 123456",
						CreatedBy:     TestUserRegisterModel.ID,
					},
				}

				for _, exp := range expenses {
					err := testDB.CreateHouseExpense(&exp)
					if err != nil {
						t.Skipf("Error setting up existing expense test: %v", err)
					}
				}
			},
			expected: []models.HouseExpense{
				{
					Amount:      250.00,
					ExpenseDate: expenseDate,
					UtilityType: "Gas",
					Notes:       "Test 1234",
				},
			},
			wantErr: false,
			validate: func(t *testing.T, exp []models.HouseExpense, got *[]models.HouseExpense) {
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

func TestGetTotalHomeExpenseMonth(t *testing.T) {
	cfg, err := config.LoadConfig()
	if err != nil {
		log.Panicf("Couldn't load config in test Get Total Home Expense Month %v", err)
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
				return testDB.GetTotalHouseExpenseForMonth(expenseDate.Month(), userId)
			},
			setup: func(t *testing.T) {
				testDB.CreateUser(TestUserRegisterModel)
				expenses := []models.HouseExpense{
					{
						Amount:        250.00,
						ExpenseDate:   expenseDate,
						UtilityTypeID: 3,
						Notes:         "Test 1234",
						CreatedBy:     TestUserRegisterModel.ID,
					},
					{
						Amount:        350.00,
						ExpenseDate:   expenseDate.Add(time.Duration(31 * 24 * time.Hour)),
						UtilityTypeID: 2,
						Notes:         "Test 12345",
						CreatedBy:     TestUserRegisterModel.ID,
					},
					{
						Amount:        450.00,
						ExpenseDate:   expenseDate.Add(time.Duration(31 * 24 * time.Hour)),
						UtilityTypeID: 5,
						Notes:         "Test 123456",
						CreatedBy:     TestUserRegisterModel.ID,
					},
				}

				for _, exp := range expenses {
					err := testDB.CreateHouseExpense(&exp)
					if err != nil {
						t.Skipf("Error setting up existing expense test: %v", err)
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
				return testDB.GetTotalHouseExpenseForMonth(expenseDate.Month(), userId)
			},
			setup: func(t *testing.T) {
				testDB.CreateUser(TestUserRegisterModel)
				expenses := []models.HouseExpense{
					{
						Amount:        250.00,
						ExpenseDate:   expenseDate.AddDate(0, -1, 0),
						UtilityTypeID: 3,
						Notes:         "Test 1234",
						CreatedBy:     TestUserRegisterModel.ID,
					},
					{
						Amount:        350.00,
						ExpenseDate:   expenseDate.Add(time.Duration(31 * 24 * time.Hour)),
						UtilityTypeID: 2,
						Notes:         "Test 12345",
						CreatedBy:     TestUserRegisterModel.ID,
					},
					{
						Amount:        450.00,
						ExpenseDate:   expenseDate.Add(time.Duration(31 * 24 * time.Hour)),
						UtilityTypeID: 5,
						Notes:         "Test 123456",
						CreatedBy:     TestUserRegisterModel.ID,
					},
				}

				for _, exp := range expenses {
					err := testDB.CreateHouseExpense(&exp)
					if err != nil {
						t.Skipf("Error setting up existing expense test: %v", err)
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

func TestGetHighestHomeExpenseMonth(t *testing.T) {
	cfg, err := config.LoadConfig()
	if err != nil {
		log.Panicf("Couldn't load config in test Get Highest Home Expense Month %v", err)
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
				return testDB.GetHighestHouseExpenseForMonth(expenseDate.Month(), userId)
			},
			setup: func(t *testing.T) (float64, string) {
				testDB.CreateUser(TestUserRegisterModel)
				expenses := []models.HouseExpense{
					{
						Amount:        250.00,
						ExpenseDate:   expenseDate.Add(time.Duration(30 * 24 * time.Hour)),
						UtilityTypeID: 3,
						Notes:         "Test 1234",
						CreatedBy:     TestUserRegisterModel.ID,
					},
					{
						Amount:        350.00,
						ExpenseDate:   expenseDate,
						UtilityTypeID: 2,
						Notes:         "Test 12345",
						CreatedBy:     TestUserRegisterModel.ID,
					},
					{
						Amount:        450.00,
						ExpenseDate:   expenseDate,
						UtilityTypeID: 5,
						Notes:         "Test 123456",
						CreatedBy:     TestUserRegisterModel.ID,
					},
				}

				for i := range expenses {
					err := testDB.CreateHouseExpense(&expenses[i])
					if err != nil {
						t.Skipf("Error setting up existing expense test: %v", err)
					}
				}

				return expenses[2].Amount, expenses[2].UtilityType
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

func TestDeleteHomeExpense(t *testing.T) {
	cfg, err := config.LoadConfig()
	if err != nil {
		log.Panicf("Couldn't load config in test Delete Home Expense %v", err)
		t.FailNow()
	}

	testDB := InitTestDB(cfg)
	defer testDB.Close()

	type testCase struct {
		name     string
		input    *models.HouseExpense
		setup    func(t *testing.T, he *models.HouseExpense)
		wantErr  bool
		validate func(t *testing.T, got bool)
	}

	expenseDate := time.Now()
	tests := []testCase{
		{
			name: "Has existing expense",
			input: &models.HouseExpense{
				Amount:        250.00,
				ExpenseDate:   expenseDate,
				UtilityTypeID: 3,
				Notes:         "Test 1234",
			},
			setup: func(t *testing.T, he *models.HouseExpense) {
				testDB.CreateUser(TestUserRegisterModel)
				he.CreatedBy = TestUserRegisterModel.ID
				err := testDB.CreateHouseExpense(he)
				if err != nil {
					t.Skipf("Error setting up deleting existing expense test: %v", err)
				}
			},
			wantErr: false,
			validate: func(t *testing.T, got bool) {
				assert.True(t, got)
			},
		},
		{
			name: "No existing expense",
			setup: func(t *testing.T, he *models.HouseExpense) {
				testDB.CreateUser(TestUserRegisterModel)
				he.CreatedBy = TestUserRegisterModel.ID
				he.ID = 15000
			},
			input: &models.HouseExpense{
				Amount:        250.00,
				ExpenseDate:   expenseDate,
				UtilityTypeID: 0,
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

			got, err := testDB.DeleteHouseExpense(tt.input.ID)

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
