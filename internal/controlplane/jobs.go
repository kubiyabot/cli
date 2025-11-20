package controlplane

import (
	"fmt"

	"github.com/kubiyabot/cli/internal/controlplane/entities"
)

// CreateJob creates a new scheduled job
func (c *Client) CreateJob(req *entities.CreateJobRequest) (*entities.Job, error) {
	var job entities.Job
	if err := c.post("/api/v1/jobs", req, &job); err != nil {
		return nil, err
	}
	return &job, nil
}

// ListJobs retrieves all jobs
func (c *Client) ListJobs() ([]*entities.Job, error) {
	var jobs []*entities.Job
	if err := c.get("/api/v1/jobs", &jobs); err != nil {
		return nil, err
	}
	return jobs, nil
}

// GetJob retrieves a specific job by ID
func (c *Client) GetJob(jobID string) (*entities.Job, error) {
	var job entities.Job
	if err := c.get(fmt.Sprintf("/api/v1/jobs/%s", jobID), &job); err != nil {
		return nil, err
	}
	return &job, nil
}

// UpdateJob updates an existing job
func (c *Client) UpdateJob(jobID string, req *entities.UpdateJobRequest) (*entities.Job, error) {
	var job entities.Job
	if err := c.patch(fmt.Sprintf("/api/v1/jobs/%s", jobID), req, &job); err != nil {
		return nil, err
	}
	return &job, nil
}

// DeleteJob deletes a job
func (c *Client) DeleteJob(jobID string) error {
	return c.delete(fmt.Sprintf("/api/v1/jobs/%s", jobID))
}

// TriggerJob manually triggers a job execution
func (c *Client) TriggerJob(jobID string) (*entities.TriggerJobResponse, error) {
	var response entities.TriggerJobResponse
	if err := c.post(fmt.Sprintf("/api/v1/jobs/%s/trigger", jobID), nil, &response); err != nil {
		return nil, err
	}
	return &response, nil
}

// EnableJob enables a job
func (c *Client) EnableJob(jobID string) (*entities.Job, error) {
	var job entities.Job
	if err := c.post(fmt.Sprintf("/api/v1/jobs/%s/enable", jobID), nil, &job); err != nil {
		return nil, err
	}
	return &job, nil
}

// DisableJob disables a job
func (c *Client) DisableJob(jobID string) (*entities.Job, error) {
	var job entities.Job
	if err := c.post(fmt.Sprintf("/api/v1/jobs/%s/disable", jobID), nil, &job); err != nil {
		return nil, err
	}
	return &job, nil
}

// GetJobExecutions retrieves all executions for a specific job
func (c *Client) GetJobExecutions(jobID string) (*entities.JobExecutionsResponse, error) {
	var response entities.JobExecutionsResponse
	if err := c.get(fmt.Sprintf("/api/v1/jobs/%s/executions", jobID), &response); err != nil {
		return nil, err
	}
	return &response, nil
}
