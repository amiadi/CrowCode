package bundledtools

import (
	"bytes"
	"compress/gzip"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
)

func TestBundledTools(t *testing.T) {
	if len(rgData) == 0 {
		t.Skip("platform has no bundled tools")
	}
	t.Setenv("PATH", "")
	if err := Initialize(); err != nil {
		t.Fatal(err)
	}
	defer Cleanup()
	if err := Initialize(); err != nil {
		t.Fatal(err)
	}
	if os.Getenv("PATH") != directory {
		t.Fatalf("bundled directory not on PATH: %q", os.Getenv("PATH"))
	}
	if runtime.GOOS != "windows" {
		info, err := os.Stat(directory)
		if err != nil || info.Mode().Perm() != 0700 {
			t.Fatalf("extraction directory must be private: %v, %v", info, err)
		}
	}
	for _, tool := range []struct {
		name    string
		version string
	}{{"rg", "ripgrep 15.2.0"}, {"fzf", "0.74.5"}} {
		t.Run(tool.name, func(t *testing.T) {
			path, err := LookPath(tool.name)
			if err != nil {
				t.Fatal(err)
			}
			if filepath.Dir(path) != directory {
				t.Fatalf("tool resolved outside private directory: %s", path)
			}
			found, err := exec.LookPath(tool.name)
			if err != nil || found != path {
				t.Fatalf("child PATH resolution: %q, %v", found, err)
			}
			output, err := exec.Command(path, "--version").CombinedOutput()
			if err != nil || !strings.Contains(string(output), tool.version) {
				t.Fatalf("unexpected tool version: %s, %v", output, err)
			}
		})
	}
	Cleanup()
	if _, err := os.Stat(directory); !os.IsNotExist(err) {
		t.Fatalf("temporary directory remains after cleanup: %v", err)
	}
}

func TestExtract(t *testing.T) {
	var compressed bytes.Buffer
	writer := gzip.NewWriter(&compressed)
	if _, err := writer.Write([]byte("payload")); err != nil {
		t.Fatal(err)
	}
	if err := writer.Close(); err != nil {
		t.Fatal(err)
	}
	path := filepath.Join(t.TempDir(), "tool")
	if err := extract(path, compressed.Bytes()); err != nil {
		t.Fatal(err)
	}
	content, err := os.ReadFile(path)
	if err != nil || string(content) != "payload" {
		t.Fatalf("extracted content: %q, %v", content, err)
	}
	if err := extract(path, compressed.Bytes()); err == nil {
		t.Fatal("must not overwrite an existing file")
	}
	if err := extract(filepath.Join(t.TempDir(), "bad"), []byte("not gzip")); err == nil {
		t.Fatal("invalid compressed data accepted")
	}
	data := compressed.Bytes()
	if err := extract(filepath.Join(t.TempDir(), "truncated"), data[:len(data)-4]); err == nil {
		t.Fatal("truncated compressed data accepted")
	}
}
