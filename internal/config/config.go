package config

import (
	"fmt"
	"os"
	"path/filepath"
	"strconv"
	"strings"

	"github.com/golang-jwt/jwt/v4"
)

const (
	// KUBIYA_DIR is the name of the configuration directory in the user's home.
	KUBIYA_DIR = ".kubiya"
	// CONFIG_FILENAME is the name of the config file within KUBIYA_DIR.
	CONFIG_FILENAME = "config.yaml" // Or config.json, config.toml depending on format
)

type Config struct {
	Org         string
	Email       string
	APIKey      string
	BaseURL     string
	ComposerURL string
	Debug       bool
	AutoSession bool
}

// GetConfigFilePath returns the expected full path to the config file.
func GetConfigFilePath() (string, error) {
	homeDir, err := os.UserHomeDir()
	if err != nil {
		return "", fmt.Errorf("failed to get user home directory: %w", err)
	}
	return filepath.Join(homeDir, KUBIYA_DIR, CONFIG_FILENAME), nil
}

// SaveAPIKey saves the provided API key to the config file.
// This is a placeholder - needs proper file writing logic (e.g., YAML/JSON)
func SaveAPIKey(apiKey string) error {
	configPath, err := GetConfigFilePath()
	if err != nil {
		return fmt.Errorf("could not determine config path: %w", err)
	}

	kubiyaDir := filepath.Dir(configPath)
	if err := os.MkdirAll(kubiyaDir, 0750); err != nil {
		return fmt.Errorf("could not create config directory '%s': %w", kubiyaDir, err)
	}

	// TODO: Implement proper config file writing (YAML or JSON)
	// For now, just write the key directly - VERY BASIC
	fmt.Printf("Saving API Key to %s (placeholder implementation)\n", configPath)
	content := fmt.Sprintf("KUBIYA_API_KEY: %s\n", apiKey)                  // Example YAML format
	if err := os.WriteFile(configPath, []byte(content), 0600); err != nil { // owner rw only
		return fmt.Errorf("failed to write config file '%s': %w", configPath, err)
	}
	fmt.Println("API Key saved successfully.")
	return nil
}

func Load() (*Config, error) {
	// TODO: Implement actual config file loading (e.g., Viper, YAML parser)
	// For now, rely solely on environment variables.

	apiKey := os.Getenv("KUBIYA_API_KEY")
	// Don't error out here if APIKey is missing. Check later in commands that need it.
	// if apiKey == "" {
	// 	fmt.Println("KUBIYA_API_KEY environment variable is required in ~/.kubiya/config.yaml or as an environment variable")
	// 	fmt.Println("You can get your API key from https://app.kubiya.ai/api-keys")
	// 	return nil, fmt.Errorf("KUBIYA_API_KEY environment variable is required")
	// }

	baseURL := os.Getenv("KUBIYA_BASE_URL")
	if baseURL == "" {
		baseURL = "https://api.kubiya.ai/api/v1"
	}

	composerURL := os.Getenv("KUBIYA_COMPOSER_URL")
	if composerURL == "" {
		composerURL = "https://compose.kubiya.ai/api"
	}

	// AutoSession is enabled by default but can be overridden by KUBIYA_AUTO_SESSION environment variable
	autoSession := true
	if val, exists := os.LookupEnv("KUBIYA_AUTO_SESSION"); exists {
		if parsed, err := strconv.ParseBool(val); err == nil {
			autoSession = parsed
		}
	}

	cfg := &Config{
		APIKey:      apiKey,
		BaseURL:     baseURL,
		ComposerURL: composerURL,
		Debug:       os.Getenv("KUBIYA_DEBUG") == "true",
		AutoSession: autoSession,
	}

	// Try to decode Org and Email from JWT if API key exists
	if cfg.APIKey != "" {
		// jwtDecoder modifies cfg in place, ignore error for now if decoding fails
		_, _ = cfg.jwtDecoder()
	}

	return cfg, nil // Return the config even if API key is missing
}

func (c *Config) BaseURLV2() string {
	const (
		v1 = "/v1"
		v2 = "/v2"
	)

	return strings.ReplaceAll(c.BaseURL, v1, v2)
}

func (c *Config) jwtDecoder() (*Config, error) {
	token, _, _ := new(jwt.Parser).ParseUnverified(c.APIKey, jwt.MapClaims{})
	if token == nil {
		return c, fmt.Errorf("invalid JWT format")
	}

	if claims, ok := token.Claims.(jwt.MapClaims); ok {
		const (
			emailKey = "email"
			orgKey   = "organization"
		)

		if val, exists := claims[orgKey]; exists {
			if org, ok := val.(string); ok {
				c.Org = org
			}
		}

		if val, exists := claims[emailKey]; exists {
			if email, ok := val.(string); ok {
				c.Email = email
			}
		}
	}

	return c, nil
}
