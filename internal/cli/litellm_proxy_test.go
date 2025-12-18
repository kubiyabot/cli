package cli

import (
	"os"
	"strings"
	"testing"
)

func TestIsLocalProxyEnabled(t *testing.T) {
	tests := []struct {
		name     string
		settings map[string]interface{}
		expected bool
	}{
		{
			name:     "enabled",
			settings: map[string]interface{}{"enable_local_litellm_proxy": true},
			expected: true,
		},
		{
			name:     "disabled",
			settings: map[string]interface{}{"enable_local_litellm_proxy": false},
			expected: false,
		},
		{
			name:     "missing",
			settings: map[string]interface{}{},
			expected: false,
		},
		{
			name:     "nil settings",
			settings: nil,
			expected: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := IsLocalProxyEnabled(tt.settings)
			if result != tt.expected {
				t.Errorf("IsLocalProxyEnabled() = %v, want %v", result, tt.expected)
			}
		})
	}
}

func TestGetProxyTimeoutSettings(t *testing.T) {
	tests := []struct {
		name                    string
		settings                map[string]interface{}
		expectedTimeoutSeconds  int
		expectedMaxRetries      int
	}{
		{
			name:                   "default values",
			settings:               map[string]interface{}{},
			expectedTimeoutSeconds: 10,
			expectedMaxRetries:     3,
		},
		{
			name: "custom values as float64",
			settings: map[string]interface{}{
				"local_proxy_timeout_seconds": 15.0,
				"local_proxy_max_retries":     5.0,
			},
			expectedTimeoutSeconds: 15,
			expectedMaxRetries:     5,
		},
		{
			name: "custom values as int",
			settings: map[string]interface{}{
				"local_proxy_timeout_seconds": 20,
				"local_proxy_max_retries":     7,
			},
			expectedTimeoutSeconds: 20,
			expectedMaxRetries:     7,
		},
		{
			name:                   "nil settings",
			settings:               nil,
			expectedTimeoutSeconds: 10,
			expectedMaxRetries:     3,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			timeoutSeconds, maxRetries := GetProxyTimeoutSettings(tt.settings)
			if timeoutSeconds != tt.expectedTimeoutSeconds {
				t.Errorf("timeoutSeconds = %d, want %d", timeoutSeconds, tt.expectedTimeoutSeconds)
			}
			if maxRetries != tt.expectedMaxRetries {
				t.Errorf("maxRetries = %d, want %d", maxRetries, tt.expectedMaxRetries)
			}
		})
	}
}

func TestParseLiteLLMConfigFromSettings(t *testing.T) {
	tests := []struct {
		name        string
		settings    map[string]interface{}
		expectError bool
		checkConfig func(*testing.T, *LiteLLMProxyConfig)
	}{
		{
			name: "valid minimal config",
			settings: map[string]interface{}{
				"litellm_config": map[string]interface{}{
					"model_list": []interface{}{
						map[string]interface{}{
							"model_name": "gpt-4",
							"litellm_params": map[string]interface{}{
								"model": "azure/gpt-4",
							},
						},
					},
				},
			},
			expectError: false,
			checkConfig: func(t *testing.T, config *LiteLLMProxyConfig) {
				if len(config.ModelList) != 1 {
					t.Errorf("expected 1 model, got %d", len(config.ModelList))
				}
				if config.ModelList[0].ModelName != "gpt-4" {
					t.Errorf("expected model name 'gpt-4', got '%s'", config.ModelList[0].ModelName)
				}
			},
		},
		{
			name: "valid full config",
			settings: map[string]interface{}{
				"litellm_config": map[string]interface{}{
					"model_list": []interface{}{
						map[string]interface{}{
							"model_name": "gpt-4",
							"litellm_params": map[string]interface{}{
								"model":    "azure/gpt-4",
								"api_base": "https://example.com",
								"api_key":  "env:AZURE_KEY",
							},
						},
					},
					"litellm_settings": map[string]interface{}{
						"success_callback": []interface{}{"langfuse"},
					},
					"environment_variables": map[string]interface{}{
						"LANGFUSE_PUBLIC_KEY": "pk-test",
						"LANGFUSE_SECRET_KEY": "sk-test",
					},
				},
			},
			expectError: false,
			checkConfig: func(t *testing.T, config *LiteLLMProxyConfig) {
				if len(config.ModelList) != 1 {
					t.Errorf("expected 1 model, got %d", len(config.ModelList))
				}
				if config.LiteLLMSettings == nil {
					t.Error("expected litellm_settings to be present")
				}
				if config.EnvironmentVariables == nil {
					t.Error("expected environment_variables to be present")
				}
				if config.EnvironmentVariables["LANGFUSE_PUBLIC_KEY"] != "pk-test" {
					t.Errorf("expected LANGFUSE_PUBLIC_KEY='pk-test', got '%s'", config.EnvironmentVariables["LANGFUSE_PUBLIC_KEY"])
				}
			},
		},
		{
			name: "missing litellm_config",
			settings: map[string]interface{}{
				"other_setting": "value",
			},
			expectError: true,
		},
		{
			name: "invalid litellm_config type",
			settings: map[string]interface{}{
				"litellm_config": "not a map",
			},
			expectError: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			config, err := ParseLiteLLMConfigFromSettings(tt.settings)

			if tt.expectError {
				if err == nil {
					t.Error("expected error but got none")
				}
				return
			}

			if err != nil {
				t.Errorf("unexpected error: %v", err)
				return
			}

			if config == nil {
				t.Error("expected config but got nil")
				return
			}

			if tt.checkConfig != nil {
				tt.checkConfig(t, config)
			}
		})
	}
}

func TestFindAvailablePort(t *testing.T) {
	port, err := findAvailablePort()
	if err != nil {
		t.Errorf("findAvailablePort() error = %v", err)
		return
	}

	if port < 1024 || port > 65535 {
		t.Errorf("findAvailablePort() = %d, expected port in range 1024-65535", port)
	}

	// Try to find another port to ensure it's different (usually)
	port2, err := findAvailablePort()
	if err != nil {
		t.Errorf("findAvailablePort() second call error = %v", err)
		return
	}

	if port2 < 1024 || port2 > 65535 {
		t.Errorf("findAvailablePort() second call = %d, expected port in range 1024-65535", port2)
	}
}

func TestWriteLiteLLMConfig(t *testing.T) {
	// Create a temporary directory for testing
	tempDir := t.TempDir()
	configPath := tempDir + "/test_config.yaml"

	config := &LiteLLMProxyConfig{
		ModelList: []LiteLLMModel{
			{
				ModelName: "test-model",
				LiteLLMParams: map[string]interface{}{
					"model": "test/model",
				},
			},
		},
	}

	err := writeLiteLLMConfig(configPath, config)
	if err != nil {
		t.Errorf("writeLiteLLMConfig() error = %v", err)
		return
	}

	// Verify file exists
	if _, err := os.Stat(configPath); os.IsNotExist(err) {
		t.Error("config file was not created")
		return
	}

	// Read and verify content
	content, err := os.ReadFile(configPath)
	if err != nil {
		t.Errorf("failed to read config file: %v", err)
		return
	}

	// Basic content check
	contentStr := string(content)
	if !strings.Contains(contentStr, "test-model") {
		t.Error("config file does not contain expected model name")
	}
}
