package rag

import (
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"math"
	"sort"
	"strings"
	"time"
	"unicode"

	"github.com/google/uuid"
	"github.com/opencode-ai/opencode/internal/config"
	"github.com/opencode-ai/opencode/internal/db"
)

type Service struct {
	q        db.Querier
	embedder Embedder
}

type IndexParams struct {
	SourceType     string
	URI            string
	Name           string
	ExternalID     string
	Title          string
	Text           string
	ChunkSize      int
	ChunkOverlap   int
	EmbeddingModel string
}

type IndexResult struct {
	SourceID      string
	DocumentID    string
	ChunksCreated int
}

type HybridResult struct {
	Chunk        db.SearchRAGChunksLexicalRow
	LexicalScore float64
	VectorScore  float64
	HybridScore  float64
}

type BenchmarkResult struct {
	Runs                 int
	Limit                int64
	AvgLatencyMs         float64
	P50LatencyMs         float64
	P95LatencyMs         float64
	AvgProxyQualityScore float64
}

func NewService(q db.Querier) *Service {
	return &Service{q: q, embedder: newHashEmbedder(128, "crowcode-hash-128")}
}

func NewServiceWithEmbedder(q db.Querier, embedder Embedder) *Service {
	if embedder == nil {
		embedder = newHashEmbedder(128, "crowcode-hash-128")
	}
	return &Service{q: q, embedder: embedder}
}

func NewServiceFromConfig(q db.Querier, ragCfg config.RAGConfig) *Service {
	return NewServiceWithEmbedder(q, newEmbedderFromConfig(ragCfg))
}

func EmbedText(text string, dims int) []float64 {
	return hashEmbedText(text, dims)
}

func hashEmbedText(text string, dims int) []float64 {
	if dims <= 0 {
		dims = 128
	}
	vec := make([]float64, dims)
	tokens := strings.Fields(strings.ToLower(text))
	for _, token := range tokens {
		h := 2166136261
		for i := 0; i < len(token); i++ {
			h ^= int(token[i])
			h *= 16777619
		}
		idx := h % dims
		if idx < 0 {
			idx = -idx
		}
		vec[idx] += 1
	}
	norm := l2Norm(vec)
	if norm == 0 {
		return vec
	}
	for i := range vec {
		vec[i] /= norm
	}
	return vec
}

