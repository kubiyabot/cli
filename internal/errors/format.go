package errors

import (
	"fmt"
	"strings"
)

// FormatError formats a CLIError for display to the user
// Returns a user-friendly error message with context
func FormatError(err *CLIError) string {
	if err == nil {
		return ""
	}

	var sb strings.Builder

	// Add error type prefix for clarity
	switch err.Type {
	case ErrorTypeValidation:
		sb.WriteString("✗ Validation Error: ")
	case ErrorTypeAuth:
		sb.WriteString("✗ Authentication Error: ")
	case ErrorTypeAPI:
		sb.WriteString("✗ API Error: ")
	case ErrorTypeNetwork:
		sb.WriteString("✗ Network Error: ")
	case ErrorTypeConfig:
		sb.WriteString("✗ Configuration Error: ")
	case ErrorTypeRuntime:
		sb.WriteString("✗ Error: ")
	default:
		sb.WriteString("✗ Error: ")
	}

	// Add the main error message
	sb.WriteString(err.Err.Error())

	// Add context/help text if provided
	if err.Context != "" {
		sb.WriteString("\n\n")
		sb.WriteString(err.Context)
	}

	return sb.String()
}

// FormatSimple formats an error without type prefix
// Useful for wrapping non-CLIError types
func FormatSimple(err error) string {
	if err == nil {
		return ""
	}

	if cliErr, ok := err.(*CLIError); ok {
		return FormatError(cliErr)
	}

	return fmt.Sprintf("✗ Error: %v", err)
}
