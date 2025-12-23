package streaming

import (
	"fmt"
	"io"
	"os"
	"strings"
	"sync"
	"time"

	"github.com/mattn/go-isatty"
)

// TextRenderer renders streaming events as formatted text with prefixes
type TextRenderer struct {
	out            io.Writer
	verbose        bool
	startTime      time.Time
	isTTY          bool
	mu             sync.Mutex
	lastWasChunk   bool   // Track if last output was a streaming chunk
	activeToolName string // Currently executing tool
	spinnerFrame   int    // For animated spinner

	// Output control fields (enhanced streaming UX)
	outputLines int  // Maximum lines per tool output (default: 10)
	fullOutput  bool // Show complete outputs without truncation
	compact     bool // Minimal output mode (tool names + status only)
}

// NewTextRenderer creates a new TextRenderer
func NewTextRenderer(out io.Writer, verbose bool) *TextRenderer {
	return NewTextRendererWithOptions(out, verbose, 10, false, false)
}

// NewTextRendererWithOptions creates a new TextRenderer with output control options
func NewTextRendererWithOptions(out io.Writer, verbose bool, outputLines int, fullOutput bool, compact bool) *TextRenderer {
	// Detect if output is a TTY for color support
	isTTY := false
	if f, ok := out.(*os.File); ok {
		isTTY = isatty.IsTerminal(f.Fd()) || isatty.IsCygwinTerminal(f.Fd())
	}

	if outputLines <= 0 {
		outputLines = 10 // default
	}

	return &TextRenderer{
		out:         out,
		verbose:     verbose,
		startTime:   time.Now(),
		isTTY:       isTTY,
		outputLines: outputLines,
		fullOutput:  fullOutput,
		compact:     compact,
	}
}

// RenderEvent renders a single streaming event as formatted text
func (r *TextRenderer) RenderEvent(event StreamEvent) error {
	r.mu.Lock()
	defer r.mu.Unlock()

	switch event.Type {
	case EventTypeConnected:
		return r.renderConnected(event)
	case EventTypeToolStarted:
		return r.renderToolStarted(event)
	case EventTypeToolCompleted:
		return r.renderToolCompleted(event)
	case EventTypeMessageChunk:
		return r.renderMessageChunk(event)
	case EventTypeMessage:
		return r.renderMessage(event)
	case EventTypeStatus:
		return r.renderStatus(event)
	case EventTypeProgress:
		return r.renderProgress(event)
	case EventTypeError:
		return r.renderError(event)
	case EventTypeDone:
		return r.renderDone(event)
	default:
		// Unknown event type - ignore silently
		return nil
	}
}

// Flush ensures all buffered output is written
func (r *TextRenderer) Flush() error {
	r.mu.Lock()
	defer r.mu.Unlock()

	// End any streaming content with a newline
	if r.lastWasChunk {
		fmt.Fprintln(r.out)
		r.lastWasChunk = false
	}
	return nil
}

// Close cleans up resources
func (r *TextRenderer) Close() error {
	return r.Flush()
}

func (r *TextRenderer) renderConnected(event StreamEvent) error {
	executionID := event.ExecutionID
	if len(executionID) > 11 {
		executionID = executionID[:11]
	}

	// Clear line and show connected message
	r.clearCurrentLine()

	box := r.createBox("CONNECTED", fmt.Sprintf("Execution %s", executionID), colorCyan)
	_, err := fmt.Fprintln(r.out, box)
	return err
}

func (r *TextRenderer) renderToolStarted(event StreamEvent) error {
	if event.Tool == nil {
		return nil
	}

	// In compact mode, skip tool start events (only show completion)
	if r.compact {
		r.activeToolName = event.Tool.Name
		return nil
	}

	// End any previous streaming content
	if r.lastWasChunk {
		fmt.Fprintln(r.out)
		r.lastWasChunk = false
	}

	r.activeToolName = event.Tool.Name
	cleanName := r.cleanToolName(event.Tool.Name)

	// Show tool with spinner
	spinner := r.getSpinner()
	icon := r.getToolIcon(event.Tool.Name)

	line := fmt.Sprintf("\n%s %s %s %s",
		r.colorize(spinner, colorYellow),
		r.colorize(icon, colorCyan),
		r.colorize(cleanName, colorCyan+colorBold),
		r.colorize("running...", colorDim))

	_, err := fmt.Fprint(r.out, line)
	if err != nil {
		return err
	}

	// Always show inputs (enhanced UX)
	if len(event.Tool.Inputs) > 0 {
		fmt.Fprintln(r.out)
		r.renderToolInputs(event.Tool.Name, event.Tool.Inputs)
	}

	return nil
}

