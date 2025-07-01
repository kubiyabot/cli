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

// Initialize sets up Sentry if DSN is provided
func Initialize(version string) error {
	dsn := os.Getenv("SENTRY_DSN")
	if dsn == "" {
		// Sentry not configured, skip initialization
		return nil
	}

	environment := os.Getenv("SENTRY_ENVIRONMENT")
	if environment == "" {
		environment = "production"
	}

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
		BeforeSend: func(event *sentry.Event, hint *sentry.EventHint) *sentry.Event {
			// Add custom data
			event.Extra["kubiya_api_url"] = os.Getenv("KUBIYA_API_URL")
			event.Extra["runner"] = os.Getenv("KUBIYA_DEFAULT_RUNNER")
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
