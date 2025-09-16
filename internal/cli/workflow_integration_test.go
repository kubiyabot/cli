package cli

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/kubiyabot/cli/internal/composer"
	"github.com/kubiyabot/cli/internal/config"
)

// MockComposerServer creates a test HTTP server that mocks the composer API
func MockComposerServer() *httptest.Server {
	mux := http.NewServeMux()

	// Mock GET /workflows - list workflows
	mux.HandleFunc("/workflows", func(w http.ResponseWriter, r *http.Request) {
		if r.Method != "GET" {
			http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
			return
		}

		// Check for authorization header
		auth := r.Header.Get("Authorization")
		if auth == "" {
			w.WriteHeader(http.StatusUnauthorized)
			json.NewEncoder(w).Encode(map[string]string{"error": "Unauthorized"})
			return
		}

		// Mock response data
		mockWorkflows := map[string]interface{}{
			"workflows": []map[string]interface{}{
				{
					"id":          "test-workflow-123",
					"name":        "Test Deploy Workflow",
					"description": "A test deployment workflow",
					"status":      "published",
					"owner_id":    "user-123",
					"user_name":   "Test User",
					"created_at":  "2024-01-01T10:00:00.000000+00:00",
					"updated_at":  "2024-01-02T15:30:00.000000+00:00",
					"recent_executions": []map[string]interface{}{
						{
							"id":           "exec-123",
							"status":       "completed",
							"started_at":   "2024-01-02T14:00:00.000000+00:00",
							"finished_at":  "2024-01-02T14:05:00.000000+00:00",
							"duration_ms":  300000,
							"trigger_type": "manual",
						},
					},
				},
				{
					"id":          "test-workflow-456",
					"name":        "Incident Response",
					"description": "Automated incident response workflow",
					"status":      "draft",
					"owner_id":    "user-456",
					"user_name":   "Admin User",
					"created_at":  "2024-01-03T09:00:00.000000+00:00",
					"updated_at":  "2024-01-03T12:00:00.000000+00:00",
					"recent_executions": []map[string]interface{}{},
				},
			},
			"total":       2,
			"page":        1,
			"totalPages":  1,
			"pageSize":    12,
			"status":      "all",
		}

		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(mockWorkflows)
	})

	// Mock GET /workflows/{id} - get specific workflow
	mux.HandleFunc("/workflows/", func(w http.ResponseWriter, r *http.Request) {
		if r.Method == "GET" {
			// Extract workflow ID from path
			path := strings.TrimPrefix(r.URL.Path, "/workflows/")
			workflowID := strings.Split(path, "/")[0]

			// Check for authorization header
			auth := r.Header.Get("Authorization")
			if auth == "" {
				w.WriteHeader(http.StatusUnauthorized)
				json.NewEncoder(w).Encode(map[string]string{"error": "Unauthorized"})
				return
			}

			var mockWorkflow map[string]interface{}
			switch workflowID {
			case "test-workflow-123":
				mockWorkflow = map[string]interface{}{
					"id":          "test-workflow-123",
					"name":        "Test Deploy Workflow",
					"description": "A test deployment workflow",
					"status":      "published",
					"owner_id":    "user-123",
					"user_name":   "Test User",
					"created_at":  "2024-01-01T10:00:00.000000+00:00",
					"updated_at":  "2024-01-02T15:30:00.000000+00:00",
				}
			case "test-workflow-456":
				mockWorkflow = map[string]interface{}{
					"id":          "test-workflow-456",
					"name":        "Incident Response",
					"description": "Automated incident response workflow",
					"status":      "draft",
					"owner_id":    "user-456",
					"user_name":   "Admin User",
					"created_at":  "2024-01-03T09:00:00.000000+00:00",
					"updated_at":  "2024-01-03T12:00:00.000000+00:00",
				}
			default:
				w.WriteHeader(http.StatusNotFound)
				json.NewEncoder(w).Encode(map[string]string{"error": "Workflow not found"})
				return
			}

			w.Header().Set("Content-Type", "application/json")
			json.NewEncoder(w).Encode(mockWorkflow)
		} else if r.Method == "POST" && strings.HasSuffix(r.URL.Path, "/execute") {
			// Mock POST /workflows/{id}/execute
			workflowID := strings.TrimSuffix(strings.TrimPrefix(r.URL.Path, "/workflows/"), "/execute")

			// Check for authorization header
			auth := r.Header.Get("Authorization")
			if auth == "" {
				w.WriteHeader(http.StatusUnauthorized)
				json.NewEncoder(w).Encode(map[string]string{"error": "Unauthorized"})
				return
			}

			// Parse request body
			var reqBody map[string]interface{}
			if err := json.NewDecoder(r.Body).Decode(&reqBody); err != nil {
				w.WriteHeader(http.StatusBadRequest)
				json.NewEncoder(w).Encode(map[string]string{"error": "Invalid request body"})
				return
			}

			// Validate runner is kubiya-hosted
			runner, ok := reqBody["runner"].(string)
			if !ok || runner != "kubiya-hosted" {
				w.WriteHeader(http.StatusBadRequest)
				json.NewEncoder(w).Encode(map[string]string{
					"error": "Invalid runner. Must be kubiya-hosted",
				})
				return
			}

			// Mock successful execution response
			mockResponse := map[string]interface{}{
				"executionId":         "exec-" + workflowID + "-" + fmt.Sprintf("%d", time.Now().Unix()),
				"requestId":          "req-" + fmt.Sprintf("%d", time.Now().Unix()),
				"supabaseExecutionId": "sb-exec-123",
				"workflowId":         workflowID,
				"status":             "running",
				"message":            "Workflow execution started successfully",
				"streamUrl":          "/api/workflows/executions/" + workflowID + "/stream",
				"statusUrl":          "/api/workflows/executions/" + workflowID + "/status",
			}

			w.Header().Set("Content-Type", "application/json")
			json.NewEncoder(w).Encode(mockResponse)
		}
	})

	return httptest.NewServer(mux)
}

