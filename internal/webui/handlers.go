package webui

import (
	"encoding/json"
	"net/http"
	"strconv"
	"strings"
	"time"
)

// handleHealth handles GET /api/health
func (s *Server) handleHealth(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		writeError(w, http.StatusMethodNotAllowed, "method not allowed")
		return
	}

	health := HealthResponse{
		Status: "ok",
		Uptime: time.Since(s.startTime),
		Components: map[string]HealthStatus{
			"webui": {
				Status:    "ok",
				CheckedAt: time.Now(),
			},
			"worker": {
				Status:    "ok",
				CheckedAt: time.Now(),
			},
		},
	}

	// Check control plane
	cp := s.state.GetControlPlane()
	if cp.Connected {
		health.Components["control_plane"] = HealthStatus{
			Status:    "ok",
			CheckedAt: cp.LastCheck,
		}
	} else {
		health.Components["control_plane"] = HealthStatus{
			Status:    "error",
			Message:   cp.ErrorMessage,
			CheckedAt: cp.LastCheck,
		}
		health.Status = "degraded"
	}

	// Check LiteLLM proxy if enabled
	config := s.state.GetConfig()
	if config.EnableLocalProxy {
		// For now, assume it's OK if we have a port configured
		if config.ProxyPort > 0 {
			health.Components["litellm_proxy"] = HealthStatus{
				Status:    "ok",
				CheckedAt: time.Now(),
			}
		} else {
			health.Components["litellm_proxy"] = HealthStatus{
				Status:    "unknown",
				Message:   "proxy not initialized",
				CheckedAt: time.Now(),
			}
		}
	}

	writeJSON(w, health)
}

// handleOverview handles GET /api/overview
func (s *Server) handleOverview(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		writeError(w, http.StatusMethodNotAllowed, "method not allowed")
		return
	}

	overview := s.state.GetOverview()
	writeJSON(w, overview)
}

// handleWorkers handles GET /api/workers
func (s *Server) handleWorkers(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		writeError(w, http.StatusMethodNotAllowed, "method not allowed")
		return
	}

	workers := s.state.GetWorkers()
	writeJSON(w, workers)
}

// handleWorkerByID handles GET/POST /api/workers/{id}
func (s *Server) handleWorkerByID(w http.ResponseWriter, r *http.Request) {
	// Extract worker ID from path
	path := strings.TrimPrefix(r.URL.Path, "/api/workers/")
	parts := strings.Split(path, "/")
	if len(parts) == 0 || parts[0] == "" {
		writeError(w, http.StatusBadRequest, "worker ID required")
		return
	}

	workerID := parts[0]

	// Check for action paths
	if len(parts) > 1 {
		action := parts[1]
		switch action {
		case "restart":
			s.handleWorkerRestart(w, r, workerID)
			return
		case "logs":
			s.handleWorkerLogs(w, r, workerID)
			return
		default:
			writeError(w, http.StatusNotFound, "action not found")
			return
		}
	}

	// GET worker detail
	if r.Method != http.MethodGet {
		writeError(w, http.StatusMethodNotAllowed, "method not allowed")
		return
	}

	worker, ok := s.state.GetWorker(workerID)
	if !ok {
		writeError(w, http.StatusNotFound, "worker not found")
		return
	}

	// Build detail with metrics
	detail := WorkerDetail{
		WorkerInfo: *worker,
	}

	if metrics, ok := s.state.GetWorkerMetrics(workerID); ok {
		detail.Metrics = metrics
	}

	writeJSON(w, detail)
}

// handleWorkerRestart handles POST /api/workers/{id}/restart
func (s *Server) handleWorkerRestart(w http.ResponseWriter, r *http.Request, workerID string) {
	if r.Method != http.MethodPost {
		writeError(w, http.StatusMethodNotAllowed, "method not allowed")
		return
	}

	worker, ok := s.state.GetWorker(workerID)
	if !ok {
		writeError(w, http.StatusNotFound, "worker not found")
		return
	}

	// Log the restart request
	s.state.AddLog(LogEntry{
		Timestamp: time.Now(),
		Level:     LogLevelInfo,
		Component: "webui",
		Message:   "Worker restart requested via WebUI",
		WorkerID:  workerID,
	})

	s.state.AddActivity(RecentActivity{
		Type:        "worker_restart_requested",
		Description: "Worker restart requested via WebUI",
		Timestamp:   time.Now(),
		WorkerID:    workerID,
	})

	// Update worker status
	s.state.UpdateWorker(workerID, func(w *WorkerInfo) {
		w.Status = WorkerStatusStopping
	})

	// Note: Actual restart logic would need to be handled by the parent process
	// For now, we just acknowledge the request
	writeJSON(w, ActionResponse{
		Success: true,
		Message: "Restart requested for worker " + worker.ID,
	})
}

