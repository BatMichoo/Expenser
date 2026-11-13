-- +goose Up

-- General categories catch food that has no specific category (e.g. zucchini
-- goes to Vegetables), so Other is left for non-food items.
INSERT INTO grocery_categories (name) VALUES
('Vegetables'), ('Fruits'), ('Meat'), ('Seafood'), ('Dairy'), ('Bakery'),
('Pantry'), ('Frozen Food'), ('Ready Meals'), ('Snacks'), ('Sweets'),
('Beverages'), ('Alcohol')
ON CONFLICT (name) DO NOTHING;

-- +goose Down

UPDATE groceries_expenses
SET category_id = (SELECT id FROM grocery_categories WHERE name = 'Other'),
    size_status = 'not_tracked'
WHERE category_id IN (SELECT id FROM grocery_categories WHERE name IN (
	'Vegetables', 'Fruits', 'Meat', 'Seafood', 'Dairy', 'Bakery',
	'Pantry', 'Frozen Food', 'Ready Meals', 'Snacks', 'Sweets',
	'Beverages', 'Alcohol'
));

DELETE FROM grocery_categories WHERE name IN (
	'Vegetables', 'Fruits', 'Meat', 'Seafood', 'Dairy', 'Bakery',
	'Pantry', 'Frozen Food', 'Ready Meals', 'Snacks', 'Sweets',
	'Beverages', 'Alcohol'
);
