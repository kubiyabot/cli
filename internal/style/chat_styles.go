package style

import (
	"fmt"
	"strings"

	"github.com/charmbracelet/lipgloss"
)

// Chat UI Styles - Beautiful and modern
var (
	// Banner and Header
	ChatBannerStyle = lipgloss.NewStyle().
			Bold(true).
			Foreground(lipgloss.Color("#A78BFA")).
			Background(lipgloss.Color("#1E1B29")).
			Padding(1, 3).
			MarginBottom(1).
			Border(lipgloss.DoubleBorder()).
			BorderForeground(lipgloss.Color("#8B5CF6"))

	ChatHeaderStyle = lipgloss.NewStyle().
			Bold(true).
			Foreground(lipgloss.Color("#C4B5FD")).
			MarginTop(1)

	// Message Bubbles
	UserMessageStyle = lipgloss.NewStyle().
			Bold(true).
			Foreground(lipgloss.Color("#10B981")).
			PaddingLeft(0)

	AssistantMessageStyle = lipgloss.NewStyle().
			Bold(true).
			Foreground(lipgloss.Color("#60A5FA")).
			PaddingLeft(0)

	UserPromptStyle = lipgloss.NewStyle().
			Bold(true).
			Foreground(lipgloss.Color("#10B981")).
			Background(lipgloss.Color("#064E3B")).
			Padding(0, 1).
			MarginRight(1)

	AssistantPromptStyle = lipgloss.NewStyle().
			Bold(true).
			Foreground(lipgloss.Color("#60A5FA")).
			Background(lipgloss.Color("#1E3A8A")).
			Padding(0, 1).
			MarginRight(1)

	// Status and Metadata
	ExecutionMetadataStyle = lipgloss.NewStyle().
				Border(lipgloss.RoundedBorder()).
				BorderForeground(lipgloss.Color("#6B7280")).
				Padding(0, 2).
				MarginTop(1).
				MarginBottom(1)

	MetadataKeyStyle = lipgloss.NewStyle().
				Foreground(lipgloss.Color("#9CA3AF")).
				Bold(true)

	MetadataValueStyle = lipgloss.NewStyle().
				Foreground(lipgloss.Color("#D1D5DB"))

	StatusBadgeRunning = lipgloss.NewStyle().
				Bold(true).
				Foreground(lipgloss.Color("#FFFFFF")).
				Background(lipgloss.Color("#10B981")).
				Padding(0, 1)

	StatusBadgePending = lipgloss.NewStyle().
				Bold(true).
				Foreground(lipgloss.Color("#1F2937")).
				Background(lipgloss.Color("#FBBF24")).
				Padding(0, 1)

	StatusBadgeCompleted = lipgloss.NewStyle().
				Bold(true).
				Foreground(lipgloss.Color("#FFFFFF")).
				Background(lipgloss.Color("#8B5CF6")).
				Padding(0, 1)

	StatusBadgeFailed = lipgloss.NewStyle().
				Bold(true).
				Foreground(lipgloss.Color("#FFFFFF")).
				Background(lipgloss.Color("#EF4444")).
				Padding(0, 1)

	// Progress and Loading
	ThinkingStyle = lipgloss.NewStyle().
			Foreground(lipgloss.Color("#6B7280")).
			Italic(true).
			Blink(true)

	ProcessingDotStyle = lipgloss.NewStyle().
				Foreground(lipgloss.Color("#60A5FA")).
				Bold(true)

	// Dividers
	ChatDividerStyle = lipgloss.NewStyle().
				Foreground(lipgloss.Color("#374151"))

	SectionDividerStyle = lipgloss.NewStyle().
				Foreground(lipgloss.Color("#4B5563")).
				Bold(true)

	// Info and Help
	HelpBoxStyle = lipgloss.NewStyle().
			Border(lipgloss.RoundedBorder()).
			BorderForeground(lipgloss.Color("#60A5FA")).
			Foreground(lipgloss.Color("#93C5FD")).
			Padding(1, 2).
			MarginTop(1).
			MarginBottom(1)

	InstructionStyle = lipgloss.NewStyle().
				Foreground(lipgloss.Color("#9CA3AF")).
				Italic(true)

	KeyboardShortcutStyle = lipgloss.NewStyle().
				Foreground(lipgloss.Color("#FBBF24")).
				Background(lipgloss.Color("#78350F")).
				Padding(0, 1).
				Bold(true)

	// Success and Error
	SuccessBoxStyle = lipgloss.NewStyle().
			Border(lipgloss.RoundedBorder()).
			BorderForeground(lipgloss.Color("#10B981")).
			Foreground(lipgloss.Color("#10B981")).
			Padding(1, 2).
			MarginTop(1).
			MarginBottom(1)

	ErrorBoxStyle = lipgloss.NewStyle().
			Border(lipgloss.RoundedBorder()).
			BorderForeground(lipgloss.Color("#EF4444")).
			Foreground(lipgloss.Color("#EF4444")).
			Padding(1, 2).
			MarginTop(1).
			MarginBottom(1)

	WarningBoxStyle = lipgloss.NewStyle().
			Border(lipgloss.RoundedBorder()).
			BorderForeground(lipgloss.Color("#F59E0B")).
			Foreground(lipgloss.Color("#F59E0B")).
			Padding(1, 2).
			MarginTop(1).
			MarginBottom(1)

	// Icons and Emojis
	RobotIconStyle = lipgloss.NewStyle().
			Foreground(lipgloss.Color("#8B5CF6")).
			Bold(true)

	UserIconStyle = lipgloss.NewStyle().
			Foreground(lipgloss.Color("#10B981")).
			Bold(true)

	// Agent Info
	AgentInfoBoxStyle = lipgloss.NewStyle().
				Border(lipgloss.RoundedBorder()).
				BorderForeground(lipgloss.Color("#8B5CF6")).
				Padding(1, 3).
				MarginBottom(1)

	AgentNameLabelStyle = lipgloss.NewStyle().
				Bold(true).
				Foreground(lipgloss.Color("#C4B5FD")).
				Background(lipgloss.Color("#5B21B6")).
				Padding(0, 2)

	AgentDetailStyle = lipgloss.NewStyle().
				Foreground(lipgloss.Color("#D1D5DB"))

	// Response Container
	ResponseContainerStyle = lipgloss.NewStyle().
				Border(lipgloss.NormalBorder(), false, false, false, true).
				BorderForeground(lipgloss.Color("#60A5FA")).
				PaddingLeft(2).
				MarginTop(1).
				MarginBottom(1)

	// Goodbye Message
	GoodbyeBoxStyle = lipgloss.NewStyle().
			Border(lipgloss.DoubleBorder()).
			BorderForeground(lipgloss.Color("#8B5CF6")).
			Foreground(lipgloss.Color("#A78BFA")).
			Padding(1, 3).
			MarginTop(1).
			Bold(true).
			Align(lipgloss.Center)
)

