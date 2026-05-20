-- +goose Up
-- Insert specific categories
INSERT INTO grocery_categories (name) VALUES 
('Tomatoes'), ('Cucumbers'), ('Milk'), ('Pork'), ('Beef'), ('Chicken'), ('Eggs'), ('Cheese'), ('Bread'), ('Potatoes'), ('Apples'), ('Bananas')
ON CONFLICT (name) DO NOTHING;

-- Set any expenses pointing to the general categories to 'Other' before deleting them
UPDATE groceries_expenses 
SET category_id = (SELECT id FROM grocery_categories WHERE name = 'Other')
WHERE category_id IN (SELECT id FROM grocery_categories WHERE name IN ('Vegetables', 'Fruits', 'Dairy', 'Meat', 'Bakery', 'Beverages'));

-- Delete general categories
DELETE FROM grocery_categories WHERE name IN ('Vegetables', 'Fruits', 'Dairy', 'Meat', 'Bakery', 'Beverages');

-- +goose Down
-- Re-insert general categories
INSERT INTO grocery_categories (name) VALUES 
('Vegetables'), ('Fruits'), ('Dairy'), ('Meat'), ('Bakery'), ('Beverages')
ON CONFLICT (name) DO NOTHING;

-- Delete specific categories (after pointing any expenses back to 'Other')
UPDATE groceries_expenses 
SET category_id = (SELECT id FROM grocery_categories WHERE name = 'Other')
WHERE category_id IN (SELECT id FROM grocery_categories WHERE name IN ('Tomatoes', 'Cucumbers', 'Milk', 'Pork', 'Beef', 'Chicken', 'Eggs', 'Cheese', 'Bread', 'Potatoes', 'Apples', 'Bananas'));

DELETE FROM grocery_categories WHERE name IN ('Tomatoes', 'Cucumbers', 'Milk', 'Pork', 'Beef', 'Chicken', 'Eggs', 'Cheese', 'Bread', 'Potatoes', 'Apples', 'Bananas');
