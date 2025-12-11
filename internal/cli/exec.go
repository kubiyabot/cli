package cli

import (
	"bufio"
	"context"
	"encoding/json"
	"fmt"
	"os"
	"strings"
	"time"

	"github.com/kubiyabot/cli/internal/config"
	"github.com/kubiyabot/cli/internal/controlplane"
	"github.com/kubiyabot/cli/internal/controlplane/entities"
	"github.com/kubiyabot/cli/internal/kubiya"
	"github.com/kubiyabot/cli/internal/style"
	"github.com/spf13/cobra"
)

// NewExecCommand creates the exec command with auto-planning support
func NewExecCommand(cfg *config.Config) *cobra.Command {
	var (
		planFile       string
		autoApprove    bool
		outputFormat   string
		savePlanPath   string
		nonInteractive bool
		priority       string
	)

	cmd := &cobra.Command{
		Use:   "exec [agent|team <id>] <prompt>",
		Short: "Execute a task with intelligent planning",
		Long: `Execute a task with automatic agent/team selection and planning.

When agent/team is not specified, the planner automatically:
1. Analyzes your task
2. Selects the best agent or team
3. Creates a detailed execution plan
4. Shows cost estimates
5. Asks for approval
6. Executes the plan

All plans are automatically saved to ~/.kubiya/plans/ for future reference.`,
		Example: `  # Auto-planning mode (recommended)
  kubiya exec "Deploy my app to production"
  kubiya exec "Analyze security vulnerabilities in my cluster"
  kubiya exec "Create a new microservice with tests" --yes

  # Load and execute existing plan
  kubiya exec --plan-file ~/.kubiya/plans/abc123.json

  # Direct execution (without planning)
  kubiya exec agent 8064f4c8 "Deploy to production"
  kubiya exec team my-team "Run integration tests"

  # Output formats
  kubiya exec "task" --output json
  kubiya exec "task" --output yaml
  kubiya exec "task" --output text`,
		RunE: func(cmd *cobra.Command, args []string) error {
			// Create control plane client
			client, err := controlplane.New(cfg.APIKey, cfg.Debug)
			if err != nil {
				return fmt.Errorf("failed to create control plane client: %w", err)
			}

			// Detect non-interactive mode
			if os.Getenv("KUBIYA_NON_INTERACTIVE") != "" {
				nonInteractive = true
				autoApprove = true
			}

			executor := &ExecCommand{
				cfg:            cfg,
				client:         client,
				autoApprove:    autoApprove,
				outputFormat:   outputFormat,
				savePlanPath:   savePlanPath,
				nonInteractive: nonInteractive,
				priority:       priority,
			}

			// Route based on input
			if planFile != "" {
				// Load existing plan
				return executor.ExecuteFromPlan(cmd.Context(), planFile)
			}

			// Parse arguments
			if len(args) >= 2 && (args[0] == "agent" || args[0] == "team") {
				// Direct execution mode: kubiya exec agent <id> "task"
				entityType := args[0]
				entityID := args[1]
				prompt := strings.Join(args[2:], " ")

				if prompt == "" {
					return fmt.Errorf("prompt is required")
				}

				return executor.ExecuteDirect(cmd.Context(), entityType, entityID, prompt)
			}

			// Auto-planning mode
			prompt := strings.Join(args, " ")
			if prompt == "" {
				return fmt.Errorf("prompt is required")
			}

			return executor.ExecuteWithPlanning(cmd.Context(), prompt)
		},
	}

	// Flags
	cmd.Flags().StringVar(&planFile, "plan-file", "", "Execute from saved plan file")
	cmd.Flags().BoolVarP(&autoApprove, "yes", "y", false, "Auto-approve plan without confirmation")
	cmd.Flags().StringVarP(&outputFormat, "output", "o", "text", "Output format: text, json, yaml")
	cmd.Flags().StringVar(&savePlanPath, "save-plan", "", "Custom path to save plan (default: ~/.kubiya/plans/<plan-id>.json)")
	cmd.Flags().BoolVar(&nonInteractive, "non-interactive", false, "Non-interactive mode (skip all prompts)")
	cmd.Flags().StringVar(&priority, "priority", "medium", "Task priority: low, medium, high, critical")

	return cmd
}

