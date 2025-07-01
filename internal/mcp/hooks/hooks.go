package hooks

import (
	"context"
	"fmt"
	"log"
	"sync"
	"time"

	"github.com/getsentry/sentry-go"
	sentryutil "github.com/kubiyabot/cli/internal/sentry"
)

// Hooks defines lifecycle callbacks for the MCP server
type Hooks interface {
	OnServerStart(ctx context.Context)
	OnServerStop(ctx context.Context)
	OnSessionStart(ctx context.Context, sessionID string, clientInfo map[string]interface{})
	OnSessionEnd(ctx context.Context, sessionID string, duration time.Duration)
	OnToolCall(ctx context.Context, sessionID, toolName string, duration time.Duration, err error)
	OnResourceRead(ctx context.Context, sessionID, uri string, duration time.Duration, err error)
	OnPromptCall(ctx context.Context, sessionID, promptName string, duration time.Duration, err error)
	OnError(ctx context.Context, sessionID string, err error)
}

// CompositeHooks allows multiple hooks to be combined
type CompositeHooks struct {
	hooks []Hooks
	mutex sync.RWMutex
}

// NewCompositeHooks creates a new composite hooks instance
func NewCompositeHooks(hooks ...Hooks) *CompositeHooks {
	return &CompositeHooks{
		hooks: hooks,
	}
}

// AddHooks adds more hooks to the composite
func (c *CompositeHooks) AddHooks(hooks ...Hooks) {
	c.mutex.Lock()
	defer c.mutex.Unlock()
	c.hooks = append(c.hooks, hooks...)
}

// OnServerStart is called when server starts
func (c *CompositeHooks) OnServerStart(ctx context.Context) {
	c.mutex.RLock()
	defer c.mutex.RUnlock()

	for _, h := range c.hooks {
		h.OnServerStart(ctx)
	}
}

// OnServerStop is called when server stops
func (c *CompositeHooks) OnServerStop(ctx context.Context) {
	c.mutex.RLock()
	defer c.mutex.RUnlock()

	for _, h := range c.hooks {
		h.OnServerStop(ctx)
	}
}

// OnSessionStart is called when a new session starts
func (c *CompositeHooks) OnSessionStart(ctx context.Context, sessionID string, clientInfo map[string]interface{}) {
	c.mutex.RLock()
	defer c.mutex.RUnlock()

	for _, h := range c.hooks {
		h.OnSessionStart(ctx, sessionID, clientInfo)
	}
}

// OnSessionEnd is called when a session ends
func (c *CompositeHooks) OnSessionEnd(ctx context.Context, sessionID string, duration time.Duration) {
	c.mutex.RLock()
	defer c.mutex.RUnlock()

	for _, h := range c.hooks {
		h.OnSessionEnd(ctx, sessionID, duration)
	}
}

// OnToolCall is called after a tool is executed
func (c *CompositeHooks) OnToolCall(ctx context.Context, sessionID, toolName string, duration time.Duration, err error) {
	c.mutex.RLock()
	defer c.mutex.RUnlock()

	for _, h := range c.hooks {
		h.OnToolCall(ctx, sessionID, toolName, duration, err)
	}
}

// OnResourceRead is called after a resource is read
func (c *CompositeHooks) OnResourceRead(ctx context.Context, sessionID, uri string, duration time.Duration, err error) {
	c.mutex.RLock()
	defer c.mutex.RUnlock()

	for _, h := range c.hooks {
		h.OnResourceRead(ctx, sessionID, uri, duration, err)
	}
}

// OnPromptCall is called after a prompt is executed
func (c *CompositeHooks) OnPromptCall(ctx context.Context, sessionID, promptName string, duration time.Duration, err error) {
	c.mutex.RLock()
	defer c.mutex.RUnlock()

	for _, h := range c.hooks {
		h.OnPromptCall(ctx, sessionID, promptName, duration, err)
	}
}

// OnError is called when an error occurs
func (c *CompositeHooks) OnError(ctx context.Context, sessionID string, err error) {
	c.mutex.RLock()
	defer c.mutex.RUnlock()

	for _, h := range c.hooks {
		h.OnError(ctx, sessionID, err)
	}
}

