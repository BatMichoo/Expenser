package services

import (
	"context"
	"expenser/internal/models"
	"math"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/stretchr/testify/require"

	"expenser/internal/utilities"
)

func initI18n(t *testing.T) {
	t.Helper()
	require.NoError(t, utilities.InitI18n())
}

func TestPeriodMonths(t *testing.T) {
	now := models.Date(2026, 11, 12)
	m, ok := PeriodMonths("12", now)
	if !ok || len(m) != 12 || m[0] != (YearMonth{2025, 12}) || m[11] != (YearMonth{2026, 11}) {
		t.Fatalf("12: %v %v", m, ok)
	}
	m, ok = PeriodMonths("y2025", now)
	if !ok || len(m) != 12 || m[0] != (YearMonth{2025, 1}) || m[11] != (YearMonth{2025, 12}) {
		t.Fatalf("y2025: %v %v", m, ok)
	}
	if _, ok := PeriodMonths("abc", now); ok {
		t.Fatal("abc should be invalid")
	}
}

func TestMedian(t *testing.T) {
	for _, c := range []struct {
		in   []float64
		want float64
	}{{[]float64{3, 1, 2}, 2}, {[]float64{4, 1, 2, 3}, 2.5}, {nil, 0}} {
		if got := Median(c.in); got != c.want {
			t.Errorf("Median(%v)=%v want %v", c.in, got, c.want)
		}
	}
}

func TestSoFarWindow(t *testing.T) {
	d := models.Date
	check := func(name string, got, want time.Time) {
		if !got.Equal(want) {
			t.Errorf("%s = %v want %v", name, got, want)
		}
	}
	fa, ta, fb, tb := SoFarWindow(2026, 2025, d(2026, 11, 12))
	check("fromA", fa, d(2026, 1, 1))
	check("toA", ta, d(2026, 11, 13))
	check("fromB", fb, d(2025, 1, 1))
	check("toB", tb, d(2025, 11, 13))

	_, _, _, tb = SoFarWindow(2028, 2027, d(2028, 2, 29))
	check("leap toB", tb, d(2027, 3, 1))

	fa, ta, fb, tb = SoFarWindow(2024, 2023, d(2026, 11, 12))
	check("past fromA", fa, d(2024, 1, 1))
	check("past toA", ta, d(2025, 1, 1))
	check("past fromB", fb, d(2023, 1, 1))
	check("past toB", tb, d(2024, 1, 1))

	_, ta, _, tb = SoFarWindow(2026, 2025, d(2026, 1, 1))
	check("jan1 toA", ta, d(2026, 1, 2))
	check("jan1 toB", tb, d(2025, 1, 2))
}

func row(month string, id int, name string, amt float64) models.MonthTypeTotal {
	return models.MonthTypeTotal{Month: month, TypeID: id, Type: name, Amount: amt}
}

func months3() []YearMonth {
	return []YearMonth{{2026, 1}, {2026, 2}, {2026, 3}}
}

func TestBuildMonthTrend(t *testing.T) {
	initI18n(t)
	rows := []models.MonthTypeTotal{
		row("2026-01", 1, "Rent", 100), row("2026-02", 1, "Rent", 100),
		row("2026-03", 1, "Rent", 100), row("2026-03", 2, "Power", 50),
	}
	prev := []models.MonthTypeTotal{row("2025-12", 1, "Rent", 150)}
	m := BuildMonthTrend(rows, prev, months3(), 0, false, "en")
	if m.Total != 350 || m.PrevTotal != 150 || !m.HasChange {
		t.Fatalf("totals %+v", m)
	}
	if math.Abs(m.ChangePct-(350.0-150)/150*100) > 1e-9 {
		t.Errorf("pct %v", m.ChangePct)
	}
	if m.Median != 100 {
		t.Errorf("median %v", m.Median)
	}
	if m.TopMonth != "Mar" || m.TopMonthAmount != 150 || m.TopMonthType != "Rent" {
		t.Errorf("top %q %v %q", m.TopMonth, m.TopMonthAmount, m.TopMonthType)
	}
	if len(m.Legend) != 2 || m.Legend[0].Name != "Rent" {
		t.Errorf("legend %+v", m.Legend)
	}
	if !m.Chart.HasAvg || math.Abs(m.Chart.Avg-350.0/3) > 1e-9 {
		t.Errorf("avg %v", m.Chart.Avg)
	}

	one := BuildMonthTrend(rows, nil, months3(), 2, false, "en")
	if one.Total != 50 || len(one.Legend) != 1 || one.Legend[0].Name != "Power" {
		t.Errorf("filter %+v", one)
	}
}

