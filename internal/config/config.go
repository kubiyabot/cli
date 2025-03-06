package config

import (
	"fmt"
	"os"
	"strconv"
	"strings"

	"github.com/golang-jwt/jwt/v4"
)

type Config struct {
	Org         string
	Email       string
	APIKey      string
	BaseURL     string
	Debug       bool
	AutoSession bool
}

func Load() (*Config, error) {
	apiKey := os.Getenv("KUBIYA_API_KEY")
	if apiKey == "" {
		fmt.Println("KUBIYA_API_KEY environment variable is required in ~/.kubiya/config.yaml or as an environment variable")
		fmt.Println("You can get your API key from https://app.kubiya.ai/api-keys")
		return nil, fmt.Errorf("KUBIYA_API_KEY environment variable is required")
	}

	baseURL := os.Getenv("KUBIYA_BASE_URL")
	if baseURL == "" {
		baseURL = "https://api.kubiya.ai/api/v1"
	}

	// AutoSession is enabled by default but can be overridden by KUBIYA_AUTO_SESSION environment variable
	autoSession := true
	if val, exists := os.LookupEnv("KUBIYA_AUTO_SESSION"); exists {
		if parsed, err := strconv.ParseBool(val); err == nil {
			autoSession = parsed
		}
	}

	return &Config{
		APIKey:      apiKey,
		BaseURL:     baseURL,
		Debug:       os.Getenv("KUBIYA_DEBUG") == "true",
		AutoSession: autoSession,
	}, nil
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
