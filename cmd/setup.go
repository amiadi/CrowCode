package cmd

import (
	"bufio"
	"context"
	"fmt"
	"os"
	"strconv"
	"strings"
	"time"

	"github.com/opencode-ai/opencode/internal/config"
	"github.com/spf13/cobra"
)

var setupCmd = &cobra.Command{
	Use:   "setup",
	Short: "Run connection setup wizard (Ollama, vLLM, SGLang)",
	RunE: func(cmd *cobra.Command, args []string) error {
		providerFlag, _ := cmd.Flags().GetString("provider")
		endpointFlag, _ := cmd.Flags().GetString("endpoint")
		localOnly, _ := cmd.Flags().GetBool("local-only")
		preferEmbedder, _ := cmd.Flags().GetBool("prefer-ollama-embedder")
		noProbe, _ := cmd.Flags().GetBool("no-probe")
		yes, _ := cmd.Flags().GetBool("yes")

		provider := config.NormalizeConnectionProvider(providerFlag)
		endpoint := config.NormalizeEndpoint(endpointFlag)

		if !yes {
			reader := bufio.NewReader(os.Stdin)
			if strings.TrimSpace(providerFlag) == "" {
				fmt.Println("Select connection type:")
				fmt.Println("1) Ollama")
				fmt.Println("2) vLLM")
				fmt.Println("3) SGLang")
				fmt.Print("Choice [1]: ")
				choiceRaw, _ := reader.ReadString('\n')
				choice := strings.TrimSpace(choiceRaw)
				switch choice {
				case "", "1":
					provider = config.ConnectionProviderOllama
				case "2":
					provider = config.ConnectionProviderVLLM
				case "3":
					provider = config.ConnectionProviderSGLang
				default:
					return fmt.Errorf("invalid provider choice: %s", choice)
				}
			}

			if endpoint == "" {
				defaultEndpoint := defaultEndpointForProvider(provider)
				fmt.Printf("Endpoint URL [%s]: ", defaultEndpoint)
				raw, _ := reader.ReadString('\n')
				value := strings.TrimSpace(raw)
				if value == "" {
					value = defaultEndpoint
				}
				endpoint = config.NormalizeEndpoint(value)
			}

			if !cmd.Flags().Changed("local-only") {
				fmt.Print("Enable local-only mode? [y/N]: ")
				raw, _ := reader.ReadString('\n')
				localOnly = strings.EqualFold(strings.TrimSpace(raw), "y") || strings.EqualFold(strings.TrimSpace(raw), "yes")
			}

			if !cmd.Flags().Changed("prefer-ollama-embedder") {
				defaultPrompt := "N"
				if provider == config.ConnectionProviderOllama {
					defaultPrompt = "Y"
				}
				fmt.Printf("Use ollama embedder for RAG? [y/%s]: ", strings.ToLower(defaultPrompt))
				raw, _ := reader.ReadString('\n')
				v := strings.TrimSpace(strings.ToLower(raw))
				if v == "y" || v == "yes" {
					preferEmbedder = true
				} else if v == "n" || v == "no" {
					preferEmbedder = false
				} else {
					preferEmbedder = provider == config.ConnectionProviderOllama
				}
			}
		}

		if endpoint == "" {
			return fmt.Errorf("endpoint is required")
		}

		if !noProbe {
			ctx, cancel := context.WithTimeout(context.Background(), 9*time.Second)
			defer cancel()
			probe, err := config.ProbeLocalConnection(ctx, endpoint)
			if err != nil {
				fmt.Printf("Warning: connection probe failed: %v\n", err)
			} else {
				shown := probe.Models
				if len(shown) > 5 {
					shown = shown[:5]
				}
				fmt.Printf("Connection probe succeeded: %d models discovered\n", probe.RawCount)
				if len(shown) > 0 {
					fmt.Printf("Sample models: %s\n", strings.Join(shown, ", "))
				}
			}
		}

		err := config.ApplyConnectionSetup(config.ConnectionSetupOptions{
			Provider:       provider,
			Endpoint:       endpoint,
			LocalOnly:      localOnly,
			PreferEmbedder: preferEmbedder,
		})
		if err != nil {
			return err
		}

		fmt.Println("Connection setup saved.")
		fmt.Printf("Provider: %s\n", provider)
		fmt.Printf("Endpoint: %s\n", endpoint)
		fmt.Printf("localOnly: %v\n", localOnly)
		fmt.Println("Tip: restart CrowCode to fully refresh local model discovery in all flows.")
		return nil
	},
}

func defaultEndpointForProvider(provider config.ConnectionProvider) string {
	switch provider {
	case config.ConnectionProviderOllama:
		return "http://127.0.0.1:11434"
	case config.ConnectionProviderVLLM:
		return "http://127.0.0.1:8000"
	case config.ConnectionProviderSGLang:
		return "http://127.0.0.1:30000"
	default:
		return "http://127.0.0.1:11434"
	}
}

func init() {
	rootCmd.AddCommand(setupCmd)
	setupCmd.Flags().String("provider", "", "Connection provider: ollama|vllm|sglang")
	setupCmd.Flags().String("endpoint", "", "Endpoint URL, e.g. http://127.0.0.1:11434")
	setupCmd.Flags().Bool("local-only", false, "Enable localOnly mode")
	setupCmd.Flags().Bool("prefer-ollama-embedder", false, "Set rag.embeddings.provider to ollama when applicable")
	setupCmd.Flags().Bool("no-probe", false, "Skip endpoint probe")
	setupCmd.Flags().BoolP("yes", "y", false, "Non-interactive mode: use provided flags only")

	_ = setupCmd.RegisterFlagCompletionFunc("provider", func(cmd *cobra.Command, args []string, toComplete string) ([]string, cobra.ShellCompDirective) {
		return []string{"ollama", "vllm", "sglang"}, cobra.ShellCompDirectiveNoFileComp
	})

	_ = setupCmd.RegisterFlagCompletionFunc("local-only", func(cmd *cobra.Command, args []string, toComplete string) ([]string, cobra.ShellCompDirective) {
		return []string{strconv.FormatBool(true), strconv.FormatBool(false)}, cobra.ShellCompDirectiveNoFileComp
	})
}
