-- +goose NO TRANSACTION
-- +goose Up
CREATE TABLE IF NOT EXISTS wish_images (
    wish_id UUID PRIMARY KEY REFERENCES wishes(id) ON DELETE CASCADE,
    version UUID NOT NULL,
    data BYTES NOT NULL CHECK (octet_length(data) BETWEEN 1 AND 524288)
);

-- +goose Down
DROP TABLE IF EXISTS wish_images;
