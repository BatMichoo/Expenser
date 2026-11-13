package services

import (
	"context"
	"expenser/internal/chart"
	"expenser/internal/models"
	"expenser/internal/utilities"
	"fmt"
	"sort"
	"strings"
	"time"

	"github.com/google/uuid"
)

// MonthStore is the data access the month views need. *database.DB
// implements it.
type MonthStore interface {
	GetHouseExpensesForMonth(ctx context.Context, month time.Month, year int, userID uuid.UUID) ([]models.HouseExpense, error)
	GetHouseMonthlyTotals(ctx context.Context, userID uuid.UUID, from, to time.Time) (map[string]float64, error)
	GetHouseUtilityTypes(ctx context.Context) ([]models.HomeUtilityType, error)
	GetCarExpensesForMonth(ctx context.Context, month time.Month, year int, userID uuid.UUID) ([]models.CarExpense, error)
	GetCarMonthlyTotals(ctx context.Context, userID uuid.UUID, from, to time.Time) (map[string]float64, error)
	GetCarExpenseTypes(ctx context.Context) ([]models.CarExpenseType, error)
	GetGroceriesExpensesForMonth(ctx context.Context, month time.Month, year int, userID uuid.UUID) ([]models.GroceriesExpense, error)
	GetGroceriesMonthlyTotals(ctx context.Context, userID uuid.UUID, from, to time.Time) (map[string]float64, error)
	GetAllCategories(ctx context.Context) ([]models.Category, error)
	GetUpcomingRecurringPayments(ctx context.Context, userID uuid.UUID, until time.Time) ([]models.RecurringPayment, error)
	GetHouseTypeTotals(ctx context.Context, userID uuid.UUID, from, to time.Time) (map[string]float64, error)
	GetCarTypeTotals(ctx context.Context, userID uuid.UUID, from, to time.Time) (map[string]float64, error)
	GetGroceriesCategoryTotals(ctx context.Context, userID uuid.UUID, from, to time.Time) (map[string]float64, error)
	GetHouseSpendHistory(ctx context.Context, userID uuid.UUID, from, to time.Time) ([]models.SpendEntry, error)
	GetCarSpendHistory(ctx context.Context, userID uuid.UUID, from, to time.Time) ([]models.SpendEntry, error)
	GetGroceriesSpendHistory(ctx context.Context, userID uuid.UUID, from, to time.Time) ([]models.SpendEntry, error)
}

// spendHistoryFunc is one section's per-type daily spending query.
type spendHistoryFunc func(ctx context.Context, userID uuid.UUID, from, to time.Time) ([]models.SpendEntry, error)

// groceriesAsOneType drops the category from groceries history. Shopping
// is steady as a whole but each category comes and goes, so per category
// the estimate would take most of them for one-offs (and backtests showed
// the section as one type predicts as well or better).
func groceriesAsOneType(history spendHistoryFunc) spendHistoryFunc {
	return func(ctx context.Context, userID uuid.UUID, from, to time.Time) ([]models.SpendEntry, error) {
		entries, err := history(ctx, userID, from, to)
		for i := range entries {
			entries[i].Type = ""
		}
		return entries, err
	}
}

// addEstimate fills in the month-end estimate when v is the current month.
func addEstimate(ctx context.Context, v *models.MonthView, history spendHistoryFunc, userID uuid.UUID, ym YearMonth, now time.Time) error {
	if !v.IsCurrent {
		return nil
	}
	from, to := EstimateHistoryWindow(ym)
	entries, err := history(ctx, userID, from, to)
	if err != nil {
		return fmt.Errorf("%s spend history: %w", v.Section, err)
	}
	e := EstimateMonthEnd(entries, ym, now)
	v.Estimate, v.HasEstimate, v.EstimateFromPace = e.Amount, e.OK, e.FromPace
	return nil
}

// MonthFilter narrows the expense list (not the KPIs) of a month view.
type MonthFilter struct {
	Query  string
	TypeID int // 0 = all types
}

// barMonths is how many months the mini chart shows; usualMonths is how
// many previous months "usual" spending is averaged over.
const (
	barMonths   = 6
	usualMonths = 6
)

// MonthViewService builds the per-month page data for each section.
type MonthViewService struct {
	store MonthStore
}

func NewMonthViewService(store MonthStore) *MonthViewService {
	return &MonthViewService{store: store}
}

// base fills the fields every section shares: month navigation, KPIs,
// change vs previous month and the mini-chart bars.
func (s *MonthViewService) base(section string, ym YearMonth, filter MonthFilter, items []SummaryItem,
	totals, typeTotals map[string]float64, lang string, now time.Time) *models.MonthView {
	summary := Summarize(items)
	isCurrent := ym == YearMonth{now.Year(), now.Month()}
	bars := Bars(totals, ym, barMonths)
	barsTotal := 0.0
	for _, b := range bars {
		barsTotal += b.Total
	}
	change, hasChange := ChangePercent(summary.Total, totals[ym.Prev().String()])
	return &models.MonthView{
		Section:   section,
		Month:     ym.String(),
		MonthNum:  ym.Month,
		Year:      ym.Year,
		PrevMonth: ym.Prev().String(),
		NextMonth: ym.Next().String(),
		IsCurrent: isCurrent,
		Summary:   summary,
		ChangePct: change,
		HasChange: hasChange,
		Bars:      bars,
		BarsTotal: barsTotal,
		Query:     filter.Query,
		TypeID:    filter.TypeID,
		Filtered:  filter.TypeID != 0 || strings.TrimSpace(filter.Query) != "",
		Lang:      lang,

		Breakdown: Breakdown(items, typeTotals, usualMonths),
		YearSoFar: YearSoFar(totals, ym),
	}
}

