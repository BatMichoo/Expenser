package services

import (
	"context"
	"fmt"
	"sort"
	"time"

	"expenser/internal/models"

	"github.com/google/uuid"
)

// homeBillCount is how many upcoming bills the home page lists; homeBars is
// how many months of bars each card shows.
const (
	homeBillCount = 3
	homeBars      = 12
)

// BillTypeTotaler sums expenses of one bill type over a date range.
type BillTypeTotaler interface {
	GetBillTypeTotal(ctx context.Context, userID uuid.UUID, category models.RecurringPaymentCategory, typeID int, from, to time.Time) (float64, bool, error)
}

// HomeStore is the data access the home page needs. *database.DB
// implements it.
type HomeStore interface {
	BillTypeTotaler
	GetHouseMonthlyTotals(userID uuid.UUID, from, to time.Time) (map[string]float64, error)
	GetCarMonthlyTotals(userID uuid.UUID, from, to time.Time) (map[string]float64, error)
	GetGroceriesMonthlyTotals(userID uuid.UUID, from, to time.Time) (map[string]float64, error)
	GetHouseTypeTotals(userID uuid.UUID, from, to time.Time) (map[string]float64, error)
	GetCarTypeTotals(userID uuid.UUID, from, to time.Time) (map[string]float64, error)
	GetGroceriesStoreTotals(userID uuid.UUID, from, to time.Time) (map[string]float64, error)
	ListRecurringPayments(ctx context.Context, userID uuid.UUID) ([]models.RecurringPayment, error)
}

// ytdRange is 1 January to today (inclusive) this year, and the same
// stretch of last year. Bounds are exclusive at the end.
type ytdRange struct {
	ThisFrom, ThisTo, LastFrom, LastTo time.Time
}

func yearToDate(now time.Time) ytdRange {
	y, m, d := now.Date()
	loc := now.Location()
	lastDay := time.Date(y-1, m, d, 0, 0, 0, 0, loc)
	if lastDay.Month() != m { // 29 February: last year ends on the 28th
		lastDay = time.Date(y-1, m+1, 0, 0, 0, 0, 0, loc)
	}
	return ytdRange{
		ThisFrom: time.Date(y, time.January, 1, 0, 0, 0, 0, loc),
		ThisTo:   time.Date(y, m, d+1, 0, 0, 0, 0, loc),
		LastFrom: time.Date(y-1, time.January, 1, 0, 0, 0, 0, loc),
		LastTo:   lastDay.AddDate(0, 0, 1),
	}
}

func sumTotals(m map[string]float64) float64 {
	total := 0.0
	for _, v := range m {
		total += v
	}
	return total
}

// buildHomeCategory turns the raw totals into a card. monthly is keyed
// "YYYY-MM" and covers at least the last 12 months; top is the year's
// totals per type or store.
func buildHomeCategory(section string, yearTotal, lastYearTotal float64, monthly, top map[string]float64, now time.Time) models.HomeCategory {
	ym := YearMonth{now.Year(), now.Month()}
	c := models.HomeCategory{
		Section:       section,
		YearTotal:     yearTotal,
		LastYearTotal: lastYearTotal,
		ThisMonth:     monthly[ym.String()],
		Bars:          Bars(monthly, ym, homeBars),
		Empty:         yearTotal <= 0,
	}
	c.ChangePct, c.HasLastYear = ChangePercent(yearTotal, lastYearTotal)

	if all := sumTotals(top); all > 0 {
		names := make([]string, 0, len(top))
		for name := range top {
			names = append(names, name)
		}
		sort.Strings(names) // ties go to the first name, so the card is stable
		for _, name := range names {
			if top[name] > top[c.TopName] || c.TopName == "" {
				c.TopName = name
			}
		}
		c.TopPct = top[c.TopName] / all * 100
	}
	return c
}

// pickHomeBills returns the first n active bills by due date (overdue and
// waiting ones come first, as their dates are earliest) and how many more
// active bills there are.
func pickHomeBills(payments []models.RecurringPayment, today time.Time, n int) ([]models.HomeBillRow, int) {
	active := make([]models.RecurringPayment, 0, len(payments))
	for _, p := range payments {
		if p.Active {
			active = append(active, p)
		}
	}
	sort.SliceStable(active, func(i, j int) bool { return active[i].DueDate.Before(active[j].DueDate) })
	more := 0
	if len(active) > n {
		more = len(active) - n
		active = active[:n]
	}
	rows := make([]models.HomeBillRow, len(active))
	for i := range active {
		rows[i] = models.HomeBillRow{Payment: active[i], DaysLeft: active[i].DaysUntilDue(today)}
	}
	return rows, more
}

