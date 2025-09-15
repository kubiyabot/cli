package composer

import (
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
	return &Client{
		httpClient: util.NewHTTPClient(
			cfg.BaseURL,
			util.WithDebug(cfg.Debug),
			util.WithAPIKey(cfg.APIKey),
			util.WithTimeout(30*time.Second),
		),
	}
}
