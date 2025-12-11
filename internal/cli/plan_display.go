package cli

import (
	"bufio"
	"encoding/json"
	"fmt"
	"os"
	"strings"

	"github.com/kubiyabot/cli/internal/kubiya"
	"github.com/kubiyabot/cli/internal/style"
	"gopkg.in/yaml.v3"
)

// PlanDisplayer displays execution plans with progressive disclosure
type PlanDisplayer struct {
	plan         *kubiya.PlanResponse
	outputFormat string
	interactive  bool
}

// NewPlanDisplayer creates a new plan displayer
func NewPlanDisplayer(plan *kubiya.PlanResponse, outputFormat string, interactive bool) *PlanDisplayer {
	return &PlanDisplayer{
		plan:         plan,
		outputFormat: outputFormat,
		interactive:  interactive,
	}
}

// DisplayPlan shows the plan in the appropriate format
func (pd *PlanDisplayer) DisplayPlan() error {
	switch pd.outputFormat {
	case "json":
		return pd.displayJSON()
	case "yaml":
		return pd.displayYAML()
	default: // "text"
		if pd.interactive {
			return pd.displayInteractive()
		}
		return pd.displayText()
	}
}

// displayInteractive shows plan with drill-down prompts
func (pd *PlanDisplayer) displayInteractive() error {
	// 1. Show banner
	fmt.Println()
	fmt.Println(style.CreateBanner("Task Execution Plan", "📋"))
	fmt.Println()

	// 2. Show summary
	pd.displaySummary()
	fmt.Println()

	// 3. Show recommended execution
	pd.displayRecommendedExecution()
	fmt.Println()

	// 4. Show cost estimate
	pd.displayCostEstimate()
	fmt.Println()

	// 5. Offer drill-down options
	return pd.offerDrillDown()
}

// displayText shows plan in non-interactive text format
func (pd *PlanDisplayer) displayText() error {
	// Show banner
	fmt.Println()
	fmt.Println(style.CreateBanner("Task Execution Plan", "📋"))
	fmt.Println()

	// Show summary
	pd.displaySummary()
	fmt.Println()

	// Show recommended execution
	pd.displayRecommendedExecution()
	fmt.Println()

	// Show cost estimate
	pd.displayCostEstimate()
	fmt.Println()

	// Show task breakdown
	pd.displayTaskBreakdownText()
	fmt.Println()

	// Show risks and prerequisites
	pd.displayRisksAndPrereqsText()
	fmt.Println()

	return nil
}

// displaySummary shows high-level plan overview
func (pd *PlanDisplayer) displaySummary() {
	summary := map[string]string{
		"Title":      pd.plan.Title,
		"Summary":    pd.plan.Summary,
		"Complexity": fmt.Sprintf("%d story points (%s confidence)", pd.plan.Complexity.StoryPoints, pd.plan.Complexity.Confidence),
	}
	fmt.Println(style.CreateMetadataBox(summary))
}

// displayRecommendedExecution shows agent/team recommendation
func (pd *PlanDisplayer) displayRecommendedExecution() {
	rec := pd.plan.RecommendedExecution

	fmt.Println(style.HeadingStyle.Render("Recommended Execution"))
	fmt.Println()

	entityType := strings.Title(rec.EntityType)
	fmt.Printf("  %s %s: %s\n",
		style.RobotIconStyle.Render("🤖"),
		entityType,
		style.HighlightStyle.Render(rec.EntityName))

	if rec.RecommendedEnvironmentName != nil {
		fmt.Printf("  %s Environment: %s\n",
			style.DimStyle.Render("🌍"),
			style.DimStyle.Render(*rec.RecommendedEnvironmentName))
	}

	if rec.RecommendedWorkerQueueName != nil {
		fmt.Printf("  %s Worker Queue: %s\n",
			style.DimStyle.Render("⚙️"),
			style.DimStyle.Render(*rec.RecommendedWorkerQueueName))
	}

	fmt.Println()
	fmt.Printf("  %s %s\n",
		style.DimStyle.Render("💡"),
		style.DimStyle.Render(rec.Reasoning))
}