func (r *TextRenderer) renderToolCompleted(event StreamEvent) error {
	if event.Tool == nil {
		return nil
	}

	// Clear the "running..." line
	r.clearCurrentLine()

	cleanName := r.cleanToolName(event.Tool.Name)
	icon := r.getToolIcon(event.Tool.Name)

	var statusIcon, statusColor string
	if event.Tool.Success {
		statusIcon = "✓"
		statusColor = colorGreen
	} else {
		statusIcon = "✗"
		statusColor = colorRed
	}

	duration := ""
	if event.Tool.DurationSeconds > 0 {
		duration = fmt.Sprintf(" %s", r.colorize(fmt.Sprintf("(%.1fs)", event.Tool.DurationSeconds), colorDim))
	}

	line := fmt.Sprintf("%s %s %s%s",
		r.colorize(statusIcon, statusColor),
		r.colorize(icon, colorCyan),
		r.colorize(cleanName, colorCyan),
		duration)

	_, err := fmt.Fprintln(r.out, line)
	if err != nil {
		return err
	}

	// Always show outputs (enhanced UX)
	if len(event.Tool.Outputs) > 0 {
		r.renderToolOutputs(event.Tool.Outputs, event.Tool.Success)
	}

	// Show error if failed
	if !event.Tool.Success && event.Tool.Error != "" {
		errLine := fmt.Sprintf("  %s %s",
			r.colorize("└─", colorRed),
			r.colorize(event.Tool.Error, colorRed))
		fmt.Fprintln(r.out, errLine)
	}

	r.activeToolName = ""
	return nil
}

func (r *TextRenderer) renderMessageChunk(event StreamEvent) error {
	if event.Message == nil || event.Message.Content == "" {
		return nil
	}

	// Skip "(no content)" placeholder
	if event.Message.Content == "(no content)" {
		return nil
	}

	// In compact mode, accumulate content but don't show chunks
	if r.compact {
		return nil
	}

	// If this is the first chunk after a tool or status, add newline
	if !r.lastWasChunk {
		// Add visual separator for assistant response
		fmt.Fprintln(r.out)
		assistantHeader := fmt.Sprintf("%s %s",
			r.colorize("💬", ""),
			r.colorize("Assistant:", colorBold))
		fmt.Fprintln(r.out, assistantHeader)
	}

	// Stream content directly for real-time effect
	content := event.Message.Content

	// Apply subtle styling to the response
	_, err := fmt.Fprint(r.out, r.colorize(content, colorWhite))
	r.lastWasChunk = true

	return err
}

func (r *TextRenderer) renderMessage(event StreamEvent) error {
	if event.Message == nil {
		return nil
	}

	// End any previous streaming content
	if r.lastWasChunk {
		fmt.Fprintln(r.out)
		r.lastWasChunk = false
	}

	role := event.Message.Role
	content := event.Message.Content

	// Truncate long messages in non-verbose mode
	if len(content) > 300 && !r.verbose {
		content = content[:300] + "..."
	}

	switch role {
	case "user":
		// Show user message with icon
		userHeader := fmt.Sprintf("\n%s %s",
			r.colorize("👤", ""),
			r.colorize("User:", colorBold))
		fmt.Fprintln(r.out, userHeader)
		fmt.Fprintln(r.out, r.colorize(content, colorDim))

	case "assistant":
		// Show assistant message with icon
		assistantHeader := fmt.Sprintf("\n%s %s",
			r.colorize("💬", ""),
			r.colorize("Assistant:", colorBold))
		fmt.Fprintln(r.out, assistantHeader)
		fmt.Fprintln(r.out, content)

	case "system":
		// Show system message subtly
		systemLine := fmt.Sprintf("%s %s",
			r.colorize("ℹ", colorBlue),
			r.colorize(content, colorDim))
		fmt.Fprintln(r.out, systemLine)

	case "tool":
		// Tool result - show inline
		fmt.Fprintln(r.out, r.colorize(content, colorDim))

	default:
		fmt.Fprintf(r.out, "[%s] %s\n", strings.ToUpper(role), content)
	}

	return nil
}

