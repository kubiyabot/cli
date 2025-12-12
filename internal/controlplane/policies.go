package controlplane

import (
	"fmt"

	"github.com/kubiyabot/cli/internal/controlplane/entities"
)

// CreatePolicy creates a new policy
func (c *Client) CreatePolicy(req *entities.PolicyCreateRequest) (*entities.Policy, error) {
	var policy entities.Policy
	if err := c.post("/api/v1/policies", req, &policy); err != nil {
		return nil, err
	}
	return &policy, nil
}

// GetPolicy retrieves a policy by ID
func (c *Client) GetPolicy(id string) (*entities.Policy, error) {
	var policy entities.Policy
	if err := c.get(fmt.Sprintf("/api/v1/policies/%s", id), &policy); err != nil {
		return nil, err
	}
	return &policy, nil
}

// PolicyListResponse represents the paginated response from list policies API
type PolicyListResponse struct {
	Policies []*entities.Policy `json:"policies"`
	Total    int                `json:"total"`
	Page     int                `json:"page"`
	Limit    int                `json:"limit"`
	HasMore  bool               `json:"has_more"`
}

// ListPolicies lists all policies
func (c *Client) ListPolicies() ([]*entities.Policy, error) {
	var response PolicyListResponse
	if err := c.get("/api/v1/policies", &response); err != nil {
		return nil, err
	}
	return response.Policies, nil
}

// UpdatePolicy updates an existing policy
func (c *Client) UpdatePolicy(id string, req *entities.PolicyUpdateRequest) (*entities.Policy, error) {
	var policy entities.Policy
	if err := c.put(fmt.Sprintf("/api/v1/policies/%s", id), req, &policy); err != nil {
		return nil, err
	}
	return &policy, nil
}

// DeletePolicy deletes a policy
func (c *Client) DeletePolicy(id string) error {
	return c.delete(fmt.Sprintf("/api/v1/policies/%s", id))
}

// ValidatePolicy validates a policy
func (c *Client) ValidatePolicy(id string) error {
	path := fmt.Sprintf("/api/v1/policies/%s/validate", id)
	return c.post(path, nil, nil)
}

// CreatePolicyAssociation creates a policy association
func (c *Client) CreatePolicyAssociation(req *entities.PolicyAssociationRequest) (*entities.PolicyAssociation, error) {
	var association entities.PolicyAssociation
	if err := c.post("/api/v1/policies/associations", req, &association); err != nil {
		return nil, err
	}
	return &association, nil
}

// GetPolicyAssociation retrieves a policy association
func (c *Client) GetPolicyAssociation(id string) (*entities.PolicyAssociation, error) {
	var association entities.PolicyAssociation
	path := fmt.Sprintf("/api/v1/policies/associations/%s", id)
	if err := c.get(path, &association); err != nil {
		return nil, err
	}
	return &association, nil
}

// ListPolicyAssociations lists all policy associations
func (c *Client) ListPolicyAssociations() ([]*entities.PolicyAssociation, error) {
	var associations []*entities.PolicyAssociation
	if err := c.get("/api/v1/policies/associations", &associations); err != nil {
		return nil, err
	}
	return associations, nil
}

// UpdatePolicyAssociation updates a policy association
func (c *Client) UpdatePolicyAssociation(id string, req *entities.PolicyAssociationUpdateRequest) (*entities.PolicyAssociation, error) {
	var association entities.PolicyAssociation
	path := fmt.Sprintf("/api/v1/policies/associations/%s", id)
	if err := c.patch(path, req, &association); err != nil {
		return nil, err
	}
	return &association, nil
}

// DeletePolicyAssociation deletes a policy association
func (c *Client) DeletePolicyAssociation(id string) error {
	path := fmt.Sprintf("/api/v1/policies/associations/%s", id)
	return c.delete(path)
}

// GetResolvedPolicies gets resolved policies for an entity (with inheritance)
func (c *Client) GetResolvedPolicies(entityType, entityID string) ([]*entities.Policy, error) {
	var policies []*entities.Policy
	path := fmt.Sprintf("/api/v1/policies/resolved/%s/%s", entityType, entityID)
	if err := c.get(path, &policies); err != nil {
		return nil, err
	}
	return policies, nil
}

// EvaluatePolicies evaluates policies for an entity
func (c *Client) EvaluatePolicies(entityType, entityID string, req *entities.PolicyEvaluationRequest) (*entities.PolicyEvaluationResponse, error) {
	var response entities.PolicyEvaluationResponse
	path := fmt.Sprintf("/api/v1/policies/evaluate/%s/%s", entityType, entityID)
	if err := c.post(path, req, &response); err != nil {
		return nil, err
	}
	return &response, nil
}
