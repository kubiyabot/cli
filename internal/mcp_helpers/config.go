package mcp_helpers

import (
	"fmt"
	"os"
	"path/filepath"

	"github.com/spf13/afero"
	"gopkg.in/yaml.v3"
)

type KubiyaMcpConfig struct {
	Version     string   `yaml:"version"` // Required: Version of the configuration
	ApiKey      string   `yaml:"api_key"`
	AgentIds []string `yaml:"agent_ids,omitempty"` // Optional: List of agent IDs
}

const (
	// KUBIYA_DIR is the name of the configuration directory in the user's home.
	KUBIYA_DIR = ".kubiya"
	// DefaultMcpFileName is the name of the directory within the user's Kubiya config dir.
	DefaultMcpFileName = "mcp_config.yaml"
	// Claude Desktop Config
	ClaudeDesktopDefaultConfigName = "claude_desktop.yaml"
	// Cursor IDE Config
	CursorDefaultConfigName = "cursor_ide.yaml"
)

// GetKubiyaDir returns the absolute path to the root Kubiya configuration directory (~/.kubiya).
// It does *not* create the directory.
func GetKubiyaDir() (string, error) {
	homeDir, err := os.UserHomeDir()
	if err != nil {
		return "", fmt.Errorf("failed to get user home directory: %w", err)
	}
	return filepath.Join(homeDir, KUBIYA_DIR), nil
}

// GetMcpConfigFile returns the absolute path to the MCP configuration file (~/.kubiya/mcp_config.yaml).
// It creates the directory if it doesn't exist, using the provided filesystem.
func GetMcpConfigFile(fs afero.Fs) (string, error) {
	kubiyaDir, err := GetKubiyaDir()
	if err != nil {
		return "", err
	}

	if err := fs.MkdirAll(kubiyaDir, 0750); err != nil {
		return "", fmt.Errorf("failed to create kubiya config directory '%s': %w", kubiyaDir, err)
	}

	ret := filepath.Join(kubiyaDir, DefaultMcpFileName)
	return ret, nil
}

func SaveMcpConfig(fs afero.Fs, apiKey string, agentIds []string) error {
	payload := KubiyaMcpConfig{
		Version:     "1.0",
		ApiKey:      apiKey,
		AgentIds: agentIds,
	}
	yamlData, err := yaml.Marshal(payload)
	if err != nil {
		return fmt.Errorf("failed to marshal YAML: %w", err)
	}
	configPath, err := GetMcpConfigFile(fs)
	if err != nil {
		return fmt.Errorf("failed to get MCP config file path: %w", err)
	}
	if err := afero.WriteFile(fs, configPath, yamlData, 0644); err != nil {
		return fmt.Errorf("failed to write MCP config file: %w", err)
	}
	return nil
}

func LoadMcpConfig(fs afero.Fs) (*KubiyaMcpConfig, error) {
	configPath, err := GetMcpConfigFile(fs)
	if err != nil {
		return nil, fmt.Errorf("failed to get MCP config file path: %w", err)
	}

	file, err := fs.Open(configPath)
	if err != nil {
		return nil, fmt.Errorf("failed to open MCP config file: %w", err)
	}
	defer file.Close()

	var config KubiyaMcpConfig
	if err := yaml.NewDecoder(file).Decode(&config); err != nil {
		return nil, fmt.Errorf("failed to decode YAML: %w", err)
	}
	return &config, nil
}

// ProviderInfo holds basic information parsed from an MCP config YAML.
type ProviderInfo struct {
	Filename string // Original filename (e.g., claude_desktop.yaml)
	Name     string // Display name from the YAML file (e.g., "Claude Desktop")
	OS       string // Target OS from the YAML file (e.g., "darwin")
	// Add more fields later as needed (e.g., target_file, status)
}

// ProviderConfig holds the full configuration parsed from an MCP config YAML.
type ProviderConfig struct {
	Name       string `yaml:"name"`
	OS         string `yaml:"os,omitempty"` // Optional: Target OS (e.g., "darwin", "linux", "windows")
	TargetFile string `yaml:"target_file,omitempty"`
	Template   string `yaml:"template"` // Go template string for the target file content
	// Validation map[string][]string `yaml:"validation,omitempty"` // Future enhancement
}