func (r *TextRenderer) renderStatus(event StreamEvent) error {
	if event.Status == nil {
		return nil
	}

	// End any previous streaming content
	if r.lastWasChunk {
		fmt.Fprintln(r.out)
		fmt.Fprintln(r.out) // Extra line for spacing
		r.lastWasChunk = false
	}

	state := strings.ToLower(event.Status.State)

	// Skip uninteresting status updates
	if state == "unknown" {
		return nil
	}

	var icon, color string
	var showStatus bool = true

	switch state {
	case "running":
		icon = "▶"
		color = colorGreen
		showStatus = false // Don't show running status - implied by activity
	case "completed", "done":
		icon = "✓"
		color = colorGreen
	case "failed", "error":
		icon = "✗"
		color = colorRed
	case "waiting_for_input", "paused":
		icon = "⏸"
		color = colorYellow
		// Show a nice completion message
		fmt.Fprintln(r.out)
		box := r.createBox("COMPLETE", "Waiting for input", colorGreen)
		fmt.Fprintln(r.out, box)
		return nil
	case "cancelled":
		icon = "⊘"
		color = colorYellow
	default:
		icon = "●"
		color = colorDim
	}

	if showStatus {
		statusLine := fmt.Sprintf("%s %s",
			r.colorize(icon, color),
			r.colorize(state, color))

		if event.Status.Reason != "" {
			statusLine += fmt.Sprintf(" - %s", r.colorize(event.Status.Reason, colorDim))
		}

		fmt.Fprintln(r.out, statusLine)
	}

	return nil
}

func (r *TextRenderer) renderProgress(event StreamEvent) error {
	if event.Progress == nil {
		return nil
	}

	// End any previous streaming content
	if r.lastWasChunk {
		fmt.Fprintln(r.out)
		r.lastWasChunk = false
	}

	stage := event.Progress.Stage
	message := event.Progress.Message
	percent := event.Progress.Percent

	// Create a simple progress bar
	barWidth := 20
	filled := (percent * barWidth) / 100
	bar := strings.Repeat("█", filled) + strings.Repeat("░", barWidth-filled)

	progressLine := fmt.Sprintf("%s %s [%s] %d%%",
		r.colorize("⏳", ""),
		r.colorize(stage, colorCyan),
		r.colorize(bar, colorBlue),
		percent)

	if message != "" {
		progressLine += fmt.Sprintf(" %s", r.colorize(message, colorDim))
	}

	// Use carriage return to update in place if TTY
	if r.isTTY {
		fmt.Fprintf(r.out, "\r%s", progressLine)
	} else {
		fmt.Fprintln(r.out, progressLine)
	}

	return nil
}

func (r *TextRenderer) renderError(event StreamEvent) error {
	if event.Error == nil {
		return nil
	}

	// End any previous streaming content
	if r.lastWasChunk {
		fmt.Fprintln(r.out)
		r.lastWasChunk = false
	}

	fmt.Fprintln(r.out)
	box := r.createBox("ERROR", event.Error.Message, colorRed)
	fmt.Fprintln(r.out, box)

	if event.Error.Code != "" {
		fmt.Fprintf(r.out, "  %s\n", r.colorize(fmt.Sprintf("Code: %s", event.Error.Code), colorDim))
	}

	return nil
}

func (r *TextRenderer) renderDone(event StreamEvent) error {
	// End any previous streaming content
	if r.lastWasChunk {
		fmt.Fprintln(r.out)
		fmt.Fprintln(r.out)
		r.lastWasChunk = false
	}

	elapsed := time.Since(r.startTime)
	box := r.createBox("DONE", fmt.Sprintf("Completed in %.1fs", elapsed.Seconds()), colorGreen)
	fmt.Fprintln(r.out, box)

	return nil
}

// Helper functions

func (r *TextRenderer) clearCurrentLine() {
	if r.isTTY {
		fmt.Fprint(r.out, "\r\033[K")
	}
}

func (r *TextRenderer) getSpinner() string {
	spinners := []string{"⠋", "⠙", "⠹", "⠸", "⠼", "⠴", "⠦", "⠧", "⠇", "⠏"}
	r.spinnerFrame = (r.spinnerFrame + 1) % len(spinners)
	return spinners[r.spinnerFrame]
}

