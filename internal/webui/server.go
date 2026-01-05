package webui

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"io/fs"
	"net"
	"net/http"
	"strings"
	"sync"
	"time"
)

// ServerConfig contains configuration for the WebUI server
type ServerConfig struct {
	// QueueID is the worker queue ID
	QueueID string

	// WorkerDir is the directory for worker files
	WorkerDir string

	// ControlPlaneURL is the control plane API URL
	ControlPlaneURL string

	// APIKey is the API key for control plane authentication
	APIKey string

	// Port is the port to listen on (0 for auto-assign)
	Port int

	// WorkerPID is the PID of the worker process to monitor
	WorkerPID int

	// EnableLocalProxy indicates if LiteLLM proxy is enabled
	EnableLocalProxy bool

	// ProxyPort is the port of the local LiteLLM proxy
	ProxyPort int

	// ModelOverride is the explicit model ID if set
	ModelOverride string

	// DeploymentType is the worker deployment type (local/docker)
	DeploymentType string

	// DaemonMode indicates if worker is running as daemon
	DaemonMode bool

	// Version info for UI display
	Version     string
	BuildCommit string
	BuildDate   string
	GoVersion   string
	OS          string
	Arch        string
}

// Server is the WebUI HTTP server
type Server struct {
	config     ServerConfig
	httpServer *http.Server
	state      *State
	collector  *MetricsCollector
	listener   net.Listener
	wg         sync.WaitGroup
	stopCh     chan struct{}
	startTime  time.Time

	// Log capture
	logWriter *LogCaptureWriter

	// Control plane client for direct API access
	cpClient *cpClient
}

// cpClient wraps the control plane client interface we need
type cpClient struct {
	apiKey  string
	baseURL string
}

// LogCaptureWriter captures output and sends it to the WebUI state
type LogCaptureWriter struct {
	state     *State
	workerID  string
	component string
	level     LogLevel
	buffer    []byte
	mu        sync.Mutex
}

// NewLogCaptureWriter creates a new log capture writer
func NewLogCaptureWriter(state *State, workerID, component string, level LogLevel) *LogCaptureWriter {
	return &LogCaptureWriter{
		state:     state,
		workerID:  workerID,
		component: component,
		level:     level,
		buffer:    make([]byte, 0, 4096),
	}
}

// Write implements io.Writer
func (w *LogCaptureWriter) Write(p []byte) (n int, err error) {
	w.mu.Lock()
	defer w.mu.Unlock()

	w.buffer = append(w.buffer, p...)

	// Process complete lines
	for {
		idx := -1
		for i, b := range w.buffer {
			if b == '\n' {
				idx = i
				break
			}
		}
		if idx == -1 {
			break
		}

		line := string(w.buffer[:idx])
		w.buffer = w.buffer[idx+1:]

		if line == "" {
			continue
		}

		// Parse log level from line if present
		level := w.level
		if strings.Contains(line, "ERROR") || strings.Contains(line, "error") {
			level = LogLevelError
		} else if strings.Contains(line, "WARNING") || strings.Contains(line, "WARN") || strings.Contains(line, "warning") {
			level = LogLevelWarning
		} else if strings.Contains(line, "DEBUG") || strings.Contains(line, "debug") {
			level = LogLevelDebug
		}

		entry := LogEntry{
			Timestamp: time.Now(),
			Level:     level,
			Component: w.component,
			Message:   line,
			WorkerID:  w.workerID,
		}

		w.state.AddLog(entry)
	}

	return len(p), nil
}

// NewServer creates a new WebUI server
func NewServer(config ServerConfig) (*Server, error) {
	state := NewState()

	// Set initial configuration in state
	state.SetConfig(WorkerConfig{
		QueueID:          config.QueueID,
		DeploymentType:   config.DeploymentType,
		ControlPlaneURL:  config.ControlPlaneURL,
		EnableLocalProxy: config.EnableLocalProxy,
		ProxyPort:        config.ProxyPort,
		ModelOverride:    config.ModelOverride,
		DaemonMode:       config.DaemonMode,
		WorkerDir:        config.WorkerDir,
		Version:          config.Version,
		BuildCommit:      config.BuildCommit,
		BuildDate:        config.BuildDate,
		GoVersion:        config.GoVersion,
		OS:               config.OS,
		Arch:             config.Arch,
	})

	// Initialize control plane client if we have credentials
	var client *cpClient
	if config.APIKey != "" && config.ControlPlaneURL != "" {
		client = &cpClient{
			apiKey:  config.APIKey,
			baseURL: config.ControlPlaneURL,
		}
	}

	return &Server{
		config:    config,
		state:     state,
		stopCh:    make(chan struct{}),
		startTime: time.Now(),
		cpClient:  client,
	}, nil
}

