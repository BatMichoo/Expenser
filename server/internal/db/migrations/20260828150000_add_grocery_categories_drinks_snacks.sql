-- +goose Up

INSERT INTO grocery_categories (name) VALUES
('Lettuce'), ('Spinach'), ('Broccoli'), ('Mushrooms'), ('Lemons'), ('Avocados'),
('Sausages'), ('Bacon'), ('Ham'),
('Flour'), ('Sugar'), ('Cereal'), ('Beans'), ('Olive Oil'), ('Honey'), ('Ketchup'),
('Chips'), ('Nuts'), ('Chocolate'), ('Candy'), ('Cookies'), ('Ice Cream'),
('Water'), ('Juice'), ('Soda'), ('Coffee'), ('Tea'), ('Beer'), ('Wine')
ON CONFLICT (name) DO NOTHING;

-- +goose Down

UPDATE groceries_expenses
SET category_id = (SELECT id FROM grocery_categories WHERE name = 'Other')
WHERE category_id IN (SELECT id FROM grocery_categories WHERE name IN (
	'Lettuce', 'Spinach', 'Broccoli', 'Mushrooms', 'Lemons', 'Avocados',
	'Sausages', 'Bacon', 'Ham',
	'Flour', 'Sugar', 'Cereal', 'Beans', 'Olive Oil', 'Honey', 'Ketchup',
	'Chips', 'Nuts', 'Chocolate', 'Candy', 'Cookies', 'Ice Cream',
	'Water', 'Juice', 'Soda', 'Coffee', 'Tea', 'Beer', 'Wine'
));

DELETE FROM grocery_categories WHERE name IN (
	'Lettuce', 'Spinach', 'Broccoli', 'Mushrooms', 'Lemons', 'Avocados',
	'Sausages', 'Bacon', 'Ham',
	'Flour', 'Sugar', 'Cereal', 'Beans', 'Olive Oil', 'Honey', 'Ketchup',
	'Chips', 'Nuts', 'Chocolate', 'Candy', 'Cookies', 'Ice Cream',
	'Water', 'Juice', 'Soda', 'Coffee', 'Tea', 'Beer', 'Wine'
);
