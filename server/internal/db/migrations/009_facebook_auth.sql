-- +goose NO TRANSACTION
-- +goose Up
ALTER TABLE users ALTER COLUMN email DROP NOT NULL;
ALTER TABLE users ALTER COLUMN email_verified_at DROP NOT NULL;
CREATE TABLE IF NOT EXISTS auth_facebook_identities (
    subject STRING PRIMARY KEY,
    user_id UUID NOT NULL UNIQUE REFERENCES users(id) ON DELETE CASCADE,
    created_at TIMESTAMPTZ NOT NULL
);

CREATE TABLE IF NOT EXISTS auth_facebook_flows (
    state_hash STRING PRIMARY KEY,
    verifier STRING NOT NULL,
    mode STRING NOT NULL CHECK (mode IN ('login', 'link')),
    session_hash STRING NOT NULL DEFAULT '',
    expires_at TIMESTAMPTZ NOT NULL
);
CREATE INDEX IF NOT EXISTS auth_facebook_flows_expiry ON auth_facebook_flows (expires_at);

CREATE TABLE IF NOT EXISTS auth_facebook_pending (
    token_hash STRING PRIMARY KEY,
    subject STRING NOT NULL,
    email STRING NOT NULL,
    expires_at TIMESTAMPTZ NOT NULL
);
CREATE INDEX IF NOT EXISTS auth_facebook_pending_expiry ON auth_facebook_pending (expires_at);

-- +goose Down
-- Refuse rollback while accounts without email exist; never invent addresses.
ALTER TABLE users ALTER COLUMN email SET NOT NULL;
ALTER TABLE users ALTER COLUMN email_verified_at SET NOT NULL;
DROP TABLE IF EXISTS auth_facebook_pending;
DROP TABLE IF EXISTS auth_facebook_flows;
DROP TABLE IF EXISTS auth_facebook_identities;