func TestBuildMonthTrendGroceries(t *testing.T) {
	initI18n(t)
	var rows []models.MonthTypeTotal
	for i := 1; i <= 8; i++ {
		rows = append(rows, row("2026-01", i, string(rune('A'+i)), float64(i*10)))
	}
	rows = append(rows, row("2026-01", 99, "Other", 500))
	m := BuildMonthTrend(rows, nil, months3(), 0, true, "en")
	if len(m.Legend) != 7 {
		t.Fatalf("legend %+v", m.Legend)
	}
	last := m.Legend[6]
	if last.Color != 0 || last.Name != "Everything else" {
		t.Errorf("last %+v", last)
	}
	for _, l := range m.Legend[:6] {
		if l.Color == 0 {
			t.Errorf("coloured series has colour 0: %+v", l)
		}
	}
	// top 6 non-Other by total are 8..3; folded: 1,2 and Other.
	if m.Legend[0].Name != string(rune('A'+8)) {
		t.Errorf("first %+v", m.Legend[0])
	}
	if m.Total != 36*10+500 {
		t.Errorf("total includes Other: %v", m.Total)
	}
	got := 0.0
	for _, r := range m.Chart.Columns[0].Rects {
		if r.Color == 0 {
			got = r.Value
		}
	}
	if got != 10+20+500 {
		t.Errorf("everything else = %v", got)
	}
}

func TestMonthTrendEmpty(t *testing.T) {
	initI18n(t)
	m := BuildMonthTrend(nil, nil, months3(), 0, false, "en")
	if !m.Empty || m.HasChange || math.IsNaN(m.ChangePct) || m.TopMonth != "" {
		t.Errorf("%+v", m)
	}
}

func TestMonthTrendLabelsAcrossYears(t *testing.T) {
	initI18n(t)
	ms := []YearMonth{{2025, 12}, {2026, 1}}
	m := BuildMonthTrend(nil, nil, ms, 0, false, "en")
	if m.Chart.Columns[0].Label != "Dec '25" || m.Chart.Columns[1].Label != "Jan '26" {
		t.Errorf("%q %q", m.Chart.Columns[0].Label, m.Chart.Columns[1].Label)
	}
}

func TestBuildYearTrend(t *testing.T) {
	initI18n(t)
	rowsA := []models.MonthTypeTotal{row("2026-01", 1, "Rent", 100), row("2026-02", 2, "Power", 80)}
	rowsB := []models.MonthTypeTotal{row("2025-01", 1, "Rent", 90), row("2025-02", 2, "Power", 20)}
	y := BuildYearTrend(rowsA, rowsB, rowsA, rowsB,
		map[int]float64{2024: 5, 2026: 180, 2025: 110}, 2026, 2025, 7, 3, 0, "en")
	if len(y.Chart.Columns) != 12 {
		t.Fatalf("cols %d", len(y.Chart.Columns))
	}
	if y.Legend[0].Color != 0 || y.Legend[0].Name != "2025" || y.Legend[1].Color != 1 || y.Legend[1].Name != "2026" {
		t.Errorf("legend %+v", y.Legend)
	}
	r := y.Chart.Columns[0].Rects
	if len(r) != 2 || r[0].Color != 0 || r[1].Color != 1 {
		t.Errorf("rects %+v", r)
	}
	if y.SoFarA != 180 || y.SoFarB != 110 || !y.HasSoFar || math.Abs(y.SoFarPct-(70.0/110*100)) > 1e-9 {
		t.Errorf("sofar %+v", y)
	}
	if !y.HasRiser || y.Riser != "Power" || y.RiserAmount != 60 {
		t.Errorf("riser %+v", y)
	}
	if y.SavedA != 7 || y.SavedB != 3 {
		t.Errorf("saved %v %v", y.SavedA, y.SavedB)
	}
	if len(y.YearTotals) != 3 || y.YearTotals[0].Year != 2026 || y.YearTotals[2].Year != 2024 {
		t.Errorf("totals %+v", y.YearTotals)
	}

	fell := BuildYearTrend(rowsB, rowsA, rowsB, rowsA, nil, 2026, 2025, 0, 0, 0, "en")
	if fell.HasRiser {
		t.Errorf("riser when all fell: %+v", fell)
	}
	empty := BuildYearTrend(nil, nil, nil, nil, nil, 2026, 2025, 0, 0, 0, "en")
	if !empty.Empty || empty.HasSoFar || math.IsNaN(empty.SoFarPct) {
		t.Errorf("empty %+v", empty)
	}
}

