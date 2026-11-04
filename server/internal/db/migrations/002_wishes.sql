-- +goose NO TRANSACTION
-- +goose Up
CREATE TABLE IF NOT EXISTS wishes (
    id UUID PRIMARY KEY,
    owner_id UUID NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    title STRING NOT NULL CHECK (char_length(title) BETWEEN 1 AND 200),
    description STRING NOT NULL DEFAULT '' CHECK (char_length(description) <= 5000),
    price DECIMAL(12, 2) CHECK (price >= 0),
    currency STRING NOT NULL DEFAULT '',
    priority STRING NOT NULL CHECK (priority IN ('low', 'medium', 'high')),
    created_at TIMESTAMPTZ NOT NULL,
    CHECK ((price IS NULL AND currency = '') OR (price IS NOT NULL AND currency ~ '^[A-Z]{3}$'))
);

CREATE INDEX IF NOT EXISTS wishes_owner_created ON wishes (owner_id, created_at DESC, id DESC);

CREATE TABLE IF NOT EXISTS wish_shop_links (
    wish_id UUID NOT NULL REFERENCES wishes(id) ON DELETE CASCADE,
    position INT NOT NULL CHECK (position BETWEEN 0 AND 9),
    url STRING NOT NULL CHECK (char_length(url) BETWEEN 1 AND 2048),
    PRIMARY KEY (wish_id, position)
);

-- +goose Down
DROP TABLE IF EXISTS wish_shop_links;
DROP TABLE IF EXISTS wishes;
