package webui

import (
	"bufio"
	"context"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strconv"
	"strings"
	"sync"
	"time"
)

const (
	// MetricsCollectionInterval is how often to collect metrics
	MetricsCollectionInterval = 5 * time.Second

	// LogParseInterval is how often to parse new log entries
	LogParseInterval = 1 * time.Second
)

// MetricsCollector collects metrics from the worker process
type MetricsCollector struct {
	state     *State
	workerPID int
	workerDir string
	stopCh    chan struct{}
	wg        sync.WaitGroup

	// Log file tracking
	logFile      *os.File
	logOffset    int64
	lastLogParse time.Time
}

// NewMetricsCollector creates a new metrics collector
func NewMetricsCollector(state *State, workerPID int, workerDir string) *MetricsCollector {
	return &MetricsCollector{
		state:     state,
		workerPID: workerPID,
		workerDir: workerDir,
		stopCh:    make(chan struct{}),
	}
}

// Run starts the metrics collection loop
func (c *MetricsCollector) Run(ctx context.Context, stopCh chan struct{}) {
	metricsTicker := time.NewTicker(MetricsCollectionInterval)
	defer metricsTicker.Stop()

	logTicker := time.NewTicker(LogParseInterval)
	defer logTicker.Stop()

	// Initial collection
	c.collectMetrics()
	c.parseNewLogs()

	for {
		select {
		case <-ctx.Done():
			return
		case <-stopCh:
			return
		case <-c.stopCh:
			return
		case <-metricsTicker.C:
			c.collectMetrics()
		case <-logTicker.C:
			c.parseNewLogs()
		}
	}
}

// Stop stops the metrics collector
func (c *MetricsCollector) Stop() {
	close(c.stopCh)
	c.wg.Wait()
	if c.logFile != nil {
		c.logFile.Close()
	}
}

// SetPID updates the worker PID being monitored
func (c *MetricsCollector) SetPID(pid int) {
	c.workerPID = pid
}

// collectMetrics collects CPU and memory metrics for the worker process
func (c *MetricsCollector) collectMetrics() {
	if c.workerPID <= 0 {
		return
	}

	metrics, err := c.getProcessMetrics(c.workerPID)
	if err != nil {
		c.state.AddLog(LogEntry{
			Timestamp: time.Now(),
			Level:     LogLevelWarning,
			Component: "metrics",
			Message:   fmt.Sprintf("Failed to collect metrics: %v", err),
		})
		return
	}

	workerID := fmt.Sprintf("worker-%d", c.workerPID)
	c.state.SetWorkerMetrics(workerID, metrics)

	// Update worker info with metrics-derived status
	c.state.UpdateWorker(workerID, func(w *WorkerInfo) {
		w.LastHeartbeat = time.Now()
		// If CPU is high, mark as busy
		if metrics.CPUPercent > 50 {
			w.Status = WorkerStatusBusy
		} else if w.Status == WorkerStatusBusy {
			w.Status = WorkerStatusRunning
		}
	})
}

// getProcessMetrics gets metrics for a specific PID
func (c *MetricsCollector) getProcessMetrics(pid int) (*WorkerMetrics, error) {
	switch runtime.GOOS {
	case "darwin", "linux":
		return c.getMetricsWithPS(pid)
	default:
		return nil, fmt.Errorf("unsupported OS: %s", runtime.GOOS)
	}
}

