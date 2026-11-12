// Package migrations holds Go migrations that sit next to the SQL ones.
package migrations

import (
	"database/sql"
	"fmt"

	"expenser/internal/groceryunits"

	"github.com/pressly/goose"
)

func init() {
	goose.AddMigration(UpGroceryUnits, downGroceryUnits)
}

// UpGroceryUnits adds the unit columns and fills them for existing rows.
//
// It must stay idempotent: seedDevData runs goose.Up on the seeds directory,
// and goose v2 also collects every registered Go migration there, so on dev
// databases with seeding on this function runs a second time. Hence
// ADD COLUMN IF NOT EXISTS, and a backfill that only touches rows still
// marked 'unknown'. Exported for tests.
func UpGroceryUnits(tx *sql.Tx) error {
	_, err := tx.Exec(`
		ALTER TABLE groceries_expenses
			ADD COLUMN IF NOT EXISTS unit TEXT CHECK (unit IN ('kg','l','pc')),
			ADD COLUMN IF NOT EXISTS unit_size NUMERIC(10,4) CHECK (unit_size > 0),
			ADD COLUMN IF NOT EXISTS size_status TEXT NOT NULL DEFAULT 'unknown'
				CHECK (size_status IN ('unknown','known','skipped','not_tracked'));
	`)
	if err != nil {
		return fmt.Errorf("add grocery unit columns: %w", err)
	}
	if _, err := BackfillGroceryUnits(tx); err != nil {
		return err
	}
	return nil
}

func downGroceryUnits(tx *sql.Tx) error {
	_, err := tx.Exec(`
		ALTER TABLE groceries_expenses
			DROP COLUMN IF EXISTS unit,
			DROP COLUMN IF EXISTS unit_size,
			DROP COLUMN IF EXISTS size_status;
	`)
	return err
}

// BackfillGroceryUnits marks Other rows not_tracked and fills unit, unit_size
// and size_status for rows whose product name carries a pack size. It writes
// only those columns and returns how many rows it sized.
func BackfillGroceryUnits(tx *sql.Tx) (sized int, err error) {
	if _, err := tx.Exec(`
		UPDATE groceries_expenses SET size_status = 'not_tracked'
		WHERE size_status = 'unknown'
			AND category_id = (SELECT id FROM grocery_categories WHERE name = 'Other')`); err != nil {
		return 0, fmt.Errorf("mark untracked groceries: %w", err)
	}

	type target struct {
		id   int
		unit groceryunits.Unit
		per  float64
	}
	rows, err := tx.Query(`SELECT id, product, quantity::numeric::float8 FROM groceries_expenses WHERE size_status = 'unknown'`)
	if err != nil {
		return 0, fmt.Errorf("select groceries to backfill: %w", err)
	}
	var targets []target
	for rows.Next() {
		var id int
		var product string
		var quantity float64
		if err := rows.Scan(&id, &product, &quantity); err != nil {
			rows.Close()
			return 0, fmt.Errorf("scan grocery: %w", err)
		}
		if size, ok := groceryunits.Parse(product, quantity); ok {
			targets = append(targets, target{id, size.Unit, size.Per})
		}
	}
	if err := rows.Err(); err != nil {
		rows.Close()
		return 0, err
	}
	rows.Close()

	for _, t := range targets {
		if _, err := tx.Exec(`UPDATE groceries_expenses SET unit = $1, unit_size = $2, size_status = 'known' WHERE id = $3`,
			string(t.unit), t.per, t.id); err != nil {
			return 0, fmt.Errorf("backfill grocery %d: %w", t.id, err)
		}
	}
	return len(targets), nil
}
