package streaming

import (
	"testing"
	"time"
)

func TestEventPipeline_Process(t *testing.T) {
	mock := &MockRenderer{}
	pipeline := NewEventPipeline(mock)

	event := NewToolStartedEvent("test-tool", map[string]interface{}{"key": "value"})
	err := pipeline.Process(event)
	if err != nil {
		t.Fatalf("Process failed: %v", err)
	}

	if len(mock.Events) != 1 {
		t.Errorf("expected 1 event, got %d", len(mock.Events))
	}
}

func TestEventPipeline_FilterChain(t *testing.T) {
	mock := &MockRenderer{}
	pipeline := NewEventPipeline(mock)

	// Add a filter that blocks tool_started events
	pipeline.AddFilter(&blockingFilter{blockType: EventTypeToolStarted})

	// Try to process a tool_started event
	event := NewToolStartedEvent("test-tool", nil)
	err := pipeline.Process(event)
	if err != nil {
		t.Fatalf("Process failed: %v", err)
	}

	// Should be blocked
	if len(mock.Events) != 0 {
		t.Errorf("expected 0 events (blocked), got %d", len(mock.Events))
	}

	// Try a different event type
	doneEvent := NewDoneEvent()
	err = pipeline.Process(doneEvent)
	if err != nil {
		t.Fatalf("Process failed: %v", err)
	}

	// Should pass through
	if len(mock.Events) != 1 {
		t.Errorf("expected 1 event, got %d", len(mock.Events))
	}
}

// blockingFilter is a test filter that blocks specific event types
type blockingFilter struct {
	blockType StreamEventType
}

func (f *blockingFilter) Filter(event StreamEvent) (StreamEvent, bool) {
	return event, event.Type != f.blockType
}

func TestVerbosityFilter_NonVerbose(t *testing.T) {
	filter := NewVerbosityFilter(false)

	event := StreamEvent{
		Type: EventTypeToolCompleted,
		Tool: &ToolEventData{
			Name:            "shell",
			Inputs:          map[string]interface{}{"cmd": "ls"},
			Outputs:         map[string]interface{}{"stdout": "file1.txt"},
			DurationSeconds: 1.5,
			Success:         true,
		},
	}

	filtered, pass := filter.Filter(event)
	if !pass {
		t.Error("expected event to pass through")
	}

	if filtered.Tool.Inputs != nil {
		t.Error("expected Inputs to be stripped in non-verbose mode")
	}
	if filtered.Tool.Outputs != nil {
		t.Error("expected Outputs to be stripped in non-verbose mode")
	}
	if filtered.Tool.Name != "shell" {
		t.Errorf("expected Name to be preserved, got %s", filtered.Tool.Name)
	}
	if filtered.Tool.DurationSeconds != 1.5 {
		t.Errorf("expected DurationSeconds to be preserved, got %f", filtered.Tool.DurationSeconds)
	}
}

func TestVerbosityFilter_Verbose(t *testing.T) {
	filter := NewVerbosityFilter(true)

	event := StreamEvent{
		Type: EventTypeToolCompleted,
		Tool: &ToolEventData{
			Name:    "shell",
			Inputs:  map[string]interface{}{"cmd": "ls"},
			Outputs: map[string]interface{}{"stdout": "file1.txt"},
		},
	}

	filtered, pass := filter.Filter(event)
	if !pass {
		t.Error("expected event to pass through")
	}

	// In verbose mode, nothing should be stripped
	if filtered.Tool.Inputs == nil {
		t.Error("expected Inputs to be preserved in verbose mode")
	}
	if filtered.Tool.Outputs == nil {
		t.Error("expected Outputs to be preserved in verbose mode")
	}
}

func TestDeduplicationFilter(t *testing.T) {
	filter := NewDeduplicationFilter()

	// First event should pass
	event1 := StreamEvent{
		Type: EventTypeStatus,
		Status: &StatusEventData{
			State: "running",
		},
	}
	_, pass := filter.Filter(event1)
	if !pass {
		t.Error("first event should pass through")
	}

	// Duplicate event should be blocked
	event2 := StreamEvent{
		Type: EventTypeStatus,
		Status: &StatusEventData{
			State: "running",
		},
	}
	_, pass = filter.Filter(event2)
	if pass {
		t.Error("duplicate event should be blocked")
	}

	// Different event should pass
	event3 := StreamEvent{
		Type: EventTypeStatus,
		Status: &StatusEventData{
			State: "completed",
		},
	}
	_, pass = filter.Filter(event3)
	if !pass {
		t.Error("different event should pass through")
	}
}

func TestTimestampFilter(t *testing.T) {
	// Override timeNow for deterministic testing
	fixedTime := time.Date(2024, 1, 15, 10, 30, 0, 0, time.UTC)
	originalTimeNow := timeNow
	timeNow = func() time.Time { return fixedTime }
	defer func() { timeNow = originalTimeNow }()

	filter := NewTimestampFilter()

	// Event without timestamp
	event := StreamEvent{
		Type: EventTypeDone,
	}

	filtered, pass := filter.Filter(event)
	if !pass {
		t.Error("event should pass through")
	}

	if filtered.Timestamp.IsZero() {
		t.Error("expected timestamp to be set")
	}
}

func TestEventTypeFilter(t *testing.T) {
	filter := NewEventTypeFilter(EventTypeToolStarted, EventTypeToolCompleted)

	// Allowed types should pass
	event1 := NewToolStartedEvent("test", nil)
	_, pass := filter.Filter(event1)
	if !pass {
		t.Error("tool_started should pass through")
	}

	event2 := NewToolCompletedEvent("test", nil, 1.0, true, "")
	_, pass = filter.Filter(event2)
	if !pass {
		t.Error("tool_completed should pass through")
	}

	// Non-allowed types should be blocked
	event3 := NewStatusEvent("running", "", "")
	_, pass = filter.Filter(event3)
	if pass {
		t.Error("status should be blocked")
	}
}

func TestEventTypeFilter_Empty(t *testing.T) {
	filter := NewEventTypeFilter() // No types specified

	// All events should pass when no types specified
	event := NewStatusEvent("running", "", "")
	_, pass := filter.Filter(event)
	if !pass {
		t.Error("all events should pass when no filter types specified")
	}
}

func TestEventPipeline_MultipleFilters(t *testing.T) {
	mock := &MockRenderer{}
	pipeline := NewEventPipeline(mock)

	// Add multiple filters
	pipeline.AddFilter(NewVerbosityFilter(false))
	pipeline.AddFilter(NewDeduplicationFilter())
	pipeline.AddFilter(NewTimestampFilter())

	// Process an event
	event := StreamEvent{
		Type: EventTypeToolCompleted,
		Tool: &ToolEventData{
			Name:    "test",
			Inputs:  map[string]interface{}{"key": "value"},
			Outputs: map[string]interface{}{"result": "ok"},
		},
	}

	err := pipeline.Process(event)
	if err != nil {
		t.Fatalf("Process failed: %v", err)
	}

	if len(mock.Events) != 1 {
		t.Errorf("expected 1 event, got %d", len(mock.Events))
	}

	// Verify verbosity filter worked
	rendered := mock.Events[0]
	if rendered.Tool.Inputs != nil {
		t.Error("expected Inputs to be stripped")
	}
}
