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
	Section    string // "house", "car" or "groceries"
	Month      string // YYYY-MM being viewed
	MonthNum   time.Month
	Year       int
	PrevMonth  string
	NextMonth  string
	IsCurrent  bool // viewing the current calendar month
	Summary    MonthSummary
	ChangePct  float64
	HasChange  bool // false when last month had no spending to compare with
	Bars       []MonthBar
	Query      string
	TypeID     int
	Types      []TypeOption
	TypePrefix string // "type." or "category." for translation keys
	Filtered   bool   // a search or type filter is active
	House      []HouseExpense
	Car        []CarExpense
	Groceries  []GroceriesExpense
	Lang       string
}
