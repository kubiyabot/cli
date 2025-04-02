package mcp

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"runtime"
	"strings"

	"github.com/spf13/afero"
	"gopkg.in/yaml.v3"
)

const (
	// KUBIYA_DIR is the name of the configuration directory in the user's home.
	KUBIYA_DIR = ".kubiya"
	// DefaultMcpDirName is the name of the directory within the user's Kubiya config dir.
	DefaultMcpDirName = "mcp"
	// Claude Desktop Config
	ClaudeDesktopDefaultConfigName = "claude_desktop.yaml"
	// Cursor IDE Config
	CursorDefaultConfigName = "cursor_ide.yaml"
	// MCP Info File
	McpInfoFilename = ".kubiya_mcp_info.json"
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

// GetMcpConfigDir returns the absolute path to the MCP configuration directory (~/.kubiya/mcp).
// It creates the directory if it doesn't exist, using the provided filesystem.
func GetMcpConfigDir(fs afero.Fs) (string, error) {
	kubiyaDir, err := GetKubiyaDir()
	if err != nil {
		return "", err
	}
	mcpDir := filepath.Join(kubiyaDir, DefaultMcpDirName)

	if err := fs.MkdirAll(mcpDir, 0750); err != nil {
		return "", fmt.Errorf("failed to create MCP config directory '%s': %w", mcpDir, err)
	}

	return mcpDir, nil
}

// GetMcpGatewayDir returns the expected path for the cloned mcp-gateway repository (~/.kubiya/mcp-gateway).
func GetMcpGatewayDir() (string, error) {
	kubiyaDir, err := GetKubiyaDir()
	if err != nil {
		return "", err
	}
	gatewayDir := filepath.Join(kubiyaDir, "mcp-gateway") // Note: constant name might be better
	return gatewayDir, nil
}

// DefaultClaudeDesktopYAML holds the default configuration template for Claude Desktop.
// Note: The actual paths for UvPath and McpGatewayPath will be determined during the 'apply' phase.
const DefaultClaudeDesktopYAML = `# ~/.kubiya/mcp/claude_desktop.yaml
# Configuration for Claude Desktop MCP integration

# Target application name (used for display)
name: Claude Desktop

# Operating System compatibility (only MacOS for now)
os: darwin

# Path to the target configuration file that will be generated
# Supports ~ for home directory expansion
target_file: "~/Library/Application Support/Claude/claude_desktop_config.json"

# Go template for the target configuration file content.
# Available context variables (populated during 'kubiya mcp apply'):
#   .ApiKey: The Kubiya API key from the CLI config
#   .TeammateUUIDsJSON: JSON string array of configured teammate UUIDs.
#   .McpGatewayPath: Absolute path to the cloned mcp-gateway checkout.
#   .UvPath: Absolute path to the 'uv' executable.
template: |
  {
    "mcpServers": {
      "kubiya-local": {
        "command": "/bin/sh",
        "args": [
          "{{ .WrapperScriptPath }}"
        ],
        "env": {
          "TEAMMATE_UUIDS": "{{ .TeammateUUIDsCommaSeparated }}",
          "KUBIYA_API_KEY": "{{ .ApiKey }}"
        }
      }
    }
  }

# Optional: Validation steps (future enhancement)
# validation:
#   check_file_exists:
#     - "~/Library/Application Support/Claude" # Check if Claude app support dir exists
#   check_command_exists:
#     - "{{ .UvPath }}" # Check if uv command exists after install
`

// Default Cursor IDE YAML
const DefaultCursorYAML = `# ~/.kubiya/mcp/cursor_ide.yaml
# Configuration for Cursor IDE MCP integration.
# The output file will be ~/.cursor/mcp.json

name: Cursor IDE

# target_file: Determined automatically as ~/.cursor/mcp.json

# Template for ~/.cursor/mcp.json content.
# This defines the server entry for Kubiya.
template: |
  {
    "mcpServers": {
      "kubiya-local": {
        "command": "/bin/sh",
        "args": [
          "{{ .WrapperScriptPath }}"
        ],
        "env": {
          "TEAMMATE_UUIDS": "{{ .TeammateUUIDsCommaSeparated }}",
          "KUBIYA_API_KEY": "{{ .ApiKey }}"
        }
      }
    }
  }
`

// GetDefaultConfigs returns a map of default config filenames to their content.
func GetDefaultConfigs() map[string]string {
	defaults := make(map[string]string)
	currentOS := runtime.GOOS

	// Add Claude Desktop if on MacOS
	if currentOS == "darwin" {
		defaults[ClaudeDesktopDefaultConfigName] = DefaultClaudeDesktopYAML
	}

	// Add Cursor IDE (compatible with Mac, Linux, Windows)
	// We don't need an OS check here since the template handles paths
	defaults[CursorDefaultConfigName] = DefaultCursorYAML

	return defaults
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

// LoadProviderConfig loads and parses a specific provider's YAML configuration using the provided filesystem.
func LoadProviderConfig(fs afero.Fs, providerName string) (*ProviderConfig, error) {
	mcpDir, err := GetMcpConfigDir(fs)
	if err != nil {
		return nil, fmt.Errorf("failed to get MCP config directory: %w", err)
	}

	// Ensure providerName is just the base name (e.g., "claude_desktop")
	providerName = strings.TrimSuffix(providerName, ".yaml")
	filename := providerName + ".yaml"
	filePath := filepath.Join(mcpDir, filename)

	if _, err := fs.Stat(filePath); os.IsNotExist(err) {
		return nil, fmt.Errorf("provider configuration file not found: %s", filename)
	} else if err != nil {
		return nil, fmt.Errorf("failed to access provider file '%s': %w", filename, err)
	}

	content, err := afero.ReadFile(fs, filePath)
	if err != nil {
		return nil, fmt.Errorf("failed to read provider file '%s': %w", filename, err)
	}

	var config ProviderConfig
	err = yaml.Unmarshal(content, &config)
	if err != nil {
		return nil, fmt.Errorf("failed to parse YAML from '%s': %w", filename, err)
	}

	// Basic validation
	if config.Name == "" {
		return nil, fmt.Errorf("missing required field 'name' in %s", filename)
	}
	if config.Template == "" {
		return nil, fmt.Errorf("missing required field 'template' in %s", filename)
	}

	return &config, nil
}

// ListProviders scans the MCP config directory and parses basic info from YAML files using the provided filesystem.
func ListProviders(fs afero.Fs) ([]ProviderInfo, error) {
	mcpDir, err := GetMcpConfigDir(fs) // Ensure dir conceptually exists for path
	if err != nil {
		return nil, fmt.Errorf("failed to get MCP config directory path: %w", err)
	}

	// Check if the directory actually exists on the given FS
	if _, err := fs.Stat(mcpDir); os.IsNotExist(err) {
		return []ProviderInfo{}, nil // No directory means no providers
	} else if err != nil {
		return nil, fmt.Errorf("failed to check MCP config directory '%s': %w", mcpDir, err)
	}

	files, err := afero.ReadDir(fs, mcpDir)
	if err != nil {
		return nil, fmt.Errorf("failed to read MCP config directory '%s': %w", mcpDir, err)
	}

	var providers []ProviderInfo
	for _, file := range files {
		if file.IsDir() || filepath.Ext(file.Name()) != ".yaml" {
			continue // Skip directories and non-YAML files
		}

		filePath := filepath.Join(mcpDir, file.Name())
		content, err := afero.ReadFile(fs, filePath)
		if err != nil {
			fmt.Fprintf(os.Stderr, "⚠️ Warning: Failed to read MCP config file '%s': %v\n", file.Name(), err)
			continue // Skip files that can't be read
		}

		// Basic structure to parse just the name and OS fields for now
		var basicInfo struct {
			Name string `yaml:"name"`
			OS   string `yaml:"os"`
		}

		err = yaml.Unmarshal(content, &basicInfo)
		if err != nil {
			fmt.Fprintf(os.Stderr, "⚠️ Warning: Failed to parse YAML from '%s': %v\n", file.Name(), err)
			continue // Skip files that can't be parsed
		}

		providers = append(providers, ProviderInfo{
			Filename: file.Name(),
			Name:     basicInfo.Name,
			OS:       basicInfo.OS,
		})
	}

	return providers, nil
}

// Helper function to expand tilde (~) in paths
func ExpandTilde(path string) (string, error) {
	if !strings.HasPrefix(path, "~") {
		return path, nil // Nothing to expand
	}

	homeDir, err := os.UserHomeDir()
	if err != nil {
		return "", fmt.Errorf("cannot get user home directory: %w", err)
	}

	return filepath.Join(homeDir, path[1:]), nil
}

// McpInfo holds metadata about the installed mcp-gateway.
type McpInfo struct {
	InstalledSHA string `json:"installed_sha"`
}

// getMcpInfoFilePath returns the path to the info file within the gateway directory.
func getMcpInfoFilePath() (string, error) {
	gatewayDir, err := GetMcpGatewayDir()
	if err != nil {
		return "", err
	}
	return filepath.Join(gatewayDir, McpInfoFilename), nil
}

// StoreGatewaySHA writes the installed SHA to the info file using the provided filesystem.
func StoreGatewaySHA(fs afero.Fs, sha string) error {
	infoFilePath, err := getMcpInfoFilePath()
	if err != nil {
		return fmt.Errorf("failed to get info file path: %w", err)
	}

	info := McpInfo{InstalledSHA: sha}
	data, err := json.MarshalIndent(info, "", "  ")
	if err != nil {
		return fmt.Errorf("failed to marshal mcp info: %w", err)
	}

	gatewayDir := filepath.Dir(infoFilePath)
	if err := fs.MkdirAll(gatewayDir, 0750); err != nil {
		return fmt.Errorf("failed to ensure gateway directory exists for info file: %w", err)
	}

	if err := afero.WriteFile(fs, infoFilePath, data, 0640); err != nil {
		return fmt.Errorf("failed to write mcp info file '%s': %w", infoFilePath, err)
	}
	return nil
}

// ReadGatewaySHA reads the installed SHA from the info file using the provided filesystem.
func ReadGatewaySHA(fs afero.Fs) (string, error) {
	infoFilePath, err := getMcpInfoFilePath()
	if err != nil {
		return "", fmt.Errorf("failed to get info file path: %w", err)
	}

	if _, err := fs.Stat(infoFilePath); os.IsNotExist(err) {
		// File doesn't exist, maybe install was interrupted or old version?
		return "", fmt.Errorf("mcp info file not found: %s (try running install again)", McpInfoFilename)
	} else if err != nil {
		return "", fmt.Errorf("failed to access mcp info file '%s': %w", infoFilePath, err)
	}

	data, err := afero.ReadFile(fs, infoFilePath)
	if err != nil {
		return "", fmt.Errorf("failed to read mcp info file '%s': %w", infoFilePath, err)
	}

	var info McpInfo
	if err := json.Unmarshal(data, &info); err != nil {
		return "", fmt.Errorf("failed to parse mcp info file '%s': %w", infoFilePath, err)
	}

	return info.InstalledSHA, nil
}
