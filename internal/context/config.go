package context

import (
	"fmt"
	"os"
	"path/filepath"

	"gopkg.in/yaml.v3"
)

const (
	// DefaultConfigPath is the default path for the kubiya config file
	DefaultConfigPath = "~/.kubiya/config"
	// ConfigAPIVersion is the API version for the config file
	ConfigAPIVersion = "v1"
	// ConfigKind is the kind for the config file
	ConfigKind = "Config"
)

// GetConfigPath returns the config file path, expanding home directory
func GetConfigPath() (string, error) {
	configPath := os.Getenv("KUBIYA_CONFIG")
	if configPath == "" {
		configPath = DefaultConfigPath
	}

	// Expand ~ to home directory
	if configPath[:2] == "~/" {
		home, err := os.UserHomeDir()
		if err != nil {
			return "", fmt.Errorf("failed to get home directory: %w", err)
		}
		configPath = filepath.Join(home, configPath[2:])
	}

	return configPath, nil
}

// LoadConfig loads the config from the file
func LoadConfig() (*Config, error) {
	configPath, err := GetConfigPath()
	if err != nil {
		return nil, err
	}

	// Check if config file exists
	if _, err := os.Stat(configPath); os.IsNotExist(err) {
		// Return default empty config
		return &Config{
			APIVersion: ConfigAPIVersion,
			Kind:       ConfigKind,
			Contexts:   []NamedContext{},
			Users:      []NamedUser{},
		}, nil
	}

	data, err := os.ReadFile(configPath)
	if err != nil {
		return nil, fmt.Errorf("failed to read config file: %w", err)
	}

	var config Config
	if err := yaml.Unmarshal(data, &config); err != nil {
		return nil, fmt.Errorf("failed to parse config file: %w", err)
	}

	return &config, nil
}

// SaveConfig saves the config to the file
func SaveConfig(config *Config) error {
	configPath, err := GetConfigPath()
	if err != nil {
		return err
	}

	// Ensure directory exists
	configDir := filepath.Dir(configPath)
	if err := os.MkdirAll(configDir, 0755); err != nil {
		return fmt.Errorf("failed to create config directory: %w", err)
	}

	// Ensure API version and kind are set
	if config.APIVersion == "" {
		config.APIVersion = ConfigAPIVersion
	}
	if config.Kind == "" {
		config.Kind = ConfigKind
	}

	data, err := yaml.Marshal(config)
	if err != nil {
		return fmt.Errorf("failed to marshal config: %w", err)
	}

	if err := os.WriteFile(configPath, data, 0600); err != nil {
		return fmt.Errorf("failed to write config file: %w", err)
	}

	return nil
}

// InitConfig initializes a new config file if it doesn't exist
func InitConfig() error {
	configPath, err := GetConfigPath()
	if err != nil {
		return err
	}

	// Check if config already exists
	if _, err := os.Stat(configPath); err == nil {
		return nil // Config already exists
	}

	// Create default config
	config := &Config{
		APIVersion: ConfigAPIVersion,
		Kind:       ConfigKind,
		Contexts:   []NamedContext{},
		Users:      []NamedUser{},
	}

	return SaveConfig(config)
}
