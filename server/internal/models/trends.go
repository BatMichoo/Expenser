package models

import (
	"errors"
	"expenser/internal/chart"
	"sort"
	"strconv"
	"strings"
	"time"
)

// LegendItem is one entry of a trend chart legend.
type LegendItem struct {
	Name   string
	Color  int
	Dashed bool
}

// MonthTypeTotal is the net spend of one expense type (or grocery category)
// in one month.
type MonthTypeTotal struct {
	Month  string // YYYY-MM
	TypeID int
	Type   string
	Amount float64
}

// GroceryLine is one grocery purchase as the item-detail trends see it.
type GroceryLine struct {
	ID            int
	CategoryID    int
	Category      string
	Product       string
	Store         string
	Date          time.Time
	Quantity      float64
	UnitSize      float64
	TotalPrice    float64
	TotalDiscount float64
	Unit          string
	SizeStatus    string
}

// ErrInvalidSize is returned when a unit and size cannot be stored.
var ErrInvalidSize = errors.New("the unit or size is not valid")

// TrendQuery is the parsed query string of a section's charts page.
type TrendQuery struct {
	Tab    string // "month" | "year" | "item"
	Period string // "6" | "12" | "24" | "y<YYYY>" | "all" (item only)
	TypeID int
	YearA  int
	YearB  int
	ItemID int
	By     string // "month" | "year"
	Unit   string
}

// TrendsPage is the view model of one section's charts page. Only the field
// of the active tab (Month or Year) is set.
type TrendsPage struct {
	Section string
	Query   TrendQuery
	Lang    string
	// TypePrefix is the translation key prefix for series names: "type." for
	// house and car, "category." for groceries. Legend entries and series
	// with Color 0 are already translated ("Everything else") and must not
	// be prefixed.
	TypePrefix string
	Types      []TypeOption
	Years      []int
	Unsized    int // groceries only
	Month      *MonthTrend
	Year       *YearTrend
	Items      []Category // item tab: tracked categories for the picker
	Item       *ItemTrend
}

// MonthTrend is the month-to-month tab: stacked spend per month.
type MonthTrend struct {
	Total, PrevTotal, ChangePct float64
	HasChange                   bool
	Median                      float64
	TopMonth                    string // column label
	TopMonthAmount              float64
	TopMonthType                string // raw type name
	Chart                       chart.Bars
	Legend                      []LegendItem
	Empty                       bool
}

// YearTrend is the year-vs-year tab: one grouped bar pair per month.
type YearTrend struct {
	YearA, YearB             int
	SoFarA, SoFarB, SoFarPct float64
	HasSoFar                 bool
	SoFarLabel               string
	Riser                    string // raw type name
	RiserAmount              float64
	HasRiser                 bool
	SavedA, SavedB           float64
	Chart                    chart.Bars
	Legend                   []LegendItem
	YearTotals               []YearTotal // newest year first
	Empty                    bool
}

// YearTotal is the net spend of one calendar year.
type YearTotal struct {
	Year  int
	Total float64
}

// YearChoices are the years offered in the Year vs year pickers: every year
// with data plus the two picked years, newest first.
func (p *TrendsPage) YearChoices() []int {
	return mergeYears(p.Years, p.Query.YearA, p.Query.YearB)
}

// PeriodYears are the calendar years offered as Month to month periods:
// every year with data plus the picked "y<YYYY>" period, newest first.
func (p *TrendsPage) PeriodYears() []int {
	var extra []int
	if y, err := strconv.Atoi(strings.TrimPrefix(p.Query.Period, "y")); err == nil && strings.HasPrefix(p.Query.Period, "y") {
		extra = append(extra, y)
	}
	return mergeYears(p.Years, extra...)
}

func mergeYears(years []int, extra ...int) []int {
	seen := map[int]bool{}
	var out []int
	for _, y := range append(append([]int(nil), years...), extra...) {
		if y > 0 && !seen[y] {
			seen[y] = true
			out = append(out, y)
		}
	}
	sort.Sort(sort.Reverse(sort.IntSlice(out)))
	return out
}

// PriceLine is one purchase with its price per kg, l or piece.
type PriceLine struct {
	Date            time.Time
	Store, Product  string
	PerUnit         float64
	DiscountPerUnit float64
}

// ItemRow is one row of the all-items price-change table.
type ItemRow struct {
	ItemID       int
	Item, Unit   string
	Now, YearAgo float64
	ChangePct    float64
	HasChange    bool
	Bought12     float64
}

// ItemTrend is the item detail tab: true price per unit of one grocery
// category. Prices only come from lines with a known size in the chosen unit.
type ItemTrend struct {
	ItemID         int
	Item           string
	Unit           string
	Units          []string
	By             string
	PriceNow       float64
	HasPriceNow    bool
	PriceChangePct float64
	HasPriceChange bool
	QtyNow         float64
	QtyUsual       float64
	HasQtyUsual    bool
	Cheapest       *PriceLine
	Discounts      []PriceLine // top 5 by discount per unit
	TotalSaved     float64
	Excluded       int
	Chart          chart.PriceChart
	Items          []ItemRow
	Empty          bool
}

// UnitKey is the i18n key of the unit's short label ("kg", "l", "pcs").
func (t ItemTrend) UnitKey() string { return "trends.unit_" + t.Unit }

// UnitKey is the i18n key of the row's unit label.
func (r ItemRow) UnitKey() string { return "trends.unit_" + r.Unit }

// SizeRow is one purchase on the "needs a size" list, with the size to
// prefill its fields with. Suggested marks a size that was guessed rather
// than typed.
type SizeRow struct {
	Expense   GroceriesExpense
	Unit      string
	Size      float64
	Suggested bool
}

// SizesPage is the "needs a size" review list.
type SizesPage struct {
	Section      string // always "groceries"; picks the section buttons
	Rows         []SizeRow
	Count        int
	Lang         string
	SuggestError string
	SuggestNote  string // e.g. only the first rows were asked about
}
