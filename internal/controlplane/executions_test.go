package controlplane

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/kubiyabot/cli/internal/controlplane/entities"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestExecuteAgentV2(t *testing.T) {
	tests := []struct {
		name           string
		agentID        string
		request        *entities.ExecuteAgentRequest
		mockResponse   *entities.AgentExecution
		mockStatusCode int
		expectError    bool
	}{
		{
			name:    "successful agent execution",
			agentID: "test-agent-123",
			request: &entities.ExecuteAgentRequest{
				Prompt:        "Deploy to production",
				WorkerQueueID: "queue-123",
			},
			mockResponse: &entities.AgentExecution{
				ID:            "exec-123",
				ExecutionType: "agent",
				EntityID:      "test-agent-123",
				Prompt:        "Deploy to production",
				Status:        entities.ExecutionStatusPending,
			},
			mockStatusCode: http.StatusOK,
			expectError:    false,
		},
		{
			name:    "agent not found",
			agentID: "non-existent",
			request: &entities.ExecuteAgentRequest{
				Prompt:        "Test",
				WorkerQueueID: "queue-123",
			},
			mockStatusCode: http.StatusNotFound,
			expectError:    true,
		},
		{
			name:    "invalid worker queue",
			agentID: "test-agent-123",
			request: &entities.ExecuteAgentRequest{
				Prompt:        "Test",
				WorkerQueueID: "invalid-queue",
			},
			mockStatusCode: http.StatusBadRequest,
			expectError:    true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Create mock server
			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				// Verify request
				assert.Equal(t, "POST", r.Method)
				assert.Equal(t, "/api/v1/agents/"+tt.agentID+"/execute", r.URL.Path)
				assert.Equal(t, "Bearer test-api-key", r.Header.Get("Authorization"))

				// Decode request body
				var req entities.ExecuteAgentRequest
				err := json.NewDecoder(r.Body).Decode(&req)
				require.NoError(t, err)
				assert.Equal(t, tt.request.Prompt, req.Prompt)
				assert.Equal(t, tt.request.WorkerQueueID, req.WorkerQueueID)

				// Send response
				w.WriteHeader(tt.mockStatusCode)
				if tt.mockResponse != nil {
					json.NewEncoder(w).Encode(tt.mockResponse)
				}
			}))
			defer server.Close()

			// Create client
			client := &Client{
				APIKey:     "test-api-key",
				BaseURL:    server.URL,
				HTTPClient: &http.Client{Timeout: 5 * time.Second},
				Debug:      false,
			}

			// Execute
			execution, err := client.ExecuteAgentV2(tt.agentID, tt.request)

			// Assert
			if tt.expectError {
				assert.Error(t, err)
				assert.Nil(t, execution)
			} else {
				assert.NoError(t, err)
				assert.NotNil(t, execution)
				assert.Equal(t, tt.mockResponse.ID, execution.ID)
				assert.Equal(t, tt.mockResponse.ExecutionType, execution.ExecutionType)
				assert.Equal(t, tt.mockResponse.Status, execution.Status)
			}
		})
	}
}

func TestExecuteTeamV2(t *testing.T) {
	tests := []struct {
		name           string
		teamID         string
		request        *entities.ExecuteTeamRequest
		mockResponse   *entities.AgentExecution
		mockStatusCode int
		expectError    bool
	}{
		{
			name:   "successful team execution",
			teamID: "test-team-456",
			request: &entities.ExecuteTeamRequest{
				Prompt:        "Analyze security logs",
				WorkerQueueID: "queue-456",
			},
			mockResponse: &entities.AgentExecution{
				ID:            "exec-456",
				ExecutionType: "team",
				EntityID:      "test-team-456",
				Prompt:        "Analyze security logs",
				Status:        entities.ExecutionStatusPending,
			},
			mockStatusCode: http.StatusOK,
			expectError:    false,
		},
		{
			name:   "team not found",
			teamID: "non-existent",
			request: &entities.ExecuteTeamRequest{
				Prompt:        "Test",
				WorkerQueueID: "queue-456",
			},
			mockStatusCode: http.StatusNotFound,
			expectError:    true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Create mock server
			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				// Verify request
				assert.Equal(t, "POST", r.Method)
				assert.Equal(t, "/api/v1/teams/"+tt.teamID+"/execute", r.URL.Path)

				// Send response
				w.WriteHeader(tt.mockStatusCode)
				if tt.mockResponse != nil {
					json.NewEncoder(w).Encode(tt.mockResponse)
				}
			}))
			defer server.Close()

			// Create client
			client := &Client{
				APIKey:     "test-api-key",
				BaseURL:    server.URL,
				HTTPClient: &http.Client{Timeout: 5 * time.Second},
				Debug:      false,
			}

			// Execute
			execution, err := client.ExecuteTeamV2(tt.teamID, tt.request)

			// Assert
			if tt.expectError {
				assert.Error(t, err)
				assert.Nil(t, execution)
			} else {
				assert.NoError(t, err)
				assert.NotNil(t, execution)
				assert.Equal(t, tt.mockResponse.ID, execution.ID)
				assert.Equal(t, tt.mockResponse.ExecutionType, execution.ExecutionType)
			}
		})
	}
}

