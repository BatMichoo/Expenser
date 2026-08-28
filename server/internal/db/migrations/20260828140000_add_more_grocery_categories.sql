-- +goose Up

INSERT INTO grocery_categories (name) VALUES
('Onions'), ('Garlic'), ('Peppers'), ('Carrots'), ('Fish'), ('Yogurt'), ('Butter'), ('Rice'), ('Pasta'), ('Oranges'), ('Grapes')
ON CONFLICT (name) DO NOTHING;

-- +goose Down

UPDATE groceries_expenses
SET category_id = (SELECT id FROM grocery_categories WHERE name = 'Other')
WHERE category_id IN (SELECT id FROM grocery_categories WHERE name IN ('Onions', 'Garlic', 'Peppers', 'Carrots', 'Fish', 'Yogurt', 'Butter', 'Rice', 'Pasta', 'Oranges', 'Grapes'));

DELETE FROM grocery_categories WHERE name IN ('Onions', 'Garlic', 'Peppers', 'Carrots', 'Fish', 'Yogurt', 'Butter', 'Rice', 'Pasta', 'Oranges', 'Grapes');
