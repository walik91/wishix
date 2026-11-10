-- +goose NO TRANSACTION
-- +goose Up
ALTER TABLE wishlists ADD COLUMN IF NOT EXISTS event_timezone STRING NOT NULL DEFAULT 'UTC';

-- +goose Down
ALTER TABLE wishlists DROP COLUMN IF EXISTS event_timezone;
