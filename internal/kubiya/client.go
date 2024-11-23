package kubiya

import (
	"net/http"

	"github.com/kubiyabot/cli/internal/config"
)

// Client handles communication with the Kubiya API
type Client struct {
	cfg    *config.Config
	client *http.Client
}

// NewClient creates a new Kubiya API client
func NewClient(cfg *config.Config) *Client {
	return &Client{
		cfg:    cfg,
		client: &http.Client{},
	}
}
