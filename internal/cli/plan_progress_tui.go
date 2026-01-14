package cli

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"strings"

	"github.com/charmbracelet/bubbles/progress"
	"github.com/charmbracelet/bubbles/spinner"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
	"github.com/mattn/go-isatty"
	"github.com/kubiyabot/cli/internal/kubiya"
)

// Styles using lipgloss
var (
	stageStyle = lipgloss.NewStyle().
			Foreground(lipgloss.Color("69")).
			Bold(true)

	messageStyle = lipgloss.NewStyle().
			Foreground(lipgloss.Color("252"))

	dimStyle = lipgloss.NewStyle().
			Foreground(lipgloss.Color("241"))

	successStyle = lipgloss.NewStyle().
			Foreground(lipgloss.Color("42")).
			Bold(true)

	errorStyle = lipgloss.NewStyle().
			Foreground(lipgloss.Color("196")).
			Bold(true)

	// Tool call styles for elegant display
	toolIconStyle = lipgloss.NewStyle().
			Foreground(lipgloss.Color("214")). // Orange/amber color
			Bold(true)

	toolNameStyle = lipgloss.NewStyle().
			Foreground(lipgloss.Color("75")). // Cyan color
			Bold(false)

	toolSuccessStyle = lipgloss.NewStyle().
			Foreground(lipgloss.Color("42")). // Green
			Bold(false)

	toolDurationStyle = lipgloss.NewStyle().
			Foreground(lipgloss.Color("245")). // Gray
			Italic(true)
)

// Custom message types for bubbletea
type sseEventMsg struct {
	event kubiya.PlanStreamEvent
}

type sseErrorMsg struct {
	err error
}

// toolCallInfo tracks a tool call with its status
type toolCallInfo struct {
	name     string
	id       string
	status   string  // "running", "success", "failed"
	duration float64 // Duration in seconds (0 if still running)
}

// planProgressModel is the bubbletea model for plan generation progress
type planProgressModel struct {
	// UI Components
	spinner  spinner.Model
	progress progress.Model

	// Resource discovery state
	discoveringResources bool
	resourcesFound       string

	// State from SSE events
	stage           string
	message         string
	progressPercent float64
	stepName        string
	stepDescription string
	toolCalls       []toolCallInfo // Last 5 tool calls with status
	reasoningLines  []string       // Last 5 reasoning sentences from planner

	// Control
	done bool
	err  error
	plan *kubiya.PlanResponse

	// Event channels
	eventChan <-chan kubiya.PlanStreamEvent
	errChan   <-chan error
	ctx       context.Context
}

// Init initializes the model and starts the spinner animation
func (m planProgressModel) Init() tea.Cmd {
	return tea.Batch(
		m.spinner.Tick,
		waitForSSEEvent(m.eventChan, m.errChan),
	)
}

// Update handles all message types and updates the model
func (m planProgressModel) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
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

	case progress.FrameMsg:
		// Update progress bar animation
		progressModel, cmd := m.progress.Update(msg)
		m.progress = progressModel.(progress.Model)
		return m, cmd

	case sseEventMsg:
		// Process the SSE event
		return m.handleSSEEvent(msg.event)

	case sseErrorMsg:
		// Handle error from SSE stream
		m.done = true
		m.err = msg.err
		return m, tea.Quit

	case tea.WindowSizeMsg:
		// Handle terminal resize
		m.progress.Width = msg.Width - 4
		if m.progress.Width > 80 {
			m.progress.Width = 80
		}
		return m, nil
	}

	return m, nil
}

