package config

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"sort"
	"strings"
	"time"

	"github.com/opencode-ai/opencode/internal/llm/models"
)

type ConnectionProvider string

const (
	ConnectionProviderOllama ConnectionProvider = "ollama"
	ConnectionProviderVLLM   ConnectionProvider = "vllm"
	ConnectionProviderSGLang ConnectionProvider = "sglang"
)

type ConnectionSetupOptions struct {
	Provider       ConnectionProvider
	Endpoint       string
	LocalOnly      bool
	PreferEmbedder bool
}

type ConnectionProbeResult struct {
	Endpoint string
	Models   []string
	RawCount int
}

func NormalizeConnectionProvider(raw string) ConnectionProvider {
	switch strings.ToLower(strings.TrimSpace(raw)) {
	case "ollama":
		return ConnectionProviderOllama
	case "vllm", "vllm-openai":
		return ConnectionProviderVLLM
	case "sglang":
		return ConnectionProviderSGLang
	default:
		return ConnectionProviderOllama
	}
}

func NormalizeEndpoint(endpoint string) string {
	trimmed := strings.TrimSpace(endpoint)
	trimmed = strings.TrimSuffix(trimmed, "/")
	return trimmed
}

func ProbeLocalConnection(ctx context.Context, endpoint string) (ConnectionProbeResult, error) {
	endpoint = NormalizeEndpoint(endpoint)
	if endpoint == "" {
		return ConnectionProbeResult{}, fmt.Errorf("endpoint is required")
	}

	client := &http.Client{Timeout: 8 * time.Second}
	for _, url := range probeModelURLs(endpoint) {
		req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
		if err != nil {
			continue
		}
		resp, err := client.Do(req)
		if err != nil {
			continue
		}
		raw, _ := io.ReadAll(io.LimitReader(resp.Body, 4*1024*1024))
		_ = resp.Body.Close()
		if resp.StatusCode < 200 || resp.StatusCode >= 300 {
			continue
		}

		var parsed struct {
			Data []struct {
				ID string `json:"id"`
			} `json:"data"`
		}
		if err := json.Unmarshal(raw, &parsed); err != nil {
			continue
		}

		modelIDs := make([]string, 0, len(parsed.Data))
		for _, item := range parsed.Data {
			if strings.TrimSpace(item.ID) != "" {
				modelIDs = append(modelIDs, item.ID)
			}
		}
		sort.Strings(modelIDs)
		return ConnectionProbeResult{
			Endpoint: endpoint,
			Models:   modelIDs,
			RawCount: len(parsed.Data),
		}, nil
	}

	return ConnectionProbeResult{}, fmt.Errorf("unable to reach model list endpoint at %s (/v1/models or /api/v0/models)", endpoint)
}

func probeModelURLs(endpoint string) []string {
	bases := []string{endpoint}
	lower := strings.ToLower(endpoint)
	if strings.HasSuffix(lower, "/v1") {
		bases = append(bases, strings.TrimSuffix(endpoint, endpoint[len(endpoint)-3:]))
	}

	seen := map[string]struct{}{}
	urls := make([]string, 0, len(bases)*3)
	for _, base := range bases {
		base = strings.TrimSuffix(strings.TrimSpace(base), "/")
		if base == "" {
			continue
		}
		candidates := []string{base + "/v1/models", base + "/api/v0/models", base + "/models"}
		for _, candidate := range candidates {
			if _, ok := seen[candidate]; ok {
				continue
			}
			seen[candidate] = struct{}{}
			urls = append(urls, candidate)
		}
	}
	return urls
}

func ApplyConnectionSetup(options ConnectionSetupOptions) error {
	if cfg == nil {
		return fmt.Errorf("config not loaded")
	}

	endpoint := NormalizeEndpoint(options.Endpoint)
	if endpoint == "" {
		return fmt.Errorf("endpoint is required")
	}

	provider := NormalizeConnectionProvider(string(options.Provider))
	_ = os.Setenv("LOCAL_ENDPOINT", endpoint)
	models.BootstrapLocalModels(endpoint)

	cfg.LocalEndpoint = endpoint
	cfg.LocalOnly = options.LocalOnly
	if cfg.Providers == nil {
		cfg.Providers = make(map[models.ModelProvider]Provider)
	}
	localProvider := cfg.Providers[models.ProviderLocal]
	localProvider.APIKey = "dummy"
	localProvider.Disabled = false
	cfg.Providers[models.ProviderLocal] = localProvider

	if provider == ConnectionProviderOllama && options.PreferEmbedder {
		cfg.RAG.Embeddings.Provider = "ollama"
	}

	if cfg.Agents == nil {
		cfg.Agents = make(map[AgentName]Agent)
	}
	localModelIDs := localModelIDsSorted()
	if len(localModelIDs) > 0 {
		selected := localModelIDs[0]
		for _, name := range []AgentName{AgentCoder, AgentSummarizer, AgentTask, AgentTitle} {
			agentCfg := cfg.Agents[name]
			agentCfg.Model = selected
			if name == AgentTitle {
				agentCfg.MaxTokens = 80
			}
			cfg.Agents[name] = agentCfg
		}
	}

	if err := updateCfgFile(func(fileCfg *Config) {
		fileCfg.LocalEndpoint = endpoint
		fileCfg.LocalOnly = options.LocalOnly
		if fileCfg.Providers == nil {
			fileCfg.Providers = make(map[models.ModelProvider]Provider)
		}
		lp := fileCfg.Providers[models.ProviderLocal]
		lp.APIKey = "dummy"
		lp.Disabled = false
		fileCfg.Providers[models.ProviderLocal] = lp

		if provider == ConnectionProviderOllama && options.PreferEmbedder {
			fileCfg.RAG.Embeddings.Provider = "ollama"
		}

		if fileCfg.Agents == nil {
			fileCfg.Agents = make(map[AgentName]Agent)
		}
		localIDs := localModelIDsSorted()
		if len(localIDs) > 0 {
			selected := localIDs[0]
			for _, name := range []AgentName{AgentCoder, AgentSummarizer, AgentTask, AgentTitle} {
				agentCfg := fileCfg.Agents[name]
				agentCfg.Model = selected
				if name == AgentTitle {
					agentCfg.MaxTokens = 80
				}
				fileCfg.Agents[name] = agentCfg
			}
		}
	}); err != nil {
		return err
	}

	if cfg.LocalOnly {
		if err := enforceLocalOnlyMode(cfg); err != nil {
			return err
		}
	}

	return nil
}

func localModelIDsSorted() []models.ModelID {
	ids := make([]models.ModelID, 0)
	for modelID, model := range models.SupportedModels {
		if model.Provider == models.ProviderLocal {
			ids = append(ids, modelID)
		}
	}
	sort.Slice(ids, func(i, j int) bool { return string(ids[i]) < string(ids[j]) })
	return ids
}
