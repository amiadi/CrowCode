// Package config manages application configuration from various sources.
package config

import (
	"encoding/json"
	"fmt"
	"log/slog"
	"os"
	"path/filepath"
	"runtime"
	"sort"
	"strings"

	"github.com/opencode-ai/opencode/internal/llm/models"
	"github.com/opencode-ai/opencode/internal/logging"
	"github.com/spf13/viper"
)

// MCPType defines the type of MCP (Model Control Protocol) server.
type MCPType string

// Supported MCP types
const (
	MCPStdio MCPType = "stdio"
	MCPSse   MCPType = "sse"
)

// MCPServer defines the configuration for a Model Control Protocol server.
type MCPServer struct {
	Command string            `json:"command"`
	Env     []string          `json:"env"`
	Args    []string          `json:"args"`
	Type    MCPType           `json:"type"`
	URL     string            `json:"url"`
	Headers map[string]string `json:"headers"`
}

type AgentName string

const (
	AgentCoder      AgentName = "coder"
	AgentSummarizer AgentName = "summarizer"
	AgentTask       AgentName = "task"
	AgentTitle      AgentName = "title"
)

// Agent defines configuration for different LLM models and their token limits.
type Agent struct {
	Model           models.ModelID `json:"model"`
	MaxTokens       int64          `json:"maxTokens"`
	ReasoningEffort string         `json:"reasoningEffort"` // For openai models low,medium,heigh
	Temperature     *float64       `json:"temperature,omitempty"`
	TopP            *float64       `json:"topP,omitempty"`
	MinP            *float64       `json:"minP,omitempty"`
	TopK            *int64         `json:"topK,omitempty"`
	NumCtx          *int64         `json:"numCtx,omitempty"`
	RepeatPenalty   *float64       `json:"repeatPenalty,omitempty"`
	Seed            *int64         `json:"seed,omitempty"`
}

// Provider defines configuration for an LLM provider.
type Provider struct {
	APIKey   string `json:"apiKey"`
	Disabled bool   `json:"disabled"`
}

// Data defines storage configuration.
type Data struct {
	Directory string `json:"directory,omitempty"`
}

// LSPConfig defines configuration for Language Server Protocol integration.
type LSPConfig struct {
	Disabled bool     `json:"enabled"`
	Command  string   `json:"command"`
	Args     []string `json:"args"`
	Options  any      `json:"options"`
}

// TUIConfig defines the configuration for the Terminal User Interface.
type TUIConfig struct {
	Theme string `json:"theme,omitempty"`
}

// ShellConfig defines the configuration for the shell used by the bash tool.
type ShellConfig struct {
	Path string   `json:"path,omitempty"`
	Args []string `json:"args,omitempty"`
}

type RAGEmbeddingsConfig struct {
	Provider   string `json:"provider,omitempty"`
	Model      string `json:"model,omitempty"`
	Dimensions int    `json:"dimensions,omitempty"`
	TimeoutMs  int    `json:"timeoutMs,omitempty"`
}

type RAGConfig struct {
	Enabled         bool                `json:"enabled,omitempty"`
	TopK            int64               `json:"topK,omitempty"`
	MinScore        float64             `json:"minScore,omitempty"`
	MaxContextChars int                 `json:"maxContextChars,omitempty"`
	Embeddings      RAGEmbeddingsConfig `json:"embeddings,omitempty"`
}

type PythonToolConfig struct {
	Path              string            `json:"path"`
	Args              []string          `json:"args,omitempty"`
	Env               map[string]string `json:"env,omitempty"`
	TimeoutMs         int               `json:"timeoutMs,omitempty"`
	RequirePermission *bool             `json:"requirePermission,omitempty"`
}

type PythonConfig struct {
	Enabled           bool                        `json:"enabled,omitempty"`
	Interpreter       string                      `json:"interpreter,omitempty"`
	DefaultTimeoutMs  int                         `json:"defaultTimeoutMs,omitempty"`
	RequirePermission bool                        `json:"requirePermission,omitempty"`
	Tools             map[string]PythonToolConfig `json:"tools,omitempty"`
}

// Config is the main configuration structure for the application.
type Config struct {
	Data          Data                              `json:"data"`
	WorkingDir    string                            `json:"wd,omitempty"`
	LocalEndpoint string                            `json:"localEndpoint,omitempty"`
	MCPServers    map[string]MCPServer              `json:"mcpServers,omitempty"`
	Providers     map[models.ModelProvider]Provider `json:"providers,omitempty"`
	LSP           map[string]LSPConfig              `json:"lsp,omitempty"`
	Agents        map[AgentName]Agent               `json:"agents,omitempty"`
	Debug         bool                              `json:"debug,omitempty"`
	DebugLSP      bool                              `json:"debugLSP,omitempty"`
	ContextPaths  []string                          `json:"contextPaths,omitempty"`
	TUI           TUIConfig                         `json:"tui"`
	Shell         ShellConfig                       `json:"shell,omitempty"`
	RAG           RAGConfig                         `json:"rag,omitempty"`
	Python        PythonConfig                      `json:"python,omitempty"`
	AutoCompact   bool                              `json:"autoCompact,omitempty"`
	LocalOnly     bool                              `json:"localOnly,omitempty"`
}

// Application constants
const (
	defaultDataDirectory = ".crowcode"
	defaultLogLevel      = "info"
	appName              = "crowcode"

	MaxTokensFallbackDefault = 4096
)

var defaultContextPaths = []string{
	".github/copilot-instructions.md",
	".cursorrules",
	".cursor/rules/",
	"CLAUDE.md",
	"CLAUDE.local.md",
	"crowcode.md",
	"crowcode.local.md",
	"CrowCode.md",
	"CrowCode.local.md",
	"CROWCODE.md",
	"CROWCODE.local.md",
}

// Global configuration instance
var cfg *Config

