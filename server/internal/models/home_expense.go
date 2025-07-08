package models

import "time"

type HomeExpense struct {
	ID          int       `form:"id"`
	UtilityType string    `form:"type" binding:"required"`
	Amount      float64   `form:"amount" binding:"required"`
	ExpenseDate time.Time `form:"date" binding:"required"`
	Notes       string    `form:"notes"`
	CreatedAt   time.Time `form:"createdAt"`
}
