package models

import (
	"time"

	"github.com/google/uuid"
)

type HouseExpense struct {
	ID            int       `form:"id"`
	UtilityTypeID int       `form:"typeID"`
	UtilityType   string    `form:"type" binding:"required"`
	Amount        float64   `form:"amount" binding:"required"`
	ExpenseDate   time.Time `form:"date" binding:"required"`
	Notes         string    `form:"notes"`
	CreatedAt     time.Time `form:"createdAt"`
	CreatedBy     uuid.UUID
}

type HomeUtilityType struct {
	ID   int
	Name string
}

// HouseExpResponse is the data structure returned to the client
// after a new expense has been successfully created.
// It includes details of the newly created expense and updated summary data.
type HouseExpResponse struct {
	Expense        *HouseExpense   // Expense is the newly created home expense record.
	MonthlyExpense *MonthlyExpense // MonthlyExpense provides the updated total for the current month.
	HighestExpense *HighestExpense // HighestExpense provides the updated highest expense for the current month.
	Modal          *ModalContent
}