// Load initializes the configuration from environment variables and config files.
// If debug is true, debug mode is enabled and log level is set to debug.
// It returns an error if configuration loading fails.
func Load(workingDir string, debug bool) (*Config, error) {
	if cfg != nil {
		return cfg, nil
	}

	cfg = &Config{
		WorkingDir: workingDir,
		MCPServers: make(map[string]MCPServer),
		Providers:  make(map[models.ModelProvider]Provider),
		LSP:        make(map[string]LSPConfig),
	}

	configureViper()
	setDefaults(debug)

	// Read global config
	if err := readConfig(viper.ReadInConfig()); err != nil {
		return cfg, err
	}

	// Load and merge local config
	mergeLocalConfig(workingDir)

	setProviderDefaults()

	// Apply configuration to the struct
	if err := viper.Unmarshal(cfg); err != nil {
		return cfg, fmt.Errorf("failed to unmarshal config: %w", err)
	}

	if err := mergeAutoDiscoveredMCPServers(cfg); err != nil {
		logging.Warn("failed to auto-discover mcp servers", "error", err)
	}

	applyDefaultValues()
	defaultLevel := slog.LevelInfo
	if cfg.Debug {
		defaultLevel = slog.LevelDebug
	}
	if os.Getenv("OPENCODE_DEV_DEBUG") == "true" {
		loggingFile := fmt.Sprintf("%s/%s", cfg.Data.Directory, "debug.log")
		messagesPath := fmt.Sprintf("%s/%s", cfg.Data.Directory, "messages")

		// if file does not exist create it
		if _, err := os.Stat(loggingFile); os.IsNotExist(err) {
			if err := os.MkdirAll(cfg.Data.Directory, 0o755); err != nil {
				return cfg, fmt.Errorf("failed to create directory: %w", err)
			}
			if _, err := os.Create(loggingFile); err != nil {
				return cfg, fmt.Errorf("failed to create log file: %w", err)
			}
		}

		if _, err := os.Stat(messagesPath); os.IsNotExist(err) {
			if err := os.MkdirAll(messagesPath, 0o756); err != nil {
				return cfg, fmt.Errorf("failed to create directory: %w", err)
			}
		}
		logging.MessageDir = messagesPath

		sloggingFileWriter, err := os.OpenFile(loggingFile, os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0o666)
		if err != nil {
			return cfg, fmt.Errorf("failed to open log file: %w", err)
		}
		// Configure logger
		logger := slog.New(slog.NewTextHandler(sloggingFileWriter, &slog.HandlerOptions{
			Level: defaultLevel,
		}))
		slog.SetDefault(logger)
	} else {
		// Configure logger
		logger := slog.New(slog.NewTextHandler(logging.NewWriter(), &slog.HandlerOptions{
			Level: defaultLevel,
		}))
		slog.SetDefault(logger)
	}

	// Validate configuration
	if err := Validate(); err != nil {
		return cfg, fmt.Errorf("config validation failed: %w", err)
	}

	if cfg.Agents == nil {
		cfg.Agents = make(map[AgentName]Agent)
	}

	// Override the max tokens for title agent
	cfg.Agents[AgentTitle] = Agent{
		Model:     cfg.Agents[AgentTitle].Model,
		MaxTokens: 80,
	}
	return cfg, nil
}

// configureViper sets up viper's configuration paths and environment variables.
func configureViper() {
	viper.SetConfigName(fmt.Sprintf(".%s", appName))
	viper.SetConfigType("json")
	viper.AddConfigPath("$HOME")
	viper.AddConfigPath(fmt.Sprintf("$XDG_CONFIG_HOME/%s", appName))
	viper.AddConfigPath(fmt.Sprintf("$HOME/.config/%s", appName))
	viper.SetEnvPrefix(strings.ToUpper(appName))
	viper.AutomaticEnv()
}

// setDefaults configures default values for configuration options.
func setDefaults(debug bool) {
	viper.SetDefault("data.directory", defaultDataDirectory)
	viper.SetDefault("contextPaths", defaultContextPaths)
	viper.SetDefault("tui.theme", "crowcode")
	viper.SetDefault("autoCompact", true)
	viper.SetDefault("localEndpoint", os.Getenv("LOCAL_ENDPOINT"))
	viper.SetDefault("localOnly", false)
	viper.SetDefault("rag.enabled", true)
	viper.SetDefault("rag.topK", 5)
	viper.SetDefault("rag.minScore", 0.2)
	viper.SetDefault("rag.maxContextChars", 3200)
	viper.SetDefault("rag.embeddings.provider", "ollama")
	viper.SetDefault("rag.embeddings.model", "nomic-embed-text")
	viper.SetDefault("rag.embeddings.dimensions", 768)
	viper.SetDefault("rag.embeddings.timeoutMs", 15000)
	viper.SetDefault("python.enabled", false)
	viper.SetDefault("python.interpreter", DefaultPythonInterpreter())
	viper.SetDefault("python.defaultTimeoutMs", 60000)
	viper.SetDefault("python.requirePermission", true)

	// Set default shell from environment or fallback to /bin/bash
	shellPath := os.Getenv("SHELL")
	if shellPath == "" {
		shellPath = "/bin/bash"
	}
	viper.SetDefault("shell.path", shellPath)
	viper.SetDefault("shell.args", []string{"-l"})

	if debug {
		viper.SetDefault("debug", true)
		viper.Set("log.level", "debug")
	} else {
		viper.SetDefault("debug", false)
		viper.SetDefault("log.level", defaultLogLevel)
	}
}

func DefaultPythonInterpreter() string {
	if runtime.GOOS == "windows" {
		return "python"
	}
	return "python3"
}

