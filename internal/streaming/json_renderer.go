package streaming

import (
	"encoding/json"
	"io"
	"sync"
)

// JSONRenderer renders streaming events as newline-delimited JSON (NDJSON)
type JSONRenderer struct {
	out     io.Writer
	verbose bool
	mu      sync.Mutex
}

// NewJSONRenderer creates a new JSONRenderer
func NewJSONRenderer(out io.Writer, verbose bool) *JSONRenderer {
	return &JSONRenderer{
		out:     out,
		verbose: verbose,
	}
}

// RenderEvent renders a single streaming event as a JSON line
func (r *JSONRenderer) RenderEvent(event StreamEvent) error {
	r.mu.Lock()
	defer r.mu.Unlock()

	// Apply verbosity filter - strip inputs/outputs if not verbose
	if !r.verbose && event.Tool != nil {
		event = r.filterVerbose(event)
	}

	// Marshal to JSON
	data, err := json.Marshal(event)
	if err != nil {
		return err
	}

	// Write as single line with newline
	_, err = r.out.Write(append(data, '\n'))
	return err
}

// Flush ensures all buffered output is written
func (r *JSONRenderer) Flush() error {
	r.mu.Lock()
	defer r.mu.Unlock()

	// If writer implements Flusher, flush it
	if flusher, ok := r.out.(interface{ Flush() error }); ok {
		return flusher.Flush()
	}
	return nil
}

// Close cleans up resources
func (r *JSONRenderer) Close() error {
	return r.Flush()
}

// filterVerbose creates a copy of the event with tool inputs/outputs stripped
func (r *JSONRenderer) filterVerbose(event StreamEvent) StreamEvent {
	if event.Tool == nil {
		return event
	}

	// Create a copy with filtered tool data
	filteredTool := &ToolEventData{
		Name:            event.Tool.Name,
		DurationSeconds: event.Tool.DurationSeconds,
		Success:         event.Tool.Success,
		Error:           event.Tool.Error,
		// Inputs and Outputs are intentionally omitted
	}

	// Return a copy of the event with filtered tool data
	return StreamEvent{
		Type:        event.Type,
		Timestamp:   event.Timestamp,
		ExecutionID: event.ExecutionID,
		Tool:        filteredTool,
		Message:     event.Message,
		Status:      event.Status,
		Progress:    event.Progress,
		Error:       event.Error,
		Metadata:    event.Metadata,
	}
}

// JSONOutputEvent is an alternative simplified event structure for NDJSON output
// This can be used if you want a flatter structure than StreamEvent
type JSONOutputEvent struct {
	Type        string                 `json:"type"`
	Timestamp   string                 `json:"timestamp"`
	ExecutionID string                 `json:"execution_id,omitempty"`
	Tool        *JSONToolEvent         `json:"tool,omitempty"`
	Message     *JSONMessageEvent      `json:"message,omitempty"`
	Status      string                 `json:"status,omitempty"`
	Progress    *JSONProgressEvent     `json:"progress,omitempty"`
	Error       string                 `json:"error,omitempty"`
	Data        map[string]interface{} `json:"data,omitempty"`
}

// JSONToolEvent is the tool event structure for NDJSON output
type JSONToolEvent struct {
	Name            string                 `json:"name"`
	Inputs          map[string]interface{} `json:"inputs,omitempty"`
	Outputs         map[string]interface{} `json:"outputs,omitempty"`
	DurationSeconds float64                `json:"duration_seconds,omitempty"`
	Success         bool                   `json:"success"`
	Error           string                 `json:"error,omitempty"`
}

// JSONMessageEvent is the message event structure for NDJSON output
type JSONMessageEvent struct {
	Role    string `json:"role"`
	Content string `json:"content"`
	Chunk   bool   `json:"chunk,omitempty"`
}

// JSONProgressEvent is the progress event structure for NDJSON output
type JSONProgressEvent struct {
	Stage   string `json:"stage"`
	Message string `json:"message"`
	Percent int    `json:"percent"`
}