// handleSSEEvent processes a single SSE event and updates the model
func (m planProgressModel) handleSSEEvent(event kubiya.PlanStreamEvent) (tea.Model, tea.Cmd) {
	switch event.Type {
	case "progress":
		// Update progress percentage and message
		if p, ok := event.Data["progress"].(float64); ok {
			m.progressPercent = p
		}
		if msg, ok := event.Data["message"].(string); ok {
			m.message = msg
		}
		if stage, ok := event.Data["stage"].(string); ok {
			m.stage = stage
		}

	case "thinking":
		// Extract and display reasoning from planner
		if content, ok := event.Data["content"].(string); ok && content != "" {
			// Split content into sentences (rough approximation)
			// Split on ., !, ? followed by space or newline
			sentences := splitIntoSentences(content)

			// Add new sentences to reasoning lines
			m.reasoningLines = append(m.reasoningLines, sentences...)

			// Keep only last 5 sentences
			if len(m.reasoningLines) > 5 {
				m.reasoningLines = m.reasoningLines[len(m.reasoningLines)-5:]
			}
		}

	case "tool_call":
		// Record tool call with running status
		toolName, _ := event.Data["tool_name"].(string)
		toolID, _ := event.Data["tool_id"].(string)
		if toolName != "" {
			// Add new tool call
			m.toolCalls = append(m.toolCalls, toolCallInfo{
				name:   toolName,
				id:     toolID,
				status: "running",
			})
			// Keep only last 5 tool calls
			if len(m.toolCalls) > 5 {
				m.toolCalls = m.toolCalls[len(m.toolCalls)-5:]
			}
		}

	case "tool_result":
		// Update tool call status based on result
		toolID, _ := event.Data["tool_id"].(string)
		status, _ := event.Data["status"].(string)
		duration, _ := event.Data["duration"].(float64)

		// Find and update the matching tool call
		for i := range m.toolCalls {
			if m.toolCalls[i].id == toolID {
				if status == "success" || status == "" {
					m.toolCalls[i].status = "success"
				} else {
					m.toolCalls[i].status = "failed"
				}
				m.toolCalls[i].duration = duration
				break
			}
		}

	case "step_started":
		// Update step information
		if name, ok := event.Data["step_name"].(string); ok {
			m.stepName = name
			m.stage = name
		}
		if desc, ok := event.Data["step_description"].(string); ok {
			m.stepDescription = desc
			m.message = desc
		}

	case "step_completed":
		// Silently note step completion

	case "resources_summary":
		// Silently skip resources summary

	case "complete":
		// Extract final plan and exit
		if planData, ok := event.Data["plan"].(map[string]interface{}); ok {
			// Convert to PlanResponse
			planBytes, err := json.Marshal(planData)
			if err != nil {
				m.done = true
				m.err = fmt.Errorf("failed to marshal plan data: %w", err)
				return m, tea.Quit
			}

			var plan kubiya.PlanResponse
			err = json.Unmarshal(planBytes, &plan)
			if err != nil {
				m.done = true
				m.err = fmt.Errorf("failed to unmarshal plan: %w", err)
				return m, tea.Quit
			}

			m.plan = &plan
			m.done = true
			m.progressPercent = 100
			return m, tea.Quit
		} else {
			// Plan data was not in expected format or is missing
			m.done = true
			if event.Data["plan"] == nil {
				m.err = fmt.Errorf("complete event received but plan data is missing")
			} else {
				m.err = fmt.Errorf("complete event received but plan data is in unexpected format: %T", event.Data["plan"])
			}
			return m, tea.Quit
		}

	case "error":
		// Handle error event
		errMsg := "unknown error"
		if e, ok := event.Data["error"].(string); ok {
			errMsg = e
		} else if e, ok := event.Data["message"].(string); ok {
			errMsg = e
		}
		m.done = true
		m.err = fmt.Errorf("planning error: %s", errMsg)
		return m, tea.Quit
	}

	// Continue listening for next event
	return m, waitForSSEEvent(m.eventChan, m.errChan)
}

