package cli

import (
	"bytes"
	"testing"

	"github.com/spf13/cobra"
)

func rootExecuteCommand(root *cobra.Command, args ...string) (output string, err error) {
	buf := new(bytes.Buffer)
	root.SetOut(buf)
	root.SetErr(buf)
	root.SetArgs(args)

	err = root.Execute()
	return buf.String(), err
}

// mockSourcesCommand creates a simple mock command for testing
func mockSourcesCommand() *cobra.Command {
	return &cobra.Command{
		Use:   "sources",
		Short: "Mock sources command for testing",
		Run: func(cmd *cobra.Command, args []string) {
			// Do nothing, just a mock
		},
	}
}

func rootSetupTestCommand() *cobra.Command {
	rootCmd := &cobra.Command{Use: "test"}

	// Add the mock sources command
	rootCmd.AddCommand(mockSourcesCommand())

	return rootCmd
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
			cmd := rootSetupTestCommand()
			got, err := rootExecuteCommand(cmd, tt.args...)

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