func (r *TextRenderer) getToolIcon(toolName string) string {
	name := strings.ToLower(toolName)

	// MCP tools
	if strings.HasPrefix(name, "mcp__") {
		if strings.Contains(name, "context-graph") || strings.Contains(name, "memory") {
			return "🧠"
		}
		if strings.Contains(name, "grounding") {
			return "📊"
		}
		return "🔌"
	}

	// Common tools
	switch {
	case strings.Contains(name, "bash"), strings.Contains(name, "shell"), strings.Contains(name, "terminal"):
		return "💻"
	case strings.Contains(name, "read"), strings.Contains(name, "file"):
		return "📄"
	case strings.Contains(name, "write"), strings.Contains(name, "edit"):
		return "✏️"
	case strings.Contains(name, "search"), strings.Contains(name, "grep"), strings.Contains(name, "find"):
		return "🔍"
	case strings.Contains(name, "web"), strings.Contains(name, "http"), strings.Contains(name, "api"):
		return "🌐"
	case strings.Contains(name, "database"), strings.Contains(name, "sql"), strings.Contains(name, "query"):
		return "🗃️"
	case strings.Contains(name, "task"):
		return "📋"
	default:
		return "🔧"
	}
}

func (r *TextRenderer) cleanToolName(toolName string) string {
	if toolName == "" {
		return "Tool"
	}

	// For MCP tools, extract the action part
	if strings.HasPrefix(toolName, "mcp__") {
		parts := strings.Split(strings.TrimPrefix(toolName, "mcp__"), "__")
		if len(parts) >= 2 {
			// Return server:action format
			server := parts[0]
			action := strings.Join(parts[1:], "_")
			// Clean up action name
			action = strings.ReplaceAll(action, "_", " ")
			action = strings.Title(action)
			if len(action) > 25 {
				action = action[:22] + "..."
			}
			return fmt.Sprintf("%s: %s", server, action)
		}
	}

	// Clean standard tool names
	cleaned := strings.ReplaceAll(toolName, "_", " ")
	cleaned = strings.Title(cleaned)

	if len(cleaned) > 30 {
		cleaned = cleaned[:27] + "..."
	}

	return cleaned
}

func (r *TextRenderer) renderToolInputs(toolName string, inputs map[string]interface{}) {
	if len(inputs) == 0 {
		return
	}

	// Format inputs based on tool type for cleaner display
	formatted := r.formatToolInput(toolName, inputs)

	// For single-line inputs (like bash commands), show inline
	if !strings.Contains(formatted, "\n") {
		fmt.Fprintf(r.out, "   %s\n", r.colorize(formatted, colorDim))
		return
	}

	// For multi-line inputs, use box format
	fmt.Fprintln(r.out, r.colorize("   ┌─ Input ─────────────────────────", colorDim))
	for _, line := range strings.Split(formatted, "\n") {
		fmt.Fprintf(r.out, "   │ %s\n", r.colorize(line, colorDim))
	}
	fmt.Fprintln(r.out, r.colorize("   └──────────────────────────────────", colorDim))
}

// formatToolInput formats tool inputs based on tool type for better readability
func (r *TextRenderer) formatToolInput(toolName string, inputs map[string]interface{}) string {
	name := strings.ToLower(toolName)

	// Bash/Shell: show command inline with $ prefix
	if strings.Contains(name, "bash") || strings.Contains(name, "shell") || strings.Contains(name, "terminal") {
		if cmd, ok := inputs["command"].(string); ok {
			// Truncate very long commands
			if len(cmd) > 200 {
				cmd = cmd[:197] + "..."
			}
			return fmt.Sprintf("$ %s", cmd)
		}
	}

	// File operations: show path prominently
	if strings.Contains(name, "read") || strings.Contains(name, "write") || strings.Contains(name, "edit") {
		if path, ok := inputs["path"].(string); ok {
			return fmt.Sprintf("path: %s", path)
		}
		if path, ok := inputs["file_path"].(string); ok {
			return fmt.Sprintf("path: %s", path)
		}
	}

	// Web/API: show URL
	if strings.Contains(name, "web") || strings.Contains(name, "fetch") || strings.Contains(name, "http") {
		if url, ok := inputs["url"].(string); ok {
			return fmt.Sprintf("url: %s", url)
		}
	}

	// Search/Grep: show pattern
	if strings.Contains(name, "search") || strings.Contains(name, "grep") || strings.Contains(name, "find") {
		if pattern, ok := inputs["pattern"].(string); ok {
			return fmt.Sprintf("pattern: %s", pattern)
		}
	}

	// Default: format as key-value pairs (single line if few, multi-line if many)
	var parts []string
	for k, v := range inputs {
		valueStr := fmt.Sprintf("%v", v)
		if len(valueStr) > 80 {
			valueStr = valueStr[:77] + "..."
		}
		parts = append(parts, fmt.Sprintf("%s: %s", k, valueStr))
	}

	if len(parts) <= 2 {
		return strings.Join(parts, ", ")
	}
	return strings.Join(parts, "\n")
}

