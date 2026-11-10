-- +goose NO TRANSACTION
-- +goose Up
CREATE TABLE IF NOT EXISTS reservation_challenges (
    id STRING PRIMARY KEY,
    wish_id UUID NOT NULL REFERENCES wishes(id) ON DELETE CASCADE,
    list_id UUID REFERENCES wishlists(id) ON DELETE CASCADE,
    collection_owner UUID REFERENCES users(id) ON DELETE CASCADE,
    access_hash STRING NOT NULL DEFAULT '',
    email STRING NOT NULL,
    locale STRING NOT NULL,
    user_id UUID REFERENCES users(id) ON DELETE CASCADE,
    code_hash STRING NOT NULL,
    attempts INT NOT NULL DEFAULT 0,
    expires_at TIMESTAMPTZ NOT NULL,
    created_at TIMESTAMPTZ NOT NULL,
    CHECK ((list_id IS NULL) != (collection_owner IS NULL))
);
CREATE INDEX IF NOT EXISTS reservation_challenges_expiry ON reservation_challenges (expires_at);

CREATE TABLE IF NOT EXISTS reservations (
    wish_id UUID PRIMARY KEY REFERENCES wishes(id) ON DELETE CASCADE,
    id UUID NOT NULL UNIQUE,
    list_id UUID REFERENCES wishlists(id) ON DELETE CASCADE,
    collection_owner UUID REFERENCES users(id) ON DELETE CASCADE,
    context_key STRING NOT NULL,
    email STRING NOT NULL,
    locale STRING NOT NULL,
    user_id UUID REFERENCES users(id) ON DELETE CASCADE,
    token_hash STRING NOT NULL UNIQUE,
    status STRING NOT NULL CHECK (status IN ('reserved', 'purchased')),
    expires_at TIMESTAMPTZ,
    reminder_for TIMESTAMPTZ,
    created_at TIMESTAMPTZ NOT NULL,
    CHECK ((list_id IS NULL) != (collection_owner IS NULL)),
    CHECK ((status = 'reserved' AND expires_at IS NOT NULL) OR (status = 'purchased' AND expires_at IS NULL))
);
CREATE INDEX IF NOT EXISTS reservations_context_email ON reservations (context_key, email);
CREATE INDEX IF NOT EXISTS reservations_list ON reservations (list_id);

CREATE TABLE IF NOT EXISTS reservation_limit_locks (
    key STRING PRIMARY KEY
);

-- Each event is recorded in the same transaction as the change it describes.
-- Mail is delivered outside retried transactions; no management token is stored.
CREATE TABLE IF NOT EXISTS reservation_mail_outbox (
    id UUID PRIMARY KEY,
    recipient STRING NOT NULL,
    locale STRING NOT NULL,
    kind STRING NOT NULL,
    wish_title STRING NOT NULL,
    list_title STRING NOT NULL DEFAULT '',
    event_date STRING NOT NULL DEFAULT '',
    reservation_id UUID,
    created_at TIMESTAMPTZ NOT NULL,
    available_at TIMESTAMPTZ NOT NULL,
    attempts INT NOT NULL DEFAULT 0,
    lease_until TIMESTAMPTZ,
    sent_at TIMESTAMPTZ
);
CREATE INDEX IF NOT EXISTS reservation_mail_pending ON reservation_mail_outbox (sent_at, available_at);

-- +goose Down
DROP TABLE IF EXISTS reservation_mail_outbox;
DROP TABLE IF EXISTS reservation_limit_locks;
DROP TABLE IF EXISTS reservations;
DROP TABLE IF EXISTS reservation_challenges;
