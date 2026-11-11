-- +goose Up

-- Reminder schedule per bill:
--   date_kind          pay_by (date is a deadline) or arrives (date is roughly
--                      when the bill shows up; never overdue, only waiting)
--   remind_days_after  arrives bills: first reminder N days after the date
--   repeat_every_days  NULL = remind once, 1 = daily, N = every N days until paid
--   max_reminders      NULL = no limit; counted per due date
--   reminders_sent     reminders sent for the current due date
--   snoozed_until      no reminders before this date; cleared when paid
ALTER TABLE recurring_payments
    ADD COLUMN date_kind VARCHAR(10) NOT NULL DEFAULT 'pay_by' CHECK (date_kind IN ('pay_by', 'arrives')),
    ADD COLUMN remind_days_after INTEGER NOT NULL DEFAULT 0 CHECK (remind_days_after >= 0),
    ADD COLUMN repeat_every_days INTEGER DEFAULT 3 CHECK (repeat_every_days >= 1),
    ADD COLUMN max_reminders INTEGER CHECK (max_reminders >= 1),
    ADD COLUMN reminders_sent INTEGER NOT NULL DEFAULT 0,
    ADD COLUMN snoozed_until DATE;

-- Existing bills got "every 3 days" above; from now on NULL means "once",
-- so new rows must say what they want.
ALTER TABLE recurring_payments ALTER COLUMN repeat_every_days DROP DEFAULT;

-- +goose Down
ALTER TABLE recurring_payments
    DROP COLUMN IF EXISTS date_kind,
    DROP COLUMN IF EXISTS remind_days_after,
    DROP COLUMN IF EXISTS repeat_every_days,
    DROP COLUMN IF EXISTS max_reminders,
    DROP COLUMN IF EXISTS reminders_sent,
    DROP COLUMN IF EXISTS snoozed_until;
