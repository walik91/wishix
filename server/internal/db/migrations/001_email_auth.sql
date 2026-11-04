-- +goose NO TRANSACTION
-- +goose Up
CREATE TABLE IF NOT EXISTS users (
    id UUID PRIMARY KEY,
    username STRING NOT NULL UNIQUE,
    email STRING NOT NULL UNIQUE,
    password_hash STRING NOT NULL,
    email_verified_at TIMESTAMPTZ NOT NULL,
    created_at TIMESTAMPTZ NOT NULL,
    show_all_wishes BOOL NOT NULL DEFAULT false
);

CREATE TABLE IF NOT EXISTS auth_challenges (
    id STRING PRIMARY KEY,
    purpose STRING NOT NULL CHECK (purpose IN ('register', 'reset')),
    email STRING NOT NULL,
    username STRING NOT NULL DEFAULT '',
    password_hash STRING NOT NULL DEFAULT '',
    user_id UUID REFERENCES users(id) ON DELETE CASCADE,
    locale STRING NOT NULL,
    code_hash STRING NOT NULL,
    attempts INT NOT NULL DEFAULT 0,
    expires_at TIMESTAMPTZ NOT NULL,
    sent_at TIMESTAMPTZ NOT NULL
);

CREATE INDEX IF NOT EXISTS auth_challenges_expiry ON auth_challenges (expires_at);

CREATE TABLE IF NOT EXISTS auth_sessions (
    token_hash STRING PRIMARY KEY,
    user_id UUID NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    expires_at TIMESTAMPTZ NOT NULL,
    created_at TIMESTAMPTZ NOT NULL
);

CREATE INDEX IF NOT EXISTS auth_sessions_user ON auth_sessions (user_id);
CREATE INDEX IF NOT EXISTS auth_sessions_expiry ON auth_sessions (expires_at);

CREATE TABLE IF NOT EXISTS auth_rate_limits (
    key STRING PRIMARY KEY,
    count INT NOT NULL,
    expires_at TIMESTAMPTZ NOT NULL
);

CREATE INDEX IF NOT EXISTS auth_rate_limits_expiry ON auth_rate_limits (expires_at);

-- +goose Down
DROP TABLE IF EXISTS auth_sessions;
DROP TABLE IF EXISTS auth_challenges;
DROP TABLE IF EXISTS auth_rate_limits;
DROP TABLE IF EXISTS users;
