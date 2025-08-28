package composer

import (
	"time"

	"github.com/kubiyabot/cli/internal/config"
	"github.com/kubiyabot/cli/internal/util"
)

// Client represents a Composer API client
type Client struct {
	cfg        *config.Config
	httpClient *util.HTTPClient
	baseURL    string
	debug      bool
}

// NewClient creates a new Composer API client
func NewClient(cfg *config.Config) *Client {
	// Use the new idiomatic functional options pattern
	httpClient := util.NewHTTPClient(
		cfg.BaseURL,
		util.WithDebug(cfg.Debug),
		util.WithTimeout(30*time.Second),
	)

	client := &Client{
		cfg:        cfg,
		httpClient: httpClient,
		baseURL:    cfg.BaseURL,
		debug:      cfg.Debug,
	}
	return client
}

// GetBaseURL returns the client's base URL
func (c *Client) GetBaseURL() string {
	return c.baseURL
}

// SetBaseURL sets the client's base URL
func (c *Client) SetBaseURL(url string) {
	c.baseURL = url
}
