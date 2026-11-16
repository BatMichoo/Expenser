-- +goose Up
-- People and their accounts are the home server's (its auth service);
-- Expenser only gets a user id with each request. What reminders need,
-- sent in the background without a request, moves onto each bill: who to
-- greet (owner_name), in which language (owner_lang), and where to send
-- them (notify_email, filled from the old account email where a bill had
-- none). Then the users table goes, with the foreign keys to it.
ALTER TABLE recurring_payments
    ADD COLUMN owner_name VARCHAR(60) NOT NULL DEFAULT '',
    ADD COLUMN owner_lang VARCHAR(10) NOT NULL DEFAULT 'en' CHECK (owner_lang IN ('en', 'bg'));

UPDATE recurring_payments rp SET
    owner_name = LEFT(COALESCE(NULLIF(u.display_name, ''), u.username, ''), 60),
    owner_lang = CASE WHEN u.preferred_language = 'bg' THEN 'bg' ELSE 'en' END,
    notify_email = COALESCE(NULLIF(rp.notify_email, ''), NULLIF(u.email, ''))
FROM users u
WHERE u.id = rp.user_id;

ALTER TABLE home_expenses DROP CONSTRAINT fk_home_expenses_created_by;
ALTER TABLE car_expenses DROP CONSTRAINT fk_car_expenses_created_by;
ALTER TABLE groceries_expenses DROP CONSTRAINT groceries_expenses_user_id_fkey;
ALTER TABLE recurring_payments DROP CONSTRAINT recurring_payments_user_id_fkey;

DROP TABLE users;

-- +goose Down
-- Accounts come back without usernames or passwords (they're the home
-- server's): one row per person who has data, named after their bills.
CREATE TABLE users (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    username VARCHAR(50) UNIQUE NOT NULL,
    password_hash VARCHAR(255) NOT NULL,
    created_at TIMESTAMP WITH TIME ZONE DEFAULT CURRENT_TIMESTAMP,
    updated_at TIMESTAMP WITH TIME ZONE DEFAULT CURRENT_TIMESTAMP,
    preferred_language VARCHAR(10) DEFAULT 'en',
    email VARCHAR(254),
    display_name VARCHAR(60),
    default_notify VARCHAR(10) NOT NULL DEFAULT 'email' CHECK (default_notify IN ('email', 'none')),
    default_remind_days_before INT NOT NULL DEFAULT 3 CHECK (default_remind_days_before >= 0),
    default_remind_days_after INT NOT NULL DEFAULT 0 CHECK (default_remind_days_after >= 0),
    default_repeat_every_days INT DEFAULT 3 CHECK (default_repeat_every_days IS NULL OR default_repeat_every_days >= 1),
    default_max_reminders INT CHECK (default_max_reminders IS NULL OR default_max_reminders >= 1),
    token_version INTEGER NOT NULL DEFAULT 0
);
CREATE INDEX IF NOT EXISTS idx_users_username ON users(username);

INSERT INTO users (id, username, password_hash)
SELECT id, 'user-' || LEFT(id::text, 8), '!'
FROM (
    SELECT created_by AS id FROM home_expenses
    UNION SELECT created_by FROM car_expenses
    UNION SELECT user_id FROM groceries_expenses
    UNION SELECT user_id FROM recurring_payments
) ids
WHERE id IS NOT NULL;

UPDATE users u SET display_name = NULLIF(rp.owner_name, ''), preferred_language = rp.owner_lang
FROM (SELECT DISTINCT ON (user_id) user_id, owner_name, owner_lang FROM recurring_payments ORDER BY user_id, updated_at DESC) rp
WHERE rp.user_id = u.id;

ALTER TABLE home_expenses ADD CONSTRAINT fk_home_expenses_created_by FOREIGN KEY (created_by) REFERENCES users(id);
ALTER TABLE car_expenses ADD CONSTRAINT fk_car_expenses_created_by FOREIGN KEY (created_by) REFERENCES users(id);
ALTER TABLE groceries_expenses ADD CONSTRAINT groceries_expenses_user_id_fkey FOREIGN KEY (user_id) REFERENCES users(id);
ALTER TABLE recurring_payments ADD CONSTRAINT recurring_payments_user_id_fkey FOREIGN KEY (user_id) REFERENCES users(id) ON DELETE CASCADE;

ALTER TABLE recurring_payments DROP COLUMN owner_name, DROP COLUMN owner_lang;
