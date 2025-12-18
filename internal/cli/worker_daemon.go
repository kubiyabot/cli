package cli

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"sync"
	"syscall"
	"time"

	"github.com/kubiyabot/cli/internal/controlplane"
)

const (
	defaultMaxLogSize   = 100 * 1024 * 1024 // 100MB
	defaultMaxBackups   = 5                  // Keep 5 backup log files
	maxRestartAttempts  = 5                  // Max restart attempts before giving up
	restartBackoffBase  = 2 * time.Second    // Base backoff duration
	restartBackoffMax   = 5 * time.Minute    // Max backoff duration
	healthCheckInterval = 30 * time.Second   // Health check interval
)

// DaemonInfo contains information about a running daemon worker
type DaemonInfo struct {
	PID            int       `json:"pid"`
	QueueID        string    `json:"queue_id"`
	WorkerDir      string    `json:"worker_dir"`
	LogFile        string    `json:"log_file"`
	PIDFile        string    `json:"pid_file"`
	StartedAt      time.Time `json:"started_at"`
	DeploymentType string    `json:"deployment_type"`
}

// RotatingLogWriter implements a rotating log file writer with size limits
type RotatingLogWriter struct {
	filename    string
	maxSize     int64
	maxBackups  int
	currentSize int64
	file        *os.File
	mu          sync.Mutex
}

// NewRotatingLogWriter creates a new rotating log writer
func NewRotatingLogWriter(filename string, maxSize int64, maxBackups int) (*RotatingLogWriter, error) {
	w := &RotatingLogWriter{
		filename:   filename,
		maxSize:    maxSize,
		maxBackups: maxBackups,
	}

	// Open initial file
	if err := w.openFile(); err != nil {
		return nil, err
	}

	return w, nil
}

func (w *RotatingLogWriter) openFile() error {
	// Get file size if it exists
	info, err := os.Stat(w.filename)
	if err == nil {
		w.currentSize = info.Size()
	}

	// Open file for appending
	f, err := os.OpenFile(w.filename, os.O_CREATE|os.O_APPEND|os.O_WRONLY, 0644)
	if err != nil {
		return fmt.Errorf("failed to open log file: %w", err)
	}

	w.file = f
	return nil
}

func (w *RotatingLogWriter) Write(p []byte) (n int, err error) {
	w.mu.Lock()
	defer w.mu.Unlock()

	// Check if we need to rotate
	if w.currentSize+int64(len(p)) > w.maxSize {
		if err := w.rotate(); err != nil {
			return 0, err
		}
	}

	// Write to file
	n, err = w.file.Write(p)
	w.currentSize += int64(n)
	return n, err
}

func (w *RotatingLogWriter) rotate() error {
	// Close current file
	if err := w.file.Close(); err != nil {
		return err
	}

	// Rotate backup files
	for i := w.maxBackups - 1; i >= 0; i-- {
		var src, dst string
		if i == 0 {
			src = w.filename
		} else {
			src = fmt.Sprintf("%s.%d", w.filename, i)
		}
		dst = fmt.Sprintf("%s.%d", w.filename, i+1)

		// Remove oldest backup
		if i == w.maxBackups-1 {
			os.Remove(dst)
		}

		// Rename
		if _, err := os.Stat(src); err == nil {
			os.Rename(src, dst)
		}
	}

	// Reset size and open new file
	w.currentSize = 0
	return w.openFile()
}

func (w *RotatingLogWriter) Close() error {
	w.mu.Lock()
	defer w.mu.Unlock()
	if w.file != nil {
		return w.file.Close()
	}
	return nil
}

// ProcessSupervisor manages a worker process with crash recovery
type ProcessSupervisor struct {
	queueID           string
	workerDir         string
	logWriter         *RotatingLogWriter
	daemonInfo        *DaemonInfo
	stopChan          chan struct{}
	restartCount      int
	lastRestart       time.Time
	mu                sync.Mutex
	liteLLMSupervisor *LiteLLMProxySupervisor
}