func TestWorkflowListIntegration(t *testing.T) {
	// Start mock server
	server := MockComposerServer()
	defer server.Close()

	tests := []struct {
		name           string
		apiKey         string
		expectedStatus int
		expectError    bool
		checkOutput    func(output string) bool
	}{
		{
			name:           "successful workflow listing with valid API key",
			apiKey:         "valid-api-key",
			expectedStatus: 0,
			expectError:    false,
			checkOutput: func(output string) bool {
				return strings.Contains(output, "Test Deploy Workflow") &&
					strings.Contains(output, "Incident Response") &&
					strings.Contains(output, "✓ published") &&
					strings.Contains(output, "✎ draft")
			},
		},
		{
			name:           "unauthorized with empty API key",
			apiKey:         "",
			expectedStatus: 1,
			expectError:    true,
			checkOutput: func(output string) bool {
				return strings.Contains(output, "Unauthorized") || strings.Contains(output, "401")
			},
		},
		{
			name:           "json output format",
			apiKey:         "valid-api-key",
			expectedStatus: 0,
			expectError:    false,
			checkOutput: func(output string) bool {
				// Should be valid JSON containing workflows
				var result map[string]interface{}
				err := json.Unmarshal([]byte(output), &result)
				if err != nil {
					return false
				}
				workflows, ok := result["workflows"].([]interface{})
				return ok && len(workflows) == 2
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Create test config
			cfg := &config.Config{
				BaseURL: server.URL,
				APIKey:  tt.apiKey,
				Debug:   false,
			}

			// Capture output
			var outputBuf bytes.Buffer

			// Create command
			cmd := newWorkflowListCommand(cfg)
			cmd.SetOut(&outputBuf)
			cmd.SetErr(&outputBuf)

			// Set arguments for JSON test
			if strings.Contains(tt.name, "json") {
				cmd.SetArgs([]string{"--json", "--limit", "10"})
			} else {
				cmd.SetArgs([]string{"--limit", "10"})
			}

			// Execute command
			err := cmd.Execute()

			// Check results
			output := outputBuf.String()

			if tt.expectError && err == nil {
				t.Errorf("Expected error but got none. Output: %s", output)
			}

			if !tt.expectError && err != nil {
				t.Errorf("Unexpected error: %v. Output: %s", err, output)
			}

			if !tt.checkOutput(output) {
				t.Errorf("Output check failed. Got output: %s", output)
			}
		})
	}
}