// setProviderDefaults configures LLM provider defaults based on provider provided by
// environment variables and configuration file.
func setProviderDefaults() {
	if endpoint := strings.TrimSpace(viper.GetString("localEndpoint")); endpoint != "" {
		models.BootstrapLocalModels(endpoint)
	}

	// Set all API keys we can find in the environment
	// Note: Viper does not default if the json apiKey is ""
	if apiKey := os.Getenv("ANTHROPIC_API_KEY"); apiKey != "" {
		viper.SetDefault("providers.anthropic.apiKey", apiKey)
	}
	if apiKey := os.Getenv("OPENAI_API_KEY"); apiKey != "" {
		viper.SetDefault("providers.openai.apiKey", apiKey)
	}
	if apiKey := os.Getenv("GEMINI_API_KEY"); apiKey != "" {
		viper.SetDefault("providers.gemini.apiKey", apiKey)
	}
	if apiKey := os.Getenv("GROQ_API_KEY"); apiKey != "" {
		viper.SetDefault("providers.groq.apiKey", apiKey)
	}
	if apiKey := os.Getenv("OPENROUTER_API_KEY"); apiKey != "" {
		viper.SetDefault("providers.openrouter.apiKey", apiKey)
	}
	if apiKey := os.Getenv("XAI_API_KEY"); apiKey != "" {
		viper.SetDefault("providers.xai.apiKey", apiKey)
	}
	if apiKey := os.Getenv("AZURE_OPENAI_ENDPOINT"); apiKey != "" {
		// api-key may be empty when using Entra ID credentials – that's okay
		viper.SetDefault("providers.azure.apiKey", os.Getenv("AZURE_OPENAI_API_KEY"))
	}
	if apiKey, err := LoadGitHubToken(); err == nil && apiKey != "" {
		viper.SetDefault("providers.copilot.apiKey", apiKey)
		if viper.GetString("providers.copilot.apiKey") == "" {
			viper.Set("providers.copilot.apiKey", apiKey)
		}
	}

	// Use this order to set the default models
	// 0. Local (when configured through LOCAL_ENDPOINT bootstrap)
	// 1. Copilot
	// 2. Anthropic
	// 3. OpenAI
	// 4. Google Gemini
	// 5. Groq
	// 6. OpenRouter
	// 7. AWS Bedrock
	// 8. Azure
	// 9. Google Cloud VertexAI

	// local configuration (set by local model bootstrap)
	if key := viper.GetString("providers.local.apiKey"); strings.TrimSpace(key) != "" {
		// local models are discovered dynamically and defaults are set in loadLocalModels
		return
	}

	// copilot configuration
	if key := viper.GetString("providers.copilot.apiKey"); strings.TrimSpace(key) != "" {
		viper.SetDefault("agents.coder.model", models.CopilotGPT4o)
		viper.SetDefault("agents.summarizer.model", models.CopilotGPT4o)
		viper.SetDefault("agents.task.model", models.CopilotGPT4o)
		viper.SetDefault("agents.title.model", models.CopilotGPT4o)
		return
	}

	// Anthropic configuration
	if key := viper.GetString("providers.anthropic.apiKey"); strings.TrimSpace(key) != "" {
		viper.SetDefault("agents.coder.model", models.Claude4Sonnet)
		viper.SetDefault("agents.summarizer.model", models.Claude4Sonnet)
		viper.SetDefault("agents.task.model", models.Claude4Sonnet)
		viper.SetDefault("agents.title.model", models.Claude4Sonnet)
		return
	}

	// OpenAI configuration
	if key := viper.GetString("providers.openai.apiKey"); strings.TrimSpace(key) != "" {
		viper.SetDefault("agents.coder.model", models.GPT41)
		viper.SetDefault("agents.summarizer.model", models.GPT41)
		viper.SetDefault("agents.task.model", models.GPT41Mini)
		viper.SetDefault("agents.title.model", models.GPT41Mini)
		return
	}

	// Google Gemini configuration
	if key := viper.GetString("providers.gemini.apiKey"); strings.TrimSpace(key) != "" {
		viper.SetDefault("agents.coder.model", models.Gemini25)
		viper.SetDefault("agents.summarizer.model", models.Gemini25)
		viper.SetDefault("agents.task.model", models.Gemini25Flash)
		viper.SetDefault("agents.title.model", models.Gemini25Flash)
		return
	}

	// Groq configuration
	if key := viper.GetString("providers.groq.apiKey"); strings.TrimSpace(key) != "" {
		viper.SetDefault("agents.coder.model", models.QWENQwq)
		viper.SetDefault("agents.summarizer.model", models.QWENQwq)
		viper.SetDefault("agents.task.model", models.QWENQwq)
		viper.SetDefault("agents.title.model", models.QWENQwq)
		return
	}

	// OpenRouter configuration
	if key := viper.GetString("providers.openrouter.apiKey"); strings.TrimSpace(key) != "" {
		viper.SetDefault("agents.coder.model", models.OpenRouterClaude37Sonnet)
		viper.SetDefault("agents.summarizer.model", models.OpenRouterClaude37Sonnet)
		viper.SetDefault("agents.task.model", models.OpenRouterClaude37Sonnet)
		viper.SetDefault("agents.title.model", models.OpenRouterClaude35Haiku)
		return
	}

	// XAI configuration
	if key := viper.GetString("providers.xai.apiKey"); strings.TrimSpace(key) != "" {
		viper.SetDefault("agents.coder.model", models.XAIGrok3Beta)
		viper.SetDefault("agents.summarizer.model", models.XAIGrok3Beta)
		viper.SetDefault("agents.task.model", models.XAIGrok3Beta)
		viper.SetDefault("agents.title.model", models.XAiGrok3MiniFastBeta)
		return
	}

	// AWS Bedrock configuration
	if hasAWSCredentials() {
		viper.SetDefault("agents.coder.model", models.BedrockClaude37Sonnet)
		viper.SetDefault("agents.summarizer.model", models.BedrockClaude37Sonnet)
		viper.SetDefault("agents.task.model", models.BedrockClaude37Sonnet)
		viper.SetDefault("agents.title.model", models.BedrockClaude37Sonnet)
		return
	}

	// Azure OpenAI configuration
	if os.Getenv("AZURE_OPENAI_ENDPOINT") != "" {
		viper.SetDefault("agents.coder.model", models.AzureGPT41)
		viper.SetDefault("agents.summarizer.model", models.AzureGPT41)
		viper.SetDefault("agents.task.model", models.AzureGPT41Mini)
		viper.SetDefault("agents.title.model", models.AzureGPT41Mini)
		return
	}

	// Google Cloud VertexAI configuration
	if hasVertexAICredentials() {
		viper.SetDefault("agents.coder.model", models.VertexAIGemini25)
		viper.SetDefault("agents.summarizer.model", models.VertexAIGemini25)
		viper.SetDefault("agents.task.model", models.VertexAIGemini25Flash)
		viper.SetDefault("agents.title.model", models.VertexAIGemini25Flash)
		return
	}
}

