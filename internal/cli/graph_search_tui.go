package cli

import (
	"context"
	"fmt"
	"os"
	"strings"

	"github.com/charmbracelet/bubbles/spinner"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
	"github.com/mattn/go-isatty"
	"github.com/kubiyabot/cli/internal/contextgraph"
	"github.com/kubiyabot/cli/internal/controlplane"
)

// Styles for graph search
var (
	searchHeaderStyle = lipgloss.NewStyle().
				Foreground(lipgloss.Color("69")).
				Bold(true)

	searchStageStyle = lipgloss.NewStyle().
				Foreground(lipgloss.Color("86")).
				Bold(true)

	searchToolStyle = lipgloss.NewStyle().
			Foreground(lipgloss.Color("208"))

	searchSuccessStyle = lipgloss.NewStyle().
				Foreground(lipgloss.Color("42"))

	searchAnswerStyle = lipgloss.NewStyle().
				Foreground(lipgloss.Color("252"))

	searchDimStyle = lipgloss.NewStyle().
			Foreground(lipgloss.Color("241"))
)

// Custom message types for graph search
type graphSearchEventMsg struct {
	event contextgraph.StreamEvent
}

type graphSearchCompleteMsg struct {
	result *graphSearchResult
}

type graphSearchErrorMsg struct {
	err error
}

// graphSearchResult holds the final search result
type graphSearchResult struct {
	answer        string
	sessionID     string
	toolCalls     []controlplane.ToolCall
	turnsUsed     int
	confidence    string
	suggestions   []string
	nodes         []controlplane.GraphNode
	relationships []controlplane.GraphRelationship
}

// searchStep represents a step in the search process
type searchStep struct {
	name       string
	status     string // "pending", "active", "complete", "skipped"
	message    string
	toolCalls  []string
	subMessage string // Live reasoning/progress message
}

// graphSearchModel is the bubbletea model for graph search
type graphSearchModel struct {
	// UI Components
	spinner spinner.Model

	// Query info
	query string

	// Current state
	sessionID     string
	currentStep   string
	currentReason string // Live reasoning message
	progress      int
	maxProgress   int // Track max to prevent backwards jumps
	steps         []*searchStep
	activeStepIdx int
	liveToolCalls []string // Current active tool calls
	answer        strings.Builder
	streaming     bool

	// Final result
	result *graphSearchResult
	done   bool
	err    error

	// Event channel
	eventChan <-chan contextgraph.StreamEvent
	ctx       context.Context
}

// initializeSteps creates the predefined step list
func initializeSteps() []*searchStep {
	return []*searchStep{
		{name: "Analyzing Query", status: "pending"},
		{name: "Fetching Graph Schema", status: "pending"},
		{name: "Executing Graph Searches", status: "pending"},
		{name: "Aggregating Results", status: "pending"},
		{name: "Synthesizing Answer", status: "pending"},
	}
}

// Init initializes the model
func (m graphSearchModel) Init() tea.Cmd {
	return tea.Batch(
		m.spinner.Tick,
		waitForGraphSearchEvent(m.eventChan),
	)
}

// Update handles all message types
func (m graphSearchModel) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.KeyMsg:
		// Handle Ctrl+C
		switch msg.String() {
		case "ctrl+c", "q":
			m.done = true
			m.err = fmt.Errorf("cancelled by user")
			return m, tea.Quit
		}

	case spinner.TickMsg:
		// Update spinner animation
		var cmd tea.Cmd
		m.spinner, cmd = m.spinner.Update(msg)
		return m, cmd

	case graphSearchEventMsg:
		// Process the search event
		return m.handleSearchEvent(msg.event)

	case graphSearchCompleteMsg:
		// Search completed successfully
		m.result = msg.result
		m.done = true
		return m, tea.Quit

	case graphSearchErrorMsg:
		// Handle error
		m.done = true
		m.err = msg.err
		return m, tea.Quit

	case tea.WindowSizeMsg:
		// Handle terminal resize
		return m, nil
	}

	return m, nil
}

