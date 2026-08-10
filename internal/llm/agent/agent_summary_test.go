package agent

import (
	"database/sql"
	"strings"
	"testing"

	"github.com/opencode-ai/opencode/internal/config"
	"github.com/opencode-ai/opencode/internal/db"
	"github.com/opencode-ai/opencode/internal/rag"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestParseSummaryStateJSONWrappedText(t *testing.T) {
	raw := "summary follows\n```json\n{\"conversation_summary\":\"Done\",\"current_focus\":\"Next\",\"decisions\":[\"A\"],\"completed_tasks\":[\"B\"],\"pending_tasks\":[\"C\"],\"risks\":[\"D\"],\"next_action\":\"Ship\",\"files_touched\":[\"f.go\"]}\n```"

	state, err := parseSummaryState(raw)
	require.NoError(t, err)
	assert.Equal(t, "Done", state.ConversationSummary)
	assert.Equal(t, "Ship", state.NextAction)
	assert.Equal(t, []string{"A"}, state.Decisions)
}

func TestParseSummaryStateMissingRequiredField(t *testing.T) {
	raw := `{"conversation_summary":"ok"}`
	_, err := parseSummaryState(raw)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "next_action is required")
}

func TestBuildRetrievedContextBlockRespectsMinScoreAndSize(t *testing.T) {
	results := []rag.HybridResult{
		{
			Chunk: db.SearchRAGChunksLexicalRow{
				SourceID:    "src-1",
				ExternalID:  "doc-1",
				ChunkIndex:  3,
				Content:     "first chunk is relevant and concise",
				Embedding:   "[]",
				EmbeddingModel: sql.NullString{String: "crowcode-hash-128", Valid: true},
			},
			HybridScore: 0.91,
		},
		{
			Chunk: db.SearchRAGChunksLexicalRow{
				SourceID:    "src-2",
				ExternalID:  "doc-2",
				ChunkIndex:  9,
				Content:     strings.Repeat("very long content ", 60),
				Embedding:   "[]",
				EmbeddingModel: sql.NullString{String: "crowcode-hash-128", Valid: true},
			},
			HybridScore: 0.90,
		},
		{
			Chunk: db.SearchRAGChunksLexicalRow{
				SourceID:    "src-low",
				ExternalID:  "doc-low",
				ChunkIndex:  1,
				Content:     "should be filtered",
				Embedding:   "[]",
				EmbeddingModel: sql.NullString{String: "crowcode-hash-128", Valid: true},
			},
			HybridScore: 0.05,
		},
	}

	block, count := buildRetrievedContextBlock(results, config.RAGConfig{MinScore: 0.2, MaxContextChars: 140})
	require.Equal(t, 1, count)
	assert.Contains(t, block, "Citation policy:")
	assert.Contains(t, block, "[SRC:src-1|DOC:doc-1|CHUNK:3]")
	assert.NotContains(t, block, "src-low")
}
