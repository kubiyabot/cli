package composer

import (
	"context"
	"fmt"
	"strconv"

	"github.com/kubiyabot/cli/internal/util"
)

type WorkflowParams struct {
	Page      int
	Limit     int
	Status    string
	Search    string
	OwnerID   string
	SortBy    string
	SortOrder string
}

func (p *WorkflowParams) SetDefaults() {
	if p.Page <= 0 {
		p.Page = 1
	}
	if p.Limit <= 0 {
		p.Limit = 12
	}
	if p.SortBy == "" {
		p.SortBy = "updated_at"
	}
	if p.SortOrder == "" {
		p.SortOrder = "desc"
	}
}

func (p WorkflowParams) Map() map[string]string {
	params := make(map[string]string, 7)
	if p.Page > 0 {
		params["page"] = strconv.Itoa(p.Page)
	}
	if p.Limit > 0 {
		params["limit"] = strconv.Itoa(p.Limit)
	}
	if p.Status != "" {
		params["status"] = p.Status
	}
	if p.Search != "" {
		params["search"] = p.Search
	}
	if p.OwnerID != "" {
		params["owner_id"] = p.OwnerID
	}
	if p.SortBy != "" {
		params["sort_by"] = p.SortBy
	}
	if p.SortOrder != "" {
		params["sort_order"] = p.SortOrder
	}
	return params
}

// Workflows represents the response from GET /api/workflows
type Workflows struct {
	Workflows  []Workflow `json:"workflows"`
	Total      int        `json:"total"`
	Page       int        `json:"page"`
	TotalPages int        `json:"totalPages"`
	PageSize   int        `json:"pageSize"`
	Status     string     `json:"status"`
}

// Workflow represents a composer workflow entry
type Workflow struct {
	ID               string              `json:"id"`
	Name             string              `json:"name"`
	Description      string              `json:"description"`
	Status           string              `json:"status"`
	OwnerID          string              `json:"owner_id"`
	UserName         string              `json:"user_name"`
	WorkspaceID      string              `json:"workspace_id"`
	CreatedAt        string              `json:"created_at"`
	UpdatedAt        string              `json:"updated_at"`
	PublishedAt      string              `json:"published_at"`
	RecentExecutions []WorkflowExecution `json:"recent_executions"`
}

func (wf Workflow) LastExecution() string {
	var lastExec string
	if len(wf.RecentExecutions) > 0 {
		if wf.RecentExecutions[0].CompletedAt != "" {
			lastExec = wf.RecentExecutions[0].CreatedAt
		} else {
			lastExec = wf.RecentExecutions[0].StartedAt
		}
	}
	return lastExec
}

// ListWorkflows retrieves a page of workflows from the Composer UI API.
// See API_DOCUMENTATION.md: GET /api/workflows
func (c *Client) ListWorkflows(ctx context.Context, params WorkflowParams) (*Workflows, error) {
	pathWithParams, err := c.httpClient.BuildPathWithParams("/workflows", params.Map())
	if err != nil {
		return nil, err
	}

	resp, err := c.httpClient.GET(ctx, pathWithParams)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	var out Workflows
	if err := util.DecodeJSONResponse(resp, &out); err != nil {
		return nil, err
	}
	return &out, nil
}

// GetWorkflow retrieves a specific workflow by ID from the Composer UI API
// Endpoint: GET /api/workflows/{id}
func (c *Client) GetWorkflow(ctx context.Context, workflowID string) (*Workflow, error) {
	if workflowID == "" {
		return nil, fmt.Errorf("workflow id is required")
	}

	resp, err := c.httpClient.GET(ctx, "/workflows/"+workflowID)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	var out Workflow
	if err := util.DecodeJSONResponse(resp, &out); err != nil {
		return nil, err
	}
	return &out, nil
}
