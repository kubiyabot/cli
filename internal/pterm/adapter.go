package pterm

import (
	"os"

	"github.com/mattn/go-isatty"
	"github.com/pterm/pterm"
	"github.com/kubiyabot/cli/internal/types"
)

// PTermManager manages PTerm components with OutputMode awareness
type PTermManager struct {
	mode     types.OutputMode
	disabled bool
}

// NewPTermManager creates a new PTerm manager with appropriate configuration
func NewPTermManager(mode types.OutputMode) *PTermManager {
	pm := &PTermManager{
		mode:     mode,
		disabled: false,
	}

	// Check if PTerm should be disabled via environment variable
	if os.Getenv("KUBIYA_PTERM_ENABLED") == "false" {
		pm.disabled = true
		return pm
	}

	// Disable PTerm features in CI/non-TTY environments
	if mode == types.OutputModeCI || !isatty.IsTerminal(os.Stdout.Fd()) {
		pterm.DisableColor()
		pterm.DisableStyling()
		pm.disabled = true
	}

	// Apply Kubiya theme colors
	pm.applyKubiyaTheme()

	return pm
}

// applyKubiyaTheme configures PTerm with Kubiya's color scheme
func (pm *PTermManager) applyKubiyaTheme() {
	// Success color: #10B981 (green)
	pterm.Success = *pterm.Success.WithMessageStyle(pterm.NewStyle(pterm.FgLightGreen))

	// Error color: #EF4444 (red)
	pterm.Error = *pterm.Error.WithMessageStyle(pterm.NewStyle(pterm.FgLightRed))

	// Info color: #60A5FA (cyan/blue)
	pterm.Info = *pterm.Info.WithMessageStyle(pterm.NewStyle(pterm.FgLightCyan))

	// Warning color: #F59E0B (orange/yellow)
	pterm.Warning = *pterm.Warning.WithMessageStyle(pterm.NewStyle(pterm.FgYellow))
}

// ProgressBar creates a configured progress bar
func (pm *PTermManager) ProgressBar(title string, total int) (*pterm.ProgressbarPrinter, error) {
	if pm.disabled {
		// Return a no-op progress bar for CI mode
		return nil, nil
	}

	progressBar := pterm.DefaultProgressbar.
		WithTotal(total).
		WithTitle(title).
		WithShowCount(false).
		WithShowPercentage(true)

	return progressBar, nil
}

// Spinner creates a configured spinner
func (pm *PTermManager) Spinner(message string) *pterm.SpinnerPrinter {
	if pm.disabled {
		// Return a no-op spinner for CI mode
		return nil
	}

	spinner := pterm.DefaultSpinner.
		WithText(message).
		WithStyle(pterm.NewStyle(pterm.FgCyan))

	return spinner
}

// Table creates a configured table printer
func (pm *PTermManager) Table() *pterm.TablePrinter {
	if pm.disabled {
		// Return basic table for CI mode
		return &pterm.DefaultTable
	}

	table := pterm.DefaultTable.
		WithHasHeader(true).
		WithHeaderStyle(pterm.NewStyle(pterm.FgCyan, pterm.Bold)).
		WithBoxed(false)

	return table
}

// Box creates a configured box printer
func (pm *PTermManager) Box() *pterm.BoxPrinter {
	if pm.disabled {
		return &pterm.DefaultBox
	}

	box := pterm.DefaultBox.
		WithBoxStyle(pterm.NewStyle(pterm.FgCyan)).
		WithTitleTopCenter()

	return box
}

// BulletList creates a configured bullet list printer
func (pm *PTermManager) BulletList() *pterm.BulletListPrinter {
	if pm.disabled {
		return &pterm.DefaultBulletList
	}

	return pterm.DefaultBulletList.WithBulletStyle(pterm.NewStyle(pterm.FgCyan))
}

// Tree creates a configured tree printer
func (pm *PTermManager) Tree() *pterm.TreePrinter {
	if pm.disabled {
		return &pterm.DefaultTree
	}

	return &pterm.DefaultTree
}

// Section creates a configured section printer
func (pm *PTermManager) Section() *pterm.SectionPrinter {
	if pm.disabled {
		return &pterm.DefaultSection
	}

	return pterm.DefaultSection.WithStyle(pterm.NewStyle(pterm.FgCyan, pterm.Bold))
}

// Area creates a live updating area
func (pm *PTermManager) Area() *pterm.AreaPrinter {
	if pm.disabled {
		return &pterm.DefaultArea
	}

	return &pterm.DefaultArea
}

// IsDisabled returns whether PTerm is disabled
func (pm *PTermManager) IsDisabled() bool {
	return pm.disabled
}

// Mode returns the current output mode
func (pm *PTermManager) Mode() types.OutputMode {
	return pm.mode
}

// Success prints a success message
func (pm *PTermManager) Success(message string) {
	if pm.disabled {
		pterm.Println("✓", message)
	} else {
		pterm.Success.Println(message)
	}
}

// Error prints an error message
func (pm *PTermManager) Error(message string) {
	if pm.disabled {
		pterm.Println("✗", message)
	} else {
		pterm.Error.Println(message)
	}
}

// Info prints an info message
func (pm *PTermManager) Info(message string) {
	if pm.disabled {
		pterm.Println("ℹ", message)
	} else {
		pterm.Info.Println(message)
	}
}

// Warning prints a warning message
func (pm *PTermManager) Warning(message string) {
	if pm.disabled {
		pterm.Println("⚠", message)
	} else {
		pterm.Warning.Println(message)
	}
}
