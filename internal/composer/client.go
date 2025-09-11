package composer

import (
    "context"
    "fmt"
    "strings"
    "time"

    "github.com/kubiyabot/cli/internal/config"
    "github.com/kubiyabot/cli/internal/util"
)

// Client represents a Composer API client
type Client struct {
	cfg        *config.Config
	httpClient *util.HTTPClient
	baseURL    string
	debug      bool
}

// NewClient creates a new Composer API client
func NewClient(cfg *config.Config) *Client {
	// Use the new idiomatic functional options pattern
	httpClient := util.NewHTTPClient(
		cfg.BaseURL,
        util.WithDebug(cfg.Debug),
        // Ensure API key is attached to requests
        util.WithAPIKey(cfg.APIKey),
		util.WithTimeout(30*time.Second),
	)

	client := &Client{
		cfg:        cfg,
		httpClient: httpClient,
		baseURL:    cfg.BaseURL,
		debug:      cfg.Debug,
	}
	return client
}

// GetBaseURL returns the client's base URL
func (c *Client) GetBaseURL() string {
	return c.baseURL
}

// SetBaseURL sets the client's base URL
func (c *Client) SetBaseURL(url string) {
	c.baseURL = url
}

// ==========================
// Composer Workflows (UI API)
// ==========================

// WorkflowsResponse represents the response from GET /api/workflows
type WorkflowsResponse struct {
    Workflows   []Workflow `json:"workflows"`
    Total       int        `json:"total"`
    Page        int        `json:"page"`
    TotalPages  int        `json:"totalPages"`
    PageSize    int        `json:"pageSize"`
    Status      string     `json:"status"`
}

// Workflow represents a composer workflow entry
type Workflow struct {
    ID               string               `json:"id"`
    Name             string               `json:"name"`
    Description      string               `json:"description"`
    Status           string               `json:"status"`
    OwnerID          string               `json:"owner_id"`
    UserName         string               `json:"user_name"`
    WorkspaceID      string               `json:"workspace_id"`
    CreatedAt        string               `json:"created_at"`
    UpdatedAt        string               `json:"updated_at"`
    PublishedAt      string               `json:"published_at"`
    RecentExecutions []WorkflowExecution  `json:"recent_executions"`
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
}

// ExecutionsResponse represents the response from GET /api/workflows/executions
type ExecutionsResponse struct {
    Executions []WorkflowExecution `json:"executions"`
    Total      int                 `json:"total"`
    Page       int                 `json:"page"`
    TotalPages int                 `json:"totalPages"`
    PageSize   int                 `json:"pageSize"`
}

// ListWorkflows retrieves a page of workflows from the Composer UI API.
// See API_DOCUMENTATION.md: GET /api/workflows
func (c *Client) ListWorkflows(ctx context.Context, page, limit int, status, search, ownerID, sortBy, sortOrder string) (*WorkflowsResponse, error) {
    // Build query parameters per docs
    params := map[string]string{}
    if page > 0 {
        params["page"] = fmt.Sprintf("%d", page)
    }
    if limit > 0 {
        params["limit"] = fmt.Sprintf("%d", limit)
    }
    if status != "" {
        params["status"] = status
    }
    if search != "" {
        params["search"] = search
    }
    if ownerID != "" {
        params["owner_id"] = ownerID
    }
    if sortBy != "" {
        params["sort_by"] = sortBy
    }
    if sortOrder != "" {
        params["sort_order"] = sortOrder
    }

    // Ensure we call the UI API base (strip trailing /api/v1 or /api/v2)
    savedBase := c.httpClient.GetBaseURL()
    uiBase := savedBase
    if i := strings.Index(savedBase, "/api/"); i != -1 {
        uiBase = savedBase[:i]
    }
    c.httpClient.SetBaseURL(uiBase)
    defer c.httpClient.SetBaseURL(savedBase)

    // Build path with params and perform GET
    pathWithParams, err := c.httpClient.BuildPathWithParams("/api/workflows", params)
    if err != nil {
        return nil, err
    }

    resp, err := c.httpClient.GET(ctx, pathWithParams)
    if err != nil {
        return nil, err
    }
    defer resp.Body.Close()

    var out WorkflowsResponse
    if err := util.DecodeJSONResponse(resp, &out); err != nil {
        return nil, err
    }
    return &out, nil
}

// CountWorkflowExecutions retrieves the total number of executions for a workflow
// by querying GET /api/workflows/executions with workflow_id filter and limit=1
func (c *Client) CountWorkflowExecutions(ctx context.Context, workflowID string) (int, error) {
    if workflowID == "" {
        return 0, nil
    }

    params := map[string]string{
        "workflow_id": workflowID,
        "limit":       "1",
        "page":        "1",
    }

    savedBase := c.httpClient.GetBaseURL()
    uiBase := savedBase
    if i := strings.Index(savedBase, "/api/"); i != -1 {
        uiBase = savedBase[:i]
    }
    c.httpClient.SetBaseURL(uiBase)
    defer c.httpClient.SetBaseURL(savedBase)

    pathWithParams, err := c.httpClient.BuildPathWithParams("/api/workflows/executions", params)
    if err != nil {
        return 0, err
    }

    resp, err := c.httpClient.GET(ctx, pathWithParams)
    if err != nil {
        return 0, err
    }
    defer resp.Body.Close()

    var out ExecutionsResponse
    if err := util.DecodeJSONResponse(resp, &out); err != nil {
        return 0, err
    }
    return out.Total, nil
}

// GetWorkflow retrieves a specific workflow by ID from the Composer UI API
// Endpoint: GET /api/workflows/{id}
func (c *Client) GetWorkflow(ctx context.Context, workflowID string) (*Workflow, error) {
    if workflowID == "" {
        return nil, fmt.Errorf("workflow id is required")
    }

    savedBase := c.httpClient.GetBaseURL()
    uiBase := savedBase
    if i := strings.Index(savedBase, "/api/"); i != -1 {
        uiBase = savedBase[:i]
    }
    c.httpClient.SetBaseURL(uiBase)
    defer c.httpClient.SetBaseURL(savedBase)

    path := "/api/workflows/" + workflowID
    resp, err := c.httpClient.GET(ctx, path)
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
