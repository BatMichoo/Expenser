package database

import (
	"context"
	"database/sql"
	"expenser/internal/models"
	"fmt"
	"time"

	"github.com/google/uuid"
	"github.com/lib/pq"
)

// recurringPaymentSelect selects every column needed by scanRecurringPayment,
// with the type name resolved from whichever type table the category uses.
const recurringPaymentSelect = `
	SELECT rp.id, rp.user_id, u.username, rp.category, rp.utility_type_id, rp.car_expense_type_id,
		COALESCE(ut.name, ct.name, ''), rp.name, rp.start_date, rp.due_date, rp.cycle_unit,
		rp.cycle_every, rp.remind_days_before, rp.payment_hours, rp.payment_options, rp.notes,
		rp.last_notified_for, rp.active, rp.created_at, rp.updated_at
	FROM recurring_payments rp
	JOIN users u ON u.id = rp.user_id
	LEFT JOIN utility_types ut ON ut.id = rp.utility_type_id
	LEFT JOIN car_expense_types ct ON ct.id = rp.car_expense_type_id
`

type rowScanner interface {
	Scan(dest ...any) error
}

func scanRecurringPayment(row rowScanner) (*models.RecurringPayment, error) {
	var (
		p               models.RecurringPayment
		utilityTypeID   sql.NullInt64
		carTypeID       sql.NullInt64
		lastNotifiedFor sql.NullTime
		options         pq.StringArray
	)
	err := row.Scan(
		&p.ID, &p.UserID, &p.Username, &p.Category, &utilityTypeID, &carTypeID,
		&p.TypeName, &p.Name, &p.StartDate, &p.DueDate, &p.CycleUnit,
		&p.CycleEvery, &p.RemindDaysBefore, &p.PaymentHours, &options, &p.Notes,
		&lastNotifiedFor, &p.Active, &p.CreatedAt, &p.UpdatedAt,
	)
	if err != nil {
		return nil, err
	}
	if utilityTypeID.Valid {
		v := int(utilityTypeID.Int64)
		p.UtilityTypeID = &v
	}
	if carTypeID.Valid {
		v := int(carTypeID.Int64)
		p.CarExpenseTypeID = &v
	}
	if lastNotifiedFor.Valid {
		v := models.DateOnly(lastNotifiedFor.Time)
		p.LastNotifiedFor = &v
	}
	p.StartDate = models.DateOnly(p.StartDate)
	p.DueDate = models.DateOnly(p.DueDate)
	p.PaymentOptions = []string(options)
	if p.PaymentOptions == nil {
		p.PaymentOptions = []string{}
	}
	return &p, nil
}

