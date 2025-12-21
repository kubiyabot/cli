package streaming

import (
	"testing"
	"time"

	"github.com/kubiyabot/cli/internal/controlplane/entities"
)

func TestMapControlPlaneEvent_Chunk(t *testing.T) {
	cpEvent := entities.StreamEvent{
		Type:    entities.StreamEventTypeChunk,
		Content: "Hello, I am thinking...",
		Role:    "assistant",
	}

	event := MapControlPlaneEvent(cpEvent)

	if event.Type != EventTypeMessageChunk {
		t.Errorf("expected type=%s, got %s", EventTypeMessageChunk, event.Type)
	}
	if event.Message == nil {
		t.Fatal("expected Message to be set")
	}
	if event.Message.Content != "Hello, I am thinking..." {
		t.Errorf("expected content='Hello, I am thinking...', got '%s'", event.Message.Content)
	}
	if event.Message.Role != "assistant" {
		t.Errorf("expected role=assistant, got %s", event.Message.Role)
	}
	if !event.Message.Chunk {
		t.Error("expected chunk=true")
	}
}

func TestMapControlPlaneEvent_ToolStarted(t *testing.T) {
	cpEvent := entities.StreamEvent{
		Type:       entities.StreamEventTypeToolStarted,
		ToolName:   "shell",
		ToolInputs: map[string]interface{}{"command": "ls -la"},
	}

	event := MapControlPlaneEvent(cpEvent)

	if event.Type != EventTypeToolStarted {
		t.Errorf("expected type=%s, got %s", EventTypeToolStarted, event.Type)
	}
	if event.Tool == nil {
		t.Fatal("expected Tool to be set")
	}
	if event.Tool.Name != "shell" {
		t.Errorf("expected tool.name=shell, got %s", event.Tool.Name)
	}
	if event.Tool.Inputs["command"] != "ls -la" {
		t.Errorf("expected tool.inputs.command='ls -la', got %v", event.Tool.Inputs["command"])
	}
}

func TestMapControlPlaneEvent_ToolStarted_FromMetadata(t *testing.T) {
	cpEvent := entities.StreamEvent{
		Type: entities.StreamEventTypeToolStarted,
		Metadata: map[string]interface{}{
			"tool_name": "kubectl",
			"inputs":    map[string]interface{}{"args": "get pods"},
		},
	}

	event := MapControlPlaneEvent(cpEvent)

	if event.Type != EventTypeToolStarted {
		t.Errorf("expected type=%s, got %s", EventTypeToolStarted, event.Type)
	}
	if event.Tool == nil {
		t.Fatal("expected Tool to be set")
	}
	if event.Tool.Name != "kubectl" {
		t.Errorf("expected tool.name=kubectl, got %s", event.Tool.Name)
	}
}

func TestMapControlPlaneEvent_ToolCompleted(t *testing.T) {
	success := true
	duration := 1.5
	cpEvent := entities.StreamEvent{
		Type:        entities.StreamEventTypeToolCompleted,
		ToolName:    "shell",
		ToolOutputs: map[string]interface{}{"stdout": "file1.txt"},
		Duration:    &duration,
		Success:     &success,
	}

	event := MapControlPlaneEvent(cpEvent)

	if event.Type != EventTypeToolCompleted {
		t.Errorf("expected type=%s, got %s", EventTypeToolCompleted, event.Type)
	}
	if event.Tool == nil {
		t.Fatal("expected Tool to be set")
	}
	if event.Tool.Name != "shell" {
		t.Errorf("expected tool.name=shell, got %s", event.Tool.Name)
	}
	if event.Tool.DurationSeconds != 1.5 {
		t.Errorf("expected duration=1.5, got %f", event.Tool.DurationSeconds)
	}
	if !event.Tool.Success {
		t.Error("expected success=true")
	}
}

func TestMapControlPlaneEvent_Status(t *testing.T) {
	status := entities.ExecutionStatusRunning
	cpEvent := entities.StreamEvent{
		Type:   entities.StreamEventTypeStatus,
		Status: &status,
		Metadata: map[string]interface{}{
			"reason":         "Execution started",
			"previous_state": "pending",
		},
	}

	event := MapControlPlaneEvent(cpEvent)

	if event.Type != EventTypeStatus {
		t.Errorf("expected type=%s, got %s", EventTypeStatus, event.Type)
	}
	if event.Status == nil {
		t.Fatal("expected Status to be set")
	}
	if event.Status.State != "running" {
		t.Errorf("expected state=running, got %s", event.Status.State)
	}
	if event.Status.Reason != "Execution started" {
		t.Errorf("expected reason='Execution started', got %s", event.Status.Reason)
	}
	if event.Status.PreviousState != "pending" {
		t.Errorf("expected previous_state=pending, got %s", event.Status.PreviousState)
	}
}

func TestMapControlPlaneEvent_Error(t *testing.T) {
	cpEvent := entities.StreamEvent{
		Type:    entities.StreamEventTypeError,
		Content: "Connection failed",
		Metadata: map[string]interface{}{
			"code":        "CONN_ERR",
			"recoverable": true,
		},
	}

	event := MapControlPlaneEvent(cpEvent)

	if event.Type != EventTypeError {
		t.Errorf("expected type=%s, got %s", EventTypeError, event.Type)
	}
	if event.Error == nil {
		t.Fatal("expected Error to be set")
	}
	if event.Error.Message != "Connection failed" {
		t.Errorf("expected message='Connection failed', got %s", event.Error.Message)
	}
	if event.Error.Code != "CONN_ERR" {
		t.Errorf("expected code=CONN_ERR, got %s", event.Error.Code)
	}
	if !event.Error.Recoverable {
		t.Error("expected recoverable=true")
	}
}