// getMetricsWithPS uses the ps command to get process metrics
func (c *MetricsCollector) getMetricsWithPS(pid int) (*WorkerMetrics, error) {
	// ps -p <pid> -o %cpu,%mem,rss,vsz
	cmd := exec.Command("ps", "-p", strconv.Itoa(pid), "-o", "%cpu,%mem,rss,vsz")
	output, err := cmd.Output()
	if err != nil {
		return nil, fmt.Errorf("ps command failed: %w", err)
	}

	lines := strings.Split(string(output), "\n")
	if len(lines) < 2 {
		return nil, fmt.Errorf("unexpected ps output")
	}

	// Parse the values line (skip header)
	fields := strings.Fields(lines[1])
	if len(fields) < 4 {
		return nil, fmt.Errorf("unexpected ps output format")
	}

	cpuPercent, _ := strconv.ParseFloat(fields[0], 64)
	memPercent, _ := strconv.ParseFloat(fields[1], 64)
	rssKB, _ := strconv.ParseInt(fields[2], 10, 64)
	vszKB, _ := strconv.ParseInt(fields[3], 10, 64)

	metrics := &WorkerMetrics{
		CPUPercent:    cpuPercent,
		MemoryPercent: memPercent,
		MemoryRSS:     rssKB * 1024,        // Convert KB to bytes
		MemoryVMS:     vszKB * 1024,        // Convert KB to bytes
		MemoryMB:      float64(rssKB) / 1024, // Convert KB to MB
		CollectedAt:   time.Now(),
	}

	// Try to get thread count
	metrics.Threads = c.getThreadCount(pid)

	// Try to get open files count
	metrics.OpenFiles = c.getOpenFilesCount(pid)

	return metrics, nil
}

// getThreadCount gets the number of threads for a process
func (c *MetricsCollector) getThreadCount(pid int) int {
	switch runtime.GOOS {
	case "darwin":
		// On macOS, use ps with nlwp (not available, use approximate)
		cmd := exec.Command("ps", "-M", "-p", strconv.Itoa(pid))
		output, err := cmd.Output()
		if err != nil {
			return 0
		}
		// Count lines minus header
		lines := strings.Split(string(output), "\n")
		count := 0
		for _, line := range lines {
			if strings.TrimSpace(line) != "" {
				count++
			}
		}
		return count - 1 // Subtract header

	case "linux":
		// Read /proc/<pid>/stat for thread count
		data, err := os.ReadFile(fmt.Sprintf("/proc/%d/stat", pid))
		if err != nil {
			return 0
		}
		fields := strings.Fields(string(data))
		if len(fields) >= 20 {
			threads, _ := strconv.Atoi(fields[19])
			return threads
		}
	}
	return 0
}

// getOpenFilesCount gets the number of open files for a process
func (c *MetricsCollector) getOpenFilesCount(pid int) int {
	switch runtime.GOOS {
	case "darwin", "linux":
		cmd := exec.Command("lsof", "-p", strconv.Itoa(pid))
		output, err := cmd.Output()
		if err != nil {
			return 0
		}
		lines := strings.Split(string(output), "\n")
		return len(lines) - 1 // Subtract header

	}
	return 0
}

// parseNewLogs reads and parses new entries from the worker log file
func (c *MetricsCollector) parseNewLogs() {
	logPath := filepath.Join(c.workerDir, "worker.log")

	// Open file if not already open
	if c.logFile == nil {
		f, err := os.Open(logPath)
		if err != nil {
			return // Log file doesn't exist yet
		}
		c.logFile = f

		// Seek to end to only read new entries
		info, err := f.Stat()
		if err == nil {
			c.logOffset = info.Size()
			f.Seek(c.logOffset, 0)
		}
	}

	// Check if file was rotated
	info, err := c.logFile.Stat()
	if err != nil {
		c.logFile.Close()
		c.logFile = nil
		return
	}

	// If file is smaller than our offset, it was rotated
	if info.Size() < c.logOffset {
		c.logFile.Close()
		c.logFile = nil
		c.logOffset = 0
		return
	}

	// Read new lines
	scanner := bufio.NewScanner(c.logFile)
	for scanner.Scan() {
		line := scanner.Text()
		if entry := c.parseLogLine(line); entry != nil {
			c.state.AddLog(*entry)
		}
	}

	// Update offset
	if pos, err := c.logFile.Seek(0, 1); err == nil {
		c.logOffset = pos
	}
}

