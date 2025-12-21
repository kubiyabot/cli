package streaming

import (
	"encoding/json"
	"testing"
	"time"
)

func TestStreamEventMarshalJSON(t *testing.T) {
	tests := []struct {
		name  string
		event StreamEvent
		check func(t *testing.T, data map[string]interface{})
	}{
		{
			name: "tool_started event",
			event: StreamEvent{
				Type:      EventTypeToolStarted,
				Timestamp: time.Date(2024, 1, 15, 10, 30, 0, 0, time.UTC),
				Tool: &ToolEventData{
					Name:   "shell",
					Inputs: map[string]interface{}{"command": "kubectl get pods"},
				},
			},
			check: func(t *testing.T, data map[string]interface{}) {
				if data["type"] != "tool_started" {
					t.Errorf("expected type=tool_started, got %v", data["type"])
				}
				tool := data["tool"].(map[string]interface{})
				if tool["name"] != "shell" {
					t.Errorf("expected tool.name=shell, got %v", tool["name"])
				}
				inputs := tool["inputs"].(map[string]interface{})
				if inputs["command"] != "kubectl get pods" {
					t.Errorf("expected tool.inputs.command='kubectl get pods', got %v", inputs["command"])
				}
			},
		},
		{
			name: "tool_completed event",
			event: StreamEvent{
				Type:      EventTypeToolCompleted,
				Timestamp: time.Date(2024, 1, 15, 10, 30, 5, 0, time.UTC),
				Tool: &ToolEventData{
					Name:            "shell",
					Outputs:         map[string]interface{}{"stdout": "pod1 Running"},
					DurationSeconds: 1.5,
					Success:         true,
				},
			},
			check: func(t *testing.T, data map[string]interface{}) {
				if data["type"] != "tool_completed" {
					t.Errorf("expected type=tool_completed, got %v", data["type"])
				}
				tool := data["tool"].(map[string]interface{})
				if tool["duration_seconds"] != 1.5 {
					t.Errorf("expected duration_seconds=1.5, got %v", tool["duration_seconds"])
				}
				if tool["success"] != true {
					t.Errorf("expected success=true, got %v", tool["success"])
				}
			},
		},
		{
			name: "message_chunk event",
			event: StreamEvent{
				Type:      EventTypeMessageChunk,
				Timestamp: time.Now(),
				Message: &MessageEventData{
					Role:    "assistant",
					Content: "I'm checking the pod status...",
					Chunk:   true,
				},
			},
			check: func(t *testing.T, data map[string]interface{}) {
				if data["type"] != "message_chunk" {
					t.Errorf("expected type=message_chunk, got %v", data["type"])
				}
				msg := data["message"].(map[string]interface{})
				if msg["role"] != "assistant" {
					t.Errorf("expected message.role=assistant, got %v", msg["role"])
				}
				if msg["chunk"] != true {
					t.Errorf("expected message.chunk=true, got %v", msg["chunk"])
				}
			},
		},
		{
			name: "status event",
			event: StreamEvent{
				Type:      EventTypeStatus,
				Timestamp: time.Now(),
				Status: &StatusEventData{
					State:         "running",
					PreviousState: "pending",
					Reason:        "Execution started",
				},
			},
			check: func(t *testing.T, data map[string]interface{}) {
				if data["type"] != "status" {
					t.Errorf("expected type=status, got %v", data["type"])
				}
				status := data["status"].(map[string]interface{})
				if status["state"] != "running" {
					t.Errorf("expected status.state=running, got %v", status["state"])
				}
			},
		},
		{
			name: "progress event",
			event: StreamEvent{
				Type:      EventTypeProgress,
				Timestamp: time.Now(),
				Progress: &ProgressEventData{
					Stage:   "Analyzing Task",
					Message: "Understanding requirements",
					Percent: 25,
				},
			},
			check: func(t *testing.T, data map[string]interface{}) {
				if data["type"] != "progress" {
					t.Errorf("expected type=progress, got %v", data["type"])
				}
				progress := data["progress"].(map[string]interface{})
				if progress["percent"] != float64(25) {
					t.Errorf("expected progress.percent=25, got %v", progress["percent"])
				}
			},
		},
		{
			name: "error event",
			event: StreamEvent{
				Type:      EventTypeError,
				Timestamp: time.Now(),
				Error: &ErrorEventData{
					Message:     "Connection failed",
					Code:        "CONN_ERR",
					Recoverable: true,
				},
			},
			check: func(t *testing.T, data map[string]interface{}) {
				if data["type"] != "error" {
					t.Errorf("expected type=error, got %v", data["type"])
				}
				errData := data["error"].(map[string]interface{})
				if errData["message"] != "Connection failed" {
					t.Errorf("expected error.message='Connection failed', got %v", errData["message"])
				}
				if errData["recoverable"] != true {
					t.Errorf("expected error.recoverable=true, got %v", errData["recoverable"])
				}
			},
		},
		{
			name: "connected event",
			event: StreamEvent{
				Type:        EventTypeConnected,
				Timestamp:   time.Now(),
				ExecutionID: "abc123",
			},
			check: func(t *testing.T, data map[string]interface{}) {
				if data["type"] != "connected" {
					t.Errorf("expected type=connected, got %v", data["type"])
				}
				if data["execution_id"] != "abc123" {
					t.Errorf("expected execution_id=abc123, got %v", data["execution_id"])
				}
			},
		},
		{
			name: "done event",
			event: StreamEvent{
				Type:      EventTypeDone,
				Timestamp: time.Now(),
			},
			check: func(t *testing.T, data map[string]interface{}) {
				if data["type"] != "done" {
					t.Errorf("expected type=done, got %v", data["type"])
				}
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			jsonBytes, err := json.Marshal(tt.event)
			if err != nil {
				t.Fatalf("failed to marshal event: %v", err)
			}

			var data map[string]interface{}
			if err := json.Unmarshal(jsonBytes, &data); err != nil {
				t.Fatalf("failed to unmarshal event: %v", err)
			}

			tt.check(t, data)
		})
	}
}

