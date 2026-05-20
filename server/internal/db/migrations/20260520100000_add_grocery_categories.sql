-- +goose Up

CREATE TABLE grocery_categories (
    id SERIAL PRIMARY KEY,
    name TEXT NOT NULL UNIQUE
);

INSERT INTO grocery_categories (name) VALUES 
('Vegetables'), ('Fruits'), ('Dairy'), ('Meat'), ('Bakery'), ('Beverages'), ('Other');

-- Rename existing price column to total_price
ALTER TABLE groceries_expenses RENAME COLUMN price TO total_price;

-- Add new columns
ALTER TABLE groceries_expenses 
ADD COLUMN category_id INTEGER REFERENCES grocery_categories(id),
ADD COLUMN unit_price NUMERIC(10, 2) NOT NULL DEFAULT 0;

-- Set category for existing rows and set a default unit_price
UPDATE groceries_expenses SET category_id = (SELECT id FROM grocery_categories WHERE name = 'Other'), unit_price = total_price / NULLIF(quantity, 0);
ALTER TABLE groceries_expenses ALTER COLUMN category_id SET NOT NULL;

-- +goose Down

ALTER TABLE groceries_expenses DROP COLUMN category_id, DROP COLUMN unit_price;
ALTER TABLE groceries_expenses RENAME COLUMN total_price TO price;
DROP TABLE grocery_categories;
