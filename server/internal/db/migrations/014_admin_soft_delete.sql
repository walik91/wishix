-- +goose NO TRANSACTION
-- +goose Up
ALTER TABLE users ADD COLUMN IF NOT EXISTS deleted_at TIMESTAMPTZ;
CREATE INDEX IF NOT EXISTS users_deleted_at ON users (deleted_at);
ALTER TABLE reservation_mail_outbox ADD COLUMN IF NOT EXISTS suppressed_at TIMESTAMPTZ;
CREATE TABLE IF NOT EXISTS admin_deleted_reservations (
    reservation_id UUID PRIMARY KEY,
    deleted_for_user_id UUID NOT NULL REFERENCES users(id),
    archived_at TIMESTAMPTZ NOT NULL,
    reservation JSONB NOT NULL
);

-- +goose Down
DROP TABLE IF EXISTS admin_deleted_reservations;
ALTER TABLE reservation_mail_outbox DROP COLUMN IF EXISTS suppressed_at;
DROP INDEX IF EXISTS users_deleted_at;
ALTER TABLE users DROP COLUMN IF EXISTS deleted_at;