// handleSearchEvent processes a single search event
func (m graphSearchModel) handleSearchEvent(event contextgraph.StreamEvent) (tea.Model, tea.Cmd) {
	switch event.Event {
	case "session":
		if sid, ok := event.Data["session_id"].(string); ok {
			m.sessionID = sid
		}

	case "progress":
		if msg, ok := event.Data["message"].(string); ok {
			m.currentReason = msg
			// Update active step based on message
			m.updateActiveStepFromMessage(msg)
		}
		if p, ok := event.Data["progress"].(float64); ok {
			// Only update progress if it's moving forward
			newProgress := int(p)
			if newProgress > m.maxProgress {
				m.progress = newProgress
				m.maxProgress = newProgress
			}
		}

	case "tool_call":
		if toolName, ok := event.Data["tool_name"].(string); ok {
			// Add to live tool calls
			toolDisplay := fmt.Sprintf("🔧 %s", toolName)
			m.liveToolCalls = append(m.liveToolCalls, toolDisplay)

			// Add to active step
			if m.activeStepIdx >= 0 && m.activeStepIdx < len(m.steps) {
				m.steps[m.activeStepIdx].toolCalls = append(
					m.steps[m.activeStepIdx].toolCalls,
					toolDisplay,
				)
			}
		}

	case "tool_result":
		if toolName, ok := event.Data["tool_name"].(string); ok {
			if summary, ok := event.Data["result_summary"].(string); ok {
				success, _ := event.Data["success"].(bool)
				icon := "✓"
				if !success {
					icon = "✗"
				}

				resultDisplay := fmt.Sprintf("%s %s: %s", icon, toolName, summary)

				// Update in live tool calls
				for i, tc := range m.liveToolCalls {
					if strings.Contains(tc, toolName) {
						m.liveToolCalls[i] = resultDisplay
						break
					}
				}

				// Update in active step
				if m.activeStepIdx >= 0 && m.activeStepIdx < len(m.steps) {
					for i, tc := range m.steps[m.activeStepIdx].toolCalls {
						if strings.Contains(tc, toolName) {
							m.steps[m.activeStepIdx].toolCalls[i] = resultDisplay
							break
						}
					}
				}
			}
		}

	case "step_complete":
		if _, ok := event.Data["step_name"].(string); ok {
			// Mark current step as complete and move to next
			if m.activeStepIdx >= 0 && m.activeStepIdx < len(m.steps) {
				m.steps[m.activeStepIdx].status = "complete"
			}
			// Move to next step
			if m.activeStepIdx+1 < len(m.steps) {
				m.activeStepIdx++
				m.steps[m.activeStepIdx].status = "active"
			}
		}

	case "partial_answer":
		// Accumulate streaming answer
		if text, ok := event.Data["text"].(string); ok {
			m.answer.WriteString(text)
			m.streaming = true
		}

		// Mark all steps complete and show answer
		for _, step := range m.steps {
			if step.status != "complete" {
				step.status = "complete"
			}
		}

	case "complete":
		// Parse final result
		result := &graphSearchResult{
			answer: m.answer.String(),
		}

		if ans, ok := event.Data["answer"].(string); ok {
			if result.answer == "" {
				result.answer = ans
			}
		}
		if sid, ok := event.Data["session_id"].(string); ok {
			result.sessionID = sid
		}
		if t, ok := event.Data["turns_used"].(float64); ok {
			result.turnsUsed = int(t)
		}
		if c, ok := event.Data["confidence"].(string); ok {
			result.confidence = c
		}

		// Parse tool calls
		if tc, ok := event.Data["tool_calls"].([]interface{}); ok {
			for _, t := range tc {
				if tcMap, ok := t.(map[string]interface{}); ok {
					toolCall := controlplane.ToolCall{}
					if name, ok := tcMap["tool_name"].(string); ok {
						toolCall.ToolName = name
					}
					if summary, ok := tcMap["result_summary"].(string); ok {
						toolCall.ResultSummary = summary
					}
					result.toolCalls = append(result.toolCalls, toolCall)
				}
			}
		}

		// Parse suggestions
		if s, ok := event.Data["suggestions"].([]interface{}); ok {
			for _, sug := range s {
				if str, ok := sug.(string); ok {
					result.suggestions = append(result.suggestions, str)
				}
			}
		}

		return m, func() tea.Msg {
			return graphSearchCompleteMsg{result: result}
		}

	case "error":
		errMsg := "unknown error"
		if msg, ok := event.Data["message"].(string); ok {
			errMsg = msg
		}
		return m, func() tea.Msg {
			return graphSearchErrorMsg{err: fmt.Errorf("%s", errMsg)}
		}
	}

	// Continue listening for next event
	return m, waitForGraphSearchEvent(m.eventChan)
}

