package mcp

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"

	"github.com/spf13/afero"
)

// Configuration defines configuration for the MCP server
type Configuration struct {
	WhitelistedTools  []WhitelistedTool `json:"whitelisted_tools,omitempty" yaml:"whitelisted_tools,omitempty"`
	ToolContexts      []ToolContext     `json:"tool_contexts,omitempty" yaml:"tool_contexts,omitempty"`
	EnableRunners     bool              `json:"enable_runners" yaml:"enable_runners"`
	AllowPlatformAPIs bool              `json:"allow_platform_apis" yaml:"allow_platform_apis"`
	EnableOPAPolicies bool              `json:"enable_opa_policies" yaml:"enable_opa_policies"`
}

// WhitelistedTool defines a preconfigured tool exposed via MCP
type WhitelistedTool struct {
	Name         string                 `json:"name" yaml:"name"`
	Description  string                 `json:"description" yaml:"description"`
	ToolName     string                 `json:"tool_name" yaml:"tool_name"`       // Kubiya tool name
	Integrations []string               `json:"integrations" yaml:"integrations"` // Integration templates to apply
	Parameters   map[string]interface{} `json:"parameters,omitempty" yaml:"parameters,omitempty"`
}

// ToolContext provides context information about tools
type ToolContext struct {
	Type        string `json:"type" yaml:"type"`               // kubernetes, aws, docker, etc.
	Description string `json:"description" yaml:"description"` // Human-readable context info
	Examples    []struct {
		Name        string `json:"name" yaml:"name"`
		Description string `json:"description" yaml:"description"`
		Command     string `json:"command" yaml:"command"`
	} `json:"examples,omitempty" yaml:"examples,omitempty"`
}

// LoadConfiguration loads MCP server configuration from file and environment variables
func LoadConfiguration(fs afero.Fs, configFile string, allowPlatformAPIsFlag bool) (*Configuration, error) {
	// Start with default config
	config := &Configuration{
		EnableRunners:     true,
		AllowPlatformAPIs: false,
	}

	// Use provided config file or default location
	configPath := configFile
	if configPath == "" {
		homeDir, err := os.UserHomeDir()
		if err == nil {
			configPath = filepath.Join(homeDir, ".kubiya", "mcp-server.json")
		}
	}

	// Load from file if it exists
	if configPath != "" {
		data, err := afero.ReadFile(fs, configPath)
		if err == nil {
			if err := json.Unmarshal(data, config); err != nil {
				return nil, fmt.Errorf("failed to parse %s: %w", configPath, err)
			}
		} else if !os.IsNotExist(err) {
			return nil, err
		}
	}

	// Override with environment variables
	if envRunners := os.Getenv("KUBIYA_MCP_ENABLE_RUNNERS"); envRunners != "" {
		config.EnableRunners = envRunners == "true"
	}

	if envPlatformAPIs := os.Getenv("KUBIYA_MCP_ALLOW_PLATFORM_APIS"); envPlatformAPIs != "" {
		config.AllowPlatformAPIs = envPlatformAPIs == "true"
	}

	if envOPAPolicies := os.Getenv("KUBIYA_OPA_ENFORCE"); envOPAPolicies != "" {
		config.EnableOPAPolicies = envOPAPolicies == "true" || envOPAPolicies == "1"
	}

	// Override with command line flag if provided
	if allowPlatformAPIsFlag {
		config.AllowPlatformAPIs = true
	}

	return config, nil
}