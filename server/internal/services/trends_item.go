package services

import (
	"context"
	"expenser/internal/chart"
	"expenser/internal/models"
	"github.com/google/uuid"
	"sort"
	"strconv"
	"time"
)

var unitOrder = []string{"kg", "l", "pc"}

const maxDiscountLines = 5

// MainUnit returns the unit most known lines are measured in (ties go in the
// order kg, l, pc) and every unit with at least one known line. Lines with an
// unknown size never count.
func MainUnit(lines []models.GroceryLine) (main string, all []string) {
	count := map[string]int{}
	for _, l := range lines {
		if l.SizeStatus == models.SizeKnown {
			count[l.Unit]++
		}
	}
	best := 0
	for _, u := range unitOrder {
		if count[u] > 0 {
			all = append(all, u)
			if count[u] > best {
				main, best = u, count[u]
			}
		}
	}
	return main, all
}

// BaseQty is the amount bought in the line's unit (kg, l or pieces).
func BaseQty(l models.GroceryLine) float64 { return l.Quantity * l.UnitSize }

func netPrice(l models.GroceryLine) float64 { return l.TotalPrice - l.TotalDiscount }

// usable reports whether a line may feed price figures in unit: a known size,
// that unit, a positive amount and a positive net price.
func usable(l models.GroceryLine, unit string) bool {
	return l.SizeStatus == models.SizeKnown && l.Unit == unit && BaseQty(l) > 0 && netPrice(l) > 0
}

// PeriodStat is the price picture of one period (a month or a year).
type PeriodStat struct {
	Key                     string
	Avg, Lo, Hi, Shelf, Qty float64 // per unit, except Qty (base amount)
	N                       int
}

func periodKey(t time.Time, by string) string {
	if by == "year" {
		return strconv.Itoa(t.Year())
	}
	return t.Format("2006-01")
}

// PriceStats groups usable lines by period ("month" or "year") and returns
// the weighted average net price per unit, the cheapest and dearest line, the
// weighted shelf (before discount) price and the amount bought.
func PriceStats(lines []models.GroceryLine, unit string, by string) map[string]PeriodStat {
	type acc struct {
		net, shelf float64
		PeriodStat
	}
	accs := map[string]*acc{}
	for _, l := range lines {
		if !usable(l, unit) {
			continue
		}
		k := periodKey(l.Date, by)
		a := accs[k]
		if a == nil {
			a = &acc{PeriodStat: PeriodStat{Key: k}}
			accs[k] = a
		}
		q, per := BaseQty(l), netPrice(l)/BaseQty(l)
		if a.N == 0 || per < a.Lo {
			a.Lo = per
		}
		if a.N == 0 || per > a.Hi {
			a.Hi = per
		}
		a.net += netPrice(l)
		a.shelf += l.TotalPrice
		a.Qty += q
		a.N++
	}
	out := make(map[string]PeriodStat, len(accs))
	for k, a := range accs {
		a.Avg, a.Shelf = a.net/a.Qty, a.shelf/a.Qty
		out[k] = a.PeriodStat
	}
	return out
}

// rangeStart is the first day of the first period key.
func rangeStart(key string) time.Time {
	if len(key) == 4 {
		y, _ := strconv.Atoi(key)
		return models.Date(y, 1, 1)
	}
	t, _ := time.Parse("2006-01", key)
	return models.Date(t.Year(), t.Month(), 1)
}

// priorYearKey is the key of the same period one year earlier.
func priorYearKey(key string) string {
	if len(key) == 4 {
		y, _ := strconv.Atoi(key)
		return strconv.Itoa(y - 1)
	}
	t, _ := time.Parse("2006-01", key)
	return t.AddDate(-1, 0, 0).Format("2006-01")
}

func periodLabels(periods []string, lang string) []string {
	labels := make([]string, len(periods))
	multiYear := len(periods) > 0 && len(periods[0]) > 4 && periods[0][:4] != periods[len(periods)-1][:4]
	for i, k := range periods {
		if len(k) == 4 {
			labels[i] = k
			continue
		}
		labels[i] = shortMonth(lang, monthOf(k))
		if multiYear {
			labels[i] += " '" + k[2:4]
		}
	}
	return labels
}

