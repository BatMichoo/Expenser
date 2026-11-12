-- +goose Up
-- Every expense list, month view and chart filters by owner and date.
CREATE INDEX IF NOT EXISTS idx_home_expenses_owner_date ON home_expenses (created_by, expense_date);
CREATE INDEX IF NOT EXISTS idx_car_expenses_owner_date ON car_expenses (created_by, expense_date);
CREATE INDEX IF NOT EXISTS idx_groceries_expenses_owner_date ON groceries_expenses (user_id, purchase_date);

-- +goose Down
DROP INDEX IF EXISTS idx_home_expenses_owner_date;
DROP INDEX IF EXISTS idx_car_expenses_owner_date;
DROP INDEX IF EXISTS idx_groceries_expenses_owner_date;
