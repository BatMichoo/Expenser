-- +goose Up
-- Dev-only dummy data. Applied by InitDatabase only when SEED_DEV_DATA=true.
-- Login: dev / password

INSERT INTO users (id, username, password_hash) VALUES
    ('00000000-0000-0000-0000-00000000de00', 'dev', '$2a$10$9ZOUn1Pz/P9bDkY8l/MwLOC49mFBicqYMC5DxXRUQT5j51jqcGv4G');

-- Monthly utility bills for the last 12 months.
INSERT INTO home_expenses (utility_type_id, amount, expense_date, notes, metadata, created_by)
SELECT ut.id, b.amount, b.expense_date, NULL, b.metadata, '00000000-0000-0000-0000-00000000de00'
FROM generate_series(0, 11) AS m,
LATERAL (SELECT date_trunc('month', CURRENT_DATE) - make_interval(months => m) + INTERVAL '9 days' AS d) AS month,
LATERAL (VALUES
    ('Electricity',
        ROUND(((180 + random() * 120) * 0.14 + (90 + random() * 60) * 0.08)::numeric, 2),
        month.d,
        jsonb_build_object('kWh_day', 180 + m * 7, 'kWh_night', 90 + m * 4, 'price_day', 0.14, 'price_night', 0.08)),
    ('Water',
        ROUND(((6 + random() * 4) * 1.95)::numeric, 2),
        month.d + INTERVAL '3 days',
        jsonb_build_object('quantity', 6 + m % 4, 'price_per_unit', 1.95)),
    ('Internet',
        19.99,
        month.d + INTERVAL '1 day',
        jsonb_build_object('provider', 'Vivacom', 'plan', 'Fiber 500')),
    ('TV',
        9.99,
        month.d + INTERVAL '1 day',
        jsonb_build_object('provider', 'Vivacom', 'plan', 'TV Basic'))
) AS b(utility, amount, expense_date, metadata)
JOIN utility_types ut ON ut.name = b.utility;

-- Gas only in the heating season.
INSERT INTO home_expenses (utility_type_id, amount, expense_date, metadata, created_by)
SELECT (SELECT id FROM utility_types WHERE name = 'Gas'),
       ROUND(((60 + random() * 80) * 0.65)::numeric, 2),
       d + INTERVAL '12 days',
       jsonb_build_object('quantity', 60 + m * 5, 'price_per_unit', 0.65),
       '00000000-0000-0000-0000-00000000de00'
FROM generate_series(0, 11) AS m,
LATERAL (SELECT date_trunc('month', CURRENT_DATE) - make_interval(months => m) AS d) AS month
WHERE EXTRACT(MONTH FROM d) IN (10, 11, 12, 1, 2, 3);

-- Waste fee twice a year.
INSERT INTO home_expenses (utility_type_id, amount, expense_date, notes, created_by)
SELECT (SELECT id FROM utility_types WHERE name = 'Waste'),
       85.50,
       date_trunc('month', CURRENT_DATE) - make_interval(months => m) + INTERVAL '20 days',
       'Municipal waste fee',
       '00000000-0000-0000-0000-00000000de00'
FROM unnest(ARRAY[2, 8]) AS m;

-- Two fuel fill-ups per month.
INSERT INTO car_expenses (car_expense_type_id, amount, expense_date, metadata, created_by)
SELECT (SELECT id FROM car_expense_types WHERE name = 'Fuel'),
       ROUND((liters * price)::numeric, 2),
       date_trunc('month', CURRENT_DATE) - make_interval(months => m) + make_interval(days => day),
       jsonb_build_object('liters', liters, 'price_per_liter', price),
       '00000000-0000-0000-0000-00000000de00'
FROM generate_series(0, 11) AS m,
     unnest(ARRAY[4, 18]) AS day,
LATERAL (SELECT ROUND((38 + random() * 14)::numeric, 2) AS liters,
                ROUND((1.30 + random() * 0.15)::numeric, 2) AS price) AS f;

-- Monthly car wash.
INSERT INTO car_expenses (car_expense_type_id, amount, expense_date, created_by)
SELECT (SELECT id FROM car_expense_types WHERE name = 'Car Wash'),
       ROUND((6 + random() * 6)::numeric, 2),
       date_trunc('month', CURRENT_DATE) - make_interval(months => m) + INTERVAL '25 days',
       '00000000-0000-0000-0000-00000000de00'
