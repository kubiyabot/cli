package cli

import (
	"fmt"
	"strings"
	"time"

	"github.com/kubiyabot/cli/internal/kubiya"
	"github.com/kubiyabot/cli/internal/style"
)

// InteractiveWorkflowRenderer provides interactive workflow visualization
type InteractiveWorkflowRenderer struct {
	totalSteps    int
	currentStep   int
	stepNames     []string
	stepStatuses  map[string]string
	stepOutputs   map[string]string
	stepDurations map[string]time.Duration
	stepStartTimes map[string]time.Time
	workflowName  string
	executionID   string
	startTime     time.Time
	isReconnecting bool
}

// NewInteractiveWorkflowRenderer creates a new interactive renderer
func NewInteractiveWorkflowRenderer(workflowName string, steps []interface{}) *InteractiveWorkflowRenderer {
	stepNames := make([]string, 0, len(steps))
	stepStatuses := make(map[string]string)
	
	// Extract step names and initialize statuses
	for _, step := range steps {
		if stepMap, ok := step.(map[string]interface{}); ok {
			if name, ok := stepMap["name"].(string); ok {
				stepNames = append(stepNames, name)
				stepStatuses[name] = "pending"
			}
		}
	}

	return &InteractiveWorkflowRenderer{
		totalSteps:     len(stepNames),
		stepNames:      stepNames,
		stepStatuses:   stepStatuses,
		stepOutputs:    make(map[string]string),
		stepDurations:  make(map[string]time.Duration),
		stepStartTimes: make(map[string]time.Time),
		workflowName:   workflowName,
		startTime:      time.Now(),
	}
}

// ProcessEvent processes workflow events and updates the display
func (r *InteractiveWorkflowRenderer) ProcessEvent(event kubiya.RobustWorkflowEvent) {
	switch event.Type {
	case "state":
		if event.ExecutionID != "" {
			r.executionID = event.ExecutionID
		}
		if r.startTime.IsZero() {
			r.startTime = time.Now()
		}
		r.render()

	case "step":
		if event.StepStatus == "running" {
			r.stepStatuses[event.StepName] = "running"
			r.stepStartTimes[event.StepName] = time.Now()
			// Update current step index
			for i, name := range r.stepNames {
				if name == event.StepName {
					r.currentStep = i + 1
					break
				}
			}
		} else if event.StepStatus == "completed" || event.StepStatus == "finished" {
			r.stepStatuses[event.StepName] = "completed"
			if event.Data != "" {
				r.stepOutputs[event.StepName] = event.Data
			}
			// Calculate duration
			if startTime, ok := r.stepStartTimes[event.StepName]; ok {
				r.stepDurations[event.StepName] = time.Since(startTime)
				delete(r.stepStartTimes, event.StepName)
			}
		} else if event.StepStatus == "failed" {
			r.stepStatuses[event.StepName] = "failed"
			if event.Data != "" {
				r.stepOutputs[event.StepName] = event.Data
			}
			// Calculate duration
			if startTime, ok := r.stepStartTimes[event.StepName]; ok {
				r.stepDurations[event.StepName] = time.Since(startTime)
				delete(r.stepStartTimes, event.StepName)
			}
		}
		r.render()

	case "reconnect":
		if event.Reconnect {
			r.isReconnecting = true
		} else {
			r.isReconnecting = false
		}
		r.render()

	case "complete":
		r.render()
		r.renderSummary()

	case "error":
		r.render()
		fmt.Printf("\n%s %s\n", 
			style.ErrorStyle.Render("❌ Error:"), 
			event.Error)
	}
}

