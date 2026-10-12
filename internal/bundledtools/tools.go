package bundledtools

import (
	"bytes"
	"compress/gzip"
	_ "embed"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"sync"
)

//go:embed assets/THIRD_PARTY_NOTICES.txt
var Notices string

var (
	once       sync.Once
	directory  string
	setupError error
)

// Initialize extracts tools into a private per-process directory, never a shared cache.
func Initialize() error {
	once.Do(func() {
		if len(rgData) == 0 {
			return
		}
		directory, setupError = os.MkdirTemp("", "crowcode-tools-")
		if setupError != nil {
			return
		}
		for _, tool := range []struct {
			name string
			data []byte
		}{{"rg", rgData}, {"fzf", fzfData}} {
			name := tool.name
			if runtime.GOOS == "windows" {
				name += ".exe"
			}
			if setupError = extract(filepath.Join(directory, name), tool.data); setupError != nil {
				Cleanup()
				return
			}
		}
		path := directory
		if existing := os.Getenv("PATH"); existing != "" {
			path += string(os.PathListSeparator) + existing
		}
		setupError = os.Setenv("PATH", path)
		if setupError != nil {
			Cleanup()
		}
	})
	return setupError
}

func extract(path string, data []byte) error {
	reader, err := gzip.NewReader(bytes.NewReader(data))
	if err != nil {
		return fmt.Errorf("read bundled tool: %w", err)
	}
	defer reader.Close()
	file, err := os.OpenFile(path, os.O_WRONLY|os.O_CREATE|os.O_EXCL, 0700)
	if err != nil {
		return fmt.Errorf("create bundled tool: %w", err)
	}
	_, copyErr := io.Copy(file, reader)
	closeErr := file.Close()
	if copyErr != nil {
		return fmt.Errorf("extract bundled tool: %w", copyErr)
	}
	return closeErr
}

// LookPath prefers the bundled executables on supported platforms.
func LookPath(name string) (string, error) {
	if err := Initialize(); err != nil {
		return "", err
	}
	if directory != "" && (name == "rg" || name == "fzf") {
		if runtime.GOOS == "windows" {
			name += ".exe"
		}
		return filepath.Join(directory, name), nil
	}
	return exec.LookPath(name)
}

// Cleanup is called after all application subprocesses have stopped.
func Cleanup() {
	if directory != "" {
		_ = os.RemoveAll(directory)
	}
}
