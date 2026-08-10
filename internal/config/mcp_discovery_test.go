package config

import (
	"os"
	"path/filepath"
	"testing"
)

func TestParseMCPManifestContentSingle(t *testing.T) {
	t.Parallel()
	content := []byte(`{"name":"coding","type":"stdio","command":"npx","args":["-y","server"]}`)
	parsed := parseMCPManifestContent(content)
	if len(parsed) != 1 {
		t.Fatalf("expected 1 manifest, got %d", len(parsed))
	}
	server, ok := parsed["coding"]
	if !ok {
		t.Fatalf("expected coding entry")
	}
	if server.Command != "npx" {
		t.Fatalf("expected command npx, got %s", server.Command)
	}
}

func TestParseMCPManifestContentNamedMap(t *testing.T) {
	t.Parallel()
	content := []byte(`{"caveman":{"type":"stdio","command":"node","args":["srv.js"]}}`)
	parsed := parseMCPManifestContent(content)
	if len(parsed) != 1 {
		t.Fatalf("expected 1 manifest, got %d", len(parsed))
	}
	if parsed["caveman"].Command != "node" {
		t.Fatalf("expected command node")
	}
}

func TestExpandMCPServerVars(t *testing.T) {
	t.Parallel()
	server := MCPServer{
		Type:    MCPStdio,
		Command: "runner",
		Args:    []string{"${CROWCODE_WORKDIR}", "${OPENCODE_WORKDIR}"},
		Env:     []string{"ROOT=${CROWCODE_WORKDIR}"},
	}
	out := expandMCPServerVars(server, "/tmp/work")
	if out.Args[0] != "/tmp/work" || out.Args[1] != "/tmp/work" {
		t.Fatalf("expected args to expand to /tmp/work, got %#v", out.Args)
	}
	if out.Env[0] != "ROOT=/tmp/work" {
		t.Fatalf("expected env to expand, got %s", out.Env[0])
	}
}

func TestReadMCPServerManifests(t *testing.T) {
	t.Parallel()
	root := t.TempDir()
	manifestDir := filepath.Join(root, "mcp", "servers")
	if err := os.MkdirAll(manifestDir, 0o755); err != nil {
		t.Fatalf("mkdir failed: %v", err)
	}

	good := `{"name":"reverse_engineering","type":"stdio","command":"npx","args":["-y","tool"]}`
	if err := os.WriteFile(filepath.Join(manifestDir, "one.json"), []byte(good), 0o644); err != nil {
		t.Fatalf("write one.json failed: %v", err)
	}

	nestedDir := filepath.Join(manifestDir, "nested")
	if err := os.MkdirAll(nestedDir, 0o755); err != nil {
		t.Fatalf("mkdir nested failed: %v", err)
	}
	mapManifest := `{"coding":{"type":"stdio","command":"node","args":["srv.js"]}}`
	if err := os.WriteFile(filepath.Join(nestedDir, "two.json"), []byte(mapManifest), 0o644); err != nil {
		t.Fatalf("write two.json failed: %v", err)
	}

	entries, err := readMCPServerManifests(manifestDir, "/workspace")
	if err != nil {
		t.Fatalf("read manifests failed: %v", err)
	}
	if len(entries) != 2 {
		t.Fatalf("expected 2 entries, got %d", len(entries))
	}
	if entries["reverse_engineering"].Server.Command != "npx" {
		t.Fatalf("expected reverse_engineering command npx")
	}
	if entries["coding"].Server.Command != "node" {
		t.Fatalf("expected coding command node")
	}
	if entries["coding"].Source != MCPServerSourceManifest {
		t.Fatalf("expected manifest source")
	}
}

func TestReadMCPServerBinaries(t *testing.T) {
	t.Parallel()
	root := t.TempDir()
	binariesDir := filepath.Join(root, "mcp", "bin")
	if err := os.MkdirAll(binariesDir, 0o755); err != nil {
		t.Fatalf("mkdir failed: %v", err)
	}

	goodBin := filepath.Join(binariesDir, "Reverse-Engineering")
	if err := os.WriteFile(goodBin, []byte("#!/bin/sh\necho hi\n"), 0o755); err != nil {
		t.Fatalf("write binary failed: %v", err)
	}

	alsoGood := filepath.Join(binariesDir, "caveman.exe")
	if err := os.WriteFile(alsoGood, []byte("dummy"), 0o644); err != nil {
		t.Fatalf("write windows style binary failed: %v", err)
	}

	notBin := filepath.Join(binariesDir, "manifest.json")
	if err := os.WriteFile(notBin, []byte(`{"name":"ignored"}`), 0o644); err != nil {
		t.Fatalf("write json failed: %v", err)
	}

	entries, err := readMCPServerBinaries(binariesDir)
	if err != nil {
		t.Fatalf("read binaries failed: %v", err)
	}
	if len(entries) != 2 {
		t.Fatalf("expected 2 entries, got %d", len(entries))
	}

	rev, ok := entries["reverse_engineering"]
	if !ok {
		t.Fatalf("expected reverse_engineering entry")
	}
	if rev.Server.Type != MCPStdio {
		t.Fatalf("expected stdio type")
	}
	if rev.Server.Command != goodBin {
		t.Fatalf("expected command path %s, got %s", goodBin, rev.Server.Command)
	}
	if rev.Source != MCPServerSourceBinary {
		t.Fatalf("expected binary source")
	}

	caveman, ok := entries["caveman"]
	if !ok {
		t.Fatalf("expected caveman entry")
	}
	if caveman.Server.Command != alsoGood {
		t.Fatalf("expected command path %s, got %s", alsoGood, caveman.Server.Command)
	}
}

func TestNormalizeMCPServerName(t *testing.T) {
	t.Parallel()
	tests := map[string]string{
		"Reverse Engineering":  "reverse_engineering",
		"caveman-server":       "caveman_server",
		"multi..dots___name":   "multi_dots_name",
		"  weird ### name 123": "weird_name_123",
	}

	for input, expected := range tests {
		got := normalizeMCPServerName(input)
		if got != expected {
			t.Fatalf("input %q expected %q got %q", input, expected, got)
		}
	}
}
