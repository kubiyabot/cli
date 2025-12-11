package config

import (
	"os"
	"testing"
)

func TestLoad(t *testing.T) {
	tests := []struct {
		name        string
		envVars     map[string]string
		wantErr     bool
		wantBaseURL string
		wantDebug   bool
	}{
		{
			name: "valid config with all vars",
			envVars: map[string]string{
				"KUBIYA_API_KEY":                   "test-key",
				"KUBIYA_CONTROL_PLANE_BASE_URL":    "https://test.api.kubiya.ai",
				"KUBIYA_DEBUG":                     "true",
			},
			wantBaseURL: "https://test.api.kubiya.ai",
			wantDebug:   true,
		},
		{
			name: "valid config with only required vars",
			envVars: map[string]string{
				"KUBIYA_API_KEY": "test-key",
			},
			wantBaseURL: "https://control-plane.kubiya.ai",
			wantDebug:   false,
		},
		{
			name:        "missing API key",
			envVars:     map[string]string{},
			wantErr:     false,
			wantBaseURL: "https://control-plane.kubiya.ai",
		},
		{
			name: "v1 API mode with flag",
			envVars: map[string]string{
				"KUBIYA_API_KEY":         "test-key",
				"KUBIYA_CLI_USE_V1_API":  "true",
			},
			wantBaseURL: "https://api.kubiya.ai/api/v1",
			wantDebug:   false,
		},
		{
			name: "v1 API mode with custom URL",
			envVars: map[string]string{
				"KUBIYA_API_KEY":         "test-key",
				"KUBIYA_CLI_USE_V1_API":  "true",
				"KUBIYA_BASE_URL":        "https://custom.api.kubiya.ai",
			},
			wantBaseURL: "https://custom.api.kubiya.ai",
			wantDebug:   false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Clear environment
			os.Clearenv()

			// Set test environment variables
			for k, v := range tt.envVars {
				os.Setenv(k, v)
			}

			// Run test
			cfg, err := Load()
			if (err != nil) != tt.wantErr {
				t.Errorf("Load() error = %v, wantErr %v", err, tt.wantErr)
				return
			}

			if err != nil {
				return
			}

			if cfg.BaseURL != tt.wantBaseURL {
				t.Errorf("Load() BaseURL = %v, want %v", cfg.BaseURL, tt.wantBaseURL)
			}

			if cfg.Debug != tt.wantDebug {
				t.Errorf("Load() Debug = %v, want %v", cfg.Debug, tt.wantDebug)
			}

			if cfg.APIKey != tt.envVars["KUBIYA_API_KEY"] {
				t.Errorf("Load() APIKey = %v, want %v", cfg.APIKey, tt.envVars["KUBIYA_API_KEY"])
			}
		})
	}
}
