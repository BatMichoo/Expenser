package database

import (
	"expenser/internal/config"
	"expenser/internal/models"
	"log"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
)

func TestHighestCarExpenseDescriptionTranslation(t *testing.T) {
	cfg, err := config.LoadConfig()
	if err != nil {
		log.Panicf("Couldn't load config: %v", err)
	}

	testDB := InitTestDB(cfg)
	defer testDB.Close()
	ResetTestDB(testDB)
	testDB.CreateUser(TestUserRegisterModel)

	expenseDate := time.Now()
	expense := &models.CarExpense{
		Amount:        450.00,
		Date:          expenseDate,
		ExpenseTypeID: 6, // Corresponds to "Other"
		Notes:         "Test 123456",
		Metadata:      nil,
		CreatedBy:     TestUserRegisterModel.ID,
	}

	err = testDB.CreateCarExpense(expense)
	assert.NoError(t, err)

	_, utilType, err := testDB.GetHighestCarExpenseForMonth(expenseDate.Month(), TestUserRegisterModel.ID)
	assert.NoError(t, err)

	// Check if the utilType returned matches a key in i18n
	// Based on i18n/en.json: "type.Other": "Other"
	assert.Equal(t, "Other", utilType, "The returned type name should match the i18n key suffix")
}
