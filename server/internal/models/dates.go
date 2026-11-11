package models

import "time"

// Calendar days (due dates, expense dates, month and year boundaries) are
// midnight UTC of that day, whatever the server's time zone. Date builds
// one; DateOnly turns any time into the day it falls on in its own location.
// Expense forms parse "2006-01-02" with time.Parse, which gives the same.

// Date is midnight UTC of the given day. Out-of-range values normalise like
// time.Date (day 0 is the last day of the month before).
func Date(year int, month time.Month, day int) time.Time {
	return time.Date(year, month, day, 0, 0, 0, 0, time.UTC)
}

// DateOnly strips the time of day, returning midnight UTC of t's calendar
// date, so time of day and time zone don't affect date comparisons.
func DateOnly(t time.Time) time.Time {
	y, m, d := t.Date()
	return Date(y, m, d)
}
