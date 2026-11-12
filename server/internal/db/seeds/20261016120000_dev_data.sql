-- +goose Up
-- Dev-only dummy data. Applied by InitDatabase only when SEED_DEV_DATA=true.
-- Login: dev / password
-- notes and metadata must never be NULL: the DB layer scans them into a
-- string and a json.RawMessage, and neither accepts NULL.
--
-- Two years of deliberately messy history, so the month view, charts and
-- month-end estimate see something like real life: bills drift a few days
-- and are sometimes paid late (into the next month), electricity and gas
-- follow the seasons, prices change once, the car has yearly costs and a
-- big one-off repair, and groceries are 1-3 trips a week of varying baskets.
-- Nothing is dated after today. setseed keeps every run identical.
-- A LATERAL subquery that calls random() must reference the row it is
-- generated for (the "+ x * 0" terms), or Postgres evaluates it only once
-- and every row gets the same values.

SELECT setseed(0.42);

INSERT INTO users (id, username, password_hash) VALUES
    ('00000000-0000-0000-0000-00000000de00', 'dev', '$2a$10$9ZOUn1Pz/P9bDkY8l/MwLOC49mFBicqYMC5DxXRUQT5j51jqcGv4G');

-- Monthly utility bills for the last 24 months. Each bill has a usual day,
-- lands up to `jitter` days either side of it, and about one in twelve is
-- paid 10-18 days late, which can push it into the following month.
INSERT INTO home_expenses (utility_type_id, amount, expense_date, notes, metadata, created_by)
SELECT ut.id, b.amount, paid.d, '', b.metadata, '00000000-0000-0000-0000-00000000de00'
FROM generate_series(0, 23) AS m,
LATERAL (SELECT date_trunc('month', CURRENT_DATE) - make_interval(months => m) AS start) AS mo,
LATERAL (SELECT EXTRACT(MONTH FROM mo.start) IN (11, 12, 1, 2, 3) AS winter,
                m < 12 AS new_prices) AS s,
LATERAL (SELECT round(180 + random() * 120 + CASE WHEN s.winter THEN 110 ELSE 0 END) AS kwh_day,
                round(90 + random() * 60 + CASE WHEN s.winter THEN 60 ELSE 0 END) AS kwh_night,
                CASE WHEN s.new_prices THEN 0.15 ELSE 0.14 END AS price_day,
                CASE WHEN s.new_prices THEN 0.09 ELSE 0.08 END AS price_night,
                6 + floor(random() * 5) AS water_m3,
                CASE WHEN s.new_prices THEN 1.95 ELSE 1.80 END AS water_price) AS u,
LATERAL (VALUES
    ('Electricity', 10, 3,
        ROUND((u.kwh_day * u.price_day + u.kwh_night * u.price_night)::numeric, 2),
        jsonb_build_object('kWh_day', u.kwh_day, 'kWh_night', u.kwh_night,
                           'price_day', u.price_day, 'price_night', u.price_night)),
    ('Water', 13, 3,
        ROUND((u.water_m3 * u.water_price)::numeric, 2),
        jsonb_build_object('quantity', u.water_m3, 'price_per_unit', u.water_price)),
    ('Internet', 11, 2,
        CASE WHEN m < 15 THEN 19.99 ELSE 17.99 END,
        jsonb_build_object('provider', 'Vivacom', 'plan', 'Fiber 500')),
    ('TV', 11, 2,
        9.99,
        jsonb_build_object('provider', 'Vivacom', 'plan', 'TV Basic'))
) AS b(utility, usual_day, jitter, amount, metadata),
LATERAL (SELECT mo.start + make_interval(days => b.usual_day - 1
                                         + floor(random() * (2 * b.jitter + 1))::int - b.jitter
                                         + CASE WHEN random() < 0.08 THEN 10 + floor(random() * 9)::int ELSE 0 END) AS d) AS paid,
     utility_types ut
WHERE ut.name = b.utility
  AND paid.d <= CURRENT_DATE;

