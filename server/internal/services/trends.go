package services

import (
	"context"
	"expenser/internal/chart"
	"expenser/internal/models"
	"expenser/internal/utilities"
	"sort"
	"strconv"
	"strings"
	"time"

	"github.com/google/uuid"
)

// TrendsStore is the data the trends views read.
type TrendsStore interface {
	GetMonthTypeTotals(ctx context.Context, userID uuid.UUID, s models.Section, from, to time.Time) ([]models.MonthTypeTotal, error)
	GetYearTotals(ctx context.Context, userID uuid.UUID, s models.Section) (map[int]float64, error)
	GetGroceriesDiscountTotal(ctx context.Context, userID uuid.UUID, from, to time.Time) (float64, error)
	GetGroceryLines(ctx context.Context, userID uuid.UUID, categoryID int, from, to time.Time) ([]models.GroceryLine, error)
	CountUnsizedGroceries(ctx context.Context, userID uuid.UUID) (int, error)
	GetTrackedCategories(ctx context.Context) ([]models.Category, error)
	GetSectionTypes(ctx context.Context, s models.Section) ([]models.TypeOption, error)
	GetAllCategories(ctx context.Context) ([]models.Category, error)
}

type TrendsService struct{ store TrendsStore }

func NewTrendsService(store TrendsStore) *TrendsService { return &TrendsService{store: store} }

const (
	defaultPeriod = "12"
	topCategories = 6
	otherCategory = "Other"
)

// PeriodMonths expands a period ("6", "12", "24" months ending with now's
// month, or "y<YYYY>" for a calendar year) into months, oldest first.
func PeriodMonths(period string, now time.Time) ([]YearMonth, bool) {
	var n int
	var end YearMonth
	switch period {
	case "6", "12", "24":
		n, _ = strconv.Atoi(period)
		end = YearMonth{now.Year(), now.Month()}
	default:
		y, err := strconv.Atoi(strings.TrimPrefix(period, "y"))
		if !strings.HasPrefix(period, "y") || err != nil || y < 1 || y > 9999 {
			return nil, false
		}
		n, end = 12, YearMonth{y, time.December}
	}
	months := make([]YearMonth, n)
	for i := range months {
		months[i] = end.Back(n - 1 - i)
	}
	return months, true
}

// Median of v; 0 for no values. v is not modified.
func Median(v []float64) float64 {
	if len(v) == 0 {
		return 0
	}
	s := append([]float64(nil), v...)
	sort.Float64s(s)
	if len(s)%2 == 1 {
		return s[len(s)/2]
	}
	return (s[len(s)/2-1] + s[len(s)/2]) / 2
}

// SoFarWindow returns the [from,to) ranges to compare year A with year B.
// When either year is the current one, both windows run from 1 January to
// today's date (clamped to the last day of that month in a shorter year, so
// 29 Feb compares with 28 Feb); otherwise they are the full years.
func SoFarWindow(yearA, yearB int, now time.Time) (fromA, toA, fromB, toB time.Time) {
	fromA, fromB = models.Date(yearA, 1, 1), models.Date(yearB, 1, 1)
	if yearA != now.Year() && yearB != now.Year() {
		return fromA, models.Date(yearA+1, 1, 1), fromB, models.Date(yearB+1, 1, 1)
	}
	today := models.DateOnly(now)
	cut := func(year int) time.Time {
		day := today.Day()
		if last := models.Date(year, today.Month()+1, 0).Day(); day > last {
			day = last
		}
		return models.Date(year, today.Month(), day).AddDate(0, 0, 1)
	}
	return fromA, cut(yearA), fromB, cut(yearB)
}

func monthOf(key string) time.Month {
	if len(key) >= 7 {
		if m, err := strconv.Atoi(key[5:7]); err == nil && m >= 1 && m <= 12 {
			return time.Month(m)
		}
	}
	return 0
}

func shortMonth(lang string, m time.Month) string {
	return utilities.T(lang, "month_short."+m.String())
}

func sumRows(rows []models.MonthTypeTotal, typeID int) float64 {
	t := 0.0
	for _, r := range rows {
		if typeID == 0 || r.TypeID == typeID {
			t += r.Amount
		}
	}
	return t
}

// namedTotal is a type's name and amount, used for ranking.
type namedTotal struct {
	id     int
	name   string
	amount float64
}