// View renders the UI
func (m planProgressModel) View() string {
	if m.done {
		// Don't show anything here - we'll print logs after TUI exits
		return ""
	}

	var output strings.Builder

	// Header - clean without boxes
	header := lipgloss.NewStyle().
		Bold(true).
		Foreground(lipgloss.Color("#A78BFA")).
		Render("🤖 Intelligent Task Planning")

	output.WriteString(header)
	output.WriteString("\n\n")

	// Resource discovery phase
	if m.discoveringResources {
		output.WriteString(m.spinner.View())
		output.WriteString(" ")
		output.WriteString(messageStyle.Render("Discovering available resources..."))
		output.WriteString("\n")
		if m.resourcesFound != "" {
			output.WriteString(dimStyle.Render("  " + m.resourcesFound))
			output.WriteString("\n")
		}
		output.WriteString("\n")
		output.WriteString(dimStyle.Render("  Press Ctrl+C to cancel"))
		return output.String()
	}

	// Planning phase - Progress bar
	output.WriteString(m.progress.ViewAs(m.progressPercent / 100.0))
	output.WriteString("\n\n")

	// Spinner + current stage
	output.WriteString(m.spinner.View())
	output.WriteString(" ")
	if m.stage != "" {
		output.WriteString(stageStyle.Render(m.stage))
	} else {
		output.WriteString(stageStyle.Render("Initializing..."))
	}

	// Progress percentage
	output.WriteString(fmt.Sprintf(" (%d%%)", int(m.progressPercent)))
	output.WriteString("\n")

	// Current message - handle long text better
	if m.message != "" && m.message != m.stage {
		// Truncate message if too long
		displayMessage := m.message
		maxLength := 80
		if len(displayMessage) > maxLength {
			displayMessage = displayMessage[:maxLength-3] + "..."
		}
		output.WriteString(messageStyle.Render(fmt.Sprintf("  %s", displayMessage)))
		output.WriteString("\n")
	}

	// Planner reasoning (last 5 sentences)
	if len(m.reasoningLines) > 0 {
		output.WriteString("\n")
		for _, line := range m.reasoningLines {
			if line != "" {
				// Truncate long reasoning lines
				displayLine := line
				maxLength := 100
				if len(displayLine) > maxLength {
					displayLine = displayLine[:maxLength-3] + "..."
				}
				// Format with dimmed style for thinking process
				output.WriteString(dimStyle.Render("  💭 " + displayLine))
				output.WriteString("\n")
			}
		}
	}

	// Show planner actions with elegant checkmark style
	if len(m.toolCalls) > 0 {
		output.WriteString("\n")

		// Deduplicate actions (same action description shown once)
		seenActions := make(map[string]bool)
		var uniqueActions []toolCallInfo

		for _, tc := range m.toolCalls {
			action := getActionDescription(tc.name)
			if !seenActions[action] {
				seenActions[action] = true
				uniqueActions = append(uniqueActions, tc)
			} else {
				// Update status of existing action if this one is more recent
				for i := range uniqueActions {
					if getActionDescription(uniqueActions[i].name) == action {
						if tc.status == "running" || (tc.status == "success" && uniqueActions[i].status != "running") {
							uniqueActions[i] = tc
						}
					}
				}
			}
		}

		for _, tc := range uniqueActions {
			action := getActionDescription(tc.name)

			var line string
			switch tc.status {
			case "running":
				// Show spinner-like indicator for running actions
				line = fmt.Sprintf("  %s %s",
					toolIconStyle.Render("›"),
					messageStyle.Render(action+"..."))
			case "success":
				// Show checkmark for completed actions
				line = fmt.Sprintf("  %s %s",
					toolSuccessStyle.Render("✓"),
					dimStyle.Render(action))
			case "failed":
				// Show X for failed actions
				line = fmt.Sprintf("  %s %s",
					errorStyle.Render("✗"),
					dimStyle.Render(action))
			default:
				line = fmt.Sprintf("  %s %s",
					dimStyle.Render("○"),
					dimStyle.Render(action))
			}
			output.WriteString(line)
			output.WriteString("\n")
		}
	}

	// Help text
	output.WriteString("\n")
	output.WriteString(dimStyle.Render("  Press Ctrl+C to cancel"))

	return output.String()
}

// waitForSSEEvent listens for the next SSE event and converts it to a tea.Msg
func waitForSSEEvent(eventChan <-chan kubiya.PlanStreamEvent, errChan <-chan error) tea.Cmd {
	return func() tea.Msg {
		select {
		case event, ok := <-eventChan:
			if !ok {
				// Channel closed, treat as completion
				return sseErrorMsg{err: fmt.Errorf("event stream closed unexpectedly without sending complete event")}
			}
			return sseEventMsg{event: event}
		case err, ok := <-errChan:
			if !ok {
				// Error channel closed without sending error - stream ended unexpectedly
				return sseErrorMsg{err: fmt.Errorf("error channel closed unexpectedly")}
			}
			if err == nil {
				// Nil error received - treat as unexpected stream end
				return sseErrorMsg{err: fmt.Errorf("stream ended without plan data")}
			}
			return sseErrorMsg{err: err}
		}
	}
}

