package config

import (
	"fmt"
	"os"
)

type Config struct {
	APIKey  string
	BaseURL string
	Debug   bool
}

func Load() (*Config, error) {
	apiKey := os.Getenv("KUBIYA_API_KEY")
	if apiKey == "" {
		return nil, fmt.Errorf("KUBIYA_API_KEY environment variable is required")
	}

	baseURL := os.Getenv("KUBIYA_BASE_URL")
	if baseURL == "" {
		baseURL = "https://api.kubiya.ai/api/v1"
	}

	return &Config{
		APIKey:  apiKey,
		BaseURL: baseURL,
		Debug:   os.Getenv("KUBIYA_DEBUG") == "true",
	}, nil
}
