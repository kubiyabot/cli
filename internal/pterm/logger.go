package pterm

import (
	"fmt"
	"os"
	"strings"

	"github.com/pterm/pterm"
)

// Logger provides structured logging with PTerm
type Logger struct {
	debugEnabled bool
	disabled     bool
}

// NewLogger creates a new logger instance
func NewLogger(disabled bool) *Logger {
	return &Logger{
		debugEnabled: os.Getenv("KUBIYA_DEBUG") == "true",
		disabled:     disabled,
	}
}

// Debug logs a debug message (only if KUBIYA_DEBUG=true)
func (l *Logger) Debug(message string, args ...interface{}) {
	if !l.debugEnabled {
		return
	}

	formatted := l.formatMessage(message, args...)

	if l.disabled {
		fmt.Printf("[DEBUG] %s\n", formatted)
	} else {
		pterm.Debug.Println(formatted)
	}
}

// Info logs an informational message
func (l *Logger) Info(message string, args ...interface{}) {
	formatted := l.formatMessage(message, args...)

	if l.disabled {
		fmt.Printf("[INFO] %s\n", formatted)
	} else {
		pterm.Info.Println(formatted)
	}
}

// Success logs a success message
func (l *Logger) Success(message string, args ...interface{}) {
	formatted := l.formatMessage(message, args...)

	if l.disabled {
		fmt.Printf("[SUCCESS] ✓ %s\n", formatted)
	} else {
		pterm.Success.Println(formatted)
	}
}

// Warning logs a warning message
func (l *Logger) Warning(message string, args ...interface{}) {
	formatted := l.formatMessage(message, args...)

	if l.disabled {
		fmt.Printf("[WARNING] ⚠ %s\n", formatted)
	} else {
		pterm.Warning.Println(formatted)
	}
}

// Error logs an error message
func (l *Logger) Error(message string, args ...interface{}) {
	formatted := l.formatMessage(message, args...)

	if l.disabled {
		fmt.Printf("[ERROR] ✗ %s\n", formatted)
	} else {
		pterm.Error.Println(formatted)
	}
}

// Debugf logs a formatted debug message (only if KUBIYA_DEBUG=true)
func (l *Logger) Debugf(format string, args ...interface{}) {
	if !l.debugEnabled {
		return
	}

	message := fmt.Sprintf(format, args...)

	if l.disabled {
		fmt.Printf("[DEBUG] %s\n", message)
	} else {
		pterm.Debug.Println(message)
	}
}

// Infof logs a formatted informational message
func (l *Logger) Infof(format string, args ...interface{}) {
	message := fmt.Sprintf(format, args...)

	if l.disabled {
		fmt.Printf("[INFO] %s\n", message)
	} else {
		pterm.Info.Println(message)
	}
}

// Successf logs a formatted success message
func (l *Logger) Successf(format string, args ...interface{}) {
	message := fmt.Sprintf(format, args...)

	if l.disabled {
		fmt.Printf("[SUCCESS] ✓ %s\n", message)
	} else {
		pterm.Success.Println(message)
	}
}

// Warningf logs a formatted warning message
func (l *Logger) Warningf(format string, args ...interface{}) {
	message := fmt.Sprintf(format, args...)

	if l.disabled {
		fmt.Printf("[WARNING] ⚠ %s\n", message)
	} else {
		pterm.Warning.Println(message)
	}
}

// Errorf logs a formatted error message
func (l *Logger) Errorf(format string, args ...interface{}) {
	message := fmt.Sprintf(format, args...)

	if l.disabled {
		fmt.Printf("[ERROR] ✗ %s\n", message)
	} else {
		pterm.Error.Println(message)
	}
}

// formatMessage formats a message with optional key-value pairs
func (l *Logger) formatMessage(message string, args ...interface{}) string {
	if len(args) == 0 {
		return message
	}

	// Support structured logging with key-value pairs
	var pairs []string
	for i := 0; i < len(args); i += 2 {
		if i+1 < len(args) {
			key := fmt.Sprint(args[i])
			value := fmt.Sprint(args[i+1])
			pairs = append(pairs, fmt.Sprintf("%s=%s", key, value))
		}
	}

	if len(pairs) > 0 {
		return fmt.Sprintf("%s (%s)", message, strings.Join(pairs, ", "))
	}

	return message
}

// IsDebugEnabled returns whether debug logging is enabled
func (l *Logger) IsDebugEnabled() bool {
	return l.debugEnabled
}

// Logger method for PTermManager
func (pm *PTermManager) Logger() *Logger {
	return NewLogger(pm.disabled)
}
