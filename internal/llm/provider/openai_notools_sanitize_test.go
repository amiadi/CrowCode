package provider

import (
	"testing"

	"github.com/opencode-ai/opencode/internal/message"
)

func TestSanitizeMessagesForNoTools(t *testing.T) {
	t.Parallel()

	msgs := []message.Message{
		{Role: message.User, Parts: []message.ContentPart{message.TextContent{Text: "hello"}}},
		{Role: message.Assistant, Parts: []message.ContentPart{message.ToolCall{ID: "1", Name: "bash", Input: "{}"}}},
		{Role: message.Tool, Parts: []message.ContentPart{message.ToolResult{ToolCallID: "1", Content: "ok"}}},
		{Role: message.Assistant, Parts: []message.ContentPart{message.TextContent{Text: "done"}}},
	}

	got := sanitizeMessagesForNoTools(msgs)
	if len(got) != 2 {
		t.Fatalf("expected 2 messages after sanitize, got %d", len(got))
	}
	if got[0].Role != message.User {
		t.Fatalf("expected first role user, got %s", got[0].Role)
	}
	if got[1].Role != message.Assistant {
		t.Fatalf("expected second role assistant, got %s", got[1].Role)
	}
	if got[1].Content().String() != "done" {
		t.Fatalf("expected assistant content preserved")
	}
}
