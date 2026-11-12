package services

import (
	"context"
	"expenser/internal/models"
	"math"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/stretchr/testify/require"
)

func gl(cat int, d time.Time, qty, size, price, disc float64, unit string) models.GroceryLine {
	return models.GroceryLine{CategoryID: cat, Category: map[int]string{1: "Dairy", 2: "Meat", 3: "Other"}[cat],
		Store: "Lidl", Product: "p", Date: d, Quantity: qty, UnitSize: size, TotalPrice: price,
		TotalDiscount: disc, Unit: unit, SizeStatus: models.SizeKnown}
}

func unknown(l models.GroceryLine) models.GroceryLine { l.SizeStatus = models.SizeUnknown; return l }

func TestMainUnit(t *testing.T) {
	d := models.Date(2026, 1, 1)
	m, all := MainUnit([]models.GroceryLine{gl(1, d, 1, 1, 1, 0, "l"), gl(1, d, 1, 1, 1, 0, "l"), gl(1, d, 1, 1, 1, 0, "l"), gl(1, d, 1, 1, 1, 0, "pc")})
	require.Equal(t, "l", m)
	require.Equal(t, []string{"l", "pc"}, all)
	m, _ = MainUnit([]models.GroceryLine{gl(1, d, 1, 1, 1, 0, "pc"), gl(1, d, 1, 1, 1, 0, "kg")})
	require.Equal(t, "kg", m)
	m, all = MainUnit([]models.GroceryLine{unknown(gl(1, d, 1, 1, 1, 0, "kg"))})
	require.Equal(t, "", m)
	require.Empty(t, all)
}

func TestPriceStats(t *testing.T) {
	d := models.Date(2026, 3, 5)
	st := PriceStats([]models.GroceryLine{gl(1, d, 1, 5, 10, 0, "kg"), gl(1, d, 1, 0.5, 3, 1, "kg")}, "kg", "month")["2026-03"]
	require.InDelta(t, 12.0/5.5, st.Avg, 1e-9)
	require.InDelta(t, 2.0, st.Lo, 1e-9)
	require.InDelta(t, 4.0, st.Hi, 1e-9)
	require.InDelta(t, 5.5, st.Qty, 1e-9)
	require.InDelta(t, 13.0/5.5, st.Shelf, 1e-9)
	require.Equal(t, 2, st.N)
	require.Contains(t, PriceStats([]models.GroceryLine{gl(1, d, 1, 5, 10, 0, "kg")}, "kg", "year"), "2026")
}

func TestPriceStatsSkipsZeroBase(t *testing.T) {
	d := models.Date(2026, 3, 5)
	st := PriceStats([]models.GroceryLine{
		gl(1, d, 0, 5, 10, 0, "kg"), gl(1, d, 1, 1, 2, 2, "kg"), gl(1, d, 1, 1, 2, 0, "l"),
		unknown(gl(1, d, 1, 1, 2, 0, "kg")), gl(1, d, 2, 1, 6, 0, "kg"),
	}, "kg", "month")
	s := st["2026-03"]
	require.Equal(t, 1, s.N)
	require.False(t, math.IsInf(s.Avg, 0))
	require.InDelta(t, 3.0, s.Avg, 1e-9)
}

func TestBuildItemTrend(t *testing.T) {
	initI18n(t)
	now := models.Date(2026, 11, 12)
	d := models.Date
	lines := []models.GroceryLine{
		gl(1, d(2025, 11, 3), 1, 1, 4, 0, "kg"), // a year earlier: 4.00
		gl(1, d(2026, 10, 3), 1, 1, 5, 0, "kg"),
		gl(1, d(2026, 11, 2), 2, 1, 12, 2, "kg"), // net 10 / 2 = 5.00, disc 1/kg
		gl(1, d(2026, 11, 4), 1, 1, 4, 1, "kg"),  // net 3.00, disc 1/kg
		gl(1, d(2026, 9, 4), 1, 1, 3, 0.5, "l"),  // other unit, saved 0.5
		unknown(gl(1, d(2026, 9, 5), 1, 1, 9, 0.25, "kg")),
		gl(1, d(2026, 9, 6), 0, 1, 9, 0, "kg"), // zero base
	}
	var periods []string
	for i := 11; i >= 0; i-- {
		periods = append(periods, YearMonth{2026, 11}.Back(i).String())
	}
	require.Equal(t, "2026-11", periods[11])
	it := BuildItemTrend(lines, lines, models.Category{ID: 1, Name: "Dairy"}, "", "month", periods, now, "en")
	require.False(t, it.Empty)
	require.Equal(t, "kg", it.Unit)
	require.Equal(t, []string{"kg", "l"}, it.Units)
	require.Equal(t, 3, it.Excluded) // other unit, unknown, zero base (2025-11 line out of range)
	require.True(t, it.HasPriceNow)
	require.InDelta(t, 13.0/3, it.PriceNow, 1e-9)
	require.True(t, it.HasPriceChange)
	require.InDelta(t, (13.0/3-4)/4*100, it.PriceChangePct, 1e-9)
	require.NotNil(t, it.Cheapest)
	require.InDelta(t, 3.0, it.Cheapest.PerUnit, 1e-9)
	require.Len(t, it.Discounts, 2)
	require.InDelta(t, 1.0, it.Discounts[0].DiscountPerUnit, 1e-9)
	require.InDelta(t, 2+1+0.5+0.25, it.TotalSaved, 1e-9)
	require.InDelta(t, 3.0, it.QtyNow, 1e-9)
	require.True(t, it.HasQtyUsual)
	require.InDelta(t, 1.0/11, it.QtyUsual, 1e-9)
	require.False(t, it.Chart.Empty)
}

