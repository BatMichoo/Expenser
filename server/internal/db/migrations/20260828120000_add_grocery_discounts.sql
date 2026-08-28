-- +goose Up

ALTER TABLE groceries_expenses
ADD COLUMN discount_per_unit NUMERIC(10, 2) NOT NULL DEFAULT 0,
ADD COLUMN total_discount NUMERIC(10, 2) NOT NULL DEFAULT 0;

-- +goose Down

ALTER TABLE groceries_expenses
DROP COLUMN discount_per_unit,
DROP COLUMN total_discount;
