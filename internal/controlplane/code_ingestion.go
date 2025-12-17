package controlplane

import (
	"fmt"

	"github.com/kubiyabot/cli/internal/controlplane/entities"
)

// ============================================================================
// Code Ingestion Operations
// ============================================================================

// StartCodeSession starts a streaming code ingestion session
func (c *Client) StartCodeSession(datasetID string, req *entities.CodeStreamSessionCreate) (*entities.CodeStreamSession, error) {
	// Get context graph API base URL
	config, err := c.GetClientConfig()
	if err != nil {
		return nil, fmt.Errorf("failed to get client config: %w", err)
	}

	path := fmt.Sprintf("%s/api/v1/graph/datasets/%s/code/stream/start", config.ContextGraphAPIBase, datasetID)

	var session entities.CodeStreamSession
	if err := c.post(path, req, &session); err != nil {
		return nil, fmt.Errorf("failed to start code session: %w", err)
	}

	return &session, nil
}

// UploadCodeBatch uploads a batch of code files to an active session
func (c *Client) UploadCodeBatch(datasetID string, batch *entities.CodeStreamBatchRequest) (*entities.CodeBatchResponse, error) {
	// Get context graph API base URL
	config, err := c.GetClientConfig()
	if err != nil {
		return nil, fmt.Errorf("failed to get client config: %w", err)
	}

	path := fmt.Sprintf("%s/api/v1/graph/datasets/%s/code/stream/batch", config.ContextGraphAPIBase, datasetID)

	var resp entities.CodeBatchResponse
	if err := c.post(path, batch, &resp); err != nil {
		return nil, fmt.Errorf("failed to upload code batch: %w", err)
	}

	return &resp, nil
}

// CommitCodeSession commits a streaming session and triggers cognify
func (c *Client) CommitCodeSession(datasetID, sessionID string) (*entities.CodeCommitResponse, error) {
	// Get context graph API base URL
	config, err := c.GetClientConfig()
	if err != nil {
		return nil, fmt.Errorf("failed to get client config: %w", err)
	}

	path := fmt.Sprintf("%s/api/v1/graph/datasets/%s/code/stream/%s/commit", config.ContextGraphAPIBase, datasetID, sessionID)

	var resp entities.CodeCommitResponse
	if err := c.post(path, nil, &resp); err != nil {
		return nil, fmt.Errorf("failed to commit code session: %w", err)
	}

	return &resp, nil
}

// GetCodeJobStatus retrieves the status of a code ingestion job
func (c *Client) GetCodeJobStatus(datasetID, jobID string) (*entities.CodeJobStatus, error) {
	// Get context graph API base URL
	config, err := c.GetClientConfig()
	if err != nil {
		return nil, fmt.Errorf("failed to get client config: %w", err)
	}

	path := fmt.Sprintf("%s/api/v1/graph/datasets/%s/code/jobs/%s", config.ContextGraphAPIBase, datasetID, jobID)

	var status entities.CodeJobStatus
	if err := c.get(path, &status); err != nil {
		return nil, fmt.Errorf("failed to get code job status: %w", err)
	}

	return &status, nil
}
