package webui

import (
	"context"
	"encoding/json"
	"net/http"
	"strings"
	"time"
)

// handleDoctor handles GET /api/doctor - runs all diagnostic checks
func (s *Server) handleDoctor(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		writeError(w, http.StatusMethodNotAllowed, "method not allowed")
		return
	}

	// Create diagnostics runner
	runner := NewDiagnosticsRunner(
		s.config.WorkerDir,
		s.config.ControlPlaneURL,
		s.config.APIKey,
		s.config.WorkerPID,
	)

	// Create context with timeout
	ctx, cancel := context.WithTimeout(r.Context(), 60*time.Second)
	defer cancel()

	// Run all checks
	report := runner.RunAllChecks(ctx)

	// Log the diagnostic run
	s.state.AddLog(LogEntry{
		Timestamp: time.Now(),
		Level:     LogLevelInfo,
		Component: "doctor",
		Message:   "Diagnostic check completed: " + report.Overall,
	})

	// Broadcast diagnostic event
	s.state.Broadcast(SSEEvent{
		Type: SSEEventDiagnostic,
		Data: report,
	})

	writeJSON(w, report)
}

// handleDoctorCategory handles GET /api/doctor/{category} - runs checks for a specific category
func (s *Server) handleDoctorCategory(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		writeError(w, http.StatusMethodNotAllowed, "method not allowed")
		return
	}

	// Extract category from path
	path := strings.TrimPrefix(r.URL.Path, "/api/doctor/")
	if path == "" || path == "/" {
		// Redirect to full diagnostics
		s.handleDoctor(w, r)
		return
	}

	// Map path to category
	var category DiagnosticCategory
	switch strings.ToLower(path) {
	case "python":
		category = DiagnosticCategoryPython
	case "packages":
		category = DiagnosticCategoryPackages
	case "connectivity":
		category = DiagnosticCategoryConnectivity
	case "config":
		category = DiagnosticCategoryConfig
	case "process":
		category = DiagnosticCategoryProcess
	default:
		writeError(w, http.StatusBadRequest, "invalid category: "+path)
		return
	}

	// Create diagnostics runner
	runner := NewDiagnosticsRunner(
		s.config.WorkerDir,
		s.config.ControlPlaneURL,
		s.config.APIKey,
		s.config.WorkerPID,
	)

	// Create context with timeout
	ctx, cancel := context.WithTimeout(r.Context(), 30*time.Second)
	defer cancel()

	// Run category checks
	report := runner.RunCategoryChecks(ctx, category)

	writeJSON(w, report)
}

// handleDoctorFix handles POST /api/doctor/fix/{check} - attempts to fix a specific issue
func (s *Server) handleDoctorFix(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		writeError(w, http.StatusMethodNotAllowed, "method not allowed")
		return
	}

	// Extract check name from path
	path := strings.TrimPrefix(r.URL.Path, "/api/doctor/fix/")
	if path == "" {
		writeError(w, http.StatusBadRequest, "check name required")
		return
	}

	// Parse request body if any
	var request struct {
		Force bool `json:"force"`
	}
	if r.ContentLength > 0 {
		if err := json.NewDecoder(r.Body).Decode(&request); err != nil {
			writeError(w, http.StatusBadRequest, "invalid request body")
			return
		}
	}

	// Handle fix based on check name
	var result ActionResponse
	switch strings.ToLower(path) {
	case "venv", "virtualenv", "virtual-env":
		result = s.fixVirtualEnv(r.Context())
	case "packages", "worker-package":
		result = s.fixPackages(r.Context())
	case "pip":
		result = s.fixPip(r.Context())
	default:
		writeError(w, http.StatusBadRequest, "fix not available for: "+path)
		return
	}

	// Log the fix attempt
	s.state.AddLog(LogEntry{
		Timestamp: time.Now(),
		Level:     LogLevelInfo,
		Component: "doctor",
		Message:   "Fix attempted for: " + path + " - " + result.Message,
	})

	if result.Success {
		w.WriteHeader(http.StatusOK)
	} else {
		w.WriteHeader(http.StatusInternalServerError)
	}
	writeJSON(w, result)
}

// fixVirtualEnv attempts to recreate the virtual environment
func (s *Server) fixVirtualEnv(ctx context.Context) ActionResponse {
	// For now, just return a message - actual fix would require elevated operations
	return ActionResponse{
		Success: false,
		Message: "Virtual environment fix requires worker restart. Please restart the worker to recreate the venv.",
		Error:   "automated venv repair not yet implemented",
	}
}

// fixPackages attempts to reinstall missing packages
func (s *Server) fixPackages(ctx context.Context) ActionResponse {
	// For now, just return a message
	return ActionResponse{
		Success: false,
		Message: "Package fix requires worker restart. Please restart the worker with --no-cache flag.",
		Error:   "automated package repair not yet implemented",
	}
}

// fixPip attempts to reinstall/upgrade pip
func (s *Server) fixPip(ctx context.Context) ActionResponse {
	// For now, just return a message
	return ActionResponse{
		Success: false,
		Message: "Pip fix requires worker restart. Please restart the worker.",
		Error:   "automated pip repair not yet implemented",
	}
}

// handleDoctorStream handles GET /api/doctor/stream - streams diagnostic progress via SSE
func (s *Server) handleDoctorStream(w http.ResponseWriter, r *http.Request) {
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

	// Create diagnostics runner
	runner := NewDiagnosticsRunner(
		s.config.WorkerDir,
		s.config.ControlPlaneURL,
		s.config.APIKey,
		s.config.WorkerPID,
	)

	// Create progress channel
	progressCh := make(chan DiagnosticCheck, 20)

	// Run diagnostics in background and stream results
	go func() {
		defer close(progressCh)
		s.runDiagnosticsWithProgress(r.Context(), runner, progressCh)
	}()

	// Stream progress
	for check := range progressCh {
		select {
		case <-r.Context().Done():
			return
		default:
			event := SSEEvent{
				Type: SSEEventDiagnostic,
				Data: check,
			}
			s.sendSSEEvent(w, flusher, event)
		}
	}

	// Send final report
	ctx, cancel := context.WithTimeout(r.Context(), 60*time.Second)
	defer cancel()
	report := runner.RunAllChecks(ctx)

	finalEvent := SSEEvent{
		Type: "diagnostic_complete",
		Data: report,
	}
	s.sendSSEEvent(w, flusher, finalEvent)
}

// runDiagnosticsWithProgress runs diagnostics and sends progress to channel
func (s *Server) runDiagnosticsWithProgress(ctx context.Context, runner *DiagnosticsRunner, progressCh chan<- DiagnosticCheck) {
	// Run each check individually and send progress
	checks := []struct {
		name string
		fn   func(context.Context) DiagnosticCheck
	}{
		{"python_version", runner.checkPythonVersion},
		{"pip_available", runner.checkPipAvailable},
		{"virtual_env", runner.checkVirtualEnv},
		{"worker_package", runner.checkWorkerPackage},
		{"litellm_package", runner.checkLiteLLMPackage},
		{"langfuse_package", runner.checkLangfusePackage},
		{"control_plane", runner.checkControlPlaneConnectivity},
		{"api_key", runner.checkAPIKeyValidity},
		{"worker_process", runner.checkWorkerProcess},
		{"worker_directory", runner.checkWorkerDirectory},
	}

	for _, check := range checks {
		select {
		case <-ctx.Done():
			return
		default:
			result := check.fn(ctx)
			progressCh <- result
		}
	}
}
