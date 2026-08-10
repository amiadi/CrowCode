-- name: CreateRAGSource :one
INSERT INTO rag_sources (
    id,
    source_type,
    uri,
    name,
    etag,
    checksum,
    metadata,
    last_indexed_at,
    updated_at,
    created_at
) VALUES (
    ?, ?, ?, ?, ?, ?, ?, ?, strftime('%s', 'now'), strftime('%s', 'now')
)
RETURNING *;

-- name: GetRAGSourceByURI :one
SELECT *
FROM rag_sources
WHERE uri = ?
LIMIT 1;

-- name: ListRAGSources :many
SELECT *
FROM rag_sources
ORDER BY created_at DESC;

-- name: UpdateRAGSourceIndexState :one
UPDATE rag_sources
SET
    etag = ?,
    checksum = ?,
    metadata = ?,
    last_indexed_at = ?,
    updated_at = strftime('%s', 'now')
WHERE id = ?
RETURNING *;

-- name: CreateRAGDocument :one
INSERT INTO rag_documents (
    id,
    source_id,
    external_id,
    title,
    metadata,
    content_hash,
    updated_at,
    created_at
) VALUES (
    ?, ?, ?, ?, ?, ?, strftime('%s', 'now'), strftime('%s', 'now')
)
RETURNING *;

-- name: GetRAGDocumentBySourceExternalID :one
SELECT *
FROM rag_documents
WHERE source_id = ? AND external_id = ?
LIMIT 1;

-- name: ListRAGDocumentsBySource :many
SELECT *
FROM rag_documents
WHERE source_id = ?
ORDER BY created_at DESC;

-- name: CreateRAGChunk :one
INSERT INTO rag_chunks (
    id,
    document_id,
    chunk_index,
    content,
    token_count,
    embedding,
    embedding_model,
    metadata,
    updated_at,
    created_at
) VALUES (
    ?, ?, ?, ?, ?, ?, ?, ?, strftime('%s', 'now'), strftime('%s', 'now')
)
RETURNING *;

-- name: UpdateRAGChunkEmbedding :one
UPDATE rag_chunks
SET
    embedding = ?,
    embedding_model = ?,
    updated_at = strftime('%s', 'now')
WHERE id = ?
RETURNING *;

-- name: DeleteRAGChunksByDocument :exec
DELETE FROM rag_chunks
WHERE document_id = ?;

-- name: ListRAGChunksByDocument :many
SELECT *
FROM rag_chunks
WHERE document_id = ?
ORDER BY chunk_index ASC;

-- name: SearchRAGChunksLexical :many
SELECT
    c.id,
    c.document_id,
    c.chunk_index,
    c.content,
    c.token_count,
    c.embedding,
    c.embedding_model,
    c.metadata,
    c.updated_at,
    c.created_at,
    d.source_id,
    d.external_id,
    d.title,
    bm25(rag_chunks_fts) AS lexical_score
FROM rag_chunks_fts
INNER JOIN rag_chunks c ON c.id = rag_chunks_fts.chunk_id
INNER JOIN rag_documents d ON d.id = c.document_id
WHERE rag_chunks_fts MATCH ?
ORDER BY lexical_score ASC
LIMIT ?;

-- name: CreateRAGIngestionJob :one
INSERT INTO rag_ingestion_jobs (
    id,
    source_id,
    status,
    error,
    documents_seen,
    chunks_created,
    started_at,
    finished_at,
    created_at
) VALUES (
    ?, ?, ?, ?, ?, ?, ?, ?, strftime('%s', 'now')
)
RETURNING *;

-- name: UpdateRAGIngestionJob :one
UPDATE rag_ingestion_jobs
SET
    status = ?,
    error = ?,
    documents_seen = ?,
    chunks_created = ?,
    started_at = ?,
    finished_at = ?
WHERE id = ?
RETURNING *;

-- name: CreateRAGRetrievalLog :one
INSERT INTO rag_retrieval_logs (
    id,
    query_text,
    strategy,
    top_k,
    latency_ms,
    metadata,
    created_at
) VALUES (
    ?, ?, ?, ?, ?, ?, strftime('%s', 'now')
)
RETURNING *;

-- name: CreateRAGRetrievalLogResult :one
INSERT INTO rag_retrieval_log_results (
    log_id,
    chunk_id,
    rank,
    lexical_score,
    vector_score,
    hybrid_score
) VALUES (
    ?, ?, ?, ?, ?, ?
)
RETURNING *;

-- name: ListRAGRetrievalLogResults :many
SELECT *
FROM rag_retrieval_log_results
WHERE log_id = ?
ORDER BY rank ASC;
