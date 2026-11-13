package database

import (
	"context"
	"database/sql"
	"fmt"
	"time"

	"expenser/internal/models"

	"github.com/google/uuid"
)

// GetMonthTypeTotals returns net spend (amount minus discount) per month and
// per expense type (house/car) or grocery category, for dates in [from, to).
func (db *DB) GetMonthTypeTotals(ctx context.Context, userID uuid.UUID, s models.Section, from, to time.Time) ([]models.MonthTypeTotal, error) {
	query, err := monthTypeQuery(s)
	if err != nil {
		return nil, err
	}
	if s == models.SectionGroceries {
		from, to = db.localRange(from, to) // purchase times are instants
	}
	rows, err := db.conn.QueryContext(ctx, query, userID, from, to)
	if err != nil {
		return nil, fmt.Errorf("failed to query month totals: %w", err)
	}
	defer rows.Close()

	var out []models.MonthTypeTotal
	for rows.Next() {
		var r models.MonthTypeTotal
		var amount sql.NullFloat64
		if err := rows.Scan(&r.Month, &r.TypeID, &r.Type, &amount); err != nil {
			return nil, fmt.Errorf("failed to read month totals: %w", err)
		}
		r.Amount = amount.Float64
		out = append(out, r)
	}
	return out, rows.Err()
}

func monthTypeQuery(s models.Section) (string, error) {
	if s == models.SectionGroceries {
		return `
			SELECT to_char(e.purchase_date, 'YYYY-MM') AS month, c.id, c.name, SUM(e.total_price - e.total_discount)::float8
			FROM groceries_expenses e
			JOIN grocery_categories c ON c.id = e.category_id
			WHERE e.user_id = $1 AND e.purchase_date >= $2 AND e.purchase_date < $3
			GROUP BY month, c.id, c.name
			ORDER BY month, c.id`, nil
	}
	t, err := expenseTableFor(s)
	if err != nil {
		return "", err
	}
	return `
		SELECT to_char(e.` + t.DateColumn + `, 'YYYY-MM') AS month, ty.id, ty.name, SUM(e.amount - e.discount_amount)::float8
		FROM ` + t.Table + ` e
		JOIN ` + t.TypeTable + ` ty ON ty.id = e.` + t.TypeColumn + `
		WHERE e.` + t.OwnerColumn + ` = $1 AND e.` + t.DateColumn + ` >= $2 AND e.` + t.DateColumn + ` < $3
		GROUP BY month, ty.id, ty.name
		ORDER BY month, ty.id`, nil
}

// GetYearTotals returns the user's net spend per calendar year of a section.
func (db *DB) GetYearTotals(ctx context.Context, userID uuid.UUID, s models.Section) (map[int]float64, error) {
	var query string
	if s == models.SectionGroceries {
		query = `
			SELECT EXTRACT(YEAR FROM purchase_date)::int, SUM(total_price - total_discount)::float8
			FROM groceries_expenses WHERE user_id = $1 GROUP BY 1`
	} else {
		t, err := expenseTableFor(s)
		if err != nil {
			return nil, err
		}
		query = `
			SELECT EXTRACT(YEAR FROM ` + t.DateColumn + `)::int, SUM(amount - discount_amount)::float8
			FROM ` + t.Table + ` WHERE ` + t.OwnerColumn + ` = $1 GROUP BY 1`
	}
	rows, err := db.conn.QueryContext(ctx, query, userID)
	if err != nil {
		return nil, fmt.Errorf("failed to query year totals: %w", err)
	}
	defer rows.Close()

	totals := map[int]float64{}
	for rows.Next() {
		var year int
		var total sql.NullFloat64
		if err := rows.Scan(&year, &total); err != nil {
			return nil, fmt.Errorf("failed to read year totals: %w", err)
		}
		totals[year] = total.Float64
	}
	return totals, rows.Err()
}

// GetGroceriesDiscountTotal returns the total discount on the user's
// groceries bought in [from, to).
func (db *DB) GetGroceriesDiscountTotal(ctx context.Context, userID uuid.UUID, from, to time.Time) (float64, error) {
	from, to = db.localRange(from, to) // purchase times are instants
	var total sql.NullFloat64
	err := db.conn.QueryRowContext(ctx, `
		SELECT SUM(total_discount)::float8 FROM groceries_expenses
		WHERE user_id = $1 AND purchase_date >= $2 AND purchase_date < $3`,
		userID, from, to).Scan(&total)
	if err != nil {
		return 0, fmt.Errorf("failed to query groceries discount total: %w", err)
	}
	return total.Float64, nil
}

