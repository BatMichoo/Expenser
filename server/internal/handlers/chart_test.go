package handlers

import (
	"expenser/internal/models"
	"net/url"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
)

func TestParseTrendQuery(t *testing.T) {
	now := time.Date(2026, 11, 12, 0, 0, 0, 0, time.UTC)
	parse := func(sec models.Section, raw string) models.TrendQuery {
		v, err := url.ParseQuery(raw)
		if err != nil {
			t.Fatal(err)
		}
		return ParseTrendQuery(v, sec, now)
	}

	d := parse(models.SectionHouse, "")
	assert.Equal(t, models.TrendQuery{Tab: "month", Period: "12", YearA: 2026, YearB: 2025, By: "month"}, d, "defaults")

	assert.Equal(t, "month", parse(models.SectionHouse, "tab=nope").Tab)
	assert.Equal(t, "year", parse(models.SectionCar, "tab=year").Tab)
	assert.Equal(t, "month", parse(models.SectionCar, "tab=item").Tab, "item is groceries only")
	assert.Equal(t, "item", parse(models.SectionGroceries, "tab=item").Tab)

	assert.Equal(t, "12", parse(models.SectionHouse, "period=abc").Period)
	assert.Equal(t, "6", parse(models.SectionHouse, "period=6").Period)
	assert.Equal(t, "24", parse(models.SectionHouse, "period=24").Period)
	assert.Equal(t, "y2024", parse(models.SectionHouse, "period=y2024").Period)
	assert.Equal(t, "12", parse(models.SectionHouse, "period=y1999").Period, "year out of range")
	assert.Equal(t, "12", parse(models.SectionHouse, "period=y2028").Period, "year out of range")
	assert.Equal(t, "12", parse(models.SectionHouse, "period=y+2025").Period)
	assert.Equal(t, "12", parse(models.SectionHouse, "period=all").Period, "all is for the item tab")
	assert.Equal(t, "all", parse(models.SectionGroceries, "tab=item&period=all").Period)

	assert.Equal(t, 2026, parse(models.SectionHouse, "a=0").YearA)
	assert.Equal(t, 2026, parse(models.SectionHouse, "a=1999").YearA)
	assert.Equal(t, 2027, parse(models.SectionHouse, "a=2027").YearA, "next year allowed")
	assert.Equal(t, 2026, parse(models.SectionHouse, "a=2028").YearA)
	assert.Equal(t, 2023, parse(models.SectionHouse, "a=2024").YearB, "b missing")
	assert.Equal(t, 2023, parse(models.SectionHouse, "a=2024&b=2024").YearB, "b equal to a")
	assert.Equal(t, 2023, parse(models.SectionHouse, "a=2024&b=x").YearB, "b invalid")
	assert.Equal(t, 2023, parse(models.SectionHouse, "a=2024&b=1990").YearB, "b out of range")
	assert.Equal(t, 2026, parse(models.SectionHouse, "a=2022&b=2026").YearB, "b may be newer than a")

	assert.Equal(t, "month", parse(models.SectionGroceries, "by=x").By)
	assert.Equal(t, "year", parse(models.SectionGroceries, "by=year").By)

	assert.Equal(t, 0, parse(models.SectionHouse, "type=-3").TypeID)
	assert.Equal(t, 0, parse(models.SectionHouse, "type=abc").TypeID)
	assert.Equal(t, 4, parse(models.SectionHouse, "type=4").TypeID)

	assert.Equal(t, 0, parse(models.SectionGroceries, "item=-1").ItemID)
	assert.Equal(t, 7, parse(models.SectionGroceries, "item=7").ItemID)

	assert.Equal(t, "", parse(models.SectionGroceries, "unit=gallon").Unit)
	for _, u := range []string{"kg", "l", "pc"} {
		assert.Equal(t, u, parse(models.SectionGroceries, "unit="+u).Unit)
	}
}
