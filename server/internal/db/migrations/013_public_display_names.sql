-- +goose Up
UPDATE users SET display_name = username WHERE trim(display_name) = '';

-- +goose Down
-- Existing names are retained: the previous empty values cannot be recovered.
SELECT 1;