// Start starts the WebUI server
func (s *Server) Start(ctx context.Context) error {
	// Find available port
	var err error
	addr := fmt.Sprintf("127.0.0.1:%d", s.config.Port)
	s.listener, err = net.Listen("tcp", addr)
	if err != nil {
		return fmt.Errorf("failed to listen on %s: %w", addr, err)
	}

	// Start metrics collector if we have a worker PID
	if s.config.WorkerPID > 0 {
		s.collector = NewMetricsCollector(s.state, s.config.WorkerPID, s.config.WorkerDir)
		s.wg.Add(1)
		go func() {
			defer s.wg.Done()
			s.collector.Run(ctx, s.stopCh)
		}()
	}

	// Start control plane health checker
	s.wg.Add(1)
	go func() {
		defer s.wg.Done()
		s.runControlPlaneChecker(ctx)
	}()

	// Start periodic overview broadcaster
	s.wg.Add(1)
	go func() {
		defer s.wg.Done()
		s.runOverviewBroadcaster(ctx)
	}()

	// Setup HTTP routes
	mux := http.NewServeMux()
	s.setupRoutes(mux)

	s.httpServer = &http.Server{
		Handler:      mux,
		ReadTimeout:  10 * time.Second,
		WriteTimeout: 0, // Allow SSE streaming (no timeout)
		IdleTimeout:  120 * time.Second,
	}

	// Start HTTP server
	s.wg.Add(1)
	go func() {
		defer s.wg.Done()
		if err := s.httpServer.Serve(s.listener); err != http.ErrServerClosed {
			// Log error but don't crash - worker should continue
			s.state.AddLog(LogEntry{
				Timestamp: time.Now(),
				Level:     LogLevelError,
				Component: "webui",
				Message:   fmt.Sprintf("HTTP server error: %v", err),
			})
		}
	}()

	// Add initial worker entry
	s.addInitialWorker()

	return nil
}

// setupRoutes configures all HTTP routes
func (s *Server) setupRoutes(mux *http.ServeMux) {
	// Static files (embedded)
	staticSubFS, err := fs.Sub(staticFS, "static")
	if err != nil {
		// Fallback to a simple handler if embed fails
		mux.HandleFunc("/", s.handleIndex)
	} else {
		fileServer := http.FileServer(http.FS(staticSubFS))
		mux.Handle("/", s.handleStatic(fileServer))
	}

	// API endpoints
	mux.HandleFunc("/api/health", s.handleHealth)
	mux.HandleFunc("/api/overview", s.handleOverview)
	mux.HandleFunc("/api/workers", s.handleWorkers)
	mux.HandleFunc("/api/workers/", s.handleWorkerByID)
	mux.HandleFunc("/api/control-plane", s.handleControlPlane)
	mux.HandleFunc("/api/control-plane/reconnect", s.handleControlPlaneReconnect)
	mux.HandleFunc("/api/metrics", s.handleMetrics)
	mux.HandleFunc("/api/sessions", s.handleSessions)
	mux.HandleFunc("/api/sessions/", s.handleSessionByID)
	mux.HandleFunc("/api/config", s.handleConfig)
	mux.HandleFunc("/api/logs", s.handleLogs)
	mux.HandleFunc("/api/activity", s.handleActivity)

	// SSE endpoints
	mux.HandleFunc("/api/events", s.handleSSE)
	mux.HandleFunc("/api/logs/stream", s.handleLogStream)

	// Doctor/Diagnostics endpoints
	mux.HandleFunc("/api/doctor", s.handleDoctor)
	mux.HandleFunc("/api/doctor/", s.handleDoctorCategory)
	mux.HandleFunc("/api/doctor/fix/", s.handleDoctorFix)
	mux.HandleFunc("/api/doctor/stream", s.handleDoctorStream)

	// Proxy control endpoints
	mux.HandleFunc("/api/proxy/status", s.handleProxyStatus)
	mux.HandleFunc("/api/proxy/control", s.handleProxyControl)
	mux.HandleFunc("/api/proxy/logs", s.handleProxyLogs)
	mux.HandleFunc("/api/proxy/logs/stream", s.handleProxyLogsStream)
	mux.HandleFunc("/api/proxy/langfuse", s.handleProxyLangfuse)

	// LLM insights endpoints
	mux.HandleFunc("/api/llm/models", s.handleLLMModels)
	mux.HandleFunc("/api/llm/providers", s.handleLLMProviders)
	mux.HandleFunc("/api/llm/insights", s.handleLLMInsights)
	mux.HandleFunc("/api/llm/default", s.handleLLMDefault)
	mux.HandleFunc("/api/llm/test", s.handleLLMTest)

	// Environment variables endpoints
	mux.HandleFunc("/api/env", s.handleEnvList)
	mux.HandleFunc("/api/env/save", s.handleEnvSave)
	mux.HandleFunc("/api/env/reload", s.handleEnvReload)

	// Execution playground endpoints (CLI-based, for local mode)
	mux.HandleFunc("/api/exec/agents", s.handleExecAgents)
	mux.HandleFunc("/api/exec/teams", s.handleExecTeams)
	mux.HandleFunc("/api/exec/environments", s.handleExecEnvironments)
	mux.HandleFunc("/api/exec/start", s.handleExecStart)
	mux.HandleFunc("/api/exec/stream/", s.handleExecStream)
	mux.HandleFunc("/api/exec/stop/", s.handleExecStop)

	// Direct execution endpoints (control plane API-based, for reliable streaming)
	mux.HandleFunc("/api/exec/direct/start", s.handleDirectExecStart)
	mux.HandleFunc("/api/exec/direct/stream/", s.handleDirectExecStream)
	mux.HandleFunc("/api/exec/direct/stop/", s.handleDirectExecStop)

	// Chat session endpoints
	mux.HandleFunc("/api/chat/start", s.handleChatStart)
	mux.HandleFunc("/api/chat/stream/", s.handleChatStream)
	mux.HandleFunc("/api/chat/send", s.handleChatSend)
	mux.HandleFunc("/api/chat/end/", s.handleChatEnd)
}

