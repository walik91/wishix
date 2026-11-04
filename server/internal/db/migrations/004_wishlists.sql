-- +goose NO TRANSACTION
-- +goose Up
CREATE TABLE IF NOT EXISTS wishlists (
    id UUID PRIMARY KEY,
    owner_id UUID NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    title STRING NOT NULL CHECK (char_length(title) BETWEEN 1 AND 120),
    description STRING NOT NULL DEFAULT '' CHECK (char_length(description) <= 2000),
    event_date DATE,
    visibility STRING NOT NULL DEFAULT 'link' CHECK (visibility IN ('private', 'link', 'public')),
    share_token STRING NOT NULL UNIQUE CHECK (char_length(share_token) = 43),
    revision INT NOT NULL DEFAULT 1 CHECK (revision >= 1),
    created_at TIMESTAMPTZ NOT NULL,
    updated_at TIMESTAMPTZ NOT NULL,
    UNIQUE (id, owner_id)
);
CREATE INDEX IF NOT EXISTS wishlists_owner_created ON wishlists (owner_id, created_at DESC, id DESC);
CREATE INDEX IF NOT EXISTS wishlists_public_owner ON wishlists (owner_id, visibility);
CREATE UNIQUE INDEX IF NOT EXISTS wishes_id_owner ON wishes (id, owner_id);
ALTER TABLE wishes ADD COLUMN IF NOT EXISTS lists_version INT NOT NULL DEFAULT 1 CHECK (lists_version >= 1);

CREATE TABLE IF NOT EXISTS wishlist_wishes (
    list_id UUID NOT NULL,
    wish_id UUID NOT NULL,
    owner_id UUID NOT NULL,
    added_at TIMESTAMPTZ NOT NULL,
    PRIMARY KEY (list_id, wish_id),
    FOREIGN KEY (list_id, owner_id) REFERENCES wishlists (id, owner_id) ON DELETE CASCADE,
    FOREIGN KEY (wish_id, owner_id) REFERENCES wishes (id, owner_id) ON DELETE CASCADE
);
CREATE INDEX IF NOT EXISTS wishlist_wishes_by_wish ON wishlist_wishes (wish_id, owner_id, list_id);

-- +goose Down
DROP TABLE IF EXISTS wishlist_wishes;
ALTER TABLE wishes DROP COLUMN IF EXISTS lists_version;
DROP INDEX IF EXISTS wishes@wishes_id_owner;
DROP TABLE IF EXISTS wishlists;