// hasAWSCredentials checks if AWS credentials are available in the environment.
func hasAWSCredentials() bool {
	// Check for explicit AWS credentials
	if os.Getenv("AWS_ACCESS_KEY_ID") != "" && os.Getenv("AWS_SECRET_ACCESS_KEY") != "" {
		return true
	}

	// Check for AWS profile
	if os.Getenv("AWS_PROFILE") != "" || os.Getenv("AWS_DEFAULT_PROFILE") != "" {
		return true
	}

	// Check for AWS region
	if os.Getenv("AWS_REGION") != "" || os.Getenv("AWS_DEFAULT_REGION") != "" {
		return true
	}

	// Check if running on EC2 with instance profile
	if os.Getenv("AWS_CONTAINER_CREDENTIALS_RELATIVE_URI") != "" ||
		os.Getenv("AWS_CONTAINER_CREDENTIALS_FULL_URI") != "" {
		return true
	}

	return false
}

// hasVertexAICredentials checks if VertexAI credentials are available in the environment.
func hasVertexAICredentials() bool {
	// Check for explicit VertexAI parameters
	if os.Getenv("VERTEXAI_PROJECT") != "" && os.Getenv("VERTEXAI_LOCATION") != "" {
		return true
	}
	// Check for Google Cloud project and location
	if os.Getenv("GOOGLE_CLOUD_PROJECT") != "" && (os.Getenv("GOOGLE_CLOUD_REGION") != "" || os.Getenv("GOOGLE_CLOUD_LOCATION") != "") {
		return true
	}
	return false
}

func hasCopilotCredentials() bool {
	// Check for explicit Copilot parameters
	if token, _ := LoadGitHubToken(); token != "" {
		return true
	}
	return false
}

// readConfig handles the result of reading a configuration file.
func readConfig(err error) error {
	if err == nil {
		return nil
	}

	// It's okay if the config file doesn't exist
	if _, ok := err.(viper.ConfigFileNotFoundError); ok {
		return nil
	}

	return fmt.Errorf("failed to read config: %w", err)
}

// mergeLocalConfig loads and merges configuration from the local directory.
func mergeLocalConfig(workingDir string) {
	local := viper.New()
	local.SetConfigName(fmt.Sprintf(".%s", appName))
	local.SetConfigType("json")
	local.AddConfigPath(workingDir)

	// Merge local config if it exists
	if err := local.ReadInConfig(); err == nil {
		viper.MergeConfigMap(local.AllSettings())
	}
}

// applyDefaultValues sets default values for configuration fields that need processing.
func applyDefaultValues() {
	// Set default MCP type if not specified
	for k, v := range cfg.MCPServers {
		if v.Type == "" {
			v.Type = MCPStdio
			cfg.MCPServers[k] = v
		}
	}

	if cfg.Python.Tools == nil {
		cfg.Python.Tools = make(map[string]PythonToolConfig)
	}
}

func validateRAGConfig(cfg *Config) {
	if cfg.RAG.TopK <= 0 {
		cfg.RAG.TopK = 5
	}
	if cfg.RAG.MinScore < 0 || cfg.RAG.MinScore > 1 {
		cfg.RAG.MinScore = 0.2
	}
	if cfg.RAG.MaxContextChars <= 0 {
		cfg.RAG.MaxContextChars = 3200
	}
	if strings.TrimSpace(cfg.RAG.Embeddings.Provider) == "" {
		cfg.RAG.Embeddings.Provider = "ollama"
	} else {
		provider := strings.ToLower(strings.TrimSpace(cfg.RAG.Embeddings.Provider))
		switch provider {
		case "ollama", "hash", "local":
			cfg.RAG.Embeddings.Provider = provider
		default:
			logging.Warn("unsupported rag embedding provider, falling back to ollama", "provider", cfg.RAG.Embeddings.Provider)
			cfg.RAG.Embeddings.Provider = "ollama"
		}
	}
	if strings.TrimSpace(cfg.RAG.Embeddings.Model) == "" {
		cfg.RAG.Embeddings.Model = "nomic-embed-text"
	}
	if cfg.RAG.Embeddings.Dimensions <= 0 {
		cfg.RAG.Embeddings.Dimensions = 768
	}
	if cfg.RAG.Embeddings.TimeoutMs <= 0 {
		cfg.RAG.Embeddings.TimeoutMs = 15000
	}
}

func validatePythonConfig(cfg *Config) {
	if strings.TrimSpace(cfg.Python.Interpreter) == "" {
		cfg.Python.Interpreter = DefaultPythonInterpreter()
	}
	if cfg.Python.DefaultTimeoutMs <= 0 {
		cfg.Python.DefaultTimeoutMs = 60000
	}
	if cfg.Python.Tools == nil {
		cfg.Python.Tools = make(map[string]PythonToolConfig)
	}

	for name, tool := range cfg.Python.Tools {
		if strings.TrimSpace(tool.Path) == "" {
			logging.Warn("python tool has empty path, removing", "tool", name)
			delete(cfg.Python.Tools, name)
			continue
		}

		if !filepath.IsAbs(tool.Path) {
			tool.Path = filepath.Clean(filepath.Join(cfg.WorkingDir, tool.Path))
		}
		if tool.TimeoutMs <= 0 {
			tool.TimeoutMs = cfg.Python.DefaultTimeoutMs
		}
		cfg.Python.Tools[name] = tool
	}
}

