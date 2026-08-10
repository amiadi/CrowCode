-- +goose Up
-- +goose StatementBegin
CREATE TABLE IF NOT EXISTS rag_sources (
    id TEXT PRIMARY KEY,
    source_type TEXT NOT NULL,
    uri TEXT NOT NULL UNIQUE,
    name TEXT NOT NULL,
    etag TEXT,
    checksum TEXT,
    metadata TEXT NOT NULL DEFAULT '{}',
    last_indexed_at INTEGER,
    updated_at INTEGER NOT NULL,
    created_at INTEGER NOT NULL
);

CREATE INDEX IF NOT EXISTS idx_rag_sources_source_type ON rag_sources (source_type);

CREATE TABLE IF NOT EXISTS rag_documents (
    id TEXT PRIMARY KEY,
    source_id TEXT NOT NULL,
    external_id TEXT NOT NULL,
    title TEXT NOT NULL,
    metadata TEXT NOT NULL DEFAULT '{}',
    content_hash TEXT,
    updated_at INTEGER NOT NULL,
    created_at INTEGER NOT NULL,
    FOREIGN KEY (source_id) REFERENCES rag_sources (id) ON DELETE CASCADE,
    UNIQUE (source_id, external_id)
);

CREATE INDEX IF NOT EXISTS idx_rag_documents_source_id ON rag_documents (source_id);

CREATE TABLE IF NOT EXISTS rag_chunks (
    id TEXT PRIMARY KEY,
    document_id TEXT NOT NULL,
    chunk_index INTEGER NOT NULL,
    content TEXT NOT NULL,
    token_count INTEGER NOT NULL DEFAULT 0,
    embedding TEXT NOT NULL DEFAULT '[]',
    embedding_model TEXT,
    metadata TEXT NOT NULL DEFAULT '{}',
    updated_at INTEGER NOT NULL,
    created_at INTEGER NOT NULL,
    FOREIGN KEY (document_id) REFERENCES rag_documents (id) ON DELETE CASCADE,
    UNIQUE (document_id, chunk_index)
);

CREATE INDEX IF NOT EXISTS idx_rag_chunks_document_id ON rag_chunks (document_id);
CREATE INDEX IF NOT EXISTS idx_rag_chunks_embedding_model ON rag_chunks (embedding_model);

CREATE VIRTUAL TABLE IF NOT EXISTS rag_chunks_fts USING fts5(
    chunk_id UNINDEXED,
    content,
    tokenize='unicode61'
);

CREATE TRIGGER IF NOT EXISTS rag_chunks_ai AFTER INSERT ON rag_chunks BEGIN
    INSERT INTO rag_chunks_fts (chunk_id, content) VALUES (new.id, new.content);
END;

CREATE TRIGGER IF NOT EXISTS rag_chunks_ad AFTER DELETE ON rag_chunks BEGIN
    INSERT INTO rag_chunks_fts (rag_chunks_fts, rowid, chunk_id, content)
    VALUES ('delete', old.rowid, old.id, old.content);
END;

CREATE TRIGGER IF NOT EXISTS rag_chunks_au AFTER UPDATE ON rag_chunks BEGIN
    INSERT INTO rag_chunks_fts (rag_chunks_fts, rowid, chunk_id, content)
    VALUES ('delete', old.rowid, old.id, old.content);
    INSERT INTO rag_chunks_fts (chunk_id, content) VALUES (new.id, new.content);
END;

CREATE TABLE IF NOT EXISTS rag_ingestion_jobs (
    id TEXT PRIMARY KEY,
    source_id TEXT NOT NULL,
    status TEXT NOT NULL,
    error TEXT,
    documents_seen INTEGER NOT NULL DEFAULT 0,
    chunks_created INTEGER NOT NULL DEFAULT 0,
    started_at INTEGER,
    finished_at INTEGER,
    created_at INTEGER NOT NULL,
    FOREIGN KEY (source_id) REFERENCES rag_sources (id) ON DELETE CASCADE
);

CREATE INDEX IF NOT EXISTS idx_rag_ingestion_jobs_source_id ON rag_ingestion_jobs (source_id);
CREATE INDEX IF NOT EXISTS idx_rag_ingestion_jobs_status ON rag_ingestion_jobs (status);

CREATE TABLE IF NOT EXISTS rag_retrieval_logs (
    id TEXT PRIMARY KEY,
    query_text TEXT NOT NULL,
    strategy TEXT NOT NULL,
    top_k INTEGER NOT NULL,
    latency_ms INTEGER,
    metadata TEXT NOT NULL DEFAULT '{}',
    created_at INTEGER NOT NULL
);

CREATE TABLE IF NOT EXISTS rag_retrieval_log_results (
    log_id TEXT NOT NULL,
    chunk_id TEXT NOT NULL,
    rank INTEGER NOT NULL,
    lexical_score REAL,
    vector_score REAL,
    hybrid_score REAL,
    PRIMARY KEY (log_id, chunk_id),
    FOREIGN KEY (log_id) REFERENCES rag_retrieval_logs (id) ON DELETE CASCADE,
    FOREIGN KEY (chunk_id) REFERENCES rag_chunks (id) ON DELETE CASCADE
);

CREATE INDEX IF NOT EXISTS idx_rag_retrieval_results_log_id ON rag_retrieval_log_results (log_id);
CREATE INDEX IF NOT EXISTS idx_rag_retrieval_results_chunk_id ON rag_retrieval_log_results (chunk_id);
-- +goose StatementEnd

-- +goose Down
-- +goose StatementBegin
DROP INDEX IF EXISTS idx_rag_retrieval_results_chunk_id;
DROP INDEX IF EXISTS idx_rag_retrieval_results_log_id;
DROP TABLE IF EXISTS rag_retrieval_log_results;
DROP TABLE IF EXISTS rag_retrieval_logs;

DROP INDEX IF EXISTS idx_rag_ingestion_jobs_status;
DROP INDEX IF EXISTS idx_rag_ingestion_jobs_source_id;
DROP TABLE IF EXISTS rag_ingestion_jobs;

DROP TRIGGER IF EXISTS rag_chunks_au;
DROP TRIGGER IF EXISTS rag_chunks_ad;
DROP TRIGGER IF EXISTS rag_chunks_ai;
DROP TABLE IF EXISTS rag_chunks_fts;

DROP INDEX IF EXISTS idx_rag_chunks_embedding_model;
DROP INDEX IF EXISTS idx_rag_chunks_document_id;
DROP TABLE IF EXISTS rag_chunks;

DROP INDEX IF EXISTS idx_rag_documents_source_id;
DROP TABLE IF EXISTS rag_documents;

DROP INDEX IF EXISTS idx_rag_sources_source_type;
DROP TABLE IF EXISTS rag_sources;
-- +goose StatementEnd
