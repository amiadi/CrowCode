-- +goose Up
-- +goose StatementBegin
ALTER TABLE files ADD COLUMN is_new INTEGER NOT NULL DEFAULT 1;

CREATE INDEX IF NOT EXISTS idx_files_is_new ON files (is_new);
-- +goose StatementEnd

-- +goose Down
-- +goose StatementBegin
DROP INDEX IF EXISTS idx_files_is_new;
-- SQLite does not support dropping columns directly, so the migration is intentionally one-way.
-- +goose StatementEnd