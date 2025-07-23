-- +goose Up
INSERT INTO car_expense_types (name) VALUES
    ('Oil'),
    ('Tires');

-- +goose Down
DELETE FROM car_expense_types 
WHERE name = 'Oil' OR 'Tires';
