-- +goose Up
-- +goose StatementBegin
ALTER TABLE users ADD COLUMN preferred_language VARCHAR(10) DEFAULT 'en';
-- +goose StatementEnd

-- +goose Down
-- +goose StatementBegin
ALTER TABLE users DROP COLUMN preferred_language;
-- +goose StatementEnd
