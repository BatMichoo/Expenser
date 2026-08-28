-- +goose Up

ALTER TABLE car_expenses
ADD COLUMN discount_amount NUMERIC(10, 2) NOT NULL DEFAULT 0;

ALTER TABLE home_expenses
ADD COLUMN discount_amount NUMERIC(10, 2) NOT NULL DEFAULT 0;

-- +goose Down

ALTER TABLE car_expenses
DROP COLUMN discount_amount;

ALTER TABLE home_expenses
DROP COLUMN discount_amount;