// homeBillsLastYear fills in what each bill's type cost in its due month,
// one year earlier.
func homeBillsLastYear(ctx context.Context, store BillTypeTotaler, userID uuid.UUID, rows []models.HomeBillRow) ([]models.HomeBillRow, error) {
	for i := range rows {
		p := rows[i].Payment
		typeID := p.UtilityTypeID
		if p.Category == models.RecurringCategoryCar {
			typeID = p.CarExpenseTypeID
		}
		if typeID == nil {
			continue
		}
		month := YearMonth{p.DueDate.Year() - 1, p.DueDate.Month()}
		total, found, err := store.GetBillTypeTotal(ctx, userID, p.Category, *typeID, month.Start(), month.End())
		if err != nil {
			return nil, err
		}
		rows[i].LastYear, rows[i].HasLastYear, rows[i].LastYearMonth = total, found, month.Month
	}
	return rows, nil
}

// HomeService builds the logged-in home page.
type HomeService struct {
	store HomeStore
}

func NewHomeService(store HomeStore) *HomeService {
	return &HomeService{store: store}
}

type sectionQueries struct {
	name    string
	monthly func(uuid.UUID, time.Time, time.Time) (map[string]float64, error)
	top     func(uuid.UUID, time.Time, time.Time) (map[string]float64, error)
}

func (s *HomeService) category(q sectionQueries, userID uuid.UUID, r ytdRange, now time.Time) (models.HomeCategory, error) {
	this, err := q.monthly(userID, r.ThisFrom, r.ThisTo)
	if err != nil {
		return models.HomeCategory{}, fmt.Errorf("%s this year: %w", q.name, err)
	}
	last, err := q.monthly(userID, r.LastFrom, r.LastTo)
	if err != nil {
		return models.HomeCategory{}, fmt.Errorf("%s last year: %w", q.name, err)
	}
	ym := YearMonth{now.Year(), now.Month()}
	first := ym
	for i := 1; i < homeBars; i++ {
		first = first.Prev()
	}
	monthly, err := q.monthly(userID, first.Start(), ym.End())
	if err != nil {
		return models.HomeCategory{}, fmt.Errorf("%s months: %w", q.name, err)
	}
	top, err := q.top(userID, r.ThisFrom, r.ThisTo)
	if err != nil {
		return models.HomeCategory{}, fmt.Errorf("%s top: %w", q.name, err)
	}
	return buildHomeCategory(q.name, sumTotals(this), sumTotals(last), monthly, top, now), nil
}

// Build loads every card for the user. name is who to greet.
func (s *HomeService) Build(ctx context.Context, userID uuid.UUID, name, lang string, now time.Time) (*models.HomeView, error) {
	r := yearToDate(now)
	v := &models.HomeView{Name: name, Lang: lang}
	var err error
	for _, c := range []struct {
		dst *models.HomeCategory
		q   sectionQueries
	}{
		{&v.House, sectionQueries{"house", s.store.GetHouseMonthlyTotals, s.store.GetHouseTypeTotals}},
		{&v.Car, sectionQueries{"car", s.store.GetCarMonthlyTotals, s.store.GetCarTypeTotals}},
		{&v.Groceries, sectionQueries{"groceries", s.store.GetGroceriesMonthlyTotals, s.store.GetGroceriesStoreTotals}},
	} {
		if *c.dst, err = s.category(c.q, userID, r, now); err != nil {
			return nil, err
		}
	}

	payments, err := s.store.ListRecurringPayments(ctx, userID)
	if err != nil {
		return nil, fmt.Errorf("bills: %w", err)
	}
	rows, more := pickHomeBills(payments, now, homeBillCount)
	if v.Bills, err = homeBillsLastYear(ctx, s.store, userID, rows); err != nil {
		return nil, fmt.Errorf("bills last year: %w", err)
	}
	v.MoreBills = more
	return v, nil
}