// LoggingHooks provides logging for all events
type LoggingHooks struct {
	logger *log.Logger
}

// NewLoggingHooks creates logging hooks
func NewLoggingHooks(logger *log.Logger) *LoggingHooks {
	if logger == nil {
		logger = log.Default()
	}
	return &LoggingHooks{logger: logger}
}

func (h *LoggingHooks) OnServerStart(ctx context.Context) {
	h.logger.Println("[SERVER] MCP Server starting")
}

func (h *LoggingHooks) OnServerStop(ctx context.Context) {
	h.logger.Println("[SERVER] MCP Server stopping")
}

func (h *LoggingHooks) OnSessionStart(ctx context.Context, sessionID string, clientInfo map[string]interface{}) {
	h.logger.Printf("[SESSION] Started: %s, client=%v", sessionID, clientInfo)
}

func (h *LoggingHooks) OnSessionEnd(ctx context.Context, sessionID string, duration time.Duration) {
	h.logger.Printf("[SESSION] Ended: %s, duration=%v", sessionID, duration)
}

func (h *LoggingHooks) OnToolCall(ctx context.Context, sessionID, toolName string, duration time.Duration, err error) {
	if err != nil {
		h.logger.Printf("[TOOL] Failed: session=%s tool=%s duration=%v error=%v", sessionID, toolName, duration, err)
	} else {
		h.logger.Printf("[TOOL] Success: session=%s tool=%s duration=%v", sessionID, toolName, duration)
	}
}

func (h *LoggingHooks) OnResourceRead(ctx context.Context, sessionID, uri string, duration time.Duration, err error) {
	if err != nil {
		h.logger.Printf("[RESOURCE] Failed: session=%s uri=%s duration=%v error=%v", sessionID, uri, duration, err)
	} else {
		h.logger.Printf("[RESOURCE] Success: session=%s uri=%s duration=%v", sessionID, uri, duration)
	}
}

func (h *LoggingHooks) OnPromptCall(ctx context.Context, sessionID, promptName string, duration time.Duration, err error) {
	if err != nil {
		h.logger.Printf("[PROMPT] Failed: session=%s prompt=%s duration=%v error=%v", sessionID, promptName, duration, err)
	} else {
		h.logger.Printf("[PROMPT] Success: session=%s prompt=%s duration=%v", sessionID, promptName, duration)
	}
}

func (h *LoggingHooks) OnError(ctx context.Context, sessionID string, err error) {
	h.logger.Printf("[ERROR] session=%s error=%v", sessionID, err)
}

// SentryHooks provides Sentry integration for all events
type SentryHooks struct{}

// NewSentryHooks creates Sentry hooks
func NewSentryHooks() *SentryHooks {
	return &SentryHooks{}
}

func (h *SentryHooks) OnServerStart(ctx context.Context) {
	sentryutil.AddBreadcrumb("server", "MCP Server started", nil)
	sentryutil.CaptureMessage("MCP Server started", sentry.LevelInfo, map[string]string{
		"component": "mcp_server",
	})
}

func (h *SentryHooks) OnServerStop(ctx context.Context) {
	sentryutil.AddBreadcrumb("server", "MCP Server stopped", nil)
	sentryutil.CaptureMessage("MCP Server stopped", sentry.LevelInfo, map[string]string{
		"component": "mcp_server",
	})
}

func (h *SentryHooks) OnSessionStart(ctx context.Context, sessionID string, clientInfo map[string]interface{}) {
	sentryutil.AddBreadcrumb("session", "Session started", map[string]interface{}{
		"session_id": sessionID,
		"client":     clientInfo,
	})
}

func (h *SentryHooks) OnSessionEnd(ctx context.Context, sessionID string, duration time.Duration) {
	sentryutil.AddBreadcrumb("session", "Session ended", map[string]interface{}{
		"session_id": sessionID,
		"duration":   duration.String(),
	})
}

