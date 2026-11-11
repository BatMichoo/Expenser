package services

import (
	"expenser/internal/models"
	"sort"
	"time"
)

// dueSoonDays is the minimum look-ahead for "due soon"; a bill with a
// longer reminder window is due soon as soon as its reminder would fire.
const dueSoonDays = 14

// BuildBillsView groups payments by urgency (overdue, waiting, due soon,
// later, paused), each soonest first, and computes the page totals.
func BuildBillsView(payments []models.RecurringPayment, today time.Time) models.BillsView {
	today = models.DateOnly(today)
	monthEnd := time.Date(today.Year(), today.Month()+1, 0, 0, 0, 0, 0, time.UTC)

	sorted := append([]models.RecurringPayment(nil), payments...)
	sort.SliceStable(sorted, func(i, j int) bool { return sorted[i].DueDate.Before(sorted[j].DueDate) })

	var v models.BillsView
	for _, p := range sorted {
		row := models.BillRow{Payment: p, DaysLeft: p.DaysUntilDue(today), Missed: p.MissedCycles(today)}
		v.Total++

		if !p.Active {
			v.Paused = append(v.Paused, row)
			continue
		}
		if !models.DateOnly(p.DueDate).After(monthEnd) && p.ExpectedAmount != nil {
			v.DueThisMonth += *p.ExpectedAmount
		}

		soon := dueSoonDays
		if p.RemindDaysBefore > soon {
			soon = p.RemindDaysBefore
		}
		switch {
		case p.IsWaiting(today):
			v.Waiting = append(v.Waiting, row)
		case row.DaysLeft < 0:
			v.Overdue = append(v.Overdue, row)
		case row.DaysLeft <= soon:
			v.DueSoon = append(v.DueSoon, row)
			if v.Next == nil {
				r := row
				v.Next = &r
			}
		default:
			v.Later = append(v.Later, row)
			if v.Next == nil {
				r := row
				v.Next = &r
			}
		}
	}
	v.OverdueCount = len(v.Overdue)
	v.WaitingCount = len(v.Waiting)
	return v
}
