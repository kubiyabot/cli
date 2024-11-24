package kubiya

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"time"

	"github.com/kubiyabot/cli/internal/config"
)

// Client represents a Kubiya API client
type Client struct {
	cfg     *config.Config
	client  *http.Client
	baseURL string
	debug   bool
}

// NewClient creates a new Kubiya API client
func NewClient(cfg *config.Config) *Client {
	return &Client{
		cfg:     cfg,
		client:  &http.Client{Timeout: 30 * time.Second},
		baseURL: cfg.BaseURL,
		debug:   cfg.Debug,
	}
}

// do performs an HTTP request and decodes the response into v
func (c *Client) do(req *http.Request, v interface{}) error {
	req.Header.Set("Authorization", "UserKey "+c.cfg.APIKey)
	req.Header.Set("Content-Type", "application/json")

	resp, err := c.client.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("unexpected status code: %d", resp.StatusCode)
	}

	if v != nil {
		if err := json.NewDecoder(resp.Body).Decode(v); err != nil {
			return err
		}
	}

	return nil
}

// newJSONRequest creates a new HTTP request with JSON payload
func (c *Client) newJSONRequest(ctx context.Context, method, url string, payload interface{}) (*http.Request, error) {
	var body bytes.Buffer
	if payload != nil {
		if err := json.NewEncoder(&body).Encode(payload); err != nil {
			return nil, fmt.Errorf("failed to encode request payload: %w", err)
		}
	}

	req, err := http.NewRequestWithContext(ctx, method, url, &body)
	if err != nil {
		return nil, err
	}

	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", "UserKey "+c.cfg.APIKey)

	return req, nil
}