func TestWorkflowRunIntegration(t *testing.T) {
	// Start mock server
	server := MockComposerServer()
	defer server.Close()

	tests := []struct {
		name           string
		apiKey         string
		workflowID     string
		variables      []string
		expectError    bool
		checkOutput    func(output string) bool
	}{
		{
			name:       "successful execution by ID",
			apiKey:     "valid-api-key",
			workflowID: "test-workflow-123",
			variables:  []string{"env=production", "version=v1.2.3"},
			expectError: false,
			checkOutput: func(output string) bool {
				return strings.Contains(output, "Workflow execution started successfully") &&
					strings.Contains(output, "Test Deploy Workflow") &&
					strings.Contains(output, "kubiya-hosted") &&
					strings.Contains(output, "env = production") &&
					strings.Contains(output, "version = v1.2.3")
			},
		},
		{
			name:       "successful execution by name",
			apiKey:     "valid-api-key",
			workflowID: "Incident Response",
			variables:  []string{},
			expectError: false,
			checkOutput: func(output string) bool {
				return strings.Contains(output, "Workflow execution started successfully") &&
					strings.Contains(output, "Incident Response")
			},
		},
		{
			name:       "workflow not found",
			apiKey:     "valid-api-key",
			workflowID: "nonexistent-workflow",
			variables:  []string{},
			expectError: true,
			checkOutput: func(output string) bool {
				return strings.Contains(output, "no workflow found") ||
					strings.Contains(output, "not found")
			},
		},
		{
			name:       "unauthorized execution",
			apiKey:     "",
			workflowID: "test-workflow-123",
			variables:  []string{},
			expectError: true,
			checkOutput: func(output string) bool {
				return strings.Contains(output, "Unauthorized") || strings.Contains(output, "401")
			},
		},
		{
			name:       "json output format",
			apiKey:     "valid-api-key",
			workflowID: "test-workflow-123",
			variables:  []string{"env=test"},
			expectError: false,
			checkOutput: func(output string) bool {
				var result map[string]interface{}
				err := json.Unmarshal([]byte(output), &result)
				if err != nil {
					return false
				}
				_, hasWorkflowID := result["workflow_id"]
				_, hasExecutionID := result["execution_id"]
				_, hasStatus := result["status"]
				return hasWorkflowID && hasExecutionID && hasStatus
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Create test config
			cfg := &config.Config{
				BaseURL: server.URL,
				APIKey:  tt.apiKey,
				Debug:   false,
			}

			// Capture output
			var outputBuf bytes.Buffer

			// Create command
			cmd := newWorkflowRunCommand(cfg)
			cmd.SetOut(&outputBuf)
			cmd.SetErr(&outputBuf)

			// Prepare arguments
			args := []string{tt.workflowID}
			for _, v := range tt.variables {
				args = append(args, "--var", v)
			}
			if strings.Contains(tt.name, "json") {
				args = append(args, "--json")
			}

			cmd.SetArgs(args)

			// Execute command
			err := cmd.Execute()

			// Check results
			output := outputBuf.String()

			if tt.expectError && err == nil {
				t.Errorf("Expected error but got none. Output: %s", output)
			}

			if !tt.expectError && err != nil {
				t.Errorf("Unexpected error: %v. Output: %s", err, output)
			}

			if !tt.checkOutput(output) {
				t.Errorf("Output check failed. Got output: %s", output)
			}
		})
	}
}

func TestWorkflowDiscoveryIntegration(t *testing.T) {
	// Start mock server
	server := MockComposerServer()
	defer server.Close()

	tests := []struct {
		name        string
		searchTerm  string
		expectFound bool
		expectedID  string
	}{
		{
			name:        "find by exact ID",
			searchTerm:  "test-workflow-123",
			expectFound: true,
			expectedID:  "test-workflow-123",
		},
		{
			name:        "find by exact name",
			searchTerm:  "Test Deploy Workflow",
			expectFound: true,
			expectedID:  "test-workflow-123",
		},
		{
			name:        "find by partial name match",
			searchTerm:  "incident",
			expectFound: true,
			expectedID:  "test-workflow-456",
		},
		{
			name:        "not found",
			searchTerm:  "nonexistent-workflow-xyz",
			expectFound: false,
			expectedID:  "",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			cfg := &config.Config{
				BaseURL: server.URL,
				APIKey:  "valid-api-key",
				Debug:   false,
			}

			// Test the findWorkflowByIdentifier function with mock client
			ctx := context.Background()
			comp := NewMockComposerClient(server.URL, cfg.APIKey)

			// Use a custom implementation for testing since we can't easily mock the composer.Client
			workflow, err := findWorkflowByIdentifierMock(ctx, comp, tt.searchTerm)

			if tt.expectFound {
				if err != nil {
					t.Errorf("Expected to find workflow but got error: %v", err)
				} else if workflow.ID != tt.expectedID {
					t.Errorf("Expected workflow ID %s, got %s", tt.expectedID, workflow.ID)
				}
			} else {
				if err == nil {
					t.Errorf("Expected error for workflow not found, but found workflow: %+v", workflow)
				}
			}
		})
	}
}

// Mock composer client for testing
type MockComposerClient struct {
	baseURL string
	apiKey  string
}

func NewMockComposerClient(baseURL, apiKey string) *MockComposerClient {
	return &MockComposerClient{
		baseURL: baseURL,
		apiKey:  apiKey,
	}
}

func (c *MockComposerClient) ListWorkflows(ctx context.Context, params interface{}) (*MockWorkflowsResponse, error) {
	// Make HTTP request to mock server
	req, err := http.NewRequestWithContext(ctx, "GET", c.baseURL+"/workflows", nil)
	if err != nil {
		return nil, err
	}

	if c.apiKey != "" {
		req.Header.Set("Authorization", "Bearer "+c.apiKey)
	}

	client := &http.Client{Timeout: 10 * time.Second}
	resp, err := client.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("API returned status %d", resp.StatusCode)
	}

	var result MockWorkflowsResponse
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return nil, err
	}

	return &result, nil
}