// It validates model IDs and providers, ensuring they are supported.
func validateAgent(cfg *Config, name AgentName, agent Agent) error {
	// Check if model exists
	// TODO:	If a copilot model is specified, but model is not found,
	// 		 	it might be new model. The https://api.githubcopilot.com/models
	// 		 	endpoint should be queried to validate if the model is supported.
	model, modelExists := models.SupportedModels[agent.Model]
	if !modelExists {
		logging.Warn("unsupported model configured, reverting to default",
			"agent", name,
			"configured_model", agent.Model)

		// Set default model based on available providers
		if setDefaultModelForAgent(name) {
			logging.Info("set default model for agent", "agent", name, "model", cfg.Agents[name].Model)
		} else {
			return fmt.Errorf("no valid provider available for agent %s", name)
		}
		return nil
	}

	// Check if provider for the model is configured
	provider := model.Provider
	providerCfg, providerExists := cfg.Providers[provider]

	if !providerExists {
		// Provider not configured, check if we have environment variables
		apiKey := getProviderAPIKey(provider)
		if apiKey == "" {
			logging.Warn("provider not configured for model, reverting to default",
				"agent", name,
				"model", agent.Model,
				"provider", provider)

			// Set default model based on available providers
			if setDefaultModelForAgent(name) {
				logging.Info("set default model for agent", "agent", name, "model", cfg.Agents[name].Model)
			} else {
				return fmt.Errorf("no valid provider available for agent %s", name)
			}
		} else {
			// Add provider with API key from environment
			cfg.Providers[provider] = Provider{
				APIKey: apiKey,
			}
			logging.Info("added provider from environment", "provider", provider)
		}
	} else if providerCfg.Disabled || providerCfg.APIKey == "" {
		// Provider is disabled or has no API key
		logging.Warn("provider is disabled or has no API key, reverting to default",
			"agent", name,
			"model", agent.Model,
			"provider", provider)

		// Set default model based on available providers
		if setDefaultModelForAgent(name) {
			logging.Info("set default model for agent", "agent", name, "model", cfg.Agents[name].Model)
		} else {
			return fmt.Errorf("no valid provider available for agent %s", name)
		}
	}

	// Validate max tokens
	if agent.MaxTokens <= 0 {
		logging.Warn("invalid max tokens, setting to default",
			"agent", name,
			"model", agent.Model,
			"max_tokens", agent.MaxTokens)

		// Update the agent with default max tokens
		updatedAgent := cfg.Agents[name]
		if model.DefaultMaxTokens > 0 {
			updatedAgent.MaxTokens = model.DefaultMaxTokens
		} else {
			updatedAgent.MaxTokens = MaxTokensFallbackDefault
		}
		cfg.Agents[name] = updatedAgent
	} else if model.ContextWindow > 0 && agent.MaxTokens > model.ContextWindow/2 {
		// Ensure max tokens doesn't exceed half the context window (reasonable limit)
		logging.Warn("max tokens exceeds half the context window, adjusting",
			"agent", name,
			"model", agent.Model,
			"max_tokens", agent.MaxTokens,
			"context_window", model.ContextWindow)

		// Update the agent with adjusted max tokens
		updatedAgent := cfg.Agents[name]
		updatedAgent.MaxTokens = model.ContextWindow / 2
		cfg.Agents[name] = updatedAgent
	}

	// Validate reasoning effort for models that support reasoning
	if model.CanReason && (provider == models.ProviderOpenAI || provider == models.ProviderLocal) {
		if agent.ReasoningEffort == "" {
			// Set default reasoning effort for models that support it
			logging.Info("setting default reasoning effort for model that supports reasoning",
				"agent", name,
				"model", agent.Model)

			// Update the agent with default reasoning effort
			updatedAgent := cfg.Agents[name]
			updatedAgent.ReasoningEffort = "medium"
			cfg.Agents[name] = updatedAgent
		} else {
			// Check if reasoning effort is valid (low, medium, high)
			effort := strings.ToLower(agent.ReasoningEffort)
			if effort != "low" && effort != "medium" && effort != "high" {
				logging.Warn("invalid reasoning effort, setting to medium",
					"agent", name,
					"model", agent.Model,
					"reasoning_effort", agent.ReasoningEffort)

				// Update the agent with valid reasoning effort
				updatedAgent := cfg.Agents[name]
				updatedAgent.ReasoningEffort = "medium"
				cfg.Agents[name] = updatedAgent
			}
		}
	} else if !model.CanReason && agent.ReasoningEffort != "" {
		// Model doesn't support reasoning but reasoning effort is set
		logging.Warn("model doesn't support reasoning but reasoning effort is set, ignoring",
			"agent", name,
			"model", agent.Model,
			"reasoning_effort", agent.ReasoningEffort)

		// Update the agent to remove reasoning effort
		updatedAgent := cfg.Agents[name]
		updatedAgent.ReasoningEffort = ""
		cfg.Agents[name] = updatedAgent
	}

	updatedAgent := cfg.Agents[name]
	providerIsOpenAICompatible := provider == models.ProviderOpenAI || provider == models.ProviderLocal

	if updatedAgent.Temperature != nil {
		if *updatedAgent.Temperature < 0 || *updatedAgent.Temperature > 2 {
			logging.Warn("invalid temperature, clearing value",
				"agent", name,
				"model", updatedAgent.Model,
				"temperature", *updatedAgent.Temperature)
			updatedAgent.Temperature = nil
		}
	}

	if updatedAgent.TopP != nil {
		if *updatedAgent.TopP < 0 || *updatedAgent.TopP > 1 {
			logging.Warn("invalid top_p, clearing value",
				"agent", name,
				"model", updatedAgent.Model,
				"top_p", *updatedAgent.TopP)
			updatedAgent.TopP = nil
		}
	}

	if updatedAgent.MinP != nil {
		if *updatedAgent.MinP < 0 || *updatedAgent.MinP > 1 {
			logging.Warn("invalid min_p, clearing value",
				"agent", name,
				"model", updatedAgent.Model,
				"min_p", *updatedAgent.MinP)
			updatedAgent.MinP = nil
		}
	}

	if updatedAgent.TopK != nil {
		if *updatedAgent.TopK < 0 {
			logging.Warn("invalid top_k, clearing value",
				"agent", name,
				"model", updatedAgent.Model,
				"top_k", *updatedAgent.TopK)
			updatedAgent.TopK = nil
		}
	}

	if updatedAgent.NumCtx != nil {
		if *updatedAgent.NumCtx <= 0 {
			logging.Warn("invalid num_ctx, clearing value",
				"agent", name,
				"model", updatedAgent.Model,
				"num_ctx", *updatedAgent.NumCtx)
			updatedAgent.NumCtx = nil
		}
	}

	if updatedAgent.RepeatPenalty != nil {
		if *updatedAgent.RepeatPenalty < 0 {
			logging.Warn("invalid repeat_penalty, clearing value",
				"agent", name,
				"model", updatedAgent.Model,
				"repeat_penalty", *updatedAgent.RepeatPenalty)
			updatedAgent.RepeatPenalty = nil
		}
	}

	if !providerIsOpenAICompatible {
		if updatedAgent.Temperature != nil || updatedAgent.TopP != nil || updatedAgent.MinP != nil || updatedAgent.TopK != nil || updatedAgent.NumCtx != nil || updatedAgent.RepeatPenalty != nil || updatedAgent.Seed != nil {
			logging.Warn("sampling options set for non openai-compatible provider, ignoring",
				"agent", name,
				"model", updatedAgent.Model,
				"provider", provider)
			updatedAgent.Temperature = nil
			updatedAgent.TopP = nil
			updatedAgent.MinP = nil
			updatedAgent.TopK = nil
			updatedAgent.NumCtx = nil
			updatedAgent.RepeatPenalty = nil
			updatedAgent.Seed = nil
		}
	}

	cfg.Agents[name] = updatedAgent

	return nil
}

