package kubiya

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
)

func (c *Client) ListIntegrations(ctx context.Context) ([]Integration, error) {
	req, err := http.NewRequestWithContext(ctx, "GET",
		fmt.Sprintf("%s/integrations", c.cfg.BaseURLV2()), nil)
	if err != nil {
		return nil, err
	}

	req.Header.Set("Authorization", "UserKey "+c.cfg.APIKey)

	resp, err := c.client.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("unexpected status code: %d", resp.StatusCode)
	}

	var items []Integration
	if err = json.NewDecoder(resp.Body).Decode(&items); err != nil {
		return nil, err
	}

	return items, nil
}

func (c *Client) CreateGithubIntegration(ctx context.Context) (string, error) {
	req, err := http.NewRequestWithContext(ctx, "GET",
		fmt.Sprintf("%s/integration/github_app/install", c.cfg.BaseURL), nil)
	if err != nil {
		return "", err
	}

	req.Header.Set("Authorization", "UserKey "+c.cfg.APIKey)

	resp, err := c.client.Do(req)
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return "", fmt.Errorf("failed to create install url. unexpected status code: %d", resp.StatusCode)
	}

	var result struct {
		URL string `json:"url"`
	}

	if err = json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return "", err
	}

	if result.URL == "" {
		return "", fmt.Errorf("unexpected response: %v", result)
	}

	return result.URL, nil
}

func (c *Client) GetIntegration(ctx context.Context, name string) (*Integration, error) {
	req, err := http.NewRequestWithContext(ctx, "GET",
		fmt.Sprintf("%s/integrations/%s", c.cfg.BaseURLV2(), name), nil)
	if err != nil {
		return nil, err
	}

	req.Header.Set("Authorization", "UserKey "+c.cfg.APIKey)

	resp, err := c.client.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("failed to get integration by name: %s. unexpected status code: %d", name, resp.StatusCode)
	}

	var item Integration
	if err = json.NewDecoder(resp.Body).Decode(&item); err != nil {
		return nil, err
	}

	return &item, nil
}