// ExecCommand handles execution with auto-planning
type ExecCommand struct {
	cfg            *config.Config
	client         *controlplane.Client
	autoApprove    bool
	outputFormat   string
	savePlanPath   string
	nonInteractive bool
	priority       string
	currentPlan    *kubiya.PlanResponse
}

// ExecuteWithPlanning runs the full planning workflow
func (ec *ExecCommand) ExecuteWithPlanning(ctx context.Context, prompt string) error {
	// 1. Show planning banner
	fmt.Println()
	fmt.Println(style.CreateBanner("Intelligent Task Planning", "🤖"))
	fmt.Println()

	// 2. Fetch resources
	fmt.Println(style.CreateHelpBox("Discovering available resources..."))
	fetcher := NewResourceFetcher(ec.client)
	resources, err := fetcher.FetchAllResources(ctx)
	if err != nil {
		return fmt.Errorf("failed to fetch resources: %w", err)
	}

	fmt.Printf("  %s Found %d agents, %d teams, %d environments\n\n",
		style.SuccessStyle.Render("✓"),
		len(resources.Agents),
		len(resources.Teams),
		len(resources.Environments))

	// 3. Create plan with streaming
	fmt.Println(style.CreateHelpBox("Generating execution plan..."))
	fmt.Println()

	planReq := &kubiya.PlanRequest{
		Description:  prompt,
		Priority:     ec.priority,
		Agents:       resources.Agents,
		Teams:        resources.Teams,
		Environments: resources.Environments,
		WorkerQueues: resources.Queues,
		OutputFormat: ec.outputFormat,
	}

	kubiyaClient := kubiya.NewClient(ec.cfg)
	plannerClient := kubiya.NewPlannerClient(kubiyaClient)

	// Stream plan generation
	var plan *kubiya.PlanResponse
	if !ec.nonInteractive {
		plan, err = ec.streamPlanGeneration(ctx, plannerClient, planReq)
	} else {
		plan, err = plannerClient.CreatePlan(ctx, planReq)
	}

	if err != nil {
		return fmt.Errorf("failed to create plan: %w", err)
	}

	ec.currentPlan = plan

	// 4. Auto-save plan
	storage, _ := NewPlanStorageManager()
	savedPlan, err := storage.SavePlan(plan, prompt)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Warning: Failed to save plan: %v\n", err)
	} else {
		fmt.Println()
		fmt.Printf("%s Plan saved to: %s\n",
			style.DimStyle.Render("💾"),
			style.HighlightStyle.Render(savedPlan.FilePath))
	}

	// 5. Display plan
	fmt.Println()
	displayer := NewPlanDisplayer(plan, ec.outputFormat, !ec.nonInteractive)

	if err := displayer.DisplayPlan(); err != nil {
		return err
	}

	// 6. Ask for approval (unless --yes)
	if !ec.autoApprove && !ec.nonInteractive {
		fmt.Println()
		if !ec.askApproval() {
			fmt.Println(style.CreateWarningBox("Execution cancelled by user"))
			return nil
		}
	}

	// Mark as approved
	if savedPlan != nil {
		storage.MarkApproved(savedPlan)
	}

	// 7. Execute
	return ec.executeFromPlan(ctx, plan, savedPlan)
}

// streamPlanGeneration streams plan with progress updates
func (ec *ExecCommand) streamPlanGeneration(ctx context.Context, client *kubiya.PlannerClient, req *kubiya.PlanRequest) (*kubiya.PlanResponse, error) {
	eventChan, errChan := client.StreamPlanProgress(ctx, req)

	var plan *kubiya.PlanResponse
	progressBar := NewProgressBar()

	for {
		select {
		case event, ok := <-eventChan:
			if !ok {
				progressBar.Complete()
				return plan, nil
			}

			switch event.Type {
			case "progress":
				if stage, ok := event.Data["stage"].(string); ok {
					progressBar.SetStage(stage)
				}
				if message, ok := event.Data["message"].(string); ok {
					if progress, ok := event.Data["progress"].(float64); ok {
						progressBar.Update(int(progress), message)
					}
				}

			case "thinking":
				if content, ok := event.Data["content"].(string); ok {
					progressBar.ShowThinking(content)
				}

			case "tool_call":
				if toolName, ok := event.Data["tool_name"].(string); ok {
					progressBar.ShowToolCall(toolName)
				}

			case "resources_summary":
				if summary, ok := event.Data["summary"].(string); ok {
					progressBar.ShowResourcesSummary(summary)
				}

			case "complete":
				// Extract plan from complete event
				if planData, ok := event.Data["plan"].(map[string]interface{}); ok {
					// Convert map to PlanResponse via JSON
					planBytes, _ := json.Marshal(planData)
					json.Unmarshal(planBytes, &plan)
				}

			case "error":
				if errMsg, ok := event.Data["error"].(string); ok {
					return nil, fmt.Errorf("planning error: %s", errMsg)
				}
			}

		case err := <-errChan:
			return nil, err

		case <-ctx.Done():
			return nil, ctx.Err()
		}
	}
}

