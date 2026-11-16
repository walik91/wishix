-- +goose NO TRANSACTION
-- +goose Up
CREATE TABLE IF NOT EXISTS auth_google_identities (
    subject STRING PRIMARY KEY,
    user_id UUID NOT NULL UNIQUE REFERENCES users(id) ON DELETE CASCADE,
    created_at TIMESTAMPTZ NOT NULL
);

CREATE TABLE IF NOT EXISTS auth_google_flows (
    state_hash STRING PRIMARY KEY,
    verifier STRING NOT NULL,
    mode STRING NOT NULL CHECK (mode IN ('login', 'link')),
    session_hash STRING NOT NULL DEFAULT '',
    expires_at TIMESTAMPTZ NOT NULL
);
CREATE INDEX IF NOT EXISTS auth_google_flows_expiry ON auth_google_flows (expires_at);

CREATE TABLE IF NOT EXISTS auth_google_pending (
    token_hash STRING PRIMARY KEY,
    subject STRING NOT NULL,
    email STRING NOT NULL,
    expires_at TIMESTAMPTZ NOT NULL
);
CREATE INDEX IF NOT EXISTS auth_google_pending_expiry ON auth_google_pending (expires_at);

-- +goose Down
DROP TABLE IF EXISTS auth_google_pending;
DROP TABLE IF EXISTS auth_google_flows;
DROP TABLE IF EXISTS auth_google_identities;