func (h *SentryHooks) OnToolCall(ctx context.Context, sessionID, toolName string, duration time.Duration, err error) {
	data := map[string]interface{}{
		"session_id": sessionID,
		"tool":       toolName,
		"duration":   duration.String(),
	}

	if err != nil {
		data["error"] = err.Error()
		sentryutil.AddBreadcrumb("tool", fmt.Sprintf("Tool failed: %s", toolName), data)
	} else {
		sentryutil.AddBreadcrumb("tool", fmt.Sprintf("Tool succeeded: %s", toolName), data)
	}
}

func (h *SentryHooks) OnResourceRead(ctx context.Context, sessionID, uri string, duration time.Duration, err error) {
	data := map[string]interface{}{
		"session_id": sessionID,
		"uri":        uri,
		"duration":   duration.String(),
	}

	if err != nil {
		data["error"] = err.Error()
		sentryutil.AddBreadcrumb("resource", fmt.Sprintf("Resource read failed: %s", uri), data)
	} else {
		sentryutil.AddBreadcrumb("resource", fmt.Sprintf("Resource read succeeded: %s", uri), data)
	}
}

func (h *SentryHooks) OnPromptCall(ctx context.Context, sessionID, promptName string, duration time.Duration, err error) {
	data := map[string]interface{}{
		"session_id": sessionID,
		"prompt":     promptName,
		"duration":   duration.String(),
	}

	if err != nil {
		data["error"] = err.Error()
		sentryutil.AddBreadcrumb("prompt", fmt.Sprintf("Prompt failed: %s", promptName), data)
	} else {
		sentryutil.AddBreadcrumb("prompt", fmt.Sprintf("Prompt succeeded: %s", promptName), data)
	}
}

func (h *SentryHooks) OnError(ctx context.Context, sessionID string, err error) {
	sentryutil.CaptureError(err, map[string]string{
		"session_id": sessionID,
	}, nil)
}

// MetricsHooks provides metrics collection for all events
type MetricsHooks struct {
	activeSessions sync.Map
	startTime      time.Time
}

// NewMetricsHooks creates metrics hooks
func NewMetricsHooks() *MetricsHooks {
	return &MetricsHooks{
		startTime: time.Now(),
	}
}

func (h *MetricsHooks) OnServerStart(ctx context.Context) {
	h.startTime = time.Now()
}

func (h *MetricsHooks) OnServerStop(ctx context.Context) {
	uptime := time.Since(h.startTime)
	sentryutil.CaptureMessage("Server stopped", sentry.LevelInfo, map[string]string{
		"uptime": uptime.String(),
	})
}

func (h *MetricsHooks) OnSessionStart(ctx context.Context, sessionID string, clientInfo map[string]interface{}) {
	h.activeSessions.Store(sessionID, time.Now())
}

func (h *MetricsHooks) OnSessionEnd(ctx context.Context, sessionID string, duration time.Duration) {
	h.activeSessions.Delete(sessionID)
}

func (h *MetricsHooks) OnToolCall(ctx context.Context, sessionID, toolName string, duration time.Duration, err error) {
	tags := map[string]string{
		"tool":    toolName,
		"success": fmt.Sprintf("%v", err == nil),
	}

	// Track in Sentry as a custom metric
	sentryutil.CaptureMessage("tool_execution", sentry.LevelInfo, tags)
}

func (h *MetricsHooks) OnResourceRead(ctx context.Context, sessionID, uri string, duration time.Duration, err error) {
	tags := map[string]string{
		"resource": uri,
		"success":  fmt.Sprintf("%v", err == nil),
	}

	sentryutil.CaptureMessage("resource_read", sentry.LevelInfo, tags)
}

func (h *MetricsHooks) OnPromptCall(ctx context.Context, sessionID, promptName string, duration time.Duration, err error) {
	tags := map[string]string{
		"prompt":  promptName,
		"success": fmt.Sprintf("%v", err == nil),
	}

	sentryutil.CaptureMessage("prompt_execution", sentry.LevelInfo, tags)
}

func (h *MetricsHooks) OnError(ctx context.Context, sessionID string, err error) {
	// Errors are already tracked by SentryHooks
}

// GetActiveSessionCount returns the current number of active sessions
func (h *MetricsHooks) GetActiveSessionCount() int {
	count := 0
	h.activeSessions.Range(func(key, value interface{}) bool {
		count++
		return true
	})
	return count
}