// handleWorkerLogs handles GET /api/workers/{id}/logs
func (s *Server) handleWorkerLogs(w http.ResponseWriter, r *http.Request, workerID string) {
	if r.Method != http.MethodGet {
		writeError(w, http.StatusMethodNotAllowed, "method not allowed")
		return
	}

	// Parse query parameters
	filter := LogFilter{
		WorkerID: workerID,
	}

	if level := r.URL.Query().Get("level"); level != "" {
		filter.Level = LogLevel(level)
	}
	if limitStr := r.URL.Query().Get("limit"); limitStr != "" {
		if limit, err := strconv.Atoi(limitStr); err == nil {
			filter.Limit = limit
		}
	}

	logs := s.state.GetLogs(filter)
	writeJSON(w, logs)
}

// handleControlPlane handles GET /api/control-plane
func (s *Server) handleControlPlane(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		writeError(w, http.StatusMethodNotAllowed, "method not allowed")
		return
	}

	status := s.state.GetControlPlane()
	writeJSON(w, status)
}

// handleControlPlaneReconnect handles POST /api/control-plane/reconnect
func (s *Server) handleControlPlaneReconnect(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		writeError(w, http.StatusMethodNotAllowed, "method not allowed")
		return
	}

	s.state.AddLog(LogEntry{
		Timestamp: time.Now(),
		Level:     LogLevelInfo,
		Component: "webui",
		Message:   "Control plane reconnect requested via WebUI",
	})

	s.state.AddActivity(RecentActivity{
		Type:        "control_plane_reconnect",
		Description: "Control plane reconnect requested via WebUI",
		Timestamp:   time.Now(),
	})

	// Trigger immediate health check
	go s.checkControlPlane()

	writeJSON(w, ActionResponse{
		Success: true,
		Message: "Reconnect initiated",
	})
}

// handleMetrics handles GET /api/metrics
func (s *Server) handleMetrics(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		writeError(w, http.StatusMethodNotAllowed, "method not allowed")
		return
	}

	snapshot := s.state.GetMetricsSnapshot()
	writeJSON(w, snapshot)
}

// handleSessions handles GET /api/sessions
func (s *Server) handleSessions(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		writeError(w, http.StatusMethodNotAllowed, "method not allowed")
		return
	}

	sessions := s.state.GetSessions()
	writeJSON(w, sessions)
}

// handleSessionByID handles GET /api/sessions/{id}
func (s *Server) handleSessionByID(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		writeError(w, http.StatusMethodNotAllowed, "method not allowed")
		return
	}

	// Extract session ID from path
	sessionID := strings.TrimPrefix(r.URL.Path, "/api/sessions/")
	if sessionID == "" {
		writeError(w, http.StatusBadRequest, "session ID required")
		return
	}

	session, ok := s.state.GetSession(sessionID)
	if !ok {
		writeError(w, http.StatusNotFound, "session not found")
		return
	}

	// Build session detail
	detail := SessionDetail{
		SessionInfo: *session,
		// Messages and events would be populated if we had them
		Messages: []SessionMessage{},
		Events:   []SessionEvent{},
	}

	writeJSON(w, detail)
}

// handleConfig handles GET /api/config
func (s *Server) handleConfig(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		writeError(w, http.StatusMethodNotAllowed, "method not allowed")
		return
	}

	config := s.state.GetConfig()
	writeJSON(w, config)
}

// handleLogs handles GET /api/logs
func (s *Server) handleLogs(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		writeError(w, http.StatusMethodNotAllowed, "method not allowed")
		return
	}

	// Parse query parameters
	filter := LogFilter{}

	if level := r.URL.Query().Get("level"); level != "" {
		filter.Level = LogLevel(level)
	}
	if component := r.URL.Query().Get("component"); component != "" {
		filter.Component = component
	}
	if workerID := r.URL.Query().Get("worker_id"); workerID != "" {
		filter.WorkerID = workerID
	}
	if limitStr := r.URL.Query().Get("limit"); limitStr != "" {
		if limit, err := strconv.Atoi(limitStr); err == nil {
			filter.Limit = limit
		}
	}

	// Default limit if not specified
	if filter.Limit == 0 {
		filter.Limit = 100
	}

	logs := s.state.GetLogs(filter)
	writeJSON(w, logs)
}

// handleActivity handles GET /api/activity
func (s *Server) handleActivity(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		writeError(w, http.StatusMethodNotAllowed, "method not allowed")
		return
	}

	// Parse limit parameter
	limit := 10
	if limitStr := r.URL.Query().Get("limit"); limitStr != "" {
		if l, err := strconv.Atoi(limitStr); err == nil && l > 0 {
			limit = l
		}
	}

	activity := s.state.GetRecentActivity(limit)
	writeJSON(w, activity)
}