func (c *MockComposerClient) GetWorkflow(ctx context.Context, workflowID string) (*MockWorkflow, error) {
	req, err := http.NewRequestWithContext(ctx, "GET", c.baseURL+"/workflows/"+workflowID, nil)
	if err != nil {
		return nil, err
	}

	if c.apiKey != "" {
		req.Header.Set("Authorization", "Bearer "+c.apiKey)
	}

	client := &http.Client{Timeout: 10 * time.Second}
	resp, err := client.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	if resp.StatusCode == http.StatusNotFound {
		return nil, fmt.Errorf("workflow not found")
	}

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("API returned status %d", resp.StatusCode)
	}

	var result MockWorkflow
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return nil, err
	}

	return &result, nil
}

// Mock response types
type MockWorkflowsResponse struct {
	Workflows  []MockWorkflow `json:"workflows"`
	Total      int            `json:"total"`
	Page       int            `json:"page"`
	TotalPages int            `json:"totalPages"`
	PageSize   int            `json:"pageSize"`
	Status     string         `json:"status"`
}

type MockWorkflow struct {
	ID          string `json:"id"`
	Name        string `json:"name"`
	Description string `json:"description"`
	Status      string `json:"status"`
	OwnerID     string `json:"owner_id"`
	UserName    string `json:"user_name"`
	CreatedAt   string `json:"created_at"`
	UpdatedAt   string `json:"updated_at"`
}

// Adapter function to make mock client work with findWorkflowByIdentifier
func (c *MockComposerClient) ListWorkflowsCompat(ctx context.Context, params composer.WorkflowParams) (*composer.Workflows, error) {
	mockResp, err := c.ListWorkflows(ctx, params)
	if err != nil {
		return nil, err
	}

	// Convert mock response to real response format
	workflows := make([]composer.Workflow, len(mockResp.Workflows))
	for i, mw := range mockResp.Workflows {
		workflows[i] = composer.Workflow{
			ID:          mw.ID,
			Name:        mw.Name,
			Description: mw.Description,
			Status:      mw.Status,
			OwnerID:     mw.OwnerID,
			UserName:    mw.UserName,
			CreatedAt:   mw.CreatedAt,
			UpdatedAt:   mw.UpdatedAt,
		}
	}

	return &composer.Workflows{
		Workflows:  workflows,
		Total:      mockResp.Total,
		Page:       mockResp.Page,
		TotalPages: mockResp.TotalPages,
		PageSize:   mockResp.PageSize,
		Status:     mockResp.Status,
	}, nil
}

func (c *MockComposerClient) GetWorkflowCompat(ctx context.Context, workflowID string) (*composer.Workflow, error) {
	mockWorkflow, err := c.GetWorkflow(ctx, workflowID)
	if err != nil {
		return nil, err
	}

	return &composer.Workflow{
		ID:          mockWorkflow.ID,
		Name:        mockWorkflow.Name,
		Description: mockWorkflow.Description,
		Status:      mockWorkflow.Status,
		OwnerID:     mockWorkflow.OwnerID,
		UserName:    mockWorkflow.UserName,
		CreatedAt:   mockWorkflow.CreatedAt,
		UpdatedAt:   mockWorkflow.UpdatedAt,
	}, nil
}

// Mock implementation of findWorkflowByIdentifier for testing
func findWorkflowByIdentifierMock(ctx context.Context, comp *MockComposerClient, identifier string) (*composer.Workflow, error) {
	// First try to get by ID directly
	workflow, err := comp.GetWorkflowCompat(ctx, identifier)
	if err == nil {
		return workflow, nil
	}

	// If that fails, search by name
	workflows, err := comp.ListWorkflowsCompat(ctx, composer.WorkflowParams{
		Search: identifier,
		Limit:  100,
	})
	if err != nil {
		return nil, fmt.Errorf("failed to search for workflow: %w", err)
	}

	// Look for exact name match
	var matches []*composer.Workflow
	for i := range workflows.Workflows {
		if workflows.Workflows[i].Name == identifier {
			matches = append(matches, &workflows.Workflows[i])
		}
	}

	// If no exact match, look for partial matches
	if len(matches) == 0 {
		for i := range workflows.Workflows {
			if strings.Contains(strings.ToLower(workflows.Workflows[i].Name), strings.ToLower(identifier)) {
				matches = append(matches, &workflows.Workflows[i])
			}
		}
	}

	if len(matches) == 0 {
		return nil, fmt.Errorf("no workflow found with ID or name '%s'", identifier)
	}

	if len(matches) > 1 {
		return nil, fmt.Errorf("multiple workflows found")
	}

	return matches[0], nil
}