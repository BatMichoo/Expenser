package models

// HighestExpense represents the expense with the highest amount for a given period.
// It includes the amount, the type of utility (e.g., "Electricity", "Water"),
// and an 'IsOOB' flag indicating if HTMX should update it out of bounds.
type HighestExpense struct {
	Amount float64
	Type   string
	IsOOB  bool
}

// MonthlyExpense summarizes the total expense for a specific month.
// It includes the aggregated amount, the name of the month,
// and an 'IsOOB' flag.
type MonthlyExpense struct {
	Amount float64
	Month  string
	IsOOB  bool
}
