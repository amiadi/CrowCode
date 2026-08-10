package cmd

import (
	"fmt"
	"os"
	"strings"

	"github.com/opencode-ai/opencode/internal/config"
	"github.com/spf13/cobra"
)

var mcpCmd = &cobra.Command{
	Use:   "mcp",
	Short: "Manage and inspect MCP servers",
}

var mcpListCmd = &cobra.Command{
	Use:   "list",
	Short: "List effective MCP servers with discovery source",
	RunE: func(cmd *cobra.Command, args []string) error {
		sourceFilterValues, _ := cmd.Flags().GetStringSlice("source")
		sourceFilters, err := parseMCPServerSourceFilters(sourceFilterValues)
		if err != nil {
			return err
		}

		typeFilterValues, _ := cmd.Flags().GetStringSlice("type")
		typeFilters, err := parseMCPTypeFilters(typeFilterValues)
		if err != nil {
			return err
		}

		debug, _ := cmd.Flags().GetBool("debug")
		cwd, _ := cmd.Flags().GetString("cwd")
		if strings.TrimSpace(cwd) != "" {
			if err := os.Chdir(cwd); err != nil {
				return fmt.Errorf("failed to change directory: %w", err)
			}
		}
		if strings.TrimSpace(cwd) == "" {
			current, err := os.Getwd()
			if err != nil {
				return fmt.Errorf("failed to get current working directory: %w", err)
			}
			cwd = current
		}

		cfg, err := config.Load(cwd, debug)
		if err != nil {
			return err
		}

		resolved, err := config.ResolveMCPServers(cfg)
		if err != nil {
			return err
		}

		hasFilters := len(sourceFilters) > 0 || len(typeFilters) > 0
		if len(resolved) == 0 {
			fmt.Println("No MCP servers discovered")
			return nil
		}

		shown := 0
		for _, item := range resolved {
			if !matchesMCPFilters(item, sourceFilters, typeFilters) {
				continue
			}

			endpoint := item.Server.Command
			if item.Server.Type == config.MCPSse {
				endpoint = item.Server.URL
			}
			if strings.TrimSpace(endpoint) == "" {
				endpoint = "(none)"
			}

			pathInfo := item.Path
			if strings.TrimSpace(pathInfo) == "" {
				pathInfo = "(runtime)"
			}

			fmt.Printf("- %s\n", item.Name)
			fmt.Printf("  source: %s\n", item.Source)
			fmt.Printf("  type: %s\n", item.Server.Type)
			fmt.Printf("  endpoint: %s\n", endpoint)
			fmt.Printf("  path: %s\n", pathInfo)
			shown++
		}

		if shown == 0 {
			if hasFilters {
				fmt.Println("No MCP servers matched the requested filters")
				return nil
			}
			fmt.Println("No MCP servers discovered")
		}

		return nil
	},
}

func parseMCPServerSourceFilters(values []string) (map[config.MCPServerSource]struct{}, error) {
	result := make(map[config.MCPServerSource]struct{})
	for _, value := range values {
		for _, part := range strings.Split(value, ",") {
			normalized := strings.TrimSpace(strings.ToLower(part))
			if normalized == "" {
				continue
			}

			switch normalized {
			case string(config.MCPServerSourceExplicit):
				result[config.MCPServerSourceExplicit] = struct{}{}
			case string(config.MCPServerSourceManifest):
				result[config.MCPServerSourceManifest] = struct{}{}
			case string(config.MCPServerSourceBinary):
				result[config.MCPServerSourceBinary] = struct{}{}
			default:
				return nil, fmt.Errorf("invalid --source value %q (allowed: explicit, manifest, binary)", normalized)
			}
		}
	}

	if len(result) == 0 {
		return nil, nil
	}
	return result, nil
}

func parseMCPTypeFilters(values []string) (map[config.MCPType]struct{}, error) {
	result := make(map[config.MCPType]struct{})
	for _, value := range values {
		for _, part := range strings.Split(value, ",") {
			normalized := strings.TrimSpace(strings.ToLower(part))
			if normalized == "" {
				continue
			}

			switch normalized {
			case string(config.MCPStdio):
				result[config.MCPStdio] = struct{}{}
			case string(config.MCPSse):
				result[config.MCPSse] = struct{}{}
			default:
				return nil, fmt.Errorf("invalid --type value %q (allowed: stdio, sse)", normalized)
			}
		}
	}

	if len(result) == 0 {
		return nil, nil
	}
	return result, nil
}

func matchesMCPFilters(
	item config.MCPServerResolved,
	sourceFilters map[config.MCPServerSource]struct{},
	typeFilters map[config.MCPType]struct{},
) bool {
	if len(sourceFilters) > 0 {
		if _, ok := sourceFilters[item.Source]; !ok {
			return false
		}
	}

	if len(typeFilters) > 0 {
		if _, ok := typeFilters[item.Server.Type]; !ok {
			return false
		}
	}

	return true
}

func init() {
	mcpCmd.AddCommand(mcpListCmd)
	rootCmd.AddCommand(mcpCmd)
	mcpListCmd.Flags().StringSlice("source", nil, "Filter by source: explicit, manifest, binary (repeatable or comma-separated)")
	mcpListCmd.Flags().StringSlice("type", nil, "Filter by type: stdio, sse (repeatable or comma-separated)")

	for _, sub := range []*cobra.Command{mcpListCmd} {
		sub.Flags().Bool("debug", false, "Debug")
		sub.Flags().String("cwd", "", "Current working directory")
	}
}
