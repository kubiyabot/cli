package kubiya

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
)

// ListAgents retrieves all available agents
func (c *Client) ListAgents(ctx context.Context) ([]Teammate, error) {
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

	// Convert agents to teammates
	teammates := make([]Teammate, 0, len(agents))
	for _, agent := range agents {
		teammates = append(teammates, Teammate{
			UUID:        agent.UUID,
			Name:        agent.Name,
			Description: agent.Desc,
		})
	}
	return teammates, nil
}

// ListTeammates is an alias for ListAgents for backward compatibility
func (c *Client) ListTeammates(ctx context.Context) ([]Teammate, error) {
	req, err := http.NewRequestWithContext(ctx, "GET",
		fmt.Sprintf("%s/agents", c.cfg.BaseURL), nil)
	if err != nil {
		return nil, err
	}

	req.Header.Set("Authorization", "UserKey "+c.cfg.APIKey)

	if c.debug {
		fmt.Printf("ListTeammates: Making request to %s\n", req.URL.String())
	}

	resp, err := c.client.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("unexpected status code: %d", resp.StatusCode)
	}

	var teammates []Teammate
	if err := json.NewDecoder(resp.Body).Decode(&teammates); err != nil {
		return nil, fmt.Errorf("failed to decode teammates: %w", err)
	}

	// Process teammates to ensure UUID is properly set
	validTeammates := make([]Teammate, 0, len(teammates))
	for _, t := range teammates {
		// If UUID is empty but ID is present, use ID as UUID
		if t.UUID == "" && t.ID != "" {
			if c.debug {
				fmt.Printf("ListTeammates: UUID empty for %s, using ID: %s\n", t.Name, t.ID)
			}
			t.UUID = t.ID
		}

		// Only include valid teammates
		if t.UUID != "" && t.Name != "" {
			validTeammates = append(validTeammates, t)
		}
	}

	return validTeammates, nil
}

// GetTeammate retrieves a specific teammate by ID
func (c *Client) GetTeammate(ctx context.Context, teammateID string) (*Teammate, error) {
	req, err := http.NewRequestWithContext(ctx, "GET",
		fmt.Sprintf("%s/agents/%s", c.cfg.BaseURL, teammateID), nil)
	if err != nil {
		return nil, err
	}

	req.Header.Set("Authorization", "UserKey "+c.cfg.APIKey)

	if c.debug {
		fmt.Printf("GetTeammate: Making request to %s\n", req.URL.String())
	}

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
		return nil, fmt.Errorf("failed to decode teammate: %w", err)
	}

	// Handle ID field if UUID is empty
	if teammate.UUID == "" && teammate.ID != "" {
		if c.debug {
			fmt.Printf("GetTeammate: UUID empty, using ID: %s\n", teammate.ID)
		}
		teammate.UUID = teammate.ID
	}

	return &teammate, nil
}