type fakeTrendsStore struct{}

func (fakeTrendsStore) GetMonthTypeTotals(context.Context, uuid.UUID, models.Section, time.Time, time.Time) ([]models.MonthTypeTotal, error) {
	return nil, nil
}
func (fakeTrendsStore) GetYearTotals(context.Context, uuid.UUID, models.Section) (map[int]float64, error) {
	return map[int]float64{2025: 1, 2026: 2}, nil
}
func (fakeTrendsStore) GetGroceriesDiscountTotal(context.Context, uuid.UUID, time.Time, time.Time) (float64, error) {
	return 0, nil
}
func (fakeTrendsStore) GetGroceryLines(context.Context, uuid.UUID, int, time.Time, time.Time) ([]models.GroceryLine, error) {
	return nil, nil
}
func (fakeTrendsStore) CountUnsizedGroceries(context.Context, uuid.UUID) (int, error) { return 4, nil }
func (fakeTrendsStore) GetTrackedCategories(context.Context) ([]models.Category, error) {
	return nil, nil
}
func (fakeTrendsStore) GetSectionTypes(context.Context, models.Section) ([]models.TypeOption, error) {
	return []models.TypeOption{{ID: 1, Name: "Rent"}}, nil
}
func (fakeTrendsStore) GetAllCategories(context.Context) ([]models.Category, error) {
	return []models.Category{{ID: 1, Name: "Dairy"}}, nil
}

func TestPageDefaults(t *testing.T) {
	initI18n(t)
	svc := NewTrendsService(fakeTrendsStore{})
	now := models.Date(2026, 11, 12)
	p, err := svc.Page(context.Background(), uuid.New(), models.SectionHouse, models.TrendQuery{}, "en", now)
	if err != nil {
		t.Fatal(err)
	}
	if p.Query.Tab != "month" || p.Query.Period != "12" || p.Month == nil || p.Year != nil {
		t.Errorf("month %+v", p.Query)
	}
	if p.TypePrefix != "type." || p.Unsized != 0 || len(p.Types) != 1 || len(p.Years) != 2 || p.Years[0] != 2026 {
		t.Errorf("common %+v", p)
	}
	p, err = svc.Page(context.Background(), uuid.New(), models.SectionHouse, models.TrendQuery{Tab: "year"}, "en", now)
	if err != nil {
		t.Fatal(err)
	}
	if p.Query.YearA != 2026 || p.Query.YearB != 2025 || p.Year == nil {
		t.Errorf("year %+v", p.Query)
	}
	g, err := svc.Page(context.Background(), uuid.New(), models.SectionGroceries, models.TrendQuery{}, "en", now)
	if err != nil {
		t.Fatal(err)
	}
	if g.TypePrefix != "category." || g.Unsized != 4 || g.Types[0].Name != "Dairy" {
		t.Errorf("groceries %+v", g)
	}
}

func TestSoFarWindowPastVsCurrent(t *testing.T) {
	d := models.Date
	fa, ta, fb, tb := SoFarWindow(2025, 2026, d(2026, 11, 12))
	for name, c := range map[string][2]time.Time{
		"fromA": {fa, d(2025, 1, 1)}, "toA": {ta, d(2025, 11, 13)},
		"fromB": {fb, d(2026, 1, 1)}, "toB": {tb, d(2026, 11, 13)},
	} {
		if !c[0].Equal(c[1]) {
			t.Errorf("%s = %v want %v", name, c[0], c[1])
		}
	}
}

type rangeStore struct {
	fakeTrendsStore
	totals, discounts [][2]time.Time
}

func (r *rangeStore) GetMonthTypeTotals(_ context.Context, _ uuid.UUID, _ models.Section, from, to time.Time) ([]models.MonthTypeTotal, error) {
	r.totals = append(r.totals, [2]time.Time{from, to})
	return nil, nil
}
func (r *rangeStore) GetGroceriesDiscountTotal(_ context.Context, _ uuid.UUID, from, to time.Time) (float64, error) {
	r.discounts = append(r.discounts, [2]time.Time{from, to})
	return 0, nil
}

