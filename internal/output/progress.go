package output

import (
	"fmt"
	"io"
	"os"
	"sync"

	"github.com/kubiyabot/cli/internal/pterm"
	"github.com/kubiyabot/cli/internal/types"
)

// OutputMode is re-exported from types for backward compatibility
type OutputMode = types.OutputMode

const (
	// OutputModeInteractive shows progress bars, spinners, and emojis
	OutputModeInteractive = types.OutputModeInteractive
	// OutputModeCI shows plain text with timestamps, no spinners
	OutputModeCI = types.OutputModeCI
)

// ProgressManager manages output mode and creates appropriate indicators
type ProgressManager struct {
	mode   OutputMode
	mu     sync.Mutex
	writer io.Writer
}

// NewProgressManager creates a new progress manager with auto-detected mode
func NewProgressManager() *ProgressManager {
	mode := OutputModeInteractive
	if IsCI() {
		mode = OutputModeCI
	}

	return &ProgressManager{
		mode:   mode,
		writer: os.Stderr,
	}
}

// NewProgressManagerWithMode creates a progress manager with explicit mode
func NewProgressManagerWithMode(mode OutputMode) *ProgressManager {
	return &ProgressManager{
		mode:   mode,
		writer: os.Stderr,
	}
}

// Mode returns the current output mode
func (pm *ProgressManager) Mode() OutputMode {
	pm.mu.Lock()
	defer pm.mu.Unlock()
	return pm.mode
}

// Spinner creates a new spinner with the given message
func (pm *ProgressManager) Spinner(message string) *Spinner {
	return NewSpinner(message, pm.mode)
}

// ProgressBar creates a new progress bar
func (pm *ProgressManager) ProgressBar(total int, message string) *ProgressBar {
	return NewProgressBar(total, message, pm.mode)
}

// Phase creates a phase indicator
func (pm *ProgressManager) Phase(name string) *PhaseIndicator {
	return &PhaseIndicator{
		name:   name,
		mode:   pm.mode,
		writer: pm.writer,
	}
}

// Printf prints a formatted message (respects output mode)
func (pm *ProgressManager) Printf(format string, args ...interface{}) {
	fmt.Fprintf(pm.writer, format, args...)
}

// Println prints a message with newline (respects output mode)
func (pm *ProgressManager) Println(message string) {
	fmt.Fprintln(pm.writer, message)
}

// Success prints a success message
func (pm *ProgressManager) Success(message string) {
	fmt.Fprintf(pm.writer, "✓ %s\n", message)
}

// Error prints an error message
func (pm *ProgressManager) Error(message string) {
	fmt.Fprintf(pm.writer, "✗ %s\n", message)
}

// Info prints an informational message
func (pm *ProgressManager) Info(message string) {
	if pm.mode == OutputModeInteractive {
		fmt.Fprintf(pm.writer, "ℹ %s\n", message)
	} else {
		fmt.Fprintf(pm.writer, "INFO: %s\n", message)
	}
}

// Warning prints a warning message
func (pm *ProgressManager) Warning(message string) {
	if pm.mode == OutputModeInteractive {
		fmt.Fprintf(pm.writer, "⚠ %s\n", message)
	} else {
		fmt.Fprintf(pm.writer, "WARNING: %s\n", message)
	}
}

// PhaseIndicator represents a phase in a multi-step process
type PhaseIndicator struct {
	name   string
	mode   OutputMode
	writer io.Writer
}

// Start starts the phase
func (pi *PhaseIndicator) Start() {
	if pi.mode == OutputModeInteractive {
		fmt.Fprintf(pi.writer, "▶ %s\n", pi.name)
	} else {
		fmt.Fprintf(pi.writer, "==> %s\n", pi.name)
	}
}

// Complete marks the phase as complete
func (pi *PhaseIndicator) Complete() {
	if pi.mode == OutputModeInteractive {
		fmt.Fprintf(pi.writer, "✓ %s completed\n", pi.name)
	} else {
		fmt.Fprintf(pi.writer, "==> %s: DONE\n", pi.name)
	}
}

// Fail marks the phase as failed
func (pi *PhaseIndicator) Fail(err error) {
	if pi.mode == OutputModeInteractive {
		fmt.Fprintf(pi.writer, "✗ %s failed: %v\n", pi.name, err)
	} else {
		fmt.Fprintf(pi.writer, "==> %s: FAILED: %v\n", pi.name, err)
	}
}

// PTermManager creates a PTerm manager configured for this ProgressManager's mode
// This allows seamless integration with PTerm-based UI components
func (pm *ProgressManager) PTermManager() *pterm.PTermManager {
	return pterm.NewPTermManager(pm.mode)
}
