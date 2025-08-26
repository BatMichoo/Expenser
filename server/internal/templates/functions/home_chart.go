package functions

import (
	"expenser/internal/utilities"
	"time"
)

// DateStartOfCurrentMonth returns the first day of the current month in HTML format.
func StartOfCurrentMonth() string {
	now := time.Now()
	// Create a new time.Time object set to the first day of the current month.
	// The day is set to 1, and the time is set to midnight (00:00:00).
	startOfMonth := time.Date(now.Year(), now.Month(), 1, 0, 0, 0, 0, now.Location())
	return startOfMonth.Format(utilities.DateFormats.HTML)
}

// DateEndOfCurrentMonth returns the last day of the current month in HTML format.
func EndOfCurrentMonth() string {
	now := time.Now()
	// First, find the start of the next month.
	// This is done by adding one month and setting the day to 1.
	startOfNextMonth := time.Date(now.Year(), now.Month()+1, 1, 0, 0, 0, 0, now.Location())
	// Then, subtract one nanosecond from the start of the next month.
	// This takes us to the very end of the previous second, which is the last day of the current month.
	endOfMonth := startOfNextMonth.Add(-time.Nanosecond)
	return endOfMonth.Format(utilities.DateFormats.HTML)
}