// askApproval prompts user for approval
func (ec *ExecCommand) askApproval() bool {
	fmt.Println(style.CreateDivider(80))
	fmt.Println()
	fmt.Printf("%s\n", style.InfoStyle.Render("Do you want to proceed with this plan?"))
	fmt.Printf("  • Estimated cost: $%.2f\n", ec.currentPlan.CostEstimate.EstimatedCostUSD)
	fmt.Printf("  • Estimated time: %.1f hours\n", ec.getTotalEstimatedTime())
	fmt.Println()
	fmt.Print(style.UserPromptStyle.Render(" Approve? (y/N) "))
	fmt.Print(" ")

	reader := bufio.NewReader(os.Stdin)
	response, _ := reader.ReadString('\n')
	response = strings.TrimSpace(strings.ToLower(response))

	return response == "y" || response == "yes"
}

// getTotalEstimatedTime calculates total estimated time from plan
func (ec *ExecCommand) getTotalEstimatedTime() float64 {
	total := 0.0
	for _, breakdown := range ec.currentPlan.TeamBreakdown {
		total += breakdown.EstimatedTimeHours
	}
	return total
}

// executeFromPlan executes the approved plan
func (ec *ExecCommand) executeFromPlan(ctx context.Context, plan *kubiya.PlanResponse, savedPlan *SavedPlan) error {
	fmt.Println()
	fmt.Println(style.CreateBanner("Executing Plan", "🚀"))
	fmt.Println()

	rec := plan.RecommendedExecution

	// Submit execution
	streamFlag := true

	var execution *entities.AgentExecution
	var err error

	if rec.EntityType == "agent" {
		req := &entities.ExecuteAgentRequest{
			Prompt:        plan.Summary,
			WorkerQueueID: *rec.RecommendedWorkerQueueID,
			Stream:        &streamFlag,
		}
		execution, err = ec.client.ExecuteAgentV2(rec.EntityID, req)
	} else {
		req := &entities.ExecuteTeamRequest{
			Prompt:        plan.Summary,
			WorkerQueueID: *rec.RecommendedWorkerQueueID,
			Stream:        &streamFlag,
		}
		execution, err = ec.client.ExecuteTeamV2(rec.EntityID, req)
	}

	if err != nil {
		return fmt.Errorf("failed to submit execution: %w", err)
	}

	// Mark as executed
	if savedPlan != nil {
		storage, _ := NewPlanStorageManager()
		storage.MarkExecuted(savedPlan, execution.GetID())
	}

	// Stream output
	return ec.streamExecutionOutput(ctx, execution.GetID())
}

// streamExecutionOutput streams execution output to terminal
func (ec *ExecCommand) streamExecutionOutput(ctx context.Context, executionID string) error {
	eventChan, errChan := ec.client.StreamExecutionOutput(ctx, executionID)

	var fullResponse strings.Builder
	streamStarted := false

	for {
		select {
		case event, ok := <-eventChan:
			if !ok {
				// Channel closed
				if !streamStarted {
					// No streaming data received, fetch final result
					finalExecution, err := ec.client.GetExecution(executionID)
					if err == nil && finalExecution.Response != nil {
						fmt.Println(*finalExecution.Response)
					}
				}
				fmt.Println()
				fmt.Println()
				fmt.Println(style.CreateSuccessBox("Execution completed successfully"))
				return nil
			}

			streamStarted = true

			switch event.Type {
			case "chunk":
				// Stream content in real-time
				fmt.Print(style.OutputStyle.Render(event.Content))
				fullResponse.WriteString(event.Content)
			case "error":
				// Show error
				fmt.Println()
				fmt.Println()
				fmt.Println(style.CreateErrorBox(event.Content))
				return fmt.Errorf("execution error: %s", event.Content)
			case "complete":
				// Completion
				fmt.Println()
				fmt.Println()
				fmt.Println(style.CreateSuccessBox("Execution completed successfully"))
				return nil
			case "status":
				// Status update (shown in debug mode only)
				if event.Status != nil {
					fmt.Printf(" %s ", style.CreateStatusBadge(string(*event.Status)))
				}
			}

		case err := <-errChan:
			if err != nil {
				fmt.Println()
				fmt.Println(style.CreateErrorBox(fmt.Sprintf("Streaming error: %v", err)))
				return fmt.Errorf("streaming error: %w", err)
			}

		case <-ctx.Done():
			fmt.Println()
			fmt.Println(style.CreateWarningBox("Execution interrupted by user"))
			return ctx.Err()
		}
	}
}

