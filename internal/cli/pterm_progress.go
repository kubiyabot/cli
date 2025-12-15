package cli

import (
	"fmt"
	"sync"

	"github.com/kubiyabot/cli/internal/pterm"
	ptermlib "github.com/pterm/pterm"
)

// PTermProgressBar is a PTerm-backed implementation of the ProgressBar interface
// It maintains the same API as the custom ProgressBar but uses PTerm internally
type PTermProgressBar struct {
	bar     *ptermlib.ProgressbarPrinter
	manager *pterm.PTermManager
	spinner *ptermlib.SpinnerPrinter
	current int
	total   int
	message string
	stage   string
	mu      sync.Mutex
	done    bool
	useBar  bool // true for progress bar, false for spinner
}

// NewPTermProgressBar creates a new PTerm-backed progress bar
func NewPTermProgressBar(manager *pterm.PTermManager, title string, total int) *PTermProgressBar {
	pb := &PTermProgressBar{
		manager: manager,
		total:   total,
		current: 0,
		useBar:  total > 0, // Use bar if we have a known total, otherwise use spinner
	}

	if manager.IsDisabled() {
		// In CI mode, just print the title
		fmt.Printf("▶ %s\n", title)
		return pb
	}

	if pb.useBar {
		// Create progress bar for known work
		bar, err := manager.ProgressBar(title, total)
		if err == nil && bar != nil {
			pb.bar, _ = bar.Start()
		}
	} else {
		// Create spinner for unknown work duration
		spinner := manager.Spinner(title)
		if spinner != nil {
			pb.spinner, _ = spinner.Start()
		}
	}

	return pb
}

// Update updates the progress bar with new progress and message
func (pb *PTermProgressBar) Update(progress int, message string) {
	pb.mu.Lock()
	defer pb.mu.Unlock()

	if pb.done {
		return
	}

	pb.current = progress
	pb.message = message

	if pb.manager.IsDisabled() {
		// In CI mode, print progress updates
		if pb.useBar {
			fmt.Printf("  [%d%%] %s\n", progress, message)
		} else {
			fmt.Printf("  %s\n", message)
		}
		return
	}

	if pb.useBar && pb.bar != nil {
		// Update progress bar
		increment := progress - pb.current
		if increment > 0 {
			pb.bar.Add(increment)
		}
		if message != "" {
			pb.bar.UpdateTitle(message)
		}
	} else if pb.spinner != nil {
		// Update spinner message
		pb.spinner.UpdateText(message)
	}
}

// SetStage sets the current stage name
func (pb *PTermProgressBar) SetStage(stage string) {
	pb.mu.Lock()
	defer pb.mu.Unlock()

	if pb.done {
		return
	}

	pb.stage = stage

	if pb.manager.IsDisabled() {
		fmt.Printf("\n▶ Stage: %s\n", stage)
		return
	}

	// Show stage as a section
	if !pb.manager.IsDisabled() {
		ptermlib.DefaultSection.Println(stage)
	}
}

// Complete marks the progress as complete
func (pb *PTermProgressBar) Complete() {
	pb.mu.Lock()
	defer pb.mu.Unlock()

	if pb.done {
		return
	}

	pb.done = true

	if pb.manager.IsDisabled() {
		fmt.Println("✓ Complete")
		return
	}

	// Stop progress bar or spinner
	if pb.useBar && pb.bar != nil {
		pb.bar.Stop()
	} else if pb.spinner != nil {
		pb.spinner.Success()
	}
}

// ShowThinking displays a thinking message
func (pb *PTermProgressBar) ShowThinking(message string) {
	pb.mu.Lock()
	defer pb.mu.Unlock()

	if pb.manager.IsDisabled() {
		fmt.Printf("  💭 %s\n", message)
		return
	}

	logger := pb.manager.Logger()
	logger.Debug(fmt.Sprintf("💭 %s", message))
}

// ShowToolCall displays a tool call message
func (pb *PTermProgressBar) ShowToolCall(toolName string) {
	pb.mu.Lock()
	defer pb.mu.Unlock()

	if pb.manager.IsDisabled() {
		fmt.Printf("  🔧 Using tool: %s\n", toolName)
		return
	}

	logger := pb.manager.Logger()
	logger.Info(fmt.Sprintf("🔧 Using tool: %s", toolName))
}

// ShowResourcesSummary displays a resources summary
func (pb *PTermProgressBar) ShowResourcesSummary(summary string) {
	pb.mu.Lock()
	defer pb.mu.Unlock()

	if pb.manager.IsDisabled() {
		fmt.Printf("\n📊 Resources Summary\n%s\n", summary)
		return
	}

	// Use PTerm box for resources summary
	box := pb.manager.Box().
		WithTitle("📊 Resources Summary").
		WithTitleTopCenter()

	box.Println(summary)
}

