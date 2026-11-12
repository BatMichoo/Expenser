-- +goose Up
-- Purchase times from receipts are wall-clock times. The app used to read
-- them as UTC (the container had no time zone), so 18:30 at the shop was
-- stored as 18:30Z. The app and database sessions now use Europe/Sofia;
-- reinterpret the stored wall-clock time in that zone so every purchase
-- keeps the day and time it is shown with today.
UPDATE groceries_expenses
SET purchase_date = (purchase_date AT TIME ZONE 'UTC') AT TIME ZONE 'Europe/Sofia';

-- +goose Down
UPDATE groceries_expenses
SET purchase_date = (purchase_date AT TIME ZONE 'Europe/Sofia') AT TIME ZONE 'UTC';
