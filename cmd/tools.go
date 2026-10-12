package cmd

import (
	"errors"
	"fmt"
	"os/exec"

	"github.com/opencode-ai/opencode/internal/bundledtools"
	"github.com/spf13/cobra"
)

func newToolCommand(name, description string) *cobra.Command {
	return &cobra.Command{
		Use:                name + " [arguments...]",
		Short:              description,
		DisableFlagParsing: true,
		SilenceUsage:       true,
		SilenceErrors:      true,
		RunE: func(cmd *cobra.Command, args []string) error {
			path, err := bundledtools.LookPath(name)
			if err != nil {
				fmt.Fprintf(cmd.ErrOrStderr(), "Unable to find %s: %v\n", name, err)
				return err
			}
			process := exec.CommandContext(cmd.Context(), path, args...)
			process.Stdin = cmd.InOrStdin()
			process.Stdout = cmd.OutOrStdout()
			process.Stderr = cmd.ErrOrStderr()
			err = process.Run()
			var exitErr *exec.ExitError
			if err != nil && !errors.As(err, &exitErr) {
				fmt.Fprintf(cmd.ErrOrStderr(), "Unable to run %s: %v\n", name, err)
			}
			return err
		},
	}
}

func init() {
	rootCmd.AddCommand(newToolCommand("rg", "Run the bundled ripgrep with its native flags"))
	rootCmd.AddCommand(newToolCommand("fzf", "Run the bundled fuzzy finder with its native flags"))
	rootCmd.AddCommand(&cobra.Command{
		Use:   "licenses",
		Short: "Print bundled ripgrep and fzf license notices",
		Args:  cobra.NoArgs,
		RunE: func(cmd *cobra.Command, args []string) error {
			_, err := fmt.Fprint(cmd.OutOrStdout(), bundledtools.Notices)
			return err
		},
	})
}
