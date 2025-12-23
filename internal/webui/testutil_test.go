package webui

import (
	"encoding/json"
	"net/http/httptest"
	"testing"
	"time"
)

// newTestServer creates a new Server for testing
func newTestServer(t *testing.T) *Server {
	t.Helper()

	config := ServerConfig{
		QueueID:    "test-queue",
		WorkerDir:  t.TempDir(),
		Port:       0, // Auto-assign port
		WorkerPID:  12345,
		APIKey:     "test-api-key",
	}

	state := NewState()

	srv := &Server{
		config:    config,
		state:     state,
		stopCh:    make(chan struct{}),
		startTime: time.Now(),
	}

	return srv
}

// mockWorker creates a mock worker for testing
func mockWorker(id string) *WorkerInfo {
	return &WorkerInfo{
		ID:            id,
		QueueID:       "test-queue",
		Status:        WorkerStatusRunning,
		PID:           12345,
		StartedAt:     time.Now(),
		LastHeartbeat: time.Now(),
		TasksActive:   1,
		TasksTotal:    10,
		Version:       "1.0.0",
		Hostname:      "localhost",
	}
}

// mockSession creates a mock session for testing
func mockSession(id string) *SessionInfo {
	return &SessionInfo{
		ID:            id,
		Type:          SessionTypeExecution,
		Status:        SessionStatusActive,
		WorkerID:      "worker-1",
		AgentID:       "agent-1",
		AgentName:     "Test Agent",
		StartedAt:     time.Now(),
		Duration:      60 * time.Second,
		DurationStr:   "1m 0s",
		MessagesCount: 5,
	}
}

// mockLogEntry creates a mock log entry for testing
func mockLogEntry(level LogLevel, message string) LogEntry {
	return LogEntry{
		Timestamp: time.Now(),
		Level:     level,
		Component: "test",
		Message:   message,
		WorkerID:  "worker-1",
	}
}

// parseJSONResponse parses JSON response body into the provided struct
func parseJSONResponse(t *testing.T, rec *httptest.ResponseRecorder, v interface{}) {
	t.Helper()
	if err := json.NewDecoder(rec.Body).Decode(v); err != nil {
		t.Fatalf("failed to parse response JSON: %v", err)
	}
}

// assertStatusCode asserts the HTTP status code
func assertStatusCode(t *testing.T, rec *httptest.ResponseRecorder, expected int) {
	t.Helper()
	if rec.Code != expected {
		t.Errorf("expected status %d, got %d. body: %s", expected, rec.Code, rec.Body.String())
	}
}

// assertContentType asserts the Content-Type header
func assertContentType(t *testing.T, rec *httptest.ResponseRecorder, expected string) {
	t.Helper()
	ct := rec.Header().Get("Content-Type")
	if ct != expected {
		t.Errorf("expected Content-Type %q, got %q", expected, ct)
	}
}
