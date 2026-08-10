package cmd

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestNormalizeSharePath(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name        string
		input       string
		contains    []string
		notContains []string
	}{
		{
			name:        "file scheme path",
			input:       "file:///mnt/team-share/docs",
			contains:    []string{"mnt", "team-share", "docs"},
			notContains: []string{"file://", "\\"},
		},
		{
			name:        "smb scheme",
			input:       "smb://server/share/folder",
			contains:    []string{"server", "share", "folder"},
			notContains: []string{"smb://", "\\"},
		},
		{
			name:        "windows unc",
			input:       "\\\\server\\share\\folder",
			contains:    []string{"server", "share", "folder"},
			notContains: []string{"\\"},
		},
		{
			name:        "windows drive style",
			input:       "C:\\team\\share\\docs",
			contains:    []string{"C:", "team", "share", "docs"},
			notContains: []string{"\\"},
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			out := normalizeSharePath(tc.input)
			for _, needle := range tc.contains {
				if !strings.Contains(out, needle) {
					t.Fatalf("expected output %q to contain %q", out, needle)
				}
			}
			for _, needle := range tc.notContains {
				if strings.Contains(out, needle) {
					t.Fatalf("expected output %q to not contain %q", out, needle)
				}
			}
		})
	}
}

func TestNormalizeLocalFilePathFromURI(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name     string
		input    string
		expected string
	}{
		{
			name:     "linux file uri",
			input:    "file:///mnt/team-share/docs",
			expected: "/mnt/team-share/docs",
		},
		{
			name:     "windows drive canonical",
			input:    "file:///C:/Users/alice/docs",
			expected: "C:/Users/alice/docs",
		},
		{
			name:     "windows drive permissive host form",
			input:    "file://C:/Users/alice/docs",
			expected: "C:/Users/alice/docs",
		},
		{
			name:     "windows unc file uri",
			input:    "file://server/share/docs",
			expected: "//server/share/docs",
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			got := normalizeLocalFilePathFromURI(tc.input)
			if got != tc.expected {
				t.Fatalf("expected %q got %q", tc.expected, got)
			}
		})
	}
}

func TestFetchDirectoryContentRecursive(t *testing.T) {
	t.Parallel()

	root := t.TempDir()
	nested := filepath.Join(root, "nested")
	if err := os.MkdirAll(nested, 0o755); err != nil {
		t.Fatalf("mkdir nested: %v", err)
	}
	if err := os.WriteFile(filepath.Join(root, "a.txt"), []byte("alpha"), 0o644); err != nil {
		t.Fatalf("write root text: %v", err)
	}
	if err := os.WriteFile(filepath.Join(nested, "b.txt"), []byte("beta"), 0o644); err != nil {
		t.Fatalf("write nested text: %v", err)
	}
	if err := os.WriteFile(filepath.Join(nested, "binary.bin"), []byte{0, 1, 2, 3}, 0o644); err != nil {
		t.Fatalf("write binary: %v", err)
	}

	content, resolved, err := fetchDirectoryContent(context.Background(), root, true, 100, 1024*1024)
	if err != nil {
		t.Fatalf("fetchDirectoryContent recursive: %v", err)
	}
	if resolved == "" {
		t.Fatalf("expected resolved path")
	}
	if !strings.Contains(content, "# file: a.txt") {
		t.Fatalf("expected root file in content")
	}
	if !strings.Contains(content, "# file: nested/b.txt") {
		t.Fatalf("expected nested file in content")
	}
	if strings.Contains(content, "binary.bin") {
		t.Fatalf("did not expect binary file in content")
	}
}

func TestFetchDirectoryContentNonRecursive(t *testing.T) {
	t.Parallel()

	root := t.TempDir()
	nested := filepath.Join(root, "nested")
	if err := os.MkdirAll(nested, 0o755); err != nil {
		t.Fatalf("mkdir nested: %v", err)
	}
	if err := os.WriteFile(filepath.Join(root, "a.txt"), []byte("alpha"), 0o644); err != nil {
		t.Fatalf("write root text: %v", err)
	}
	if err := os.WriteFile(filepath.Join(nested, "b.txt"), []byte("beta"), 0o644); err != nil {
		t.Fatalf("write nested text: %v", err)
	}

	content, _, err := fetchDirectoryContent(context.Background(), root, false, 100, 1024*1024)
	if err != nil {
		t.Fatalf("fetchDirectoryContent non-recursive: %v", err)
	}
	if !strings.Contains(content, "# file: a.txt") {
		t.Fatalf("expected root file in content")
	}
	if strings.Contains(content, "nested/b.txt") {
		t.Fatalf("did not expect nested file in non-recursive mode")
	}
}