// GetGroceryLines returns the user's tracked-category grocery purchases in
// [from, to), oldest first, whatever their size status. categoryID 0 means
// every category.
func (db *DB) GetGroceryLines(ctx context.Context, userID uuid.UUID, categoryID int, from, to time.Time) ([]models.GroceryLine, error) {
	from, to = db.localRange(from, to) // purchase times are instants
	rows, err := db.conn.QueryContext(ctx, `
		SELECT e.id, e.category_id, c.name, e.product, e.supermarket_name, e.purchase_date,
			e.quantity::float8, e.unit_size::float8, e.total_price::float8, e.total_discount::float8, e.unit, e.size_status
		FROM groceries_expenses e
		JOIN grocery_categories c ON c.id = e.category_id
		WHERE e.user_id = $1 AND c.name <> 'Other'
			AND ($2 = 0 OR e.category_id = $2)
			AND e.purchase_date >= $3 AND e.purchase_date < $4
		ORDER BY e.purchase_date, e.id`, userID, categoryID, from, to)
	if err != nil {
		return nil, fmt.Errorf("failed to query grocery lines: %w", err)
	}
	defer rows.Close()

	var out []models.GroceryLine
	for rows.Next() {
		var l models.GroceryLine
		var size sql.NullFloat64
		var unit sql.NullString
		if err := rows.Scan(&l.ID, &l.CategoryID, &l.Category, &l.Product, &l.Store, &l.Date,
			&l.Quantity, &size, &l.TotalPrice, &l.TotalDiscount, &unit, &l.SizeStatus); err != nil {
			return nil, fmt.Errorf("failed to read grocery lines: %w", err)
		}
		l.UnitSize, l.Unit = size.Float64, unit.String
		out = append(out, l)
	}
	return out, rows.Err()
}

// GetUnsizedGroceries returns the user's purchases still waiting for a size,
// newest first.
func (db *DB) GetUnsizedGroceries(ctx context.Context, userID uuid.UUID) ([]models.GroceriesExpense, error) {
	out, err := queryAll(ctx, db.conn, scanGroceriesExpense, groceriesSelect+`
		WHERE e.user_id = $1 AND e.size_status = 'unknown'
		ORDER BY e.purchase_date DESC, e.id DESC`, userID)
	if err != nil {
		return nil, fmt.Errorf("failed to read unsized groceries: %w", err)
	}
	return out, nil
}

// CountUnsizedGroceries counts the user's purchases still waiting for a size.
func (db *DB) CountUnsizedGroceries(ctx context.Context, userID uuid.UUID) (int, error) {
	var n int
	err := db.conn.QueryRowContext(ctx,
		`SELECT COUNT(*) FROM groceries_expenses WHERE user_id = $1 AND size_status = 'unknown'`, userID).Scan(&n)
	if err != nil {
		return 0, fmt.Errorf("failed to count unsized groceries: %w", err)
	}
	return n, nil
}

// SetGrocerySize records a unit and size for one of the user's tracked
// purchases and marks it known. It returns models.ErrInvalidSize for a size
// that cannot be stored, and an error wrapping sql.ErrNoRows when the row is
// not the user's or belongs to the Other category.
func (db *DB) SetGrocerySize(ctx context.Context, userID uuid.UUID, id int, unit string, size float64) error {
	if !storableSize(unit, size) {
		return fmt.Errorf("size %v %q: %w", size, unit, models.ErrInvalidSize)
	}
	return db.updateSizeStatus(ctx, userID, id, `
		UPDATE groceries_expenses SET unit = $3, unit_size = $4, size_status = 'known'
		WHERE id = $1 AND user_id = $2 AND category_id <> (SELECT id FROM grocery_categories WHERE name = 'Other')`,
		unit, size)
}

// SkipGrocerySize marks one of the user's still-unsized purchases as not worth
// sizing. A row that already has a size (or was skipped) is left alone and
// yields sql.ErrNoRows, so a stale or racing request cannot wipe a real size.
func (db *DB) SkipGrocerySize(ctx context.Context, userID uuid.UUID, id int) error {
	return db.updateSizeStatus(ctx, userID, id, `
		UPDATE groceries_expenses SET size_status = 'skipped'
		WHERE id = $1 AND user_id = $2 AND size_status = 'unknown' AND category_id <> (SELECT id FROM grocery_categories WHERE name = 'Other')`)
}

func (db *DB) updateSizeStatus(ctx context.Context, userID uuid.UUID, id int, query string, extra ...any) error {
	res, err := db.conn.ExecContext(ctx, query, append([]any{id, userID}, extra...)...)
	if err != nil {
		return fmt.Errorf("failed to update grocery size: %w", err)
	}
	n, err := res.RowsAffected()
	if err != nil {
		return fmt.Errorf("failed to update grocery size: %w", err)
	}
	if n == 0 {
		return fmt.Errorf("grocery %d: %w", id, sql.ErrNoRows)
	}
	return nil
}

// GetTrackedCategories lists every grocery category except Other, by name.
func (db *DB) GetTrackedCategories(ctx context.Context) ([]models.Category, error) {
	rows, err := db.conn.QueryContext(ctx, `SELECT id, name FROM grocery_categories WHERE name <> 'Other' ORDER BY name`)
	if err != nil {
		return nil, fmt.Errorf("failed to query tracked categories: %w", err)
	}
	defer rows.Close()
	var out []models.Category
	for rows.Next() {
		var c models.Category
		if err := rows.Scan(&c.ID, &c.Name); err != nil {
			return nil, fmt.Errorf("failed to read tracked categories: %w", err)
		}
		out = append(out, c)
	}
	return out, rows.Err()
}