func TestStreamEventUnmarshalJSON(t *testing.T) {
	jsonStr := `{
		"type": "tool_completed",
		"timestamp": "2024-01-15T10:30:05Z",
		"tool": {
			"name": "shell",
			"outputs": {"stdout": "pod1 Running"},
			"duration_seconds": 1.5,
			"success": true
		}
	}`

	var event StreamEvent
	if err := json.Unmarshal([]byte(jsonStr), &event); err != nil {
		t.Fatalf("failed to unmarshal event: %v", err)
	}

	if event.Type != EventTypeToolCompleted {
		t.Errorf("expected type=tool_completed, got %v", event.Type)
	}
	if event.Tool == nil {
		t.Fatal("expected tool data, got nil")
	}
	if event.Tool.Name != "shell" {
		t.Errorf("expected tool.name=shell, got %v", event.Tool.Name)
	}
	if event.Tool.DurationSeconds != 1.5 {
		t.Errorf("expected tool.duration_seconds=1.5, got %v", event.Tool.DurationSeconds)
	}
	if !event.Tool.Success {
		t.Error("expected tool.success=true")
	}
}

func TestEventTypeConstants(t *testing.T) {
	// Verify all event types are defined correctly
	expectedTypes := map[StreamEventType]string{
		EventTypeToolStarted:   "tool_started",
		EventTypeToolCompleted: "tool_completed",
		EventTypeMessage:       "message",
		EventTypeMessageChunk:  "message_chunk",
		EventTypeStatus:        "status",
		EventTypeProgress:      "progress",
		EventTypeError:         "error",
		EventTypeConnected:     "connected",
		EventTypeDone:          "done",
	}

	for eventType, expected := range expectedTypes {
		if string(eventType) != expected {
			t.Errorf("expected %s, got %s", expected, string(eventType))
		}
	}
}

func TestStreamFormatConstants(t *testing.T) {
	if StreamFormatAuto != "auto" {
		t.Errorf("expected StreamFormatAuto=auto, got %s", StreamFormatAuto)
	}
	if StreamFormatText != "text" {
		t.Errorf("expected StreamFormatText=text, got %s", StreamFormatText)
	}
	if StreamFormatJSON != "json" {
		t.Errorf("expected StreamFormatJSON=json, got %s", StreamFormatJSON)
	}
}

