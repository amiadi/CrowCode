package tools

import (
	"context"
	"encoding/json"
	"os"
	"path/filepath"
	"testing"

	"github.com/opencode-ai/opencode/internal/config"
	"github.com/opencode-ai/opencode/internal/permission"
	"github.com/opencode-ai/opencode/internal/pubsub"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

type mockPermissionService struct {
	allow       bool
	requestCall int
}

func (m *mockPermissionService) Subscribe(context.Context) <-chan pubsub.Event[permission.PermissionRequest] {
	ch := make(chan pubsub.Event[permission.PermissionRequest])
	close(ch)
	return ch
}

func (m *mockPermissionService) GrantPersistant(permission.PermissionRequest) {}
func (m *mockPermissionService) Grant(permission.PermissionRequest)          {}
func (m *mockPermissionService) Deny(permission.PermissionRequest)           {}
func (m *mockPermissionService) AutoApproveSession(string)                   {}
func (m *mockPermissionService) Request(permission.CreatePermissionRequest) bool {
	m.requestCall++
	return m.allow
}

func ensureTestConfig(t *testing.T, wd string) {
	t.Helper()
	_, err := config.Load(wd, false)
	require.NoError(t, err)
	cfg := config.Get()
	cfg.WorkingDir = wd
}

func TestPythonToolRunPermissionDenied(t *testing.T) {
	tempDir := t.TempDir()
	ensureTestConfig(t, tempDir)

	scriptPath := filepath.Join(tempDir, "tool.sh")
	require.NoError(t, os.WriteFile(scriptPath, []byte("#!/bin/sh\necho hello\n"), 0o755))

	perm := &mockPermissionService{allow: false}
	pt := &pythonTool{
		name:      "test",
		globalCfg: config.PythonConfig{Interpreter: "/bin/sh", DefaultTimeoutMs: 3000, RequirePermission: true},
		toolCfg: config.PythonToolConfig{
			Path: scriptPath,
		},
		permissions: perm,
	}

	resp, err := pt.Run(context.Background(), ToolCall{Name: "python.test", Input: "{}"})
	require.NoError(t, err)
	assert.True(t, resp.IsError)
	assert.Contains(t, resp.Content, "permission denied")
	assert.Equal(t, 1, perm.requestCall)
}

func TestPythonToolRunOverrideNoPermission(t *testing.T) {
	tempDir := t.TempDir()
	ensureTestConfig(t, tempDir)

	scriptPath := filepath.Join(tempDir, "tool.sh")
	require.NoError(t, os.WriteFile(scriptPath, []byte("#!/bin/sh\necho success\n"), 0o755))

	override := false
	perm := &mockPermissionService{allow: false}
	pt := &pythonTool{
		name:      "test",
		globalCfg: config.PythonConfig{Interpreter: "/bin/sh", DefaultTimeoutMs: 3000, RequirePermission: true},
		toolCfg: config.PythonToolConfig{
			Path:              scriptPath,
			RequirePermission: &override,
		},
		permissions: perm,
	}

	payload, err := json.Marshal(PythonParams{Args: []string{"-c", "echo ignored"}})
	require.NoError(t, err)

	resp, err := pt.Run(context.Background(), ToolCall{Name: "python.test", Input: string(payload)})
	require.NoError(t, err)
	assert.False(t, resp.IsError)
	assert.Contains(t, resp.Content, "success")
	assert.Equal(t, 0, perm.requestCall)
}
