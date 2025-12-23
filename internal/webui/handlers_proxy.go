package webui

import (
	"bufio"
	"encoding/json"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"time"
)

// ProxySupervisor interface defines what we need from a proxy supervisor
// This allows us to interface with cli.LiteLLMProxySupervisor without import cycles
type ProxySupervisor interface {
	GetBaseURL() string
	Stop() error
}

// proxyRef holds an optional reference to the proxy supervisor
var proxyRef ProxySupervisor

// SetProxySupervisor sets the proxy supervisor reference for the webui package
func SetProxySupervisor(supervisor ProxySupervisor) {
	proxyRef = supervisor
}

// handleProxyStatus handles GET /api/proxy/status
func (s *Server) handleProxyStatus(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		writeError(w, http.StatusMethodNotAllowed, "method not allowed")
		return
	}

	status := s.getProxyStatus()
	writeJSON(w, status)
}

// handleProxyControl handles POST /api/proxy/control
func (s *Server) handleProxyControl(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		writeError(w, http.StatusMethodNotAllowed, "method not allowed")
		return
	}

	var request ProxyControlRequest
	if err := json.NewDecoder(r.Body).Decode(&request); err != nil {
		writeError(w, http.StatusBadRequest, "invalid request body")
		return
	}

	var result ActionResponse

	switch request.Action {
	case "start":
		// Starting the proxy requires access to the worker start process
		// This is a complex operation that requires restarting the worker
		result = ActionResponse{
			Success: false,
			Message: "Proxy control requires worker restart. Please restart the worker with --enable-local-proxy flag.",
			Error:   "automated proxy start not yet implemented",
		}

	case "stop":
		if proxyRef == nil {
			result = ActionResponse{
				Success: false,
				Error:   "no proxy is running",
			}
		} else {
			if err := proxyRef.Stop(); err != nil {
				result = ActionResponse{
					Success: false,
					Error:   err.Error(),
				}
			} else {
				result = ActionResponse{
					Success: true,
					Message: "Proxy stopped",
				}
				// Clear reference
				proxyRef = nil
			}
		}

	case "restart":
		result = ActionResponse{
			Success: false,
			Message: "Proxy restart requires worker restart. Please restart the worker.",
			Error:   "automated proxy restart not yet implemented",
		}

	default:
		writeError(w, http.StatusBadRequest, "invalid action: "+request.Action)
		return
	}

	// Log the action
	s.state.AddLog(LogEntry{
		Timestamp: time.Now(),
		Level:     LogLevelInfo,
		Component: "proxy",
		Message:   "Proxy control action: " + request.Action + " - " + result.Message,
	})

	if result.Success {
		w.WriteHeader(http.StatusOK)
	} else {
		w.WriteHeader(http.StatusInternalServerError)
	}
	writeJSON(w, result)
}

// handleProxyLogs handles GET /api/proxy/logs
func (s *Server) handleProxyLogs(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		writeError(w, http.StatusMethodNotAllowed, "method not allowed")
		return
	}

	// Read proxy log file
	logPath := filepath.Join(s.config.WorkerDir, "litellm_proxy.log")
	content, err := os.ReadFile(logPath)
	if err != nil {
		if os.IsNotExist(err) {
			writeJSON(w, map[string]interface{}{
				"logs":    []string{},
				"message": "No proxy logs available",
			})
			return
		}
		writeError(w, http.StatusInternalServerError, "failed to read log file")
		return
	}

	// Parse query parameters
	limitStr := r.URL.Query().Get("lines")
	limit := 100
	if limitStr != "" {
		if n, err := parseIntParam(limitStr); err == nil && n > 0 {
			limit = n
		}
	}

	searchTerm := r.URL.Query().Get("search")
	levelFilter := r.URL.Query().Get("level")

	// Split into lines and filter
	lines := strings.Split(string(content), "\n")
	var filteredLines []string

	for _, line := range lines {
		if line == "" {
			continue
		}

		// Apply search filter
		if searchTerm != "" && !strings.Contains(strings.ToLower(line), strings.ToLower(searchTerm)) {
			continue
		}

		// Apply level filter
		if levelFilter != "" {
			levelFilter = strings.ToUpper(levelFilter)
			if levelFilter == "ERROR" && !strings.Contains(line, "ERROR") {
				continue
			}
			if levelFilter == "WARNING" && !strings.Contains(line, "WARNING") && !strings.Contains(line, "WARN") {
				continue
			}
			if levelFilter == "INFO" && !strings.Contains(line, "INFO") {
				continue
			}
		}

		filteredLines = append(filteredLines, line)
	}

	// Return last N lines
	startIdx := 0
	if len(filteredLines) > limit {
		startIdx = len(filteredLines) - limit
	}

	writeJSON(w, map[string]interface{}{
		"logs":      filteredLines[startIdx:],
		"total":     len(filteredLines),
		"log_path":  logPath,
		"truncated": startIdx > 0,
	})
}