// displayCostEstimate shows cost breakdown
func (pd *PlanDisplayer) displayCostEstimate() {
	cost := pd.plan.CostEstimate

	fmt.Println(style.HeadingStyle.Render("Cost Estimate"))
	fmt.Println()

	fmt.Printf("  %s Total Cost: %s\n",
		style.RobotIconStyle.Render("💰"),
		style.HighlightStyle.Render(fmt.Sprintf("$%.2f", cost.EstimatedCostUSD)))

	// Show top cost components
	if len(cost.LLMCosts) > 0 {
		totalLLMCost := 0.0
		for _, llm := range cost.LLMCosts {
			totalLLMCost += llm.TotalCost
		}
		fmt.Printf("  %s LLM Costs: $%.2f\n",
			style.DimStyle.Render("🧠"),
			totalLLMCost)
	}

	if len(cost.ToolCosts) > 0 {
		totalToolCost := 0.0
		for _, tc := range cost.ToolCosts {
			totalToolCost += tc.TotalCost
		}
		fmt.Printf("  %s Tool Costs: $%.2f\n",
			style.DimStyle.Render("🔧"),
			totalToolCost)
	}

	// Show realized savings
	if pd.plan.RealizedSavings.MoneySaved > 0 {
		fmt.Println()
		fmt.Printf("  %s Realized Savings: %s (%.1f hours saved)\n",
			style.RobotIconStyle.Render("✨"),
			style.SuccessStyle.Render(fmt.Sprintf("$%.2f", pd.plan.RealizedSavings.MoneySaved)),
			pd.plan.RealizedSavings.TimeSavedHours)
	}
}

// offerDrillDown offers interactive drill-down options
func (pd *PlanDisplayer) offerDrillDown() error {
	for {
		fmt.Println()
		fmt.Println(style.HeadingStyle.Render("What would you like to see?"))
		fmt.Println()

		options := []string{
			"1. Task Breakdown",
			"2. Detailed Cost Analysis",
			"3. Risks & Prerequisites",
			"4. Full Plan (JSON)",
			"5. Continue to Approval",
		}

		for _, opt := range options {
			fmt.Println("  " + opt)
		}

		fmt.Println()
		fmt.Print(style.UserPromptStyle.Render(" Choice "))
		fmt.Print(" ")

		reader := bufio.NewReader(os.Stdin)
		choice, _ := reader.ReadString('\n')
		choice = strings.TrimSpace(choice)

		switch choice {
		case "1":
			pd.displayTaskBreakdown()
		case "2":
			pd.displayDetailedCosts()
		case "3":
			pd.displayRisksAndPrereqs()
		case "4":
			pd.displayJSON()
		case "5":
			return nil // Continue to approval
		default:
			fmt.Println(style.CreateWarningBox("Invalid choice, please try again"))
		}
	}
}

// displayTaskBreakdown shows hierarchical task structure
func (pd *PlanDisplayer) displayTaskBreakdown() {
	fmt.Println()
	fmt.Println(style.CreateBanner("Task Breakdown", "📝"))
	fmt.Println()

	for i, breakdown := range pd.plan.TeamBreakdown {
		entityName := breakdown.TeamName
		if breakdown.AgentName != nil {
			entityName = *breakdown.AgentName
		}

		fmt.Printf("%s %s\n",
			style.HeadingStyle.Render(fmt.Sprintf("Phase %d", i+1)),
			style.HighlightStyle.Render(entityName))
		fmt.Println()

		// Show responsibilities
		fmt.Println(style.DimStyle.Render("  Responsibilities:"))
		for _, resp := range breakdown.Responsibilities {
			fmt.Printf("    %s %s\n", style.BulletStyle.Render("•"), resp)
		}
		fmt.Println()

		// Show tasks
		if len(breakdown.Tasks) > 0 {
			fmt.Println(style.DimStyle.Render("  Tasks:"))
			pd.displayTaskList(breakdown.Tasks, 2)
		}

		// Show estimated time
		fmt.Printf("  %s Estimated Time: %.1f hours\n\n",
			style.RobotIconStyle.Render("⏱️"),
			breakdown.EstimatedTimeHours)
	}

	fmt.Println()
	fmt.Print(style.HelpTextStyle.Render("Press Enter to continue..."))
	bufio.NewReader(os.Stdin).ReadString('\n')
}