// createSpinner creates a configured spinner with better animation
func createSpinner() spinner.Model {
	s := spinner.New()
	// Use a more animated spinner (similar to minikube)
	s.Spinner = spinner.Points
	s.Style = lipgloss.NewStyle().
		Foreground(lipgloss.Color("#60A5FA")).
		Bold(true)
	return s
}

// createProgressBar creates a configured progress bar with custom colors
func createProgressBar() progress.Model {
	p := progress.New(
		progress.WithGradient("#60A5FA", "#A78BFA"), // Blue to purple gradient
		progress.WithWidth(80),
		progress.WithoutPercentage(),
	)
	return p
}

// resourceDiscoveryMsg is sent when resources are discovered
type resourceDiscoveryMsg struct {
	agentCount int
	teamCount  int
	envCount   int
}

// planningStartMsg is sent when planning phase begins
type planningStartMsg struct{}

// runPlanProgressTUI runs the bubbletea TUI for plan generation progress
func runPlanProgressTUI(ctx context.Context, eventChan <-chan kubiya.PlanStreamEvent, errChan <-chan error, resources *PlanningResources) (*kubiya.PlanResponse, error) {
	// Check if we're in a TTY environment
	isTTY := isatty.IsTerminal(os.Stdout.Fd()) || isatty.IsCygwinTerminal(os.Stdout.Fd())

	// If not TTY (CI/CD), use real-time logging instead
	if !isTTY {
		return runPlanProgressLogger(ctx, eventChan, errChan, resources)
	}

	// Create initial model
	initialModel := planProgressModel{
		spinner:              createSpinner(),
		progress:             createProgressBar(),
		eventChan:            eventChan,
		errChan:              errChan,
		ctx:                  ctx,
		toolCalls:            make([]toolCallInfo, 0, 5),
		progressPercent:      0,
		discoveringResources: resources == nil,
	}

	// If resources are already fetched, show them
	if resources != nil {
		initialModel.resourcesFound = fmt.Sprintf("✓ Found %d agents, %d teams, %d environments",
			len(resources.Agents), len(resources.Teams), len(resources.Environments))
		initialModel.discoveringResources = false
	}

	// Run bubbletea program
	program := tea.NewProgram(initialModel)
	finalModel, err := program.Run()
	if err != nil {
		return nil, fmt.Errorf("UI error: %w", err)
	}

	// Extract result
	m := finalModel.(planProgressModel)
	if m.err != nil {
		// Print error log
		fmt.Printf("\n❌ Error: %v\n\n", m.err)
		return nil, m.err
	}
	if m.plan == nil {
		fmt.Printf("\n❌ Error: plan generation completed but no plan was received\n\n")
		return nil, fmt.Errorf("plan generation completed but no plan was received from the planner service")
	}

	// Print success log
	printPlanCompletionLog(m.plan)

	return m.plan, nil
}

