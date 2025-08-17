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

// GetAgentByName retrieves a specific agent by name for backward compatibility
func (c *Client) GetAgentByName(ctx context.Context, name string) (*Agent, error) {
	agents, err := c.ListAgents(ctx)
	if err != nil {
		return nil, err
	}

	for _, agent := range agents {
		if agent.Name == name {
			return &agent, nil
		}
	}

	return nil, fmt.Errorf("agent not found: %s", name)
}

// Legacy ListAgents method with different endpoint
func (c *Client) ListAgentsLegacy(ctx context.Context) ([]Agent, error) {
	req, err := http.NewRequestWithContext(ctx, "GET",
		fmt.Sprintf("%s/agents", c.cfg.BaseURL), nil)
	if err != nil {
		return nil, err
	}

	if c.debug {
		fmt.Printf("ListAgents: Making request to %s\n", req.URL.String())
	}

	resp, err := c.client.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("unexpected status code: %d", resp.StatusCode)
	}

	var agents []Agent
	if err := json.NewDecoder(resp.Body).Decode(&agents); err != nil {
		return nil, fmt.Errorf("failed to decode agents: %w", err)
	}

	// Process agents to ensure UUID is properly set
	validAgents := make([]Agent, 0, len(agents))
	for _, t := range agents {
		// If UUID is empty but ID is present, use ID as UUID
		if t.UUID == "" && t.ID != "" {
			if c.debug {
				fmt.Printf("ListAgents: UUID empty for %s, using ID: %s\n", t.Name, t.ID)
			}
			t.UUID = t.ID
		}

		// Only include valid agents
		if t.UUID != "" && t.Name != "" {
			validAgents = append(validAgents, t)
		}
	}

	return validAgents, nil
}

// GetAgent retrieves a specific agent by ID
func (c *Client) GetAgent(ctx context.Context, agentID string) (*Agent, error) {
	req, err := http.NewRequestWithContext(ctx, "GET",
		fmt.Sprintf("%s/agents/%s", c.cfg.BaseURL, agentID), nil)
	if err != nil {
		return nil, err
	}

	if c.debug {
		fmt.Printf("GetAgent: Making request to %s\n", req.URL.String())
	}

	resp, err := c.client.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("unexpected status code: %d", resp.StatusCode)
	}

	var agent Agent
	if err := json.NewDecoder(resp.Body).Decode(&agent); err != nil {
		return nil, fmt.Errorf("failed to decode agent: %w", err)
	}

	// Handle ID field if UUID is empty
	if agent.UUID == "" && agent.ID != "" {
		if c.debug {
			fmt.Printf("GetAgent: UUID empty, using ID: %s\n", agent.ID)
		}
		agent.UUID = agent.ID
	}

	return &agent, nil
}
