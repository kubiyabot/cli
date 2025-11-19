package controlplane

import (
	"fmt"

	"github.com/kubiyabot/cli/internal/controlplane/entities"
)

// CreateAgent creates a new agent
func (c *Client) CreateAgent(req *entities.AgentCreateRequest) (*entities.Agent, error) {
	var agent entities.Agent
	if err := c.post("/api/v1/agents", req, &agent); err != nil {
		return nil, err
	}
	return &agent, nil
}

// GetAgent retrieves an agent by ID
func (c *Client) GetAgent(id string) (*entities.Agent, error) {
	var agent entities.Agent
	if err := c.get(fmt.Sprintf("/api/v1/agents/%s", id), &agent); err != nil {
		return nil, err
	}
	return &agent, nil
}

// ListAgents lists all agents
func (c *Client) ListAgents() ([]*entities.Agent, error) {
	var agents []*entities.Agent
	if err := c.get("/api/v1/agents", &agents); err != nil {
		return nil, err
	}
	return agents, nil
}

// UpdateAgent updates an existing agent
func (c *Client) UpdateAgent(id string, req *entities.AgentUpdateRequest) (*entities.Agent, error) {
	var agent entities.Agent
	if err := c.patch(fmt.Sprintf("/api/v1/agents/%s", id), req, &agent); err != nil {
		return nil, err
	}
	return &agent, nil
}

// DeleteAgent deletes an agent
func (c *Client) DeleteAgent(id string) error {
	return c.delete(fmt.Sprintf("/api/v1/agents/%s", id))
}

// ExecuteAgent executes an agent task
func (c *Client) ExecuteAgent(id string, req *entities.AgentExecuteRequest) (*entities.AgentExecuteResponse, error) {
	var response entities.AgentExecuteResponse
	if err := c.post(fmt.Sprintf("/api/v1/agents/%s/execute", id), req, &response); err != nil {
		return nil, err
	}
	return &response, nil
}
