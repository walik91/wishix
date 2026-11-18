-- +goose NO TRANSACTION
-- +goose Up
ALTER TABLE users ADD COLUMN IF NOT EXISTS username_changed_at TIMESTAMPTZ;
ALTER TABLE users ADD COLUMN IF NOT EXISTS biography STRING NOT NULL DEFAULT '';
ALTER TABLE users ADD COLUMN IF NOT EXISTS birth_date DATE;
ALTER TABLE users ADD COLUMN IF NOT EXISTS show_age BOOL NOT NULL DEFAULT false;
ALTER TABLE users ADD COLUMN IF NOT EXISTS show_biography BOOL NOT NULL DEFAULT false;
ALTER TABLE users ADD COLUMN IF NOT EXISTS show_avatar BOOL NOT NULL DEFAULT false;
ALTER TABLE users ADD COLUMN IF NOT EXISTS show_followers BOOL NOT NULL DEFAULT false;
ALTER TABLE users ADD COLUMN IF NOT EXISTS show_following BOOL NOT NULL DEFAULT false;

CREATE TABLE IF NOT EXISTS username_registry (
    username STRING PRIMARY KEY,
    user_id UUID NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    expires_at TIMESTAMPTZ
);
INSERT INTO username_registry (username, user_id)
SELECT username, id FROM users ON CONFLICT (username) DO NOTHING;

CREATE TABLE IF NOT EXISTS profile_avatars (
    user_id UUID PRIMARY KEY REFERENCES users(id) ON DELETE CASCADE,
    version UUID NOT NULL,
    data BYTES NOT NULL
);
CREATE TABLE IF NOT EXISTS user_follows (
    follower_id UUID NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    followed_id UUID NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    created_at TIMESTAMPTZ NOT NULL,
    PRIMARY KEY (follower_id, followed_id),
    CHECK (follower_id <> followed_id)
);
CREATE INDEX IF NOT EXISTS user_follows_followed ON user_follows (followed_id, follower_id);
CREATE TABLE IF NOT EXISTS user_blocks (
    blocker_id UUID NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    blocked_id UUID NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    created_at TIMESTAMPTZ NOT NULL,
    PRIMARY KEY (blocker_id, blocked_id),
    CHECK (blocker_id <> blocked_id)
);
CREATE TABLE IF NOT EXISTS user_reports (
    id UUID PRIMARY KEY,
    reporter_id UUID NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    reported_id UUID NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    message STRING NOT NULL,
    created_at TIMESTAMPTZ NOT NULL,
    CHECK (reporter_id <> reported_id)
);

-- +goose Down
DROP TABLE IF EXISTS user_reports;
DROP TABLE IF EXISTS user_blocks;
DROP TABLE IF EXISTS user_follows;
DROP TABLE IF EXISTS profile_avatars;
DROP TABLE IF EXISTS username_registry;
ALTER TABLE users DROP COLUMN IF EXISTS show_following;
ALTER TABLE users DROP COLUMN IF EXISTS show_followers;
ALTER TABLE users DROP COLUMN IF EXISTS show_avatar;
ALTER TABLE users DROP COLUMN IF EXISTS show_biography;
ALTER TABLE users DROP COLUMN IF EXISTS show_age;
ALTER TABLE users DROP COLUMN IF EXISTS birth_date;
ALTER TABLE users DROP COLUMN IF EXISTS biography;
ALTER TABLE users DROP COLUMN IF EXISTS username_changed_at;
