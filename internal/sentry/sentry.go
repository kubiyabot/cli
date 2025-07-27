package sentry

import (
	"context"
	"fmt"
	"os"
	"time"

	"github.com/getsentry/sentry-go"
)

// Config holds Sentry configuration
type Config struct {
	DSN         string
	Environment string
	Release     string
	SampleRate  float64
	Debug       bool
}

// Initialize sets up Sentry with hardcoded DSN for production-grade tracing
func Initialize(version string) error {
	// Hardcoded DSN as requested - CLI can run anywhere
	dsn := "https://bd0e536cf329a4e27a18f478924df9e1@o1306664.ingest.us.sentry.io/4509739770904576"
	
	environment := os.Getenv("SENTRY_ENVIRONMENT")
	if environment == "" {
		environment = "production"
	}

	// Set to 1.0 for comprehensive tracing as requested
	sampleRate := 1.0
	if rate := os.Getenv("SENTRY_TRACES_SAMPLE_RATE"); rate != "" {
		fmt.Sscanf(rate, "%f", &sampleRate)
	}

	debug := os.Getenv("SENTRY_DEBUG") == "true"

	err := sentry.Init(sentry.ClientOptions{
		Dsn:              dsn,
		Environment:      environment,
		Release:          version,
		TracesSampleRate: sampleRate,
		Debug:            debug,
		AttachStacktrace: true,
		// Enhanced before send hook for richer context
		BeforeSend: func(event *sentry.Event, hint *sentry.EventHint) *sentry.Event {
			// Add CLI-specific context
			event.Extra["kubiya_api_url"] = os.Getenv("KUBIYA_API_URL")
			event.Extra["runner"] = os.Getenv("KUBIYA_DEFAULT_RUNNER")
			event.Extra["automation_mode"] = os.Getenv("KUBIYA_AUTOMATION")
			event.Extra["cli_version"] = version
			
			// Add OS and runtime info
			event.Extra["os"] = os.Getenv("GOOS")
			event.Extra["arch"] = os.Getenv("GOARCH")
			event.Extra["pwd"], _ = os.Getwd()
			
			return event
		},
		// Enhanced before send transaction for better performance monitoring
		BeforeSendTransaction: func(event *sentry.Event, hint *sentry.EventHint) *sentry.Event {
			// Add performance context
			event.Extra["cli_command"] = os.Args
			return event
		},
	})

	if err != nil {
		return fmt.Errorf("failed to initialize Sentry: %w", err)
	}

	// Set user context if available
	if userEmail := os.Getenv("KUBIYA_USER_EMAIL"); userEmail != "" {
		sentry.ConfigureScope(func(scope *sentry.Scope) {
			scope.SetUser(sentry.User{
				Email: userEmail,
			})
		})
	}

	return nil
}

// Flush waits for all events to be sent
func Flush(timeout time.Duration) {
	if sentry.CurrentHub().Client() != nil {
		sentry.Flush(timeout)
	}
}

// StartSpan starts a new span for tracing
func StartSpan(ctx context.Context, operation string, opts ...sentry.SpanOption) (*sentry.Span, context.Context) {
	if sentry.CurrentHub().Client() == nil {
		return nil, ctx
	}
	span := sentry.StartSpan(ctx, operation, opts...)
	return span, span.Context()
}

// CaptureError captures an error with additional context
func CaptureError(err error, tags map[string]string, extras map[string]interface{}) {
	if sentry.CurrentHub().Client() == nil || err == nil {
		return
	}

	sentry.WithScope(func(scope *sentry.Scope) {
		for k, v := range tags {
			scope.SetTag(k, v)
		}
		for k, v := range extras {
			scope.SetExtra(k, v)
		}
		sentry.CaptureException(err)
	})
}

// CaptureMessage captures a message with level
func CaptureMessage(message string, level sentry.Level, tags map[string]string) {
	if sentry.CurrentHub().Client() == nil {
		return
	}

	sentry.WithScope(func(scope *sentry.Scope) {
		for k, v := range tags {
			scope.SetTag(k, v)
		}
		scope.SetLevel(level)
		sentry.CaptureMessage(message)
	})
}

