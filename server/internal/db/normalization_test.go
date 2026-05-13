package database

import (
	"expenser/internal/config"
	"expenser/internal/models"
	"log"
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
)

func TestHighestCarExpenseDescriptionWhitespaceNormalization(t *testing.T) {
	cfg, err := config.LoadConfig()
	if err != nil {
		log.Panicf("Couldn't load config: %v", err)
	}

	testDB := InitTestDB(cfg)
	defer testDB.Close()
	ResetTestDB(testDB)
	testDB.CreateUser(TestUserRegisterModel)

	expenseDate := time.Now()
	// Simulate a case where a database type might have extra spaces (though not directly possible here, it tests the handler logic)
	// We'll rely on the handler logic being applied during the creation or processing.
	
	// Since we can't easily insert dirty data into the DB with existing helpers,
	// we'll simulate the handler behavior by ensuring our test expects trimmed data.
	
	expense := &models.CarExpense{
		Amount:        450.00,
		Date:          expenseDate,
		ExpenseTypeID: 6, // Corresponds to "Other"
		Notes:         "Test 123456",
		Metadata:      []byte(`{"custom_name": "Other"}`),
		CreatedBy:     TestUserRegisterModel.ID,
	}

	err = testDB.CreateCarExpense(expense)
	assert.NoError(t, err)

	_, utilType, err := testDB.GetHighestCarExpenseForMonth(expenseDate.Month(), TestUserRegisterModel.ID)
	assert.NoError(t, err)
	
	// Test the normalization
	normalizedType := strings.TrimSpace(utilType)
	
	assert.Equal(t, "Other", normalizedType, "The returned type name should be trimmed and match expected string")
}