// handleSSE handles GET /api/events (Server-Sent Events)
func (s *Server) handleSSE(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		writeError(w, http.StatusMethodNotAllowed, "method not allowed")
		return
	}

	// Set SSE headers
	w.Header().Set("Content-Type", "text/event-stream")
	w.Header().Set("Cache-Control", "no-cache")
	w.Header().Set("Connection", "keep-alive")
	w.Header().Set("Access-Control-Allow-Origin", "*")

	// Get flusher
	flusher, ok := w.(http.Flusher)
	if !ok {
		writeError(w, http.StatusInternalServerError, "streaming not supported")
		return
	}

	// Subscribe to events
	eventCh := s.state.Subscribe()
	defer s.state.Unsubscribe(eventCh)

	// Send initial data
	s.sendSSEEvent(w, flusher, SSEEvent{
		Type: SSEEventOverview,
		Data: s.state.GetOverview(),
	})

	// Send heartbeat to confirm connection
	s.sendSSEEvent(w, flusher, SSEEvent{
		Type: SSEEventHeartbeat,
		Data: map[string]interface{}{
			"timestamp": time.Now(),
		},
	})

	// Start heartbeat ticker
	heartbeat := time.NewTicker(15 * time.Second)
	defer heartbeat.Stop()

	// Stream events
	for {
		select {
		case <-r.Context().Done():
			return
		case <-s.stopCh:
			return
		case event := <-eventCh:
			s.sendSSEEvent(w, flusher, event)
		case <-heartbeat.C:
			s.sendSSEEvent(w, flusher, SSEEvent{
				Type: SSEEventHeartbeat,
				Data: map[string]interface{}{
					"timestamp": time.Now(),
				},
			})
		}
	}
}

// handleLogStream handles GET /api/logs/stream (SSE for logs)
func (s *Server) handleLogStream(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		writeError(w, http.StatusMethodNotAllowed, "method not allowed")
		return
	}

	// Set SSE headers
	w.Header().Set("Content-Type", "text/event-stream")
	w.Header().Set("Cache-Control", "no-cache")
	w.Header().Set("Connection", "keep-alive")
	w.Header().Set("Access-Control-Allow-Origin", "*")

	// Get flusher
	flusher, ok := w.(http.Flusher)
	if !ok {
		writeError(w, http.StatusInternalServerError, "streaming not supported")
		return
	}

	// Parse filter from query
	filter := LogFilter{}
	if level := r.URL.Query().Get("level"); level != "" {
		filter.Level = LogLevel(level)
	}
	if component := r.URL.Query().Get("component"); component != "" {
		filter.Component = component
	}
	if workerID := r.URL.Query().Get("worker_id"); workerID != "" {
		filter.WorkerID = workerID
	}

	// Subscribe to events
	eventCh := s.state.Subscribe()
	defer s.state.Unsubscribe(eventCh)

	// Send recent logs first
	recentLogs := s.state.GetRecentLogs(50)
	for i := len(recentLogs) - 1; i >= 0; i-- {
		log := recentLogs[i]
		if matchesFilter(log, filter) {
			s.sendSSEEvent(w, flusher, SSEEvent{
				Type: SSEEventLog,
				Data: log,
			})
		}
	}

	// Stream new logs
	for {
		select {
		case <-r.Context().Done():
			return
		case <-s.stopCh:
			return
		case event := <-eventCh:
			if event.Type == SSEEventLog {
				if log, ok := event.Data.(LogEntry); ok {
					if matchesFilter(log, filter) {
						s.sendSSEEvent(w, flusher, event)
					}
				}
			}
		}
	}
}

// sendSSEEvent sends an SSE event
func (s *Server) sendSSEEvent(w http.ResponseWriter, flusher http.Flusher, event SSEEvent) {
	data, err := json.Marshal(event)
	if err != nil {
		return
	}

	// Write SSE format
	_, _ = w.Write([]byte("event: " + string(event.Type) + "\n"))
	_, _ = w.Write([]byte("data: " + string(data) + "\n\n"))
	flusher.Flush()
}

// matchesFilter checks if a log entry matches the given filter
func matchesFilter(log LogEntry, filter LogFilter) bool {
	if filter.Level != "" && log.Level != filter.Level {
		return false
	}
	if filter.Component != "" && log.Component != filter.Component {
		return false
	}
	if filter.WorkerID != "" && log.WorkerID != filter.WorkerID {
		return false
	}
	return true
}
