package streaming

import (
	"bytes"
	"strings"
	"testing"
	"time"
)

func TestTextRenderer_RenderEvent(t *testing.T) {
	tests := []struct {
		name     string
		event    StreamEvent
		verbose  bool
		contains []string
		excludes []string
	}{
		{
			name: "connected event",
			event: StreamEvent{
				Type:        EventTypeConnected,
				Timestamp:   time.Now(),
				ExecutionID: "abc123def456",
			},
			contains: []string{"[CONNECTED]", "abc123def456"},
		},
		{
			name: "tool_started event",
			event: StreamEvent{
				Type:      EventTypeToolStarted,
				Timestamp: time.Now(),
				Tool: &ToolEventData{
					Name:   "shell",
					Inputs: map[string]interface{}{"command": "ls -la"},
				},
			},
			verbose:  false,
			contains: []string{"[TOOL]", "shell", "started"},
			excludes: []string{"Inputs:", "ls -la"}, // Inputs hidden in non-verbose
		},
		{
			name: "tool_started event verbose",
			event: StreamEvent{
				Type:      EventTypeToolStarted,
				Timestamp: time.Now(),
				Tool: &ToolEventData{
					Name:   "shell",
					Inputs: map[string]interface{}{"command": "ls -la"},
				},
			},
			verbose:  true,
			contains: []string{"[TOOL]", "shell", "started", "Inputs:", "command:", "ls -la"},
		},
		{
			name: "tool_completed success",
			event: StreamEvent{
				Type:      EventTypeToolCompleted,
				Timestamp: time.Now(),
				Tool: &ToolEventData{
					Name:            "shell",
					DurationSeconds: 1.5,
					Success:         true,
					Outputs:         map[string]interface{}{"stdout": "file1.txt"},
				},
			},
			verbose:  false,
			contains: []string{"[TOOL]", "shell", "done", "1.50s"},
			excludes: []string{"Outputs:", "file1.txt"},
		},
		{
			name: "tool_completed failure",
			event: StreamEvent{
				Type:      EventTypeToolCompleted,
				Timestamp: time.Now(),
				Tool: &ToolEventData{
					Name:            "shell",
					DurationSeconds: 0.5,
					Success:         false,
					Error:           "command not found",
				},
			},
			contains: []string{"[TOOL]", "shell", "failed", "Error", "command not found"},
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
			contains: []string{"I am thinking..."},
		},
		{
			name: "message event",
			event: StreamEvent{
				Type:      EventTypeMessage,
				Timestamp: time.Now(),
				Message: &MessageEventData{
					Role:    "assistant",
					Content: "Here is my response",
				},
			},
			contains: []string{"[THINKING]", "Here is my response"},
		},
		{
			name: "status event running",
			event: StreamEvent{
				Type:      EventTypeStatus,
				Timestamp: time.Now(),
				Status: &StatusEventData{
					State:  "running",
					Reason: "Execution started",
				},
			},
			contains: []string{"[STATUS]", "running", "Execution started"},
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
			contains: []string{"[PROGRESS]", "Planning", "Analyzing task", "50%"},
		},
		{
			name: "error event",
			event: StreamEvent{
				Type:      EventTypeError,
				Timestamp: time.Now(),
				Error: &ErrorEventData{
					Message: "Connection failed",
					Code:    "CONN_ERR",
				},
			},
			contains: []string{"[ERROR]", "Connection failed", "CONN_ERR"},
		},
		{
			name: "done event",
			event: StreamEvent{
				Type:      EventTypeDone,
				Timestamp: time.Now(),
			},
			contains: []string{"[DONE]", "Execution completed"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var buf bytes.Buffer
			renderer := NewTextRenderer(&buf, tt.verbose)

			err := renderer.RenderEvent(tt.event)
			if err != nil {
				t.Fatalf("RenderEvent failed: %v", err)
			}

			output := buf.String()

			for _, s := range tt.contains {
				if !strings.Contains(output, s) {
					t.Errorf("expected output to contain %q, got: %s", s, output)
				}
			}

			for _, s := range tt.excludes {
				if strings.Contains(output, s) {
					t.Errorf("expected output NOT to contain %q, got: %s", s, output)
				}
			}
		})
	}
}

func TestTextRenderer_ElapsedTime(t *testing.T) {
	var buf bytes.Buffer
	renderer := NewTextRenderer(&buf, false)

	// Render an event
	event := NewDoneEvent()
	err := renderer.RenderEvent(event)
	if err != nil {
		t.Fatalf("RenderEvent failed: %v", err)
	}

	output := buf.String()

	// Check for elapsed time format [  X.Xs]
	if !strings.Contains(output, "[") || !strings.Contains(output, "s]") {
		t.Errorf("expected elapsed time format, got: %s", output)
	}
}

func TestTextRenderer_ThreadSafety(t *testing.T) {
	var buf bytes.Buffer
	renderer := NewTextRenderer(&buf, false)

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

	// Verify we got output
	output := buf.String()
	if output == "" {
		t.Error("expected some output from concurrent renders")
	}
}

func TestTextRenderer_FlushAndClose(t *testing.T) {
	var buf bytes.Buffer
	renderer := NewTextRenderer(&buf, false)

	err := renderer.Flush()
	if err != nil {
		t.Errorf("Flush failed: %v", err)
	}

	err = renderer.Close()
	if err != nil {
		t.Errorf("Close failed: %v", err)
	}
}
