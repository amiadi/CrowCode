package tools

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"sort"
	"strings"
	"time"

	"github.com/opencode-ai/opencode/internal/config"
	"github.com/opencode-ai/opencode/internal/permission"
)

type PythonParams struct {
	Args  []string `json:"args"`
	Input string   `json:"input"`
}

type PythonPermissionsParams struct {
	Script string   `json:"script"`
	Args   []string `json:"args"`
}

type pythonTool struct {
	name        string
	globalCfg   config.PythonConfig
	toolCfg     config.PythonToolConfig
	permissions permission.Service
}

func NewPythonTool(name string, globalCfg config.PythonConfig, toolCfg config.PythonToolConfig, permissions permission.Service) BaseTool {
	return &pythonTool{
		name:        name,
		globalCfg:   globalCfg,
		toolCfg:     toolCfg,
		permissions: permissions,
	}
}

func NewConfiguredPythonTools(cfg *config.Config, permissions permission.Service) []BaseTool {
	if cfg == nil || !cfg.Python.Enabled || len(cfg.Python.Tools) == 0 {
		return nil
	}

	keys := make([]string, 0, len(cfg.Python.Tools))
	for name := range cfg.Python.Tools {
		keys = append(keys, name)
	}
	sort.Strings(keys)

	out := make([]BaseTool, 0, len(keys))
	for _, name := range keys {
		out = append(out, NewPythonTool(name, cfg.Python, cfg.Python.Tools[name], permissions))
	}
	return out
}

func (p *pythonTool) Info() ToolInfo {
	return ToolInfo{
		Name:        "python." + p.name,
		Description: fmt.Sprintf("Runs configured Python script '%s'", p.name),
		Parameters: map[string]any{
			"args": map[string]any{
				"type":        "array",
				"description": "Optional runtime arguments appended to configured script arguments",
				"items": map[string]any{
					"type": "string",
				},
			},
			"input": map[string]any{
				"type":        "string",
				"description": "Optional stdin text passed to the script",
			},
		},
		Required: []string{},
	}
}

func (p *pythonTool) Run(ctx context.Context, call ToolCall) (ToolResponse, error) {
	var params PythonParams
	if strings.TrimSpace(call.Input) != "" {
		if err := json.Unmarshal([]byte(call.Input), &params); err != nil {
			return NewTextErrorResponse(fmt.Sprintf("invalid parameters: %s", err)), nil
		}
	}

	scriptPath := p.toolCfg.Path
	if !filepath.IsAbs(scriptPath) {
		scriptPath = filepath.Join(config.WorkingDirectory(), scriptPath)
	}
	if strings.TrimSpace(scriptPath) == "" {
		return NewTextErrorResponse("python tool script path is empty"), nil
	}
	if _, err := os.Stat(scriptPath); err != nil {
		return NewTextErrorResponse(fmt.Sprintf("python script not found: %s", scriptPath)), nil
	}

	sessionID, _ := GetContextValues(ctx)
	if p.requiresPermission() {
		description := fmt.Sprintf("run python tool %s", p.name)
		allowed := p.permissions.Request(permission.CreatePermissionRequest{
			SessionID:   sessionID,
			ToolName:    "python." + p.name,
			Description: description,
			Action:      "execute",
			Path:        scriptPath,
			Params: PythonPermissionsParams{
				Script: scriptPath,
				Args:   append(append([]string{}, p.toolCfg.Args...), params.Args...),
			},
		})
		if !allowed {
			return NewTextErrorResponse("permission denied"), nil
		}
	}

	timeout := p.toolCfg.TimeoutMs
	if timeout <= 0 {
		timeout = p.globalCfg.DefaultTimeoutMs
	}
	if timeout <= 0 {
		timeout = 60000
	}

	runCtx, cancel := context.WithTimeout(ctx, time.Duration(timeout)*time.Millisecond)
	defer cancel()

	interpreter := strings.TrimSpace(p.globalCfg.Interpreter)
	if interpreter == "" {
		interpreter = config.DefaultPythonInterpreter()
	}
	args := make([]string, 0, 1+len(p.toolCfg.Args)+len(params.Args))
	args = append(args, scriptPath)
	args = append(args, p.toolCfg.Args...)
	args = append(args, params.Args...)

	cmd := exec.CommandContext(runCtx, interpreter, args...)
	cmd.Dir = config.WorkingDirectory()
	cmd.Env = os.Environ()
	for k, v := range p.toolCfg.Env {
		cmd.Env = append(cmd.Env, fmt.Sprintf("%s=%s", k, v))
	}
	if params.Input != "" {
		cmd.Stdin = strings.NewReader(params.Input)
	}

	var stdout, stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr

	err := cmd.Run()
	if runCtx.Err() == context.DeadlineExceeded {
		return NewTextErrorResponse(fmt.Sprintf("python tool timed out after %dms", timeout)), nil
	}
	if err != nil {
		output := strings.TrimSpace(stderr.String())
		if output == "" {
			output = strings.TrimSpace(stdout.String())
		}
		if output == "" {
			output = err.Error()
		}
		return NewTextErrorResponse(output), nil
	}

	output := strings.TrimSpace(stdout.String())
	if output == "" {
		output = "python tool executed successfully with no output"
	}
	return NewTextResponse(output), nil
}

func (p *pythonTool) requiresPermission() bool {
	if p.toolCfg.RequirePermission != nil {
		return *p.toolCfg.RequirePermission
	}
	return p.globalCfg.RequirePermission
}