-- Gas only in the heating season (November to March), most in deep winter.
INSERT INTO home_expenses (utility_type_id, amount, expense_date, notes, metadata, created_by)
SELECT (SELECT id FROM utility_types WHERE name = 'Gas'),
       ROUND((g.quantity * 0.65)::numeric, 2),
       g.d,
       '',
       jsonb_build_object('quantity', g.quantity, 'price_per_unit', 0.65),
       '00000000-0000-0000-0000-00000000de00'
FROM generate_series(0, 23) AS m,
LATERAL (SELECT date_trunc('month', CURRENT_DATE) - make_interval(months => m) AS start) AS mo,
LATERAL (SELECT round(CASE WHEN EXTRACT(MONTH FROM mo.start) IN (12, 1, 2) THEN 110 ELSE 60 END
                      + random() * 60) AS quantity,
                mo.start + make_interval(days => 17 + floor(random() * 9)::int) AS d) AS g
WHERE EXTRACT(MONTH FROM mo.start) IN (11, 12, 1, 2, 3)
  AND g.d <= CURRENT_DATE;

-- Waste fee twice a year, in April and October.
INSERT INTO home_expenses (utility_type_id, amount, expense_date, notes, metadata, created_by)
SELECT (SELECT id FROM utility_types WHERE name = 'Waste'),
       CASE WHEN m < 12 THEN 85.50 ELSE 79.00 END,
       w.d,
       'Municipal waste fee',
       '{}',
       '00000000-0000-0000-0000-00000000de00'
FROM generate_series(0, 23) AS m,
LATERAL (SELECT date_trunc('month', CURRENT_DATE) - make_interval(months => m) AS start) AS mo,
LATERAL (SELECT mo.start + make_interval(days => 14 + floor(random() * 12)::int) AS d) AS w
WHERE EXTRACT(MONTH FROM mo.start) IN (4, 10)
  AND w.d <= CURRENT_DATE;

-- Odd jobs around the house.
INSERT INTO home_expenses (utility_type_id, amount, expense_date, notes, metadata, created_by)
SELECT (SELECT id FROM utility_types WHERE name = 'Other'),
       o.amount,
       date_trunc('month', CURRENT_DATE) - make_interval(months => o.months_ago) + make_interval(days => o.day - 1),
       o.notes,
       '{}',
       '00000000-0000-0000-0000-00000000de00'
FROM (VALUES
    (60.00, 3, 16, 'Plumber, kitchen tap'),
    (145.00, 11, 5, 'Boiler check'),
    (38.00, 19, 22, 'Chimney sweep')
) AS o(amount, months_ago, day, notes);

-- Fuel: two to four fill-ups a month on random days.
INSERT INTO car_expenses (car_expense_type_id, amount, expense_date, notes, metadata, created_by)
SELECT (SELECT id FROM car_expense_types WHERE name = 'Fuel'),
       ROUND((f.liters * f.price)::numeric, 2),
       f.d,
       '',
       jsonb_build_object('liters', f.liters, 'price_per_liter', f.price),
       '00000000-0000-0000-0000-00000000de00'
FROM generate_series(0, 23) AS m,
LATERAL generate_series(1, 2 + floor(random() * 3)::int + m * 0) AS fill,
LATERAL (SELECT ROUND((30 + random() * 22 + fill * 0)::numeric, 2) AS liters,
                ROUND((1.28 + random() * 0.12 + CASE WHEN m < 12 THEN 0.06 ELSE 0 END)::numeric, 2) AS price,
                date_trunc('month', CURRENT_DATE) - make_interval(months => m)
                    + make_interval(days => floor(random() * 28)::int) AS d) AS f
WHERE f.d <= CURRENT_DATE;