func TestGetExecution(t *testing.T) {
	tests := []struct {
		name           string
		executionID    string
		mockResponse   *entities.AgentExecution
		mockStatusCode int
		expectError    bool
	}{
		{
			name:        "successful get execution",
			executionID: "exec-789",
			mockResponse: &entities.AgentExecution{
				ID:            "exec-789",
				ExecutionType: "agent",
				EntityID:      "agent-123",
				Status:        entities.ExecutionStatusCompleted,
				Response:      stringPtr("Task completed successfully"),
			},
			mockStatusCode: http.StatusOK,
			expectError:    false,
		},
		{
			name:           "execution not found",
			executionID:    "non-existent",
			mockStatusCode: http.StatusNotFound,
			expectError:    true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Create mock server
			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				assert.Equal(t, "GET", r.Method)
				assert.Equal(t, "/api/v1/executions/"+tt.executionID, r.URL.Path)

				w.WriteHeader(tt.mockStatusCode)
				if tt.mockResponse != nil {
					// API returns an array with a single execution
					json.NewEncoder(w).Encode([]*entities.AgentExecution{tt.mockResponse})
				}
			}))
			defer server.Close()

			client := &Client{
				APIKey:     "test-api-key",
				BaseURL:    server.URL,
				HTTPClient: &http.Client{Timeout: 5 * time.Second},
				Debug:      false,
			}

			execution, err := client.GetExecution(tt.executionID)

			if tt.expectError {
				assert.Error(t, err)
				assert.Nil(t, execution)
			} else {
				assert.NoError(t, err)
				assert.NotNil(t, execution)
				assert.Equal(t, tt.mockResponse.ID, execution.ID)
				assert.Equal(t, tt.mockResponse.Status, execution.Status)
			}
		})
	}
}

func TestListExecutions(t *testing.T) {
	tests := []struct {
		name           string
		filters        map[string]string
		mockResponse   []*entities.AgentExecution
		mockStatusCode int
		expectError    bool
	}{
		{
			name:    "list all executions",
			filters: nil,
			mockResponse: []*entities.AgentExecution{
				{
					ID:            "exec-1",
					ExecutionType: "agent",
					Status:        entities.ExecutionStatusCompleted,
				},
				{
					ID:            "exec-2",
					ExecutionType: "team",
					Status:        entities.ExecutionStatusRunning,
				},
			},
			mockStatusCode: http.StatusOK,
			expectError:    false,
		},
		{
			name: "list with filters",
			filters: map[string]string{
				"status":        "completed",
				"execution_type": "agent",
			},
			mockResponse: []*entities.AgentExecution{
				{
					ID:            "exec-1",
					ExecutionType: "agent",
					Status:        entities.ExecutionStatusCompleted,
				},
			},
			mockStatusCode: http.StatusOK,
			expectError:    false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				assert.Equal(t, "GET", r.Method)
				assert.Contains(t, r.URL.Path, "/api/v1/executions")

				// Verify filters in query params
				for k, v := range tt.filters {
					assert.Equal(t, v, r.URL.Query().Get(k))
				}

				w.WriteHeader(tt.mockStatusCode)
				if tt.mockResponse != nil {
					json.NewEncoder(w).Encode(tt.mockResponse)
				}
			}))
			defer server.Close()

			client := &Client{
				APIKey:     "test-api-key",
				BaseURL:    server.URL,
				HTTPClient: &http.Client{Timeout: 5 * time.Second},
				Debug:      false,
			}

			executions, err := client.ListExecutions(tt.filters)

			if tt.expectError {
				assert.Error(t, err)
			} else {
				assert.NoError(t, err)
				assert.Equal(t, len(tt.mockResponse), len(executions))
			}
		})
	}
}

