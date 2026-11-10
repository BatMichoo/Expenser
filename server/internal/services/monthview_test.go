package services

import (
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestParseYearMonth(t *testing.T) {
	now := time.Date(2026, 11, 10, 15, 0, 0, 0, time.UTC)
	assert.Equal(t, YearMonth{2026, time.March}, ParseYearMonth("2026-03", now))
	assert.Equal(t, YearMonth{2026, time.November}, ParseYearMonth("", now), "empty means current month")
	assert.Equal(t, YearMonth{2026, time.November}, ParseYearMonth("garbage", now))
	assert.Equal(t, YearMonth{2026, time.November}, ParseYearMonth("2026-13", now), "invalid month")
}

func TestYearMonthBoundaries(t *testing.T) {
	jan := YearMonth{2026, time.January}
	assert.Equal(t, YearMonth{2025, time.December}, jan.Prev())
	assert.Equal(t, YearMonth{2026, time.February}, jan.Next())
	assert.Equal(t, YearMonth{2027, time.January}, YearMonth{2026, time.December}.Next())
	assert.Equal(t, "2026-01", jan.String())
	assert.Equal(t, time.Date(2026, 1, 1, 0, 0, 0, 0, time.Local), jan.Start())
	assert.Equal(t, time.Date(2026, 2, 1, 0, 0, 0, 0, time.Local), jan.End(), "end is exclusive")
}

func TestSummarize(t *testing.T) {
	s := Summarize([]SummaryItem{
		{Type: "Gas", Amount: 50}, {Type: "Water", Amount: 20}, {Type: "Gas", Amount: 35}, {Type: "TV", Amount: 10},
	})
	assert.InDelta(t, 115, s.Total, 0.001)
	assert.Equal(t, 4, s.Count)
	assert.InDelta(t, 28.75, s.Average, 0.001)
	assert.Equal(t, "Gas", s.TopType, "highest is the largest per-type sum")
	assert.InDelta(t, 85, s.TopAmount, 0.001)

	empty := Summarize(nil)
	assert.Zero(t, empty.Total)
	assert.Zero(t, empty.Average, "no division by zero")
	assert.Empty(t, empty.TopType)
}

func TestChangePercent(t *testing.T) {
	pct, ok := ChangePercent(88, 100)
	require.True(t, ok)
	assert.InDelta(t, -12, pct, 0.001)

	pct, ok = ChangePercent(150, 100)
	require.True(t, ok)
	assert.InDelta(t, 50, pct, 0.001)

	_, ok = ChangePercent(50, 0)
	assert.False(t, ok, "no previous month, no percentage")
}

func TestBars(t *testing.T) {
	totals := map[string]float64{"2025-10": 40, "2025-12": 80, "2026-02": 20}
	bars := Bars(totals, YearMonth{2026, time.February}, 6)
	require.Len(t, bars, 6)

	assert.Equal(t, "2025-09", bars[0].Month)
	assert.Equal(t, "2026-02", bars[5].Month)
	assert.True(t, bars[5].Current)
	assert.False(t, bars[0].Current)
	assert.InDelta(t, 100, bars[3].Height, 0.001, "largest month is full height (2025-12)")
	assert.InDelta(t, 50, bars[1].Height, 0.001)
	assert.Zero(t, bars[0].Height)
	assert.Equal(t, 9, int(bars[0].MonthNum))

	none := Bars(map[string]float64{}, YearMonth{2026, time.February}, 6)
	assert.Zero(t, none[5].Height, "all-zero months do not divide by zero")
}

func TestMatchesQuery(t *testing.T) {
	assert.True(t, MatchesQuery("", "anything"))
	assert.True(t, MatchesQuery("  ", "anything"))
	assert.True(t, MatchesQuery("gas", "Gas", ""))
	assert.True(t, MatchesQuery("газ", "Gas", "Газ"), "matches the translated name")
	assert.True(t, MatchesQuery("LIDL", "Milk", "Lidl"))
	assert.False(t, MatchesQuery("water", "Gas", "Meter read"))
}