// ShowToolResult displays the result of a tool execution
func (pb *PTermProgressBar) ShowToolResult(toolName string, status string, duration float64) {
	pb.mu.Lock()
	defer pb.mu.Unlock()

	if pb.manager.IsDisabled() {
		if status == "success" {
			fmt.Printf("  ✓ %s completed (%.2fs)\n", toolName, duration)
		} else {
			fmt.Printf("  ✗ %s failed (%.2fs)\n", toolName, duration)
		}
		return
	}

	logger := pb.manager.Logger()
	if status == "success" {
		logger.Success(fmt.Sprintf("✓ %s completed (%.2fs)", toolName, duration))
	} else {
		logger.Error(fmt.Sprintf("✗ %s failed (%.2fs)", toolName, duration))
	}
}

// ShowStepStarted displays when a workflow step starts
func (pb *PTermProgressBar) ShowStepStarted(stepName string, stepDescription string) {
	pb.mu.Lock()
	defer pb.mu.Unlock()

	if pb.manager.IsDisabled() {
		fmt.Printf("\n▶ %s\n  %s\n", stepName, stepDescription)
		return
	}

	// Use PTerm section for step changes
	ptermlib.DefaultSection.Println(fmt.Sprintf("⚙️  %s", stepName))
	logger := pb.manager.Logger()
	logger.Info(stepDescription)
}

// ShowStepCompleted displays when a workflow step completes
func (pb *PTermProgressBar) ShowStepCompleted(stepName string) {
	pb.mu.Lock()
	defer pb.mu.Unlock()

	if pb.manager.IsDisabled() {
		fmt.Printf("  ✓ %s complete\n", stepName)
		return
	}

	logger := pb.manager.Logger()
	logger.Success(fmt.Sprintf("✓ %s complete", stepName))
}

// ShowError displays an error message
func (pb *PTermProgressBar) ShowError(message string) {
	pb.mu.Lock()
	defer pb.mu.Unlock()

	if pb.manager.IsDisabled() {
		fmt.Printf("\n✗ Error: %s\n", message)
		return
	}

	// Use PTerm error box
	box := pb.manager.Box().
		WithTitle("✗ Error").
		WithTitleTopCenter().
		WithBoxStyle(ptermlib.NewStyle(ptermlib.FgRed))

	box.Println(message)
}

// PTermSpinner is a PTerm-backed spinner implementation
type PTermSpinner struct {
	spinner *ptermlib.SpinnerPrinter
	manager *pterm.PTermManager
	message string
	mu      sync.Mutex
	done    bool
}

// NewPTermSpinner creates a new PTerm-backed spinner
func NewPTermSpinner(manager *pterm.PTermManager, message string) *PTermSpinner {
	s := &PTermSpinner{
		manager: manager,
		message: message,
	}

	if manager.IsDisabled() {
		// In CI mode, just print the message
		fmt.Printf("▶ %s...\n", message)
		return s
	}

	spinner := manager.Spinner(message)
	if spinner != nil {
		s.spinner, _ = spinner.Start()
	}

	return s
}

// Update updates the spinner message
func (s *PTermSpinner) Update(message string) {
	s.mu.Lock()
	defer s.mu.Unlock()

	if s.done {
		return
	}

	s.message = message

	if s.manager.IsDisabled() {
		fmt.Printf("  %s\n", message)
		return
	}

	if s.spinner != nil {
		s.spinner.UpdateText(message)
	}
}

// Success marks the spinner as successful
func (s *PTermSpinner) Success(message string) {
	s.mu.Lock()
	defer s.mu.Unlock()

	if s.done {
		return
	}

	s.done = true

	if s.manager.IsDisabled() {
		fmt.Printf("✓ %s\n", message)
		return
	}

	if s.spinner != nil {
		s.spinner.Success(message)
	}
}

// Fail marks the spinner as failed
func (s *PTermSpinner) Fail(message string) {
	s.mu.Lock()
	defer s.mu.Unlock()

	if s.done {
		return
	}

	s.done = true

	if s.manager.IsDisabled() {
		fmt.Printf("✗ %s\n", message)
		return
	}

	if s.spinner != nil {
		s.spinner.Fail(message)
	}
}

// Stop stops the spinner
func (s *PTermSpinner) Stop() {
	s.mu.Lock()
	defer s.mu.Unlock()

	if s.done {
		return
	}

	s.done = true

	if s.manager.IsDisabled() {
		return
	}

	if s.spinner != nil {
		s.spinner.Stop()
	}
}

// Warning displays a warning message
func (s *PTermSpinner) Warning(message string) {
	s.mu.Lock()
	defer s.mu.Unlock()

	if s.done {
		return
	}

	s.done = true

	if s.manager.IsDisabled() {
		fmt.Printf("⚠ %s\n", message)
		return
	}

	if s.spinner != nil {
		s.spinner.Warning(message)
	}
}
