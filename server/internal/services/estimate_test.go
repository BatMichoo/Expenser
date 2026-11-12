package services

import (
	"expenser/internal/models"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// spend is one history entry on day d of the month `back` months before
// November 2026 (0 = November itself).
func spend(typ string, back, d int, amount float64) models.SpendEntry {
	m := YearMonth{2026, time.November}
	for i := 0; i < back; i++ {
		m = m.Prev()
	}
	return models.SpendEntry{Type: typ, Date: time.Date(m.Year, m.Month, d, 12, 0, 0, 0, time.UTC), Amount: amount}
}

// everyMonth repeats one entry on day d for each of the months back..1.
func everyMonth(typ string, months, d int, amount float64) []models.SpendEntry {
	var out []models.SpendEntry
	for back := 1; back <= months; back++ {
		out = append(out, spend(typ, back, d, amount))
	}
	return out
}

var nov = YearMonth{2026, time.November}

func novDay(d int) time.Time { return time.Date(2026, 11, d, 9, 0, 0, 0, time.UTC) }

func TestEstimatePaidBillIsDone(t *testing.T) {
	h := append(everyMonth("Electricity", 6, 10, 50), spend("Electricity", 0, 8, 55))
	e := EstimateMonthEnd(h, nov, novDay(12))
	require.True(t, e.OK)
	assert.InDelta(t, 55, e.Amount, 0.001, "paid early: nothing more is coming, no double count")
	assert.False(t, e.FromPace)
}

func TestEstimateUnpaidBillAddsUsual(t *testing.T) {
	h := everyMonth("Electricity", 6, 10, 50)
	e := EstimateMonthEnd(h, nov, novDay(25))
	require.True(t, e.OK)
	assert.InDelta(t, 50, e.Amount, 0.001, "running late is still coming, even near month end")
}

func TestEstimateUsualIsMedian(t *testing.T) {
	h := append(everyMonth("Electricity", 5, 10, 50), spend("Electricity", 6, 10, 500))
	e := EstimateMonthEnd(h, nov, novDay(3))
	assert.InDelta(t, 50, e.Amount, 0.001, "one odd month does not drag the usual amount")
}

func TestEstimateFrequentSpendingUsesCurve(t *testing.T) {
	// Two 40 € fill-ups a month, on the 5th and the 20th: by the 10th half
	// of a usual month is spent, so half of 80 € is still to come.
	h := append(everyMonth("Fuel", 6, 5, 40), everyMonth("Fuel", 6, 20, 40)...)
	h = append(h, spend("Fuel", 0, 6, 40))
	e := EstimateMonthEnd(h, nov, novDay(10))
	assert.InDelta(t, 40+40, e.Amount, 0.001)
}

func TestEstimateIgnoresOneOffs(t *testing.T) {
	h := append(everyMonth("Electricity", 6, 10, 50), spend("Repair", 3, 15, 800))
	e := EstimateMonthEnd(h, nov, novDay(12))
	assert.InDelta(t, 50, e.Amount, 0.001, "a repair seen once is not forecast")

	h = append(h, spend("Repair", 0, 4, 300))
	e = EstimateMonthEnd(h, nov, novDay(12))
	assert.InDelta(t, 50+300, e.Amount, 0.001, "but what was spent on it counts")
}

func TestEstimateSeasonEnds(t *testing.T) {
	// Gas from November to March, viewed in April with 13 months of history.
	apr := YearMonth{2026, time.April}
	var h []models.SpendEntry
	for _, m := range []YearMonth{{2025, time.March}, {2025, time.November}, {2025, time.December}, {2026, time.January}, {2026, time.February}, {2026, time.March}} {
		h = append(h, models.SpendEntry{Type: "Gas", Date: time.Date(m.Year, m.Month, 20, 0, 0, 0, 0, time.UTC), Amount: 80})
	}
	for m := (YearMonth{2025, time.March}); m != apr; m = m.Next() {
		h = append(h, models.SpendEntry{Type: "Water", Date: time.Date(m.Year, m.Month, 13, 0, 0, 0, 0, time.UTC), Amount: 20})
	}
	e := EstimateMonthEnd(h, apr, time.Date(2026, 4, 5, 9, 0, 0, 0, time.UTC))
	assert.InDelta(t, 20, e.Amount, 0.001, "no gas in April last year, so none expected now")
}

func TestEstimateSeasonStarts(t *testing.T) {
	// Gas last November and December, none in the six months before now.
	h := []models.SpendEntry{spend("Gas", 12, 20, 90), spend("Gas", 11, 20, 110)}
	for back := 1; back <= 13; back++ {
		h = append(h, spend("Water", back, 13, 20))
	}
	e := EstimateMonthEnd(h, nov, novDay(5))
	assert.InDelta(t, 20+90, e.Amount, 0.001, "heating season is back: expect last November's gas")
}

func TestEstimateSeasonBlendsLastYear(t *testing.T) {
	// Electricity every month: 50 recently, 90 last November (and December).
	h := everyMonth("Electricity", 10, 10, 50)
	h = append(h, spend("Electricity", 11, 10, 90), spend("Electricity", 12, 10, 90))
	e := EstimateMonthEnd(h, nov, novDay(5))
	assert.InDelta(t, (50+90)/2.0, e.Amount, 0.001)
}

func TestEstimateOneMonthOfHistory(t *testing.T) {
	h := []models.SpendEntry{spend("Fuel", 1, 5, 100), spend("Fuel", 0, 3, 20)}
	e := EstimateMonthEnd(h, nov, novDay(10))
	require.True(t, e.OK)
	assert.InDelta(t, 20+100*20.0/30, e.Amount, 0.001, "usual is that one month, not a sixth of it")
	assert.False(t, e.FromPace)
}

func TestEstimateNoHistoryUsesPace(t *testing.T) {
	h := []models.SpendEntry{spend("Fuel", 0, 3, 100)}
	e := EstimateMonthEnd(h, nov, novDay(10))
	require.True(t, e.OK)
	assert.InDelta(t, 100.0/10*30, e.Amount, 0.001)
	assert.True(t, e.FromPace)

	e = EstimateMonthEnd(h, nov, novDay(3))
	assert.False(t, e.OK, "too early for a pace estimate")
}
