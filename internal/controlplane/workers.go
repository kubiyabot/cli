package controlplane

import (
	"fmt"

	"github.com/kubiyabot/cli/internal/controlplane/entities"
)

// CreateWorkerQueue creates a new worker queue
func (c *Client) CreateWorkerQueue(environmentID string, req *entities.WorkerQueueCreateRequest) (*entities.WorkerQueue, error) {
	var queue entities.WorkerQueue
	path := fmt.Sprintf("/api/v1/environments/%s/worker-queues", environmentID)
	if err := c.post(path, req, &queue); err != nil {
		return nil, err
	}
	return &queue, nil
}

// GetWorkerQueue retrieves a worker queue by ID
func (c *Client) GetWorkerQueue(id string) (*entities.WorkerQueue, error) {
	var queue entities.WorkerQueue
	path := fmt.Sprintf("/api/v1/worker-queues/%s", id)
	if err := c.get(path, &queue); err != nil {
		return nil, err
	}
	return &queue, nil
}

// ListWorkerQueues lists all worker queues
func (c *Client) ListWorkerQueues() ([]*entities.WorkerQueue, error) {
	var queues []*entities.WorkerQueue
	if err := c.get("/api/v1/worker-queues", &queues); err != nil {
		return nil, err
	}
	return queues, nil
}

// ListWorkerQueuesByEnvironment lists worker queues by environment
func (c *Client) ListWorkerQueuesByEnvironment(environmentID string) ([]*entities.WorkerQueue, error) {
	var queues []*entities.WorkerQueue
	path := fmt.Sprintf("/api/v1/environments/%s/worker-queues", environmentID)
	if err := c.get(path, &queues); err != nil {
		return nil, err
	}
	return queues, nil
}

// UpdateWorkerQueue updates an existing worker queue
func (c *Client) UpdateWorkerQueue(id string, req *entities.WorkerQueueUpdateRequest) (*entities.WorkerQueue, error) {
	var queue entities.WorkerQueue
	path := fmt.Sprintf("/api/v1/worker-queues/%s", id)
	if err := c.patch(path, req, &queue); err != nil {
		return nil, err
	}
	return &queue, nil
}

// DeleteWorkerQueue deletes a worker queue
func (c *Client) DeleteWorkerQueue(id string) error {
	path := fmt.Sprintf("/api/v1/worker-queues/%s", id)
	return c.delete(path)
}

// GetWorkerQueueInstallScript gets the install script for a worker queue
func (c *Client) GetWorkerQueueInstallScript(id string) (string, error) {
	var result map[string]string
	path := fmt.Sprintf("/api/v1/worker-queues/%s/install-script", id)
	if err := c.get(path, &result); err != nil {
		return "", err
	}
	if script, ok := result["script"]; ok {
		return script, nil
	}
	return "", fmt.Errorf("install script not found in response")
}

// GetWorkerQueueWorkerCommand gets the worker command for a queue
func (c *Client) GetWorkerQueueWorkerCommand(id string) (string, error) {
	var result map[string]string
	path := fmt.Sprintf("/api/v1/worker-queues/%s/worker-command", id)
	if err := c.get(path, &result); err != nil {
		return "", err
	}
	if command, ok := result["command"]; ok {
		return command, nil
	}
	return "", fmt.Errorf("worker command not found in response")
}

// StartWorkerQueue starts workers for a queue
func (c *Client) StartWorkerQueue(id string) error {
	path := fmt.Sprintf("/api/v1/worker-queues/%s/start", id)
	return c.post(path, nil, nil)
}

// ListQueueWorkers lists workers in a queue
func (c *Client) ListQueueWorkers(queueID string) ([]*entities.Worker, error) {
	var workers []*entities.Worker
	path := fmt.Sprintf("/api/v1/worker-queues/%s/workers", queueID)
	if err := c.get(path, &workers); err != nil {
		return nil, err
	}
	return workers, nil
}

// ============================================================================
// Worker Auto-Update Methods
// ============================================================================

// GetWorkerQueueConfig gets the worker queue configuration with version tracking
func (c *Client) GetWorkerQueueConfig(queueID string) (*entities.WorkerQueueConfig, error) {
	var config entities.WorkerQueueConfig
	path := fmt.Sprintf("/api/v1/worker-queues/%s/config", queueID)
	if err := c.get(path, &config); err != nil {
		return nil, err
	}
	return &config, nil
}

// AcquireUpdateLock acquires an update lock for coordinated rolling updates
func (c *Client) AcquireUpdateLock(queueID, workerID string, durationSeconds int) (*entities.UpdateLock, error) {
	var lock entities.UpdateLock
	req := entities.UpdateLockRequest{
		WorkerID:            workerID,
		LockDurationSeconds: durationSeconds,
	}
	path := fmt.Sprintf("/api/v1/worker-queues/%s/workers/%s/update-lock", queueID, workerID)
	if err := c.post(path, req, &lock); err != nil {
		return nil, err
	}
	return &lock, nil
}

// ReleaseUpdateLock releases an update lock after worker has completed its update
func (c *Client) ReleaseUpdateLock(queueID, workerID string) error {
	path := fmt.Sprintf("/api/v1/worker-queues/%s/workers/%s/update-lock", queueID, workerID)
	return c.delete(path)
}

// GetUpdateLockStatus gets the current update lock status for a queue
func (c *Client) GetUpdateLockStatus(queueID string) (*entities.UpdateLockStatus, error) {
	var status entities.UpdateLockStatus
	path := fmt.Sprintf("/api/v1/worker-queues/%s/update-lock-status", queueID)
	if err := c.get(path, &status); err != nil {
		return nil, err
	}
	return &status, nil
}
