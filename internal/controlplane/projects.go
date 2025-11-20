package controlplane

import (
	"fmt"

	"github.com/kubiyabot/cli/internal/controlplane/entities"
)

// CreateProject creates a new project
func (c *Client) CreateProject(req *entities.ProjectCreateRequest) (*entities.Project, error) {
	var project entities.Project
	if err := c.post("/api/v1/projects", req, &project); err != nil {
		return nil, err
	}
	return &project, nil
}

// GetProject retrieves a project by ID
func (c *Client) GetProject(id string) (*entities.Project, error) {
	var project entities.Project
	if err := c.get(fmt.Sprintf("/api/v1/projects/%s", id), &project); err != nil {
		return nil, err
	}
	return &project, nil
}

// ListProjects lists all projects
func (c *Client) ListProjects() ([]*entities.Project, error) {
	var projects []*entities.Project
	if err := c.get("/api/v1/projects", &projects); err != nil {
		return nil, err
	}
	return projects, nil
}

// GetDefaultProject gets or creates the default project
func (c *Client) GetDefaultProject() (*entities.Project, error) {
	var project entities.Project
	if err := c.get("/api/v1/projects/default", &project); err != nil {
		return nil, err
	}
	return &project, nil
}

// UpdateProject updates an existing project
func (c *Client) UpdateProject(id string, req *entities.ProjectUpdateRequest) (*entities.Project, error) {
	var project entities.Project
	if err := c.patch(fmt.Sprintf("/api/v1/projects/%s", id), req, &project); err != nil {
		return nil, err
	}
	return &project, nil
}

// DeleteProject deletes a project
func (c *Client) DeleteProject(id string) error {
	return c.delete(fmt.Sprintf("/api/v1/projects/%s", id))
}

// AddAgentToProject adds an agent to a project
func (c *Client) AddAgentToProject(projectID, agentID string) error {
	path := fmt.Sprintf("/api/v1/projects/%s/agents", projectID)
	body := map[string]string{"agent_id": agentID}
	return c.post(path, body, nil)
}

// RemoveAgentFromProject removes an agent from a project
func (c *Client) RemoveAgentFromProject(projectID, agentID string) error {
	path := fmt.Sprintf("/api/v1/projects/%s/agents/%s", projectID, agentID)
	return c.delete(path)
}

// ListProjectAgents lists agents in a project
func (c *Client) ListProjectAgents(projectID string) ([]*entities.Agent, error) {
	var agents []*entities.Agent
	path := fmt.Sprintf("/api/v1/projects/%s/agents", projectID)
	if err := c.get(path, &agents); err != nil {
		return nil, err
	}
	return agents, nil
}

// AddTeamToProject adds a team to a project
func (c *Client) AddTeamToProject(projectID, teamID string) error {
	path := fmt.Sprintf("/api/v1/projects/%s/teams", projectID)
	body := map[string]string{"team_id": teamID}
	return c.post(path, body, nil)
}

// RemoveTeamFromProject removes a team from a project
func (c *Client) RemoveTeamFromProject(projectID, teamID string) error {
	path := fmt.Sprintf("/api/v1/projects/%s/teams/%s", projectID, teamID)
	return c.delete(path)
}

// ListProjectTeams lists teams in a project
func (c *Client) ListProjectTeams(projectID string) ([]*entities.Team, error) {
	var teams []*entities.Team
	path := fmt.Sprintf("/api/v1/projects/%s/teams", projectID)
	if err := c.get(path, &teams); err != nil {
		return nil, err
	}
	return teams, nil
}
