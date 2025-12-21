package streaming

import (
	"github.com/kubiyabot/cli/internal/controlplane/entities"
)

// MapControlPlaneEvent converts a control plane StreamEvent to a unified streaming.StreamEvent
func MapControlPlaneEvent(cpEvent entities.StreamEvent) StreamEvent {
	event := StreamEvent{
		Timestamp: timeNow(),
		Metadata:  cpEvent.Metadata,
	}

	// Set timestamp from control plane event if available
	if cpEvent.Timestamp != nil {
		event.Timestamp = cpEvent.Timestamp.Time
	}

	switch cpEvent.Type {
	case entities.StreamEventTypeChunk:
		event.Type = EventTypeMessageChunk
		event.Message = &MessageEventData{
			Role:    getStringOrDefault(cpEvent.Role, "assistant"),
			Content: cpEvent.Content,
			Chunk:   true,
		}

	case entities.StreamEventTypeMessage:
		event.Type = EventTypeMessage
		event.Message = &MessageEventData{
			Role:    getStringOrDefault(cpEvent.Role, "assistant"),
			Content: cpEvent.Content,
			Chunk:   getBoolOrDefault(cpEvent.Chunk, false),
		}

	case entities.StreamEventTypeMessageChunk:
		event.Type = EventTypeMessageChunk
		event.Message = &MessageEventData{
			Role:    getStringOrDefault(cpEvent.Role, "assistant"),
			Content: cpEvent.Content,
			Chunk:   true,
		}

	case entities.StreamEventTypeToolStarted:
		event.Type = EventTypeToolStarted
		event.Tool = &ToolEventData{
			Name:   cpEvent.ToolName,
			Inputs: cpEvent.ToolInputs,
		}
		// Also check metadata for tool info if not in direct fields
		if event.Tool.Name == "" && cpEvent.Metadata != nil {
			if name, ok := cpEvent.Metadata["tool_name"].(string); ok {
				event.Tool.Name = name
			}
			if inputs, ok := cpEvent.Metadata["inputs"].(map[string]interface{}); ok {
				event.Tool.Inputs = inputs
			}
		}

	case entities.StreamEventTypeToolCompleted:
		event.Type = EventTypeToolCompleted
		event.Tool = &ToolEventData{
			Name:            cpEvent.ToolName,
			Outputs:         cpEvent.ToolOutputs,
			DurationSeconds: getFloatOrDefault(cpEvent.Duration, 0),
			Success:         getBoolOrDefault(cpEvent.Success, true),
		}
		// Also check metadata for tool info if not in direct fields
		if event.Tool.Name == "" && cpEvent.Metadata != nil {
			if name, ok := cpEvent.Metadata["tool_name"].(string); ok {
				event.Tool.Name = name
			}
			if outputs, ok := cpEvent.Metadata["outputs"].(map[string]interface{}); ok {
				event.Tool.Outputs = outputs
			}
			if duration, ok := cpEvent.Metadata["duration"].(float64); ok {
				event.Tool.DurationSeconds = duration
			}
			if success, ok := cpEvent.Metadata["success"].(bool); ok {
				event.Tool.Success = success
			}
		}

	case entities.StreamEventTypeStatus:
		event.Type = EventTypeStatus
		state := "unknown"
		if cpEvent.Status != nil {
			state = string(*cpEvent.Status)
		}
		event.Status = &StatusEventData{
			State: state,
		}
		// Check metadata for additional status info
		if cpEvent.Metadata != nil {
			if reason, ok := cpEvent.Metadata["reason"].(string); ok {
				event.Status.Reason = reason
			}
			if prevState, ok := cpEvent.Metadata["previous_state"].(string); ok {
				event.Status.PreviousState = prevState
			}
		}

	case entities.StreamEventTypeError:
		event.Type = EventTypeError
		event.Error = &ErrorEventData{
			Message: cpEvent.Content,
		}
		// Check metadata for additional error info
		if cpEvent.Metadata != nil {
			if code, ok := cpEvent.Metadata["code"].(string); ok {
				event.Error.Code = code
			}
			if recoverable, ok := cpEvent.Metadata["recoverable"].(bool); ok {
				event.Error.Recoverable = recoverable
			}
		}

	case entities.StreamEventTypeComplete:
		event.Type = EventTypeDone

	case entities.StreamEventTypeDone:
		event.Type = EventTypeDone

	case entities.StreamEventTypeConnected:
		event.Type = EventTypeConnected
		// Check metadata for execution_id
		if cpEvent.Metadata != nil {
			if execID, ok := cpEvent.Metadata["execution_id"].(string); ok {
				event.ExecutionID = execID
			}
		}

	default:
		// Unknown event type - try to map based on content
		if cpEvent.Content != "" {
			event.Type = EventTypeMessageChunk
			event.Message = &MessageEventData{
				Role:    "assistant",
				Content: cpEvent.Content,
				Chunk:   true,
			}
		} else {
			// Return as-is with unknown type indicator
			event.Type = StreamEventType(cpEvent.Type)
		}
	}

	return event
}