func enforceLocalOnlyMode(cfg *Config) error {
	localModels := make([]models.ModelID, 0)
	for modelID, model := range models.SupportedModels {
		if model.Provider == models.ProviderLocal {
			localModels = append(localModels, modelID)
		}
	}

	if len(localModels) == 0 {
		return fmt.Errorf("localOnly is enabled, but no local models were discovered; set LOCAL_ENDPOINT to a reachable local or container endpoint")
	}

	sort.Slice(localModels, func(i, j int) bool {
		return string(localModels[i]) < string(localModels[j])
	})

	if cfg.Providers == nil {
		cfg.Providers = make(map[models.ModelProvider]Provider)
	}

	for providerName, providerCfg := range cfg.Providers {
		if providerName == models.ProviderLocal {
			continue
		}
		providerCfg.Disabled = true
		cfg.Providers[providerName] = providerCfg
	}

	localProviderCfg := cfg.Providers[models.ProviderLocal]
	localProviderCfg.Disabled = false
	if strings.TrimSpace(localProviderCfg.APIKey) == "" {
		localProviderCfg.APIKey = "dummy"
	}
	cfg.Providers[models.ProviderLocal] = localProviderCfg

	if cfg.Agents == nil {
		cfg.Agents = make(map[AgentName]Agent)
	}

	selectedLocalModel := localModels[0]
	if coderAgent, ok := cfg.Agents[AgentCoder]; ok {
		if model, modelExists := models.SupportedModels[coderAgent.Model]; modelExists && model.Provider == models.ProviderLocal {
			selectedLocalModel = coderAgent.Model
		}
	}

	targetAgents := []AgentName{AgentCoder, AgentSummarizer, AgentTask, AgentTitle}
	for _, agentName := range targetAgents {
		agentCfg := cfg.Agents[agentName]
		if model, modelExists := models.SupportedModels[agentCfg.Model]; !modelExists || model.Provider != models.ProviderLocal {
			agentCfg.Model = selectedLocalModel
		}

		selectedModel := models.SupportedModels[agentCfg.Model]
		if agentName == AgentTitle {
			agentCfg.MaxTokens = 80
		} else if agentCfg.MaxTokens <= 0 {
			if selectedModel.DefaultMaxTokens > 0 {
				agentCfg.MaxTokens = selectedModel.DefaultMaxTokens
			} else {
				agentCfg.MaxTokens = MaxTokensFallbackDefault
			}
		}

		cfg.Agents[agentName] = agentCfg
	}

	return nil
}

// Validate checks if the configuration is valid and applies defaults where needed.
func Validate() error {
	if cfg == nil {
		return fmt.Errorf("config not loaded")
	}

	if cfg.LocalOnly {
		if err := enforceLocalOnlyMode(cfg); err != nil {
			return err
		}
	}

	validateRAGConfig(cfg)
	validatePythonConfig(cfg)

	// Validate agent models
	for name, agent := range cfg.Agents {
		if err := validateAgent(cfg, name, agent); err != nil {
			return err
		}
	}

	// Validate providers
	for provider, providerCfg := range cfg.Providers {
		if providerCfg.APIKey == "" && !providerCfg.Disabled {
			fmt.Printf("provider has no API key, marking as disabled %s", provider)
			logging.Warn("provider has no API key, marking as disabled", "provider", provider)
			providerCfg.Disabled = true
			cfg.Providers[provider] = providerCfg
		}
	}

	// Validate LSP configurations
	for language, lspConfig := range cfg.LSP {
		if lspConfig.Command == "" && !lspConfig.Disabled {
			logging.Warn("LSP configuration has no command, marking as disabled", "language", language)
			lspConfig.Disabled = true
			cfg.LSP[language] = lspConfig
		}
	}

	return nil
}