func (s *Service) IndexText(ctx context.Context, params IndexParams) (IndexResult, error) {
	if strings.TrimSpace(params.URI) == "" {
		return IndexResult{}, fmt.Errorf("uri is required")
	}
	if strings.TrimSpace(params.Text) == "" {
		return IndexResult{}, fmt.Errorf("text is required")
	}
	if params.ChunkSize <= 0 {
		params.ChunkSize = 1200
	}
	if params.ChunkOverlap < 0 {
		params.ChunkOverlap = 0
	}
	if params.ChunkOverlap >= params.ChunkSize {
		params.ChunkOverlap = params.ChunkSize / 4
	}
	if params.SourceType == "" {
		params.SourceType = "manual"
	}
	if params.Name == "" {
		params.Name = params.URI
	}
	if params.ExternalID == "" {
		params.ExternalID = uuid.NewString()
	}
	if params.Title == "" {
		params.Title = params.Name
	}

	now := time.Now().Unix()
	source, err := s.q.GetRAGSourceByURI(ctx, params.URI)
	if err != nil {
		if err == sql.ErrNoRows {
			source, err = s.q.CreateRAGSource(ctx, db.CreateRAGSourceParams{
				ID:         uuid.NewString(),
				SourceType: params.SourceType,
				URI:        params.URI,
				Name:       params.Name,
				Metadata:   "{}",
			})
			if err != nil {
				return IndexResult{}, err
			}
		} else {
			return IndexResult{}, err
		}
	}

	job, err := s.q.CreateRAGIngestionJob(ctx, db.CreateRAGIngestionJobParams{
		ID:            uuid.NewString(),
		SourceID:      source.ID,
		Status:        "running",
		DocumentsSeen: 0,
		ChunksCreated: 0,
		StartedAt:     sql.NullInt64{Int64: now, Valid: true},
	})
	if err != nil {
		return IndexResult{}, err
	}

	finalizeError := func(runErr error) error {
		_, _ = s.q.UpdateRAGIngestionJob(ctx, db.UpdateRAGIngestionJobParams{
			ID:            job.ID,
			Status:        "failed",
			Error:         sql.NullString{String: runErr.Error(), Valid: true},
			DocumentsSeen: 1,
			ChunksCreated: 0,
			StartedAt:     job.StartedAt,
			FinishedAt:    sql.NullInt64{Int64: time.Now().Unix(), Valid: true},
		})
		return runErr
	}

	doc, err := s.q.GetRAGDocumentBySourceExternalID(ctx, db.GetRAGDocumentBySourceExternalIDParams{
		SourceID:   source.ID,
		ExternalID: params.ExternalID,
	})
	if err != nil {
		if err == sql.ErrNoRows {
			doc, err = s.q.CreateRAGDocument(ctx, db.CreateRAGDocumentParams{
				ID:         uuid.NewString(),
				SourceID:   source.ID,
				ExternalID: params.ExternalID,
				Title:      params.Title,
				Metadata:   "{}",
			})
			if err != nil {
				return IndexResult{}, finalizeError(err)
			}
		} else {
			return IndexResult{}, finalizeError(err)
		}
	}

	if err := s.q.DeleteRAGChunksByDocument(ctx, doc.ID); err != nil {
		return IndexResult{}, finalizeError(err)
	}

	chunks := splitTextWithOverlap(params.Text, params.ChunkSize, params.ChunkOverlap)
	created := 0
	for idx, chunk := range chunks {
		emb, err := s.embedder.Embed(ctx, chunk)
		if err != nil {
			return IndexResult{}, finalizeError(err)
		}
		embJSON, err := json.Marshal(emb)
		if err != nil {
			return IndexResult{}, finalizeError(err)
		}

		embeddingModel := params.EmbeddingModel
		if strings.TrimSpace(embeddingModel) == "" {
			embeddingModel = s.embedder.ModelLabel()
		}

		_, err = s.q.CreateRAGChunk(ctx, db.CreateRAGChunkParams{
			ID:             uuid.NewString(),
			DocumentID:     doc.ID,
			ChunkIndex:     int64(idx),
			Content:        chunk,
			TokenCount:     int64(len(strings.Fields(chunk))),
			Embedding:      string(embJSON),
			EmbeddingModel: sql.NullString{String: embeddingModel, Valid: embeddingModel != ""},
			Metadata:       "{}",
		})
		if err != nil {
			return IndexResult{}, finalizeError(err)
		}
		created++
	}

	_, err = s.q.UpdateRAGSourceIndexState(ctx, db.UpdateRAGSourceIndexStateParams{
		ID:            source.ID,
		Metadata:      "{}",
		LastIndexedAt: sql.NullInt64{Int64: time.Now().Unix(), Valid: true},
	})
	if err != nil {
		return IndexResult{}, finalizeError(err)
	}

	_, err = s.q.UpdateRAGIngestionJob(ctx, db.UpdateRAGIngestionJobParams{
		ID:            job.ID,
		Status:        "completed",
		DocumentsSeen: 1,
		ChunksCreated: int64(created),
		StartedAt:     job.StartedAt,
		FinishedAt:    sql.NullInt64{Int64: time.Now().Unix(), Valid: true},
	})
	if err != nil {
		return IndexResult{}, err
	}

	return IndexResult{SourceID: source.ID, DocumentID: doc.ID, ChunksCreated: created}, nil
}

