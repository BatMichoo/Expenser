package database

import (
	"context"
	"database/sql"
	"testing"
	"time"

	"expenser/internal/models"

	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func utc(y int, m time.Month, d int) time.Time { return time.Date(y, m, d, 12, 0, 0, 0, time.UTC) }

func trendUser(t *testing.T, tdb *DB, name string) uuid.UUID {
	t.Helper()
	u := &models.User{Username: name, PasswordHash: TestUserRegisterModel.PasswordHash}
	require.NoError(t, tdb.CreateUser(u))
	return u.ID
}

func typeID(t *testing.T, tdb *DB, table, name string) int {
	t.Helper()
	var id int
	require.NoError(t, tdb.conn.QueryRow(`SELECT id FROM `+table+` WHERE name = $1`, name).Scan(&id))
	return id
}

func insertTyped(t *testing.T, tdb *DB, table, typeCol string, user uuid.UUID, typ int, amount, discount float64, d time.Time) {
	t.Helper()
	_, err := tdb.conn.Exec(`INSERT INTO `+table+` (`+typeCol+`, amount, discount_amount, expense_date, created_by) VALUES ($1,$2,$3,$4,$5)`,
		typ, amount, discount, d, user)
	require.NoError(t, err)
}

func insertGrocery(t *testing.T, tdb *DB, user uuid.UUID, cat, product string, total, discount float64, d time.Time, status string, unit *string, size *float64) int {
	t.Helper()
	var id int
	require.NoError(t, tdb.conn.QueryRow(`
		INSERT INTO groceries_expenses (user_id, category_id, product, quantity, unit_price, total_price, total_discount, supermarket_name, purchase_date, unit, unit_size, size_status)
		VALUES ($1,$2,$3,1,$4,$4,$5,'Lidl',$6,$7,$8,$9) RETURNING id`,
		user, categoryID(t, tdb, cat), product, total, discount, d, unit, size, status).Scan(&id))
	return id
}

type trendFixture struct {
	db     *DB
	a, b   uuid.UUID
	milk   int
	otherG int
}

func setupTrends(t *testing.T) *trendFixture {
	tdb, _ := setupUnitsTestDB(t)
	f := &trendFixture{db: tdb, a: trendUser(t, tdb, "TrendA"), b: trendUser(t, tdb, "TrendB")}
	elec, water := typeID(t, tdb, "utility_types", "Electricity"), typeID(t, tdb, "utility_types", "Water")
	fuel, ins := typeID(t, tdb, "car_expense_types", "Fuel"), typeID(t, tdb, "car_expense_types", "Insurance")
	for _, u := range []struct {
		id uuid.UUID
		k  float64
	}{{f.a, 1}, {f.b, 1000}} {
		insertTyped(t, tdb, "home_expenses", "utility_type_id", u.id, elec, 100*u.k, 10*u.k, utc(2025, 1, 5))
		insertTyped(t, tdb, "home_expenses", "utility_type_id", u.id, elec, 50*u.k, 0, utc(2025, 1, 20))
		insertTyped(t, tdb, "home_expenses", "utility_type_id", u.id, water, 30*u.k, 0, utc(2025, 2, 1))
		insertTyped(t, tdb, "home_expenses", "utility_type_id", u.id, elec, 70*u.k, 0, utc(2026, 1, 3))
		insertTyped(t, tdb, "car_expenses", "car_expense_type_id", u.id, fuel, 60*u.k, 5*u.k, utc(2025, 1, 7))
		insertTyped(t, tdb, "car_expenses", "car_expense_type_id", u.id, ins, 200*u.k, 0, utc(2025, 2, 10))
		insertTyped(t, tdb, "car_expenses", "car_expense_type_id", u.id, fuel, 40*u.k, 0, utc(2026, 3, 1))
		l, one := "l", 1.0
		f.milk = insertGrocery(t, tdb, u.id, "Milk", "Milk", 2*u.k, 0.5*u.k, utc(2025, 1, 3), "known", &l, &one)
		insertGrocery(t, tdb, u.id, "Milk", "Milk", 3*u.k, 0, utc(2025, 2, 3), "unknown", nil, nil)
		insertGrocery(t, tdb, u.id, "Bread", "Bread", 4*u.k, 1*u.k, utc(2026, 2, 3), "skipped", nil, nil)
		f.otherG = insertGrocery(t, tdb, u.id, "Other", "Soap", 9*u.k, 2*u.k, utc(2025, 1, 4), "not_tracked", nil, nil)
	}
	return f
}

func TestGetMonthTypeTotals(t *testing.T) {
	f := setupTrends(t)
	ctx := context.Background()
	type key struct {
		m, typ string
	}
	toMap := func(rows []models.MonthTypeTotal, err error) map[key]float64 {
		require.NoError(t, err)
		m := map[key]float64{}
		for _, r := range rows {
			m[key{r.Month, r.Type}] = r.Amount
			assert.NotZero(t, r.TypeID)
		}
		return m
	}
	from, to := utc(2025, 1, 1), utc(2025, 3, 1)

	house := toMap(f.db.GetMonthTypeTotals(ctx, f.a, models.SectionHouse, from, to))
	assert.Equal(t, map[key]float64{{"2025-01", "Electricity"}: 140, {"2025-02", "Water"}: 30}, house)

	car := toMap(f.db.GetMonthTypeTotals(ctx, f.a, models.SectionCar, from, to))
	assert.Equal(t, map[key]float64{{"2025-01", "Fuel"}: 55, {"2025-02", "Insurance"}: 200}, car)

	groc := toMap(f.db.GetMonthTypeTotals(ctx, f.a, models.SectionGroceries, from, to))
	assert.Equal(t, map[key]float64{{"2025-01", "Milk"}: 1.5, {"2025-01", "Other"}: 7, {"2025-02", "Milk"}: 3}, groc)

	// [from, to): the upper bound is exclusive, the lower inclusive.
	got := toMap(f.db.GetMonthTypeTotals(ctx, f.a, models.SectionHouse, utc(2025, 1, 5), utc(2025, 2, 1)))
	assert.Equal(t, map[key]float64{{"2025-01", "Electricity"}: 90 + 50}, got)
	got = toMap(f.db.GetMonthTypeTotals(ctx, f.a, models.SectionHouse, utc(2025, 1, 5).Add(time.Second), utc(2025, 2, 1)))
	assert.Equal(t, map[key]float64{{"2025-01", "Electricity"}: 50}, got)
	got = toMap(f.db.GetMonthTypeTotals(ctx, f.a, models.SectionHouse, utc(2025, 1, 1), utc(2025, 2, 1)))
	assert.NotContains(t, got, key{"2025-02", "Water"})

	_, err := f.db.GetMonthTypeTotals(ctx, f.a, models.Section("nope"), from, to)
	assert.Error(t, err)
}

func TestGetYearTotals(t *testing.T) {
	f := setupTrends(t)
	ctx := context.Background()
	h, err := f.db.GetYearTotals(ctx, f.a, models.SectionHouse)
	require.NoError(t, err)
	assert.Equal(t, map[int]float64{2025: 170, 2026: 70}, h)
	c, err := f.db.GetYearTotals(ctx, f.a, models.SectionCar)
	require.NoError(t, err)
	assert.Equal(t, map[int]float64{2025: 255, 2026: 40}, c)
	g, err := f.db.GetYearTotals(ctx, f.a, models.SectionGroceries)
	require.NoError(t, err)
	assert.Equal(t, map[int]float64{2025: 11.5, 2026: 3}, g)
}

func TestGetGroceriesDiscountTotal(t *testing.T) {
	f := setupTrends(t)
	got, err := f.db.GetGroceriesDiscountTotal(context.Background(), f.a, utc(2025, 1, 1), utc(2026, 1, 1))
	require.NoError(t, err)
	assert.Equal(t, 2.5, got)
	got, err = f.db.GetGroceriesDiscountTotal(context.Background(), f.a, utc(2030, 1, 1), utc(2031, 1, 1))
	require.NoError(t, err)
	assert.Equal(t, 0.0, got)
}

func TestGetGroceryLines(t *testing.T) {
	f := setupTrends(t)
	ctx := context.Background()
	lines, err := f.db.GetGroceryLines(ctx, f.a, 0, utc(2025, 1, 1), utc(2027, 1, 1))
	require.NoError(t, err)
	require.Len(t, lines, 3)
	for _, l := range lines {
		assert.NotEqual(t, "Other", l.Category)
	}
	assert.True(t, lines[0].Date.Before(lines[1].Date) && lines[1].Date.Before(lines[2].Date))
	assert.Equal(t, "known", lines[0].SizeStatus)
	assert.Equal(t, "l", lines[0].Unit)
	assert.Equal(t, 1.0, lines[0].UnitSize)
	assert.Equal(t, 2.0, lines[0].TotalPrice)
	assert.Equal(t, 0.5, lines[0].TotalDiscount)
	assert.Equal(t, "Lidl", lines[0].Store)
	assert.Equal(t, "unknown", lines[1].SizeStatus)
	assert.Equal(t, "skipped", lines[2].SizeStatus)

	milk := categoryID(t, f.db, "Milk")
	lines, err = f.db.GetGroceryLines(ctx, f.a, milk, utc(2025, 1, 1), utc(2027, 1, 1))
	require.NoError(t, err)
	assert.Len(t, lines, 2)
	lines, err = f.db.GetGroceryLines(ctx, f.a, milk, utc(2025, 1, 1), utc(2025, 2, 1))
	require.NoError(t, err)
	assert.Len(t, lines, 1)
}

func TestSetAndSkipGrocerySize(t *testing.T) {
	f := setupTrends(t)
	ctx := context.Background()
	id := insertLegacyRow(t, f.db, &models.User{ID: f.a}, "Milk", "Milk x", 1, 2)

	require.NoError(t, f.db.SetGrocerySize(ctx, f.a, id, "l", 1.5))
	r := readUnitRow(t, f.db, id)
	assert.Equal(t, "known", r.status)
	assert.Equal(t, "l", r.unit.String)
	assert.Equal(t, 1.5, r.size.Float64)

	assert.ErrorIs(t, f.db.SetGrocerySize(ctx, f.a, id, "parsec", 1), models.ErrInvalidSize)
	assert.ErrorIs(t, f.db.SetGrocerySize(ctx, f.a, id, "l", 0), models.ErrInvalidSize)
	assert.ErrorIs(t, f.db.SetGrocerySize(ctx, f.a, id, "l", 1e12), models.ErrInvalidSize)

	id2 := insertLegacyRow(t, f.db, &models.User{ID: f.a}, "Milk", "Milk y", 1, 2)
	require.NoError(t, f.db.SkipGrocerySize(ctx, f.a, id2))
	assert.Equal(t, "skipped", readUnitRow(t, f.db, id2).status)

	// Another user's row, and an Other row, are untouchable.
	assert.ErrorIs(t, f.db.SetGrocerySize(ctx, f.b, id, "l", 2), sql.ErrNoRows)
	assert.ErrorIs(t, f.db.SkipGrocerySize(ctx, f.b, id), sql.ErrNoRows)
	assert.ErrorIs(t, f.db.SetGrocerySize(ctx, f.a, f.otherG, "l", 2), sql.ErrNoRows)
	assert.ErrorIs(t, f.db.SkipGrocerySize(ctx, f.a, f.otherG), sql.ErrNoRows)
	assert.Equal(t, "not_tracked", readUnitRow(t, f.db, f.otherG).status)
	assert.Equal(t, 1.5, readUnitRow(t, f.db, id).size.Float64)
}

func TestCountAndGetUnsizedGroceries(t *testing.T) {
	f := setupTrends(t)
	ctx := context.Background()
	n, err := f.db.CountUnsizedGroceries(ctx, f.a)
	require.NoError(t, err)
	assert.Equal(t, 1, n)
	rows, err := f.db.GetUnsizedGroceries(ctx, f.a)
	require.NoError(t, err)
	require.Len(t, rows, 1)
	assert.Equal(t, "unknown", rows[0].SizeStatus)
	assert.Equal(t, f.a, rows[0].UserID)

	id := insertLegacyRow(t, f.db, &models.User{ID: f.a}, "Bread", "Newer", 1, 1)
	rows, err = f.db.GetUnsizedGroceries(ctx, f.a)
	require.NoError(t, err)
	require.Len(t, rows, 2)
	assert.Equal(t, id, rows[0].ID, "newest first")
}

func TestGetTrackedCategories(t *testing.T) {
	f := setupTrends(t)
	cats, err := f.db.GetTrackedCategories(context.Background())
	require.NoError(t, err)
	require.NotEmpty(t, cats)
	for i, c := range cats {
		assert.NotEqual(t, "Other", c.Name)
		if i > 0 {
			assert.LessOrEqual(t, cats[i-1].Name, c.Name)
		}
	}
}

func TestTrendQueriesScopedToUser(t *testing.T) {
	f := setupTrends(t)
	ctx := context.Background()
	big := 500.0 // user B's rows are 1000x bigger; any leak shows up as a large number
	from, to := utc(2020, 1, 1), utc(2030, 1, 1)
	for _, s := range []models.Section{models.SectionHouse, models.SectionCar, models.SectionGroceries} {
		rows, err := f.db.GetMonthTypeTotals(ctx, f.a, s, from, to)
		require.NoError(t, err)
		for _, r := range rows {
			assert.Less(t, r.Amount, big, s)
		}
		years, err := f.db.GetYearTotals(ctx, f.a, s)
		require.NoError(t, err)
		for _, v := range years {
			assert.Less(t, v, big, s)
		}
	}
	d, err := f.db.GetGroceriesDiscountTotal(ctx, f.a, from, to)
	require.NoError(t, err)
	assert.Equal(t, 3.5, d)
	lines, err := f.db.GetGroceryLines(ctx, f.a, 0, from, to)
	require.NoError(t, err)
	for _, l := range lines {
		assert.Less(t, l.TotalPrice, big)
	}
	un, err := f.db.GetUnsizedGroceries(ctx, f.a)
	require.NoError(t, err)
	for _, e := range un {
		assert.Equal(t, f.a, e.UserID)
	}
	n, err := f.db.CountUnsizedGroceries(ctx, f.a)
	require.NoError(t, err)
	assert.Equal(t, 1, n)
}
