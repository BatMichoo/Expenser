package services

import (
	"context"
	"strconv"
	"testing"
	"time"

	"expenser/internal/models"

	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func local(y int, m time.Month, d int) time.Time {
	return time.Date(y, m, d, 0, 0, 0, 0, time.Local)
}

func TestYearToDateRanges(t *testing.T) {
	r := yearToDate(time.Date(2026, 3, 15, 18, 30, 0, 0, time.Local))
	assert.Equal(t, local(2026, 1, 1), r.ThisFrom)
	assert.Equal(t, local(2026, 3, 16), r.ThisTo, "today counts")
	assert.Equal(t, local(2025, 1, 1), r.LastFrom)
	assert.Equal(t, local(2025, 3, 16), r.LastTo, "same date last year")

	leap := yearToDate(time.Date(2028, 2, 29, 9, 0, 0, 0, time.Local))
	assert.Equal(t, local(2027, 3, 1), leap.LastTo, "29 Feb compares with up to 28 Feb")
}

func TestBuildHomeCategory(t *testing.T) {
	now := time.Date(2026, 11, 11, 12, 0, 0, 0, time.Local)
	monthly := map[string]float64{"2025-12": 100, "2026-10": 300, "2026-11": 212}
	c := buildHomeCategory("house", 2840, 2535.71, monthly, map[string]float64{"Electricity": 1000, "Water": 300, "Gas": 140}, now)

	assert.Equal(t, "house", c.Section)
	assert.Equal(t, 2840.0, c.YearTotal)
	assert.True(t, c.HasLastYear)
	assert.InDelta(t, 12.0, c.ChangePct, 0.1)
	assert.Equal(t, 212.0, c.ThisMonth)
	assert.Equal(t, "Electricity", c.TopName)
	assert.InDelta(t, 69.4, c.TopPct, 0.1, "share of the type totals")
	require.Len(t, c.Bars, 12)
	assert.Equal(t, time.December, c.Bars[0].MonthNum, "oldest first: last December")
	assert.True(t, c.Bars[11].Current)
	assert.Equal(t, 100.0, c.Bars[10].Height, "October is the biggest month")
	assert.InDelta(t, 70.7, c.Bars[11].Height, 0.1, "this month relative to it")
	assert.False(t, c.Empty)
}

func TestBuildHomeCategoryWithoutLastYear(t *testing.T) {
	now := time.Date(2026, 11, 11, 12, 0, 0, 0, time.Local)
	c := buildHomeCategory("car", 120, 0, map[string]float64{"2026-11": 120}, map[string]float64{"Fuel": 120}, now)
	assert.False(t, c.HasLastYear, "nothing to compare with")
	assert.InDelta(t, 100, c.TopPct, 0.01)
}

func TestBuildHomeCategoryEmptyYear(t *testing.T) {
	now := time.Date(2026, 11, 11, 12, 0, 0, 0, time.Local)
	c := buildHomeCategory("groceries", 0, 50, map[string]float64{"2025-12": 50}, map[string]float64{}, now)
	assert.True(t, c.Empty)
	assert.Empty(t, c.TopName)
}

func homeBill(name string, due time.Time, active bool) models.RecurringPayment {
	typeID := 2
	return models.RecurringPayment{ID: uuid.New(), Name: name, DueDate: due, Active: active,
		Category: models.RecurringCategoryHouse, UtilityTypeID: &typeID, CycleUnit: models.CycleMonth, CycleEvery: 1}
}

func TestPickHomeBills(t *testing.T) {
	today := local(2026, 11, 11)
	var bills []models.RecurringPayment
	for i, d := range []int{20, 5, 14, 12, 30, 25, 18} {
		bills = append(bills, homeBill(string(rune('A'+i)), local(2026, 11, d), true))
	}
	bills = append(bills, homeBill("Paused", local(2026, 11, 1), false))

	rows, more := pickHomeBills(bills, today, 5)
	var names []string
	for _, r := range rows {
		names = append(names, r.Payment.Name)
	}
	assert.Equal(t, []string{"B", "D", "C", "G", "A"}, names, "by date, overdue first, paused left out")
	assert.Equal(t, -6, rows[0].DaysLeft)
	assert.Equal(t, 2, more)
}

type fakeHomeStore struct {
	typeTotals map[string]float64 // "category/typeID/YYYY-MM" -> total
}

func (f *fakeHomeStore) GetBillTypeTotal(_ context.Context, _ uuid.UUID, category models.RecurringPaymentCategory, typeID int, from, _ time.Time) (float64, bool, error) {
	v, ok := f.typeTotals[string(category)+"/"+strconv.Itoa(typeID)+"/"+from.Format("2006-01")]
	return v, ok, nil
}

func TestHomeBillsLastYear(t *testing.T) {
	today := local(2026, 11, 11)
	elec := homeBill("Electricity", local(2026, 11, 14), true)
	fresh := homeBill("New", local(2026, 12, 2), true)
	other := 9
	fresh.UtilityTypeID = &other
	store := &fakeHomeStore{typeTotals: map[string]float64{"house/2/2025-11": 58}}

	rows, err := homeBillsLastYear(context.Background(), store, uuid.New(), []models.HomeBillRow{
		{Payment: elec, DaysLeft: elec.DaysUntilDue(today)}, {Payment: fresh, DaysLeft: fresh.DaysUntilDue(today)},
	})
	require.NoError(t, err)
	assert.True(t, rows[0].HasLastYear)
	assert.Equal(t, 58.0, rows[0].LastYear)
	assert.Equal(t, time.November, rows[0].LastYearMonth)
	assert.False(t, rows[1].HasLastYear, "nothing paid for that type last December")
}