func rankTypes(totals map[int]*namedTotal) []namedTotal {
	out := make([]namedTotal, 0, len(totals))
	for _, t := range totals {
		out = append(out, *t)
	}
	sort.Slice(out, func(i, j int) bool {
		if out[i].amount != out[j].amount {
			return out[i].amount > out[j].amount
		}
		return out[i].name < out[j].name
	})
	return out
}

// BuildMonthTrend builds the month-to-month tab from the period's rows and
// the previous period's rows. typeID 0 means all types.
//
// Series names are raw type names (templates translate them with
// TrendsPage.TypePrefix) except the colour-0 "Everything else" series, which
// is already translated. For groceries the "Other" category never has its own
// series: it folds into "Everything else" together with all but the top 6
// other categories by period total. Totals still include Other.
func BuildMonthTrend(rows, prevRows []models.MonthTypeTotal, months []YearMonth, typeID int, groceries bool, lang string) models.MonthTrend {
	idx := make(map[string]int, len(months))
	for i, m := range months {
		idx[m.String()] = i
	}
	multiYear := len(months) > 0 && months[0].Year != months[len(months)-1].Year
	labels := make([]string, len(months))
	for i, m := range months {
		labels[i] = shortMonth(lang, m.Month)
		if multiYear {
			labels[i] += " '" + strconv.Itoa(m.Year%100/10) + strconv.Itoa(m.Year%10)
		}
	}

	totals := map[int]*namedTotal{}
	monthTotals := make([]float64, len(months))
	perMonthType := make([]map[int]*namedTotal, len(months))
	for _, r := range rows {
		i, ok := idx[r.Month]
		if !ok || (typeID != 0 && r.TypeID != typeID) {
			continue
		}
		t := totals[r.TypeID]
		if t == nil {
			t = &namedTotal{id: r.TypeID, name: r.Type}
			totals[r.TypeID] = t
		}
		t.amount += r.Amount
		monthTotals[i] += r.Amount
		if perMonthType[i] == nil {
			perMonthType[i] = map[int]*namedTotal{}
		}
		pt := perMonthType[i][r.TypeID]
		if pt == nil {
			pt = &namedTotal{id: r.TypeID, name: r.Type}
			perMonthType[i][r.TypeID] = pt
		}
		pt.amount += r.Amount
	}

	ranked := rankTypes(totals)
	var series []chart.Series
	var legend []models.LegendItem
	slot := map[int]int{} // type ID -> series index, -1 = folded
	folded := false
	for _, t := range ranked {
		if groceries && (t.name == otherCategory || len(series) >= topCategories) {
			slot[t.id] = -1
			folded = true
			continue
		}
		slot[t.id] = len(series)
		c := chart.TypeColor(t.id)
		if groceries {
			// ~55 categories collide mod 8; give the shown ones distinct slots.
			c = len(series) + 1
		}
		series = append(series, chart.Series{Name: t.name, Color: c, Values: make([]float64, len(months))})
		legend = append(legend, models.LegendItem{Name: t.name, Color: c})
	}
	if folded {
		name := utilities.T(lang, "trends.everything_else")
		if typeID != 0 {
			// Filtered to one category: only Other can fold, so name it.
			name = utilities.T(lang, "category."+otherCategory)
		}
		series = append(series, chart.Series{Name: name, Color: 0, Values: make([]float64, len(months))})
		legend = append(legend, models.LegendItem{Name: name, Color: 0})
		for id, s := range slot {
			if s == -1 {
				slot[id] = len(series) - 1
			}
		}
	}
	for _, r := range rows {
		i, ok := idx[r.Month]
		if !ok || (typeID != 0 && r.TypeID != typeID) {
			continue
		}
		series[slot[r.TypeID]].Values[i] += r.Amount
	}

	m := models.MonthTrend{Legend: legend}
	m.Chart = chart.Stacked(chart.DefaultFrame, labels, series)
	topIdx := -1
	for i, v := range monthTotals {
		m.Total += v
		if v > 0 && (topIdx < 0 || v > monthTotals[topIdx]) {
			topIdx = i
		}
	}
	m.Empty = m.Total <= 0
	// Typical month counts from the first month with data; months before it
	// (before the user started tracking) would only drag the median to zero.
	first := 0
	for first < len(monthTotals) && monthTotals[first] <= 0 {
		first++
	}
	m.Median = Median(monthTotals[first:])
	if len(months) > 0 {
		m.Chart.SetAverage(m.Total / float64(len(months)))
	}
	if topIdx >= 0 {
		m.TopMonth, m.TopMonthAmount = labels[topIdx], monthTotals[topIdx]
		if top := rankTypes(perMonthType[topIdx]); len(top) > 0 {
			m.TopMonthType = top[0].name
		}
	}
	m.PrevTotal = sumRows(prevRows, typeID)
	m.ChangePct, m.HasChange = ChangePercent(m.Total, m.PrevTotal)
	return m
}