func TestStreamExecutionOutput(t *testing.T) {
	tests := []struct {
		name         string
		executionID  string
		sseEvents    []string
		expectedType []string
		expectError  bool
	}{
		{
			name:        "successful streaming with chunks",
			executionID: "exec-stream-1",
			sseEvents: []string{
				`data: {"type":"chunk","content":"Starting deployment..."}`,
				`data: {"type":"chunk","content":"Deploying to server..."}`,
				`data: {"type":"chunk","content":"Deployment complete!"}`,
				`data: {"type":"complete"}`,
			},
			expectedType: []string{"chunk", "chunk", "chunk", "complete"},
			expectError:  false,
		},
		{
			name:        "streaming with error",
			executionID: "exec-stream-2",
			sseEvents: []string{
				`data: {"type":"chunk","content":"Starting..."}`,
				`data: {"type":"error","content":"Deployment failed: connection timeout"}`,
			},
			expectedType: []string{"chunk", "error"},
			expectError:  false,
		},
		{
			name:        "empty stream",
			executionID: "exec-stream-3",
			sseEvents:   []string{},
			expectedType: []string{},
			expectError:  false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Create mock SSE server
			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				assert.Equal(t, "GET", r.Method)
				assert.Contains(t, r.URL.Path, "/api/v1/executions/"+tt.executionID+"/stream")
				assert.Equal(t, "text/event-stream", r.Header.Get("Accept"))

				w.Header().Set("Content-Type", "text/event-stream")
				w.Header().Set("Cache-Control", "no-cache")
				w.Header().Set("Connection", "keep-alive")
				w.WriteHeader(http.StatusOK)

				flusher, ok := w.(http.Flusher)
				require.True(t, ok, "ResponseWriter doesn't support flushing")

				// Send SSE events
				for _, event := range tt.sseEvents {
					w.Write([]byte(event + "\n"))
					flusher.Flush()
					time.Sleep(10 * time.Millisecond) // Simulate real-time streaming
				}
			}))
			defer server.Close()

			client := &Client{
				APIKey:     "test-api-key",
				BaseURL:    server.URL,
				HTTPClient: &http.Client{Timeout: 5 * time.Second},
				Debug:      false,
			}

			ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
			defer cancel()

			eventChan, errChan := client.StreamExecutionOutput(ctx, tt.executionID)

			var receivedEvents []entities.StreamEvent
			done := false

			for !done {
				select {
				case event, ok := <-eventChan:
					if !ok {
						done = true
						break
					}
					receivedEvents = append(receivedEvents, event)
					if event.Type == "complete" || event.Type == "error" {
						done = true
					}
				case err := <-errChan:
					if err != nil {
						if tt.expectError {
							assert.Error(t, err)
							return
						}
						t.Fatalf("Unexpected error: %v", err)
					}
				case <-ctx.Done():
					done = true
				}
			}

			// Verify received events
			assert.Equal(t, len(tt.expectedType), len(receivedEvents), "Expected %d events, got %d", len(tt.expectedType), len(receivedEvents))
			for i, expectedType := range tt.expectedType {
				if i < len(receivedEvents) {
					assert.Equal(t, expectedType, receivedEvents[i].Type, "Event %d type mismatch", i)
				}
			}
		})
	}
}

func TestStreamExecutionOutputCancellation(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/event-stream")
		w.WriteHeader(http.StatusOK)

		flusher := w.(http.Flusher)

		// Send continuous events
		for i := 0; i < 100; i++ {
			w.Write([]byte(`data: {"type":"chunk","content":"data"}` + "\n"))
			flusher.Flush()
			time.Sleep(50 * time.Millisecond)
		}
	}))
	defer server.Close()

	client := &Client{
		APIKey:     "test-api-key",
		BaseURL:    server.URL,
		HTTPClient: &http.Client{Timeout: 10 * time.Second},
		Debug:      false,
	}

	ctx, cancel := context.WithCancel(context.Background())

	eventChan, errChan := client.StreamExecutionOutput(ctx, "test-exec")

	// Receive a few events
	count := 0
	for count < 3 {
		select {
		case <-eventChan:
			count++
		case <-errChan:
		case <-time.After(2 * time.Second):
			t.Fatal("Timeout waiting for events")
		}
	}

	// Cancel the context
	cancel()

	// Give the goroutine time to detect cancellation and close channels
	time.Sleep(100 * time.Millisecond)

	// Verify stream stops - drain remaining events and check channel eventually closes
	timeout := time.After(2 * time.Second)
	channelClosed := false
	for !channelClosed {
		select {
		case _, ok := <-eventChan:
			if !ok {
				channelClosed = true
			}
			// If ok=true, drain the event and continue
		case <-timeout:
			t.Error("Event channel did not close within 2 seconds of context cancellation")
			return
		}
	}
}

// Helper function
func stringPtr(s string) *string {
	return &s
}