// runPlanProgressLogger runs real-time logging for non-TTY environments (CI/CD)
func runPlanProgressLogger(ctx context.Context, eventChan <-chan kubiya.PlanStreamEvent, errChan <-chan error, resources *PlanningResources) (*kubiya.PlanResponse, error) {
	var plan *kubiya.PlanResponse
	var planObj kubiya.PlanResponse

	// Print header - cleaner for CI/CD
	fmt.Println()
	fmt.Println("🤖 Intelligent Task Planning")
	fmt.Println()

	// Show resources
	if resources != nil {
		fmt.Printf("✓ Discovered %d agents, %d teams, %d environments\n",
			len(resources.Agents), len(resources.Teams), len(resources.Environments))
		fmt.Println()
	}

	fmt.Println("⚙️  Generating execution plan...")
	fmt.Println()

	// Process events in real-time
	for {
		select {
		case event, ok := <-eventChan:
			if !ok {
				if plan == nil {
					fmt.Println()
					fmt.Println("❌ Error: plan generation completed but no plan was received")
					fmt.Println()
					return nil, fmt.Errorf("plan generation completed but no plan was received from the planner service")
				}
				printPlanCompletionLog(plan)
				return plan, nil
			}

			switch event.Type {
			case "progress":
				if stage, ok := event.Data["stage"].(string); ok {
					fmt.Printf("  [%s] ", stage)
				}
				if message, ok := event.Data["message"].(string); ok {
					fmt.Println(message)
				}
				if progress, ok := event.Data["progress"].(float64); ok {
					fmt.Printf("  Progress: %d%%\n", int(progress))
				}

			case "thinking":
				// Display planner reasoning with [PLANNER]: prefix
				if content, ok := event.Data["content"].(string); ok && content != "" {
					sentences := splitIntoSentences(content)
					for _, sentence := range sentences {
						if sentence != "" {
							fmt.Printf("  [PLANNER]: %s\n", sentence)
						}
					}
				}

			case "tool_call":
				// Show user-friendly action description instead of tool name
				if toolName, ok := event.Data["tool_name"].(string); ok {
					action := getActionDescription(toolName)
					fmt.Printf("  › %s...\n", action)
				}

			case "tool_result":
				// Show completion with checkmark
				if toolName, ok := event.Data["tool_name"].(string); ok {
					action := getActionDescription(toolName)
					status, _ := event.Data["status"].(string)
					if status == "success" || status == "" {
						fmt.Printf("  ✓ %s\n", action)
					} else {
						fmt.Printf("  ✗ %s (failed)\n", action)
					}
				}

			case "step_started":
				if stepName, ok := event.Data["step_name"].(string); ok {
					fmt.Printf("\n  ⚙️  %s\n", stepName)
					if desc, ok := event.Data["step_description"].(string); ok {
						fmt.Printf("     %s\n", desc)
					}
				}

			case "step_completed":
				if stepName, ok := event.Data["step_name"].(string); ok {
					fmt.Printf("  ✓ %s complete\n", stepName)
				}

			case "complete":
				if planData, ok := event.Data["plan"].(map[string]interface{}); ok {
					planBytes, err := json.Marshal(planData)
					if err != nil {
						fmt.Printf("\n❌ Error: failed to marshal plan data: %v\n\n", err)
						return nil, fmt.Errorf("failed to marshal plan data: %w", err)
					}

					err = json.Unmarshal(planBytes, &planObj)
					if err != nil {
						fmt.Printf("\n❌ Error: failed to unmarshal plan: %v\n\n", err)
						return nil, fmt.Errorf("failed to unmarshal plan: %w", err)
					}

					plan = &planObj
					printPlanCompletionLog(plan)
					return plan, nil
				} else {
					// Plan data was not in expected format or is missing
					if event.Data["plan"] == nil {
						fmt.Printf("\n❌ Error: complete event received but plan data is missing\n\n")
						return nil, fmt.Errorf("complete event received but plan data is missing")
					}
					fmt.Printf("\n❌ Error: complete event received but plan data is in unexpected format: %T\n\n", event.Data["plan"])
					return nil, fmt.Errorf("complete event received but plan data is in unexpected format: %T", event.Data["plan"])
				}

			case "error":
				errMsg := "unknown error"
				if msg, ok := event.Data["error"].(string); ok {
					errMsg = msg
				} else if msg, ok := event.Data["message"].(string); ok {
					errMsg = msg
				}
				fmt.Printf("\n❌ Error: %s\n\n", errMsg)
				return nil, fmt.Errorf("planning error: %s", errMsg)
			}

		case err, ok := <-errChan:
			if !ok {
				// Error channel closed without plan - stream ended unexpectedly
				if plan == nil {
					fmt.Printf("\n❌ Error: stream ended unexpectedly without plan data\n\n")
					return nil, fmt.Errorf("stream ended unexpectedly without plan data")
				}
				return plan, nil
			}
			if err == nil {
				// Nil error - stream completed but plan might not have been received yet
				if plan == nil {
					fmt.Printf("\n❌ Error: stream completed without plan data\n\n")
					return nil, fmt.Errorf("stream completed without plan data")
				}
				return plan, nil
			}
			fmt.Printf("\n❌ Error: %v\n\n", err)
			return nil, err

		case <-ctx.Done():
			fmt.Println("\n⚠️  Cancelled by user")
			return nil, ctx.Err()
		}
	}
}

