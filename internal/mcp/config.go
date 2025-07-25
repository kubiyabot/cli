package mcp

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"

	"github.com/kubiyabot/cli/internal/mcp/filter"
	"github.com/spf13/afero"
)

// Configuration defines configuration for the MCP server
type Configuration struct {
	WhitelistedTools    []WhitelistedTool `json:"whitelisted_tools,omitempty" yaml:"whitelisted_tools,omitempty"`
	ToolContexts        []ToolContext     `json:"tool_contexts,omitempty" yaml:"tool_contexts,omitempty"`
	EnableRunners       bool              `json:"enable_runners" yaml:"enable_runners"`
	AllowPlatformAPIs   bool              `json:"allow_platform_apis" yaml:"allow_platform_apis"`
	EnableOPAPolicies   bool              `json:"enable_opa_policies" yaml:"enable_opa_policies"`
	AllowDynamicTools   bool              `json:"allow_dynamic_tools" yaml:"allow_dynamic_tools"`
	VerboseLogging      bool              `json:"verbose_logging" yaml:"verbose_logging"`
	EnableDocumentation bool              `json:"enable_documentation" yaml:"enable_documentation"`
}

// Config defines complete configuration for the production MCP server
type Config struct {
	ServerName             string              `json:"server_name" yaml:"server_name"`
	ServerVersion          string              `json:"server_version" yaml:"server_version"`
	SessionTimeout         int                 `json:"session_timeout" yaml:"session_timeout"` // in seconds
	RequireAuth            bool                `json:"require_auth" yaml:"require_auth"`
	EnableTimeRestrictions bool                `json:"enable_time_restrictions" yaml:"enable_time_restrictions"`
	FeatureFlags           map[string]bool     `json:"feature_flags,omitempty" yaml:"feature_flags,omitempty"`
	ToolPermissions        map[string][]string `json:"tool_permissions,omitempty" yaml:"tool_permissions,omitempty"`
	ToolTimeouts           map[string]int      `json:"tool_timeouts,omitempty" yaml:"tool_timeouts,omitempty"` // in seconds
	WhitelistedTools       []WhitelistedTool   `json:"whitelisted_tools,omitempty" yaml:"whitelisted_tools,omitempty"`
	ToolContexts           []ToolContext       `json:"tool_contexts,omitempty" yaml:"tool_contexts,omitempty"`
	RateLimit              RateLimitConfig     `json:"rate_limit" yaml:"rate_limit"`

	// Core functionality flags
	EnableRunners       bool `json:"enable_runners" yaml:"enable_runners"`
	AllowPlatformAPIs   bool `json:"allow_platform_apis" yaml:"allow_platform_apis"`
	EnableOPAPolicies   bool `json:"enable_opa_policies" yaml:"enable_opa_policies"`
	AllowDynamicTools   bool `json:"allow_dynamic_tools" yaml:"allow_dynamic_tools"`
	VerboseLogging      bool `json:"verbose_logging" yaml:"verbose_logging"`
	EnableDocumentation bool `json:"enable_documentation" yaml:"enable_documentation"`

	// User organization ID (set automatically from user config)
	OrgID string `json:"org_id,omitempty" yaml:"org_id,omitempty"`
}

// RateLimitConfig defines rate limiting configuration
type RateLimitConfig struct {
	RequestsPerSecond float64 `json:"requests_per_second" yaml:"requests_per_second"`
	Burst             int     `json:"burst" yaml:"burst"`
}

// WhitelistedTool defines a preconfigured tool exposed via MCP
// This now embeds the full Kubiya Tool definition with additional MCP-specific overrides
type WhitelistedTool struct {
	Name        string      `json:"name" yaml:"name"`
	Source      ToolSource  `json:"source"`
	Description string      `json:"description" yaml:"description"`
	Args        []ToolArg   `json:"args" yaml:"args,omitempty"`
	Env         []string    `json:"env" yaml:"env,omitempty"`
	Content     string      `json:"content" yaml:"content,omitempty"`
	FileName    string      `json:"file_name" yaml:"file_name,omitempty"`
	Secrets     []string    `json:"secrets,omitempty"`
	IconURL     string      `json:"icon_url,omitempty"`
	Type        string      `json:"type,omitempty"`
	Alias       string      `json:"alias,omitempty"`
	WithFiles   interface{} `json:"with_files,omitempty"`   // Can be []string or map[string]interface{}
	WithVolumes interface{} `json:"with_volumes,omitempty"` // Can be []string or map[string]interface{}
	LongRunning bool        `json:"long_running,omitempty"`
	Metadata    interface{} `json:"metadata,omitempty"` // Can be []string or other formats
	Mermaid     string      `json:"mermaid,omitempty"`
	Image       string      `json:"image,omitempty"`

	// MCP-specific overrides
	Integrations  []string                          `json:"integrations,omitempty" yaml:"integrations,omitempty"` // Integration templates to apply
	Parameters    map[string]interface{}            `json:"parameters,omitempty" yaml:"parameters,omitempty"`
	DefaultConfig map[string]interface{}            `json:"default_config,omitempty" yaml:"default_config,omitempty"`
	Arguments     map[string]map[string]interface{} `json:"arguments,omitempty" yaml:"arguments,omitempty"`
	Runner        string                            `json:"runner,omitempty" yaml:"runner,omitempty"`
	Timeout       int                               `json:"timeout,omitempty" yaml:"timeout,omitempty"` // in seconds
}