// render displays the current workflow state
func (r *InteractiveWorkflowRenderer) render() {
	// Clear screen and move cursor to top
	fmt.Print("\033[2J\033[H")
	
	// Header
	fmt.Printf("%s %s\n", 
		style.StatusStyle.Render("🚀 Workflow Execution"), 
		style.HighlightStyle.Render(r.workflowName))
	
	if r.executionID != "" {
		fmt.Printf("%s %s\n", 
			style.DimStyle.Render("ID:"), 
			r.executionID)
	}
	
	duration := time.Since(r.startTime)
	fmt.Printf("%s %s\n", 
		style.DimStyle.Render("Duration:"), 
		duration.Round(time.Second))
	
	if r.isReconnecting {
		fmt.Printf("%s %s\n", 
			style.WarningStyle.Render("🔄 Status:"), 
			"Reconnecting...")
	}
	
	fmt.Println()
	
	// Progress bar
	r.renderProgressBar()
	fmt.Println()
	
	// Step visualization
	r.renderStepGraph()
	
	// Current step details
	r.renderCurrentStepDetails()
}

// renderProgressBar renders the overall progress
func (r *InteractiveWorkflowRenderer) renderProgressBar() {
	completedSteps := 0
	for _, status := range r.stepStatuses {
		if status == "completed" {
			completedSteps++
		}
	}
	
	progress := float64(completedSteps) / float64(r.totalSteps)
	barWidth := 50
	filledWidth := int(progress * float64(barWidth))
	
	fmt.Printf("%s [", style.DimStyle.Render("Progress:"))
	
	for i := 0; i < barWidth; i++ {
		if i < filledWidth {
			fmt.Print(style.SuccessStyle.Render("█"))
		} else if i == filledWidth && r.currentStep > completedSteps {
			// Show current running step
			fmt.Print(style.WarningStyle.Render("▓"))
		} else {
			fmt.Print(style.DimStyle.Render("░"))
		}
	}
	
	fmt.Printf("] %s (%d/%d steps)\n", 
		style.HighlightStyle.Render(fmt.Sprintf("%.1f%%", progress*100)),
		completedSteps, 
		r.totalSteps)
}

// renderStepGraph renders the workflow steps as a visual graph
func (r *InteractiveWorkflowRenderer) renderStepGraph() {
	fmt.Printf("%s\n", style.SectionStyle.Render("Workflow Steps"))
	
	for i, stepName := range r.stepNames {
		status := r.stepStatuses[stepName]
		isLast := i == len(r.stepNames)-1
		
		// Step connector
		if i > 0 {
			fmt.Print("    │\n")
			fmt.Print("    ↓\n")
		}
		
		// Step box
		r.renderStepBox(stepName, status, i+1, isLast)
	}
}

// renderStepBox renders an individual step in the graph
func (r *InteractiveWorkflowRenderer) renderStepBox(stepName, status string, stepIndex int, isLast bool) {
	var icon, statusText string
	var styleFunc func(...string) string
	
	switch status {
	case "pending":
		icon = "⭕"
		statusText = "Pending"
		styleFunc = style.DimStyle.Render
	case "running":
		icon = r.getSpinner()
		statusText = "Running"
		styleFunc = style.WarningStyle.Render
	case "completed":
		icon = "✅"
		statusText = "Completed"
		styleFunc = style.SuccessStyle.Render
	case "failed":
		icon = "❌"
		statusText = "Failed"
		styleFunc = style.ErrorStyle.Render
	case "skipped":
		icon = "⏭️"
		statusText = "Skipped"
		styleFunc = style.DimStyle.Render
	default:
		icon = "❓"
		statusText = "Unknown"
		styleFunc = style.DimStyle.Render
	}
	
	// Create box with step info
	stepText := fmt.Sprintf("%s %s", icon, stepName)
	statusWithDuration := statusText
	
	if duration, ok := r.stepDurations[stepName]; ok {
		statusWithDuration = fmt.Sprintf("%s (%v)", statusText, duration.Round(time.Millisecond))
	} else if status == "running" {
		if startTime, ok := r.stepStartTimes[stepName]; ok {
			elapsed := time.Since(startTime)
			statusWithDuration = fmt.Sprintf("%s (%v)", statusText, elapsed.Round(time.Second))
		}
	}
	
	fmt.Printf("┌─ %s %s\n", 
		styleFunc(fmt.Sprintf("[%d]", stepIndex)), 
		style.ToolNameStyle.Render(stepText))
	fmt.Printf("│  %s %s\n", 
		style.DimStyle.Render("Status:"), 
		statusWithDuration)
	
	// Show output preview if available
	if output, ok := r.stepOutputs[stepName]; ok && output != "" {
		preview := output
		if len(preview) > 60 {
			preview = preview[:60] + "..."
		}
		// Replace newlines with spaces for single-line preview
		preview = strings.ReplaceAll(preview, "\n", " ")
		fmt.Printf("│  %s %s\n", 
			style.DimStyle.Render("Output:"), 
			style.ToolOutputStyle.Render(preview))
	}
	
	fmt.Print("└─────────────────────────────────────────────────────\n")
}

