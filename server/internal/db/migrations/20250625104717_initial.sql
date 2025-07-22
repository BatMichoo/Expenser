-- +goose Up

CREATE TABLE IF NOT EXISTS users (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    username VARCHAR(50) UNIQUE NOT NULL,
    password_hash VARCHAR(255) NOT NULL,
    created_at TIMESTAMP WITH TIME ZONE DEFAULT CURRENT_TIMESTAMP,
    updated_at TIMESTAMP WITH TIME ZONE DEFAULT CURRENT_TIMESTAMP
);

-- Create index on username for faster lookups
CREATE INDEX IF NOT EXISTS idx_users_username ON users(username);

-- 1. Create utility_types lookup table
CREATE TABLE IF NOT EXISTS utility_types (
    id SERIAL PRIMARY KEY,
    name VARCHAR(50) UNIQUE NOT NULL
);

-- 2. Insert initial data into utility_types
INSERT INTO utility_types (name) VALUES
    ('Electricity'),
    ('Water'),
    ('Gas'),
    ('Internet'),
    ('TV'),
    ('Waste'),
    ('Other');

-- 3. Create car_expense_types lookup table
CREATE TABLE IF NOT EXISTS car_expense_types (
    id SERIAL PRIMARY KEY,
    name VARCHAR(50) UNIQUE NOT NULL
);

-- 4. Insert initial data into car_expense_types
INSERT INTO car_expense_types (name) VALUES
    ('Fuel'),
    ('Maintenance/Repair'),
    ('Insurance'),
    ('Car Wash'),
    ('Parking/Tolls'),
    ('Other');

-- 5. Create home_expenses table with foreign key
CREATE TABLE IF NOT EXISTS home_expenses (
    id SERIAL PRIMARY KEY,
    utility_type_id INTEGER NOT NULL,
    amount NUMERIC(10, 2) NOT NULL,
    expense_date TIMESTAMP WITH TIME ZONE NOT NULL,
    notes TEXT,
    created_by UUID,
    created_at TIMESTAMP WITH TIME ZONE DEFAULT CURRENT_TIMESTAMP,

    CONSTRAINT fk_home_utility_type
        FOREIGN KEY (utility_type_id) REFERENCES utility_types(id),

    CONSTRAINT fk_home_expenses_created_by
    FOREIGN KEY (created_by) REFERENCES users(id)
);

-- 6. Create car_expenses table with foreign key
CREATE TABLE IF NOT EXISTS car_expenses (
    id SERIAL PRIMARY KEY,
    car_expense_type_id INTEGER NOT NULL,
    amount NUMERIC(10, 2) NOT NULL,
    expense_date TIMESTAMP WITH TIME ZONE NOT NULL,
    notes TEXT,
    created_by UUID,
    created_at TIMESTAMP WITH TIME ZONE DEFAULT CURRENT_TIMESTAMP,

    CONSTRAINT fk_car_expense_type
        FOREIGN KEY (car_expense_type_id) REFERENCES car_expense_types(id),

    CONSTRAINT fk_car_expenses_created_by
    FOREIGN KEY (created_by) REFERENCES users(id)
);

-- +goose Down

-- 1. Drop home_utilities_expenses table
DROP TABLE IF EXISTS home_expenses;

-- 2. Drop car_maintenance_expenses table
DROP TABLE IF EXISTS car_expenses;

-- 3. Drop car_expense_types table
DROP TABLE IF EXISTS car_expense_types;

-- 4. Drop utility_types table
DROP TABLE IF EXISTS utility_types;

DROP TABLE IF EXISTS users;