// NewProcessSupervisor creates a new process supervisor
func NewProcessSupervisor(queueID, workerDir string, maxLogSize int64, maxBackups int) (*ProcessSupervisor, error) {
	// Create log file
	logFile := filepath.Join(workerDir, "worker.log")
	logWriter, err := NewRotatingLogWriter(logFile, maxLogSize, maxBackups)
	if err != nil {
		return nil, fmt.Errorf("failed to create log writer: %w", err)
	}

	return &ProcessSupervisor{
		queueID:   queueID,
		workerDir: workerDir,
		logWriter: logWriter,
		stopChan:  make(chan struct{}),
	}, nil
}

// Start starts the supervised process in the background
func (s *ProcessSupervisor) Start(pythonPath, workerPyPath, apiKey string) (*DaemonInfo, error) {
	// Create daemon info
	pidFile := filepath.Join(s.workerDir, "worker.pid")
	daemonInfo := &DaemonInfo{
		PID:            os.Getpid(),
		QueueID:        s.queueID,
		WorkerDir:      s.workerDir,
		LogFile:        filepath.Join(s.workerDir, "worker.log"),
		PIDFile:        pidFile,
		StartedAt:      time.Now(),
		DeploymentType: "local",
	}
	s.daemonInfo = daemonInfo

	// Write PID file
	if err := s.writePIDFile(pidFile, daemonInfo); err != nil {
		return nil, err
	}

	// Start supervision loop in background
	go s.supervisionLoop(pythonPath, workerPyPath, apiKey)

	return daemonInfo, nil
}

func (s *ProcessSupervisor) supervisionLoop(pythonPath, workerPyPath, apiKey string) {
	defer s.logWriter.Close()

	for {
		select {
		case <-s.stopChan:
			s.log("Supervisor received stop signal, exiting...")
			return
		default:
			// Calculate backoff
			backoff := s.calculateBackoff()
			if backoff > 0 {
				s.log("Waiting %v before restarting (attempt %d/%d)...", backoff, s.restartCount, maxRestartAttempts)
				time.Sleep(backoff)
			}

			// Check if we've exceeded max restart attempts
			if s.restartCount >= maxRestartAttempts {
				// Reset counter after cooldown period
				if time.Since(s.lastRestart) > 10*time.Minute {
					s.log("Cooldown period elapsed, resetting restart counter")
					s.mu.Lock()
					s.restartCount = 0
					s.mu.Unlock()
				} else {
					s.log("Max restart attempts (%d) exceeded, stopping supervisor", maxRestartAttempts)
					return
				}
			}

			// Start worker process
			s.log("Starting worker process (attempt %d)...", s.restartCount+1)
			err := s.runWorker(pythonPath, workerPyPath, apiKey)

			s.mu.Lock()
			s.restartCount++
			s.lastRestart = time.Now()
			s.mu.Unlock()

			if err != nil {
				s.log("Worker process exited with error: %v", err)
			} else {
				s.log("Worker process exited normally")
			}
		}
	}
}

