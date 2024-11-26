package cli

import (
	"bytes"
	"testing"
)

func TestSourcesCommand(t *testing.T) {
	tests := []struct {
		name        string
		args        []string
		wantErr     bool
		wantContain string
	}{
		{
			name:        "sources help",
			args:        []string{"source", "--help"},
			wantErr:     false,
			wantContain: "Manage sources",
		},
		{
			name:        "list sources",
			args:        []string{"source", "list"},
			wantErr:     false,
			wantContain: "SOURCES",
		},
		{
			name:        "describe without uuid",
			args:        []string{"source", "describe"},
			wantErr:     true,
			wantContain: "requires exactly 1 arg",
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
