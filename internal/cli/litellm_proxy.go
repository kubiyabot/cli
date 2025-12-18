package cli

import (
	"context"
	"encoding/json"
	"fmt"
	"net"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"sync"
	"time"

	"gopkg.in/yaml.v3"
)

// LiteLLMProxyConfig represents the configuration for local LiteLLM proxy
type LiteLLMProxyConfig struct {
	ModelList            []LiteLLMModel         `yaml:"model_list,omitempty"`
	LiteLLMSettings      map[string]interface{} `yaml:"litellm_settings,omitempty"`
	EnvironmentVariables map[string]string      `yaml:"environment_variables,omitempty"`
}

// LiteLLMModel represents a single model configuration
type LiteLLMModel struct {
	ModelName     string                 `yaml:"model_name"`
	LiteLLMParams map[string]interface{} `yaml:"litellm_params"`
}

// LiteLLMProxySupervisor manages the LiteLLM proxy subprocess
type LiteLLMProxySupervisor struct {
	queueID        string
	workerDir      string
	configPath     string
	host           string
	port           int
	cmd            *exec.Cmd
	stopChan       chan struct{}
	readyChan      chan struct{}
	mu             sync.Mutex
	healthCheckURL string
	timeoutSeconds int
	maxRetries     int
	logFile        *os.File
}

// LiteLLMProxyInfo contains runtime information about the proxy
type LiteLLMProxyInfo struct {
	Host       string    `json:"host"`
	Port       int       `json:"port"`
	BaseURL    string    `json:"base_url"`
	ConfigPath string    `json:"config_path"`
	PID        int       `json:"pid,omitempty"`
	StartedAt  time.Time `json:"started_at"`
	Status     string    `json:"status"` // "starting", "ready", "failed"
}

// NewLiteLLMProxySupervisor creates a new LiteLLM proxy supervisor
func NewLiteLLMProxySupervisor(
	queueID string,
	workerDir string,
	config *LiteLLMProxyConfig,
	timeoutSeconds int,
	maxRetries int,
) (*LiteLLMProxySupervisor, error) {
	// Write config to YAML file
	configPath := filepath.Join(workerDir, "litellm_config.yaml")
	if err := writeLiteLLMConfig(configPath, config); err != nil {
		return nil, fmt.Errorf("failed to write LiteLLM config: %w", err)
	}

	return &LiteLLMProxySupervisor{
		queueID:        queueID,
		workerDir:      workerDir,
		configPath:     configPath,
		host:           "127.0.0.1",
		port:           0, // Auto-assign
		stopChan:       make(chan struct{}),
		readyChan:      make(chan struct{}),
		timeoutSeconds: timeoutSeconds,
		maxRetries:     maxRetries,
	}, nil
}

// Start starts the LiteLLM proxy process
func (s *LiteLLMProxySupervisor) Start(ctx context.Context) (*LiteLLMProxyInfo, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	// Find available port
	port, err := findAvailablePort()
	if err != nil {
		return nil, fmt.Errorf("failed to find available port: %w", err)
	}
	s.port = port
	s.healthCheckURL = fmt.Sprintf("http://%s:%d/health/readiness", s.host, s.port)

	// Prepare LiteLLM command
	s.cmd = exec.Command(
		"litellm",
		"--config", s.configPath,
		"--host", s.host,
		"--port", fmt.Sprintf("%d", s.port),
	)

	// Set environment from current process
	s.cmd.Env = os.Environ()

	// Set working directory
	s.cmd.Dir = s.workerDir

	// Setup logging
	logFilePath := filepath.Join(s.workerDir, "litellm_proxy.log")
	logWriter, err := os.OpenFile(logFilePath, os.O_CREATE|os.O_APPEND|os.O_WRONLY, 0644)
	if err != nil {
		return nil, fmt.Errorf("failed to open log file: %w", err)
	}
	s.logFile = logWriter
	s.cmd.Stdout = logWriter
	s.cmd.Stderr = logWriter

	// Set process group for clean shutdown
	setupProcessGroup(s.cmd)

	// Start process
	if err := s.cmd.Start(); err != nil {
		logWriter.Close()
		return nil, fmt.Errorf("failed to start LiteLLM proxy: %w", err)
	}

	info := &LiteLLMProxyInfo{
		Host:       s.host,
		Port:       s.port,
		BaseURL:    fmt.Sprintf("http://%s:%d", s.host, s.port),
		ConfigPath: s.configPath,
		PID:        s.cmd.Process.Pid,
		StartedAt:  time.Now(),
		Status:     "starting",
	}

	// Wait for health check in background
	go s.waitForReady(ctx, info)

	return info, nil
}

// waitForReady polls health endpoint until ready or timeout
func (s *LiteLLMProxySupervisor) waitForReady(ctx context.Context, info *LiteLLMProxyInfo) {
	timeout := time.Duration(s.timeoutSeconds) * time.Second
	deadline := time.Now().Add(timeout)
	interval := 500 * time.Millisecond

	httpClient := &http.Client{
		Timeout: 2 * time.Second,
	}

	for time.Now().Before(deadline) {
		select {
		case <-ctx.Done():
			info.Status = "failed"
			return
		case <-s.stopChan:
			info.Status = "failed"
			return
		default:
			resp, err := httpClient.Get(s.healthCheckURL)
			if err == nil && resp.StatusCode == 200 {
				resp.Body.Close()
				info.Status = "ready"
				close(s.readyChan)
				return
			}
			if resp != nil {
				resp.Body.Close()
			}
			time.Sleep(interval)
		}
	}

	info.Status = "failed"
}

