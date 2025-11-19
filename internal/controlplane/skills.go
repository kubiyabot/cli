package controlplane

import (
	"fmt"

	"github.com/kubiyabot/cli/internal/controlplane/entities"
)

// CreateSkill creates a new skill
func (c *Client) CreateSkill(req *entities.SkillCreateRequest) (*entities.Skill, error) {
	var skill entities.Skill
	if err := c.post("/api/v1/skills", req, &skill); err != nil {
		return nil, err
	}
	return &skill, nil
}

// GetSkill retrieves a skill by ID
func (c *Client) GetSkill(id string) (*entities.Skill, error) {
	var skill entities.Skill
	if err := c.get(fmt.Sprintf("/api/v1/skills/%s", id), &skill); err != nil {
		return nil, err
	}
	return &skill, nil
}

// ListSkills lists all skills
func (c *Client) ListSkills() ([]*entities.Skill, error) {
	var skills []*entities.Skill
	if err := c.get("/api/v1/skills", &skills); err != nil {
		return nil, err
	}
	return skills, nil
}

// UpdateSkill updates an existing skill
func (c *Client) UpdateSkill(id string, req *entities.SkillUpdateRequest) (*entities.Skill, error) {
	var skill entities.Skill
	if err := c.patch(fmt.Sprintf("/api/v1/skills/%s", id), req, &skill); err != nil {
		return nil, err
	}
	return &skill, nil
}

// DeleteSkill deletes a skill
func (c *Client) DeleteSkill(id string) error {
	return c.delete(fmt.Sprintf("/api/v1/skills/%s", id))
}

// GetSkillDefinitions lists all skill type definitions
func (c *Client) GetSkillDefinitions() ([]*entities.SkillDefinition, error) {
	var definitions []*entities.SkillDefinition
	if err := c.get("/api/v1/skills/definitions", &definitions); err != nil {
		return nil, err
	}
	return definitions, nil
}

// GetSkillDefinition retrieves a specific skill type definition
func (c *Client) GetSkillDefinition(skillType string) (*entities.SkillDefinition, error) {
	var definition entities.SkillDefinition
	if err := c.get(fmt.Sprintf("/api/v1/skills/definitions/%s", skillType), &definition); err != nil {
		return nil, err
	}
	return &definition, nil
}

// AssociateSkill associates a skill with an entity
func (c *Client) AssociateSkill(entityType, entityID, skillID string) (*entities.SkillAssociation, error) {
	req := &entities.SkillAssociationRequest{
		EntityType: entityType,
		EntityID:   entityID,
		SkillID:    skillID,
	}
	var association entities.SkillAssociation
	path := fmt.Sprintf("/api/v1/skills/associations/%s/%s/skills", entityType, entityID)
	if err := c.post(path, req, &association); err != nil {
		return nil, err
	}
	return &association, nil
}

// ListSkillAssociations lists skills associated with an entity
func (c *Client) ListSkillAssociations(entityType, entityID string) ([]*entities.Skill, error) {
	var skills []*entities.Skill
	path := fmt.Sprintf("/api/v1/skills/associations/%s/%s/skills", entityType, entityID)
	if err := c.get(path, &skills); err != nil {
		return nil, err
	}
	return skills, nil
}

// RemoveSkillAssociation removes a skill association from an entity
func (c *Client) RemoveSkillAssociation(entityType, entityID, skillID string) error {
	path := fmt.Sprintf("/api/v1/skills/associations/%s/%s/skills/%s", entityType, entityID, skillID)
	return c.delete(path)
}

// GetResolvedAgentSkills gets resolved skills for an agent (with inheritance)
func (c *Client) GetResolvedAgentSkills(agentID string) ([]*entities.Skill, error) {
	var skills []*entities.Skill
	path := fmt.Sprintf("/api/v1/skills/associations/agents/%s/skills/resolved", agentID)
	if err := c.get(path, &skills); err != nil {
		return nil, err
	}
	return skills, nil
}

// GetResolvedTeamSkills gets resolved skills for a team (with inheritance)
func (c *Client) GetResolvedTeamSkills(teamID string) ([]*entities.Skill, error) {
	var skills []*entities.Skill
	path := fmt.Sprintf("/api/v1/skills/associations/teams/%s/skills/resolved", teamID)
	if err := c.get(path, &skills); err != nil {
		return nil, err
	}
	return skills, nil
}