-- Car wash most months; parking a few times a month.
INSERT INTO car_expenses (car_expense_type_id, amount, expense_date, notes, metadata, created_by)
SELECT ct.id, x.amount, x.d, '', x.metadata, '00000000-0000-0000-0000-00000000de00'
FROM generate_series(0, 23) AS m,
LATERAL generate_series(1, 4 + m * 0) AS slot,
LATERAL (SELECT CASE WHEN slot = 1 THEN 'Car Wash' ELSE 'Parking/Tolls' END AS type_name,
                CASE WHEN slot = 1 THEN 0.65 ELSE 0.45 END AS chance,
                CASE WHEN slot = 1 THEN ROUND((6 + random() * 6)::numeric, 2)
                     ELSE ROUND((2 + random() * 10)::numeric, 2) END AS amount,
                CASE WHEN slot = 1 THEN '{}'::jsonb
                     ELSE jsonb_build_object('location', 'Sofia center', 'duration', (1 + floor(random() * 4)::int) || ' hours') END AS metadata,
                random() AS roll,
                date_trunc('month', CURRENT_DATE) - make_interval(months => m)
                    + make_interval(days => floor(random() * 28)::int) AS d) AS x
JOIN car_expense_types ct ON ct.name = x.type_name
WHERE x.roll < x.chance
  AND x.d <= CURRENT_DATE;

-- Yearly and one-off car costs, including a big repair.
INSERT INTO car_expenses (car_expense_type_id, amount, discount_amount, expense_date, notes, metadata, created_by)
SELECT ct.id, e.amount, e.discount,
       date_trunc('month', CURRENT_DATE) - make_interval(months => e.months_ago) + make_interval(days => e.day - 1),
       COALESCE(e.notes, ''), COALESCE(e.metadata, '{}'), '00000000-0000-0000-0000-00000000de00'
FROM (VALUES
    ('Insurance', 310.00, 15.00, 10, 7, 'Annual civil liability',
        jsonb_build_object('provider', 'Bulstrad', 'coverage_type', 'Civil liability',
                           'valid_until', to_char(date_trunc('month', CURRENT_DATE) + INTERVAL '2 months', 'YYYY-MM-DD"T"00:00:00"Z"'))),
    ('Insurance', 289.00, 0.00, 22, 9, 'Annual civil liability',
        jsonb_build_object('provider', 'Bulstrad', 'coverage_type', 'Civil liability')),
    ('Parking/Tolls', 49.60, 0.00, 1, 3, 'Annual vignette',
        jsonb_build_object('location', 'Highway', 'duration', '1 year')),
    ('Parking/Tolls', 49.60, 0.00, 13, 2, 'Annual vignette',
        jsonb_build_object('location', 'Highway', 'duration', '1 year')),
    ('Maintenance/Repair', 245.00, 0.00, 9, 14, 'Yearly service',
        jsonb_build_object('parts', jsonb_build_array('Oil', 'Filters'), 'labor_cost', 60)),
    ('Maintenance/Repair', 229.00, 0.00, 21, 18, 'Yearly service',
        jsonb_build_object('parts', jsonb_build_array('Oil', 'Filters'), 'labor_cost', 55)),
    ('Maintenance/Repair', 180.00, 20.00, 4, 8, 'Front brake pads',
        jsonb_build_object('parts', jsonb_build_array('Brakes'), 'labor_cost', 45)),
    ('Maintenance/Repair', 860.00, 0.00, 15, 20, 'Clutch replacement',
        jsonb_build_object('parts', jsonb_build_array('Clutch kit', 'Flywheel'), 'labor_cost', 240)),
    ('Tires', 420.00, 30.00, 6, 11, 'Summer tires', NULL),
    ('Tires', 40.00, 0.00, 7, 4, 'Seasonal tire change', NULL),
    ('Tires', 40.00, 0.00, 12, 6, 'Seasonal tire change', NULL),
    ('Tires', 38.00, 0.00, 19, 9, 'Seasonal tire change', NULL),
    ('Oil', 38.50, 0.00, 2, 17, 'Top-up', NULL),
    ('Oil', 34.90, 0.00, 14, 12, 'Top-up', NULL),
    ('Other', 25.00, 0.00, 3, 23, NULL,
        jsonb_build_object('custom_name', 'Phone holder'))
) AS e(type_name, amount, discount, months_ago, day, notes, metadata)
JOIN car_expense_types ct ON ct.name = e.type_name;

