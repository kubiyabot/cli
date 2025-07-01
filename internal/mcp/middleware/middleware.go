package middleware

import (
	"context"
	"fmt"
	"log"
	"sync"
	"time"

	"github.com/getsentry/sentry-go"
	"github.com/kubiyabot/cli/internal/mcp/session"
	sentryutil "github.com/kubiyabot/cli/internal/sentry"
	"github.com/mark3labs/mcp-go/mcp"
	"golang.org/x/time/rate"
)

// ToolHandler is the handler function for tools
type ToolHandler func(context.Context, mcp.CallToolRequest) (*mcp.CallToolResult, error)

// Middleware is a function that wraps a ToolHandler
type Middleware func(ToolHandler) ToolHandler

// Chain chains multiple middleware together
func Chain(middlewares ...Middleware) Middleware {
	return func(next ToolHandler) ToolHandler {
		for i := len(middlewares) - 1; i >= 0; i-- {
			next = middlewares[i](next)
		}
		return next
	}
}

// LoggingMiddleware logs all tool calls
type LoggingMiddleware struct {
	logger *log.Logger
}

// NewLoggingMiddleware creates a new logging middleware
func NewLoggingMiddleware(logger *log.Logger) *LoggingMiddleware {
	if logger == nil {
		logger = log.Default()
	}
	return &LoggingMiddleware{logger: logger}
}

// Apply applies the logging middleware
func (m *LoggingMiddleware) Apply(next ToolHandler) ToolHandler {
	return func(ctx context.Context, req mcp.CallToolRequest) (*mcp.CallToolResult, error) {
		start := time.Now()

		// Get session info
		sessionInfo := "unknown"
		if sess, ok := session.SessionFromContext(ctx); ok {
			sessionInfo = fmt.Sprintf("%s (%s)", sess.ID, sess.Email)
		}

		// Create Sentry span
		span, ctx := sentryutil.StartSpan(ctx, fmt.Sprintf("mcp.tool.%s", req.Params.Name))
		if span != nil {
			span.SetTag("tool.name", req.Params.Name)
			span.SetTag("session.id", sessionInfo)
			defer span.Finish()
		}

		// Add breadcrumb
		sentryutil.AddBreadcrumb("tool_call", fmt.Sprintf("Calling tool: %s", req.Params.Name), map[string]interface{}{
			"session": sessionInfo,
			"args":    req.Params.Arguments,
		})

		m.logger.Printf("[TOOL START] session=%s tool=%s args=%v", sessionInfo, req.Params.Name, req.Params.Arguments)

		// Call the next handler
		result, err := next(ctx, req)

		duration := time.Since(start)

		if err != nil {
			m.logger.Printf("[TOOL ERROR] session=%s tool=%s duration=%v error=%v",
				sessionInfo, req.Params.Name, duration, err)

			// Report error to Sentry
			sentryutil.CaptureError(err, map[string]string{
				"tool":    req.Params.Name,
				"session": sessionInfo,
			}, map[string]interface{}{
				"arguments": req.Params.Arguments,
				"duration":  duration.String(),
			})

			if span != nil {
				span.Status = sentry.SpanStatusInternalError
			}
		} else {
			m.logger.Printf("[TOOL SUCCESS] session=%s tool=%s duration=%v",
				sessionInfo, req.Params.Name, duration)

			if span != nil {
				span.Status = sentry.SpanStatusOK
			}
		}

		return result, err
	}
}

// RateLimitMiddleware implements rate limiting per session
type RateLimitMiddleware struct {
	limiters map[string]*rate.Limiter
	mutex    sync.RWMutex
	rate     rate.Limit
	burst    int
}

// NewRateLimitMiddleware creates a new rate limit middleware
func NewRateLimitMiddleware(requestsPerSecond float64, burst int) *RateLimitMiddleware {
	return &RateLimitMiddleware{
		limiters: make(map[string]*rate.Limiter),
		rate:     rate.Limit(requestsPerSecond),
		burst:    burst,
	}
}