// displayTaskBreakdownText shows task breakdown in non-interactive mode
func (pd *PlanDisplayer) displayTaskBreakdownText() {
	fmt.Println(style.HeadingStyle.Render("Task Breakdown"))
	fmt.Println()

	for i, breakdown := range pd.plan.TeamBreakdown {
		entityName := breakdown.TeamName
		if breakdown.AgentName != nil {
			entityName = *breakdown.AgentName
		}

		fmt.Printf("%s %s\n",
			style.DimStyle.Render(fmt.Sprintf("Phase %d:", i+1)),
			style.HighlightStyle.Render(entityName))

		// Show responsibilities
		if len(breakdown.Responsibilities) > 0 {
			fmt.Println(style.DimStyle.Render("  Responsibilities:"))
			for _, resp := range breakdown.Responsibilities {
				fmt.Printf("    %s %s\n", style.BulletStyle.Render("•"), resp)
			}
		}

		// Show estimated time
		fmt.Printf("  %s Estimated Time: %.1f hours\n",
			style.DimStyle.Render("⏱️"),
			breakdown.EstimatedTimeHours)
		fmt.Println()
	}
}

// displayTaskList recursively displays task hierarchy
func (pd *PlanDisplayer) displayTaskList(tasks []kubiya.TaskItem, indent int) {
	indentStr := strings.Repeat("  ", indent)

	for _, task := range tasks {
		statusIcon := pd.getTaskStatusIcon(task.Status)
		priorityIcon := pd.getPriorityIcon(task.Priority)

		fmt.Printf("%s%s %s %s\n",
			indentStr,
			statusIcon,
			priorityIcon,
			task.Title)

		// Show dependencies
		if len(task.Dependencies) > 0 {
			fmt.Printf("%s  %s Depends on: %v\n",
				indentStr,
				style.DimStyle.Render("↳"),
				task.Dependencies)
		}

		// Recursive subtasks
		if len(task.Subtasks) > 0 {
			pd.displayTaskList(task.Subtasks, indent+1)
		}
	}
}

// getTaskStatusIcon returns status icon
func (pd *PlanDisplayer) getTaskStatusIcon(status string) string {
	switch status {
	case "done":
		return style.SuccessStyle.Render("✓")
	case "in_progress":
		return style.WarningStyle.Render("⟳")
	default: // "pending"
		return style.DimStyle.Render("○")
	}
}

// getPriorityIcon returns priority icon
func (pd *PlanDisplayer) getPriorityIcon(priority string) string {
	switch priority {
	case "high":
		return style.ErrorStyle.Render("↑↑")
	case "medium":
		return style.WarningStyle.Render("↑")
	default: // "low"
		return style.DimStyle.Render("→")
	}
}

// displayDetailedCosts shows full cost breakdown
func (pd *PlanDisplayer) displayDetailedCosts() {
	fmt.Println()
	fmt.Println(style.CreateBanner("Detailed Cost Analysis", "💰"))
	fmt.Println()

	cost := pd.plan.CostEstimate

	// LLM Costs
	if len(cost.LLMCosts) > 0 {
		fmt.Println(style.HeadingStyle.Render("LLM Costs"))
		fmt.Println()

		for _, llm := range cost.LLMCosts {
			fmt.Printf("  %s %s\n",
				style.RobotIconStyle.Render("🧠"),
				style.HighlightStyle.Render(llm.ModelID))
			fmt.Printf("    Input Tokens:  %d ($%.4f per 1k)\n",
				llm.EstimatedInputTokens, llm.CostPer1kInputTokens)
			fmt.Printf("    Output Tokens: %d ($%.4f per 1k)\n",
				llm.EstimatedOutputTokens, llm.CostPer1kOutputTokens)
			fmt.Printf("    Total: %s\n\n",
				style.SuccessStyle.Render(fmt.Sprintf("$%.4f", llm.TotalCost)))
		}
	}

	// Tool Costs
	if len(cost.ToolCosts) > 0 {
		fmt.Println(style.HeadingStyle.Render("Tool Costs"))
		fmt.Println()

		for _, tool := range cost.ToolCosts {
			fmt.Printf("  %s %s\n",
				style.RobotIconStyle.Render("🔧"),
				style.HighlightStyle.Render(tool.Category))
			fmt.Printf("    Calls: %d × $%.4f = $%.4f\n\n",
				tool.EstimatedCalls, tool.CostPerCall, tool.TotalCost)
		}
	}

	fmt.Println()
	fmt.Printf("Total Estimated Cost: %s\n",
		style.HighlightStyle.Render(fmt.Sprintf("$%.2f", cost.EstimatedCostUSD)))

	fmt.Println()
	fmt.Print(style.HelpTextStyle.Render("Press Enter to continue..."))
	bufio.NewReader(os.Stdin).ReadString('\n')
}

