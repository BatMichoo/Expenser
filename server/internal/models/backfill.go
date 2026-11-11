package models

import "github.com/google/uuid"

// BackfillExpense is the expense repeated by a backfill: one per month, of
// one house or car type.
type BackfillExpense struct {
	UserID   uuid.UUID
	Category RecurringPaymentCategory // house or car
	TypeID   int                      // utility type or car expense type
	Amount   float64
	Discount float64
	Notes    string
}
