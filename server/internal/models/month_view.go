package models

import "time"

// MonthSummary holds the KPI figures for one month of one section.
type MonthSummary struct {
	Total     float64
	Count     int
	Average   float64
	TopType   string // type (house/car) or category (groceries) with the largest sum
	TopAmount float64
}

// MonthBar is one bar of the "last months" mini chart.
type MonthBar struct {
	Month    string     // YYYY-MM, used in links
	MonthNum time.Month // for the translated short label
	Total    float64
	Height   float64 // 0–100, relative to the largest month shown
	Current  bool    // the month being viewed
}

// TypeOption is one entry of the type/category filter.
type TypeOption struct {
	ID   int
	Name string // untranslated; templates prefix it with TypePrefix
}

// MonthView is everything one section page shows for one month.
type MonthView struct {
	Section          string // "house", "car" or "groceries"
	Month            string // YYYY-MM being viewed
	MonthNum         time.Month
	Year             int
	PrevMonth        string
	NextMonth        string
	IsCurrent        bool // viewing the current calendar month
	Summary          MonthSummary
	ChangePct        float64
	HasChange        bool // false when last month had no spending to compare with
	Bars             []MonthBar
	BarsTotal        float64 // sum of the months shown in the mini chart
	Query            string
	TypeID           int
	Types            []TypeOption
	TypePrefix       string      // "type." or "category." for translation keys
	Filtered         bool        // a search or type filter is active
	Breakdown        []TypeShare // this month by type, largest first
	YearSoFar        YearSummary
	Estimate         float64 // predicted month-end total (current month only)
	HasEstimate      bool
	EstimateFromPace bool              // no history yet: based on the daily pace alone
	UpcomingBills    []UpcomingPayment // unpaid bills for this section, next 30 days
	House            []HouseExpense
	Car              []CarExpense
	Groceries        []GroceriesExpense
	Lang             string
}

// TypeShare is one type's slice of a month's spending, compared with its
// usual monthly amount.
type TypeShare struct {
	Type     string
	Amount   float64
	Share    float64 // percent of the month's total
	Usual    float64 // average per month over the comparison window
	HasUsual bool
	VsUsual  float64 // percent above (+) or below (-) usual
}

// YearSummary is spending from January up to the viewed month.
type YearSummary struct {
	Total         float64
	Months        int
	Average       float64
	VsAverage     float64 // viewed month vs the year's monthly average, percent
	HasComparison bool
}

// SpendEntry is what one type cost on one day: the history the month-end
// estimate learns from.
type SpendEntry struct {
	Type   string
	Date   time.Time
	Amount float64
}
