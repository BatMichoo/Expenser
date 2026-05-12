-- +goose Up
-- Convert amounts from BGN to EUR using RoE 0.51130
UPDATE car_expenses SET amount = ROUND(amount * 0.51130, 2);
UPDATE home_expenses SET amount = ROUND(amount * 0.51130, 2);

-- +goose Down
-- Convert amounts back from EUR to BGN using RoE 0.51130
UPDATE car_expenses SET amount = ROUND(amount / 0.51130, 2);
UPDATE home_expenses SET amount = ROUND(amount / 0.51130, 2);