-- Groceries: 1-3 trips a week for two years. Each trip picks a store and a
-- basket of items by chance, with the odd discount.
INSERT INTO groceries_expenses (user_id, category_id, product, quantity, unit_price, discount_per_unit, total_price, total_discount, supermarket_name, purchase_date)
SELECT '00000000-0000-0000-0000-00000000de00',
       gc.id, i.product, q.quantity, i.unit_price, q.discount,
       ROUND((q.quantity * i.unit_price)::numeric, 2),
       ROUND((q.quantity * q.discount)::numeric, 2),
       t.store,
       t.d
FROM generate_series(0, 103) AS w,
LATERAL generate_series(1, 1 + floor(random() * 3)::int + w * 0) AS trip,
LATERAL (SELECT (ARRAY['Lidl', 'Kaufland', 'Billa', 'Fantastico'])[1 + floor(random() * 4)::int + trip * 0] AS store,
                date_trunc('day', CURRENT_DATE) - make_interval(weeks => w, days => floor(random() * 7)::int)
                    + make_interval(hours => 9 + floor(random() * 11)::int, mins => floor(random() * 60)::int) AS d) AS t,
LATERAL (VALUES
    ('Milk', 'Fresh milk 1L', 2, 1.45, 0.7),
    ('Bread', 'White bread', 1, 1.20, 0.8),
    ('Eggs', 'Eggs 10 pcs', 1, 3.60, 0.4),
    ('Tomatoes', 'Tomatoes', 1.2, 3.20, 0.5),
    ('Cucumbers', 'Cucumbers', 1, 2.10, 0.4),
    ('Potatoes', 'Potatoes 2kg', 1, 2.80, 0.25),
    ('Chicken', 'Chicken breast', 0.8, 9.90, 0.4),
    ('Pork', 'Pork chops', 0.7, 8.50, 0.25),
    ('Cheese', 'White cheese', 0.4, 12.50, 0.35),
    ('Yogurt', 'Yogurt 400g', 2, 1.10, 0.5),
    ('Bananas', 'Bananas', 1, 2.40, 0.4),
    ('Apples', 'Apples', 1, 2.20, 0.35),
    ('Pasta', 'Spaghetti 500g', 1, 1.60, 0.2),
    ('Coffee', 'Ground coffee 250g', 1, 6.80, 0.2),
    ('Chocolate', 'Milk chocolate', 1, 2.30, 0.3),
    ('Beer', 'Beer 0.5L', 4, 1.30, 0.3),
    ('Wine', 'Red wine 0.75L', 1, 9.50, 0.1),
    ('Olive Oil', 'Olive oil 1L', 1, 11.90, 0.05),
    ('Other', 'Dish soap', 1, 3.40, 0.1)
) AS i(category, product, base_quantity, unit_price, chance)
CROSS JOIN LATERAL (SELECT random() + w * 0 + trip * 0 AS roll,
                           ROUND((i.base_quantity * (0.6 + random() * 0.9))::numeric, 2) AS quantity,
                           CASE WHEN random() < 0.15 THEN ROUND((i.unit_price * 0.2)::numeric, 2) ELSE 0 END AS discount) AS q
JOIN grocery_categories gc ON gc.name = i.category
WHERE q.roll < i.chance
  AND t.d <= now();

-- +goose Down

DELETE FROM groceries_expenses WHERE user_id = '00000000-0000-0000-0000-00000000de00';
DELETE FROM car_expenses WHERE created_by = '00000000-0000-0000-0000-00000000de00';
DELETE FROM home_expenses WHERE created_by = '00000000-0000-0000-0000-00000000de00';
DELETE FROM users WHERE id = '00000000-0000-0000-0000-00000000de00';
