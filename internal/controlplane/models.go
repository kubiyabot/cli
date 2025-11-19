package controlplane

import (
	"fmt"

	"github.com/kubiyabot/cli/internal/controlplane/entities"
)

// CreateModel creates a new model
func (c *Client) CreateModel(req *entities.ModelCreateRequest) (*entities.Model, error) {
	var model entities.Model
	if err := c.post("/api/v1/models", req, &model); err != nil {
		return nil, err
	}
	return &model, nil
}

// GetModel retrieves a model by ID
func (c *Client) GetModel(id string) (*entities.Model, error) {
	var model entities.Model
	if err := c.get(fmt.Sprintf("/api/v1/models/%s", id), &model); err != nil {
		return nil, err
	}
	return &model, nil
}

// ListModels lists all models (with optional filters)
func (c *Client) ListModels() ([]*entities.Model, error) {
	var models []*entities.Model
	if err := c.get("/api/v1/models", &models); err != nil {
		return nil, err
	}
	return models, nil
}

// GetDefaultModel gets the recommended default model
func (c *Client) GetDefaultModel() (*entities.Model, error) {
	var model entities.Model
	if err := c.get("/api/v1/models/default", &model); err != nil {
		return nil, err
	}
	return &model, nil
}

// ListModelProviders lists unique model providers
func (c *Client) ListModelProviders() ([]string, error) {
	var providers []string
	if err := c.get("/api/v1/models/providers", &providers); err != nil {
		return nil, err
	}
	return providers, nil
}

// UpdateModel updates an existing model
func (c *Client) UpdateModel(id string, req *entities.ModelUpdateRequest) (*entities.Model, error) {
	var model entities.Model
	if err := c.patch(fmt.Sprintf("/api/v1/models/%s", id), req, &model); err != nil {
		return nil, err
	}
	return &model, nil
}

// DeleteModel deletes a model
func (c *Client) DeleteModel(id string) error {
	return c.delete(fmt.Sprintf("/api/v1/models/%s", id))
}