// updateActiveStepFromMessage updates the active step based on progress message
func (m *graphSearchModel) updateActiveStepFromMessage(msg string) {
	msgLower := strings.ToLower(msg)

	// Map messages to steps
	if strings.Contains(msgLower, "analyzing query") || strings.Contains(msgLower, "initializing") {
		m.setActiveStep(0)
	} else if strings.Contains(msgLower, "fetching") && strings.Contains(msgLower, "schema") {
		m.setActiveStep(1)
	} else if strings.Contains(msgLower, "executing") && strings.Contains(msgLower, "search") {
		m.setActiveStep(2)
	} else if strings.Contains(msgLower, "aggregating") || strings.Contains(msgLower, "validating") {
		m.setActiveStep(3)
	} else if strings.Contains(msgLower, "synthesizing") || strings.Contains(msgLower, "answer") {
		m.setActiveStep(4)
	}
}

// setActiveStep sets a step as active
func (m *graphSearchModel) setActiveStep(idx int) {
	if idx < 0 || idx >= len(m.steps) {
		return
	}

	// Mark all previous steps as complete
	for i := 0; i < idx; i++ {
		if m.steps[i].status == "pending" || m.steps[i].status == "active" {
			m.steps[i].status = "complete"
		}
	}

	// Set current step as active
	if m.steps[idx].status == "pending" {
		m.steps[idx].status = "active"
		m.activeStepIdx = idx
	}
}

// View renders the UI
func (m graphSearchModel) View() string {
	if m.done {
		// Don't show anything here - we'll print logs after TUI exits
		return ""
	}

	var output strings.Builder

	// Header
	output.WriteString(searchHeaderStyle.Render("🔍 Intelligent Graph Search"))
	output.WriteString("\n\n")

	// Query
	output.WriteString(searchDimStyle.Render("Query: "))
	output.WriteString(searchAnswerStyle.Render(m.query))
	output.WriteString("\n\n")

	// Progress bar
	if m.progress > 0 {
		barWidth := 40
		filled := int(float64(barWidth) * float64(m.progress) / 100.0)
		bar := strings.Repeat("█", filled) + strings.Repeat("░", barWidth-filled)
		output.WriteString(searchStageStyle.Render(fmt.Sprintf("[%s] %d%%", bar, m.progress)))
		output.WriteString("\n\n")
	}

	// Steps with status icons
	output.WriteString(searchHeaderStyle.Render("Progress:"))
	output.WriteString("\n\n")

	for i, step := range m.steps {
		var icon string
		var style lipgloss.Style

		switch step.status {
		case "complete":
			icon = "✓"
			style = searchSuccessStyle
		case "active":
			icon = m.spinner.View()
			style = searchStageStyle
		case "pending":
			icon = "○"
			style = searchDimStyle
		default:
			icon = "○"
			style = searchDimStyle
		}

		output.WriteString(fmt.Sprintf("  %s %s", icon, style.Render(step.name)))

		// Show sub-message for active step
		if step.status == "active" && m.currentReason != "" {
			output.WriteString(searchDimStyle.Render(fmt.Sprintf(" - %s", m.currentReason)))
		}
		output.WriteString("\n")

		// Show tool calls for this step (only recent ones for active, all for complete)
		toolsToShow := step.toolCalls
		if step.status == "active" && len(toolsToShow) > 5 {
			toolsToShow = toolsToShow[len(toolsToShow)-5:]
		}

		for _, tc := range toolsToShow {
			output.WriteString(searchDimStyle.Render(fmt.Sprintf("     %s\n", tc)))
		}

		// Add spacing between steps
		if i < len(m.steps)-1 {
			output.WriteString("\n")
		}
	}

	output.WriteString("\n")

	// Live reasoning/activity for active step
	if m.currentReason != "" && !m.streaming {
		output.WriteString("\n")
		output.WriteString(searchToolStyle.Render("💭 " + m.currentReason))
		output.WriteString("\n")
	}

	// Streaming answer preview
	if m.streaming && m.answer.Len() > 0 {
		output.WriteString("\n")
		output.WriteString(searchHeaderStyle.Render("📝 Answer:"))
		output.WriteString("\n\n")
		// Show first 300 chars of streaming answer
		answerPreview := m.answer.String()
		if len(answerPreview) > 300 {
			answerPreview = answerPreview[:300] + "..."
		}
		output.WriteString(searchAnswerStyle.Render(answerPreview))
		output.WriteString("\n")
	}

	// Help text
	output.WriteString("\n")
	output.WriteString(searchDimStyle.Render("Press Ctrl+C to cancel"))

	return output.String()
}