// BuildYearTrend builds the year-vs-year tab. rowsA/rowsB are the full
// years, windowA/windowB the comparable so-far windows (see SoFarWindow),
// savedA/savedB the groceries discounts over the same so-far windows. Year B is drawn
// in colour 0 (grey, the older year) and year A in colour 1. YearTotals is
// ordered newest year first.
func BuildYearTrend(rowsA, rowsB []models.MonthTypeTotal, windowA, windowB []models.MonthTypeTotal, yearTotals map[int]float64, yearA, yearB int, savedA, savedB float64, typeID int, lang string) models.YearTrend {
	labels := make([]string, 12)
	for i := range labels {
		labels[i] = shortMonth(lang, time.Month(i+1))
	}
	perMonth := func(rows []models.MonthTypeTotal) []float64 {
		v := make([]float64, 12)
		for _, r := range rows {
			if m := monthOf(r.Month); m > 0 && (typeID == 0 || r.TypeID == typeID) {
				v[m-1] += r.Amount
			}
		}
		return v
	}
	nameA, nameB := strconv.Itoa(yearA), strconv.Itoa(yearB)
	valsA, valsB := perMonth(rowsA), perMonth(rowsB)
	y := models.YearTrend{
		YearA: yearA, YearB: yearB, SavedA: savedA, SavedB: savedB,
		Legend: []models.LegendItem{{Name: nameB, Color: 0}, {Name: nameA, Color: 1}},
		Chart: chart.Grouped(chart.DefaultFrame, labels, []chart.Series{
			{Name: nameB, Color: 0, Values: valsB},
			{Name: nameA, Color: 1, Values: valsA},
		}),
	}
	y.Empty = y.Chart.Empty

	y.SoFarA, y.SoFarB = sumRows(windowA, typeID), sumRows(windowB, typeID)
	y.SoFarPct, y.HasSoFar = ChangePercent(y.SoFarA, y.SoFarB)

	byType := func(rows []models.MonthTypeTotal) map[int]*namedTotal {
		out := map[int]*namedTotal{}
		for _, r := range rows {
			if typeID != 0 && r.TypeID != typeID {
				continue
			}
			t := out[r.TypeID]
			if t == nil {
				t = &namedTotal{id: r.TypeID, name: r.Type}
				out[r.TypeID] = t
			}
			t.amount += r.Amount
		}
		return out
	}
	a, b := byType(windowA), byType(windowB)
	delta := map[int]*namedTotal{}
	for id, t := range a {
		delta[id] = &namedTotal{id: id, name: t.name, amount: t.amount}
	}
	for id, t := range b {
		if d := delta[id]; d != nil {
			d.amount -= t.amount
		} else {
			delta[id] = &namedTotal{id: id, name: t.name, amount: -t.amount}
		}
	}
	if r := rankTypes(delta); len(r) > 0 && r[0].amount > 0 {
		y.Riser, y.RiserAmount, y.HasRiser = r[0].name, r[0].amount, true
	}

	for yr, t := range yearTotals {
		y.YearTotals = append(y.YearTotals, models.YearTotal{Year: yr, Total: t})
	}
	sort.Slice(y.YearTotals, func(i, j int) bool { return y.YearTotals[i].Year > y.YearTotals[j].Year })
	return y
}

