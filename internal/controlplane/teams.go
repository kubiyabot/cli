package controlplane

import (
	"fmt"

	"github.com/kubiyabot/cli/internal/controlplane/entities"
)

// CreateTeam creates a new team
func (c *Client) CreateTeam(req *entities.TeamCreateRequest) (*entities.Team, error) {
	var team entities.Team
	if err := c.post("/api/v1/teams", req, &team); err != nil {
		return nil, err
	}
	return &team, nil
}

// GetTeam retrieves a team by ID
func (c *Client) GetTeam(id string) (*entities.Team, error) {
	var team entities.Team
	if err := c.get(fmt.Sprintf("/api/v1/teams/%s", id), &team); err != nil {
		return nil, err
	}
	return &team, nil
}

// ListTeams lists all teams
func (c *Client) ListTeams() ([]*entities.Team, error) {
	var teams []*entities.Team
	if err := c.get("/api/v1/teams", &teams); err != nil {
		return nil, err
	}
	return teams, nil
}

// UpdateTeam updates an existing team
func (c *Client) UpdateTeam(id string, req *entities.TeamUpdateRequest) (*entities.Team, error) {
	var team entities.Team
	if err := c.patch(fmt.Sprintf("/api/v1/teams/%s", id), req, &team); err != nil {
		return nil, err
	}
	return &team, nil
}

// DeleteTeam deletes a team
func (c *Client) DeleteTeam(id string) error {
	return c.delete(fmt.Sprintf("/api/v1/teams/%s", id))
}
