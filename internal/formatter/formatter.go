package formatter

import (
	"fmt"
	"os"
	"strings"
	"text/tabwriter"
	"time"

	"github.com/kubiyabot/cli/internal/style"
)

// TableFormatter handles consistent table output across all commands
type TableFormatter struct {
	writer  *tabwriter.Writer
	headers []string
	rows    [][]string
}

// NewTable creates a new table formatter with styled headers
func NewTable(headers ...string) *TableFormatter {
	return &TableFormatter{
		writer:  tabwriter.NewWriter(os.Stdout, 0, 0, 3, ' ', 0),
		headers: headers,
		rows:    [][]string{},
	}
}

// AddRow adds a row to the table
func (t *TableFormatter) AddRow(columns ...string) {
	t.rows = append(t.rows, columns)
}

// Render displays the table with styled headers
func (t *TableFormatter) Render() {
	// Print styled header
	headerStr := strings.Join(t.headers, "\t")
	fmt.Fprintln(t.writer, style.TableHeaderStyle.Render(headerStr))
	fmt.Fprintln(t.writer, style.CreateDivider(100))

	// Print rows
	for _, row := range t.rows {
		fmt.Fprintln(t.writer, strings.Join(row, "\t"))
	}

	t.writer.Flush()
}

// ListOutput handles list command output with banner and optional callback
func ListOutput(title string, icon string, count int, tableFunc func()) {
	fmt.Println()
	fmt.Println(style.CreateBanner(fmt.Sprintf("%s (%d)", title, count), icon))
	fmt.Println()
	tableFunc()
	fmt.Println()
}

// DetailOutput handles get command output with banner and metadata box
func DetailOutput(title string, icon string, fields map[string]string) {
	fmt.Println()
	fmt.Println(style.CreateBanner(title, icon))
	fmt.Println()
	fmt.Println(style.CreateMetadataBox(fields))
	fmt.Println()
}

// SuccessMessage displays operation success with optional details
func SuccessMessage(message string, details map[string]string) {
	fmt.Println()
	fmt.Println(style.CreateSuccessBox(message))
	if len(details) > 0 {
		fmt.Println()
		fmt.Println(style.CreateMetadataBox(details))
	}
	fmt.Println()
}

// ErrorMessage displays operation error
func ErrorMessage(message string) {
	fmt.Println()
	fmt.Println(style.CreateErrorBox(message))
	fmt.Println()
}

// WarningMessage displays a warning
func WarningMessage(message string) {
	fmt.Println()
	fmt.Println(style.CreateWarningBox(message))
	fmt.Println()
}

// EmptyListMessage displays a message when no items are found
func EmptyListMessage(resourceType string) {
	fmt.Println(style.CreateHelpBox(fmt.Sprintf("No %s found", resourceType)))
}

// TruncateID truncates UUIDs for display (shows first 12 chars)
func TruncateID(id string) string {
	if len(id) <= 12 {
		return id
	}
	return id[:12] + "..."
}

// FormatStatus returns a styled status badge
func FormatStatus(status string) string {
	return style.CreateStatusBadge(status)
}

// FormatBoolean returns a styled boolean indicator
func FormatBoolean(value bool) string {
	if value {
		return style.SuccessStyle.Render("✓")
	}
	return style.DimStyle.Render("✗")
}

// FormatBooleanYesNo returns styled Yes/No
func FormatBooleanYesNo(value bool) string {
	if value {
		return style.SuccessStyle.Render("Yes")
	}
	return style.DimStyle.Render("No")
}

// FormatDate formats a timestamp for display
func FormatDate(t *time.Time) string {
	if t == nil {
		return style.DimStyle.Render("N/A")
	}
	return style.DimStyle.Render(t.Format("2006-01-02 15:04"))
}

// FormatCustomTime formats a CustomTime timestamp for display
// This is a helper for entities that use CustomTime wrapper
func FormatCustomTime(t interface{}) string {
	if t == nil {
		return style.DimStyle.Render("N/A")
	}

	// Handle pointer to CustomTime or other time types
	switch v := t.(type) {
	case *time.Time:
		return FormatDate(v)
	default:
		// Try to use reflection to get the embedded time.Time
		// CustomTime embeds time.Time, so we can format it directly
		if timeVal, ok := interface{}(t).(interface{ Format(string) string }); ok {
			formatted := timeVal.Format("2006-01-02 15:04")
			return style.DimStyle.Render(formatted)
		}
		return style.DimStyle.Render("N/A")
	}
}

// FormatDateString formats a date string
func FormatDateString(dateStr string) string {
	if dateStr == "" {
		return style.DimStyle.Render("N/A")
	}
	// Try to parse and format
	t, err := time.Parse(time.RFC3339, dateStr)
	if err != nil {
		// If parsing fails, return as-is
		return style.DimStyle.Render(dateStr)
	}
	return style.DimStyle.Render(t.Format("2006-01-02"))
}

// FormatCount formats a count with styling
func FormatCount(count int) string {
	if count == 0 {
		return style.DimStyle.Render("0")
	}
	return style.NumberStyle.Render(fmt.Sprintf("%d", count))
}

// FormatOptionalString formats an optional string pointer
func FormatOptionalString(s *string, defaultValue string) string {
	if s == nil || *s == "" {
		return style.DimStyle.Render(defaultValue)
	}
	return *s
}

// TruncateString truncates a string to maxLen with ellipsis
func TruncateString(s string, maxLen int) string {
	if len(s) <= maxLen {
		return s
	}
	if maxLen <= 3 {
		return s[:maxLen]
	}
	return s[:maxLen-3] + "..."
}

// StyledID returns a styled ID string
func StyledID(id string) string {
	return style.ValueStyle.Render(TruncateID(id))
}

// StyledName returns a styled name string
func StyledName(name string) string {
	return style.InfoStyle.Render(name)
}

// StyledValue returns a styled value string
func StyledValue(value string) string {
	return style.ValueStyle.Render(value)
}

// StyledDim returns a dimmed styled string
func StyledDim(value string) string {
	return style.DimStyle.Render(value)
}
