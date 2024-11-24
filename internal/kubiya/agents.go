package kubiya

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
)

// ListAgents retrieves all available agents
func (c *Client) ListAgents(ctx context.Context) ([]Agent, error) {
	req, err := http.NewRequestWithContext(ctx, "GET",
		fmt.Sprintf("%s/agents?mode=all", c.cfg.BaseURL), nil)
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
		// print more details
		fmt.Printf("path: %s, url: %s\n", req.URL.Path, req.URL.String())
		return nil, fmt.Errorf("unexpected status code: %d", resp.StatusCode)
	}

	var agents []Agent
	if err := json.NewDecoder(resp.Body).Decode(&agents); err != nil {
		return nil, err
	}

	return agents, nil
}

// ListTeammates is an alias for ListAgents for backward compatibility
func (c *Client) ListTeammates(ctx context.Context) ([]Teammate, error) {
	return c.ListAgents(ctx)
}

// GetTeammate retrieves a specific teammate by ID
func (c *Client) GetTeammate(ctx context.Context, teammateID string) (*Teammate, error) {
	req, err := http.NewRequestWithContext(ctx, "GET",
		fmt.Sprintf("%s/agents/%s", c.cfg.BaseURL, teammateID), nil)
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

	var teammate Teammate
	if err := json.NewDecoder(resp.Body).Decode(&teammate); err != nil {
		return nil, err
	}

	return &teammate, nil
}