func TestMapControlPlaneEvent_Complete(t *testing.T) {
	cpEvent := entities.StreamEvent{
		Type: entities.StreamEventTypeComplete,
	}

	event := MapControlPlaneEvent(cpEvent)

	if event.Type != EventTypeDone {
		t.Errorf("expected type=%s, got %s", EventTypeDone, event.Type)
	}
}

func TestMapControlPlaneEvent_Connected(t *testing.T) {
	cpEvent := entities.StreamEvent{
		Type: entities.StreamEventTypeConnected,
		Metadata: map[string]interface{}{
			"execution_id": "exec-123",
		},
	}

	event := MapControlPlaneEvent(cpEvent)

	if event.Type != EventTypeConnected {
		t.Errorf("expected type=%s, got %s", EventTypeConnected, event.Type)
	}
	if event.ExecutionID != "exec-123" {
		t.Errorf("expected execution_id=exec-123, got %s", event.ExecutionID)
	}
}

func TestMapControlPlaneEvent_WithTimestamp(t *testing.T) {
	ts := entities.CustomTime{Time: time.Date(2024, 1, 15, 10, 30, 0, 0, time.UTC)}
	cpEvent := entities.StreamEvent{
		Type:      entities.StreamEventTypeDone,
		Timestamp: &ts,
	}

	event := MapControlPlaneEvent(cpEvent)

	expectedTime := time.Date(2024, 1, 15, 10, 30, 0, 0, time.UTC)
	if !event.Timestamp.Equal(expectedTime) {
		t.Errorf("expected timestamp=%v, got %v", expectedTime, event.Timestamp)
	}
}

func TestMapPlanEvent_Progress(t *testing.T) {
	planEvent := PlanStreamEvent{
		Type: "progress",
		Data: map[string]interface{}{
			"stage":    "Analyzing",
			"message":  "Analyzing task requirements",
			"progress": float64(45),
		},
	}

	event := MapPlanEvent(planEvent)

	if event.Type != EventTypeProgress {
		t.Errorf("expected type=%s, got %s", EventTypeProgress, event.Type)
	}
	if event.Progress == nil {
		t.Fatal("expected Progress to be set")
	}
	if event.Progress.Stage != "Analyzing" {
		t.Errorf("expected stage=Analyzing, got %s", event.Progress.Stage)
	}
	if event.Progress.Percent != 45 {
		t.Errorf("expected percent=45, got %d", event.Progress.Percent)
	}
}

func TestMapPlanEvent_Thinking(t *testing.T) {
	planEvent := PlanStreamEvent{
		Type: "thinking",
		Data: map[string]interface{}{
			"content": "I need to analyze the user's request...",
		},
	}

	event := MapPlanEvent(planEvent)

	if event.Type != EventTypeMessageChunk {
		t.Errorf("expected type=%s, got %s", EventTypeMessageChunk, event.Type)
	}
	if event.Message == nil {
		t.Fatal("expected Message to be set")
	}
	if event.Message.Role != "planner" {
		t.Errorf("expected role=planner, got %s", event.Message.Role)
	}
	if event.Message.Content != "I need to analyze the user's request..." {
		t.Errorf("unexpected content: %s", event.Message.Content)
	}
}

func TestMapPlanEvent_ToolCall(t *testing.T) {
	planEvent := PlanStreamEvent{
		Type: "tool_call",
		Data: map[string]interface{}{
			"tool_name": "list_agents",
			"args":      map[string]interface{}{"limit": 10},
		},
	}

	event := MapPlanEvent(planEvent)

	if event.Type != EventTypeToolStarted {
		t.Errorf("expected type=%s, got %s", EventTypeToolStarted, event.Type)
	}
	if event.Tool == nil {
		t.Fatal("expected Tool to be set")
	}
	if event.Tool.Name != "list_agents" {
		t.Errorf("expected tool.name=list_agents, got %s", event.Tool.Name)
	}
}

func TestMapPlanEvent_Complete(t *testing.T) {
	planEvent := PlanStreamEvent{
		Type: "complete",
	}

	event := MapPlanEvent(planEvent)

	if event.Type != EventTypeDone {
		t.Errorf("expected type=%s, got %s", EventTypeDone, event.Type)
	}
}

func TestMapPlanEvent_Error(t *testing.T) {
	planEvent := PlanStreamEvent{
		Type: "error",
		Data: map[string]interface{}{
			"message": "Planning failed",
		},
	}

	event := MapPlanEvent(planEvent)

	if event.Type != EventTypeError {
		t.Errorf("expected type=%s, got %s", EventTypeError, event.Type)
	}
	if event.Error == nil {
		t.Fatal("expected Error to be set")
	}
	if event.Error.Message != "Planning failed" {
		t.Errorf("expected message='Planning failed', got %s", event.Error.Message)
	}
}