// Page builds the charts page of a section for the query's tab, applying
// defaults for missing or invalid query values.
func (s *TrendsService) Page(ctx context.Context, userID uuid.UUID, sec models.Section, q models.TrendQuery, lang string, now time.Time) (*models.TrendsPage, error) {
	groceries := sec == models.SectionGroceries
	p := &models.TrendsPage{Section: string(sec), Lang: lang, TypePrefix: "type."}

	if groceries {
		p.TypePrefix = "category."
		cats, err := s.store.GetAllCategories(ctx)
		if err != nil {
			return nil, err
		}
		for _, c := range cats {
			p.Types = append(p.Types, models.TypeOption{ID: c.ID, Name: c.Name})
		}
		if p.Unsized, err = s.store.CountUnsizedGroceries(ctx, userID); err != nil {
			return nil, err
		}
	} else {
		types, err := s.store.GetSectionTypes(ctx, sec)
		if err != nil {
			return nil, err
		}
		p.Types = types
	}

	yearTotals, err := s.store.GetYearTotals(ctx, userID, sec)
	if err != nil {
		return nil, err
	}
	for y := range yearTotals {
		p.Years = append(p.Years, y)
	}
	sort.Sort(sort.Reverse(sort.IntSlice(p.Years)))

	switch q.Tab {
	case "year", "item":
	default:
		q.Tab = "month"
	}
	months, ok := PeriodMonths(q.Period, now)
	if !ok {
		if q.Tab == "item" && q.Period == "all" {
			// item tab handles "all" itself
		} else {
			q.Period = defaultPeriod
			months, _ = PeriodMonths(q.Period, now)
		}
	}
	if q.YearA == 0 {
		q.YearA = now.Year()
	}
	if q.YearB == 0 {
		q.YearB = q.YearA - 1
	}
	p.Query = q

	switch q.Tab {
	case "month":
		prev := make([]YearMonth, len(months))
		for i, m := range months {
			prev[i] = m.Back(len(months))
		}
		rows, err := s.store.GetMonthTypeTotals(ctx, userID, sec, months[0].Start(), months[len(months)-1].End())
		if err != nil {
			return nil, err
		}
		prevRows, err := s.store.GetMonthTypeTotals(ctx, userID, sec, prev[0].Start(), prev[len(prev)-1].End())
		if err != nil {
			return nil, err
		}
		m := BuildMonthTrend(rows, prevRows, months, q.TypeID, groceries, lang)
		if !m.Empty && m.Chart.HasAvg {
			m.Legend = append(m.Legend, models.LegendItem{Name: utilities.T(lang, "trends.monthly_average"), Dashed: true})
		}
		for i, ym := range months {
			if ym.Year == now.Year() && ym.Month == now.Month() {
				m.Chart.Columns[i].Current = true
			}
		}
		p.Month = &m
	case "item":
		if !groceries {
			q.Tab = "month"
			p.Query = q
			return s.Page(ctx, userID, sec, q, lang, now)
		}
		if err := s.itemTrend(ctx, userID, p, months, lang, now); err != nil {
			return nil, err
		}
	case "year":
		y, err := s.yearTrend(ctx, userID, sec, q, yearTotals, lang, now)
		if err != nil {
			return nil, err
		}
		p.Year = y
	}
	return p, nil
}

func (s *TrendsService) yearTrend(ctx context.Context, userID uuid.UUID, sec models.Section, q models.TrendQuery, yearTotals map[int]float64, lang string, now time.Time) (*models.YearTrend, error) {
	get := func(from, to time.Time) ([]models.MonthTypeTotal, error) {
		return s.store.GetMonthTypeTotals(ctx, userID, sec, from, to)
	}
	fullA, fullB := models.Date(q.YearA, 1, 1), models.Date(q.YearB, 1, 1)
	rowsA, err := get(fullA, models.Date(q.YearA+1, 1, 1))
	if err != nil {
		return nil, err
	}
	rowsB, err := get(fullB, models.Date(q.YearB+1, 1, 1))
	if err != nil {
		return nil, err
	}
	fromA, toA, fromB, toB := SoFarWindow(q.YearA, q.YearB, now)
	winA, err := get(fromA, toA)
	if err != nil {
		return nil, err
	}
	winB, err := get(fromB, toB)
	if err != nil {
		return nil, err
	}
	var savedA, savedB float64
	if sec == models.SectionGroceries {
		if savedA, err = s.store.GetGroceriesDiscountTotal(ctx, userID, fromA, toA); err != nil {
			return nil, err
		}
		if savedB, err = s.store.GetGroceriesDiscountTotal(ctx, userID, fromB, toB); err != nil {
			return nil, err
		}
	}
	y := BuildYearTrend(rowsA, rowsB, winA, winB, yearTotals, q.YearA, q.YearB, savedA, savedB, q.TypeID, lang)
	if q.YearA == now.Year() || q.YearB == now.Year() {
		y.SoFarLabel = shortMonth(lang, time.January) + "–" + shortMonth(lang, now.Month())
	}
	return &y, nil
}