// Helper functions for creating beautiful UI elements

// CreateBanner creates a beautiful banner for chat sessions
func CreateBanner(title string, icon string) string {
	content := fmt.Sprintf("%s  %s", icon, title)
	return ChatBannerStyle.Render(content)
}

// CreateMetadataBox creates a styled metadata display
func CreateMetadataBox(items map[string]string) string {
	var lines []string
	for key, value := range items {
		line := fmt.Sprintf("%s %s",
			MetadataKeyStyle.Render(key+":"),
			MetadataValueStyle.Render(value))
		lines = append(lines, line)
	}
	content := strings.Join(lines, "\n")
	return ExecutionMetadataStyle.Render(content)
}

// CreateStatusBadge creates a colored status badge
func CreateStatusBadge(status string) string {
	statusLower := strings.ToLower(status)
	switch statusLower {
	case "running":
		return StatusBadgeRunning.Render(" RUNNING ")
	case "pending":
		return StatusBadgePending.Render(" PENDING ")
	case "completed":
		return StatusBadgeCompleted.Render(" COMPLETED ")
	case "failed":
		return StatusBadgeFailed.Render(" FAILED ")
	default:
		return StatusBadgePending.Render(fmt.Sprintf(" %s ", strings.ToUpper(status)))
	}
}

// CreateDivider creates a horizontal divider
func CreateDivider(width int) string {
	return ChatDividerStyle.Render(strings.Repeat("─", width))
}

// CreateSectionDivider creates a thick section divider
func CreateSectionDivider(width int) string {
	return SectionDividerStyle.Render(strings.Repeat("━", width))
}

// CreateHelpBox creates a styled help/info box
func CreateHelpBox(content string) string {
	return HelpBoxStyle.Render(content)
}

// CreateSuccessBox creates a success message box
func CreateSuccessBox(message string) string {
	return SuccessBoxStyle.Render(fmt.Sprintf("✓ %s", message))
}

// CreateErrorBox creates an error message box
func CreateErrorBox(message string) string {
	return ErrorBoxStyle.Render(fmt.Sprintf("✗ %s", message))
}

// CreateWarningBox creates a warning message box
func CreateWarningBox(message string) string {
	return WarningBoxStyle.Render(fmt.Sprintf("⚠ %s", message))
}

// CreateAgentInfoBox creates a beautiful agent information display
func CreateAgentInfoBox(name, id, runtime, status string) string {
	var lines []string

	// Agent name header
	lines = append(lines, AgentNameLabelStyle.Render(" "+name+" "))
	lines = append(lines, "")

	// Details
	lines = append(lines, fmt.Sprintf("%s  %s",
		MetadataKeyStyle.Render("ID:"),
		AgentDetailStyle.Render(id)))

	lines = append(lines, fmt.Sprintf("%s  %s",
		MetadataKeyStyle.Render("Runtime:"),
		AgentDetailStyle.Render(runtime)))

	lines = append(lines, fmt.Sprintf("%s  %s",
		MetadataKeyStyle.Render("Status:"),
		CreateStatusBadge(status)))

	content := strings.Join(lines, "\n")
	return AgentInfoBoxStyle.Render(content)
}

// CreateThinkingIndicator creates an animated thinking indicator
func CreateThinkingIndicator() string {
	return ThinkingStyle.Render("⠋ Thinking...")
}

// GetTerminalWidth returns a reasonable terminal width
func GetTerminalWidth() int {
	// Default to 80 if we can't detect
	return 80
}
