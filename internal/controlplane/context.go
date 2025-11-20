package controlplane

import (
	"fmt"

	"github.com/kubiyabot/cli/internal/controlplane/entities"
)

// GetContext gets the context for an entity
func (c *Client) GetContext(entityType, entityID string) (*entities.Context, error) {
	var context entities.Context
	path := fmt.Sprintf("/api/v1/context/%s/%s", entityType, entityID)
	if err := c.get(path, &context); err != nil {
		return nil, err
	}
	return &context, nil
}

// SetContext sets the context for an entity
func (c *Client) SetContext(entityType, entityID string, req *entities.ContextRequest) (*entities.Context, error) {
	var context entities.Context
	path := fmt.Sprintf("/api/v1/context/%s/%s", entityType, entityID)
	if err := c.put(path, req, &context); err != nil {
		return nil, err
	}
	return &context, nil
}

// ClearContext clears the context for an entity
func (c *Client) ClearContext(entityType, entityID string) error {
	path := fmt.Sprintf("/api/v1/context/%s/%s", entityType, entityID)
	return c.delete(path)
}

// GetResolvedContext gets the resolved context for an entity (with inheritance)
func (c *Client) GetResolvedContext(entityType, entityID string) (*entities.ResolvedContext, error) {
	var context entities.ResolvedContext
	path := fmt.Sprintf("/api/v1/context/resolve/%s/%s", entityType, entityID)
	if err := c.get(path, &context); err != nil {
		return nil, err
	}
	return &context, nil
}

// GetTeamContext gets the context configuration for a team
func (c *Client) GetTeamContext(teamID string) (*entities.Context, error) {
	var context entities.Context
	path := fmt.Sprintf("/api/v1/teams/%s/context", teamID)
	if err := c.get(path, &context); err != nil {
		return nil, err
	}
	return &context, nil
}

// SetTeamContext sets the context configuration for a team
func (c *Client) SetTeamContext(teamID string, req *entities.ContextRequest) (*entities.Context, error) {
	var context entities.Context
	path := fmt.Sprintf("/api/v1/teams/%s/context", teamID)
	if err := c.put(path, req, &context); err != nil {
		return nil, err
	}
	return &context, nil
}

// ClearTeamContext clears the context configuration for a team
func (c *Client) ClearTeamContext(teamID string) error {
	path := fmt.Sprintf("/api/v1/teams/%s/context", teamID)
	return c.delete(path)
}

// GetResolvedAgentContext gets the resolved context for an agent
func (c *Client) GetResolvedAgentContext(agentID string) (*entities.ResolvedContext, error) {
	var context entities.ResolvedContext
	path := fmt.Sprintf("/api/v1/agents/%s/context/resolved", agentID)
	if err := c.get(path, &context); err != nil {
		return nil, err
	}
	return &context, nil
}

// GetResolvedTeamContext gets the resolved context for a team
func (c *Client) GetResolvedTeamContext(teamID string) (*entities.ResolvedContext, error) {
	var context entities.ResolvedContext
	path := fmt.Sprintf("/api/v1/teams/%s/context/resolved", teamID)
	if err := c.get(path, &context); err != nil {
		return nil, err
	}
	return &context, nil
}