// ToolSource embedded struct for tool source information
type ToolSource struct {
	ID  string `json:"id"`
	URL string `json:"url"`
}

// ToolArg embedded struct for tool arguments
type ToolArg struct {
	Name        string       `yaml:"name" json:"name"`
	Type        string       `yaml:"type,omitempty" json:"type,omitempty"`
	Description string       `yaml:"description" json:"description"`
	Required    bool         `yaml:"required,omitempty" json:"required,omitempty"`
	Default     string       `yaml:"default,omitempty" json:"default,omitempty"`
	Options     []string     `yaml:"options,omitempty" json:"options,omitempty"`
	OptionsFrom *OptionsFrom `yaml:"options_from,omitempty" json:"options_from,omitempty"`
}

// OptionsFrom struct for dynamic options
type OptionsFrom struct {
	// Add fields as needed for dynamic options
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
func LoadConfiguration(fs afero.Fs, configFile string, disablePlatformAPIsFlag bool, whitelistedTools []string) (*Configuration, error) {
	// Start with default config - platform APIs enabled by default, dynamic tools enabled, verbose logging disabled
	config := &Configuration{
		EnableRunners:     true,
		AllowPlatformAPIs: true,
		AllowDynamicTools: true,
		VerboseLogging:    false,
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
				return nil, fmt.Errorf("failed to parse config file %s: %w", configPath, err)
			}
		} else if !os.IsNotExist(err) {
			return nil, fmt.Errorf("failed to read config file %s: %w", configPath, err)
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

	// Override with command line flags
	if disablePlatformAPIsFlag {
		config.AllowPlatformAPIs = false
	}

	// Add whitelisted tools from command line
	if len(whitelistedTools) > 0 {
		for _, toolName := range whitelistedTools {
			config.WhitelistedTools = append(config.WhitelistedTools, WhitelistedTool{
				Name:        toolName,
				Description: fmt.Sprintf("Tool %s added via command line", toolName),
			})
		}
	}

	return config, nil
}

// LoadProductionConfig loads configuration for the production MCP server
func LoadProductionConfig(fs afero.Fs, configFile string, disablePlatformAPIsFlag bool, whitelistedTools []string) (*Config, error) {
	// Start with default config - platform APIs enabled by default, dynamic tools enabled, verbose logging disabled
	config := &Config{
		ServerName:          "Kubiya MCP Server",
		ServerVersion:       "1.0.0",
		SessionTimeout:      1800, // 30 minutes
		RequireAuth:         false,
		EnableRunners:       true,
		AllowPlatformAPIs:   true,
		AllowDynamicTools:   true,
		VerboseLogging:      false,
		EnableDocumentation: false,
		RateLimit: RateLimitConfig{
			RequestsPerSecond: 10.0,
			Burst:             20,
		},
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
				return nil, fmt.Errorf("failed to parse config file %s: %w", configPath, err)
			}
		} else if !os.IsNotExist(err) {
			return nil, fmt.Errorf("failed to read config file %s: %w", configPath, err)
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

	if envAuth := os.Getenv("KUBIYA_MCP_REQUIRE_AUTH"); envAuth != "" {
		config.RequireAuth = envAuth == "true"
	}

	// Override with command line flags
	if disablePlatformAPIsFlag {
		config.AllowPlatformAPIs = false
	}

	// Add whitelisted tools from command line
	if len(whitelistedTools) > 0 {
		for _, toolName := range whitelistedTools {
			config.WhitelistedTools = append(config.WhitelistedTools, WhitelistedTool{
				Name:        toolName,
				Description: fmt.Sprintf("Tool %s added via command line", toolName),
			})
		}
	}

	// Set default tool permissions if not provided
	if config.ToolPermissions == nil {
		config.ToolPermissions = filter.DefaultToolPermissions()
	}

	return config, nil
}