func (s *Service) HybridSearch(ctx context.Context, query string, limit int64) ([]HybridResult, error) {
	if strings.TrimSpace(query) == "" {
		return nil, fmt.Errorf("query is required")
	}
	if limit <= 0 {
		limit = 5
	}

	searchQuery := normalizeFTSQuery(query)
	if searchQuery == "" {
		return nil, fmt.Errorf("query is required")
	}

	candidateLimit := limit * 5
	if candidateLimit < 20 {
		candidateLimit = 20
	}
	lexicalRows, err := s.q.SearchRAGChunksLexical(ctx, db.SearchRAGChunksLexicalParams{
		Query: searchQuery,
		Limit: candidateLimit,
	})
	if err != nil {
		return nil, err
	}

	queryEmbedding, err := s.embedder.Embed(ctx, query)
	if err != nil {
		return nil, err
	}
	embeddingModelLabel := strings.TrimSpace(s.embedder.ModelLabel())
	results := make([]HybridResult, 0, len(lexicalRows))
	for _, row := range lexicalRows {
		chunkEmbedding := parseEmbedding(row.Embedding)
		if shouldBackfillEmbedding(row.EmbeddingModel, embeddingModelLabel) {
			refreshed, refreshErr := s.embedder.Embed(ctx, row.Content)
			if refreshErr == nil && len(refreshed) > 0 {
				chunkEmbedding = refreshed
				updatedRaw, marshalErr := json.Marshal(refreshed)
				if marshalErr == nil {
					_, _ = s.q.UpdateRAGChunkEmbedding(ctx, db.UpdateRAGChunkEmbeddingParams{
						ID:             row.ID,
						Embedding:      string(updatedRaw),
						EmbeddingModel: sql.NullString{String: embeddingModelLabel, Valid: embeddingModelLabel != ""},
					})
				}
			}
		}
		vectorScore := cosineSimilarity(queryEmbedding, chunkEmbedding)
		vectorRel := (vectorScore + 1) / 2

		rawLexical := row.LexicalScore
		if rawLexical < 0 {
			rawLexical = 0
		}
		lexicalRel := 1.0 / (1.0 + rawLexical)

		hybrid := 0.55*lexicalRel + 0.45*vectorRel
		results = append(results, HybridResult{
			Chunk:        row,
			LexicalScore: lexicalRel,
			VectorScore:  vectorRel,
			HybridScore:  hybrid,
		})
	}

	sort.Slice(results, func(i, j int) bool {
		return results[i].HybridScore > results[j].HybridScore
	})
	if int64(len(results)) > limit {
		results = results[:limit]
	}

	latencyMs := int64(0)
	if len(lexicalRows) > 0 {
		latencyMs = 1
	}
	logRecord, err := s.q.CreateRAGRetrievalLog(ctx, db.CreateRAGRetrievalLogParams{
		ID:        uuid.NewString(),
		QueryText: query,
		Strategy:  "hybrid_lexical_vector",
		TopK:      limit,
		LatencyMs: sql.NullInt64{Int64: latencyMs, Valid: true},
		Metadata:  "{}",
	})
	if err == nil {
		for i, result := range results {
			_, _ = s.q.CreateRAGRetrievalLogResult(ctx, db.CreateRAGRetrievalLogResultParams{
				LogID:        logRecord.ID,
				ChunkID:      result.Chunk.ID,
				Rank:         int64(i + 1),
				LexicalScore: sql.NullFloat64{Float64: result.LexicalScore, Valid: true},
				VectorScore:  sql.NullFloat64{Float64: result.VectorScore, Valid: true},
				HybridScore:  sql.NullFloat64{Float64: result.HybridScore, Valid: true},
			})
		}
	}
	return results, nil
}

func (s *Service) BackfillEmbeddings(ctx context.Context, limit int64) (int64, error) {
	modelLabel := strings.TrimSpace(s.embedder.ModelLabel())
	if modelLabel == "" {
		return 0, fmt.Errorf("embedder model label is empty")
	}

	var updated int64
	sources, err := s.q.ListRAGSources(ctx)
	if err != nil {
		return 0, err
	}

	for _, source := range sources {
		docs, err := s.q.ListRAGDocumentsBySource(ctx, source.ID)
		if err != nil {
			return updated, err
		}
		for _, doc := range docs {
			chunks, err := s.q.ListRAGChunksByDocument(ctx, doc.ID)
			if err != nil {
				return updated, err
			}
			for _, chunk := range chunks {
				if !shouldBackfillEmbedding(chunk.EmbeddingModel, modelLabel) {
					continue
				}
				emb, err := s.embedder.Embed(ctx, chunk.Content)
				if err != nil {
					return updated, err
				}
				embJSON, err := json.Marshal(emb)
				if err != nil {
					return updated, err
				}

				_, err = s.q.UpdateRAGChunkEmbedding(ctx, db.UpdateRAGChunkEmbeddingParams{
					ID:             chunk.ID,
					Embedding:      string(embJSON),
					EmbeddingModel: sql.NullString{String: modelLabel, Valid: modelLabel != ""},
				})
				if err != nil {
					return updated, err
				}
				updated++
				if limit > 0 && updated >= limit {
					return updated, nil
				}
			}
		}
	}

	return updated, nil
}

func shouldBackfillEmbedding(existing sql.NullString, currentModel string) bool {
	if strings.TrimSpace(currentModel) == "" {
		return false
	}
	if !existing.Valid || strings.TrimSpace(existing.String) == "" {
		return true
	}
	existingModel := strings.ToLower(strings.TrimSpace(existing.String))
	current := strings.ToLower(strings.TrimSpace(currentModel))
	if existingModel == current {
		return false
	}
	if strings.Contains(existingModel, "hash") || strings.HasPrefix(existingModel, "crowcode-hash-") {
		return true
	}
	return false
}

