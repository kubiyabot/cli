package errors

import (
	"fmt"
)

// ErrorType represents the category of error
type ErrorType int

const (
	// ErrorTypeUnknown represents an unclassified error
	ErrorTypeUnknown ErrorType = iota
	// ErrorTypeValidation represents argument/flag validation errors
	ErrorTypeValidation
	// ErrorTypeAuth represents authentication/authorization errors
	ErrorTypeAuth
	// ErrorTypeAPI represents Control Plane API errors
	ErrorTypeAPI
	// ErrorTypeNetwork represents network connectivity errors
	ErrorTypeNetwork
	// ErrorTypeRuntime represents general runtime errors
	ErrorTypeRuntime
	// ErrorTypeConfig represents configuration file errors
	ErrorTypeConfig
)

// CLIError wraps errors with type information and context for better UX
type CLIError struct {
	Type    ErrorType
	Err     error
	Context string // Additional context or help text for the user
}

// Error implements the error interface
func (e *CLIError) Error() string {
	if e.Context != "" {
		return fmt.Sprintf("%v\n%s", e.Err, e.Context)
	}
	return e.Err.Error()
}

// Unwrap implements error unwrapping for Go 1.13+ error chains
func (e *CLIError) Unwrap() error {
	return e.Err
}

// ValidationError creates a validation error (shows usage hints)
func ValidationError(err error, context string) *CLIError {
	return &CLIError{
		Type:    ErrorTypeValidation,
		Err:     err,
		Context: context,
	}
}

// AuthError creates an authentication error
func AuthError(err error) *CLIError {
	return &CLIError{
		Type: ErrorTypeAuth,
		Err:  err,
	}
}

// AuthErrorWithContext creates an authentication error with context
func AuthErrorWithContext(err error, context string) *CLIError {
	return &CLIError{
		Type:    ErrorTypeAuth,
		Err:     err,
		Context: context,
	}
}

// APIError creates an API error
func APIError(err error) *CLIError {
	return &CLIError{
		Type: ErrorTypeAPI,
		Err:  err,
	}
}

// APIErrorWithContext creates an API error with context
func APIErrorWithContext(err error, context string) *CLIError {
	return &CLIError{
		Type:    ErrorTypeAPI,
		Err:     err,
		Context: context,
	}
}

// NetworkError creates a network error
func NetworkError(err error) *CLIError {
	return &CLIError{
		Type: ErrorTypeNetwork,
		Err:  err,
	}
}

// NetworkErrorWithContext creates a network error with context
func NetworkErrorWithContext(err error, context string) *CLIError {
	return &CLIError{
		Type:    ErrorTypeNetwork,
		Err:     err,
		Context: context,
	}
}

// RuntimeError creates a runtime error
func RuntimeError(err error) *CLIError {
	return &CLIError{
		Type: ErrorTypeRuntime,
		Err:  err,
	}
}

// RuntimeErrorWithContext creates a runtime error with context
func RuntimeErrorWithContext(err error, context string) *CLIError {
	return &CLIError{
		Type:    ErrorTypeRuntime,
		Err:     err,
		Context: context,
	}
}

// ConfigError creates a configuration error
func ConfigError(err error) *CLIError {
	return &CLIError{
		Type: ErrorTypeConfig,
		Err:  err,
	}
}

// ConfigErrorWithContext creates a configuration error with context
func ConfigErrorWithContext(err error, context string) *CLIError {
	return &CLIError{
		Type:    ErrorTypeConfig,
		Err:     err,
		Context: context,
	}
}
