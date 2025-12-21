// Package streaming provides streaming event rendering for the kubiya exec command.
// It supports multiple output formats (text, JSON, TUI) and environments (TTY, CI, pipes).
package streaming

import (
	"io"
	"time"
)

// StreamEventType represents the type of streaming event
type StreamEventType string

const (
	// EventTypeToolStarted indicates a tool execution has begun
	EventTypeToolStarted StreamEventType = "tool_started"
	// EventTypeToolCompleted indicates a tool execution has finished
	EventTypeToolCompleted StreamEventType = "tool_completed"
	// EventTypeMessage indicates a complete message
	EventTypeMessage StreamEventType = "message"
	// EventTypeMessageChunk indicates a streaming token/chunk
	EventTypeMessageChunk StreamEventType = "message_chunk"
	// EventTypeStatus indicates a status change
	EventTypeStatus StreamEventType = "status"
	// EventTypeProgress indicates planning progress
	EventTypeProgress StreamEventType = "progress"
	// EventTypeError indicates an error occurred
	EventTypeError StreamEventType = "error"
	// EventTypeConnected indicates the stream is connected
	EventTypeConnected StreamEventType = "connected"
	// EventTypeDone indicates the execution is complete
	EventTypeDone StreamEventType = "done"
)

// StreamEvent represents a unified streaming event from execution or planning
type StreamEvent struct {
	// Type is the event type
	Type StreamEventType `json:"type"`

	// Timestamp when the event occurred
	Timestamp time.Time `json:"timestamp"`

	// ExecutionID is the ID of the execution (if applicable)
	ExecutionID string `json:"execution_id,omitempty"`

	// Tool contains tool-related event data
	Tool *ToolEventData `json:"tool,omitempty"`

	// Message contains message-related event data
	Message *MessageEventData `json:"message,omitempty"`

	// Status contains status-related event data
	Status *StatusEventData `json:"status,omitempty"`

	// Progress contains progress-related event data
	Progress *ProgressEventData `json:"progress,omitempty"`

	// Error contains error-related event data
	Error *ErrorEventData `json:"error,omitempty"`

	// Metadata contains additional unstructured data
	Metadata map[string]interface{} `json:"metadata,omitempty"`
}

// ToolEventData contains data for tool-related events
type ToolEventData struct {
	// Name is the tool name
	Name string `json:"name"`

	// Inputs are the tool input parameters (omitted in non-verbose mode)
	Inputs map[string]interface{} `json:"inputs,omitempty"`

	// Outputs are the tool output results (omitted in non-verbose mode)
	Outputs map[string]interface{} `json:"outputs,omitempty"`

	// DurationSeconds is the execution duration in seconds
	DurationSeconds float64 `json:"duration_seconds,omitempty"`

	// Success indicates if the tool execution succeeded
	Success bool `json:"success"`

	// Error contains error message if the tool failed
	Error string `json:"error,omitempty"`
}

// MessageEventData contains data for message-related events
type MessageEventData struct {
	// Role is the message role (assistant, user, system, tool)
	Role string `json:"role"`

	// Content is the message content
	Content string `json:"content"`

	// Chunk indicates if this is a streaming chunk (vs complete message)
	Chunk bool `json:"chunk,omitempty"`
}

// StatusEventData contains data for status-related events
type StatusEventData struct {
	// State is the current status state
	State string `json:"state"`

	// PreviousState is the previous status state (if changed)
	PreviousState string `json:"previous_state,omitempty"`

	// Reason for the status change
	Reason string `json:"reason,omitempty"`
}

// ProgressEventData contains data for progress-related events
type ProgressEventData struct {
	// Stage is the current planning/execution stage
	Stage string `json:"stage"`

	// Message describes what's happening
	Message string `json:"message"`

	// Percent is the completion percentage (0-100)
	Percent int `json:"percent"`
}

