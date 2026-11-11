-- +goose Up

-- Optional preferred email for reminders, and per-bill delivery settings:
--   notify        email | none
--   notify_email  optional address that overrides the user's email
ALTER TABLE users ADD COLUMN email VARCHAR(254);

ALTER TABLE recurring_payments
    ADD COLUMN notify VARCHAR(10) NOT NULL DEFAULT 'email' CHECK (notify IN ('email', 'none')),
    ADD COLUMN notify_email VARCHAR(254);

-- +goose Down
ALTER TABLE recurring_payments
    DROP COLUMN IF EXISTS notify,
    DROP COLUMN IF EXISTS notify_email;

ALTER TABLE users DROP COLUMN IF EXISTS email;
