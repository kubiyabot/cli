package streaming

import (
	"sync"
	"time"
)

// EventFilter processes an event and determines whether it should be passed through
type EventFilter interface {
	// Filter processes an event and returns the (possibly modified) event
	// and a boolean indicating whether the event should be passed through (true) or skipped (false)
	Filter(event StreamEvent) (StreamEvent, bool)
}

// EventPipeline processes streaming events through a chain of filters before rendering
type EventPipeline struct {
	renderer StreamRenderer
	filters  []EventFilter
	mu       sync.Mutex
}

// NewEventPipeline creates a new EventPipeline with the given renderer
func NewEventPipeline(renderer StreamRenderer) *EventPipeline {
	return &EventPipeline{
		renderer: renderer,
		filters:  make([]EventFilter, 0),
	}
}

// AddFilter adds a filter to the pipeline
func (p *EventPipeline) AddFilter(filter EventFilter) *EventPipeline {
	p.mu.Lock()
	defer p.mu.Unlock()
	p.filters = append(p.filters, filter)
	return p
}

// Process applies all filters to the event and renders it if not filtered out
func (p *EventPipeline) Process(event StreamEvent) error {
	p.mu.Lock()
	filters := make([]EventFilter, len(p.filters))
	copy(filters, p.filters)
	p.mu.Unlock()

	// Apply filters in order
	for _, filter := range filters {
		var shouldPass bool
		event, shouldPass = filter.Filter(event)
		if !shouldPass {
			return nil // Event filtered out
		}
	}

	// Render the event
	return p.renderer.RenderEvent(event)
}

// Flush flushes the underlying renderer
func (p *EventPipeline) Flush() error {
	return p.renderer.Flush()
}

// Close closes the underlying renderer
func (p *EventPipeline) Close() error {
	return p.renderer.Close()
}

// VerbosityFilter strips tool inputs/outputs when verbose is false
type VerbosityFilter struct {
	verbose bool
}

// NewVerbosityFilter creates a new VerbosityFilter
func NewVerbosityFilter(verbose bool) *VerbosityFilter {
	return &VerbosityFilter{verbose: verbose}
}

// Filter implements EventFilter
func (f *VerbosityFilter) Filter(event StreamEvent) (StreamEvent, bool) {
	if f.verbose || event.Tool == nil {
		return event, true
	}

	// Create a copy with stripped tool data
	filteredTool := &ToolEventData{
		Name:            event.Tool.Name,
		DurationSeconds: event.Tool.DurationSeconds,
		Success:         event.Tool.Success,
		Error:           event.Tool.Error,
		// Inputs and Outputs are intentionally omitted
	}

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
	}, true
}

// DeduplicationFilter skips duplicate consecutive events
type DeduplicationFilter struct {
	lastEventKey string
	mu           sync.Mutex
}

// NewDeduplicationFilter creates a new DeduplicationFilter
func NewDeduplicationFilter() *DeduplicationFilter {
	return &DeduplicationFilter{}
}

// Filter implements EventFilter
func (f *DeduplicationFilter) Filter(event StreamEvent) (StreamEvent, bool) {
	f.mu.Lock()
	defer f.mu.Unlock()

	key := f.eventKey(event)
	if key == f.lastEventKey {
		return event, false // Skip duplicate
	}

	f.lastEventKey = key
	return event, true
}

// eventKey generates a unique key for comparison
func (f *DeduplicationFilter) eventKey(event StreamEvent) string {
	switch event.Type {
	case EventTypeStatus:
		if event.Status != nil {
			return string(event.Type) + ":" + event.Status.State
		}
	case EventTypeToolStarted, EventTypeToolCompleted:
		if event.Tool != nil {
			return string(event.Type) + ":" + event.Tool.Name
		}
	case EventTypeMessageChunk:
		if event.Message != nil {
			return string(event.Type) + ":" + event.Message.Content
		}
	case EventTypeProgress:
		if event.Progress != nil {
			return string(event.Type) + ":" + event.Progress.Stage
		}
	}
	return string(event.Type)
}

// TimestampFilter ensures all events have timestamps
type TimestampFilter struct{}

// NewTimestampFilter creates a new TimestampFilter
func NewTimestampFilter() *TimestampFilter {
	return &TimestampFilter{}
}

// Filter implements EventFilter
func (f *TimestampFilter) Filter(event StreamEvent) (StreamEvent, bool) {
	if event.Timestamp.IsZero() {
		event.Timestamp = event.Timestamp.UTC()
		// If still zero, use current time
		if event.Timestamp.IsZero() {
			event = StreamEvent{
				Type:        event.Type,
				Timestamp:   timeNow(),
				ExecutionID: event.ExecutionID,
				Tool:        event.Tool,
				Message:     event.Message,
				Status:      event.Status,
				Progress:    event.Progress,
				Error:       event.Error,
				Metadata:    event.Metadata,
			}
		}
	}
	return event, true
}

// EventTypeFilter only allows specific event types through
type EventTypeFilter struct {
	allowedTypes map[StreamEventType]bool
}

// NewEventTypeFilter creates a new EventTypeFilter that only allows the specified types
func NewEventTypeFilter(types ...StreamEventType) *EventTypeFilter {
	allowed := make(map[StreamEventType]bool)
	for _, t := range types {
		allowed[t] = true
	}
	return &EventTypeFilter{allowedTypes: allowed}
}

// Filter implements EventFilter
func (f *EventTypeFilter) Filter(event StreamEvent) (StreamEvent, bool) {
	if len(f.allowedTypes) == 0 {
		return event, true // No filter configured, allow all
	}
	return event, f.allowedTypes[event.Type]
}

// timeNow is a variable for testing - can be overridden in tests
var timeNow = time.Now