// printPlanCompletionLog prints a clean, permanent log after TUI exits
func printPlanCompletionLog(plan *kubiya.PlanResponse) {
	fmt.Println()
	fmt.Println("✓ Plan generated successfully")
	fmt.Println()

	if plan.RecommendedExecution.EntityType != "" {
		entityTypeIcon := "🤖"
		if plan.RecommendedExecution.EntityType == "team" {
			entityTypeIcon = "👥"
		}
		fmt.Printf("  %s Using %s: %s\n",
			entityTypeIcon,
			plan.RecommendedExecution.EntityType,
			plan.RecommendedExecution.EntityName)
	}

	if plan.Summary != "" {
		// Truncate summary if too long
		summary := plan.Summary
		maxLen := 120
		if len(summary) > maxLen {
			summary = summary[:maxLen-3] + "..."
		}
		fmt.Printf("  📋 Summary: %s\n", summary)
	}

	if plan.CostEstimate.EstimatedCostUSD > 0 {
		fmt.Printf("  💰 Estimated cost: $%.2f\n", plan.CostEstimate.EstimatedCostUSD)
	}

	fmt.Println()
}

// splitIntoSentences splits text into sentences, handling common punctuation
func splitIntoSentences(text string) []string {
	if text == "" {
		return nil
	}

	var sentences []string
	var currentSentence strings.Builder

	runes := []rune(text)
	for i := 0; i < len(runes); i++ {
		r := runes[i]
		currentSentence.WriteRune(r)

		// Check if we hit a sentence-ending punctuation
		if r == '.' || r == '!' || r == '?' {
			// Look ahead to see if followed by space, newline, or end of string
			if i+1 >= len(runes) || runes[i+1] == ' ' || runes[i+1] == '\n' || runes[i+1] == '\r' {
				sentence := strings.TrimSpace(currentSentence.String())
				if sentence != "" && len(sentence) > 3 { // Filter out very short fragments
					sentences = append(sentences, sentence)
				}
				currentSentence.Reset()
			}
		}
	}

	// Add any remaining text as a sentence
	if currentSentence.Len() > 0 {
		sentence := strings.TrimSpace(currentSentence.String())
		if sentence != "" && len(sentence) > 3 {
			sentences = append(sentences, sentence)
		}
	}

	return sentences
}

// toolToAction maps tool names to user-friendly action descriptions
var toolToAction = map[string]string{
	// Agent/Team discovery
	"list_agents":        "Discovering available agents",
	"search_agents":      "Searching for matching agents",
	"get_agent":          "Analyzing agent capabilities",
	"list_teams":         "Discovering available teams",
	"search_teams":       "Searching for matching teams",
	"get_team":           "Analyzing team composition",

	// Environment/Queue discovery
	"list_environments":  "Checking available environments",
	"get_environment":    "Analyzing environment details",
	"list_worker_queues": "Finding available worker queues",
	"get_worker_queue":   "Checking queue availability",

	// Knowledge/Context
	"search_knowledge":   "Searching knowledge base",
	"get_context":        "Gathering execution context",

	// Fallback patterns (partial matches)
	"agent":              "Analyzing agents",
	"team":               "Analyzing teams",
	"environment":        "Checking environments",
	"queue":              "Checking queues",
	"search":             "Searching resources",
	"list":               "Discovering resources",
}

// getActionDescription converts a tool name to a user-friendly action
func getActionDescription(toolName string) string {
	// Try exact match first
	if action, ok := toolToAction[toolName]; ok {
		return action
	}

	// Try partial matches
	lowerName := strings.ToLower(toolName)
	for pattern, action := range toolToAction {
		if strings.Contains(lowerName, pattern) {
			return action
		}
	}

	// Default fallback
	return "Processing"
}
