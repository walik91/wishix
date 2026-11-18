-- +goose NO TRANSACTION
-- +goose Up
ALTER TABLE users ADD COLUMN IF NOT EXISTS display_name STRING NOT NULL DEFAULT '';
ALTER TABLE users ADD COLUMN IF NOT EXISTS show_display_name BOOL NOT NULL DEFAULT false;

-- +goose Down
ALTER TABLE users DROP COLUMN IF EXISTS show_display_name;
ALTER TABLE users DROP COLUMN IF EXISTS display_name;