// RecoverWithSentry recovers from panic and reports to Sentry
func RecoverWithSentry(ctx context.Context, extras map[string]interface{}) {
	if err := recover(); err != nil {
		if sentry.CurrentHub().Client() != nil {
			sentry.WithScope(func(scope *sentry.Scope) {
				for k, v := range extras {
					scope.SetExtra(k, v)
				}
				sentry.CurrentHub().RecoverWithContext(ctx, err)
			})
		}
		// Re-panic after reporting
		panic(err)
	}
}

// WrapError wraps an error with additional context for Sentry
type SentryError struct {
	Err    error
	Tags   map[string]string
	Extras map[string]interface{}
}

func (e *SentryError) Error() string {
	return e.Err.Error()
}

func (e *SentryError) Unwrap() error {
	return e.Err
}

// NewError creates a new error with Sentry context
func NewError(err error, tags map[string]string, extras map[string]interface{}) error {
	if err == nil {
		return nil
	}
	return &SentryError{
		Err:    err,
		Tags:   tags,
		Extras: extras,
	}
}

// SetTransaction sets the transaction name for the current scope
func SetTransaction(name string) {
	if sentry.CurrentHub().Client() != nil {
		sentry.ConfigureScope(func(scope *sentry.Scope) {
			scope.SetContext("transaction", sentry.Context{
				"name": name,
			})
		})
	}
}

// AddBreadcrumb adds a breadcrumb for debugging
func AddBreadcrumb(category, message string, data map[string]interface{}) {
	if sentry.CurrentHub().Client() != nil {
		sentry.AddBreadcrumb(&sentry.Breadcrumb{
			Category:  category,
			Message:   message,
			Level:     sentry.LevelInfo,
			Data:      data,
			Timestamp: time.Now(),
		})
	}
}

// WithTransaction runs a function within a transaction
func WithTransaction(ctx context.Context, name string, fn func(context.Context) error) error {
	if sentry.CurrentHub().Client() == nil {
		return fn(ctx)
	}

	span := sentry.StartTransaction(ctx, name)
	defer span.Finish()

	ctx = span.Context()
	err := fn(ctx)

	if err != nil {
		span.Status = sentry.SpanStatusInternalError
		sentry.CaptureException(err)
	} else {
		span.Status = sentry.SpanStatusOK
	}

	return err
}

// WithMCPTransaction runs MCP operations with comprehensive tracing and context deadline monitoring
func WithMCPTransaction(ctx context.Context, operation string, toolName string, fn func(context.Context) error) error {
	if sentry.CurrentHub().Client() == nil {
		return fn(ctx)
	}

	transactionName := fmt.Sprintf("mcp.%s", operation)
	span := sentry.StartTransaction(ctx, transactionName)
	defer span.Finish()

	// Add MCP-specific tags and context
	span.SetTag("mcp.operation", operation)
	span.SetTag("mcp.tool_name", toolName)
	span.SetData("mcp.context_deadline", getContextDeadline(ctx))
	span.SetData("mcp.has_timeout", hasContextTimeout(ctx))

	// Monitor context deadline issues
	if deadline, ok := ctx.Deadline(); ok {
		timeRemaining := time.Until(deadline)
		span.SetData("mcp.time_remaining_ms", timeRemaining.Milliseconds())
		
		// Log warning if deadline is very close
		if timeRemaining < 5*time.Second {
			AddBreadcrumb("mcp.deadline_warning", 
				fmt.Sprintf("MCP operation %s has only %v remaining", operation, timeRemaining),
				map[string]interface{}{
					"operation": operation,
					"tool_name": toolName,
					"time_remaining_ms": timeRemaining.Milliseconds(),
				})
		}
	}

	ctx = span.Context()
	err := fn(ctx)

	if err != nil {
		span.Status = sentry.SpanStatusInternalError
		span.SetTag("error.type", fmt.Sprintf("%T", err))
		
		// Special handling for context deadline exceeded
		if ctx.Err() == context.DeadlineExceeded {
			span.SetTag("mcp.deadline_exceeded", "true")
			CaptureError(err, map[string]string{
				"mcp.operation": operation,
				"mcp.tool_name": toolName,
				"error.type": "context_deadline_exceeded",
			}, map[string]interface{}{
				"context_error": ctx.Err().Error(),
			})
		}
		
		sentry.CaptureException(err)
	} else {
		span.Status = sentry.SpanStatusOK
	}

	return err
}

