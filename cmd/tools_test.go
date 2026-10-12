package cmd

import (
	"bytes"
	"errors"
	"os/exec"
	"runtime"
	"strings"
	"testing"
)

func TestToolCommands(t *testing.T) {
	if runtime.GOARCH != "amd64" || (runtime.GOOS != "linux" && runtime.GOOS != "windows") {
		t.Skip("platform has no bundled tools")
	}
	for _, tc := range []struct {
		name  string
		args  []string
		input string
		want  string
		code  int
	}{
		{"rg", []string{"--version"}, "", "ripgrep 15.2.0", 0},
		{"fzf", []string{"--version"}, "", "0.74.5", 0},
		{"rg", []string{"--help"}, "", "ripgrep", 0},
		{"fzf", []string{"--help"}, "", "--filter", 0},
		{"rg", []string{"-n", "needle", "-"}, "needle\nother\n", "1:needle\n", 0},
		{"fzf", []string{"--filter", "needle"}, "other\nneedle\n", "needle\n", 0},
		{"rg", []string{"missing", "-"}, "needle\n", "", 1},
		{"rg", []string{"--invalid-crowcode-test-flag"}, "", "", 2},
		{"fzf", []string{"--filter", "missing"}, "needle\n", "", 1},
	} {
		t.Run(tc.name+"/"+strings.Join(tc.args, " "), func(t *testing.T) {
			command := newToolCommand(tc.name, "test")
			command.SetArgs(tc.args)
			command.SetIn(strings.NewReader(tc.input))
			var stdout, stderr bytes.Buffer
			command.SetOut(&stdout)
			command.SetErr(&stderr)
			err := command.Execute()
			if tc.code == 0 && err != nil {
				t.Fatalf("unexpected error: %v (%s)", err, stderr.String())
			}
			if tc.code != 0 {
				var exitErr *exec.ExitError
				if !errors.As(err, &exitErr) || exitErr.ExitCode() != tc.code {
					t.Fatalf("expected exit code %d, got %v", tc.code, err)
				}
			}
			if !strings.Contains(stdout.String(), tc.want) {
				t.Fatalf("expected %q in output %q", tc.want, stdout.String())
			}
		})
	}
}
