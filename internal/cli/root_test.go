package cli

import (
	"bytes"
	"testing"

	"github.com/kubiyabot/cli/internal/config"
	"github.com/spf13/cobra"
)

func executeCommand(root *cobra.Command, args ...string) (output string, err error) {
	buf := new(bytes.Buffer)
	root.SetOut(buf)
	root.SetErr(buf)
	root.SetArgs(args)

	err = root.Execute()
	return buf.String(), err
}

func setupTestCommand() (*cobra.Command, *config.Config) {
	cfg := &config.Config{
		APIKey:  "test-key",
		BaseURL: "https://api.test",
		Debug:   false,
	}

	rootCmd := &cobra.Command{Use: "test"}
	rootCmd.AddCommand(
		newListCommand(cfg),
		newChatCommand(cfg),
		newSourcesCommand(cfg),
	)

	return rootCmd, cfg
}

func TestRootCommand(t *testing.T) {
	tests := []struct {
		name        string
		args        []string
		wantErr     bool
		wantContain string
	}{
		{
			name:        "help command",
			args:        []string{"--help"},
			wantErr:     false,
			wantContain: "Available Commands:",
		},
		{
			name:    "invalid command",
			args:    []string{"invalid"},
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			cmd, _ := setupTestCommand()
			got, err := executeCommand(cmd, tt.args...)

			if (err != nil) != tt.wantErr {
				t.Errorf("command error = %v, wantErr %v", err, tt.wantErr)
				return
			}

			if tt.wantContain != "" && !bytes.Contains([]byte(got), []byte(tt.wantContain)) {
				t.Errorf("command output = %v, want to contain %v", got, tt.wantContain)
			}
		})
	}
} 