// getProviderAPIKey gets the API key for a provider from environment variables
func getProviderAPIKey(provider models.ModelProvider) string {
	switch provider {
	case models.ProviderAnthropic:
		return os.Getenv("ANTHROPIC_API_KEY")
	case models.ProviderOpenAI:
		return os.Getenv("OPENAI_API_KEY")
	case models.ProviderGemini:
		return os.Getenv("GEMINI_API_KEY")
	case models.ProviderGROQ:
		return os.Getenv("GROQ_API_KEY")
	case models.ProviderAzure:
		return os.Getenv("AZURE_OPENAI_API_KEY")
	case models.ProviderOpenRouter:
		return os.Getenv("OPENROUTER_API_KEY")
	case models.ProviderBedrock:
		if hasAWSCredentials() {
			return "aws-credentials-available"
		}
	case models.ProviderVertexAI:
		if hasVertexAICredentials() {
			return "vertex-ai-credentials-available"
		}
	}
	return ""
}

// setDefaultModelForAgent sets a default model for an agent based on available providers
func setDefaultModelForAgent(agent AgentName) bool {
	if hasCopilotCredentials() {
		maxTokens := int64(5000)
		if agent == AgentTitle {
			maxTokens = 80
		}

		cfg.Agents[agent] = Agent{
			Model:     models.CopilotGPT4o,
			MaxTokens: maxTokens,
		}
		return true
	}
	// Check providers in order of preference
	if apiKey := os.Getenv("ANTHROPIC_API_KEY"); apiKey != "" {
		maxTokens := int64(5000)
		if agent == AgentTitle {
			maxTokens = 80
		}
		cfg.Agents[agent] = Agent{
			Model:     models.Claude37Sonnet,
			MaxTokens: maxTokens,
		}
		return true
	}

	if apiKey := os.Getenv("OPENAI_API_KEY"); apiKey != "" {
		var model models.ModelID
		maxTokens := int64(5000)
		reasoningEffort := ""

		switch agent {
		case AgentTitle:
			model = models.GPT41Mini
			maxTokens = 80
		case AgentTask:
			model = models.GPT41Mini
		default:
			model = models.GPT41
		}

		// Check if model supports reasoning
		if modelInfo, ok := models.SupportedModels[model]; ok && modelInfo.CanReason {
			reasoningEffort = "medium"
		}

		cfg.Agents[agent] = Agent{
			Model:           model,
			MaxTokens:       maxTokens,
			ReasoningEffort: reasoningEffort,
		}
		return true
	}

	if apiKey := os.Getenv("OPENROUTER_API_KEY"); apiKey != "" {
		var model models.ModelID
		maxTokens := int64(5000)
		reasoningEffort := ""

		switch agent {
		case AgentTitle:
			model = models.OpenRouterClaude35Haiku
			maxTokens = 80
		case AgentTask:
			model = models.OpenRouterClaude37Sonnet
		default:
			model = models.OpenRouterClaude37Sonnet
		}

		// Check if model supports reasoning
		if modelInfo, ok := models.SupportedModels[model]; ok && modelInfo.CanReason {
			reasoningEffort = "medium"
		}

		cfg.Agents[agent] = Agent{
			Model:           model,
			MaxTokens:       maxTokens,
			ReasoningEffort: reasoningEffort,
		}
		return true
	}

	if apiKey := os.Getenv("GEMINI_API_KEY"); apiKey != "" {
		var model models.ModelID
		maxTokens := int64(5000)

		if agent == AgentTitle {
			model = models.Gemini25Flash
			maxTokens = 80
		} else {
			model = models.Gemini25
		}

		cfg.Agents[agent] = Agent{
			Model:     model,
			MaxTokens: maxTokens,
		}
		return true
	}

	if apiKey := os.Getenv("GROQ_API_KEY"); apiKey != "" {
		maxTokens := int64(5000)
		if agent == AgentTitle {
			maxTokens = 80
		}

		cfg.Agents[agent] = Agent{
			Model:     models.QWENQwq,
			MaxTokens: maxTokens,
		}
		return true
	}

	if hasAWSCredentials() {
		maxTokens := int64(5000)
		if agent == AgentTitle {
			maxTokens = 80
		}

		cfg.Agents[agent] = Agent{
			Model:           models.BedrockClaude37Sonnet,
			MaxTokens:       maxTokens,
			ReasoningEffort: "medium", // Claude models support reasoning
		}
		return true
	}

	if hasVertexAICredentials() {
		var model models.ModelID
		maxTokens := int64(5000)

		if agent == AgentTitle {
			model = models.VertexAIGemini25Flash
			maxTokens = 80
		} else {
			model = models.VertexAIGemini25
		}

		cfg.Agents[agent] = Agent{
			Model:     model,
			MaxTokens: maxTokens,
		}
		return true
	}

	return false
}

func updateCfgFile(updateCfg func(config *Config)) error {
	if cfg == nil {
		return fmt.Errorf("config not loaded")
	}

	// Get the config file path
	configFile := viper.ConfigFileUsed()
	var configData []byte
	if configFile == "" {
		homeDir, err := os.UserHomeDir()
		if err != nil {
			return fmt.Errorf("failed to get home directory: %w", err)
		}
		configFile = filepath.Join(homeDir, fmt.Sprintf(".%s.json", appName))
		logging.Info("config file not found, creating new one", "path", configFile)
		configData = []byte(`{}`)
	} else {
		// Read the existing config file
		data, err := os.ReadFile(configFile)
		if err != nil {
			return fmt.Errorf("failed to read config file: %w", err)
		}
		configData = data
	}

	// Parse the JSON
	var userCfg *Config
	if err := json.Unmarshal(configData, &userCfg); err != nil {
		return fmt.Errorf("failed to parse config file: %w", err)
	}

	updateCfg(userCfg)

	// Write the updated config back to file
	updatedData, err := json.MarshalIndent(userCfg, "", "  ")
	if err != nil {
		return fmt.Errorf("failed to marshal config: %w", err)
	}

	if err := os.WriteFile(configFile, updatedData, 0o644); err != nil {
		return fmt.Errorf("failed to write config file: %w", err)
	}

	return nil
}

