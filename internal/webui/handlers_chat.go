package webui

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"strings"
	"sync"
	"time"
)

// ChatSession represents an active chat session using control plane streaming
type ChatSession struct {
	ID           string
	AgentID      string
	AgentName    string
	Cancel       context.CancelFunc
	Done         chan struct{}
	StartTime    time.Time
	MessageCount int
	mu           sync.Mutex
	subscribers  []chan ChatEvent

	// Current execution context
	currentExecID     string
	currentExecCancel context.CancelFunc
}

// ChatEvent represents a streaming chat event
type ChatEvent struct {
	Type       string `json:"type"` // connected, message_start, content_delta, thinking, tool_call, tool_result, message_end, error, done
	Content    string `json:"content,omitempty"`
	MessageID  string `json:"message_id,omitempty"`
	Role       string `json:"role,omitempty"`
	ToolCallID string `json:"tool_call_id,omitempty"`
	Name       string `json:"name,omitempty"`
	Input      string `json:"input,omitempty"`
	Output     string `json:"output,omitempty"`
	IsError    bool   `json:"is_error,omitempty"`
	Timestamp  string `json:"timestamp"`
}

// ChatManager manages active chat sessions
type ChatManager struct {
	sessions      map[string]*ChatSession
	cleanupTimers map[string]*time.Timer
	mu            sync.RWMutex
}

var chatManager = &ChatManager{
	sessions:      make(map[string]*ChatSession),
	cleanupTimers: make(map[string]*time.Timer),
}

// StartChatRequest is the request to start a chat session
type StartChatRequest struct {
	AgentID string `json:"agent_id"`
}

// SendMessageRequest is the request to send a message
type SendMessageRequest struct {
	SessionID string `json:"session_id"`
	Content   string `json:"content"`
}

// handleChatStart handles POST /api/chat/start - start a chat session
func (s *Server) handleChatStart(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		writeError(w, http.StatusMethodNotAllowed, "method not allowed")
		return
	}

	var req StartChatRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, http.StatusBadRequest, "invalid request body")
		return
	}

	if req.AgentID == "" {
		writeError(w, http.StatusBadRequest, "agent_id required")
		return
	}

	// Check we have control plane client
	if s.cpClient == nil {
		writeError(w, http.StatusServiceUnavailable, "control plane client not configured")
		return
	}

	// Generate session ID
	sessionID := fmt.Sprintf("chat-%d", time.Now().UnixNano())

	// Create chat session
	ctx, cancel := context.WithCancel(context.Background())
	session := &ChatSession{
		ID:        sessionID,
		AgentID:   req.AgentID,
		Cancel:    cancel,
		Done:      make(chan struct{}),
		StartTime: time.Now(),
	}

	chatManager.mu.Lock()
	chatManager.sessions[sessionID] = session
	chatManager.mu.Unlock()

	// Start session lifecycle manager
	go s.runChatSession(ctx, session)

	writeJSON(w, map[string]interface{}{
		"success":    true,
		"session_id": sessionID,
		"message":    "Chat session started",
	})
}

// handleChatStream handles GET /api/chat/stream/{id} - SSE stream for chat events
func (s *Server) handleChatStream(w http.ResponseWriter, r *http.Request) {
	path := r.URL.Path
	sessionID := strings.TrimPrefix(path, "/api/chat/stream/")

	if sessionID == "" {
		writeError(w, http.StatusBadRequest, "session_id required")
		return
	}

	chatManager.mu.RLock()
	session, exists := chatManager.sessions[sessionID]
	chatManager.mu.RUnlock()

	if !exists {
		writeError(w, http.StatusNotFound, "session not found")
		return
	}

	// Set up SSE
	w.Header().Set("Content-Type", "text/event-stream")
	w.Header().Set("Cache-Control", "no-cache")
	w.Header().Set("Connection", "keep-alive")
	w.Header().Set("X-Accel-Buffering", "no")

	flusher, ok := w.(http.Flusher)
	if !ok {
		writeError(w, http.StatusInternalServerError, "streaming not supported")
		return
	}

	// Create subscriber channel
	subscriber := make(chan ChatEvent, 100)
	session.mu.Lock()
	session.subscribers = append(session.subscribers, subscriber)
	session.mu.Unlock()

	// Send connected event
	connectedEvent := ChatEvent{
		Type:      "connected",
		Timestamp: time.Now().Format(time.RFC3339),
	}
	data, _ := json.Marshal(connectedEvent)
	fmt.Fprintf(w, "data: %s\n\n", data)
	flusher.Flush()

	// Stream events
	for {
		select {
		case event, ok := <-subscriber:
			if !ok {
				return
			}
			data, _ := json.Marshal(event)
			fmt.Fprintf(w, "data: %s\n\n", data)
			flusher.Flush()

		case <-session.Done:
			return

		case <-r.Context().Done():
			return
		}
	}
}

