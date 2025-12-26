package models

import (
	"time"

	"github.com/google/uuid"
)

type CarExpense struct {
	ID            int       `form:"id"`
	ExpenseTypeID int       `form:"typeID"`
	Type          string    `form:"type" binding:"required"`
	Amount        float64   `form:"amount" binding:"required"`
	Date          time.Time `form:"date" binding:"required"`
	Notes         string    `form:"notes"`
	CreatedAt     time.Time `form:"createdAt"`
	CreatedBy     uuid.UUID
}

type CarExpenseType struct {
	ID   int
	Name string
}

// CarExpResponse is the data structure returned to the client
// after a new expense has been successfully created.
// It includes details of the newly created expense and updated summary data.
type CarExpResponse struct {
	Expense        *CarExpense     // Expense is the newly created car expense record.
	MonthlyExpense *MonthlyExpense // MonthlyExpense provides the updated total for the current month.
	HighestExpense *HighestExpense // HighestExpense provides the updated highest expense for the current month.
	Modal          *ModalContent
}