// Helper functions for safe type assertions

func getStringOrDefault(ptr string, defaultVal string) string {
	if ptr != "" {
		return ptr
	}
	return defaultVal
}

func getBoolOrDefault(ptr *bool, defaultVal bool) bool {
	if ptr != nil {
		return *ptr
	}
	return defaultVal
}

func getFloatOrDefault(ptr *float64, defaultVal float64) float64 {
	if ptr != nil {
		return *ptr
	}
	return defaultVal
}

// MapPlanStreamEvent maps a plan stream event to a unified StreamEvent
// This is for the planning phase events from the planner API
type PlanStreamEvent struct {
	Type string                 `json:"type"`
	Data map[string]interface{} `json:"data,omitempty"`
}

// MapPlanEvent converts a plan stream event to a unified streaming.StreamEvent
func MapPlanEvent(planEvent PlanStreamEvent) StreamEvent {
	event := StreamEvent{
		Timestamp: timeNow(),
		Metadata:  planEvent.Data,
	}

	switch planEvent.Type {
	case "progress":
		event.Type = EventTypeProgress
		stage := ""
		message := ""
		percent := 0
		if s, ok := planEvent.Data["stage"].(string); ok {
			stage = s
		}
		if m, ok := planEvent.Data["message"].(string); ok {
			message = m
		}
		if p, ok := planEvent.Data["progress"].(float64); ok {
			percent = int(p)
		}
		event.Progress = &ProgressEventData{
			Stage:   stage,
			Message: message,
			Percent: percent,
		}

	case "thinking":
		event.Type = EventTypeMessageChunk
		content := ""
		if c, ok := planEvent.Data["content"].(string); ok {
			content = c
		}
		event.Message = &MessageEventData{
			Role:    "planner",
			Content: content,
			Chunk:   true,
		}

	case "tool_call":
		event.Type = EventTypeToolStarted
		toolName := ""
		if name, ok := planEvent.Data["tool_name"].(string); ok {
			toolName = name
		}
		event.Tool = &ToolEventData{
			Name:   toolName,
			Inputs: planEvent.Data,
		}

	case "tool_result":
		event.Type = EventTypeToolCompleted
		toolName := ""
		if name, ok := planEvent.Data["tool_name"].(string); ok {
			toolName = name
		}
		event.Tool = &ToolEventData{
			Name:    toolName,
			Success: true,
			Outputs: planEvent.Data,
		}

	case "step_started", "step_running":
		event.Type = EventTypeProgress
		step := ""
		if s, ok := planEvent.Data["step"].(string); ok {
			step = s
		}
		event.Progress = &ProgressEventData{
			Stage:   step,
			Message: "Running step",
		}

	case "step_completed":
		event.Type = EventTypeProgress
		step := ""
		if s, ok := planEvent.Data["step"].(string); ok {
			step = s
		}
		event.Progress = &ProgressEventData{
			Stage:   step,
			Message: "Step completed",
			Percent: 100,
		}

	case "complete":
		event.Type = EventTypeDone

	case "error":
		event.Type = EventTypeError
		message := ""
		if m, ok := planEvent.Data["message"].(string); ok {
			message = m
		}
		if message == "" {
			if m, ok := planEvent.Data["error"].(string); ok {
				message = m
			}
		}
		event.Error = &ErrorEventData{
			Message: message,
		}

	case "resources_summary":
		event.Type = EventTypeProgress
		event.Progress = &ProgressEventData{
			Stage:   "Resources",
			Message: "Resource discovery complete",
		}

	default:
		// Unknown type - return as generic message if has content
		if content, ok := planEvent.Data["content"].(string); ok && content != "" {
			event.Type = EventTypeMessageChunk
			event.Message = &MessageEventData{
				Role:    "planner",
				Content: content,
				Chunk:   true,
			}
		} else {
			event.Type = StreamEventType(planEvent.Type)
		}
	}

	return event
}