func (r *TextRenderer) renderToolOutputs(outputs map[string]interface{}, success bool) {
	if len(outputs) == 0 {
		return
	}

	// Skip outputs in compact mode
	if r.compact {
		return
	}

	// Determine max lines based on settings
	maxLines := r.outputLines
	if r.fullOutput {
		maxLines = 0 // unlimited
	} else if r.verbose {
		maxLines = 50 // Show more in verbose mode
	}

	// Determine output color based on success
	outputColor := colorWhite
	if !success {
		outputColor = colorRed
	}

	// Get the main output content
	var mainOutput string
	for _, value := range outputs {
		valueStr := fmt.Sprintf("%v", value)
		if len(valueStr) > len(mainOutput) {
			mainOutput = valueStr
		}
	}

	// Apply line-based truncation
	truncated, omitted := truncateByLines(mainOutput, maxLines)

	// Render the output box
	fmt.Fprintln(r.out, r.colorize("   ┌─ Output ─────────────────────────", colorDim))
	for _, line := range strings.Split(truncated, "\n") {
		// Truncate very long lines
		if len(line) > 100 {
			line = line[:97] + "..."
		}
		fmt.Fprintf(r.out, "   │ %s\n", r.colorize(line, outputColor))
	}

	// Show truncation indicator
	if omitted > 0 {
		indicator := fmt.Sprintf("+%d more lines", omitted)
		fmt.Fprintf(r.out, "   │ %s\n", r.colorize(indicator, colorYellow))
	}

	fmt.Fprintln(r.out, r.colorize("   └──────────────────────────────────", colorDim))
}

func (r *TextRenderer) createBox(title, content string, color string) string {
	titleLen := len(title)
	contentLen := len(content)
	width := titleLen + contentLen + 5

	if width < 30 {
		width = 30
	}

	topBorder := "╭" + strings.Repeat("─", width) + "╮"
	bottomBorder := "╰" + strings.Repeat("─", width) + "╯"

	innerContent := fmt.Sprintf("│ %s %s %s │",
		r.colorize(title, color+colorBold),
		r.colorize("•", colorDim),
		content)

	// Pad to width
	padding := width - titleLen - contentLen - 5
	if padding > 0 {
		innerContent = fmt.Sprintf("│ %s %s %s%s │",
			r.colorize(title, color+colorBold),
			r.colorize("•", colorDim),
			content,
			strings.Repeat(" ", padding))
	}

	return fmt.Sprintf("%s\n%s\n%s",
		r.colorize(topBorder, color),
		innerContent,
		r.colorize(bottomBorder, color))
}

// Color codes for terminal output
const (
	colorReset  = "\033[0m"
	colorBold   = "\033[1m"
	colorDim    = "\033[2m"
	colorRed    = "\033[31m"
	colorGreen  = "\033[32m"
	colorYellow = "\033[33m"
	colorBlue   = "\033[34m"
	colorCyan   = "\033[36m"
	colorWhite  = "\033[37m"
)

// colorize adds ANSI color codes if TTY is available
func (r *TextRenderer) colorize(text, color string) string {
	if !r.isTTY || color == "" {
		return text
	}
	return color + text + colorReset
}

// truncateByLines truncates content to maxLines, returning truncated content
// and the number of lines omitted
func truncateByLines(content string, maxLines int) (string, int) {
	if content == "" || maxLines <= 0 {
		return content, 0
	}

	lines := strings.Split(content, "\n")
	totalLines := len(lines)

	if totalLines <= maxLines {
		return content, 0
	}

	truncated := strings.Join(lines[:maxLines], "\n")
	omitted := totalLines - maxLines
	return truncated, omitted
}
