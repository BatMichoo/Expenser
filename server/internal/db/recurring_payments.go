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
		rp.expected_amount, rp.remember_last_amount, rp.last_paid_on, rp.last_notified_for,
		rp.last_notified_on, rp.active, rp.created_at, rp.updated_at,
		rp.date_kind, rp.remind_days_after, rp.repeat_every_days, rp.max_reminders,
		rp.reminders_sent, rp.snoozed_until,
		rp.notify, COALESCE(rp.notify_email, ''), COALESCE(u.email, ''), u.preferred_language,
		COALESCE(u.display_name, '')
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
		expectedAmount  sql.NullFloat64
		lastPaidOn      sql.NullTime
		lastNotifiedFor sql.NullTime
		lastNotifiedOn  sql.NullTime
		options         pq.StringArray
		repeatEvery     sql.NullInt64
		maxReminders    sql.NullInt64
		snoozedUntil    sql.NullTime
	)
	err := row.Scan(
		&p.ID, &p.UserID, &p.Username, &p.Category, &utilityTypeID, &carTypeID,
		&p.TypeName, &p.Name, &p.StartDate, &p.DueDate, &p.CycleUnit,
		&p.CycleEvery, &p.RemindDaysBefore, &p.PaymentHours, &options, &p.Notes,
		&expectedAmount, &p.RememberLastAmount, &lastPaidOn, &lastNotifiedFor,
		&lastNotifiedOn, &p.Active, &p.CreatedAt, &p.UpdatedAt,
		&p.DateKind, &p.RemindDaysAfter, &repeatEvery, &maxReminders,
		&p.RemindersSent, &snoozedUntil,
		&p.Notify, &p.NotifyEmail, &p.UserEmail, &p.UserLang,
		&p.UserDisplayName,
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
	if expectedAmount.Valid {
		v := expectedAmount.Float64
		p.ExpectedAmount = &v
	}
	p.LastPaidOn = nullDate(lastPaidOn)
	p.LastNotifiedFor = nullDate(lastNotifiedFor)
	p.LastNotifiedOn = nullDate(lastNotifiedOn)
	p.SnoozedUntil = nullDate(snoozedUntil)
	p.RepeatEveryDays = nullInt(repeatEvery)
	p.MaxReminders = nullInt(maxReminders)
	p.StartDate = models.DateOnly(p.StartDate)
	p.DueDate = models.DateOnly(p.DueDate)
	p.PaymentOptions = []string(options)
	if p.PaymentOptions == nil {
		p.PaymentOptions = []string{}
	}
	return &p, nil
}

func nullDate(t sql.NullTime) *time.Time {
	if !t.Valid {
		return nil
	}
	v := models.DateOnly(t.Time)
	return &v
}