// colorBreakdown gives each breakdown entry its type's fixed colour, found
// by name in v.Types. A type that is not listed keeps colour 0 (grey).
func colorBreakdown(v *models.MonthView) {
	ids := make(map[string]int, len(v.Types))
	for _, t := range v.Types {
		ids[t.Name] = t.ID
	}
	for i := range v.Breakdown {
		v.Breakdown[i].Color = chart.TypeColor(ids[v.Breakdown[i].Type])
	}
}

// rankColorBreakdown colours the groceries breakdown like the trends grocery
// chart: there are far more categories than colours, so the six largest
// (largest first, Other excluded) get slots 1..6 and the rest, and Other,
// get colour 0.
func rankColorBreakdown(v *models.MonthView) {
	slot := 0
	for i := range v.Breakdown {
		v.Breakdown[i].Color = 0
		if v.Breakdown[i].Type != otherCategory && slot < topCategories {
			slot++
			v.Breakdown[i].Color = slot
		}
	}
}

// totalsWindow is the date range of monthly totals a view needs: the mini
// chart and the usual months before ym, and January onwards for the year
// so far.
func totalsWindow(ym YearMonth) (time.Time, time.Time) {
	first := ym
	for i := 0; i < usualMonths; i++ {
		first = first.Prev()
	}
	if jan := (YearMonth{ym.Year, time.January}); jan.Start().Before(first.Start()) {
		first = jan
	}
	return first.Start(), ym.End()
}

// usualWindow is the range "usual" per-type spending is summed over: the
// months before ym.
func usualWindow(ym YearMonth) (time.Time, time.Time) {
	first := ym
	for i := 0; i < usualMonths; i++ {
		first = first.Prev()
	}
	return first.Start(), ym.Start()
}

// House builds the house page for one month.
func (s *MonthViewService) House(ctx context.Context, userID uuid.UUID, ym YearMonth, filter MonthFilter, lang string, now time.Time) (*models.MonthView, error) {
	rows, err := s.store.GetHouseExpensesForMonth(ctx, ym.Month, ym.Year, userID)
	if err != nil {
		return nil, fmt.Errorf("house expenses for %s: %w", ym, err)
	}
	from, to := totalsWindow(ym)
	totals, err := s.store.GetHouseMonthlyTotals(ctx, userID, from, to)
	if err != nil {
		return nil, fmt.Errorf("house totals: %w", err)
	}
	types, err := s.store.GetHouseUtilityTypes(ctx)
	if err != nil {
		return nil, fmt.Errorf("house types: %w", err)
	}

	items := make([]SummaryItem, 0, len(rows))
	shown := []models.HouseExpense{}
	for _, e := range rows {
		items = append(items, SummaryItem{Type: e.UtilityType, Amount: e.Amount - e.DiscountAmount})
		if filter.TypeID != 0 && e.UtilityTypeID != filter.TypeID {
			continue
		}
		if MatchesQuery(filter.Query, e.UtilityType, utilities.T(lang, "type."+e.UtilityType), e.Notes, utilities.MetadataDetails(e.UnmarshalMetadata(), lang)) {
			shown = append(shown, e)
		}
	}

	uFrom, uTo := usualWindow(ym)
	typeTotals, err := s.store.GetHouseTypeTotals(ctx, userID, uFrom, uTo)
	if err != nil {
		return nil, fmt.Errorf("house type totals: %w", err)
	}

	v := s.base("house", ym, filter, items, totals, typeTotals, lang, now)
	if err := addEstimate(ctx, v, s.store.GetHouseSpendHistory, userID, ym, now); err != nil {
		return nil, err
	}
	if v.UpcomingBills, err = s.upcomingBills(ctx, userID, models.SectionHouse, v.IsCurrent, now); err != nil {
		return nil, err
	}
	v.House = shown
	v.TypePrefix = "type."
	for _, t := range types {
		v.Types = append(v.Types, models.TypeOption{ID: t.ID, Name: t.Name})
	}
	colorBreakdown(v)
	return v, nil
}