// WaitReady waits for proxy to be ready or timeout
func (s *LiteLLMProxySupervisor) WaitReady(ctx context.Context) error {
	timeout := time.Duration(s.timeoutSeconds) * time.Second
	timer := time.NewTimer(timeout)
	defer timer.Stop()

	select {
	case <-s.readyChan:
		return nil
	case <-timer.C:
		return fmt.Errorf("LiteLLM proxy did not become ready within %v", timeout)
	case <-ctx.Done():
		return ctx.Err()
	}
}

// Stop stops the LiteLLM proxy gracefully
func (s *LiteLLMProxySupervisor) Stop() error {
	s.mu.Lock()
	defer s.mu.Unlock()

	// Close stop channel if not already closed
	select {
	case <-s.stopChan:
		// Already closed
	default:
		close(s.stopChan)
	}

	if s.cmd == nil || s.cmd.Process == nil {
		if s.logFile != nil {
			s.logFile.Close()
		}
		return nil
	}

	// Send SIGTERM to process group
	pid := s.cmd.Process.Pid
	if err := killProcessGroup(pid, false); err != nil {
		// Fallback to direct kill
		if killErr := s.cmd.Process.Kill(); killErr != nil {
			if s.logFile != nil {
				s.logFile.Close()
			}
			return fmt.Errorf("failed to kill LiteLLM proxy: %w", killErr)
		}
	}

	// Wait for process to exit (with timeout)
	done := make(chan error, 1)
	go func() {
		done <- s.cmd.Wait()
	}()

	select {
	case <-time.After(5 * time.Second):
		// Force kill
		s.cmd.Process.Kill()
		if s.logFile != nil {
			s.logFile.Close()
		}
		return fmt.Errorf("LiteLLM proxy did not stop gracefully")
	case err := <-done:
		if s.logFile != nil {
			s.logFile.Close()
		}
		return err
	}
}

// GetBaseURL returns the proxy base URL
func (s *LiteLLMProxySupervisor) GetBaseURL() string {
	return fmt.Sprintf("http://%s:%d", s.host, s.port)
}

// Helper functions

// writeLiteLLMConfig writes the LiteLLM configuration to a YAML file
func writeLiteLLMConfig(path string, config *LiteLLMProxyConfig) error {
	data, err := yaml.Marshal(config)
	if err != nil {
		return fmt.Errorf("failed to marshal config: %w", err)
	}

	// Write with owner-only permissions for security
	if err := os.WriteFile(path, data, 0600); err != nil {
		return fmt.Errorf("failed to write config file: %w", err)
	}

	return nil
}

// findAvailablePort finds an available port by binding to port 0
func findAvailablePort() (int, error) {
	listener, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		return 0, err
	}
	defer listener.Close()

	addr := listener.Addr().(*net.TCPAddr)
	return addr.Port, nil
}

// ParseLiteLLMConfigFromSettings extracts LiteLLM config from WorkerQueueConfig.Settings
func ParseLiteLLMConfigFromSettings(settings map[string]interface{}) (*LiteLLMProxyConfig, error) {
	// Extract litellm_config map
	configMap, ok := settings["litellm_config"].(map[string]interface{})
	if !ok {
		return nil, fmt.Errorf("litellm_config not found or invalid type")
	}

	// Marshal to JSON then unmarshal to struct (handles type conversions)
	jsonData, err := json.Marshal(configMap)
	if err != nil {
		return nil, fmt.Errorf("failed to marshal config: %w", err)
	}

	var config LiteLLMProxyConfig
	if err := json.Unmarshal(jsonData, &config); err != nil {
		return nil, fmt.Errorf("failed to unmarshal config: %w", err)
	}

	return &config, nil
}

// IsLocalProxyEnabled checks if local proxy is enabled in settings
func IsLocalProxyEnabled(settings map[string]interface{}) bool {
	if settings == nil {
		return false
	}
	if enabled, ok := settings["enable_local_litellm_proxy"].(bool); ok {
		return enabled
	}
	return false
}

// LoadLiteLLMConfigFromFile loads LiteLLM config from a JSON or YAML file
func LoadLiteLLMConfigFromFile(filePath string) (*LiteLLMProxyConfig, error) {
	data, err := os.ReadFile(filePath)
	if err != nil {
		return nil, fmt.Errorf("failed to read config file: %w", err)
	}

	var config LiteLLMProxyConfig

	// Try JSON first
	if err := json.Unmarshal(data, &config); err == nil {
		return &config, nil
	}

	// Try YAML
	if err := yaml.Unmarshal(data, &config); err != nil {
		return nil, fmt.Errorf("failed to parse config as JSON or YAML: %w", err)
	}

	return &config, nil
}

// ParseLiteLLMConfigFromJSON parses LiteLLM config from a JSON string
func ParseLiteLLMConfigFromJSON(jsonStr string) (*LiteLLMProxyConfig, error) {
	var config LiteLLMProxyConfig
	if err := json.Unmarshal([]byte(jsonStr), &config); err != nil {
		return nil, fmt.Errorf("failed to parse JSON config: %w", err)
	}
	return &config, nil
}

// GetProxyTimeoutSettings extracts timeout configuration from settings
func GetProxyTimeoutSettings(settings map[string]interface{}) (timeoutSeconds int, maxRetries int) {
	// Defaults
	timeoutSeconds = 10
	maxRetries = 3

	if settings == nil {
		return
	}

	if val, ok := settings["local_proxy_timeout_seconds"].(float64); ok {
		timeoutSeconds = int(val)
	} else if val, ok := settings["local_proxy_timeout_seconds"].(int); ok {
		timeoutSeconds = val
	}

	if val, ok := settings["local_proxy_max_retries"].(float64); ok {
		maxRetries = int(val)
	} else if val, ok := settings["local_proxy_max_retries"].(int); ok {
		maxRetries = val
	}

	return
}