// handleChatSend handles POST /api/chat/send - send a message
func (s *Server) handleChatSend(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		writeError(w, http.StatusMethodNotAllowed, "method not allowed")
		return
	}

	var req SendMessageRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, http.StatusBadRequest, "invalid request body")
		return
	}

	if req.SessionID == "" {
		writeError(w, http.StatusBadRequest, "session_id required")
		return
	}

	if req.Content == "" {
		writeError(w, http.StatusBadRequest, "content required")
		return
	}

	chatManager.mu.RLock()
	session, exists := chatManager.sessions[req.SessionID]
	chatManager.mu.RUnlock()

	if !exists {
		writeError(w, http.StatusNotFound, "session not found")
		return
	}

	// Start execution in background
	go s.sendChatMessage(session, req.Content)

	session.MessageCount++

	writeJSON(w, map[string]interface{}{
		"success": true,
		"message": "Message sent",
	})
}

// handleChatEnd handles POST /api/chat/end/{id} - end a chat session
func (s *Server) handleChatEnd(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		writeError(w, http.StatusMethodNotAllowed, "method not allowed")
		return
	}

	path := r.URL.Path
	sessionID := strings.TrimPrefix(path, "/api/chat/end/")

	if sessionID == "" {
		writeError(w, http.StatusBadRequest, "session_id required")
		return
	}

	chatManager.mu.RLock()
	session, exists := chatManager.sessions[sessionID]
	chatManager.mu.RUnlock()

	if !exists {
		writeError(w, http.StatusNotFound, "session not found")
		return
	}

	// Cancel session and any running execution
	session.Cancel()
	if session.currentExecCancel != nil {
		session.currentExecCancel()
	}

	writeJSON(w, map[string]interface{}{
		"success": true,
		"message": "Chat session ended",
	})
}

// runChatSession manages the chat session lifecycle
func (s *Server) runChatSession(ctx context.Context, session *ChatSession) {
	defer func() {
		close(session.Done)
		// Notify all subscribers
		session.mu.Lock()
		for _, sub := range session.subscribers {
			close(sub)
		}
		session.mu.Unlock()

		// Schedule cleanup with cancellable timer
		chatManager.mu.Lock()
		// Cancel any existing cleanup timer for this session
		if timer, exists := chatManager.cleanupTimers[session.ID]; exists {
			timer.Stop()
		}
		// Create new cleanup timer
		chatManager.cleanupTimers[session.ID] = time.AfterFunc(10*time.Minute, func() {
			chatManager.mu.Lock()
			delete(chatManager.sessions, session.ID)
			delete(chatManager.cleanupTimers, session.ID)
			chatManager.mu.Unlock()
		})
		chatManager.mu.Unlock()
	}()

	// Wait for context cancellation
	<-ctx.Done()
}

