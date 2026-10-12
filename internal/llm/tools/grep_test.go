package tools

import (
	"os"
	"path/filepath"
	"runtime"
	"testing"
)

func TestSearchWithBundledRipgrep(t *testing.T) {
	if runtime.GOARCH != "amd64" || (runtime.GOOS != "linux" && runtime.GOOS != "windows") {
		t.Skip("platform has no bundled tools")
	}
	t.Setenv("PATH", "")
	dir := t.TempDir()
	name := "example.txt"
	if runtime.GOOS != "windows" {
		name = "example:colon.txt"
	}
	path := filepath.Join(dir, name)
	if err := os.WriteFile(path, []byte("first\n-needle: detail\n"), 0600); err != nil {
		t.Fatal(err)
	}
	matches, err := searchWithRipgrep("-needle", dir, "*.txt")
	if err != nil {
		t.Fatal(err)
	}
	if len(matches) != 1 || matches[0].path != path || matches[0].lineNum != 2 || matches[0].lineText != "-needle: detail" {
		t.Fatalf("unexpected matches: %+v", matches)
	}
	matches, err = searchWithRipgrep("missing", dir, "*.txt")
	if err != nil || len(matches) != 0 {
		t.Fatalf("no-match result: %+v, %v", matches, err)
	}
	if runtime.GOOS == "linux" {
		path = filepath.Join(dir, "invalid-\xff.txt")
		if err := os.WriteFile(path, []byte("byte-needle: \xff\n"), 0600); err != nil {
			t.Fatal(err)
		}
		matches, err = searchWithRipgrep("byte-needle", dir, "*.txt")
		if err != nil || len(matches) != 1 || matches[0].path != path || matches[0].lineText != "byte-needle: \xff" {
			t.Fatalf("non-UTF-8 match: %+v, %v", matches, err)
		}
	}
}
