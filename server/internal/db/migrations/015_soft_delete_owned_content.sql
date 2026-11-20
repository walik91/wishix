-- +goose NO TRANSACTION
-- +goose Up
ALTER TABLE wishes ADD COLUMN IF NOT EXISTS deleted_at TIMESTAMPTZ;
ALTER TABLE wishlists ADD COLUMN IF NOT EXISTS deleted_at TIMESTAMPTZ;
UPDATE wishes SET deleted_at = (SELECT u.deleted_at FROM users u WHERE u.id = wishes.owner_id)
WHERE deleted_at IS NULL AND EXISTS (SELECT 1 FROM users u WHERE u.id = wishes.owner_id AND u.deleted_at IS NOT NULL);
UPDATE wishlists SET deleted_at = (SELECT u.deleted_at FROM users u WHERE u.id = wishlists.owner_id)
WHERE deleted_at IS NULL AND EXISTS (SELECT 1 FROM users u WHERE u.id = wishlists.owner_id AND u.deleted_at IS NOT NULL);

-- +goose Down
ALTER TABLE wishlists DROP COLUMN IF EXISTS deleted_at;
ALTER TABLE wishes DROP COLUMN IF EXISTS deleted_at;