// Car builds the car page for one month.
func (s *MonthViewService) Car(ctx context.Context, userID uuid.UUID, ym YearMonth, filter MonthFilter, lang string, now time.Time) (*models.MonthView, error) {
	rows, err := s.store.GetCarExpensesForMonth(ctx, ym.Month, ym.Year, userID)
	if err != nil {
		return nil, fmt.Errorf("car expenses for %s: %w", ym, err)
	}
	from, to := totalsWindow(ym)
	totals, err := s.store.GetCarMonthlyTotals(ctx, userID, from, to)
	if err != nil {
		return nil, fmt.Errorf("car totals: %w", err)
	}
	types, err := s.store.GetCarExpenseTypes(ctx)
	if err != nil {
		return nil, fmt.Errorf("car types: %w", err)
	}

	items := make([]SummaryItem, 0, len(rows))
	shown := []models.CarExpense{}
	for _, e := range rows {
		items = append(items, SummaryItem{Type: e.Type, Amount: e.Amount - e.DiscountAmount})
		if filter.TypeID != 0 && e.ExpenseTypeID != filter.TypeID {
			continue
		}
		if MatchesQuery(filter.Query, e.Type, utilities.T(lang, "type."+e.Type), e.Notes, utilities.MetadataDetails(e.UnmarshalMetadata(), lang)) {
			shown = append(shown, e)
		}
	}

	uFrom, uTo := usualWindow(ym)
	typeTotals, err := s.store.GetCarTypeTotals(ctx, userID, uFrom, uTo)
	if err != nil {
		return nil, fmt.Errorf("car type totals: %w", err)
	}

	v := s.base("car", ym, filter, items, totals, typeTotals, lang, now)
	if err := addEstimate(ctx, v, s.store.GetCarSpendHistory, userID, ym, now); err != nil {
		return nil, err
	}
	if v.UpcomingBills, err = s.upcomingBills(ctx, userID, models.SectionCar, v.IsCurrent, now); err != nil {
		return nil, err
	}
	v.Car = shown
	v.TypePrefix = "type."
	for _, t := range types {
		v.Types = append(v.Types, models.TypeOption{ID: t.ID, Name: t.Name})
	}
	colorBreakdown(v)
	return v, nil
}

// Groceries builds the groceries page for one month. Amounts are net of
// discounts, like the monthly totals.
func (s *MonthViewService) Groceries(ctx context.Context, userID uuid.UUID, ym YearMonth, filter MonthFilter, lang string, now time.Time) (*models.MonthView, error) {
	rows, err := s.store.GetGroceriesExpensesForMonth(ctx, ym.Month, ym.Year, userID)
	if err != nil {
		return nil, fmt.Errorf("groceries for %s: %w", ym, err)
	}
	from, to := totalsWindow(ym)
	totals, err := s.store.GetGroceriesMonthlyTotals(ctx, userID, from, to)
	if err != nil {
		return nil, fmt.Errorf("groceries totals: %w", err)
	}
	categories, err := s.store.GetAllCategories(ctx)
	if err != nil {
		return nil, fmt.Errorf("groceries categories: %w", err)
	}

	items := make([]SummaryItem, 0, len(rows))
	shown := []models.GroceriesExpense{}
	for _, e := range rows {
		items = append(items, SummaryItem{Type: e.CategoryName, Amount: e.TotalPrice - e.TotalDiscount})
		if filter.TypeID != 0 && e.CategoryID != filter.TypeID {
			continue
		}
		if MatchesQuery(filter.Query, e.Product, e.CategoryName, utilities.T(lang, "category."+e.CategoryName), e.SupermarketName) {
			shown = append(shown, e)
		}
	}

	uFrom, uTo := usualWindow(ym)
	typeTotals, err := s.store.GetGroceriesCategoryTotals(ctx, userID, uFrom, uTo)
	if err != nil {
		return nil, fmt.Errorf("groceries type totals: %w", err)
	}

	v := s.base("groceries", ym, filter, items, totals, typeTotals, lang, now)
	if err := addEstimate(ctx, v, groceriesAsOneType(s.store.GetGroceriesSpendHistory), userID, ym, now); err != nil {
		return nil, err
	}
	v.Groceries = shown
	v.TypePrefix = "category."
	for _, c := range categories {
		v.Types = append(v.Types, models.TypeOption{ID: c.ID, Name: c.Name})
	}
	rankColorBreakdown(v)
	return v, nil
}

// upcomingBillDays is how far ahead the Upcoming bills panel looks.
const upcomingBillDays = 30

// upcomingBills returns the user's unpaid bills of one category due in the
// next 30 days (overdue included), soonest first. Only the current month
// shows them: they are about today, not the month being browsed.
func (s *MonthViewService) upcomingBills(ctx context.Context, userID uuid.UUID, category models.Section, isCurrent bool, now time.Time) ([]models.UpcomingPayment, error) {
	if !isCurrent {
		return nil, nil
	}
	today := models.DateOnly(now)
	payments, err := s.store.GetUpcomingRecurringPayments(ctx, userID, today.AddDate(0, 0, upcomingBillDays))
	if err != nil {
		return nil, fmt.Errorf("upcoming bills: %w", err)
	}
	sort.SliceStable(payments, func(i, j int) bool { return payments[i].DueDate.Before(payments[j].DueDate) })

	var out []models.UpcomingPayment
	for _, p := range payments {
		if p.Category != category || !p.Active {
			continue
		}
		left := p.DaysUntilDue(today)
		out = append(out, models.UpcomingPayment{Payment: p, DaysLeft: left, Overdue: left < 0})
	}
	return out, nil
}