// ExecuteFromPlan loads and executes a saved plan
func (ec *ExecCommand) ExecuteFromPlan(ctx context.Context, planFile string) error {
	// 1. Load plan
	storage, err := NewPlanStorageManager()
	if err != nil {
		return err
	}

	savedPlan, err := storage.LoadPlan(planFile)
	if err != nil {
		return fmt.Errorf("failed to load plan: %w", err)
	}

	// 2. Show banner
	fmt.Println()
	fmt.Println(style.CreateBanner("Executing from Saved Plan", "📋"))
	fmt.Println()

	// 3. Display plan summary
	displayer := NewPlanDisplayer(savedPlan.Plan, ec.outputFormat, !ec.autoApprove)

	if err := displayer.DisplayPlan(); err != nil {
		return err
	}

	// 4. Check if already executed
	if savedPlan.ExecutedAt != nil {
		fmt.Println()
		fmt.Printf("%s This plan was already executed on %s\n",
			style.WarningStyle.Render("⚠️"),
			savedPlan.ExecutedAt.Format(time.RFC1123))

		if savedPlan.ExecutionID != "" {
			fmt.Printf("  Execution ID: %s\n", savedPlan.ExecutionID)
		}

		if !ec.nonInteractive {
			fmt.Println()
			fmt.Print(style.UserPromptStyle.Render(" Execute again? (y/N) "))
			fmt.Print(" ")

			reader := bufio.NewReader(os.Stdin)
			response, _ := reader.ReadString('\n')
			response = strings.TrimSpace(strings.ToLower(response))

			if response != "y" && response != "yes" {
				return nil
			}
		}
	}

	// 5. Ask for approval (unless --yes or already approved)
	if !ec.autoApprove && !savedPlan.Approved {
		ec.currentPlan = savedPlan.Plan
		fmt.Println()
		if !ec.askApproval() {
			fmt.Println(style.CreateWarningBox("Execution cancelled by user"))
			return nil
		}

		storage.MarkApproved(savedPlan)
	}

	// 6. Execute
	return ec.executeFromPlan(ctx, savedPlan.Plan, savedPlan)
}

// ExecuteDirect executes directly without planning
func (ec *ExecCommand) ExecuteDirect(ctx context.Context, entityType, entityID, prompt string) error {
	fmt.Println()
	fmt.Println(style.CreateBanner("Direct Execution", "⚡"))
	fmt.Println()

	fmt.Printf("Executing %s: %s\n", entityType, style.HighlightStyle.Render(entityID))
	fmt.Printf("Prompt: %s\n\n", style.DimStyle.Render(prompt))

	// Submit execution (use default queue for the entity's environment)
	streamFlag := true

	var execution *entities.AgentExecution
	var err error

	if entityType == "agent" {
		req := &entities.ExecuteAgentRequest{
			Prompt: prompt,
			Stream: &streamFlag,
		}
		execution, err = ec.client.ExecuteAgentV2(entityID, req)
	} else {
		req := &entities.ExecuteTeamRequest{
			Prompt: prompt,
			Stream: &streamFlag,
		}
		execution, err = ec.client.ExecuteTeamV2(entityID, req)
	}

	if err != nil {
		return fmt.Errorf("failed to submit execution: %w", err)
	}

	// Stream output
	return ec.streamExecutionOutput(ctx, execution.GetID())
}
