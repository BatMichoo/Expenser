-- +goose Up

-- Recurring payments the user wants to be reminded about (e.g. Internet, TV,
-- car insurance). A payment belongs to exactly one category and references
-- the matching expense type table.
CREATE TABLE IF NOT EXISTS recurring_payments (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    user_id UUID NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    category VARCHAR(10) NOT NULL CHECK (category IN ('house', 'car')),
    utility_type_id INTEGER REFERENCES utility_types(id),
    car_expense_type_id INTEGER REFERENCES car_expense_types(id),
    name VARCHAR(100) NOT NULL DEFAULT '',
    -- start_date anchors the cycle so month-end due dates do not drift
    -- (Jan 31 -> Feb 28 -> Mar 31).
    start_date DATE NOT NULL,
    due_date DATE NOT NULL,
    cycle_unit VARCHAR(10) NOT NULL CHECK (cycle_unit IN ('day', 'week', 'month', 'year')),
    cycle_every INTEGER NOT NULL DEFAULT 1 CHECK (cycle_every >= 1),
    remind_days_before INTEGER NOT NULL DEFAULT 0 CHECK (remind_days_before >= 0),
    payment_hours TEXT NOT NULL DEFAULT '',
    payment_options TEXT[] NOT NULL DEFAULT '{cash}',
    notes TEXT NOT NULL DEFAULT '',
    last_notified_for DATE,
    active BOOLEAN NOT NULL DEFAULT TRUE,
    created_at TIMESTAMP WITH TIME ZONE NOT NULL DEFAULT CURRENT_TIMESTAMP,
    updated_at TIMESTAMP WITH TIME ZONE NOT NULL DEFAULT CURRENT_TIMESTAMP,
    CONSTRAINT recurring_payments_type_matches_category CHECK (
        (category = 'house' AND utility_type_id IS NOT NULL AND car_expense_type_id IS NULL)
        OR
        (category = 'car' AND car_expense_type_id IS NOT NULL AND utility_type_id IS NULL)
    )
);

CREATE INDEX IF NOT EXISTS idx_recurring_payments_user ON recurring_payments(user_id);
CREATE INDEX IF NOT EXISTS idx_recurring_payments_active_due ON recurring_payments(due_date) WHERE active;

-- +goose Down
DROP TABLE IF EXISTS recurring_payments;
