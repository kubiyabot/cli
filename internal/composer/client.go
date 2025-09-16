package composer

import (
	"os"
	"strings"
	"time"

	"github.com/kubiyabot/cli/internal/config"
	"github.com/kubiyabot/cli/internal/util"
)

// Client represents a Composer API client
type Client struct {
	httpClient *util.HTTPClient
}

// NewClient creates a new Composer API client
func NewClient(cfg *config.Config) *Client {
	// Composer API always uses the compose.kubiya.ai domain
	composerBaseURL := getComposerBaseURL(cfg.BaseURL)

	return &Client{
		httpClient: util.NewHTTPClient(
			composerBaseURL,
			util.WithDebug(cfg.Debug),
			util.WithAPIKey(cfg.APIKey),
			util.WithTimeout(30*time.Second),
		),
	}
}

// getComposerBaseURL determines the correct composer API base URL
func getComposerBaseURL(configBaseURL string) string {
	// Check if a specific composer URL is set via environment variable
	if composerURL := os.Getenv("KUBIYA_COMPOSER_URL"); composerURL != "" {
		return composerURL
	}

	// If the config base URL is already pointing to composer, use it as-is
	if strings.Contains(configBaseURL, "compose.kubiya.ai") {
		return configBaseURL
	}

	// For testing: preserve localhost/127.0.0.1 URLs
	if strings.Contains(configBaseURL, "localhost") || strings.Contains(configBaseURL, "127.0.0.1") {
		return configBaseURL
	}

	// Default composer API URL
	return "https://compose.kubiya.ai/api"
}
