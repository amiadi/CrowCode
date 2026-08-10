-- +goose Up
-- +goose StatementBegin
DROP TRIGGER IF EXISTS rag_chunks_au;

CREATE TRIGGER IF NOT EXISTS rag_chunks_au
AFTER UPDATE OF content ON rag_chunks
BEGIN
    INSERT INTO rag_chunks_fts (rag_chunks_fts, rowid, chunk_id, content)
    VALUES ('delete', old.rowid, old.id, old.content);
    INSERT INTO rag_chunks_fts (chunk_id, content) VALUES (new.id, new.content);
END;
-- +goose StatementEnd

-- +goose Down
-- +goose StatementBegin
DROP TRIGGER IF EXISTS rag_chunks_au;

CREATE TRIGGER IF NOT EXISTS rag_chunks_au AFTER UPDATE ON rag_chunks BEGIN
    INSERT INTO rag_chunks_fts (rag_chunks_fts, rowid, chunk_id, content)
    VALUES ('delete', old.rowid, old.id, old.content);
    INSERT INTO rag_chunks_fts (chunk_id, content) VALUES (new.id, new.content);
END;
-- +goose StatementEnd