func TestYearTabUsesSoFarWindowForDiscounts(t *testing.T) {
	initI18n(t)
	d := models.Date
	st := &rangeStore{}
	now := d(2026, 11, 12)
	q := models.TrendQuery{Tab: "year", YearA: 2026, YearB: 2025}
	_, err := NewTrendsService(st).Page(context.Background(), uuid.New(), models.SectionGroceries, q, "en", now)
	require.NoError(t, err)
	wantA := [2]time.Time{d(2026, 1, 1), d(2026, 11, 13)}
	wantB := [2]time.Time{d(2025, 1, 1), d(2025, 11, 13)}
	require.Equal(t, [][2]time.Time{wantA, wantB}, st.discounts)
	require.Contains(t, st.totals, wantA)
	require.Contains(t, st.totals, wantB)
}

type rowsTrendsStore struct{ fakeTrendsStore }

func (rowsTrendsStore) GetMonthTypeTotals(_ context.Context, _ uuid.UUID, _ models.Section, from, _ time.Time) ([]models.MonthTypeTotal, error) {
	if from.Year() < 2025 {
		return nil, nil
	}
	return []models.MonthTypeTotal{{Month: "2026-10", TypeID: 1, Type: "Rent", Amount: 120}}, nil
}

func TestPageMonthLegendHasAverage(t *testing.T) {
	initI18n(t)
	now := models.Date(2026, 11, 12)
	p, err := NewTrendsService(rowsTrendsStore{}).Page(context.Background(), uuid.New(), models.SectionHouse, models.TrendQuery{}, "en", now)
	if err != nil {
		t.Fatal(err)
	}
	l := p.Month.Legend
	if len(l) != 2 || l[0].Name != "Rent" || !l[1].Dashed || l[1].Color != 0 || l[1].Name != "Monthly average" {
		t.Errorf("legend %+v", l)
	}
	empty, err := NewTrendsService(fakeTrendsStore{}).Page(context.Background(), uuid.New(), models.SectionHouse, models.TrendQuery{}, "en", now)
	if err != nil {
		t.Fatal(err)
	}
	if len(empty.Month.Legend) != 0 {
		t.Errorf("empty legend %+v", empty.Month.Legend)
	}
}

func TestMonthTrendMedianSkipsLeadingEmptyMonths(t *testing.T) {
	initI18n(t)
	months, _ := PeriodMonths("12", time.Date(2026, 12, 15, 0, 0, 0, 0, time.UTC))
	require.Len(t, months, 12)
	key := func(m YearMonth) string { return time.Date(m.Year, m.Month, 1, 0, 0, 0, 0, time.UTC).Format("2006-01") }
	rows := []models.MonthTypeTotal{
		row(key(months[8]), 1, "Rent", 10),
		row(key(months[10]), 1, "Rent", 30),
		row(key(months[11]), 1, "Rent", 20),
	}
	m := BuildMonthTrend(rows, nil, months, 0, false, "en")
	require.InDelta(t, 15, m.Median, 1e-9) // [10,0,30,20]

	none := BuildMonthTrend(nil, nil, months, 0, false, "en")
	require.Equal(t, 0.0, none.Median)
}

func TestBuildMonthTrendGroceriesDistinctColours(t *testing.T) {
	initI18n(t)
	var rows []models.MonthTypeTotal
	// IDs 1, 9, 17, 25, 33, 41 all collide mod 8.
	for i, id := range []int{1, 9, 17, 25, 33, 41, 49} {
		rows = append(rows, row("2026-01", id, string(rune('A'+i)), float64(100-i)))
	}
	m := BuildMonthTrend(rows, nil, months3(), 0, true, "en")
	require.Len(t, m.Legend, 7)
	for i, l := range m.Legend[:6] {
		require.Equal(t, i+1, l.Color, "legend %d", i)
	}
	require.Equal(t, 0, m.Legend[6].Color)
	var got []int
	for _, r := range m.Chart.Columns[0].Rects {
		got = append(got, r.Color)
	}
	require.ElementsMatch(t, []int{1, 2, 3, 4, 5, 6, 0}, got)
}

// Filtering groceries to the Other category shows it under its own name, not
// as "Everything else".
func TestBuildMonthTrendGroceriesOtherFilter(t *testing.T) {
	initI18n(t)
	rows := []models.MonthTypeTotal{row("2026-01", 99, "Other", 50), row("2026-01", 1, "Meat", 20)}
	m := BuildMonthTrend(rows, nil, months3(), 99, true, "en")
	if len(m.Legend) != 1 || m.Legend[0].Name != "Other" || m.Legend[0].Color != 0 {
		t.Fatalf("legend %+v", m.Legend)
	}
	all := BuildMonthTrend(rows, nil, months3(), 0, true, "en")
	if all.Legend[len(all.Legend)-1].Name != "Everything else" {
		t.Errorf("unfiltered legend %+v", all.Legend)
	}
}
