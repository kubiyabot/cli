package composer

import (
	"context"
	"strconv"
	"time"

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
	ID          string `json:"id"`
	WorkflowID  string `json:"workflow_id"`
	Status      string `json:"status"`
	Trigger     string `json:"trigger_type"`
	CreatedAt   string `json:"created_at"`
	StartedAt   string `json:"started_at"`
	CompletedAt string `json:"completed_at"`
	Runner      string `json:"runner,omitempty"`
	Workflow    *struct {
		ID   string `json:"id"`
		Name string `json:"name"`
	} `json:"workflow,omitempty"`
	Steps []WorkflowExecutionStep `json:"steps,omitempty"`
}

func (we *WorkflowExecution) Duration() string {
	start, err := time.Parse(time.RFC3339, we.StartedAt)
	if err != nil {
		return "N/A"
	}
	end, err := time.Parse(time.RFC3339, we.CompletedAt)
	if err != nil {
		return "N/A"
	}
	return end.Sub(start).Truncate(time.Millisecond).String()
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
