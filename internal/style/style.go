package style

import (
	"os"
	"strings"

	"github.com/charmbracelet/lipgloss"
	"github.com/mattn/go-isatty"
)

var (
	HeaderStyle = lipgloss.NewStyle().
			Bold(true).
			Foreground(lipgloss.Color("205")).
			Padding(0, 1)

	TitleStyle = lipgloss.NewStyle().
			Bold(true).
			Foreground(lipgloss.Color("205"))

	SubtitleStyle = lipgloss.NewStyle().
			Foreground(lipgloss.Color("39"))

	HighlightStyle = lipgloss.NewStyle().
			Foreground(lipgloss.Color("205")).
			Bold(true)

	DimStyle = lipgloss.NewStyle().
			Foreground(lipgloss.Color("241"))

	ErrorStyle = lipgloss.NewStyle().
			Foreground(lipgloss.Color("196")).
			Bold(true)

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

	StatusStyle = lipgloss.NewStyle().
			Foreground(lipgloss.Color("39")).
			Bold(true)

	// Additional styles for improved UI
	ActiveStyle = lipgloss.NewStyle().
			Bold(true).
			Foreground(lipgloss.Color("#10B981"))

	InactiveStyle = lipgloss.NewStyle().
			Foreground(lipgloss.Color("#EF4444"))

	TableHeaderStyle = lipgloss.NewStyle().
				Bold(true).
				Foreground(lipgloss.Color("#60A5FA")).
				Padding(0, 1)

	TableRowStyle = lipgloss.NewStyle().
			Padding(0, 1)

	TableBorderStyle = lipgloss.NewStyle().
				Foreground(lipgloss.Color("#374151"))

	InfoBoxStyle = lipgloss.NewStyle().
			Border(lipgloss.RoundedBorder()).
			BorderForeground(lipgloss.Color("#60A5FA")).
			Padding(1, 2).
			MarginTop(1).
			MarginBottom(1)

	SpinnerStyle = lipgloss.NewStyle().
			Foreground(lipgloss.Color("#60A5FA"))

	ProgressBarStyle = lipgloss.NewStyle().
				Foreground(lipgloss.Color("#10B981"))

	ProgressBarEmptyStyle = lipgloss.NewStyle().
				Foreground(lipgloss.Color("#374151"))

	CountStyle = lipgloss.NewStyle().
			Foreground(lipgloss.Color("#60A5FA")).
			Bold(true)

	KeyStyle = lipgloss.NewStyle().
			Foreground(lipgloss.Color("#F59E0B")).
			Bold(true)

	ValueStyle = lipgloss.NewStyle().
			Foreground(lipgloss.Color("#94A3B8"))

	HeadingStyle = lipgloss.NewStyle().
			Bold(true).
			Foreground(lipgloss.Color("#60A5FA")).
			MarginTop(1).
			MarginBottom(1)

	BulletStyle = lipgloss.NewStyle().
			Foreground(lipgloss.Color("#F59E0B"))

	HelpTextStyle = lipgloss.NewStyle().
			Foreground(lipgloss.Color("#94A3B8")).
			Italic(true)
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
	StatusStyle = noStyle

	// Disable additional styles
	ActiveStyle = noStyle
	InactiveStyle = noStyle
	TableHeaderStyle = noStyle
	TableRowStyle = noStyle
	TableBorderStyle = noStyle
	InfoBoxStyle = noStyle
	SpinnerStyle = noStyle
	ProgressBarStyle = noStyle
	ProgressBarEmptyStyle = noStyle
	CountStyle = noStyle
	KeyStyle = noStyle
	ValueStyle = noStyle
	HeadingStyle = noStyle
	BulletStyle = noStyle
	HelpTextStyle = noStyle
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
