-- +goose Up

-- Optional display name for emails, and the user's defaults for the
-- reminder settings of new bills. Existing users get the values new bills
-- used before this (email, 3 days before, on the day, every 3 days, no
-- limit). NULL repeat = remind once; NULL max = no limit.
ALTER TABLE users
    ADD COLUMN display_name VARCHAR(60),
    ADD COLUMN default_notify VARCHAR(10) NOT NULL DEFAULT 'email' CHECK (default_notify IN ('email', 'none')),
    ADD COLUMN default_remind_days_before INT NOT NULL DEFAULT 3 CHECK (default_remind_days_before >= 0),
    ADD COLUMN default_remind_days_after INT NOT NULL DEFAULT 0 CHECK (default_remind_days_after >= 0),
    ADD COLUMN default_repeat_every_days INT DEFAULT 3 CHECK (default_repeat_every_days IS NULL OR default_repeat_every_days >= 1),
    ADD COLUMN default_max_reminders INT CHECK (default_max_reminders IS NULL OR default_max_reminders >= 1);

-- +goose Down
ALTER TABLE users
    DROP COLUMN IF EXISTS display_name,
    DROP COLUMN IF EXISTS default_notify,
    DROP COLUMN IF EXISTS default_remind_days_before,
    DROP COLUMN IF EXISTS default_remind_days_after,
    DROP COLUMN IF EXISTS default_repeat_every_days,
    DROP COLUMN IF EXISTS default_max_reminders;