// BuildItemTrend builds the item detail view of one category. lines are the
// item's lines (they may reach back before the first period, which is only
// used for the same-period-last-year comparison), all are every grocery line
// for the all-items table, periods the charted period keys, oldest first.
func BuildItemTrend(lines []models.GroceryLine, all []models.GroceryLine, item models.Category, unit, by string, periods []string, now time.Time, lang string) models.ItemTrend {
	t := models.ItemTrend{ItemID: item.ID, Item: item.Name, By: by, Items: BuildItemRows(all, now)}
	if len(periods) == 0 {
		t.Empty = true
		return t
	}
	start := rangeStart(periods[0])
	var inRange []models.GroceryLine
	for _, l := range lines {
		if !l.Date.Before(start) {
			inRange = append(inRange, l)
			t.TotalSaved += l.TotalDiscount
		}
	}
	main, units := MainUnit(inRange)
	t.Units = units
	t.Unit = unit
	if !containsStr(units, unit) {
		t.Unit = main
	}
	for _, l := range inRange {
		if !usable(l, t.Unit) {
			t.Excluded++
		}
	}
	stats := PriceStats(lines, t.Unit, by)
	inPeriods := map[string]bool{}
	for _, k := range periods {
		inPeriods[k] = true
	}
	hasAny := false
	for _, k := range periods {
		if _, ok := stats[k]; ok {
			hasAny = true
		}
	}
	if t.Unit == "" || !hasAny {
		t.Empty = true
		return t
	}

	avg := make([]*float64, len(periods))
	lo, hi, shelf := make([]*float64, len(periods)), make([]*float64, len(periods)), make([]*float64, len(periods))
	qty := make([]float64, len(periods))
	curKey := periodKey(now, by)
	var prevQty float64
	prevN := 0
	for i, k := range periods {
		if s, ok := stats[k]; ok {
			a, l, h, sh := s.Avg, s.Lo, s.Hi, s.Shelf
			avg[i], lo[i], hi[i], shelf[i] = &a, &l, &h, &sh
			qty[i] = s.Qty
			t.PriceNow, t.HasPriceNow = s.Avg, true
			if prior, ok := stats[priorYearKey(k)]; ok {
				t.PriceChangePct, t.HasPriceChange = ChangePercent(s.Avg, prior.Avg)
			} else {
				t.PriceChangePct, t.HasPriceChange = 0, false
			}
		}
		switch {
		case k == curKey:
			t.QtyNow = qty[i]
		case k < curKey:
			prevQty += qty[i]
			prevN++
		}
	}
	if prevN > 0 {
		t.QtyUsual, t.HasQtyUsual = prevQty/float64(prevN), true
	}
	t.Chart = chart.Price(chart.DefaultFrame, periodLabels(periods, lang), avg, lo, hi, shelf, qty)

	var priced, discounted []models.PriceLine
	for _, l := range inRange {
		if !usable(l, t.Unit) || !inPeriods[periodKey(l.Date, by)] {
			continue
		}
		pl := models.PriceLine{Date: l.Date, Store: l.Store, Product: l.Product,
			PerUnit: netPrice(l) / BaseQty(l), DiscountPerUnit: l.TotalDiscount / BaseQty(l)}
		priced = append(priced, pl)
		if pl.DiscountPerUnit > 0 {
			discounted = append(discounted, pl)
		}
	}
	if len(priced) > 0 {
		c := priced[0]
		for _, p := range priced[1:] {
			if p.PerUnit < c.PerUnit {
				c = p
			}
		}
		t.Cheapest = &c
	}
	sort.SliceStable(discounted, func(i, j int) bool { return discounted[i].DiscountPerUnit > discounted[j].DiscountPerUnit })
	if len(discounted) > maxDiscountLines {
		discounted = discounted[:maxDiscountLines]
	}
	t.Discounts = discounted
	return t
}

func containsStr(s []string, v string) bool {
	for _, x := range s {
		if x == v {
			return true
		}
	}
	return false
}

