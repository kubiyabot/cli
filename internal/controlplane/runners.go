package controlplane

import (
	"fmt"

	"github.com/kubiyabot/cli/internal/controlplane/entities"
)

// ListRunners lists all runners with health data
func (c *Client) ListRunners() ([]*entities.Runner, error) {
	var runners []*entities.Runner
	if err := c.get("/api/v1/runners", &runners); err != nil {
		return nil, err
	}
	return runners, nil
}

// GetRunnerHealth gets the health status for a specific runner
func (c *Client) GetRunnerHealth(runnerName string) (*entities.RunnerHealth, error) {
	var health entities.RunnerHealth
	path := fmt.Sprintf("/api/v1/runners/%s/health", runnerName)
	if err := c.get(path, &health); err != nil {
		return nil, err
	}
	return &health, nil
}