// displayRisksAndPrereqs shows risks and prerequisites
func (pd *PlanDisplayer) displayRisksAndPrereqs() {
	fmt.Println()
	fmt.Println(style.CreateBanner("Risks & Prerequisites", "⚠️"))
	fmt.Println()

	// Risks
	if len(pd.plan.Risks) > 0 {
		fmt.Println(style.HeadingStyle.Render("Identified Risks"))
		fmt.Println()

		for i, risk := range pd.plan.Risks {
			fmt.Printf("  %d. %s %s\n", i+1,
				style.WarningStyle.Render("⚠️"),
				risk)
		}
		fmt.Println()
	}

	// Prerequisites
	if len(pd.plan.Prerequisites) > 0 {
		fmt.Println(style.HeadingStyle.Render("Prerequisites"))
		fmt.Println()

		for i, prereq := range pd.plan.Prerequisites {
			fmt.Printf("  %d. %s %s\n", i+1,
				style.InfoStyle.Render("📋"),
				prereq)
		}
		fmt.Println()
	}

	// Success Criteria
	if len(pd.plan.SuccessCriteria) > 0 {
		fmt.Println(style.HeadingStyle.Render("Success Criteria"))
		fmt.Println()

		for i, criteria := range pd.plan.SuccessCriteria {
			fmt.Printf("  %d. %s %s\n", i+1,
				style.SuccessStyle.Render("✓"),
				criteria)
		}
		fmt.Println()
	}

	fmt.Print(style.HelpTextStyle.Render("Press Enter to continue..."))
	bufio.NewReader(os.Stdin).ReadString('\n')
}

// displayRisksAndPrereqsText shows risks and prerequisites in non-interactive mode
func (pd *PlanDisplayer) displayRisksAndPrereqsText() {
	// Risks
	if len(pd.plan.Risks) > 0 {
		fmt.Println(style.HeadingStyle.Render("Identified Risks"))
		fmt.Println()

		for _, risk := range pd.plan.Risks {
			fmt.Printf("  %s %s\n",
				style.WarningStyle.Render("⚠️"),
				risk)
		}
		fmt.Println()
	}

	// Prerequisites
	if len(pd.plan.Prerequisites) > 0 {
		fmt.Println(style.HeadingStyle.Render("Prerequisites"))
		fmt.Println()

		for _, prereq := range pd.plan.Prerequisites {
			fmt.Printf("  %s %s\n",
				style.InfoStyle.Render("📋"),
				prereq)
		}
		fmt.Println()
	}

	// Success Criteria
	if len(pd.plan.SuccessCriteria) > 0 {
		fmt.Println(style.HeadingStyle.Render("Success Criteria"))
		fmt.Println()

		for _, criteria := range pd.plan.SuccessCriteria {
			fmt.Printf("  %s %s\n",
				style.SuccessStyle.Render("✓"),
				criteria)
		}
		fmt.Println()
	}
}

// displayJSON outputs plan as JSON
func (pd *PlanDisplayer) displayJSON() error {
	data, err := json.MarshalIndent(pd.plan, "", "  ")
	if err != nil {
		return fmt.Errorf("failed to marshal plan to JSON: %w", err)
	}
	fmt.Println(string(data))
	return nil
}

// displayYAML outputs plan as YAML
func (pd *PlanDisplayer) displayYAML() error {
	data, err := yaml.Marshal(pd.plan)
	if err != nil {
		return fmt.Errorf("failed to marshal plan to YAML: %w", err)
	}
	fmt.Println(string(data))
	return nil
}
