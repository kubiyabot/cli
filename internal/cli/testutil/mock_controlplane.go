package testutil

import (
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"sync"

	"github.com/kubiyabot/cli/internal/controlplane/entities"
)

// MockControlPlaneServer mocks the Kubiya control plane HTTP API for testing
type MockControlPlaneServer struct {
	server *httptest.Server
	mu     sync.RWMutex

	// State
	queues     map[string]*entities.WorkerQueueConfig
	locks      map[string]*entities.UpdateLock
	apiKey     string
	baseURL    string
	reqLog     []RequestLog
	errorMode  bool
	errorCount int
}

// RequestLog records API requests for testing
type RequestLog struct {
	Method string
	Path   string
	Body   string
}

// NewMockControlPlaneServer creates a new mock control plane server
func NewMockControlPlaneServer() *MockControlPlaneServer {
	mcp := &MockControlPlaneServer{
		queues: make(map[string]*entities.WorkerQueueConfig),
		locks:  make(map[string]*entities.UpdateLock),
		apiKey: "test-api-key",
		reqLog: []RequestLog{},
	}

	// Create the HTTP handler
	mux := http.NewServeMux()

	// Worker queue endpoints
	mux.HandleFunc("/api/v1/worker-queues", mcp.handleListWorkerQueues)
	mux.HandleFunc("/api/v1/worker-queues/", mcp.handleWorkerQueueOperations)

	// Start the test server
	mcp.server = httptest.NewServer(mux)
	mcp.baseURL = mcp.server.URL

	return mcp
}

// URL returns the base URL of the mock server
func (m *MockControlPlaneServer) URL() string {
	return m.baseURL
}

// APIKey returns the API key for authentication
func (m *MockControlPlaneServer) APIKey() string {
	return m.apiKey
}

// Close shuts down the mock server
func (m *MockControlPlaneServer) Close() {
	m.server.Close()
}

// AddQueue adds a worker queue to the mock control plane
func (m *MockControlPlaneServer) AddQueue(queue *entities.WorkerQueueConfig) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.queues[queue.QueueID] = queue
}

// RemoveQueue removes a worker queue from the mock control plane
func (m *MockControlPlaneServer) RemoveQueue(queueID string) {
	m.mu.Lock()
	defer m.mu.Unlock()
	delete(m.queues, queueID)
}

// UpdateQueue updates a worker queue in the mock control plane
func (m *MockControlPlaneServer) UpdateQueue(queue *entities.WorkerQueueConfig) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.queues[queue.QueueID] = queue
}

// GetQueue gets a worker queue from the mock control plane
func (m *MockControlPlaneServer) GetQueue(queueID string) *entities.WorkerQueueConfig {
	m.mu.RLock()
	defer m.mu.RUnlock()
	return m.queues[queueID]
}

// ListQueues lists all worker queues
func (m *MockControlPlaneServer) ListQueues() []*entities.WorkerQueueConfig {
	m.mu.RLock()
	defer m.mu.RUnlock()

	queues := make([]*entities.WorkerQueueConfig, 0, len(m.queues))
	for _, q := range m.queues {
		queues = append(queues, q)
	}
	return queues
}

// SetErrorMode enables/disables error mode for testing error handling
func (m *MockControlPlaneServer) SetErrorMode(enabled bool) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.errorMode = enabled
	m.errorCount = 0
}

// GetRequestLog returns the log of API requests
func (m *MockControlPlaneServer) GetRequestLog() []RequestLog {
	m.mu.RLock()
	defer m.mu.RUnlock()
	return append([]RequestLog{}, m.reqLog...)
}

// ClearRequestLog clears the request log
func (m *MockControlPlaneServer) ClearRequestLog() {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.reqLog = []RequestLog{}
}