// renderFinalResult renders the final search result
func (m graphSearchModel) renderFinalResult() string {
	if m.result == nil {
		return successStyle.Render("✓ Search complete\n")
	}

	var output strings.Builder

	// Success header
	output.WriteString(searchSuccessStyle.Render("✓ Search Complete"))
	output.WriteString("\n\n")

	// Answer
	if m.result.answer != "" {
		output.WriteString(searchHeaderStyle.Render("Answer:"))
		output.WriteString("\n")
		output.WriteString(searchAnswerStyle.Render(m.result.answer))
		output.WriteString("\n\n")
	}

	// Tool calls summary
	if len(m.result.toolCalls) > 0 {
		output.WriteString(searchDimStyle.Render(fmt.Sprintf("Tools Used (%d):", len(m.result.toolCalls))))
		output.WriteString("\n")
		for i, tc := range m.result.toolCalls {
			output.WriteString(searchDimStyle.Render(fmt.Sprintf("  %d. ", i+1)))
			output.WriteString(searchToolStyle.Render(tc.ToolName))
			if tc.ResultSummary != "" {
				output.WriteString(searchDimStyle.Render(" - " + tc.ResultSummary))
			}
			output.WriteString("\n")
		}
		output.WriteString("\n")
	}

	// Metadata
	if m.result.confidence != "" {
		confidenceIcon := "✓"
		if m.result.confidence == "low" {
			confidenceIcon = "⚠"
		}
		output.WriteString(searchDimStyle.Render(fmt.Sprintf("  %s Confidence: ", confidenceIcon)))
		output.WriteString(searchAnswerStyle.Render(m.result.confidence))
		output.WriteString(searchDimStyle.Render(fmt.Sprintf(" | Turns: %d", m.result.turnsUsed)))
		output.WriteString("\n\n")
	}

	// Suggestions
	if len(m.result.suggestions) > 0 {
		output.WriteString(searchDimStyle.Render("Follow-up Suggestions:"))
		output.WriteString("\n")
		for _, sug := range m.result.suggestions {
			output.WriteString(searchDimStyle.Render("  • "))
			output.WriteString(searchAnswerStyle.Render(sug))
			output.WriteString("\n")
		}
		output.WriteString("\n")
	}

	// Session ID for follow-ups
	if m.result.sessionID != "" {
		output.WriteString(searchDimStyle.Render("💡 Continue: "))
		output.WriteString(searchToolStyle.Render(fmt.Sprintf("--session %s", m.result.sessionID)))
		output.WriteString("\n")
	}

	return output.String()
}

// waitForGraphSearchEvent listens for the next search event
func waitForGraphSearchEvent(eventChan <-chan contextgraph.StreamEvent) tea.Cmd {
	return func() tea.Msg {
		event, ok := <-eventChan
		if !ok {
			// Channel closed
			return graphSearchCompleteMsg{result: nil}
		}
		return graphSearchEventMsg{event: event}
	}
}

// createGraphSearchSpinner creates a configured spinner for graph search
func createGraphSearchSpinner() spinner.Model {
	s := spinner.New()
	s.Spinner = spinner.Dot
	s.Style = lipgloss.NewStyle().Foreground(lipgloss.Color("86"))
	return s
}