func nullInt(v sql.NullInt64) *int {
	if !v.Valid {
		return nil
	}
	i := int(v.Int64)
	return &i
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
	p.Normalize()
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
			payment_options, notes, expected_amount, remember_last_amount,
			date_kind, remind_days_after, repeat_every_days, max_reminders, snoozed_until,
			notify, notify_email)
		VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11, $12, $13, $14, $15, $16, $17, $18, $19, $20,
			$21, NULLIF($22, ''))
		RETURNING id, active, created_at, updated_at`

	err := db.conn.QueryRowContext(ctx, query,
		p.UserID, p.Category, p.UtilityTypeID, p.CarExpenseTypeID, p.Name,
		p.StartDate, p.DueDate, p.CycleUnit, p.CycleEvery, p.RemindDaysBefore, p.PaymentHours,
		pq.Array(p.PaymentOptions), p.Notes, p.ExpectedAmount, p.RememberLastAmount,
		p.DateKind, p.RemindDaysAfter, p.RepeatEveryDays, p.MaxReminders, p.SnoozedUntil,
		p.Notify, p.NotifyEmail,
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
			active = $13, expected_amount = $14, remember_last_amount = $15,
			date_kind = $18, remind_days_after = $19, repeat_every_days = $20,
			max_reminders = $21, snoozed_until = $22,
			notify = $23, notify_email = NULLIF($24, ''),
			updated_at = CURRENT_TIMESTAMP
		WHERE id = $16 AND user_id = $17
		RETURNING updated_at`

	err := db.conn.QueryRowContext(ctx, query,
		p.Category, p.UtilityTypeID, p.CarExpenseTypeID, p.Name,
		p.StartDate, p.DueDate, p.CycleUnit, p.CycleEvery,
		p.RemindDaysBefore, p.PaymentHours, pq.Array(p.PaymentOptions), p.Notes,
		p.Active, p.ExpectedAmount, p.RememberLastAmount, p.ID, userID,
		p.DateKind, p.RemindDaysAfter, p.RepeatEveryDays, p.MaxReminders, p.SnoozedUntil,
		p.Notify, p.NotifyEmail,
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

// GetActiveRecurringPayments returns every user's active payments. The
// reminder scheduler decides which of them are due a reminder
// (RecurringPayment.ShouldRemind).
func (db *DB) GetActiveRecurringPayments(ctx context.Context) ([]models.RecurringPayment, error) {
	payments, err := queryRecurringPayments(ctx, db.conn, recurringPaymentSelect+`
		WHERE rp.active
		ORDER BY rp.due_date`)
	if err != nil {
		return nil, fmt.Errorf("failed to get active recurring payments: %w", err)
	}
	return payments, nil
}

// MarkRecurringPaymentNotified records that a reminder for dueDate was sent
// on sentOn.
func (db *DB) MarkRecurringPaymentNotified(ctx context.Context, id uuid.UUID, dueDate, sentOn time.Time) error {
	_, err := db.conn.ExecContext(ctx,
		`UPDATE recurring_payments
		SET reminders_sent = CASE WHEN last_notified_for = $1 THEN reminders_sent + 1 ELSE 1 END,
			last_notified_for = $1, last_notified_on = $2
		WHERE id = $3`,
		models.DateOnly(dueDate), models.DateOnly(sentOn), id)
	if err != nil {
		return fmt.Errorf("failed to mark recurring payment notified: %w", err)
	}
	return nil
}

// GetUpcomingRecurringPayments returns a user's active payments due on or
// before until, overdue ones included, soonest first. Payments only move to
// their next due date when paid, so every result is unpaid.
func (db *DB) GetUpcomingRecurringPayments(ctx context.Context, userID uuid.UUID, until time.Time) ([]models.RecurringPayment, error) {
	payments, err := queryRecurringPayments(ctx, db.conn, recurringPaymentSelect+`
		WHERE rp.user_id = $1 AND rp.active AND rp.due_date <= $2
		ORDER BY rp.due_date, rp.created_at`, userID, models.DateOnly(until))
	if err != nil {
		return nil, fmt.Errorf("failed to get upcoming recurring payments: %w", err)
	}
	return payments, nil
}

// MarkRecurringPaymentPaid records one payment atomically: it creates the
// matching house or car expense, sets last_paid_on, updates expected_amount
// when remember_last_amount is on, and moves the due date to the next cycle.
// Returns sql.ErrNoRows (wrapped) when the payment does not exist or
// belongs to another user.
func (db *DB) MarkRecurringPaymentPaid(ctx context.Context, userID, id uuid.UUID, amount float64, paidOn time.Time) (*models.PaidRecurringPayment, error) {
	tx, err := db.conn.BeginTx(ctx, nil)
	if err != nil {
		return nil, fmt.Errorf("failed to mark recurring payment paid: %w", err)
	}
	defer tx.Rollback()

	p, err := scanRecurringPayment(tx.QueryRowContext(ctx,
		recurringPaymentSelect+` WHERE rp.id = $1 AND rp.user_id = $2 FOR UPDATE OF rp`, id, userID))
	if err != nil {
		return nil, fmt.Errorf("failed to mark recurring payment paid: %w", err)
	}

	notes := p.Name
	if notes == "" {
		notes = "Recurring payment"
	}

	var expenseQuery string
	var typeID *int
	switch p.Category {
	case models.RecurringCategoryHouse:
		expenseQuery = `INSERT INTO home_expenses (utility_type_id, amount, discount_amount, expense_date, notes, metadata, created_by)
			VALUES ($1, $2, 0, $3, $4, '{}', $5) RETURNING id`
		typeID = p.UtilityTypeID
	case models.RecurringCategoryCar:
		expenseQuery = `INSERT INTO car_expenses (car_expense_type_id, amount, discount_amount, expense_date, notes, metadata, created_by)
			VALUES ($1, $2, 0, $3, $4, '{}', $5) RETURNING id`
		typeID = p.CarExpenseTypeID
	default:
		return nil, fmt.Errorf("failed to mark recurring payment paid: %w", models.ErrInvalidCategory)
	}

	result := &models.PaidRecurringPayment{PaidDue: p.DueDate}
	if err := tx.QueryRowContext(ctx, expenseQuery, typeID, amount, paidOn, notes, userID).Scan(&result.ExpenseID); err != nil {
		return nil, fmt.Errorf("failed to create expense for recurring payment: %w", err)
	}

	nextDue, err := p.NextDueAfter(p.DueDate)
	if err != nil {
		return nil, fmt.Errorf("failed to compute next due date: %w", err)
	}
	paidDate := models.DateOnly(paidOn)
	p.DueDate = nextDue
	p.LastPaidOn = &paidDate
	if p.RememberLastAmount {
		p.ExpectedAmount = &amount
	}
	p.SnoozedUntil = nil // a snooze only covers the cycle it was set in

	err = tx.QueryRowContext(ctx, `
		UPDATE recurring_payments
		SET due_date = $1, last_paid_on = $2, expected_amount = $3, snoozed_until = NULL,
			updated_at = CURRENT_TIMESTAMP
		WHERE id = $4
		RETURNING updated_at`,
		p.DueDate, paidDate, p.ExpectedAmount, p.ID,
	).Scan(&p.UpdatedAt)
	if err != nil {
		return nil, fmt.Errorf("failed to advance recurring payment: %w", err)
	}

	if err := tx.Commit(); err != nil {
		return nil, fmt.Errorf("failed to mark recurring payment paid: %w", err)
	}
	result.Payment = p
	return result, nil
}

// GetBillTypeTotal sums the user's expenses of one type (a house utility
// type or a car expense type, by the bill's category) dated in [from, to).
// found is false when there are none.
func (db *DB) GetBillTypeTotal(ctx context.Context, userID uuid.UUID, category models.RecurringPaymentCategory, typeID int, from, to time.Time) (total float64, found bool, err error) {
	query := `SELECT COALESCE(SUM(amount - discount_amount), 0), COUNT(*) FROM home_expenses
		WHERE created_by = $1 AND utility_type_id = $2 AND expense_date >= $3 AND expense_date < $4`
	if category == models.RecurringCategoryCar {
		query = `SELECT COALESCE(SUM(amount - discount_amount), 0), COUNT(*) FROM car_expenses
		WHERE created_by = $1 AND car_expense_type_id = $2 AND expense_date >= $3 AND expense_date < $4`
	}
	var count int
	if err := db.conn.QueryRowContext(ctx, query, userID, typeID, from, to).Scan(&total, &count); err != nil {
		return 0, false, fmt.Errorf("failed to get bill type total: %w", err)
	}
	return total, count > 0, nil
}