func TestNewStreamEventHelpers(t *testing.T) {
	t.Run("NewToolStartedEvent", func(t *testing.T) {
		event := NewToolStartedEvent("shell", map[string]interface{}{"cmd": "ls"})
		if event.Type != EventTypeToolStarted {
			t.Errorf("expected type=tool_started, got %v", event.Type)
		}
		if event.Tool == nil || event.Tool.Name != "shell" {
			t.Error("expected tool data with name=shell")
		}
		if event.Timestamp.IsZero() {
			t.Error("expected timestamp to be set")
		}
	})

	t.Run("NewToolCompletedEvent", func(t *testing.T) {
		event := NewToolCompletedEvent("shell", map[string]interface{}{"result": "ok"}, 1.5, true, "")
		if event.Type != EventTypeToolCompleted {
			t.Errorf("expected type=tool_completed, got %v", event.Type)
		}
		if event.Tool == nil || event.Tool.DurationSeconds != 1.5 {
			t.Error("expected tool data with duration=1.5")
		}
		if !event.Tool.Success {
			t.Error("expected success=true")
		}
	})

	t.Run("NewMessageChunkEvent", func(t *testing.T) {
		event := NewMessageChunkEvent("assistant", "Hello")
		if event.Type != EventTypeMessageChunk {
			t.Errorf("expected type=message_chunk, got %v", event.Type)
		}
		if event.Message == nil || event.Message.Content != "Hello" {
			t.Error("expected message data with content=Hello")
		}
		if !event.Message.Chunk {
			t.Error("expected chunk=true")
		}
	})

	t.Run("NewStatusEvent", func(t *testing.T) {
		event := NewStatusEvent("running", "pending", "Started")
		if event.Type != EventTypeStatus {
			t.Errorf("expected type=status, got %v", event.Type)
		}
		if event.Status == nil || event.Status.State != "running" {
			t.Error("expected status data with state=running")
		}
	})

	t.Run("NewProgressEvent", func(t *testing.T) {
		event := NewProgressEvent("Planning", "Analyzing", 50)
		if event.Type != EventTypeProgress {
			t.Errorf("expected type=progress, got %v", event.Type)
		}
		if event.Progress == nil || event.Progress.Percent != 50 {
			t.Error("expected progress data with percent=50")
		}
	})

	t.Run("NewErrorEvent", func(t *testing.T) {
		event := NewErrorEvent("Failed", "ERR_01", true)
		if event.Type != EventTypeError {
			t.Errorf("expected type=error, got %v", event.Type)
		}
		if event.Error == nil || event.Error.Message != "Failed" {
			t.Error("expected error data with message=Failed")
		}
	})

	t.Run("NewConnectedEvent", func(t *testing.T) {
		event := NewConnectedEvent("exec-123")
		if event.Type != EventTypeConnected {
			t.Errorf("expected type=connected, got %v", event.Type)
		}
		if event.ExecutionID != "exec-123" {
			t.Errorf("expected execution_id=exec-123, got %v", event.ExecutionID)
		}
	})

	t.Run("NewDoneEvent", func(t *testing.T) {
		event := NewDoneEvent()
		if event.Type != EventTypeDone {
			t.Errorf("expected type=done, got %v", event.Type)
		}
	})
}

// MockRenderer is a test implementation of StreamRenderer
type MockRenderer struct {
	Events  []StreamEvent
	Flushed bool
	Closed  bool
}

func (m *MockRenderer) RenderEvent(event StreamEvent) error {
	m.Events = append(m.Events, event)
	return nil
}

func (m *MockRenderer) Flush() error {
	m.Flushed = true
	return nil
}

func (m *MockRenderer) Close() error {
	m.Closed = true
	return nil
}

func TestMockRendererImplementsInterface(t *testing.T) {
	var _ StreamRenderer = &MockRenderer{}

	mock := &MockRenderer{}

	// Test RenderEvent
	event := NewToolStartedEvent("test", nil)
	if err := mock.RenderEvent(event); err != nil {
		t.Errorf("RenderEvent failed: %v", err)
	}
	if len(mock.Events) != 1 {
		t.Errorf("expected 1 event, got %d", len(mock.Events))
	}

	// Test Flush
	if err := mock.Flush(); err != nil {
		t.Errorf("Flush failed: %v", err)
	}
	if !mock.Flushed {
		t.Error("expected Flushed=true")
	}

	// Test Close
	if err := mock.Close(); err != nil {
		t.Errorf("Close failed: %v", err)
	}
	if !mock.Closed {
		t.Error("expected Closed=true")
	}
}
