-- +goose Up
-- Add metadata JSONB column to home_expenses and car_expenses
ALTER TABLE home_expenses ADD COLUMN metadata JSONB;
ALTER TABLE car_expenses ADD COLUMN metadata JSONB;

-- +goose Down
-- Remove metadata JSONB column from home_expenses and car_expenses
ALTER TABLE home_expenses DROP COLUMN metadata;
ALTER TABLE car_expenses DROP COLUMN metadata;
