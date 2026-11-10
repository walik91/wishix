-- +goose NO TRANSACTION
-- +goose Up
ALTER TABLE reservation_mail_outbox ADD COLUMN IF NOT EXISTS message STRING NOT NULL DEFAULT '';

-- +goose Down
ALTER TABLE reservation_mail_outbox DROP COLUMN IF EXISTS message;
