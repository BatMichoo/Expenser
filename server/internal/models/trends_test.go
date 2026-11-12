package models

import (
	"reflect"
	"testing"
)

func TestTrendsPageYearChoices(t *testing.T) {
	p := &TrendsPage{Years: []int{2024, 2026}, Query: TrendQuery{YearA: 2027, YearB: 2026, Period: "y2019"}}
	if got := p.YearChoices(); !reflect.DeepEqual(got, []int{2027, 2026, 2024}) {
		t.Errorf("YearChoices %v", got)
	}
	if got := p.PeriodYears(); !reflect.DeepEqual(got, []int{2026, 2024, 2019}) {
		t.Errorf("PeriodYears %v", got)
	}
	p.Query.Period = "12"
	if got := p.PeriodYears(); !reflect.DeepEqual(got, []int{2026, 2024}) {
		t.Errorf("PeriodYears without year %v", got)
	}
}