// WithCLICommand traces CLI command execution
func WithCLICommand(ctx context.Context, commandName string, args []string, fn func(context.Context) error) error {
	if sentry.CurrentHub().Client() == nil {
		return fn(ctx)
	}

	transactionName := fmt.Sprintf("cli.%s", commandName)
	span := sentry.StartTransaction(ctx, transactionName)
	defer span.Finish()

	// Add CLI-specific context
	span.SetTag("cli.command", commandName)
	span.SetData("cli.args", args)
	span.SetData("cli.arg_count", len(args))

	ctx = span.Context()
	err := fn(ctx)

	if err != nil {
		span.Status = sentry.SpanStatusInternalError
		span.SetTag("error.command", commandName)
		sentry.CaptureException(err)
	} else {
		span.Status = sentry.SpanStatusOK
	}

	return err
}

// WithKubiyaChat traces chat interactions
func WithKubiyaChat(ctx context.Context, chatType string, messageCount int, fn func(context.Context) error) error {
	if sentry.CurrentHub().Client() == nil {
		return fn(ctx)
	}

	transactionName := fmt.Sprintf("kubiya.chat.%s", chatType)
	span := sentry.StartTransaction(ctx, transactionName)
	defer span.Finish()

	span.SetTag("chat.type", chatType)
	span.SetData("chat.message_count", messageCount)

	ctx = span.Context()
	err := fn(ctx)

	if err != nil {
		span.Status = sentry.SpanStatusInternalError
		span.SetTag("error.chat_type", chatType)
		sentry.CaptureException(err)
	} else {
		span.Status = sentry.SpanStatusOK
	}

	return err
}

// WithWorkflowExecution traces workflow execution
func WithWorkflowExecution(ctx context.Context, workflowName string, workflowID string, fn func(context.Context) error) error {
	if sentry.CurrentHub().Client() == nil {
		return fn(ctx)
	}

	transactionName := "kubiya.workflow.execute"
	span := sentry.StartTransaction(ctx, transactionName)
	defer span.Finish()

	span.SetTag("workflow.name", workflowName)
	span.SetTag("workflow.id", workflowID)

	ctx = span.Context()
	err := fn(ctx)

	if err != nil {
		span.Status = sentry.SpanStatusInternalError
		span.SetTag("error.workflow", workflowName)
		sentry.CaptureException(err)
	} else {
		span.Status = sentry.SpanStatusOK
	}

	return err
}

// WithToolExecution traces tool execution
func WithToolExecution(ctx context.Context, toolName string, toolSource string, fn func(context.Context) error) error {
	if sentry.CurrentHub().Client() == nil {
		return fn(ctx)
	}

	transactionName := "kubiya.tool.execute"
	span := sentry.StartTransaction(ctx, transactionName)
	defer span.Finish()

	span.SetTag("tool.name", toolName)
	span.SetTag("tool.source", toolSource)

	ctx = span.Context()
	err := fn(ctx)

	if err != nil {
		span.Status = sentry.SpanStatusInternalError
		span.SetTag("error.tool_name", toolName)
		sentry.CaptureException(err)
	} else {
		span.Status = sentry.SpanStatusOK
	}

	return err
}

// Helper functions for context deadline monitoring
func getContextDeadline(ctx context.Context) string {
	if deadline, ok := ctx.Deadline(); ok {
		return deadline.Format(time.RFC3339)
	}
	return "no_deadline"
}

func hasContextTimeout(ctx context.Context) bool {
	_, ok := ctx.Deadline()
	return ok
}