// Get returns the current configuration.
// It's safe to call this function multiple times.
func Get() *Config {
	return cfg
}

// WorkingDirectory returns the current working directory from the configuration.
func WorkingDirectory() string {
	if cfg == nil {
		panic("config not loaded")
	}
	return cfg.WorkingDir
}

func UpdateAgentModel(agentName AgentName, modelID models.ModelID) error {
	if cfg == nil {
		panic("config not loaded")
	}

	existingAgentCfg := cfg.Agents[agentName]

	model, ok := models.SupportedModels[modelID]
	if !ok {
		return fmt.Errorf("model %s not supported", modelID)
	}

	maxTokens := existingAgentCfg.MaxTokens
	if model.DefaultMaxTokens > 0 {
		maxTokens = model.DefaultMaxTokens
	}

	newAgentCfg := Agent{
		Model:           modelID,
		MaxTokens:       maxTokens,
		ReasoningEffort: existingAgentCfg.ReasoningEffort,
		Temperature:     existingAgentCfg.Temperature,
		TopP:            existingAgentCfg.TopP,
		MinP:            existingAgentCfg.MinP,
		TopK:            existingAgentCfg.TopK,
		NumCtx:          existingAgentCfg.NumCtx,
		RepeatPenalty:   existingAgentCfg.RepeatPenalty,
		Seed:            existingAgentCfg.Seed,
	}
	cfg.Agents[agentName] = newAgentCfg

	if err := validateAgent(cfg, agentName, newAgentCfg); err != nil {
		// revert config update on failure
		cfg.Agents[agentName] = existingAgentCfg
		return fmt.Errorf("failed to update agent model: %w", err)
	}

	return updateCfgFile(func(config *Config) {
		if config.Agents == nil {
			config.Agents = make(map[AgentName]Agent)
		}
		config.Agents[agentName] = newAgentCfg
	})
}

func updateAgentConfig(agentName AgentName, mutate func(agent *Agent)) error {
	if cfg == nil {
		return fmt.Errorf("config not loaded")
	}

	if cfg.Agents == nil {
		cfg.Agents = make(map[AgentName]Agent)
	}

	existingAgentCfg := cfg.Agents[agentName]
	updatedAgent := existingAgentCfg
	mutate(&updatedAgent)

	cfg.Agents[agentName] = updatedAgent
	if err := validateAgent(cfg, agentName, updatedAgent); err != nil {
		cfg.Agents[agentName] = existingAgentCfg
		return fmt.Errorf("failed to update agent config: %w", err)
	}

	validatedAgent := cfg.Agents[agentName]

	return updateCfgFile(func(config *Config) {
		if config.Agents == nil {
			config.Agents = make(map[AgentName]Agent)
		}
		config.Agents[agentName] = validatedAgent
	})
}

func UpdateAgentTemperature(agentName AgentName, value *float64) error {
	return updateAgentConfig(agentName, func(agent *Agent) {
		agent.Temperature = value
	})
}

func UpdateAgentTopK(agentName AgentName, value *int64) error {
	return updateAgentConfig(agentName, func(agent *Agent) {
		agent.TopK = value
	})
}

func UpdateAgentNumCtx(agentName AgentName, value *int64) error {
	return updateAgentConfig(agentName, func(agent *Agent) {
		agent.NumCtx = value
	})
}

func UpdateAgentMinP(agentName AgentName, value *float64) error {
	return updateAgentConfig(agentName, func(agent *Agent) {
		agent.MinP = value
	})
}

func UpdateAgentRepeatPenalty(agentName AgentName, value *float64) error {
	return updateAgentConfig(agentName, func(agent *Agent) {
		agent.RepeatPenalty = value
	})
}

// UpdateTheme updates the theme in the configuration and writes it to the config file.
func UpdateTheme(themeName string) error {
	if cfg == nil {
		return fmt.Errorf("config not loaded")
	}

	// Update the in-memory config
	cfg.TUI.Theme = themeName

	// Update the file config
	return updateCfgFile(func(config *Config) {
		config.TUI.Theme = themeName
	})
}

// Tries to load Github token from all possible locations
func LoadGitHubToken() (string, error) {
	// First check environment variable
	if token := os.Getenv("GITHUB_TOKEN"); token != "" {
		return token, nil
	}

	// Get config directory
	var configDir string
	if xdgConfig := os.Getenv("XDG_CONFIG_HOME"); xdgConfig != "" {
		configDir = xdgConfig
	} else if runtime.GOOS == "windows" {
		if localAppData := os.Getenv("LOCALAPPDATA"); localAppData != "" {
			configDir = localAppData
		} else {
			configDir = filepath.Join(os.Getenv("HOME"), "AppData", "Local")
		}
	} else {
		configDir = filepath.Join(os.Getenv("HOME"), ".config")
	}

	// Try both hosts.json and apps.json files
	filePaths := []string{
		filepath.Join(configDir, "github-copilot", "hosts.json"),
		filepath.Join(configDir, "github-copilot", "apps.json"),
	}

	for _, filePath := range filePaths {
		data, err := os.ReadFile(filePath)
		if err != nil {
			continue
		}

		var config map[string]map[string]interface{}
		if err := json.Unmarshal(data, &config); err != nil {
			continue
		}

		for key, value := range config {
			if strings.Contains(key, "github.com") {
				if oauthToken, ok := value["oauth_token"].(string); ok {
					return oauthToken, nil
				}
			}
		}
	}

	return "", fmt.Errorf("GitHub token not found in standard locations")
}