func queryRecurringPayments(ctx context.Context, conn *sql.DB, query string, args ...any) ([]models.RecurringPayment, error) {
	rows, err := conn.QueryContext(ctx, query, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	payments := []models.RecurringPayment{}
	for rows.Next() {
		p, err := scanRecurringPayment(rows)
		if err != nil {
			return nil, err
		}
		payments = append(payments, *p)
	}
	return payments, rows.Err()
}

// prepareRecurringPayment validates input and fills defaults shared by
// create and update.
func prepareRecurringPayment(p *models.RecurringPayment) error {
	if err := p.Validate(); err != nil {
		return err
	}
	p.DueDate = models.DateOnly(p.DueDate)
	if p.StartDate.IsZero() {
		p.StartDate = p.DueDate
	}
	p.StartDate = models.DateOnly(p.StartDate)
	return nil
}

// CreateRecurringPayment inserts a new recurring payment. Nil PaymentOptions
// get the defaults (cash); an empty non-nil slice is stored as empty.
// New payments are always active.
func (db *DB) CreateRecurringPayment(ctx context.Context, p *models.RecurringPayment) error {
	if err := prepareRecurringPayment(p); err != nil {
		return fmt.Errorf("invalid recurring payment: %w", err)
	}
	if p.PaymentOptions == nil {
		p.PaymentOptions = models.DefaultPaymentOptions()
	}

	query := `
		INSERT INTO recurring_payments (user_id, category, utility_type_id, car_expense_type_id, name,
			start_date, due_date, cycle_unit, cycle_every, remind_days_before, payment_hours,
			payment_options, notes)
		VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11, $12, $13)
		RETURNING id, active, created_at, updated_at`

	err := db.conn.QueryRowContext(ctx, query,
		p.UserID, p.Category, p.UtilityTypeID, p.CarExpenseTypeID, p.Name,
		p.StartDate, p.DueDate, p.CycleUnit, p.CycleEvery, p.RemindDaysBefore, p.PaymentHours,
		pq.Array(p.PaymentOptions), p.Notes,
	).Scan(&p.ID, &p.Active, &p.CreatedAt, &p.UpdatedAt)
	if err != nil {
		return fmt.Errorf("failed to create recurring payment: %w", err)
	}
	return nil
}

// GetRecurringPaymentByID returns sql.ErrNoRows (wrapped) when the payment
// does not exist or belongs to another user.
func (db *DB) GetRecurringPaymentByID(ctx context.Context, id, userID uuid.UUID) (*models.RecurringPayment, error) {
	row := db.conn.QueryRowContext(ctx, recurringPaymentSelect+` WHERE rp.id = $1 AND rp.user_id = $2`, id, userID)
	p, err := scanRecurringPayment(row)
	if err != nil {
		return nil, fmt.Errorf("failed to get recurring payment: %w", err)
	}
	return p, nil
}

// ListRecurringPayments returns all of a user's payments, soonest due first.
func (db *DB) ListRecurringPayments(ctx context.Context, userID uuid.UUID) ([]models.RecurringPayment, error) {
	payments, err := queryRecurringPayments(ctx, db.conn,
		recurringPaymentSelect+` WHERE rp.user_id = $1 ORDER BY rp.due_date, rp.created_at`, userID)
	if err != nil {
		return nil, fmt.Errorf("failed to list recurring payments: %w", err)
	}
	return payments, nil
}

// UpdateRecurringPayment overwrites the user-editable fields, including
// Active. Nil PaymentOptions are stored as empty. Returns sql.ErrNoRows
// (wrapped) when the payment does not exist or belongs to another user.
func (db *DB) UpdateRecurringPayment(ctx context.Context, p *models.RecurringPayment, userID uuid.UUID) error {
	if err := prepareRecurringPayment(p); err != nil {
		return fmt.Errorf("invalid recurring payment: %w", err)
	}
	if p.PaymentOptions == nil {
		p.PaymentOptions = []string{}
	}

	query := `
		UPDATE recurring_payments
		SET category = $1, utility_type_id = $2, car_expense_type_id = $3, name = $4,
			start_date = $5, due_date = $6, cycle_unit = $7, cycle_every = $8,
			remind_days_before = $9, payment_hours = $10, payment_options = $11, notes = $12,
			active = $13, updated_at = CURRENT_TIMESTAMP
		WHERE id = $14 AND user_id = $15
		RETURNING updated_at`

	err := db.conn.QueryRowContext(ctx, query,
		p.Category, p.UtilityTypeID, p.CarExpenseTypeID, p.Name,
		p.StartDate, p.DueDate, p.CycleUnit, p.CycleEvery,
		p.RemindDaysBefore, p.PaymentHours, pq.Array(p.PaymentOptions), p.Notes,
		p.Active, p.ID, userID,
	).Scan(&p.UpdatedAt)
	if err != nil {
		return fmt.Errorf("failed to update recurring payment: %w", err)
	}
	return nil
}

// DeleteRecurringPayment reports whether a payment was deleted.
func (db *DB) DeleteRecurringPayment(ctx context.Context, id, userID uuid.UUID) (bool, error) {
	res, err := db.conn.ExecContext(ctx, `DELETE FROM recurring_payments WHERE id = $1 AND user_id = $2`, id, userID)
	if err != nil {
		return false, fmt.Errorf("failed to delete recurring payment: %w", err)
	}
	n, err := res.RowsAffected()
	if err != nil {
		return false, fmt.Errorf("failed to delete recurring payment: %w", err)
	}
	return n > 0, nil
}

// GetOverdueRecurringPayments returns active payments whose due date is
// before today and therefore needs to move to the next cycle.
func (db *DB) GetOverdueRecurringPayments(ctx context.Context, today time.Time) ([]models.RecurringPayment, error) {
	payments, err := queryRecurringPayments(ctx, db.conn,
		recurringPaymentSelect+` WHERE rp.active AND rp.due_date < $1`, models.DateOnly(today))
	if err != nil {
		return nil, fmt.Errorf("failed to get overdue recurring payments: %w", err)
	}
	return payments, nil
}

// UpdateRecurringPaymentDueDate moves a payment to a new due date.
func (db *DB) UpdateRecurringPaymentDueDate(ctx context.Context, id uuid.UUID, dueDate time.Time) error {
	_, err := db.conn.ExecContext(ctx,
		`UPDATE recurring_payments SET due_date = $1, updated_at = CURRENT_TIMESTAMP WHERE id = $2`,
		models.DateOnly(dueDate), id)
	if err != nil {
		return fmt.Errorf("failed to update recurring payment due date: %w", err)
	}
	return nil
}

// GetRecurringPaymentsToRemind returns active payments that are due today or
// later, whose reminder window has opened, and that have not been notified
// for their current due date yet.
func (db *DB) GetRecurringPaymentsToRemind(ctx context.Context, today time.Time) ([]models.RecurringPayment, error) {
	payments, err := queryRecurringPayments(ctx, db.conn, recurringPaymentSelect+`
		WHERE rp.active
			AND rp.due_date >= $1
			AND rp.due_date - rp.remind_days_before <= $1
			AND rp.last_notified_for IS DISTINCT FROM rp.due_date
		ORDER BY rp.due_date`, models.DateOnly(today))
	if err != nil {
		return nil, fmt.Errorf("failed to get recurring payments to remind: %w", err)
	}
	return payments, nil
}

// MarkRecurringPaymentNotified records that a reminder was sent for dueDate.
func (db *DB) MarkRecurringPaymentNotified(ctx context.Context, id uuid.UUID, dueDate time.Time) error {
	_, err := db.conn.ExecContext(ctx,
		`UPDATE recurring_payments SET last_notified_for = $1 WHERE id = $2`,
		models.DateOnly(dueDate), id)
	if err != nil {
		return fmt.Errorf("failed to mark recurring payment notified: %w", err)
	}
	return nil
}
