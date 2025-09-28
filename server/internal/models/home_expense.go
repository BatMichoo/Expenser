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