// sendChatMessage sends a message via control plane and streams the response
func (s *Server) sendChatMessage(session *ChatSession, message string) {
	if s.cpClient == nil {
		session.broadcastEvent(ChatEvent{
			Type:      "error",
			Content:   "Control plane client not configured",
			Timestamp: time.Now().Format(time.RFC3339),
		})
		return
	}

	// Broadcast message start
	session.broadcastEvent(ChatEvent{
		Type:      "message_start",
		MessageID: fmt.Sprintf("msg-%d", time.Now().UnixNano()),
		Role:      "assistant",
		Timestamp: time.Now().Format(time.RFC3339),
	})

	// Submit execution to control plane
	streamFlag := true
	queueID := s.config.QueueID
	execReq := &CPExecuteRequest{
		Prompt: message,
		Stream: &streamFlag,
	}

	// Only set queue ID if we have one
	if queueID != "" {
		execReq.WorkerQueueID = &queueID
	}

	execution, err := s.cpClient.executeAgent(session.AgentID, execReq)
	if err != nil {
		session.broadcastEvent(ChatEvent{
			Type:      "error",
			Content:   fmt.Sprintf("Failed to start execution: %v", err),
			Timestamp: time.Now().Format(time.RFC3339),
		})
		return
	}

	// Store execution context for cancellation
	ctx, cancel := context.WithCancel(context.Background())
	session.mu.Lock()
	session.currentExecID = execution.ID
	session.currentExecCancel = cancel
	session.mu.Unlock()

	defer func() {
		session.mu.Lock()
		session.currentExecID = ""
		session.currentExecCancel = nil
		session.mu.Unlock()
	}()

	// Stream events from control plane
	eventChan, errChan := s.cpClient.streamExecution(ctx, execution.ID)

	for {
		select {
		case event, ok := <-eventChan:
			if !ok {
				// Stream ended
				session.broadcastEvent(ChatEvent{
					Type:      "message_end",
					Timestamp: time.Now().Format(time.RFC3339),
				})
				return
			}

			// Convert StreamEvent to ChatEvent
			chatEvent := convertToChatEvent(event)
			if chatEvent.Type != "" {
				session.broadcastEvent(chatEvent)
			}

		case err, ok := <-errChan:
			if ok && err != nil {
				session.broadcastEvent(ChatEvent{
					Type:      "error",
					Content:   err.Error(),
					Timestamp: time.Now().Format(time.RFC3339),
				})
			}
			return

		case <-ctx.Done():
			return
		}
	}
}

// convertToChatEvent converts a StreamEvent to ChatEvent
func convertToChatEvent(event StreamEvent) ChatEvent {
	chatEvent := ChatEvent{
		Timestamp: event.Timestamp,
	}

	switch event.Type {
	case "output":
		chatEvent.Type = "content_delta"
		chatEvent.Content = event.Content
		chatEvent.Role = event.Role
		if chatEvent.Role == "" {
			chatEvent.Role = "assistant"
		}

	case "tool_call":
		chatEvent.Type = "tool_call"
		chatEvent.Name = event.ToolName
		chatEvent.Input = event.ToolInput
		chatEvent.ToolCallID = fmt.Sprintf("tool-%d", time.Now().UnixNano())

	case "tool_result":
		chatEvent.Type = "tool_result"
		chatEvent.Name = event.ToolName
		chatEvent.Output = event.ToolOutput
		chatEvent.IsError = event.Status == "error" || event.Status == "failed"

	case "reasoning":
		chatEvent.Type = "thinking"
		chatEvent.Content = event.Content

	case "status":
		// Skip most status events, but handle certain ones
		if event.Status == "completed" || event.Status == "done" {
			chatEvent.Type = "message_end"
		} else {
			return ChatEvent{} // Skip
		}

	case "error":
		chatEvent.Type = "error"
		chatEvent.Content = event.Content

	case "done":
		chatEvent.Type = "done"
		chatEvent.Content = event.Content

	default:
		// Unknown type - skip
		return ChatEvent{}
	}

	return chatEvent
}

// broadcastEvent sends an event to all subscribers
func (s *ChatSession) broadcastEvent(event ChatEvent) {
	s.mu.Lock()
	defer s.mu.Unlock()

	for _, sub := range s.subscribers {
		select {
		case sub <- event:
		default:
			// Channel full, skip
		}
	}
}