FROM generate_series(0, 11) AS m;

-- One-off car expenses.
INSERT INTO car_expenses (car_expense_type_id, amount, discount_amount, expense_date, notes, metadata, created_by)
SELECT ct.id, e.amount, e.discount, date_trunc('month', CURRENT_DATE) - make_interval(months => e.months_ago) + INTERVAL '7 days',
       e.notes, e.metadata, '00000000-0000-0000-0000-00000000de00'
FROM (VALUES
    ('Insurance', 310.00, 15.00, 10, 'Annual civil liability',
        jsonb_build_object('provider', 'Bulstrad', 'coverage_type', 'Civil liability',
                           'valid_until', to_char(date_trunc('month', CURRENT_DATE) + INTERVAL '2 months', 'YYYY-MM-DD"T"00:00:00"Z"'))),
    ('Maintenance/Repair', 245.00, 0.00, 9, 'Yearly service',
        jsonb_build_object('parts', jsonb_build_array('Oil', 'Filters'), 'labor_cost', 60)),
    ('Maintenance/Repair', 180.00, 20.00, 4, 'Front brake pads',
        jsonb_build_object('parts', jsonb_build_array('Brakes'), 'labor_cost', 45)),
    ('Tires', 420.00, 30.00, 6, 'Summer tires', NULL),
    ('Oil', 38.50, 0.00, 2, 'Top-up', NULL),
    ('Parking/Tolls', 15.00, 0.00, 1, NULL,
        jsonb_build_object('location', 'Sofia center', 'duration', '3 hours')),
    ('Parking/Tolls', 8.70, 0.00, 5, 'Vignette',
        jsonb_build_object('location', 'Highway', 'duration', '1 week')),
    ('Other', 25.00, 0.00, 3, NULL,
        jsonb_build_object('custom_name', 'Phone holder'))
) AS e(type_name, amount, discount, months_ago, notes, metadata)
JOIN car_expense_types ct ON ct.name = e.type_name;

-- Weekly grocery trips for the last 12 weeks.
INSERT INTO groceries_expenses (user_id, category_id, product, quantity, unit_price, discount_per_unit, total_price, total_discount, supermarket_name, purchase_date)
SELECT '00000000-0000-0000-0000-00000000de00',
       gc.id, i.product, i.quantity, i.unit_price, i.discount,
       ROUND((i.quantity * i.unit_price)::numeric, 2),
       ROUND((i.quantity * i.discount)::numeric, 2),
       (ARRAY['Lidl', 'Kaufland', 'Billa', 'Fantastico'])[1 + w % 4],
       date_trunc('day', CURRENT_DATE) - make_interval(weeks => w) + INTERVAL '18 hours'
FROM generate_series(0, 11) AS w,
LATERAL (VALUES
    ('Milk', 'Fresh milk 1L', 2, 1.45, 0.00),
    ('Bread', 'White bread', 1, 1.20, 0.00),
    ('Eggs', 'Eggs 10 pcs', 1, 3.60, CASE WHEN w % 3 = 0 THEN 0.50 ELSE 0 END),
    ('Tomatoes', 'Tomatoes', 1.2, 3.20, 0.00),
    ('Chicken', 'Chicken breast', 0.8, 9.90, CASE WHEN w % 2 = 0 THEN 1.50 ELSE 0 END),
    ('Cheese', 'White cheese', 0.4, 12.50, 0.00),
    ('Bananas', 'Bananas', 1, 2.40, 0.00),
    ('Coffee', 'Ground coffee 250g', CASE WHEN w % 2 = 0 THEN 1 ELSE 0 END, 6.80, 0.80),
    ('Beer', 'Beer 0.5L', CASE WHEN w % 2 = 1 THEN 4 ELSE 0 END, 1.30, 0.00)
) AS i(category, product, quantity, unit_price, discount)
JOIN grocery_categories gc ON gc.name = i.category
WHERE i.quantity > 0;

-- +goose Down

DELETE FROM groceries_expenses WHERE user_id = '00000000-0000-0000-0000-00000000de00';
DELETE FROM car_expenses WHERE created_by = '00000000-0000-0000-0000-00000000de00';
DELETE FROM home_expenses WHERE created_by = '00000000-0000-0000-0000-00000000de00';
DELETE FROM users WHERE id = '00000000-0000-0000-0000-00000000de00';