// BuildItemRows builds the all-items price-change table from the last 24
// months of lines. Per category it uses the main unit; Now is the weighted
// average net price over the last 90 days, YearAgo the same 90 days a year
// earlier. Rows with a change come first (largest rise first), then the rest by
// name.
func BuildItemRows(all []models.GroceryLine, now time.Time) []models.ItemRow {
	today := models.DateOnly(now)
	nowFrom, yearFrom := today.AddDate(0, 0, -90), today.AddDate(-1, 0, -90)
	yearTo, boughtFrom := today.AddDate(-1, 0, 0), today.AddDate(-1, 0, 0)
	byCat := map[int][]models.GroceryLine{}
	for _, l := range all {
		byCat[l.CategoryID] = append(byCat[l.CategoryID], l)
	}
	window := func(lines []models.GroceryLine, unit string, after, upTo time.Time) (price, qty float64) {
		var net float64
		for _, l := range lines {
			if usable(l, unit) && l.Date.After(after) && !l.Date.After(upTo) {
				net += netPrice(l)
				qty += BaseQty(l)
			}
		}
		if qty > 0 {
			price = net / qty
		}
		return price, qty
	}
	var rows []models.ItemRow
	for id, lines := range byCat {
		unit, _ := MainUnit(lines)
		if unit == "" {
			continue
		}
		r := models.ItemRow{ItemID: id, Item: lines[0].Category, Unit: unit}
		r.Now, _ = window(lines, unit, nowFrom, today)
		r.YearAgo, _ = window(lines, unit, yearFrom, yearTo)
		_, r.Bought12 = window(lines, unit, boughtFrom, today)
		if r.Now == 0 && r.YearAgo == 0 && r.Bought12 == 0 {
			continue
		}
		if r.Now > 0 && r.YearAgo > 0 {
			r.ChangePct, r.HasChange = ChangePercent(r.Now, r.YearAgo)
		}
		rows = append(rows, r)
	}
	sort.Slice(rows, func(i, j int) bool {
		a, b := rows[i], rows[j]
		if a.HasChange != b.HasChange {
			return a.HasChange
		}
		if a.HasChange && a.ChangePct != b.ChangePct {
			return a.ChangePct > b.ChangePct
		}
		return a.Item < b.Item
	})
	return rows
}

// itemTrend fills the item tab of p. months is the charted month range, or
// invalid/empty for period "all".
func (s *TrendsService) itemTrend(ctx context.Context, userID uuid.UUID, p *models.TrendsPage, months []YearMonth, lang string, now time.Time) error {
	q := p.Query
	cats, err := s.store.GetTrackedCategories(ctx)
	if err != nil {
		return err
	}
	p.Items = cats
	if q.By != "year" {
		q.By = "month"
	}
	cur := YearMonth{now.Year(), now.Month()}
	from := now.AddDate(-2, 0, 0)
	if q.Period == "all" {
		from = models.Date(2000, 1, 1)
	} else {
		from = earliest(from, months[0].Start().AddDate(-1, 0, 0))
	}
	all, err := s.store.GetGroceryLines(ctx, userID, 0, from, cur.End())
	if err != nil {
		return err
	}
	if q.Period == "all" {
		first := cur.Start()
		for _, l := range all {
			if l.Date.Before(first) {
				first = l.Date
			}
		}
		n := (now.Year()-first.Year())*12 + int(now.Month()) - int(first.Month()) + 1
		months = make([]YearMonth, n)
		for i := range months {
			months[i] = cur.Back(n - 1 - i)
		}
	}
	rangeFrom := months[0].Start()

	var periods []string
	if q.By == "year" {
		for y := rangeFrom.Year(); y <= now.Year(); y++ {
			periods = append(periods, strconv.Itoa(y))
		}
		rangeFrom = models.Date(rangeFrom.Year(), 1, 1)
	} else {
		for _, m := range months {
			periods = append(periods, m.String())
		}
	}

	// Pick the item: the asked one if tracked, else the most bought.
	var item models.Category
	for _, c := range cats {
		if c.ID == q.ItemID {
			item = c
		}
	}
	if item.ID == 0 && len(cats) > 0 {
		known := map[int]int{}
		for _, l := range all {
			if l.SizeStatus == models.SizeKnown && !l.Date.Before(rangeFrom) {
				known[l.CategoryID]++
			}
		}
		item = cats[0]
		for _, c := range cats[1:] {
			if known[c.ID] > known[item.ID] {
				item = c
			}
		}
	}
	if item.ID == 0 {
		p.Item = &models.ItemTrend{By: q.By, Empty: true, Items: BuildItemRows(all, now)}
		return nil
	}
	q.ItemID = item.ID
	var lines []models.GroceryLine
	for _, l := range all {
		if l.CategoryID == item.ID {
			lines = append(lines, l)
		}
	}
	t := BuildItemTrend(lines, all, item, q.Unit, q.By, periods, now, lang)
	q.Unit = t.Unit
	p.Query = q
	p.Item = &t
	return nil
}

func earliest(a, b time.Time) time.Time {
	if b.Before(a) {
		return b
	}
	return a
}