// getLimiter gets or creates a rate limiter for a session
func (m *RateLimitMiddleware) getLimiter(sessionID string) *rate.Limiter {
	m.mutex.RLock()
	limiter, exists := m.limiters[sessionID]
	m.mutex.RUnlock()

	if !exists {
		m.mutex.Lock()
		// Double-check after acquiring write lock
		if limiter, exists = m.limiters[sessionID]; !exists {
			limiter = rate.NewLimiter(m.rate, m.burst)
			m.limiters[sessionID] = limiter
		}
		m.mutex.Unlock()
	}

	return limiter
}

// Apply applies the rate limit middleware
func (m *RateLimitMiddleware) Apply(next ToolHandler) ToolHandler {
	return func(ctx context.Context, req mcp.CallToolRequest) (*mcp.CallToolResult, error) {
		sessionID := "anonymous"
		if sess, ok := session.SessionFromContext(ctx); ok {
			sessionID = sess.ID
		}

		limiter := m.getLimiter(sessionID)

		if !limiter.Allow() {
			sentryutil.CaptureMessage("Rate limit exceeded", sentry.LevelWarning, map[string]string{
				"session": sessionID,
				"tool":    req.Params.Name,
			})

			return mcp.NewToolResultError(fmt.Sprintf("Rate limit exceeded for session %s. Please wait before making more requests.", sessionID)), nil
		}

		return next(ctx, req)
	}
}

// Cleanup removes inactive session limiters
func (m *RateLimitMiddleware) Cleanup(sessionID string) {
	m.mutex.Lock()
	defer m.mutex.Unlock()
	delete(m.limiters, sessionID)
}

// AuthMiddleware implements authentication checks
type AuthMiddleware struct {
	sessionManager *session.Manager
	requireAuth    bool
}

// NewAuthMiddleware creates a new auth middleware
func NewAuthMiddleware(sessionManager *session.Manager, requireAuth bool) *AuthMiddleware {
	return &AuthMiddleware{
		sessionManager: sessionManager,
		requireAuth:    requireAuth,
	}
}

// Apply applies the auth middleware
func (m *AuthMiddleware) Apply(next ToolHandler) ToolHandler {
	return func(ctx context.Context, req mcp.CallToolRequest) (*mcp.CallToolResult, error) {
		// Get session from context
		sess, ok := session.SessionFromContext(ctx)
		if !ok && m.requireAuth {
			return mcp.NewToolResultError("Authentication required. No valid session found."), nil
		}

		// For authenticated sessions, check if user is active
		if ok && sess.UserID == "" && m.requireAuth {
			return mcp.NewToolResultError("Session is not authenticated. Please log in."), nil
		}

		// Add session info to Sentry scope
		if ok {
			sentry.ConfigureScope(func(scope *sentry.Scope) {
				scope.SetUser(sentry.User{
					ID:    sess.UserID,
					Email: sess.Email,
				})
				scope.SetTag("session.id", sess.ID)
			})
		}

		return next(ctx, req)
	}
}

// PermissionMiddleware checks tool permissions
type PermissionMiddleware struct {
	toolPermissions map[string]string // tool name -> required permission
}

// NewPermissionMiddleware creates a new permission middleware
func NewPermissionMiddleware(toolPermissions map[string]string) *PermissionMiddleware {
	if toolPermissions == nil {
		toolPermissions = make(map[string]string)
	}
	return &PermissionMiddleware{
		toolPermissions: toolPermissions,
	}
}

// Apply applies the permission middleware
func (m *PermissionMiddleware) Apply(next ToolHandler) ToolHandler {
	return func(ctx context.Context, req mcp.CallToolRequest) (*mcp.CallToolResult, error) {
		requiredPerm, needsCheck := m.toolPermissions[req.Params.Name]
		if !needsCheck {
			// No specific permission required
			return next(ctx, req)
		}

		sess, ok := session.SessionFromContext(ctx)
		if !ok {
			return mcp.NewToolResultError("Session required to check permissions"), nil
		}

		if !sess.HasPermission(requiredPerm) {
			sentryutil.CaptureMessage("Permission denied", sentry.LevelWarning, map[string]string{
				"user":                sess.UserID,
				"tool":                req.Params.Name,
				"required_permission": requiredPerm,
			})

			return mcp.NewToolResultError(fmt.Sprintf("Permission denied. Required permission: %s", requiredPerm)), nil
		}

		return next(ctx, req)
	}
}

