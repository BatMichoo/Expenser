-- +goose Up

-- General categories (Vegetables, Meat, ...) group many different products,
-- so their price per unit is only a rough guide. Trends list them apart.
ALTER TABLE grocery_categories ADD COLUMN general BOOLEAN NOT NULL DEFAULT false;

UPDATE grocery_categories SET general = true WHERE name IN (
	'Vegetables', 'Fruits', 'Meat', 'Seafood', 'Dairy', 'Bakery',
	'Pantry', 'Frozen Food', 'Ready Meals', 'Snacks', 'Sweets',
	'Beverages', 'Alcohol'
);

-- +goose Down

ALTER TABLE grocery_categories DROP COLUMN general;
