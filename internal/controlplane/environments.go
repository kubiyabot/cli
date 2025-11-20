package controlplane

import (
	"fmt"

	"github.com/kubiyabot/cli/internal/controlplane/entities"
)

// CreateEnvironment creates a new environment
func (c *Client) CreateEnvironment(req *entities.EnvironmentCreateRequest) (*entities.Environment, error) {
	var environment entities.Environment
	if err := c.post("/api/v1/environments", req, &environment); err != nil {
		return nil, err
	}
	return &environment, nil
}

// GetEnvironment retrieves an environment by ID
func (c *Client) GetEnvironment(id string) (*entities.Environment, error) {
	var environment entities.Environment
	if err := c.get(fmt.Sprintf("/api/v1/environments/%s", id), &environment); err != nil {
		return nil, err
	}
	return &environment, nil
}

// ListEnvironments lists all environments
func (c *Client) ListEnvironments() ([]*entities.Environment, error) {
	var environments []*entities.Environment
	if err := c.get("/api/v1/environments", &environments); err != nil {
		return nil, err
	}
	return environments, nil
}

// UpdateEnvironment updates an existing environment
func (c *Client) UpdateEnvironment(id string, req *entities.EnvironmentUpdateRequest) (*entities.Environment, error) {
	var environment entities.Environment
	if err := c.patch(fmt.Sprintf("/api/v1/environments/%s", id), req, &environment); err != nil {
		return nil, err
	}
	return &environment, nil
}

// DeleteEnvironment deletes an environment
func (c *Client) DeleteEnvironment(id string) error {
	return c.delete(fmt.Sprintf("/api/v1/environments/%s", id))
}

// GetEnvironmentWorkerCommand gets the worker registration command for an environment
func (c *Client) GetEnvironmentWorkerCommand(id string) (string, error) {
	var result map[string]string
	path := fmt.Sprintf("/api/v1/environments/%s/worker-command", id)
	if err := c.get(path, &result); err != nil {
		return "", err
	}
	if command, ok := result["command"]; ok {
		return command, nil
	}
	return "", fmt.Errorf("worker command not found in response")
}

// GetEnvironmentContext gets the context configuration for an environment
func (c *Client) GetEnvironmentContext(id string) (*entities.Context, error) {
	var context entities.Context
	path := fmt.Sprintf("/api/v1/environments/%s/context", id)
	if err := c.get(path, &context); err != nil {
		return nil, err
	}
	return &context, nil
}

// SetEnvironmentContext sets the context configuration for an environment
func (c *Client) SetEnvironmentContext(id string, req *entities.ContextRequest) (*entities.Context, error) {
	var context entities.Context
	path := fmt.Sprintf("/api/v1/environments/%s/context", id)
	if err := c.put(path, req, &context); err != nil {
		return nil, err
	}
	return &context, nil
}

// ClearEnvironmentContext clears the context configuration for an environment
func (c *Client) ClearEnvironmentContext(id string) error {
	path := fmt.Sprintf("/api/v1/environments/%s/context", id)
	return c.delete(path)
}
