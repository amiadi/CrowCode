package provider

import "testing"

func TestNormalizeLocalOpenAIBaseURL(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name     string
		input    string
		expected string
	}{
		{name: "empty", input: "", expected: ""},
		{name: "root ollama", input: "http://127.0.0.1:11434", expected: "http://127.0.0.1:11434/v1"},
		{name: "trim slash", input: "http://127.0.0.1:11434/", expected: "http://127.0.0.1:11434/v1"},
		{name: "already v1", input: "http://127.0.0.1:11434/v1", expected: "http://127.0.0.1:11434/v1"},
		{name: "openai path", input: "http://host/openai/v1", expected: "http://host/openai/v1"},
		{name: "api v1 path", input: "http://host/api/v1", expected: "http://host/api/v1"},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			got := normalizeLocalOpenAIBaseURL(tc.input)
			if got != tc.expected {
				t.Fatalf("expected %q got %q", tc.expected, got)
			}
		})
	}
}