func TestBuildItemTrendNoPriceChange(t *testing.T) {
	initI18n(t)
	now := models.Date(2026, 11, 12)
	lines := []models.GroceryLine{gl(1, models.Date(2026, 11, 2), 1, 1, 4, 0, "kg")}
	it := BuildItemTrend(lines, lines, models.Category{ID: 1, Name: "Dairy"}, "kg", "month", []string{"2026-10", "2026-11"}, now, "en")
	require.True(t, it.HasPriceNow)
	require.False(t, it.HasPriceChange)
}

func TestItemTrendNoKnownLines(t *testing.T) {
	initI18n(t)
	d := models.Date(2026, 10, 1)
	lines := []models.GroceryLine{unknown(gl(1, d, 1, 1, 4, 0, "kg")), unknown(gl(1, d, 1, 1, 5, 0, "l"))}
	it := BuildItemTrend(lines, lines, models.Category{ID: 1, Name: "Dairy"}, "", "month", []string{"2026-10", "2026-11"}, models.Date(2026, 11, 12), "en")
	require.True(t, it.Empty)
	require.Equal(t, 2, it.Excluded)
}

func TestBuildItemRows(t *testing.T) {
	now := models.Date(2026, 11, 12)
	d := models.Date
	all := []models.GroceryLine{
		gl(1, d(2026, 10, 1), 1, 1, 6, 0, "kg"), gl(1, d(2026, 9, 1), 1, 1, 8, 0, "kg"), // now 7.0
		gl(1, d(2025, 10, 20), 1, 1, 5, 0, "kg"), // year ago 5.0
		gl(1, d(2026, 5, 1), 1, 1, 3, 0, "kg"),   // bought only
		gl(2, d(2026, 10, 1), 1, 1, 2, 0, "pc"),  // no year ago
		gl(3, d(2026, 10, 1), 1, 1, 10, 0, "kg"), // second no-change row sorts by name
	}
	all[5].Category = "Beer"
	rows := BuildItemRows(all, now)
	require.Len(t, rows, 3)
	require.Equal(t, "Dairy", rows[0].Item)
	require.True(t, rows[0].HasChange)
	require.InDelta(t, 7.0, rows[0].Now, 1e-9)
	require.InDelta(t, 5.0, rows[0].YearAgo, 1e-9)
	require.InDelta(t, 40.0, rows[0].ChangePct, 1e-9)
	require.InDelta(t, 3.0, rows[0].Bought12, 1e-9)
	require.Equal(t, "Beer", rows[1].Item)
	require.Equal(t, "Meat", rows[2].Item)
	require.False(t, rows[1].HasChange)
}

type itemStore struct {
	fakeTrendsStore
	lines []models.GroceryLine
}

func (s itemStore) GetGroceryLines(_ context.Context, _ uuid.UUID, cat int, from, to time.Time) ([]models.GroceryLine, error) {
	var out []models.GroceryLine
	for _, l := range s.lines {
		if (cat == 0 || l.CategoryID == cat) && !l.Date.Before(from) && l.Date.Before(to) {
			out = append(out, l)
		}
	}
	return out, nil
}
func (itemStore) GetTrackedCategories(context.Context) ([]models.Category, error) {
	return []models.Category{{ID: 1, Name: "Dairy"}, {ID: 2, Name: "Meat"}}, nil
}

func TestPageItemDefaults(t *testing.T) {
	initI18n(t)
	d := models.Date
	st := itemStore{lines: []models.GroceryLine{
		gl(2, d(2026, 10, 1), 1, 1, 8, 0, "kg"), gl(2, d(2026, 9, 1), 1, 1, 8, 0, "kg"),
		gl(1, d(2026, 10, 1), 1, 1, 2, 0, "kg"),
		gl(3, d(2026, 10, 1), 1, 1, 2, 0, "kg"), gl(3, d(2026, 10, 2), 1, 1, 2, 0, "kg"), gl(3, d(2026, 10, 3), 1, 1, 2, 0, "kg"),
	}}
	svc := NewTrendsService(st)
	now := d(2026, 11, 12)
	for _, id := range []int{0, 3, 99} {
		p, err := svc.Page(context.Background(), uuid.New(), models.SectionGroceries, models.TrendQuery{Tab: "item", Period: "12", ItemID: id, By: "month"}, "en", now)
		require.NoError(t, err)
		require.Equal(t, 2, p.Item.ItemID, "item id %d", id)
		require.Len(t, p.Items, 2)
	}
	p, err := svc.Page(context.Background(), uuid.New(), models.SectionGroceries, models.TrendQuery{Tab: "item", Period: "all", ItemID: 1, By: "year"}, "en", now)
	require.NoError(t, err)
	require.Equal(t, 1, p.Item.ItemID)
	require.Equal(t, "kg", p.Query.Unit)
}