// runGraphSearchTUI runs the bubbletea TUI for graph search
func runGraphSearchTUI(ctx context.Context, query string, eventChan <-chan contextgraph.StreamEvent) (*graphSearchResult, error) {
	// Check if we're in a TTY environment
	isTTY := isatty.IsTerminal(os.Stdout.Fd()) || isatty.IsCygwinTerminal(os.Stdout.Fd())

	// If not TTY (CI/CD), use real-time logging instead
	if !isTTY {
		return runGraphSearchLogger(ctx, query, eventChan)
	}

	// Create initial model
	initialModel := graphSearchModel{
		spinner:       createGraphSearchSpinner(),
		query:         query,
		eventChan:     eventChan,
		ctx:           ctx,
		steps:         initializeSteps(),
		activeStepIdx: -1,
		liveToolCalls: make([]string, 0),
	}

	// Mark first step as active
	if len(initialModel.steps) > 0 {
		initialModel.steps[0].status = "active"
		initialModel.activeStepIdx = 0
	}

	// Run bubbletea program
	program := tea.NewProgram(initialModel)
	finalModel, err := program.Run()
	if err != nil {
		return nil, fmt.Errorf("UI error: %w", err)
	}

	// Extract result
	m := finalModel.(graphSearchModel)
	if m.err != nil {
		// Print error log
		fmt.Printf("\n❌ Error: %s\n\n", m.err.Error())
		return nil, m.err
	}

	// Print success log
	if m.result != nil {
		printGraphSearchCompletionLog(m.result)
	}

	return m.result, nil
}

// runGraphSearchLogger runs real-time logging for non-TTY environments (CI/CD)
func runGraphSearchLogger(ctx context.Context, query string, eventChan <-chan contextgraph.StreamEvent) (*graphSearchResult, error) {
	result := &graphSearchResult{}
	var answerBuilder strings.Builder
	steps := initializeSteps()
	activeStepIdx := 0
	maxProgress := 0

	// Print header
	fmt.Println()
	fmt.Println("🔍 Intelligent Graph Search")
	fmt.Println()
	fmt.Printf("Query: %s\n", query)
	fmt.Println()
	fmt.Println("Progress:")
	fmt.Println()

	// Print initial steps
	for _, step := range steps {
		fmt.Printf("  ○ %s\n", step.name)
	}
	fmt.Println()

	// Process events in real-time
	for {
		select {
		case event, ok := <-eventChan:
			if !ok {
				// Channel closed
				result.answer = answerBuilder.String()
				printGraphSearchCompletionLog(result)
				return result, nil
			}

			switch event.Event {
			case "session":
				if sid, ok := event.Data["session_id"].(string); ok {
					result.sessionID = sid
				}

			case "progress":
				if msg, ok := event.Data["message"].(string); ok {
					// Print progress message
					fmt.Printf("\r  💭 %s", msg)
				}
				if p, ok := event.Data["progress"].(float64); ok {
					newProgress := int(p)
					if newProgress > maxProgress {
						maxProgress = newProgress
					}
				}

			case "tool_call":
				if toolName, ok := event.Data["tool_name"].(string); ok {
					fmt.Printf("\n     🔧 %s...", toolName)
					if activeStepIdx >= 0 && activeStepIdx < len(steps) {
						steps[activeStepIdx].toolCalls = append(steps[activeStepIdx].toolCalls, toolName)
					}
				}

			case "tool_result":
				if toolName, ok := event.Data["tool_name"].(string); ok {
					if summary, ok := event.Data["result_summary"].(string); ok {
						success, _ := event.Data["success"].(bool)
						icon := "✓"
						if !success {
							icon = "✗"
						}
						fmt.Printf("\r     %s %s: %s\n", icon, toolName, summary)
					}
				}

			case "step_complete":
				// Mark current step complete and move to next
				if activeStepIdx >= 0 && activeStepIdx < len(steps) {
					steps[activeStepIdx].status = "complete"
					fmt.Printf("\r  ✓ %s\n", steps[activeStepIdx].name)
				}
				if activeStepIdx+1 < len(steps) {
					activeStepIdx++
					steps[activeStepIdx].status = "active"
				}

			case "partial_answer":
				if text, ok := event.Data["text"].(string); ok {
					answerBuilder.WriteString(text)
				}

			case "complete":
				// Parse final result
				if ans, ok := event.Data["answer"].(string); ok {
					if answerBuilder.Len() == 0 {
						result.answer = ans
					} else {
						result.answer = answerBuilder.String()
					}
				}
				if sid, ok := event.Data["session_id"].(string); ok {
					result.sessionID = sid
				}
				if t, ok := event.Data["turns_used"].(float64); ok {
					result.turnsUsed = int(t)
				}
				if c, ok := event.Data["confidence"].(string); ok {
					result.confidence = c
				}

				// Parse tool calls
				if tc, ok := event.Data["tool_calls"].([]interface{}); ok {
					for _, t := range tc {
						if tcMap, ok := t.(map[string]interface{}); ok {
							toolCall := controlplane.ToolCall{}
							if name, ok := tcMap["tool_name"].(string); ok {
								toolCall.ToolName = name
							}
							if summary, ok := tcMap["result_summary"].(string); ok {
								toolCall.ResultSummary = summary
							}
							result.toolCalls = append(result.toolCalls, toolCall)
						}
					}
				}

				// Parse suggestions
				if s, ok := event.Data["suggestions"].([]interface{}); ok {
					for _, sug := range s {
						if str, ok := sug.(string); ok {
							result.suggestions = append(result.suggestions, str)
						}
					}
				}

				printGraphSearchCompletionLog(result)
				return result, nil

			case "error":
				errMsg := "unknown error"
				if msg, ok := event.Data["message"].(string); ok {
					errMsg = msg
				}
				fmt.Printf("\n❌ Error: %s\n\n", errMsg)
				return nil, fmt.Errorf("%s", errMsg)
			}

		case <-ctx.Done():
			fmt.Println("\n⚠️  Cancelled by user")
			return nil, ctx.Err()
		}
	}
}