// ErrorEventData contains data for error-related events
type ErrorEventData struct {
	// Message is the error message
	Message string `json:"message"`

	// Code is the error code (if applicable)
	Code string `json:"code,omitempty"`

	// Recoverable indicates if the error is recoverable
	Recoverable bool `json:"recoverable,omitempty"`
}

// StreamRenderer defines the interface for rendering stream events
type StreamRenderer interface {
	// RenderEvent renders a single streaming event
	RenderEvent(event StreamEvent) error

	// Flush ensures all buffered output is written
	Flush() error

	// Close cleans up resources and performs final writes
	Close() error
}

// StreamFormat represents the output format for streaming
type StreamFormat string

const (
	// StreamFormatAuto automatically selects format based on environment
	StreamFormatAuto StreamFormat = "auto"
	// StreamFormatText outputs formatted text with prefixes
	StreamFormatText StreamFormat = "text"
	// StreamFormatJSON outputs newline-delimited JSON
	StreamFormatJSON StreamFormat = "json"
)

// StreamOptions configures streaming behavior
type StreamOptions struct {
	// Enabled indicates if streaming is enabled
	Enabled bool

	// Format is the output format
	Format StreamFormat

	// Verbose enables detailed tool inputs/outputs
	Verbose bool

	// EventsWriter is where streaming events are written (default: stderr)
	EventsWriter io.Writer

	// ResultWriter is where the final result is written (default: stdout)
	ResultWriter io.Writer
}

// NewStreamEvent creates a new StreamEvent with the current timestamp
func NewStreamEvent(eventType StreamEventType) StreamEvent {
	return StreamEvent{
		Type:      eventType,
		Timestamp: time.Now(),
	}
}

// NewToolStartedEvent creates a tool_started event
func NewToolStartedEvent(toolName string, inputs map[string]interface{}) StreamEvent {
	event := NewStreamEvent(EventTypeToolStarted)
	event.Tool = &ToolEventData{
		Name:   toolName,
		Inputs: inputs,
	}
	return event
}

// NewToolCompletedEvent creates a tool_completed event
func NewToolCompletedEvent(toolName string, outputs map[string]interface{}, duration float64, success bool, errMsg string) StreamEvent {
	event := NewStreamEvent(EventTypeToolCompleted)
	event.Tool = &ToolEventData{
		Name:            toolName,
		Outputs:         outputs,
		DurationSeconds: duration,
		Success:         success,
		Error:           errMsg,
	}
	return event
}

// NewMessageChunkEvent creates a message_chunk event
func NewMessageChunkEvent(role, content string) StreamEvent {
	event := NewStreamEvent(EventTypeMessageChunk)
	event.Message = &MessageEventData{
		Role:    role,
		Content: content,
		Chunk:   true,
	}
	return event
}

// NewStatusEvent creates a status event
func NewStatusEvent(state, previousState, reason string) StreamEvent {
	event := NewStreamEvent(EventTypeStatus)
	event.Status = &StatusEventData{
		State:         state,
		PreviousState: previousState,
		Reason:        reason,
	}
	return event
}

// NewProgressEvent creates a progress event
func NewProgressEvent(stage, message string, percent int) StreamEvent {
	event := NewStreamEvent(EventTypeProgress)
	event.Progress = &ProgressEventData{
		Stage:   stage,
		Message: message,
		Percent: percent,
	}
	return event
}

// NewErrorEvent creates an error event
func NewErrorEvent(message, code string, recoverable bool) StreamEvent {
	event := NewStreamEvent(EventTypeError)
	event.Error = &ErrorEventData{
		Message:     message,
		Code:        code,
		Recoverable: recoverable,
	}
	return event
}

// NewConnectedEvent creates a connected event
func NewConnectedEvent(executionID string) StreamEvent {
	event := NewStreamEvent(EventTypeConnected)
	event.ExecutionID = executionID
	return event
}

// NewDoneEvent creates a done event
func NewDoneEvent() StreamEvent {
	return NewStreamEvent(EventTypeDone)
}