// handleStatic wraps the file server to handle SPA routing
func (s *Server) handleStatic(fileServer http.Handler) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		// For API requests, return 404
		if strings.HasPrefix(r.URL.Path, "/api/") {
			http.NotFound(w, r)
			return
		}

		// Try to serve the file
		fileServer.ServeHTTP(w, r)
	}
}

// handleIndex serves the index page (fallback if embed fails)
func (s *Server) handleIndex(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "text/html")
	fmt.Fprintf(w, `<!DOCTYPE html>
<html>
<head>
    <title>Kubiya Worker Pool</title>
    <style>
        body {
            font-family: -apple-system, BlinkMacSystemFont, 'Segoe UI', Roboto, sans-serif;
            background: #1a1a2e;
            color: #eee;
            margin: 0;
            padding: 20px;
        }
        .container { max-width: 800px; margin: 0 auto; }
        h1 { color: #4ecdc4; }
        .card {
            background: #16213e;
            border-radius: 8px;
            padding: 20px;
            margin: 10px 0;
        }
        .status { color: #4ecdc4; }
        a { color: #4ecdc4; }
    </style>
</head>
<body>
    <div class="container">
        <h1>Kubiya Worker Pool</h1>
        <div class="card">
            <h2>Queue ID: %s</h2>
            <p class="status">Worker is running</p>
            <p>API endpoints available at <code>/api/*</code></p>
        </div>
        <div class="card">
            <h3>Available Endpoints</h3>
            <ul>
                <li><a href="/api/health">/api/health</a> - Health check</li>
                <li><a href="/api/overview">/api/overview</a> - Overview data</li>
                <li><a href="/api/workers">/api/workers</a> - Worker list</li>
                <li><a href="/api/control-plane">/api/control-plane</a> - Control plane status</li>
                <li><a href="/api/config">/api/config</a> - Configuration</li>
            </ul>
        </div>
    </div>
</body>
</html>`, s.config.QueueID)
}

// URL returns the server URL
func (s *Server) URL() string {
	if s.listener == nil {
		return ""
	}
	return fmt.Sprintf("http://%s", s.listener.Addr().String())
}

// Port returns the actual port the server is listening on
func (s *Server) Port() int {
	if s.listener == nil {
		return 0
	}
	addr := s.listener.Addr().(*net.TCPAddr)
	return addr.Port
}

// Stop gracefully stops the server
func (s *Server) Stop() error {
	close(s.stopCh)

	// Stop metrics collector
	if s.collector != nil {
		s.collector.Stop()
	}

	// Shutdown HTTP server with timeout
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	if err := s.httpServer.Shutdown(ctx); err != nil {
		return fmt.Errorf("server shutdown error: %w", err)
	}

	// Wait for goroutines
	s.wg.Wait()
	return nil
}

// State returns the server's state (for external updates)
func (s *Server) State() *State {
	return s.state
}

// GetLogWriter returns a log capture writer for worker stdout/stderr
// The caller should create separate writers for stdout and stderr
func (s *Server) GetLogWriter(component string, level LogLevel) *LogCaptureWriter {
	workerID := fmt.Sprintf("worker-%d", s.config.WorkerPID)
	return NewLogCaptureWriter(s.state, workerID, component, level)
}