// handleProxyLogsStream handles GET /api/proxy/logs/stream (SSE)
func (s *Server) handleProxyLogsStream(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		writeError(w, http.StatusMethodNotAllowed, "method not allowed")
		return
	}

	// Set SSE headers
	w.Header().Set("Content-Type", "text/event-stream")
	w.Header().Set("Cache-Control", "no-cache")
	w.Header().Set("Connection", "keep-alive")
	w.Header().Set("Access-Control-Allow-Origin", "*")

	flusher, ok := w.(http.Flusher)
	if !ok {
		writeError(w, http.StatusInternalServerError, "streaming not supported")
		return
	}

	// Open log file
	logPath := filepath.Join(s.config.WorkerDir, "litellm_proxy.log")
	file, err := os.Open(logPath)
	if err != nil {
		s.sendSSEEvent(w, flusher, SSEEvent{
			Type: "proxy_log_error",
			Data: map[string]string{"error": "Log file not available"},
		})
		return
	}
	defer file.Close()

	// Seek to end of file
	file.Seek(0, 2)

	// Create scanner
	reader := bufio.NewReader(file)
	ticker := time.NewTicker(500 * time.Millisecond)
	defer ticker.Stop()

	for {
		select {
		case <-r.Context().Done():
			return
		case <-s.stopCh:
			return
		case <-ticker.C:
			// Try to read new lines
			for {
				line, err := reader.ReadString('\n')
				if err != nil {
					break
				}
				line = strings.TrimSpace(line)
				if line != "" {
					s.sendSSEEvent(w, flusher, SSEEvent{
						Type: "proxy_log",
						Data: map[string]string{"line": line},
					})
				}
			}
		}
	}
}

// handleProxyLangfuse handles PUT /api/proxy/langfuse
func (s *Server) handleProxyLangfuse(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPut {
		writeError(w, http.StatusMethodNotAllowed, "method not allowed")
		return
	}

	var config LangfuseConfig
	if err := json.NewDecoder(r.Body).Decode(&config); err != nil {
		writeError(w, http.StatusBadRequest, "invalid request body")
		return
	}

	// Langfuse configuration requires updating environment variables
	// and potentially restarting the proxy
	result := ActionResponse{
		Success: false,
		Message: "Langfuse configuration requires proxy restart. Please restart the worker with appropriate environment variables.",
		Error:   "automated langfuse config not yet implemented",
	}

	// Log the attempt
	s.state.AddLog(LogEntry{
		Timestamp: time.Now(),
		Level:     LogLevelInfo,
		Component: "proxy",
		Message:   "Langfuse configuration update attempted",
	})

	writeJSON(w, result)
}

// getProxyStatus constructs the current proxy status
func (s *Server) getProxyStatus() ProxyStatus {
	status := ProxyStatus{
		Running:      false,
		HealthStatus: "unknown",
		LogFile:      filepath.Join(s.config.WorkerDir, "litellm_proxy.log"),
	}

	// Check if proxy is configured
	if !s.config.EnableLocalProxy {
		status.HealthStatus = "disabled"
		return status
	}

	// Check if we have a proxy reference
	if proxyRef != nil {
		status.Running = true
		status.BaseURL = proxyRef.GetBaseURL()

		// Parse port from URL
		if parts := strings.Split(status.BaseURL, ":"); len(parts) == 3 {
			if port, err := parseIntParam(parts[2]); err == nil {
				status.Port = port
			}
		}
	}

	// Check if proxy port is configured
	if s.config.ProxyPort > 0 {
		status.Port = s.config.ProxyPort
		status.BaseURL = "http://127.0.0.1:" + string(rune(s.config.ProxyPort))
	}

	// Check health by hitting the proxy health endpoint
	if status.Port > 0 {
		status.HealthStatus = s.checkProxyHealth(status.Port)
		if status.HealthStatus == "healthy" {
			status.Running = true
		}
	}

	// Read config to get models
	configPath := filepath.Join(s.config.WorkerDir, "litellm_config.yaml")
	if _, err := os.Stat(configPath); err == nil {
		status.ConfigPath = configPath
		status.Models = s.readProxyModels(configPath)
	}

	return status
}

// checkProxyHealth checks if the proxy is responding
func (s *Server) checkProxyHealth(port int) string {
	client := &http.Client{Timeout: 2 * time.Second}
	resp, err := client.Get("http://127.0.0.1:" + string(rune(port)) + "/health/readiness")
	if err != nil {
		return "unhealthy"
	}
	defer resp.Body.Close()

	if resp.StatusCode == http.StatusOK {
		return "healthy"
	}
	return "unhealthy"
}

// readProxyModels reads model names from the config file
func (s *Server) readProxyModels(configPath string) []string {
	content, err := os.ReadFile(configPath)
	if err != nil {
		return nil
	}

	// Simple YAML parsing for model names
	var models []string
	lines := strings.Split(string(content), "\n")
	inModelList := false

	for _, line := range lines {
		trimmed := strings.TrimSpace(line)
		if strings.HasPrefix(trimmed, "model_list:") {
			inModelList = true
			continue
		}
		if inModelList && strings.HasPrefix(trimmed, "- model_name:") {
			modelName := strings.TrimPrefix(trimmed, "- model_name:")
			modelName = strings.TrimSpace(modelName)
			modelName = strings.Trim(modelName, "\"'")
			if modelName != "" {
				models = append(models, modelName)
			}
		}
		// Exit model list when hitting a new top-level key
		if inModelList && !strings.HasPrefix(line, " ") && !strings.HasPrefix(line, "-") && trimmed != "" {
			break
		}
	}

	return models
}

// parseIntParam parses a string parameter to int
func parseIntParam(s string) (int, error) {
	var n int
	_, err := parseIntFromString(s, &n)
	return n, err
}

// parseIntFromString is a helper for parsing integers
func parseIntFromString(s string, n *int) (int, error) {
	var result int
	for _, c := range s {
		if c < '0' || c > '9' {
			break
		}
		result = result*10 + int(c-'0')
	}
	*n = result
	return result, nil
}
