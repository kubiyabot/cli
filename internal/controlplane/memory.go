package controlplane

import (
	"fmt"

	"github.com/kubiyabot/cli/internal/controlplane/entities"
)

// StoreMemory stores a new memory in the context graph
func (c *Client) StoreMemory(req *entities.MemoryStoreRequest) (*entities.MemoryStoreResponse, error) {
	// Get context graph API base URL
	config, err := c.GetClientConfig()
	if err != nil {
		return nil, fmt.Errorf("failed to get client config: %w", err)
	}

	path := config.ContextGraphAPIBase + "/api/v1/graph/memory/store"

	var resp entities.MemoryStoreResponse
	if err := c.post(path, req, &resp); err != nil {
		return nil, fmt.Errorf("failed to store memory: %w", err)
	}

	return &resp, nil
}

// RecallMemory queries memories using semantic search
func (c *Client) RecallMemory(req *entities.MemoryRecallRequest) (*entities.MemoryRecallResponse, error) {
	// Get context graph API base URL
	config, err := c.GetClientConfig()
	if err != nil {
		return nil, fmt.Errorf("failed to get client config: %w", err)
	}

	// Use /graph/search endpoint (same as Composer UI)
	path := config.ContextGraphAPIBase + "/api/v1/graph/search"

	var resp entities.MemoryRecallResponse
	if err := c.post(path, req, &resp); err != nil {
		return nil, fmt.Errorf("failed to recall memory: %w", err)
	}

	return &resp, nil
}

// ListMemories lists all memories for the authenticated user
func (c *Client) ListMemories() ([]*entities.Memory, error) {
	// Get context graph API base URL
	config, err := c.GetClientConfig()
	if err != nil {
		return nil, fmt.Errorf("failed to get client config: %w", err)
	}

	path := config.ContextGraphAPIBase + "/api/v1/graph/memory/list"

	var memories []*entities.Memory
	if err := c.get(path, &memories); err != nil {
		return nil, fmt.Errorf("failed to list memories: %w", err)
	}

	return memories, nil
}

// GetMemoryJobStatus checks the status of an async memory job
func (c *Client) GetMemoryJobStatus(jobID string) (*entities.MemoryJobStatus, error) {
	// Get context graph API base URL
	config, err := c.GetClientConfig()
	if err != nil {
		return nil, fmt.Errorf("failed to get client config: %w", err)
	}

	path := fmt.Sprintf("%s/api/v1/graph/memory/status/%s", config.ContextGraphAPIBase, jobID)

	var status entities.MemoryJobStatus
	if err := c.get(path, &status); err != nil {
		return nil, fmt.Errorf("failed to get memory job status: %w", err)
	}

	return &status, nil
}

// ============================================================================
// Dataset Operations
// ============================================================================

// CreateDataset creates a new cognitive dataset
func (c *Client) CreateDataset(req *entities.DatasetCreateRequest) (*entities.Dataset, error) {
	// Get context graph API base URL
	config, err := c.GetClientConfig()
	if err != nil {
		return nil, fmt.Errorf("failed to get client config: %w", err)
	}

	path := config.ContextGraphAPIBase + "/api/v1/graph/datasets"

	var dataset entities.Dataset
	if err := c.post(path, req, &dataset); err != nil {
		return nil, fmt.Errorf("failed to create dataset: %w", err)
	}

	return &dataset, nil
}

// ListDatasets lists all datasets accessible to the authenticated user
func (c *Client) ListDatasets() ([]*entities.Dataset, error) {
	// Get context graph API base URL
	config, err := c.GetClientConfig()
	if err != nil {
		return nil, fmt.Errorf("failed to get client config: %w", err)
	}

	path := config.ContextGraphAPIBase + "/api/v1/graph/datasets"

	var response entities.DatasetListResponse
	if err := c.get(path, &response); err != nil {
		return nil, fmt.Errorf("failed to list datasets: %w", err)
	}

	return response.Datasets, nil
}

// GetDataset retrieves a specific dataset by ID
func (c *Client) GetDataset(id string) (*entities.Dataset, error) {
	// Get context graph API base URL
	config, err := c.GetClientConfig()
	if err != nil {
		return nil, fmt.Errorf("failed to get client config: %w", err)
	}

	path := fmt.Sprintf("%s/api/v1/graph/datasets/%s", config.ContextGraphAPIBase, id)

	var dataset entities.Dataset
	if err := c.get(path, &dataset); err != nil {
		return nil, fmt.Errorf("failed to get dataset: %w", err)
	}

	return &dataset, nil
}

// DeleteDataset deletes a dataset by ID
func (c *Client) DeleteDataset(id string) error {
	// Get context graph API base URL
	config, err := c.GetClientConfig()
	if err != nil {
		return fmt.Errorf("failed to get client config: %w", err)
	}

	path := fmt.Sprintf("%s/api/v1/graph/datasets/%s", config.ContextGraphAPIBase, id)

	if err := c.delete(path); err != nil {
		return fmt.Errorf("failed to delete dataset: %w", err)
	}

	return nil
}

// GetDatasetData retrieves all data entries from a dataset
func (c *Client) GetDatasetData(id string) (*entities.DatasetDataResponse, error) {
	// Get context graph API base URL
	config, err := c.GetClientConfig()
	if err != nil {
		return nil, fmt.Errorf("failed to get client config: %w", err)
	}

	path := fmt.Sprintf("%s/api/v1/graph/datasets/%s/data", config.ContextGraphAPIBase, id)

	var dataResp entities.DatasetDataResponse
	if err := c.get(path, &dataResp); err != nil {
		return nil, fmt.Errorf("failed to get dataset data: %w", err)
	}

	return &dataResp, nil
}

// PurgeDatasetData clears all data from a dataset while keeping the dataset
func (c *Client) PurgeDatasetData(id string) (*entities.PurgeResponse, error) {
	// Get context graph API base URL
	config, err := c.GetClientConfig()
	if err != nil {
		return nil, fmt.Errorf("failed to get client config: %w", err)
	}

	path := fmt.Sprintf("%s/api/v1/graph/datasets/%s/data", config.ContextGraphAPIBase, id)

	// Use DoRequest directly since DELETE returns a response body
	resp, err := c.DoRequest("DELETE", path, nil)
	if err != nil {
		return nil, fmt.Errorf("failed to purge dataset data: %w", err)
	}

	var response entities.PurgeResponse
	if err := c.ParseResponse(resp, &response); err != nil {
		return nil, fmt.Errorf("failed to parse purge response: %w", err)
	}

	return &response, nil
}
