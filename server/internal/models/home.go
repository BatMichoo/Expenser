package models

import "time"

// HomeView is the logged-in home page: a greeting and a card per section.
type HomeView struct {
	Name      string // display name, else username
	House     HomeCategory
	Car       HomeCategory
	Groceries HomeCategory
	Bills     []HomeBillRow // the next few active bills, soonest first
	MoreBills int           // active bills not shown
	Lang      string
}

// HomeCategory is one expense section's card: this year so far against the
// same stretch of last year, this month, the biggest share and 12 bars.
type HomeCategory struct {
	Section       string // house, car or groceries; also the tab's path
	YearTotal     float64
	LastYearTotal float64
	HasLastYear   bool    // last year had spending by this date
	ChangePct     float64 // vs LastYearTotal, when HasLastYear
	ThisMonth     float64
	TopName       string  // biggest type (house, car) or store (groceries)
	TopPct        float64 // its share of the year, 0–100
	Bars          []MonthBar
	Empty         bool // nothing spent this year
}

// HomeBillRow is an upcoming bill with what its type cost in the same month
// last year, when anything was paid then.
type HomeBillRow struct {
	Payment       RecurringPayment
	DaysLeft      int
	LastYear      float64
	HasLastYear   bool
	LastYearMonth time.Month
}