// handleListWorkerQueues handles GET /api/v1/worker-queues
func (m *MockControlPlaneServer) handleListWorkerQueues(w http.ResponseWriter, r *http.Request) {
	m.logRequest(r, "")

	if !m.checkAuth(r, w) {
		return
	}

	if m.shouldReturnError() {
		http.Error(w, "Internal server error", http.StatusInternalServerError)
		return
	}

	if r.Method != http.MethodGet {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	m.mu.RLock()
	defer m.mu.RUnlock()

	// Convert map to slice
	queues := make([]*entities.WorkerQueue, 0, len(m.queues))
	for _, config := range m.queues {
		maxWorkers := 1
		if config.MaxWorkers != nil {
			maxWorkers = *config.MaxWorkers
		}
		queue := &entities.WorkerQueue{
			ID:            config.QueueID,
			Name:          config.Name,
			MaxWorkers:    maxWorkers,
			EnvironmentID: config.EnvironmentID,
		}
		queues = append(queues, queue)
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(queues)
}

// handleWorkerQueueOperations handles /api/v1/worker-queues/{queue_id}/*
func (m *MockControlPlaneServer) handleWorkerQueueOperations(w http.ResponseWriter, r *http.Request) {
	m.logRequest(r, "")

	if !m.checkAuth(r, w) {
		return
	}

	if m.shouldReturnError() {
		http.Error(w, "Internal server error", http.StatusInternalServerError)
		return
	}

	// Parse the URL path
	// Expected patterns:
	// /api/v1/worker-queues/{queue_id}
	// /api/v1/worker-queues/{queue_id}/config
	// /api/v1/worker-queues/{queue_id}/workers/{worker_id}/update-lock
	path := r.URL.Path

	// Extract queue ID (simplified parsing)
	// In a real implementation, you'd want more robust path parsing
	if r.Method == http.MethodGet && len(path) > len("/api/v1/worker-queues/") {
		// Check if it's a config request
		if contains(path, "/config") {
			m.handleGetQueueConfig(w, r, path)
			return
		}
	}

	http.Error(w, "Not found", http.StatusNotFound)
}

// handleGetQueueConfig handles GET /api/v1/worker-queues/{queue_id}/config
func (m *MockControlPlaneServer) handleGetQueueConfig(w http.ResponseWriter, r *http.Request, path string) {
	// Extract queue ID from path (simplified)
	// Path format: /api/v1/worker-queues/{queue_id}/config
	queueID := extractQueueIDFromPath(path)

	m.mu.RLock()
	config, exists := m.queues[queueID]
	m.mu.RUnlock()

	if !exists {
		http.Error(w, "Queue not found", http.StatusNotFound)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(config)
}

// checkAuth verifies the API key
func (m *MockControlPlaneServer) checkAuth(r *http.Request, w http.ResponseWriter) bool {
	authHeader := r.Header.Get("Authorization")
	expectedAuth := "Bearer " + m.apiKey

	if authHeader != expectedAuth {
		http.Error(w, "Unauthorized", http.StatusUnauthorized)
		return false
	}
	return true
}

// shouldReturnError checks if error mode is enabled
func (m *MockControlPlaneServer) shouldReturnError() bool {
	m.mu.Lock()
	defer m.mu.Unlock()

	if m.errorMode {
		m.errorCount++
		return true
	}
	return false
}

// logRequest logs an API request
func (m *MockControlPlaneServer) logRequest(r *http.Request, body string) {
	m.mu.Lock()
	defer m.mu.Unlock()

	m.reqLog = append(m.reqLog, RequestLog{
		Method: r.Method,
		Path:   r.URL.Path,
		Body:   body,
	})
}

// Helper functions

func contains(s, substr string) bool {
	return len(s) >= len(substr) && s[len(s)-len(substr):] == substr ||
		   len(s) > len(substr) && contains(s[:len(s)-1], substr)
}

func extractQueueIDFromPath(path string) string {
	// Path format: /api/v1/worker-queues/{queue_id}/config
	// Simple extraction - in production, use a proper router
	const prefix = "/api/v1/worker-queues/"
	const suffix = "/config"

	if len(path) <= len(prefix)+len(suffix) {
		return ""
	}

	// Remove prefix and suffix
	queueID := path[len(prefix):]
	if len(queueID) > len(suffix) && queueID[len(queueID)-len(suffix):] == suffix {
		queueID = queueID[:len(queueID)-len(suffix)]
	}

	return queueID
}

// Helper function to create a response
func respondJSON(w http.ResponseWriter, status int, data interface{}) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	if data != nil {
		json.NewEncoder(w).Encode(data)
	}
}

// Helper function to respond with error
func respondError(w http.ResponseWriter, status int, message string) {
	respondJSON(w, status, map[string]string{"error": message})
}

// AssertRequestCount checks if the server received the expected number of requests
func (m *MockControlPlaneServer) AssertRequestCount(expected int) error {
	m.mu.RLock()
	actual := len(m.reqLog)
	m.mu.RUnlock()

	if actual != expected {
		return fmt.Errorf("expected %d requests, got %d", expected, actual)
	}
	return nil
}

// AssertQueueExists checks if a queue exists in the mock server
func (m *MockControlPlaneServer) AssertQueueExists(queueID string) error {
	m.mu.RLock()
	_, exists := m.queues[queueID]
	m.mu.RUnlock()

	if !exists {
		return fmt.Errorf("expected queue %s to exist, but it doesn't", queueID)
	}
	return nil
}
