-- +goose Up
-- Logins move to the home server's auth service: people arrive with its
-- user id (X-User-Id) and name (X-User-Name), and a row is made for anyone
-- new. Expenser no longer keeps usernames or passwords for them, so both
-- become optional. Existing rows keep theirs (the move to the home server
-- copies them into auth, ids unchanged).
ALTER TABLE users ALTER COLUMN username DROP NOT NULL;
ALTER TABLE users ALTER COLUMN password_hash DROP NOT NULL;

-- +goose Down
-- Rows made from the home server have neither; give them placeholders that
-- can't be logged in with.
UPDATE users SET username = 'user-' || id WHERE username IS NULL;
UPDATE users SET password_hash = '!' WHERE password_hash IS NULL;
ALTER TABLE users ALTER COLUMN username SET NOT NULL;
ALTER TABLE users ALTER COLUMN password_hash SET NOT NULL;
