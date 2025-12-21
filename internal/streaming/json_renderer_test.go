package streaming

import (
	"bufio"
	"bytes"
	"encoding/json"
	"strings"
	"testing"
	"time"
)

func TestJSONRenderer_RenderEvent(t *testing.T) {
	tests := []struct {
		name    string
		event   StreamEvent
		verbose bool
		check   func(t *testing.T, data map[string]interface{})
	}{
		{
			name: "connected event",
			event: StreamEvent{
				Type:        EventTypeConnected,
				Timestamp:   time.Date(2024, 1, 15, 10, 30, 0, 0, time.UTC),
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
			name: "tool_started event non-verbose strips inputs",
			event: StreamEvent{
				Type:      EventTypeToolStarted,
				Timestamp: time.Now(),
				Tool: &ToolEventData{
					Name:   "shell",
					Inputs: map[string]interface{}{"command": "ls -la"},
				},
			},
			verbose: false,
			check: func(t *testing.T, data map[string]interface{}) {
				if data["type"] != "tool_started" {
					t.Errorf("expected type=tool_started, got %v", data["type"])
				}
				tool := data["tool"].(map[string]interface{})
				if tool["name"] != "shell" {
					t.Errorf("expected tool.name=shell, got %v", tool["name"])
				}
				// Inputs should be stripped in non-verbose mode
				if _, exists := tool["inputs"]; exists {
					t.Error("expected inputs to be stripped in non-verbose mode")
				}
			},
		},
		{
			name: "tool_started event verbose keeps inputs",
			event: StreamEvent{
				Type:      EventTypeToolStarted,
				Timestamp: time.Now(),
				Tool: &ToolEventData{
					Name:   "shell",
					Inputs: map[string]interface{}{"command": "ls -la"},
				},
			},
			verbose: true,
			check: func(t *testing.T, data map[string]interface{}) {
				tool := data["tool"].(map[string]interface{})
				inputs := tool["inputs"].(map[string]interface{})
				if inputs["command"] != "ls -la" {
					t.Errorf("expected inputs.command='ls -la', got %v", inputs["command"])
				}
			},
		},
		{
			name: "tool_completed event non-verbose strips outputs",
			event: StreamEvent{
				Type:      EventTypeToolCompleted,
				Timestamp: time.Now(),
				Tool: &ToolEventData{
					Name:            "shell",
					Outputs:         map[string]interface{}{"stdout": "file1.txt\nfile2.txt"},
					DurationSeconds: 1.5,
					Success:         true,
				},
			},
			verbose: false,
			check: func(t *testing.T, data map[string]interface{}) {
				tool := data["tool"].(map[string]interface{})
				if tool["duration_seconds"] != 1.5 {
					t.Errorf("expected duration_seconds=1.5, got %v", tool["duration_seconds"])
				}
				if tool["success"] != true {
					t.Error("expected success=true")
				}
				// Outputs should be stripped in non-verbose mode
				if _, exists := tool["outputs"]; exists {
					t.Error("expected outputs to be stripped in non-verbose mode")
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
					Content: "I am thinking...",
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
				if msg["content"] != "I am thinking..." {
					t.Errorf("expected message.content='I am thinking...', got %v", msg["content"])
				}
				if msg["chunk"] != true {
					t.Error("expected message.chunk=true")
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
					Stage:   "Planning",
					Message: "Analyzing task",
					Percent: 50,
				},
			},
			check: func(t *testing.T, data map[string]interface{}) {
				if data["type"] != "progress" {
					t.Errorf("expected type=progress, got %v", data["type"])
				}
				progress := data["progress"].(map[string]interface{})
				if progress["percent"] != float64(50) {
					t.Errorf("expected progress.percent=50, got %v", progress["percent"])
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
				if errData["code"] != "CONN_ERR" {
					t.Errorf("expected error.code='CONN_ERR', got %v", errData["code"])
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
			var buf bytes.Buffer
			renderer := NewJSONRenderer(&buf, tt.verbose)

			err := renderer.RenderEvent(tt.event)
			if err != nil {
				t.Fatalf("RenderEvent failed: %v", err)
			}

			output := buf.String()

			// Verify it's valid JSON
			var data map[string]interface{}
			if err := json.Unmarshal([]byte(strings.TrimSpace(output)), &data); err != nil {
				t.Fatalf("output is not valid JSON: %v\nOutput: %s", err, output)
			}

			tt.check(t, data)
		})
	}
}

func TestJSONRenderer_NDJSON(t *testing.T) {
	var buf bytes.Buffer
	renderer := NewJSONRenderer(&buf, false)

	// Render multiple events
	events := []StreamEvent{
		NewConnectedEvent("exec-1"),
		NewToolStartedEvent("tool1", nil),
		NewToolCompletedEvent("tool1", nil, 1.0, true, ""),
		NewDoneEvent(),
	}

	for _, event := range events {
		err := renderer.RenderEvent(event)
		if err != nil {
			t.Fatalf("RenderEvent failed: %v", err)
		}
	}

	// Verify each line is valid JSON
	scanner := bufio.NewScanner(&buf)
	lineCount := 0
	for scanner.Scan() {
		line := scanner.Text()
		var data map[string]interface{}
		if err := json.Unmarshal([]byte(line), &data); err != nil {
			t.Errorf("line %d is not valid JSON: %v\nLine: %s", lineCount+1, err, line)
		}
		lineCount++
	}

	if lineCount != len(events) {
		t.Errorf("expected %d lines, got %d", len(events), lineCount)
	}
}

func TestJSONRenderer_ThreadSafety(t *testing.T) {
	var buf bytes.Buffer
	renderer := NewJSONRenderer(&buf, false)

	// Render multiple events concurrently
	done := make(chan bool)
	for i := 0; i < 10; i++ {
		go func(id int) {
			event := NewToolStartedEvent("tool", map[string]interface{}{"id": id})
			_ = renderer.RenderEvent(event)
			done <- true
		}(i)
	}

	// Wait for all goroutines
	for i := 0; i < 10; i++ {
		<-done
	}

	// Verify we got 10 valid JSON lines
	scanner := bufio.NewScanner(&buf)
	lineCount := 0
	for scanner.Scan() {
		line := scanner.Text()
		var data map[string]interface{}
		if err := json.Unmarshal([]byte(line), &data); err != nil {
			t.Errorf("line is not valid JSON: %v", err)
		}
		lineCount++
	}

	if lineCount != 10 {
		t.Errorf("expected 10 lines, got %d", lineCount)
	}
}

func TestJSONRenderer_FlushAndClose(t *testing.T) {
	var buf bytes.Buffer
	renderer := NewJSONRenderer(&buf, false)

	err := renderer.Flush()
	if err != nil {
		t.Errorf("Flush failed: %v", err)
	}

	err = renderer.Close()
	if err != nil {
		t.Errorf("Close failed: %v", err)
	}
}

func TestJSONRenderer_TimestampFormat(t *testing.T) {
	var buf bytes.Buffer
	renderer := NewJSONRenderer(&buf, false)

	event := StreamEvent{
		Type:      EventTypeDone,
		Timestamp: time.Date(2024, 1, 15, 10, 30, 45, 123456789, time.UTC),
	}

	err := renderer.RenderEvent(event)
	if err != nil {
		t.Fatalf("RenderEvent failed: %v", err)
	}

	output := buf.String()
	var data map[string]interface{}
	json.Unmarshal([]byte(output), &data)

	// Timestamp should be in RFC3339 format
	timestamp := data["timestamp"].(string)
	if !strings.HasPrefix(timestamp, "2024-01-15T10:30:45") {
		t.Errorf("unexpected timestamp format: %s", timestamp)
	}
}
