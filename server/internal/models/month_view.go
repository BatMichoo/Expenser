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