func (s *ProcessSupervisor) runWorker(pythonPath, workerPyPath, apiKey string) error {
	// Get control plane URL from environment with priority:
	// 1. KUBIYA_CONTROL_PLANE_BASE_URL
	// 2. CONTROL_PLANE_GATEWAY_URL
	// 3. CONTROL_PLANE_URL
	// 4. Default
	controlPlaneURL := os.Getenv("KUBIYA_CONTROL_PLANE_BASE_URL")
	if controlPlaneURL == "" {
		controlPlaneURL = os.Getenv("CONTROL_PLANE_GATEWAY_URL")
	}
	if controlPlaneURL == "" {
		controlPlaneURL = os.Getenv("CONTROL_PLANE_URL")
	}
	if controlPlaneURL == "" {
		controlPlaneURL = "https://control-plane.kubiya.ai"
	}

	// Setup local LiteLLM proxy if enabled
	var liteLLMProxyURL string
	controlPlaneClient, err := controlplane.New(apiKey, false)
	if err == nil {
		queueConfig, err := controlPlaneClient.GetWorkerQueueConfig(s.queueID)
		if err == nil && queueConfig.Settings != nil && IsLocalProxyEnabled(queueConfig.Settings) {
			s.log("Local LiteLLM proxy enabled in queue config")

			// Parse config
			proxyConfig, err := ParseLiteLLMConfigFromSettings(queueConfig.Settings)
			if err == nil {
				// Get timeout settings
				timeoutSeconds, maxRetries := GetProxyTimeoutSettings(queueConfig.Settings)

				// Create supervisor
				liteLLMSupervisor, err := NewLiteLLMProxySupervisor(
					s.queueID,
					s.workerDir,
					proxyConfig,
					timeoutSeconds,
					maxRetries,
				)
				if err == nil {
					// Start proxy
					ctx := context.Background()
					proxyInfo, err := liteLLMSupervisor.Start(ctx)
					if err == nil && liteLLMSupervisor.WaitReady(ctx) == nil {
						liteLLMProxyURL = proxyInfo.BaseURL
						s.log("Local LiteLLM proxy started at %s (PID: %d)", liteLLMProxyURL, proxyInfo.PID)

						// Store supervisor reference for cleanup
						s.mu.Lock()
						s.liteLLMSupervisor = liteLLMSupervisor
						s.mu.Unlock()
					} else {
						s.log("LiteLLM proxy failed to start: %v", err)
					}
				} else {
					s.log("Failed to create LiteLLM proxy supervisor: %v", err)
				}
			} else {
				s.log("Failed to parse LiteLLM config: %v", err)
			}
		}
	}

	// Create command
	// Pass both env vars (safer) and CLI args (fallback)
	cmd := exec.Command(
		pythonPath,
		workerPyPath,
		"--queue-id", s.queueID,
		"--api-key", apiKey,
		"--control-plane-url", controlPlaneURL,
	)

	// Set environment variables - take precedence over CLI args
	envVars := []string{
		fmt.Sprintf("QUEUE_ID=%s", s.queueID),
		fmt.Sprintf("KUBIYA_API_KEY=%s", apiKey),
		fmt.Sprintf("CONTROL_PLANE_URL=%s", controlPlaneURL),
		"LOG_LEVEL=INFO",
	}

	// Add LiteLLM proxy env vars if available
	if liteLLMProxyURL != "" {
		envVars = append(envVars,
			fmt.Sprintf("LITELLM_API_BASE=%s", liteLLMProxyURL),
			"LITELLM_API_KEY=dummy-key",
		)
		s.log("Injected local LiteLLM proxy env vars: %s", liteLLMProxyURL)
	}

	cmd.Env = append(os.Environ(), envVars...)

	// Redirect output to log writer
	multiWriter := io.MultiWriter(s.logWriter)
	cmd.Stdout = multiWriter
	cmd.Stderr = multiWriter

	// Start process
	if err := cmd.Start(); err != nil {
		return fmt.Errorf("failed to start worker: %w", err)
	}

	s.log("Worker process started with PID %d", cmd.Process.Pid)

	// Wait for process to exit
	err = cmd.Wait()

	// Check exit status
	if err != nil {
		if exitErr, ok := err.(*exec.ExitError); ok {
			status := exitErr.ExitCode()
			s.log("Worker process exited with status %d", status)
		}
	}

	return err
}

func (s *ProcessSupervisor) calculateBackoff() time.Duration {
	s.mu.Lock()
	defer s.mu.Unlock()

	if s.restartCount == 0 {
		return 0
	}

	// Exponential backoff: 2s, 4s, 8s, 16s, 32s...
	backoff := restartBackoffBase * time.Duration(1<<uint(s.restartCount-1))
	if backoff > restartBackoffMax {
		backoff = restartBackoffMax
	}

	return backoff
}

