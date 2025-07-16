package models

import "time"

type CarExpense struct {
	ID            int       `form:"id"`
	ExpenseTypeID int       `form:"typeID"`
	Type          string    `form:"type" binding:"required"`
	Amount        float64   `form:"amount" binding:"required"`
	Date          time.Time `form:"date" binding:"required"`
	Notes         string    `form:"notes"`
	CreatedAt     time.Time `form:"createdAt"`
}

type CarExpenseType struct {
	ID   int
	Name string
}
