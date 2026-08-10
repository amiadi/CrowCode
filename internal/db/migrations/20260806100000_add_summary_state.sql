-- +goose Up
-- +goose StatementBegin
ALTER TABLE sessions ADD COLUMN summary_state TEXT;
-- +goose StatementEnd

-- +goose Down
-- +goose StatementBegin
-- SQLite does not support dropping columns directly, so this migration is intentionally one-way.
-- +goose StatementEnd