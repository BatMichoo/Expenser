package services

import (
	"expenser/internal/models"
	"sort"
	"time"
)

// Estimate is the predicted month-end total of the current month.
type Estimate struct {
	Amount   float64
	OK       bool // false when there is too little to go on
	FromPace bool // based only on the daily pace (no history yet)
}

// minPaceDays is how far into the month a pace-only estimate becomes
// worth showing.
const minPaceDays = 7

// estimateHistoryMonths is how far back EstimateMonthEnd looks: the usual
// months, plus the same month and the month after it a year ago for
// seasonal spending.
const estimateHistoryMonths = 12

// EstimateHistoryWindow is the date range EstimateMonthEnd needs history
// for when estimating ym.
func EstimateHistoryWindow(ym YearMonth) (time.Time, time.Time) {
	return ym.Back(estimateHistoryMonths).Start(), ym.End()
}

// EstimateMonthEnd predicts the total of month ym, as seen on day now,
// from per-type daily spending (history must cover EstimateHistoryWindow).
// It works type by type, the way someone who knows the household would
// guess:
//
//   - The usual amount of a type is the median over the recent months it
//     appeared in, so one odd month cannot drag it.
//   - Types seen in under two thirds of recent months (repairs, yearly
//     insurance) are not forecast: only what was spent on them counts.
//   - With a year of history, a type absent in this month last year is
//     not forecast either (no gas in April). One present then and in the
//     month after is a season: last year's amount is blended in, and it
//     is expected even if it skipped the recent months.
//   - Bills (about once a month) are done once paid, and expected at
//     their usual amount until then, however late they run.
//   - Frequent spending (fuel, groceries) adds the part of a usual month
//     that past months still had to come after today's day.
//
// With at most one month of history it falls back to that month's total
// spread over the days left, and with none to the daily pace.
func EstimateMonthEnd(history []models.SpendEntry, ym YearMonth, now time.Time) Estimate {
	h := newSpendHistory(history)
	day := now.Day()
	daysInMonth := ym.End().AddDate(0, 0, -1).Day()

	var avail []YearMonth // recent months the user has history for
	for k := 1; k <= usualMonths; k++ {
		if m := ym.Back(k); h.started && !m.Start().Before(h.first.Start()) {
			avail = append(avail, m)
		}
	}

	spent := 0.0
	for t := range h.types {
		spent += h.total(t, ym)
	}
	if len(avail) <= 1 {
		usual := 0.0
		for _, m := range avail {
			for t := range h.types {
				usual += h.total(t, m)
			}
		}
		if usual > 0 {
			return Estimate{Amount: spent + usual*float64(daysInMonth-day)/float64(daysInMonth), OK: true}
		}
		if day < minPaceDays {
			return Estimate{}
		}
		return Estimate{Amount: spent / float64(day) * float64(daysInMonth), OK: true, FromPace: true}
	}

	lastYearMonth := ym.Back(12)
	hasYear := !lastYearMonth.Start().Before(h.first.Start())
	est := 0.0
	for t := range h.types {
		s := h.total(t, ym)
		var seen []YearMonth
		for _, m := range avail {
			if h.total(t, m) > 0 {
				seen = append(seen, m)
			}
		}
		lastYear := 0.0
		if hasYear && h.total(t, lastYearMonth) > 0 && h.total(t, ym.Back(11)) > 0 {
			lastYear = h.total(t, lastYearMonth)
		}
		switch {
		case hasYear && h.total(t, lastYearMonth) == 0 && len(seen) < len(avail):
			est += s // out of season
			continue
		case len(seen)*3 < len(avail)*2 && lastYear == 0:
			est += s // rare or one-off
			continue
		case len(seen) == 0:
			est += max(s, lastYear) // a season starting again
			continue
		}

		totals := make([]float64, len(seen))
		days, done := 0, 0.0
		for i, m := range seen {
			totals[i] = h.total(t, m)
			days += h.days(t, m)
			done += h.spentBy(t, m, day) / totals[i]
		}
		usual := median(totals)
		if lastYear > 0 {
			usual = (usual + lastYear) / 2
		}
		if float64(days)/float64(len(seen)) <= 1.5 { // a bill
			if s > 0 {
				est += s
			} else {
				est += usual
			}
			continue
		}
		est += s + usual*(1-done/float64(len(seen)))
	}
	return Estimate{Amount: est, OK: true}
}

// spendHistory indexes SpendEntry rows by type and month.
type spendHistory struct {
	types   map[string]map[YearMonth][]models.SpendEntry
	first   YearMonth // earliest month with any spending
	started bool      // any history at all
}

func newSpendHistory(entries []models.SpendEntry) spendHistory {
	h := spendHistory{types: map[string]map[YearMonth][]models.SpendEntry{}}
	for _, e := range entries {
		m := YearMonth{e.Date.Year(), e.Date.Month()}
		if h.types[e.Type] == nil {
			h.types[e.Type] = map[YearMonth][]models.SpendEntry{}
		}
		h.types[e.Type][m] = append(h.types[e.Type][m], e)
		if !h.started || m.Start().Before(h.first.Start()) {
			h.first, h.started = m, true
		}
	}
	return h
}

func (h spendHistory) total(t string, m YearMonth) float64 {
	return h.spentBy(t, m, 31)
}

// spentBy is what type t cost in month m up to and including day d.
func (h spendHistory) spentBy(t string, m YearMonth, d int) float64 {
	sum := 0.0
	for _, e := range h.types[t][m] {
		if e.Date.Day() <= d {
			sum += e.Amount
		}
	}
	return sum
}

// days counts the days in month m that type t had spending on.
func (h spendHistory) days(t string, m YearMonth) int {
	seen := map[int]bool{}
	for _, e := range h.types[t][m] {
		seen[e.Date.Day()] = true
	}
	return len(seen)
}

func median(v []float64) float64 {
	s := append([]float64(nil), v...)
	sort.Float64s(s)
	if n := len(s); n%2 == 1 {
		return s[n/2]
	} else {
		return (s[n/2-1] + s[n/2]) / 2
	}
}
