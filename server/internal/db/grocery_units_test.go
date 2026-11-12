package database

import (
	"database/sql"
	"testing"
	"time"

	"expenser/internal/config"
	"expenser/internal/db/migrations"
	"expenser/internal/models"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func setupUnitsTestDB(t *testing.T) (*DB, *models.User) {
	t.Helper()
	cfg, err := config.LoadConfig()
	require.NoError(t, err)
	tdb := InitTestDB(cfg)
	t.Cleanup(func() { tdb.Close() })
	ResetTestDB(tdb)
	u := *TestUserRegisterModel
	require.NoError(t, tdb.CreateUser(&u))
	return tdb, &u
}

func categoryID(t *testing.T, tdb *DB, name string) int {
	t.Helper()
	var id int
	require.NoError(t, tdb.conn.QueryRow(`SELECT id FROM grocery_categories WHERE name = $1`, name).Scan(&id))
	return id
}

// insertLegacyRow inserts a row the way pre-migration data looks: status unknown, no unit.
func insertLegacyRow(t *testing.T, tdb *DB, u *models.User, category, product string, qty, total float64) int {
	t.Helper()
	var id int
	require.NoError(t, tdb.conn.QueryRow(`
		INSERT INTO groceries_expenses (user_id, category_id, product, quantity, unit_price, total_price, supermarket_name, purchase_date, size_status)
		VALUES ($1, $2, $3, $4::numeric, $5::numeric, $5::numeric * $4::numeric, 'Lidl', now(), 'unknown') RETURNING id`,
		u.ID, categoryID(t, tdb, category), product, qty, total).Scan(&id))
	return id
}

type unitRow struct {
	unit   sql.NullString
	size   sql.NullFloat64
	status string
	qty    float64
	total  float64
}

func readUnitRow(t *testing.T, tdb *DB, id int) unitRow {
	t.Helper()
	var r unitRow
	require.NoError(t, tdb.conn.QueryRow(
		`SELECT unit, unit_size::float8, size_status, quantity::numeric::float8, total_price::float8 FROM groceries_expenses WHERE id = $1`, id,
	).Scan(&r.unit, &r.size, &r.status, &r.qty, &r.total))
	return r
}

func seedBackfillRows(t *testing.T, tdb *DB, u *models.User) (milk, tomato, bread, soap int) {
	milk = insertLegacyRow(t, tdb, u, "Milk", "Fresh milk 1L", 2, 1.5)
	tomato = insertLegacyRow(t, tdb, u, "Tomatoes", "Tomatoes", 1.2, 4)
	bread = insertLegacyRow(t, tdb, u, "Bread", "White bread", 1, 2)
	soap = insertLegacyRow(t, tdb, u, "Other", "Dish soap", 1, 3.4)
	return
}

func assertBackfilled(t *testing.T, tdb *DB, milk, tomato, bread, soap int) {
	t.Helper()
	m := readUnitRow(t, tdb, milk)
	assert.Equal(t, "l", m.unit.String)
	assert.Equal(t, 1.0, m.size.Float64)
	assert.Equal(t, "known", m.status)
	assert.Equal(t, 2.0, m.qty)
	assert.Equal(t, 3.0, m.total)

	tm := readUnitRow(t, tdb, tomato)
	assert.Equal(t, "kg", tm.unit.String)
	assert.Equal(t, 1.0, tm.size.Float64)
	assert.Equal(t, "known", tm.status)
	assert.Equal(t, 1.2, tm.qty)
	assert.Equal(t, 4.8, tm.total)

	b := readUnitRow(t, tdb, bread)
	assert.False(t, b.unit.Valid)
	assert.False(t, b.size.Valid)
	assert.Equal(t, "unknown", b.status)

	s := readUnitRow(t, tdb, soap)
	assert.False(t, s.unit.Valid)
	assert.Equal(t, "not_tracked", s.status)
}

func TestBackfillGroceryUnits(t *testing.T) {
	tdb, u := setupUnitsTestDB(t)
	milk, tomato, bread, soap := seedBackfillRows(t, tdb, u)

	tx, err := tdb.conn.Begin()
	require.NoError(t, err)
	sized, err := migrations.BackfillGroceryUnits(tx)
	require.NoError(t, err)
	require.NoError(t, tx.Commit())

	assert.Equal(t, 2, sized)
	assertBackfilled(t, tdb, milk, tomato, bread, soap)
}

func TestBackfillIdempotent(t *testing.T) {
	tdb, u := setupUnitsTestDB(t)
	milk, tomato, bread, soap := seedBackfillRows(t, tdb, u)

	tx, err := tdb.conn.Begin()
	require.NoError(t, err)
	require.NoError(t, migrations.UpGroceryUnits(tx))
	require.NoError(t, migrations.UpGroceryUnits(tx))
	require.NoError(t, tx.Commit())

	assertBackfilled(t, tdb, milk, tomato, bread, soap)
}

func TestBackfillRollsBack(t *testing.T) {
	tdb, u := setupUnitsTestDB(t)
	milk, tomato, bread, soap := seedBackfillRows(t, tdb, u)

	tx, err := tdb.conn.Begin()
	require.NoError(t, err)
	_, err = migrations.BackfillGroceryUnits(tx)
	require.NoError(t, err)
	require.NoError(t, tx.Rollback())

	for _, id := range []int{milk, tomato, bread, soap} {
		r := readUnitRow(t, tdb, id)
		assert.Equal(t, "unknown", r.status)
		assert.False(t, r.unit.Valid)
		assert.False(t, r.size.Valid)
	}
}

func TestCreateGroceriesSizeStatus(t *testing.T) {
	tdb, u := setupUnitsTestDB(t)
	newExpense := func(category, unit string, size float64) *models.GroceriesExpense {
		return &models.GroceriesExpense{
			UserID: u.ID, CategoryID: categoryID(t, tdb, category), Product: "Thing", Quantity: 1,
			UnitPrice: 2, TotalPrice: 2, SupermarketName: "Lidl", PurchaseDate: time.Now(),
			Unit: unit, UnitSize: size,
		}
	}
	readBack := func(e *models.GroceriesExpense) *models.GroceriesExpense {
		got, err := tdb.GetGroceriesExpenseByID(e.ID, u.ID)
		require.NoError(t, err)
		return got
	}

	known := newExpense("Milk", "l", 1)
	require.NoError(t, tdb.CreateGroceriesExpense(known))
	got := readBack(known)
	assert.Equal(t, "l", got.Unit)
	assert.Equal(t, 1.0, got.UnitSize)
	assert.Equal(t, models.SizeKnown, got.SizeStatus)

	plain := newExpense("Milk", "", 0)
	require.NoError(t, tdb.CreateGroceriesExpense(plain))
	got = readBack(plain)
	assert.Equal(t, "", got.Unit)
	assert.Equal(t, 0.0, got.UnitSize)
	assert.Equal(t, models.SizeUnknown, got.SizeStatus)

	other := newExpense("Other", "", 0)
	require.NoError(t, tdb.CreateGroceriesExpense(other))
	assert.Equal(t, models.SizeNotTracked, readBack(other).SizeStatus)

	plain.Unit, plain.UnitSize = "kg", 0.5
	require.NoError(t, tdb.EditGroceriesExpense(plain, u.ID))
	got = readBack(plain)
	assert.Equal(t, "kg", got.Unit)
	assert.Equal(t, 0.5, got.UnitSize)
	assert.Equal(t, models.SizeKnown, got.SizeStatus)

	// list read path scans the new columns too
	list, err := tdb.GetGroceriesExpensesForMonth(time.Now().Month(), time.Now().Year(), u.ID)
	require.NoError(t, err)
	statuses := map[string]int{}
	for _, e := range *list {
		statuses[e.SizeStatus]++
	}
	assert.Equal(t, map[string]int{models.SizeKnown: 2, models.SizeNotTracked: 1}, statuses)
}