func (s *ProcessSupervisor) log(format string, args ...interface{}) {
	timestamp := time.Now().Format("2006-01-02 15:04:05")
	message := fmt.Sprintf(format, args...)
	logLine := fmt.Sprintf("[%s] [SUPERVISOR] %s\n", timestamp, message)
	s.logWriter.Write([]byte(logLine))
}

func (s *ProcessSupervisor) Stop() {
	// Stop LiteLLM proxy if running
	s.mu.Lock()
	if s.liteLLMSupervisor != nil {
		s.log("Stopping local LiteLLM proxy")
		if err := s.liteLLMSupervisor.Stop(); err != nil {
			s.log("Failed to stop LiteLLM proxy: %v", err)
		}
		s.liteLLMSupervisor = nil
	}
	s.mu.Unlock()

	close(s.stopChan)
}

func (s *ProcessSupervisor) writePIDFile(pidFile string, info *DaemonInfo) error {
	data, err := json.MarshalIndent(info, "", "  ")
	if err != nil {
		return fmt.Errorf("failed to marshal daemon info: %w", err)
	}

	if err := os.WriteFile(pidFile, data, 0644); err != nil {
		return fmt.Errorf("failed to write PID file: %w", err)
	}

	return nil
}

// ReadDaemonInfo reads daemon info from a PID file
func ReadDaemonInfo(pidFile string) (*DaemonInfo, error) {
	data, err := os.ReadFile(pidFile)
	if err != nil {
		return nil, fmt.Errorf("failed to read PID file: %w", err)
	}

	var info DaemonInfo
	if err := json.Unmarshal(data, &info); err != nil {
		return nil, fmt.Errorf("failed to parse PID file: %w", err)
	}

	return &info, nil
}

// IsProcessRunning checks if a process with the given PID is running
func IsProcessRunning(pid int) bool {
	process, err := os.FindProcess(pid)
	if err != nil {
		return false
	}

	// Send signal 0 to check if process exists
	err = process.Signal(syscall.Signal(0))
	return err == nil
}

// StopDaemon stops a running daemon worker
func StopDaemon(pidFile string) error {
	info, err := ReadDaemonInfo(pidFile)
	if err != nil {
		return err
	}

	if !IsProcessRunning(info.PID) {
		return fmt.Errorf("worker process (PID %d) is not running", info.PID)
	}

	// Send SIGTERM to gracefully stop
	process, err := os.FindProcess(info.PID)
	if err != nil {
		return fmt.Errorf("failed to find process: %w", err)
	}

	if err := process.Signal(syscall.SIGTERM); err != nil {
		return fmt.Errorf("failed to send SIGTERM: %w", err)
	}

	// Wait for process to exit (with timeout)
	timeout := time.After(30 * time.Second)
	ticker := time.NewTicker(500 * time.Millisecond)
	defer ticker.Stop()

	for {
		select {
		case <-timeout:
			// Force kill if it doesn't stop
			process.Signal(syscall.SIGKILL)
			return fmt.Errorf("worker did not stop gracefully, forced kill")
		case <-ticker.C:
			if !IsProcessRunning(info.PID) {
				// Clean up PID file
				os.Remove(pidFile)
				return nil
			}
		}
	}
}


// IsDaemonChild checks if this is the daemon child process
func IsDaemonChild() bool {
	return os.Getenv("KUBIYA_DAEMON_CHILD") == "1"
}

// GetMaxLogSizeFromEnv returns the configured max log size or default
func GetMaxLogSizeFromEnv() int64 {
	if sizeStr := os.Getenv("KUBIYA_MAX_LOG_SIZE"); sizeStr != "" {
		if size, err := strconv.ParseInt(sizeStr, 10, 64); err == nil && size > 0 {
			return size
		}
	}
	return defaultMaxLogSize
}
