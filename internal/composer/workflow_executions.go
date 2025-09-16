package composer

import (
	"context"
	"strconv"

	"github.com/kubiyabot/cli/internal/util"
)

type WorkflowExecutionParams struct {
	Page       int
	Limit      int
	WorkflowID string
	Status     string
}

func (p *WorkflowExecutionParams) SetDefaults() {
	if p.Page <= 0 {
		p.Page = 1
	}
	if p.Limit <= 0 {
		p.Limit = 100
	}
}

func (p WorkflowExecutionParams) Map() map[string]string {
	params := make(map[string]string, 7)
	if p.Page > 0 {
		params["page"] = strconv.Itoa(p.Page)
	}
	if p.Limit > 0 {
		params["limit"] = strconv.Itoa(p.Limit)
	}
	if p.WorkflowID != "" {
		params["workflow_id"] = p.WorkflowID
	}
	if p.Status != "" {
		params["status"] = p.Status
	}
	return params
}

// WorkflowExecution represents a single execution entry in recent_executions
type WorkflowExecution struct {
	ID         string  `json:"id"`
	WorkflowID string  `json:"workflow_id"`
	Status     string  `json:"status"`
	DurationMs float64 `json:"duration_ms"`
	Trigger    string  `json:"trigger_type"`
	StartedAt  string  `json:"started_at"`
	FinishedAt string  `json:"finished_at"`
	CreatedAt  string  `json:"created_at"`
	Runner     string  `json:"runner,omitempty"`
	Workflow   *struct {
		ID   string `json:"id"`
		Name string `json:"name"`
	} `json:"workflow,omitempty"`
	Steps []WorkflowExecutionStep `json:"steps,omitempty"`
}

// WorkflowExecutions represents the response from GET /api/workflows/executions
type WorkflowExecutions struct {
	Executions []WorkflowExecution `json:"executions"`
	Total      int                 `json:"total"`
	Page       int                 `json:"page"`
	TotalPages int                 `json:"totalPages"`
	PageSize   int                 `json:"pageSize"`
}

type WorkflowExecutionStep struct {
	Name   string `json:"step_name"`
	Status string `json:"status"`
}

// CountWorkflowExecutions retrieves the total number of executions for a workflow
// by querying GET /api/workflows/executions with workflow_id filter and limit=1
func (c *Client) CountWorkflowExecutions(ctx context.Context, workflowID string) (int, error) {
	if workflowID == "" {
		return 0, nil
	}

	pathWithParams, err := c.httpClient.BuildPathWithParams("/workflows/executions", WorkflowExecutionParams{
		WorkflowID: workflowID,
		Page:       1,
		Limit:      1,
	}.Map())
	if err != nil {
		return 0, err
	}

	resp, err := c.httpClient.GET(ctx, pathWithParams)
	if err != nil {
		return 0, err
	}
	defer resp.Body.Close()

	var out WorkflowExecutions
	if err := util.DecodeJSONResponse(resp, &out); err != nil {
		return 0, err
	}
	return out.Total, nil
}

// ListWorkflowExecutions retrieves a page of workflow executions from the Composer UI API
func (c *Client) ListWorkflowExecutions(ctx context.Context, params WorkflowExecutionParams) (*WorkflowExecutions, error) {
	pathWithParams, err := c.httpClient.BuildPathWithParams("/workflows/executions", params.Map())
	if err != nil {
		return nil, err
	}
	resp, err := c.httpClient.GET(ctx, pathWithParams)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	var out WorkflowExecutions
	if err := util.DecodeJSONResponse(resp, &out); err != nil {
		return nil, err
	}
	return &out, nil
}
