package database

import (
	"database/sql"
	"expenser/internal/config"
	"expenser/internal/models"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// TestExpenseOwnership checks that get, edit and delete by ID only reach the
// owner's expenses.
func TestExpenseOwnership(t *testing.T) {
	cfg, err := config.LoadConfig()
	require.NoError(t, err)

	testDB := InitTestDB(cfg)
	defer testDB.Close()
	ResetTestDB(testDB)

	owner := &models.User{Username: "owner", PasswordHash: TestUserRegisterModel.PasswordHash}
	other := &models.User{Username: "other", PasswordHash: TestUserRegisterModel.PasswordHash}
	require.NoError(t, testDB.CreateUser(testCtx, owner))
	require.NoError(t, testDB.CreateUser(testCtx, other))

	t.Run("Car", func(t *testing.T) {
		exp := &models.CarExpense{ExpenseTypeID: 1, Amount: 10, Date: time.Now(), CreatedBy: owner.ID}
		require.NoError(t, testDB.CreateCarExpense(testCtx, exp))

		got, err := testDB.GetCarExpenseByID(testCtx, exp.ID, other.ID)
		assert.NoError(t, err)
		assert.Nil(t, got)

		exp.Amount = 99
		assert.ErrorIs(t, testDB.EditCarExpense(testCtx, exp, other.ID), sql.ErrNoRows)

		deleted, err := testDB.DeleteCarExpense(testCtx, exp.ID, other.ID)
		assert.NoError(t, err)
		assert.False(t, deleted)

		got, err = testDB.GetCarExpenseByID(testCtx, exp.ID, owner.ID)
		require.NoError(t, err)
		require.NotNil(t, got)
		assert.Equal(t, 10.0, got.Amount)
	})

	t.Run("House", func(t *testing.T) {
		exp := &models.HouseExpense{UtilityTypeID: 1, Amount: 10, ExpenseDate: time.Now(), CreatedBy: owner.ID}
		require.NoError(t, testDB.CreateHouseExpense(testCtx, exp))

		got, err := testDB.GetHouseExpenseByID(testCtx, exp.ID, other.ID)
		assert.NoError(t, err)
		assert.Nil(t, got)

		exp.Amount = 99
		assert.ErrorIs(t, testDB.EditHouseExpense(testCtx, exp, other.ID), sql.ErrNoRows)

		deleted, err := testDB.DeleteHouseExpense(testCtx, exp.ID, other.ID)
		assert.NoError(t, err)
		assert.False(t, deleted)

		got, err = testDB.GetHouseExpenseByID(testCtx, exp.ID, owner.ID)
		require.NoError(t, err)
		require.NotNil(t, got)
		assert.Equal(t, 10.0, got.Amount)
	})

	t.Run("Groceries", func(t *testing.T) {
		exp := &models.GroceriesExpense{UserID: owner.ID, CategoryID: 7, Product: "Milk", Quantity: 1, PurchaseDate: time.Now()}
		require.NoError(t, testDB.CreateGroceriesExpense(testCtx, exp))

		_, err := testDB.GetGroceriesExpenseByID(testCtx, exp.ID, other.ID)
		assert.ErrorIs(t, err, sql.ErrNoRows)

		exp.Product = "Changed"
		assert.ErrorIs(t, testDB.EditGroceriesExpense(testCtx, exp, other.ID), sql.ErrNoRows)

		deleted, err := testDB.DeleteGroceriesExpense(testCtx, exp.ID, other.ID)
		assert.NoError(t, err)
		assert.False(t, deleted)

		got, err := testDB.GetGroceriesExpenseByID(testCtx, exp.ID, owner.ID)
		require.NoError(t, err)
		assert.Equal(t, "Milk", got.Product)
	})
}

// TestExpenseWithoutMetadata checks that expense types without metadata are
// stored as NULL and read back as empty.
func TestExpenseWithoutMetadata(t *testing.T) {
	cfg, err := config.LoadConfig()
	require.NoError(t, err)

	testDB := InitTestDB(cfg)
	defer testDB.Close()
	ResetTestDB(testDB)

	user := &models.User{Username: "nometa", PasswordHash: TestUserRegisterModel.PasswordHash}
	require.NoError(t, testDB.CreateUser(testCtx, user))

	car := &models.CarExpense{ExpenseTypeID: 4, Amount: 8, Date: time.Now(), CreatedBy: user.ID}
	require.NoError(t, testDB.CreateCarExpense(testCtx, car))
	gotCar, err := testDB.GetCarExpenseByID(testCtx, car.ID, user.ID)
	require.NoError(t, err)
	require.NotNil(t, gotCar)
	assert.Empty(t, gotCar.Metadata)

	cars, err := testDB.GetCarExpensesForYear(testCtx, 0, user.ID)
	require.NoError(t, err)
	assert.Len(t, *cars, 1)

	house := &models.HouseExpense{UtilityTypeID: 6, Amount: 20, ExpenseDate: time.Now(), CreatedBy: user.ID}
	require.NoError(t, testDB.CreateHouseExpense(testCtx, house))
	gotHouse, err := testDB.GetHouseExpenseByID(testCtx, house.ID, user.ID)
	require.NoError(t, err)
	require.NotNil(t, gotHouse)
	assert.Empty(t, gotHouse.Metadata)
}
