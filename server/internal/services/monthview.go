package services

import (
	"expenser/internal/models"
	"fmt"
	"sort"
	"strings"
	"time"
)

// YearMonth identifies a calendar month, the unit the expense views browse by.
type YearMonth struct {
	Year  int
	Month time.Month
}

// ParseYearMonth reads a "YYYY-MM" value; anything missing or invalid falls
// back to the month containing now.
func ParseYearMonth(s string, now time.Time) YearMonth {
	if t, err := time.Parse("2006-01", strings.TrimSpace(s)); err == nil {
		return YearMonth{t.Year(), t.Month()}
	}
	return YearMonth{now.Year(), now.Month()}
}

func (ym YearMonth) String() string {
	return fmt.Sprintf("%04d-%02d", ym.Year, int(ym.Month))
}

// Start is the first day of the month (midnight UTC, see models.Date).
func (ym YearMonth) Start() time.Time {
	return models.Date(ym.Year, ym.Month, 1)
}

// End is the first instant of the following month (exclusive bound).
func (ym YearMonth) End() time.Time {
	return ym.Start().AddDate(0, 1, 0)
}

func (ym YearMonth) Prev() YearMonth {
	t := ym.Start().AddDate(0, -1, 0)
	return YearMonth{t.Year(), t.Month()}
}

// Back is the month k months before ym.
func (ym YearMonth) Back(k int) YearMonth {
	t := ym.Start().AddDate(0, -k, 0)
	return YearMonth{t.Year(), t.Month()}
}

func (ym YearMonth) Next() YearMonth {
	t := ym.End()
	return YearMonth{t.Year(), t.Month()}
}

// SummaryItem is one expense reduced to what the KPIs need.
type SummaryItem struct {
	Type   string
	Amount float64
}

// Summarize computes the month KPIs. The "top" type is the one with the
// largest summed amount, matching the existing highest-expense card.
func Summarize(items []SummaryItem) models.MonthSummary {
	var s models.MonthSummary
	perType := map[string]float64{}
	for _, it := range items {
		s.Total += it.Amount
		s.Count++
		perType[it.Type] += it.Amount
	}
	if s.Count > 0 {
		s.Average = s.Total / float64(s.Count)
	}
	for typ, amount := range perType {
		if amount > s.TopAmount || (amount == s.TopAmount && typ < s.TopType) {
			s.TopType, s.TopAmount = typ, amount
		}
	}
	return s
}

// ChangePercent is the change from prev to cur in percent. ok is false when
// there is no previous amount to compare with.
func ChangePercent(cur, prev float64) (pct float64, ok bool) {
	if prev <= 0 {
		return 0, false
	}
	return (cur - prev) / prev * 100, true
}

// Bars returns n months ending at end (oldest first) with heights relative
// to the largest total. totals is keyed by YearMonth.String().
func Bars(totals map[string]float64, end YearMonth, n int) []models.MonthBar {
	months := make([]YearMonth, n)
	ym := end
	for i := n - 1; i >= 0; i-- {
		months[i] = ym
		ym = ym.Prev()
	}

	max := 0.0
	for _, m := range months {
		if totals[m.String()] > max {
			max = totals[m.String()]
		}
	}

	bars := make([]models.MonthBar, n)
	for i, m := range months {
		total := totals[m.String()]
		height := 0.0
		if max > 0 {
			height = total / max * 100
		}
		bars[i] = models.MonthBar{
			Month:    m.String(),
			MonthNum: m.Month,
			Total:    total,
			Height:   height,
			Current:  m == end,
		}
	}
	return bars
}

// MatchesQuery reports whether any field contains q, ignoring case and
// surrounding space. An empty query matches everything.
func MatchesQuery(q string, fields ...string) bool {
	q = strings.ToLower(strings.TrimSpace(q))
	if q == "" {
		return true
	}
	for _, f := range fields {
		if strings.Contains(strings.ToLower(f), q) {
			return true
		}
	}
	return false
}

// Breakdown splits a month's spending by type, largest first. typeTotals
// holds each type's summed spending over the previous `months` months and
// gives the "usual" monthly amount to compare with.
func Breakdown(items []SummaryItem, typeTotals map[string]float64, months int) []models.TypeShare {
	perType := map[string]float64{}
	order := []string{}
	total := 0.0
	for _, it := range items {
		if _, seen := perType[it.Type]; !seen {
			order = append(order, it.Type)
		}
		perType[it.Type] += it.Amount
		total += it.Amount
	}

	shares := make([]models.TypeShare, 0, len(order))
	for _, typ := range order {
		s := models.TypeShare{Type: typ, Amount: perType[typ]}
		if total > 0 {
			s.Share = s.Amount / total * 100
		}
		if months > 0 && typeTotals[typ] > 0 {
			s.Usual = typeTotals[typ] / float64(months)
			s.HasUsual = true
			s.VsUsual = (s.Amount - s.Usual) / s.Usual * 100
		}
		shares = append(shares, s)
	}
	sort.SliceStable(shares, func(i, j int) bool { return shares[i].Amount > shares[j].Amount })
	return shares
}

// YearSoFar sums January through the viewed month of its year. totals is
// keyed by YearMonth.String().
func YearSoFar(totals map[string]float64, ym YearMonth) models.YearSummary {
	y := models.YearSummary{Months: int(ym.Month)}
	for m := time.January; m <= ym.Month; m++ {
		y.Total += totals[YearMonth{ym.Year, m}.String()]
	}
	y.Average = y.Total / float64(y.Months)
	if y.Months > 1 {
		y.VsAverage, y.HasComparison = ChangePercent(totals[ym.String()], y.Average)
	}
	return y
}
