package services

import (
	"expenser/internal/models"
	"expenser/internal/utilities"
	"fmt"
	"strings"
	"time"

	"github.com/google/uuid"
)

// MonthStore is the data access the month views need. *database.DB
// implements it.
type MonthStore interface {
	GetHouseExpensesForMonth(month time.Month, year int, userID uuid.UUID) (*[]models.HouseExpense, error)
	GetHouseMonthlyTotals(userID uuid.UUID, from, to time.Time) (map[string]float64, error)
	GetHouseUtilityTypes() (*[]models.HomeUtilityType, error)
	GetCarExpensesForMonth(month time.Month, year int, userID uuid.UUID) (*[]models.CarExpense, error)
	GetCarMonthlyTotals(userID uuid.UUID, from, to time.Time) (map[string]float64, error)
	GetCarExpenseTypes() (*[]models.CarExpenseType, error)
	GetGroceriesExpensesForMonth(month time.Month, year int, userID uuid.UUID) (*[]models.GroceriesExpense, error)
	GetGroceriesMonthlyTotals(userID uuid.UUID, from, to time.Time) (map[string]float64, error)
	GetAllCategories() ([]models.Category, error)
}

// MonthFilter narrows the expense list (not the KPIs) of a month view.
type MonthFilter struct {
	Query  string
	TypeID int // 0 = all types
}

// barMonths is how many months the mini chart shows.
const barMonths = 6

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
	totals map[string]float64, lang string, now time.Time) *models.MonthView {
	summary := Summarize(items)
	change, hasChange := ChangePercent(summary.Total, totals[ym.Prev().String()])
	return &models.MonthView{
		Section:   section,
		Month:     ym.String(),
		MonthNum:  ym.Month,
		Year:      ym.Year,
		PrevMonth: ym.Prev().String(),
		NextMonth: ym.Next().String(),
		IsCurrent: ym == YearMonth{now.Year(), now.Month()},
		Summary:   summary,
		ChangePct: change,
		HasChange: hasChange,
		Bars:      Bars(totals, ym, barMonths),
		Query:     filter.Query,
		TypeID:    filter.TypeID,
		Filtered:  filter.TypeID != 0 || strings.TrimSpace(filter.Query) != "",
		Lang:      lang,
	}
}

// totalsWindow is the date range covering the mini chart (which includes
// the previous month used for the change percentage).
func totalsWindow(ym YearMonth) (time.Time, time.Time) {
	first := ym
	for i := 1; i < barMonths; i++ {
		first = first.Prev()
	}
	return first.Start(), ym.End()
}

// House builds the house page for one month.
func (s *MonthViewService) House(userID uuid.UUID, ym YearMonth, filter MonthFilter, lang string, now time.Time) (*models.MonthView, error) {
	rows, err := s.store.GetHouseExpensesForMonth(ym.Month, ym.Year, userID)
	if err != nil {
		return nil, fmt.Errorf("house expenses for %s: %w", ym, err)
	}
	from, to := totalsWindow(ym)
	totals, err := s.store.GetHouseMonthlyTotals(userID, from, to)
	if err != nil {
		return nil, fmt.Errorf("house totals: %w", err)
	}
	types, err := s.store.GetHouseUtilityTypes()
	if err != nil {
		return nil, fmt.Errorf("house types: %w", err)
	}

	items := make([]SummaryItem, 0, len(*rows))
	shown := []models.HouseExpense{}
	for _, e := range *rows {
		items = append(items, SummaryItem{Type: e.UtilityType, Amount: e.Amount})
		if filter.TypeID != 0 && e.UtilityTypeID != filter.TypeID {
			continue
		}
		if MatchesQuery(filter.Query, e.UtilityType, utilities.T(lang, "type."+e.UtilityType), e.Notes, e.FormattedMetadata(lang)) {
			shown = append(shown, e)
		}
	}

	v := s.base("house", ym, filter, items, totals, lang, now)
	v.House = shown
	v.TypePrefix = "type."
	for _, t := range *types {
		v.Types = append(v.Types, models.TypeOption{ID: t.ID, Name: t.Name})
	}
	return v, nil
}

// Car builds the car page for one month.
func (s *MonthViewService) Car(userID uuid.UUID, ym YearMonth, filter MonthFilter, lang string, now time.Time) (*models.MonthView, error) {
	rows, err := s.store.GetCarExpensesForMonth(ym.Month, ym.Year, userID)
	if err != nil {
		return nil, fmt.Errorf("car expenses for %s: %w", ym, err)
	}
	from, to := totalsWindow(ym)
	totals, err := s.store.GetCarMonthlyTotals(userID, from, to)
	if err != nil {
		return nil, fmt.Errorf("car totals: %w", err)
	}
	types, err := s.store.GetCarExpenseTypes()
	if err != nil {
		return nil, fmt.Errorf("car types: %w", err)
	}

	items := make([]SummaryItem, 0, len(*rows))
	shown := []models.CarExpense{}
	for _, e := range *rows {
		items = append(items, SummaryItem{Type: e.Type, Amount: e.Amount})
		if filter.TypeID != 0 && e.ExpenseTypeID != filter.TypeID {
			continue
		}
		if MatchesQuery(filter.Query, e.Type, utilities.T(lang, "type."+e.Type), e.Notes, e.FormattedMetadata(lang)) {
			shown = append(shown, e)
		}
	}

	v := s.base("car", ym, filter, items, totals, lang, now)
	v.Car = shown
	v.TypePrefix = "type."
	for _, t := range *types {
		v.Types = append(v.Types, models.TypeOption{ID: t.ID, Name: t.Name})
	}
	return v, nil
}

// Groceries builds the groceries page for one month. Amounts are net of
// discounts, like the monthly totals.
func (s *MonthViewService) Groceries(userID uuid.UUID, ym YearMonth, filter MonthFilter, lang string, now time.Time) (*models.MonthView, error) {
	rows, err := s.store.GetGroceriesExpensesForMonth(ym.Month, ym.Year, userID)
	if err != nil {
		return nil, fmt.Errorf("groceries for %s: %w", ym, err)
	}
	from, to := totalsWindow(ym)
	totals, err := s.store.GetGroceriesMonthlyTotals(userID, from, to)
	if err != nil {
		return nil, fmt.Errorf("groceries totals: %w", err)
	}
	categories, err := s.store.GetAllCategories()
	if err != nil {
		return nil, fmt.Errorf("groceries categories: %w", err)
	}

	items := make([]SummaryItem, 0, len(*rows))
	shown := []models.GroceriesExpense{}
	for _, e := range *rows {
		items = append(items, SummaryItem{Type: e.CategoryName, Amount: e.TotalPrice - e.TotalDiscount})
		if filter.TypeID != 0 && e.CategoryID != filter.TypeID {
			continue
		}
		if MatchesQuery(filter.Query, e.Product, e.CategoryName, utilities.T(lang, "category."+e.CategoryName), e.SupermarketName) {
			shown = append(shown, e)
		}
	}

	v := s.base("groceries", ym, filter, items, totals, lang, now)
	v.Groceries = shown
	v.TypePrefix = "category."
	for _, c := range categories {
		v.Types = append(v.Types, models.TypeOption{ID: c.ID, Name: c.Name})
	}
	return v, nil
}