// CreateMultiWriter creates an io.Writer that writes to both the log capture and original output
func CreateMultiWriter(original io.Writer, capture *LogCaptureWriter) io.Writer {
	if original == nil {
		return capture
	}
	return io.MultiWriter(original, capture)
}

// SetWorkerPID updates the worker PID after the process starts
func (s *Server) SetWorkerPID(pid int) {
	s.config.WorkerPID = pid
	// Update metrics collector with actual PID
	if s.collector != nil {
		s.collector.SetPID(pid)
	}
	// Update worker in state
	workerID := fmt.Sprintf("worker-%d", pid)
	worker := &WorkerInfo{
		ID:            workerID,
		QueueID:       s.config.QueueID,
		Status:        WorkerStatusRunning,
		PID:           pid,
		StartedAt:     s.startTime,
		LastHeartbeat: time.Now(),
		Hostname:      "localhost",
	}
	s.state.AddWorker(worker)
}

// addInitialWorker adds the current worker to state
func (s *Server) addInitialWorker() {
	hostname := "localhost"
	// Try to get actual hostname
	if h, err := getHostname(); err == nil {
		hostname = h
	}

	worker := &WorkerInfo{
		ID:            fmt.Sprintf("worker-%d", s.config.WorkerPID),
		QueueID:       s.config.QueueID,
		Status:        WorkerStatusRunning,
		PID:           s.config.WorkerPID,
		StartedAt:     s.startTime,
		LastHeartbeat: time.Now(),
		Hostname:      hostname,
	}

	s.state.AddWorker(worker)

	// Add initial activity
	s.state.AddActivity(RecentActivity{
		Type:        "worker_started",
		Description: fmt.Sprintf("Worker started (PID: %d)", s.config.WorkerPID),
		Timestamp:   s.startTime,
		WorkerID:    worker.ID,
	})
}

// runControlPlaneChecker periodically checks control plane connectivity
func (s *Server) runControlPlaneChecker(ctx context.Context) {
	ticker := time.NewTicker(30 * time.Second)
	defer ticker.Stop()

	// Initial check
	s.checkControlPlane()

	for {
		select {
		case <-ctx.Done():
			return
		case <-s.stopCh:
			return
		case <-ticker.C:
			s.checkControlPlane()
		}
	}
}

// checkControlPlane performs a control plane health check
func (s *Server) checkControlPlane() {
	start := time.Now()
	status := ControlPlaneStatus{
		URL:       s.config.ControlPlaneURL,
		LastCheck: start,
	}

	// Try the API health endpoint with authentication
	client := &http.Client{Timeout: 5 * time.Second}

	// Try /api/health first (the actual control plane endpoint)
	req, err := http.NewRequest("GET", s.config.ControlPlaneURL+"/api/health", nil)
	if err != nil {
		status.Connected = false
		status.ErrorMessage = err.Error()
		status.AuthStatus = "unknown"
		s.state.SetControlPlane(status)
		return
	}

	// Add auth header if we have an API key
	if s.config.APIKey != "" {
		req.Header.Set("Authorization", "UserKey "+s.config.APIKey)
	}

	resp, err := client.Do(req)
	if err != nil {
		status.Connected = false
		status.ErrorMessage = err.Error()
		status.AuthStatus = "unknown"
	} else {
		resp.Body.Close()
		status.Latency = time.Since(start)
		status.LatencyMS = status.Latency.Milliseconds()

		if resp.StatusCode == 401 || resp.StatusCode == 403 {
			status.Connected = false
			status.AuthStatus = "expired"
			status.ErrorMessage = "Authentication failed"
		} else if resp.StatusCode < 400 {
			status.Connected = true
			status.AuthStatus = "valid"
			status.LastSuccess = time.Now()
		} else {
			status.Connected = false
			status.AuthStatus = "error"
			status.ErrorMessage = fmt.Sprintf("HTTP %d", resp.StatusCode)
		}
	}

	s.state.SetControlPlane(status)
}

// runOverviewBroadcaster periodically broadcasts overview updates
func (s *Server) runOverviewBroadcaster(ctx context.Context) {
	ticker := time.NewTicker(5 * time.Second)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			return
		case <-s.stopCh:
			return
		case <-ticker.C:
			s.state.BroadcastOverview()
		}
	}
}

// Helper functions

func getHostname() (string, error) {
	// Import os dynamically would be nice, but let's just return localhost for now
	// The actual implementation should use os.Hostname()
	return "localhost", nil
}

// JSON response helper
func writeJSON(w http.ResponseWriter, data interface{}) {
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(data)
}

// Error response helper
func writeError(w http.ResponseWriter, status int, message string) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	json.NewEncoder(w).Encode(map[string]string{"error": message})
}