// parseLogLine parses a single log line into a LogEntry
func (c *MetricsCollector) parseLogLine(line string) *LogEntry {
	if line == "" {
		return nil
	}

	entry := &LogEntry{
		Timestamp: time.Now(),
		Level:     LogLevelInfo,
		Component: "worker",
		Message:   line,
		WorkerID:  fmt.Sprintf("worker-%d", c.workerPID),
	}

	// Try to parse common log formats

	// Format: [LEVEL] message
	if strings.HasPrefix(line, "[") {
		if idx := strings.Index(line, "]"); idx > 0 {
			levelStr := strings.ToUpper(line[1:idx])
			switch levelStr {
			case "DEBUG":
				entry.Level = LogLevelDebug
			case "INFO":
				entry.Level = LogLevelInfo
			case "WARNING", "WARN":
				entry.Level = LogLevelWarning
			case "ERROR", "ERR":
				entry.Level = LogLevelError
			}
			if len(line) > idx+2 {
				entry.Message = strings.TrimSpace(line[idx+1:])
			}
		}
	}

	// Format: TIMESTAMP - LEVEL - COMPONENT - MESSAGE (Python structlog style)
	parts := strings.SplitN(line, " - ", 4)
	if len(parts) >= 3 {
		// Try to parse timestamp
		if ts, err := time.Parse("2006-01-02 15:04:05", parts[0]); err == nil {
			entry.Timestamp = ts

			// Parse level
			switch strings.ToUpper(strings.TrimSpace(parts[1])) {
			case "DEBUG":
				entry.Level = LogLevelDebug
			case "INFO":
				entry.Level = LogLevelInfo
			case "WARNING", "WARN":
				entry.Level = LogLevelWarning
			case "ERROR", "ERR":
				entry.Level = LogLevelError
			}

			entry.Component = strings.TrimSpace(parts[2])
			if len(parts) >= 4 {
				entry.Message = parts[3]
			}
		}
	}

	// Detect special log patterns for activity tracking
	c.detectActivity(entry)

	return entry
}

// detectActivity looks for patterns that indicate activity
func (c *MetricsCollector) detectActivity(entry *LogEntry) {
	msg := strings.ToLower(entry.Message)

	// Task completion patterns
	if strings.Contains(msg, "task completed") || strings.Contains(msg, "execution completed") {
		c.state.AddActivity(RecentActivity{
			Type:        "task_completed",
			Description: "Task completed successfully",
			Timestamp:   entry.Timestamp,
			WorkerID:    entry.WorkerID,
		})

		// Update task counter
		c.state.UpdateWorker(entry.WorkerID, func(w *WorkerInfo) {
			w.TasksTotal++
			if w.TasksActive > 0 {
				w.TasksActive--
			}
		})
	}

	// Task failure patterns
	if strings.Contains(msg, "task failed") || strings.Contains(msg, "execution failed") {
		c.state.AddActivity(RecentActivity{
			Type:        "task_failed",
			Description: "Task failed",
			Timestamp:   entry.Timestamp,
			WorkerID:    entry.WorkerID,
		})

		// Update overview error count
		c.state.UpdateOverview(func(o *WorkerPoolOverview) {
			o.TasksFailed++
			if o.TasksProcessed > 0 {
				o.ErrorRate = float64(o.TasksFailed) / float64(o.TasksProcessed) * 100
			}
		})
	}

	// Session patterns
	if strings.Contains(msg, "session started") || strings.Contains(msg, "new chat session") {
		c.state.AddActivity(RecentActivity{
			Type:        "session_started",
			Description: "New session started",
			Timestamp:   entry.Timestamp,
			WorkerID:    entry.WorkerID,
		})
	}

	// Task start patterns
	if strings.Contains(msg, "task started") || strings.Contains(msg, "starting execution") {
		c.state.UpdateWorker(entry.WorkerID, func(w *WorkerInfo) {
			w.TasksActive++
		})
	}
}

// CollectOnce performs a single metrics collection (for testing)
func (c *MetricsCollector) CollectOnce() {
	c.collectMetrics()
}

// ParseLogsOnce parses logs once (for testing)
func (c *MetricsCollector) ParseLogsOnce() {
	c.parseNewLogs()
}
