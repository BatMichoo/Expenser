-- +goose Up

-- expected_amount: what the bill usually costs; used to pre-fill "mark as
-- paid" and shown in reminders. remember_last_amount updates it from each
-- payment. last_paid_on records when the user last paid. last_notified_on
-- limits overdue reminders to one per day.
ALTER TABLE recurring_payments
    ADD COLUMN expected_amount NUMERIC(10, 2) CHECK (expected_amount >= 0),
    ADD COLUMN remember_last_amount BOOLEAN NOT NULL DEFAULT FALSE,
    ADD COLUMN last_paid_on DATE,
    ADD COLUMN last_notified_on DATE;

-- +goose Down
ALTER TABLE recurring_payments
    DROP COLUMN IF EXISTS expected_amount,
    DROP COLUMN IF EXISTS remember_last_amount,
    DROP COLUMN IF EXISTS last_paid_on,
    DROP COLUMN IF EXISTS last_notified_on;
