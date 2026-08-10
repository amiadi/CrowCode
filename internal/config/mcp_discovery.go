package config

import (
	"embed"
	"encoding/json"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"runtime"
	"sort"
	"strings"
	"unicode"

	"github.com/spf13/viper"
)

//go:embed mcp_builtin/*.json
var builtinMCPManifests embed.FS

type mcpManifest struct {
	Name    string            `json:"name"`
	Command string            `json:"command"`
	Env     []string          `json:"env"`
	Args    []string          `json:"args"`
	Type    MCPType           `json:"type"`
	URL     string            `json:"url"`
	Headers map[string]string `json:"headers"`
}

type mcpDiscoveryLayout struct {
	ManifestDirs []string
	BinaryDirs   []string
}

type MCPServerSource string

const (
	MCPServerSourceExplicit MCPServerSource = "explicit"
	MCPServerSourceManifest MCPServerSource = "manifest"
	MCPServerSourceBinary   MCPServerSource = "binary"
)

type MCPServerResolved struct {
	Name   string
	Server MCPServer
	Source MCPServerSource
	Path   string
}

type discoveredMCPEntry struct {
	Server MCPServer
	Source MCPServerSource
	Path   string
}

func mergeAutoDiscoveredMCPServers(cfg *Config) error {
	if cfg == nil {
		return fmt.Errorf("config is nil")
	}

	explicit := cloneMCPServers(cfg.MCPServers)
	discovered, err := discoverMCPServersWithSource(cfg)
	if err != nil {
		return err
	}

	cfg.MCPServers = map[string]MCPServer{}
	for name, entry := range discovered {
		cfg.MCPServers[name] = entry.Server
	}
	for name, server := range explicit {
		cfg.MCPServers[name] = server
	}
	return nil
}

func discoverMCPServers(cfg *Config) (map[string]MCPServer, error) {
	entries, err := discoverMCPServersWithSource(cfg)
	if err != nil {
		return nil, err
	}
	servers := map[string]MCPServer{}
	for name, entry := range entries {
		servers[name] = entry.Server
	}
	return servers, nil
}

func discoverMCPServersWithSource(cfg *Config) (map[string]discoveredMCPEntry, error) {
	servers := map[string]discoveredMCPEntry{}
	layout, err := mcpDiscoveryDirs(cfg)
	if err != nil {
		return nil, err
	}

	// Load binaries first, then manifests so JSON manifests can override discovered binaries.
	for _, dir := range layout.BinaryDirs {
		entries, err := readMCPServerBinaries(dir)
		if err != nil {
			continue
		}
		for name, entry := range entries {
			servers[name] = entry
		}
	}

	for _, dir := range layout.ManifestDirs {
		entries, err := readMCPServerManifests(dir, cfg.WorkingDir)
		if err != nil {
			continue
		}
		for name, entry := range entries {
			servers[name] = entry
		}
	}

	return servers, nil
}

func ResolveMCPServers(cfg *Config) ([]MCPServerResolved, error) {
	if cfg == nil {
		return nil, fmt.Errorf("config is nil")
	}

	explicit, _ := loadExplicitMCPServersFromConfigFile()
	discovered, err := discoverMCPServersWithSource(cfg)
	if err != nil {
		return nil, err
	}

	effective := map[string]MCPServerResolved{}
	for name, entry := range discovered {
		effective[name] = MCPServerResolved{
			Name:   name,
			Server: entry.Server,
			Source: entry.Source,
			Path:   entry.Path,
		}
	}

	for name, server := range explicit {
		effective[name] = MCPServerResolved{
			Name:   name,
			Server: server,
			Source: MCPServerSourceExplicit,
			Path:   viperConfigPathOrDefault(),
		}
	}

	names := make([]string, 0, len(effective))
	for name := range effective {
		names = append(names, name)
	}
	sort.Strings(names)

	resolved := make([]MCPServerResolved, 0, len(names))
	for _, name := range names {
		resolved = append(resolved, effective[name])
	}

	return resolved, nil
}