// printGraphSearchCompletionLog prints a clean, permanent log after TUI exits
func printGraphSearchCompletionLog(result *graphSearchResult) {
	fmt.Println()
	fmt.Println("━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━")
	fmt.Println()
	fmt.Println(searchSuccessStyle.Render("✓ Search Complete"))
	fmt.Println()
	fmt.Println("━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━")
	fmt.Println()

	if result.answer != "" {
		fmt.Println(searchHeaderStyle.Render("📝 Answer:"))
		fmt.Println()
		// Format answer with proper line spacing
		lines := strings.Split(result.answer, "\n")
		for _, line := range lines {
			if strings.TrimSpace(line) == "" {
				fmt.Println()
			} else {
				fmt.Println(searchAnswerStyle.Render(line))
			}
		}
		fmt.Println()
		fmt.Println("━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━")
		fmt.Println()
	}

	if len(result.toolCalls) > 0 {
		fmt.Println(searchDimStyle.Render(fmt.Sprintf("🔧 Tools Used (%d):", len(result.toolCalls))))
		fmt.Println()
		for i, tc := range result.toolCalls {
			fmt.Printf("  %s %s", searchDimStyle.Render(fmt.Sprintf("%d.", i+1)), searchToolStyle.Render(tc.ToolName))
			if tc.ResultSummary != "" {
				fmt.Printf(" %s", searchDimStyle.Render("→ "+tc.ResultSummary))
			}
			fmt.Println()
		}
		fmt.Println()
	}

	if result.confidence != "" {
		confidenceIcon := "✓"
		confidenceStyle := searchSuccessStyle
		if result.confidence == "low" {
			confidenceIcon = "⚠"
			confidenceStyle = searchToolStyle
		}
		fmt.Printf("%s %s  •  ",
			confidenceStyle.Render(confidenceIcon),
			confidenceStyle.Render(fmt.Sprintf("Confidence: %s", result.confidence)))
		fmt.Printf("%s\n", searchDimStyle.Render(fmt.Sprintf("Turns: %d", result.turnsUsed)))
		fmt.Println()
	}

	if len(result.suggestions) > 0 {
		fmt.Println(searchDimStyle.Render("💡 Follow-up Suggestions:"))
		fmt.Println()
		for _, sug := range result.suggestions {
			fmt.Printf("  %s %s\n", searchDimStyle.Render("•"), searchAnswerStyle.Render(sug))
		}
		fmt.Println()
	}

	if result.sessionID != "" {
		fmt.Printf("%s %s\n\n",
			searchToolStyle.Render("🔗 Continue this conversation:"),
			searchStageStyle.Render(fmt.Sprintf("--session %s", result.sessionID)))
	}
}
