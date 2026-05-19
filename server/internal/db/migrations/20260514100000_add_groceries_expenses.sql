-- +goose Up

CREATE TABLE groceries_expenses (
    id SERIAL PRIMARY KEY,
    user_id UUID NOT NULL,
    product TEXT NOT NULL,
    quantity REAL NOT NULL,
    price NUMERIC(10, 2) NOT NULL,
    supermarket_name TEXT,
    purchase_date TIMESTAMPTZ NOT NULL,
    created_at TIMESTAMPTZ DEFAULT CURRENT_TIMESTAMP,
    FOREIGN KEY(user_id) REFERENCES users(id)
);

-- +goose Down

DROP TABLE IF EXISTS groceries_expenses;