// renderCurrentStepDetails renders details about the currently running step
func (r *InteractiveWorkflowRenderer) renderCurrentStepDetails() {
	// Find currently running step
	var runningStep string
	for stepName, status := range r.stepStatuses {
		if status == "running" {
			runningStep = stepName
			break
		}
	}
	
	if runningStep != "" {
		fmt.Printf("\n%s\n", style.SectionStyle.Render("Current Step Details"))
		fmt.Printf("%s %s %s\n", 
			r.getSpinner(), 
			style.ToolNameStyle.Render(runningStep), 
			style.DimStyle.Render("is currently running..."))
		
		if startTime, ok := r.stepStartTimes[runningStep]; ok {
			elapsed := time.Since(startTime)
			fmt.Printf("%s %v\n", 
				style.DimStyle.Render("Running for:"), 
				elapsed.Round(time.Second))
		}
		
		// Show live output if available
		if output, ok := r.stepOutputs[runningStep]; ok && output != "" {
			fmt.Printf("\n%s\n", style.DimStyle.Render("Live Output:"))
			lines := strings.Split(output, "\n")
			// Show last few lines
			start := 0
			if len(lines) > 5 {
				start = len(lines) - 5
			}
			for i := start; i < len(lines); i++ {
				if strings.TrimSpace(lines[i]) != "" {
					fmt.Printf("  %s\n", style.ToolOutputStyle.Render(lines[i]))
				}
			}
		}
	}
}

// renderSummary renders the final execution summary
func (r *InteractiveWorkflowRenderer) renderSummary() {
	fmt.Printf("\n%s\n", style.SectionStyle.Render("Execution Summary"))
	
	totalDuration := time.Since(r.startTime)
	completedSteps := 0
	failedSteps := 0
	
	for _, status := range r.stepStatuses {
		switch status {
		case "completed":
			completedSteps++
		case "failed":
			failedSteps++
		}
	}
	
	if failedSteps == 0 {
		fmt.Printf("%s Workflow completed successfully!\n", 
			style.SuccessStyle.Render("🎉"))
	} else {
		fmt.Printf("%s Workflow completed with %d failed steps\n", 
			style.ErrorStyle.Render("⚠️"), failedSteps)
	}
	
	fmt.Printf("%s %v\n", 
		style.DimStyle.Render("Total duration:"), 
		totalDuration.Round(time.Millisecond))
	fmt.Printf("%s %d/%d\n", 
		style.DimStyle.Render("Steps completed:"), 
		completedSteps, r.totalSteps)
	
	if r.executionID != "" {
		fmt.Printf("%s %s\n", 
			style.DimStyle.Render("Execution ID:"), 
			r.executionID)
	}
}

// getSpinner returns a spinning animation character
func (r *InteractiveWorkflowRenderer) getSpinner() string {
	spinners := []string{"⠋", "⠙", "⠹", "⠸", "⠼", "⠴", "⠦", "⠧", "⠇", "⠏"}
	// Use current time to animate
	index := (time.Now().UnixNano() / 100000000) % int64(len(spinners))
	return style.WarningStyle.Render(spinners[index])
}