-- +goose Up
-- Logins carry the version they were issued for; raising it (on a password
-- change) logs out every other device.
ALTER TABLE users ADD COLUMN IF NOT EXISTS token_version INTEGER NOT NULL DEFAULT 0;

-- +goose Down
ALTER TABLE users DROP COLUMN IF EXISTS token_version;
