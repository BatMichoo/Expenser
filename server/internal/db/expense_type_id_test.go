package database

import (
	"context"
	"expenser/internal/config"
	"expenser/internal/models"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// The edit forms pre-select the expense's type, so reads by ID must return it.
func TestGetExpenseByIDReturnsTypeID(t *testing.T) {
	cfg, err := config.LoadConfig()
	require.NoError(t, err)
	testDB := InitTestDB(cfg)
	defer testDB.Close()
	ResetTestDB(testDB)

	user := &models.User{Username: "TypeIDTester", PasswordHash: TestUserRegisterModel.PasswordHash}
	require.NoError(t, testDB.CreateUser(context.Background(), user))

	house := &models.HouseExpense{UtilityTypeID: 2, Amount: 10, ExpenseDate: time.Now(), Metadata: []byte("{}"), CreatedBy: user.ID}
	require.NoError(t, testDB.CreateHouseExpense(context.Background(), house))
	gotHouse, err := testDB.GetHouseExpenseByID(context.Background(), house.ID, user.ID)
	require.NoError(t, err)
	assert.Equal(t, 2, gotHouse.UtilityTypeID)

	car := &models.CarExpense{ExpenseTypeID: 3, Amount: 10, Date: time.Now(), Metadata: []byte("{}"), CreatedBy: user.ID}
	require.NoError(t, testDB.CreateCarExpense(context.Background(), car))
	gotCar, err := testDB.GetCarExpenseByID(context.Background(), car.ID, user.ID)
	require.NoError(t, err)
	assert.Equal(t, 3, gotCar.ExpenseTypeID)
}