func mcpDiscoveryDirs(cfg *Config) (mcpDiscoveryLayout, error) {
	dataDir := cfg.Data.Directory
	if !filepath.IsAbs(dataDir) {
		dataDir = filepath.Join(cfg.WorkingDir, dataDir)
	}

	builtinDir, err := ensureBuiltinMCPManifestDir(dataDir)
	if err != nil {
		return mcpDiscoveryLayout{}, err
	}

	manifestDirs := []string{builtinDir}
	binaryDirs := []string{
		filepath.Join(dataDir, "mcp", "servers"),
		filepath.Join(dataDir, "mcp", "bin"),
	}

	if xdg := strings.TrimSpace(os.Getenv("XDG_CONFIG_HOME")); xdg != "" {
		manifestDirs = append(manifestDirs, filepath.Join(xdg, appName, "mcp", "servers"))
		binaryDirs = append(binaryDirs,
			filepath.Join(xdg, appName, "mcp", "servers"),
			filepath.Join(xdg, appName, "mcp", "bin"),
		)
	}

	home, err := os.UserHomeDir()
	if err == nil && strings.TrimSpace(home) != "" {
		manifestDirs = append(manifestDirs, filepath.Join(home, fmt.Sprintf(".%s", appName), "mcp", "servers"))
		binaryDirs = append(binaryDirs,
			filepath.Join(home, fmt.Sprintf(".%s", appName), "mcp", "servers"),
			filepath.Join(home, fmt.Sprintf(".%s", appName), "mcp", "bin"),
		)
	}

	workspaceMCPDir := filepath.Join(cfg.WorkingDir, fmt.Sprintf(".%s", appName), "mcp")
	manifestDirs = append(manifestDirs, filepath.Join(workspaceMCPDir, "servers"))
	binaryDirs = append(binaryDirs,
		filepath.Join(workspaceMCPDir, "servers"),
		filepath.Join(workspaceMCPDir, "bin"),
	)

	return mcpDiscoveryLayout{
		ManifestDirs: uniquePaths(manifestDirs),
		BinaryDirs:   uniquePaths(binaryDirs),
	}, nil
}

func ensureBuiltinMCPManifestDir(dataDir string) (string, error) {
	builtinDir := filepath.Join(dataDir, "mcp", "servers", "builtin")
	if err := os.MkdirAll(builtinDir, 0o755); err != nil {
		return "", err
	}

	entries, err := fs.ReadDir(builtinMCPManifests, "mcp_builtin")
	if err != nil {
		return "", err
	}

	for _, entry := range entries {
		if entry.IsDir() || !strings.HasSuffix(strings.ToLower(entry.Name()), ".json") {
			continue
		}
		content, err := fs.ReadFile(builtinMCPManifests, filepath.ToSlash(filepath.Join("mcp_builtin", entry.Name())))
		if err != nil {
			return "", err
		}
		target := filepath.Join(builtinDir, entry.Name())
		if _, err := os.Stat(target); err == nil {
			continue
		}
		if writeErr := os.WriteFile(target, content, 0o644); writeErr != nil {
			return "", writeErr
		}
	}

	return builtinDir, nil
}

func readMCPServerManifests(rootDir, workingDir string) (map[string]discoveredMCPEntry, error) {
	entries := map[string]discoveredMCPEntry{}
	stat, err := os.Stat(rootDir)
	if err != nil {
		return entries, err
	}
	if !stat.IsDir() {
		return entries, fmt.Errorf("not a directory: %s", rootDir)
	}

	_ = filepath.WalkDir(rootDir, func(path string, d fs.DirEntry, walkErr error) error {
		if walkErr != nil {
			return nil
		}
		if d.IsDir() {
			return nil
		}
		if !strings.HasSuffix(strings.ToLower(d.Name()), ".json") {
			return nil
		}

		content, readErr := os.ReadFile(path)
		if readErr != nil {
			return nil
		}
		parsed := parseMCPManifestContent(content)
		for name, server := range parsed {
			if !isValidMCPServer(server) {
				continue
			}
			entries[name] = discoveredMCPEntry{
				Server: expandMCPServerVars(server, workingDir),
				Source: MCPServerSourceManifest,
				Path:   path,
			}
		}
		return nil
	})

	return entries, nil
}

func readMCPServerBinaries(rootDir string) (map[string]discoveredMCPEntry, error) {
	entries := map[string]discoveredMCPEntry{}
	stat, err := os.Stat(rootDir)
	if err != nil {
		return entries, err
	}
	if !stat.IsDir() {
		return entries, fmt.Errorf("not a directory: %s", rootDir)
	}

	_ = filepath.WalkDir(rootDir, func(path string, d fs.DirEntry, walkErr error) error {
		if walkErr != nil {
			return nil
		}
		if d.IsDir() {
			return nil
		}
		if strings.HasSuffix(strings.ToLower(d.Name()), ".json") {
			return nil
		}

		info, infoErr := d.Info()
		if infoErr != nil {
			return nil
		}
		if !isExecutableCandidate(path, info) {
			return nil
		}

		name := serverNameFromBinaryPath(path)
		if name == "" {
			return nil
		}

		entries[name] = discoveredMCPEntry{
			Source: MCPServerSourceBinary,
			Path:   path,
			Server: MCPServer{
				Type:    MCPStdio,
				Command: path,
				Args:    []string{},
				Env:     []string{},
			},
		}
		return nil
	})

	return entries, nil
}

