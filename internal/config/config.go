package config

import (
	"fmt"
	"os"
	"strconv"
)

type Config struct {
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
