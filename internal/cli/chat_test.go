package cli

import (
	"bytes"
	"testing"

	"github.com/kubiyabot/cli/internal/config"
)

func TestChatCommand(t *testing.T) {
	tests := []struct {
		name        string
		args        []string
		wantErr     bool
		wantContain string
	}{
		{
			name:        "chat help",
			args:        []string{"chat", "--help"},
			wantErr:     false,
			wantContain: "Chat with a teammate",
		},
		{
			name:        "chat without required flags",
			args:        []string{"chat"},
			wantErr:     true,
			wantContain: "required in non-interactive mode",
		},
		{
			name:        "chat with name but no message",
			args:        []string{"chat", "--name", "test"},
			wantErr:     true,
			wantContain: "message is required",
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