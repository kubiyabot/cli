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
}

// NewTextRenderer creates a new TextRenderer
func NewTextRenderer(out io.Writer, verbose bool) *TextRenderer {
	// Detect if output is a TTY for color support
	isTTY := false
	if f, ok := out.(*os.File); ok {
		isTTY = isatty.IsTerminal(f.Fd()) || isatty.IsCygwinTerminal(f.Fd())
	}

	return &TextRenderer{
		out:       out,
		verbose:   verbose,
		startTime: time.Now(),
		isTTY:     isTTY,
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

	// Show inputs in verbose mode
	if r.verbose && len(event.Tool.Inputs) > 0 {
		fmt.Fprintln(r.out)
		r.renderToolInputs(event.Tool.Inputs)
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

	// Show outputs in verbose mode
	if r.verbose && len(event.Tool.Outputs) > 0 {
		r.renderToolOutputs(event.Tool.Outputs)
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

func (r *TextRenderer) renderToolInputs(inputs map[string]interface{}) {
	if len(inputs) == 0 {
		return
	}

	fmt.Fprintln(r.out, r.colorize("  ┌─ Inputs:", colorDim))
	for key, value := range inputs {
		valueStr := fmt.Sprintf("%v", value)
		if len(valueStr) > 60 {
			valueStr = valueStr[:57] + "..."
		}
		fmt.Fprintf(r.out, "  │ %s: %s\n",
			r.colorize(key, colorCyan),
			r.colorize(valueStr, colorDim))
	}
	fmt.Fprintln(r.out, r.colorize("  └─", colorDim))
}

func (r *TextRenderer) renderToolOutputs(outputs map[string]interface{}) {
	if len(outputs) == 0 {
		return
	}

	fmt.Fprintln(r.out, r.colorize("  ┌─ Outputs:", colorDim))
	for key, value := range outputs {
		valueStr := fmt.Sprintf("%v", value)
		if len(valueStr) > 60 {
			valueStr = valueStr[:57] + "..."
		}
		fmt.Fprintf(r.out, "  │ %s: %s\n",
			r.colorize(key, colorCyan),
			r.colorize(valueStr, colorDim))
	}
	fmt.Fprintln(r.out, r.colorize("  └─", colorDim))
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