// ErrorRecoveryMiddleware recovers from panics
type ErrorRecoveryMiddleware struct {
	logger *log.Logger
}

// NewErrorRecoveryMiddleware creates error recovery middleware
func NewErrorRecoveryMiddleware(logger *log.Logger) *ErrorRecoveryMiddleware {
	if logger == nil {
		logger = log.Default()
	}
	return &ErrorRecoveryMiddleware{logger: logger}
}

// Apply applies the error recovery middleware
func (m *ErrorRecoveryMiddleware) Apply(next ToolHandler) ToolHandler {
	return func(ctx context.Context, req mcp.CallToolRequest) (result *mcp.CallToolResult, err error) {
		defer func() {
			if r := recover(); r != nil {
				// Get session info
				sessionInfo := "unknown"
				if sess, ok := session.SessionFromContext(ctx); ok {
					sessionInfo = fmt.Sprintf("%s (%s)", sess.ID, sess.Email)
				}

				// Log the panic
				m.logger.Printf("[PANIC] session=%s tool=%s panic=%v", sessionInfo, req.Params.Name, r)

				// Send to Sentry
				sentryutil.RecoverWithSentry(ctx, map[string]interface{}{
					"tool":      req.Params.Name,
					"session":   sessionInfo,
					"arguments": req.Params.Arguments,
				})

				// Return error result
				result = mcp.NewToolResultError(fmt.Sprintf("Internal error occurred while executing tool %s", req.Params.Name))
				err = nil // Don't return error to prevent server crash
			}
		}()

		return next(ctx, req)
	}
}

// TimeoutMiddleware adds timeout to tool execution
type TimeoutMiddleware struct {
	defaultTimeout time.Duration
	toolTimeouts   map[string]time.Duration
}

// NewTimeoutMiddleware creates timeout middleware
func NewTimeoutMiddleware(defaultTimeout time.Duration) *TimeoutMiddleware {
	return &TimeoutMiddleware{
		defaultTimeout: defaultTimeout,
		toolTimeouts:   make(map[string]time.Duration),
	}
}

// SetToolTimeout sets timeout for a specific tool
func (m *TimeoutMiddleware) SetToolTimeout(toolName string, timeout time.Duration) {
	m.toolTimeouts[toolName] = timeout
}

// Apply applies the timeout middleware
func (m *TimeoutMiddleware) Apply(next ToolHandler) ToolHandler {
	return func(ctx context.Context, req mcp.CallToolRequest) (*mcp.CallToolResult, error) {
		timeout := m.defaultTimeout
		if toolTimeout, ok := m.toolTimeouts[req.Params.Name]; ok {
			timeout = toolTimeout
		}

		// Create context with timeout
		timeoutCtx, cancel := context.WithTimeout(ctx, timeout)
		defer cancel()

		// Channel for result
		type resultWrapper struct {
			result *mcp.CallToolResult
			err    error
		}
		resultChan := make(chan resultWrapper, 1)

		// Execute in goroutine
		go func() {
			result, err := next(timeoutCtx, req)
			resultChan <- resultWrapper{result, err}
		}()

		// Wait for result or timeout
		select {
		case res := <-resultChan:
			return res.result, res.err
		case <-timeoutCtx.Done():
			sentryutil.CaptureMessage("Tool execution timeout", sentry.LevelWarning, map[string]string{
				"tool":    req.Params.Name,
				"timeout": timeout.String(),
			})
			return mcp.NewToolResultError(fmt.Sprintf("Tool execution timed out after %v", timeout)), nil
		}
	}
}
