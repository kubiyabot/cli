package config

import (
	"fmt"
	"os"
	"path/filepath"
	"strconv"
	"strings"

	"github.com/golang-jwt/jwt/v4"
	"github.com/kubiyabot/cli/internal/context"
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
	Debug       bool
	AutoSession bool
	UseV1API    bool // Whether to use V1 API (from context or env var)
	ContextName string // Current context name
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
	cfg := &Config{
		Debug: os.Getenv("KUBIYA_DEBUG") == "true",
	}

	// AutoSession is enabled by default but can be overridden by KUBIYA_AUTO_SESSION environment variable
	autoSession := true
	if val, exists := os.LookupEnv("KUBIYA_AUTO_SESSION"); exists {
		if parsed, err := strconv.ParseBool(val); err == nil {
			autoSession = parsed
		}
	}
	cfg.AutoSession = autoSession

	// Try to load from context first
	if ctx, name, err := context.GetCurrentContext(); err == nil {
		cfg.ContextName = name
		cfg.UseV1API = ctx.UseV1API
		cfg.BaseURL = ctx.APIURL

		// Get API key from user
		if user, err := context.GetUser(ctx.User); err == nil {
			cfg.APIKey = user.Token
		}

		// Extract org and email from JWT
		if cfg.APIKey != "" {
			_, _ = cfg.jwtDecoder()
		}

		return cfg, nil
	}

	// Fallback to environment variables if no context is configured
	apiKey := os.Getenv("KUBIYA_API_KEY")
	cfg.APIKey = apiKey

	// Check for V1 API flag
	cfg.UseV1API = context.ShouldUseV1API()

	// Determine base URL based on V1 or V2
	if cfg.UseV1API {
		baseURL := os.Getenv("KUBIYA_BASE_URL")
		if baseURL == "" {
			baseURL = "https://api.kubiya.ai/api/v1"
		}
		cfg.BaseURL = baseURL
	} else {
		// Use control plane URL
		baseURL := os.Getenv("KUBIYA_CONTROL_PLANE_BASE_URL")
		if baseURL == "" {
			baseURL = "https://control-plane.kubiya.ai"
		}
		cfg.BaseURL = baseURL
	}

	// Try to decode Org and Email from JWT if API key exists
	if cfg.APIKey != "" {
		_, _ = cfg.jwtDecoder()
	}

	return cfg, nil
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

// ValidateAPIKey checks if an API key is configured
func (c *Config) ValidateAPIKey() error {
	if c.APIKey == "" {
		return fmt.Errorf("API key is required. Set KUBIYA_API_KEY or configure a context using 'kubiya config set-context'")
	}
	return nil
}