func isExecutableCandidate(path string, info fs.FileInfo) bool {
	if info.Mode().IsRegular() {
		if info.Mode()&0o111 != 0 {
			return true
		}
	}

	lowerExt := strings.ToLower(filepath.Ext(path))
	switch lowerExt {
	case ".exe", ".bat", ".cmd", ".ps1", ".sh", ".run", ".bin":
		return true
	default:
		// On Unix-like systems, executable bit is the primary signal.
		// On Windows, extension is often the only available signal.
		return runtime.GOOS == "windows" && (lowerExt == ".com")
	}
}

func serverNameFromBinaryPath(path string) string {
	base := filepath.Base(path)
	ext := filepath.Ext(base)
	trimmed := base
	lowerExt := strings.ToLower(ext)
	switch lowerExt {
	case ".exe", ".bat", ".cmd", ".ps1", ".sh", ".run", ".bin", ".com":
		trimmed = strings.TrimSuffix(base, ext)
	}
	return normalizeMCPServerName(trimmed)
}

func normalizeMCPServerName(raw string) string {
	raw = strings.TrimSpace(raw)
	if raw == "" {
		return ""
	}
	buf := strings.Builder{}
	lastUnderscore := false
	for _, r := range raw {
		switch {
		case unicode.IsLetter(r) || unicode.IsDigit(r):
			buf.WriteRune(unicode.ToLower(r))
			lastUnderscore = false
		case r == '_' || r == '-' || r == '.' || unicode.IsSpace(r):
			if !lastUnderscore {
				buf.WriteRune('_')
				lastUnderscore = true
			}
		}
	}
	result := strings.Trim(buf.String(), "_")
	return result
}

func parseMCPManifestContent(content []byte) map[string]MCPServer {
	manifest := map[string]MCPServer{}

	var single mcpManifest
	if err := json.Unmarshal(content, &single); err == nil {
		if strings.TrimSpace(single.Name) != "" {
			manifest[single.Name] = MCPServer{
				Command: single.Command,
				Env:     single.Env,
				Args:    single.Args,
				Type:    single.Type,
				URL:     single.URL,
				Headers: single.Headers,
			}
			return manifest
		}
	}

	var named map[string]MCPServer
	if err := json.Unmarshal(content, &named); err == nil {
		for name, server := range named {
			manifest[name] = server
		}
	}

	return manifest
}

func isValidMCPServer(server MCPServer) bool {
	typ := server.Type
	if typ == "" {
		typ = MCPStdio
	}
	switch typ {
	case MCPStdio:
		return strings.TrimSpace(server.Command) != ""
	case MCPSse:
		return strings.TrimSpace(server.URL) != ""
	default:
		return false
	}
}

func expandMCPServerVars(server MCPServer, workingDir string) MCPServer {
	replacer := strings.NewReplacer(
		"${CROWCODE_WORKDIR}", workingDir,
		"${OPENCODE_WORKDIR}", workingDir,
	)

	server.Command = replacer.Replace(server.Command)
	server.URL = replacer.Replace(server.URL)
	for i, arg := range server.Args {
		server.Args[i] = replacer.Replace(arg)
	}
	for i, env := range server.Env {
		server.Env[i] = replacer.Replace(env)
	}
	if server.Type == "" {
		server.Type = MCPStdio
	}
	return server
}

func cloneMCPServers(input map[string]MCPServer) map[string]MCPServer {
	out := map[string]MCPServer{}
	for name, server := range input {
		out[name] = server
	}
	return out
}

func uniquePaths(paths []string) []string {
	seen := map[string]struct{}{}
	out := make([]string, 0, len(paths))
	for _, p := range paths {
		n := filepath.Clean(strings.TrimSpace(p))
		if n == "" {
			continue
		}
		if _, ok := seen[n]; ok {
			continue
		}
		seen[n] = struct{}{}
		out = append(out, n)
	}
	sort.Strings(out)
	return out
}

func loadExplicitMCPServersFromConfigFile() (map[string]MCPServer, error) {
	path := viperConfigPathOrDefault()
	content, err := os.ReadFile(path)
	if err != nil {
		if os.IsNotExist(err) {
			return map[string]MCPServer{}, nil
		}
		return nil, err
	}

	var payload struct {
		MCPServers map[string]MCPServer `json:"mcpServers"`
	}
	if err := json.Unmarshal(content, &payload); err != nil {
		return map[string]MCPServer{}, nil
	}
	if payload.MCPServers == nil {
		return map[string]MCPServer{}, nil
	}
	return payload.MCPServers, nil
}

func viperConfigPathOrDefault() string {
	path := viper.ConfigFileUsed()
	if strings.TrimSpace(path) != "" {
		return path
	}
	home, err := os.UserHomeDir()
	if err != nil {
		return ""
	}
	return filepath.Join(home, fmt.Sprintf(".%s.json", appName))
}
