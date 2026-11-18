-- +goose NO TRANSACTION
-- +goose Up
ALTER TABLE wishlist_wishes ADD COLUMN IF NOT EXISTS received BOOL NOT NULL DEFAULT false;

-- +goose Down
ALTER TABLE wishlist_wishes DROP COLUMN IF EXISTS received;
