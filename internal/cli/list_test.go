package cli

import (
	"bytes"
	"testing"
)

func TestListCommand(t *testing.T) {
	tests := []struct {
		name        string
		args        []string
		wantErr     bool
		wantContain string
	}{
		{
			name:        "list help",
			args:        []string{"list", "--help"},
			wantErr:     false,
			wantContain: "List available teammates",
		},
		{
			name:        "list with json output",
			args:        []string{"list", "--output", "json"},
			wantErr:     false,
			wantContain: "[",  // Expect JSON array
		},
		{
			name:        "list with invalid output format",
			args:        []string{"list", "--output", "invalid"},
			wantErr:     true,
			wantContain: "unknown output format",
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