func normalizeFTSQuery(query string) string {
	terms := strings.FieldsFunc(strings.ToLower(query), func(r rune) bool {
		return !unicode.IsLetter(r) && !unicode.IsNumber(r)
	})
	if len(terms) == 0 {
		return ""
	}
	return strings.Join(terms, " AND ")
}

func (s *Service) BenchmarkSearch(ctx context.Context, query string, limit int64, runs int, expectedTerm string) (BenchmarkResult, error) {
	if runs <= 0 {
		runs = 5
	}
	latencies := make([]float64, 0, runs)
	qualityScores := make([]float64, 0, runs)

	for i := 0; i < runs; i++ {
		started := time.Now()
		results, err := s.HybridSearch(ctx, query, limit)
		if err != nil {
			return BenchmarkResult{}, err
		}
		latencies = append(latencies, float64(time.Since(started).Milliseconds()))
		qualityScores = append(qualityScores, qualityProxy(results, expectedTerm))
	}

	sort.Float64s(latencies)
	avg := average(latencies)
	p50 := percentile(latencies, 0.50)
	p95 := percentile(latencies, 0.95)

	return BenchmarkResult{
		Runs:                 runs,
		Limit:                limit,
		AvgLatencyMs:         avg,
		P50LatencyMs:         p50,
		P95LatencyMs:         p95,
		AvgProxyQualityScore: average(qualityScores),
	}, nil
}

func splitTextWithOverlap(text string, chunkSize int, overlap int) []string {
	if chunkSize <= 0 {
		chunkSize = 1200
	}
	if overlap < 0 {
		overlap = 0
	}
	if overlap >= chunkSize {
		overlap = chunkSize / 4
	}

	runes := []rune(strings.TrimSpace(text))
	if len(runes) == 0 {
		return nil
	}

	chunks := make([]string, 0)
	step := chunkSize - overlap
	if step <= 0 {
		step = chunkSize
	}

	for start := 0; start < len(runes); start += step {
		end := start + chunkSize
		if end > len(runes) {
			end = len(runes)
		}
		chunk := strings.TrimSpace(string(runes[start:end]))
		if chunk != "" {
			chunks = append(chunks, chunk)
		}
		if end == len(runes) {
			break
		}
	}
	return chunks
}

func parseEmbedding(raw string) []float64 {
	if strings.TrimSpace(raw) == "" {
		return nil
	}
	values := make([]float64, 0)
	if err := json.Unmarshal([]byte(raw), &values); err != nil {
		return nil
	}
	return values
}

func cosineSimilarity(a, b []float64) float64 {
	if len(a) == 0 || len(b) == 0 {
		return 0
	}
	n := len(a)
	if len(b) < n {
		n = len(b)
	}
	var dot float64
	for i := 0; i < n; i++ {
		dot += a[i] * b[i]
	}
	aNorm := l2Norm(a)
	bNorm := l2Norm(b)
	if aNorm == 0 || bNorm == 0 {
		return 0
	}
	return dot / (aNorm * bNorm)
}

func l2Norm(v []float64) float64 {
	var sum float64
	for _, x := range v {
		sum += x * x
	}
	return math.Sqrt(sum)
}

func qualityProxy(results []HybridResult, expectedTerm string) float64 {
	if len(results) == 0 {
		return 0
	}
	if strings.TrimSpace(expectedTerm) == "" {
		return 1
	}
	needle := strings.ToLower(expectedTerm)
	hits := 0
	for _, result := range results {
		if strings.Contains(strings.ToLower(result.Chunk.Content), needle) {
			hits++
		}
	}
	return float64(hits) / float64(len(results))
}

func percentile(sorted []float64, p float64) float64 {
	if len(sorted) == 0 {
		return 0
	}
	if p <= 0 {
		return sorted[0]
	}
	if p >= 1 {
		return sorted[len(sorted)-1]
	}
	idx := int(math.Ceil(float64(len(sorted))*p)) - 1
	if idx < 0 {
		idx = 0
	}
	if idx >= len(sorted) {
		idx = len(sorted) - 1
	}
	return sorted[idx]
}

func average(values []float64) float64 {
	if len(values) == 0 {
		return 0
	}
	var sum float64
	for _, value := range values {
		sum += value
	}
	return sum / float64(len(values))
}
