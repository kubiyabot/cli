package style

import (
	"os"
	"strings"

	"github.com/charmbracelet/lipgloss"
	"github.com/mattn/go-isatty"
)

var (
	TitleStyle = lipgloss.NewStyle().
			Bold(true).
			Foreground(lipgloss.Color("#FAFAFA")).
			Background(lipgloss.Color("#7D56F4")).
			Padding(0, 1)

	SubtitleStyle = lipgloss.NewStyle().
			Bold(true).
			Foreground(lipgloss.Color("#7D56F4"))

	HighlightStyle = lipgloss.NewStyle().
			Bold(true).
			Foreground(lipgloss.Color("#7D56F4"))

	DimStyle = lipgloss.NewStyle().
			Foreground(lipgloss.Color("#666666"))

	ErrorStyle = lipgloss.NewStyle().
			Bold(true).
			Foreground(lipgloss.Color("#FF0000"))

	SuccessStyle = lipgloss.NewStyle().
			Bold(true).
			Foreground(lipgloss.Color("#00FF00"))

	WarningStyle = lipgloss.NewStyle().
			Bold(true).
			Foreground(lipgloss.Color("#FFAA00"))

	ToolStyle = lipgloss.NewStyle().
			Foreground(lipgloss.Color("#F59E0B")).
			Bold(true).
			Border(lipgloss.NormalBorder()).
			BorderForeground(lipgloss.Color("#F59E0B")).
			Padding(0, 1)

	ToolOutputStyle = lipgloss.NewStyle().
			Foreground(lipgloss.Color("#94A3B8")).
			Margin(0, 0, 0, 2)

	ToolCompleteStyle = lipgloss.NewStyle().
				Foreground(lipgloss.Color("#10B981")).
				Bold(true).
				Border(lipgloss.RoundedBorder()).
				BorderForeground(lipgloss.Color("#10B981")).
				Padding(0, 1)

	TeammateStyle = lipgloss.NewStyle().
			Foreground(lipgloss.Color("#3B82F6"))

	ToolArgsStyle = lipgloss.NewStyle().
			Foreground(lipgloss.Color("#64748B")).
			Margin(0, 0, 0, 2)

	ToolNameStyle = lipgloss.NewStyle().
			Foreground(lipgloss.Color("#0EA5E9")).
			Bold(true)

	SystemStyle = lipgloss.NewStyle().
			Foreground(lipgloss.Color("#6B7280")).
			Italic(true)

	TeammateNameStyle = lipgloss.NewStyle().
				Foreground(lipgloss.Color("#3B82F6")).
				Bold(true)

	ToolSummaryStyle = lipgloss.NewStyle().
				Foreground(lipgloss.Color("#6B7280")).
				Italic(true).
				Margin(0, 0, 0, 2)

	CodeBlockStyle = lipgloss.NewStyle().
			Foreground(lipgloss.Color("#A3BE8C")).
			Background(lipgloss.Color("#2E3440")).
			Padding(0, 1)

	ToolOutputHeaderStyle = lipgloss.NewStyle().
				Foreground(lipgloss.Color("#059669")).
				Bold(true).
				MarginLeft(2)

	ToolDividerStyle = lipgloss.NewStyle().
				Foreground(lipgloss.Color("#374151"))

	ToolHeaderStyle = lipgloss.NewStyle().
			Foreground(lipgloss.Color("#0EA5E9")).
			Bold(true)

	ToolStatusStyle = lipgloss.NewStyle().
			Bold(true)

	ToolOutputPrefixStyle = lipgloss.NewStyle().
				Foreground(lipgloss.Color("#64748B"))

	CommandStyle = lipgloss.NewStyle().
			Foreground(lipgloss.Color("#00FFFF")).
			Background(lipgloss.Color("#2E3440")).
			Padding(0, 1)
)

// DisableColors disables all color styling for non-TTY environments
func DisableColors() {
	// Create a no-op style
	noStyle := lipgloss.NewStyle()

	// Reset all styles to no-op
	TitleStyle = noStyle
	SubtitleStyle = noStyle
	HighlightStyle = noStyle
	DimStyle = noStyle
	ErrorStyle = noStyle
	SuccessStyle = noStyle
	WarningStyle = noStyle
	ToolStyle = noStyle
	ToolOutputStyle = noStyle
	ToolCompleteStyle = noStyle
	TeammateStyle = noStyle
	ToolArgsStyle = noStyle
	ToolNameStyle = noStyle
	SystemStyle = noStyle
	TeammateNameStyle = noStyle
	ToolSummaryStyle = noStyle
	CodeBlockStyle = noStyle
	ToolOutputHeaderStyle = noStyle
	ToolDividerStyle = noStyle
	ToolHeaderStyle = noStyle
	ToolStatusStyle = noStyle
	ToolOutputPrefixStyle = noStyle
	CommandStyle = noStyle
}

// Add this function to check if colors should be enabled
func ShouldUseColors() bool {
	// Check NO_COLOR environment variable
	if _, exists := os.LookupEnv("NO_COLOR"); exists {
		return false
	}

	// Check if stdout is a terminal
	return isatty.IsTerminal(os.Stdout.Fd())
}

// Add this helper function instead
func RepeatDivider(n int) string {
	return strings.Repeat("─", n)
}
