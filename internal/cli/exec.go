package cli

import (
	"bufio"
	"context"
	"encoding/json"
	"fmt"
	"os"
	"sort"
	"strings"
	"sync"
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

		// New execution mode flags
		local            bool
		queueIDs         []string
		queueNames       []string
		environment      string
		parentExecution  string
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
				cfg:             cfg,
				client:          client,
				autoApprove:     autoApprove,
				outputFormat:    outputFormat,
				savePlanPath:    savePlanPath,
				nonInteractive:  nonInteractive,
				priority:        priority,
				Local:           local,
				QueueIDs:        queueIDs,
				QueueNames:      queueNames,
				Environment:     environment,
				ParentExecution: parentExecution,
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

	// Execution mode flags
	cmd.Flags().BoolVar(&local, "local", false, "Run execution locally with ephemeral worker (uses fast planning)")
	cmd.Flags().StringSliceVar(&queueIDs, "queue", nil, "Worker queue ID(s) - comma-separated for parallel execution")
	cmd.Flags().StringSliceVar(&queueNames, "queue-name", nil, "Worker queue name(s) - alternative to IDs")
	cmd.Flags().StringVar(&environment, "environment", "", "Environment ID for execution (auto-detected if not specified)")
	cmd.Flags().StringVar(&parentExecution, "parent-execution", "", "Parent execution ID for conversation continuation")

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

	// New execution mode flags
	Local           bool     // --local: Run with ephemeral local worker (uses fast planning)
	QueueIDs        []string // --queue: Explicit queue selection (comma-separated)
	QueueNames      []string // --queue-name: Select by name instead of ID
	Environment     string   // --environment: Environment ID for execution
	ParentExecution string   // --parent-execution: Parent execution ID for conversation continuation
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
	if ec.Local {
		fmt.Println(style.CreateHelpBox("Quick agent/team selection for local execution..."))
	} else {
		fmt.Println(style.CreateHelpBox("Generating execution plan..."))
	}
	fmt.Println()

	planReq := &kubiya.PlanRequest{
		Description:  prompt,
		Priority:     ec.priority,
		Agents:       []kubiya.AgentInfo{},     // Let backend discover resources
		Teams:        []kubiya.TeamInfo{},      // Let backend discover resources
		Environments: resources.Environments,
		WorkerQueues: resources.Queues,
		OutputFormat: ec.outputFormat,
		QuickMode:    ec.Local, // Use quick mode for local execution
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
	var savedPlan *SavedPlan
	if plan != nil {
		var saveErr error
		savedPlan, saveErr = storage.SavePlan(plan, prompt)
		if saveErr != nil {
			fmt.Fprintf(os.Stderr, "Warning: Failed to save plan: %v\n", saveErr)
		} else {
			fmt.Println()
			fmt.Printf("%s Plan saved to: %s\n",
				style.DimStyle.Render("💾"),
				style.HighlightStyle.Render(savedPlan.FilePath))
		}
	}

	// 5. Display plan
	if plan == nil {
		return fmt.Errorf("plan generation failed: no plan returned")
	}

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

	// 7. Execute - route based on flags
	if ec.Local {
		return ec.executeLocal(ctx, plan)
	}

	// Check for explicit queue selection
	if len(ec.QueueIDs) > 0 || len(ec.QueueNames) > 0 {
		queueIDs, err := ec.resolveQueueIDs(ctx)
		if err != nil {
			return fmt.Errorf("failed to resolve queues: %w", err)
		}

		if len(queueIDs) > 1 {
			// Multi-queue parallel execution
			return ec.executeMultiQueue(ctx, plan, queueIDs)
		} else {
			// Single queue execution
			return ec.executeSingleQueue(ctx, plan, queueIDs[0])
		}
	}

	// Default: use planner recommendation
	return ec.executeFromPlan(ctx, plan, savedPlan)
}

// streamPlanGeneration streams plan with progress updates
func (ec *ExecCommand) streamPlanGeneration(ctx context.Context, client *kubiya.PlannerClient, req *kubiya.PlanRequest) (*kubiya.PlanResponse, error) {
	eventChan, errChan := client.StreamPlanProgress(ctx, req)

	var plan *kubiya.PlanResponse
	var planObj kubiya.PlanResponse // Declare outside loop to preserve across iterations
	progressBar := NewProgressBar()

	for {
		select {
		case event, ok := <-eventChan:
			if !ok {
				progressBar.Complete()
				if plan == nil {
					return nil, fmt.Errorf("plan generation completed but no plan was received from the planner service")
				}
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
					planBytes, err := json.Marshal(planData)
					if err != nil {
						return nil, fmt.Errorf("failed to marshal plan data: %w", err)
					}

					// Unmarshal into the outer planObj variable
					err = json.Unmarshal(planBytes, &planObj)
					if err != nil {
						return nil, fmt.Errorf("failed to unmarshal plan: %w", err)
					}

					// Assign pointer to plan
					plan = &planObj

					// Complete the progress bar and return the plan
					progressBar.Complete()
					return plan, nil
				}

			case "error":
				if errMsg, ok := event.Data["error"].(string); ok {
					return nil, fmt.Errorf("planning error: %s", errMsg)
				}
				if errMsg, ok := event.Data["message"].(string); ok {
					return nil, fmt.Errorf("planning error: %s", errMsg)
				}
				return nil, fmt.Errorf("planning error: unknown error format")
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

	// Use recommended queue from plan (backend already validated it's healthy during planning)
	// OR pass empty string to let backend auto-select
	var queueID string
	if rec.RecommendedWorkerQueueID != nil {
		queueID = *rec.RecommendedWorkerQueueID
	}
	// If empty, backend will auto-select a healthy queue

	// Submit and stream execution (backend handles queue selection if not specified)
	execution, err := ec.submitAndStreamExecution(ctx, plan, queueID)
	if err != nil {
		return err
	}

	// Mark as executed
	if savedPlan != nil {
		storage, _ := NewPlanStorageManager()
		storage.MarkExecuted(savedPlan, execution.GetID())
	}

	return nil
}

// submitAndStreamExecution submits execution to a queue and streams output
func (ec *ExecCommand) submitAndStreamExecution(ctx context.Context, plan *kubiya.PlanResponse, queueID string) (*entities.AgentExecution, error) {
	rec := plan.RecommendedExecution
	streamFlag := true

	var execution *entities.AgentExecution
	var err error

	// Only set WorkerQueueID if explicitly provided, otherwise let backend auto-select
	var queuePtr *string
	if queueID != "" {
		queuePtr = &queueID
	}

	// Set parent execution ID if provided (for conversation continuation)
	var parentExecPtr *string
	if ec.ParentExecution != "" {
		parentExecPtr = &ec.ParentExecution
	}

	if rec.EntityType == "agent" {
		req := &entities.ExecuteAgentRequest{
			Prompt:            plan.Summary,
			WorkerQueueID:     queuePtr,
			ParentExecutionID: parentExecPtr,
			Stream:            &streamFlag,
		}
		execution, err = ec.client.ExecuteAgentV2(rec.EntityID, req)
	} else {
		req := &entities.ExecuteTeamRequest{
			Prompt:            plan.Summary,
			WorkerQueueID:     queuePtr,
			ParentExecutionID: parentExecPtr,
			Stream:            &streamFlag,
		}
		execution, err = ec.client.ExecuteTeamV2(rec.EntityID, req)
	}

	if err != nil {
		return nil, fmt.Errorf("failed to submit execution: %w", err)
	}

	// Stream output
	if err := ec.streamExecutionOutput(ctx, execution.GetID()); err != nil {
		return execution, err
	}

	return execution, nil
}

// executeLocal executes plan using ephemeral local worker
func (ec *ExecCommand) executeLocal(ctx context.Context, plan *kubiya.PlanResponse) error {
	fmt.Println()
	fmt.Println(style.CreateBanner("Local Execution Mode", "💻"))
	fmt.Println()

	// Get environment from plan
	envID := plan.RecommendedExecution.RecommendedEnvironmentID
	if envID == nil {
		return fmt.Errorf("no environment specified in plan")
	}

	// Create ephemeral queue
	fmt.Println("📦 Creating ephemeral worker queue...")
	queue, err := ec.createEphemeralQueue(ctx, *envID)
	if err != nil {
		return fmt.Errorf("failed to create ephemeral queue: %w", err)
	}
	queueID := queue.ID
	fmt.Printf("✓ Queue created: %s\n\n", queueID)

	// Cleanup queue on exit
	defer func() {
		fmt.Println("\n🧹 Cleaning up ephemeral queue...")
		ec.client.DeleteWorkerQueue(queueID)
	}()

	// Start local worker
	fmt.Println("🚀 Starting local worker...")
	workerOpts := &WorkerStartOptions{
		QueueID:             queueID,
		DeploymentType:      "local",
		DaemonMode:          false,
		SingleExecutionMode: true, // Exit gracefully after completing the task
		cfg:                 ec.cfg,
	}

	// Start worker in background goroutine
	workerCtx, cancelWorker := context.WithCancel(ctx)
	defer cancelWorker()

	workerErrChan := make(chan error, 1)
	go func() {
		err := workerOpts.Run(workerCtx)
		if err != nil && err != context.Canceled {
			workerErrChan <- err
		}
	}()

	// Wait for worker to be ready
	fmt.Println("⏳ Waiting for worker to be ready...")
	fmt.Println("   (First run may take 1-2 minutes to install dependencies)")
	fmt.Println()

	startTime := time.Now()
	// Increased timeout to allow for dependency installation on first run
	// Typical first run: 40-120 seconds (venv + pip + deps + registration)
	// Typical cached run: 5-10 seconds
	if err := ec.waitForWorkerReady(ctx, queueID, 180*time.Second); err != nil {
		elapsed := time.Since(startTime)
		return fmt.Errorf("worker did not become ready after %.0fs: %w", elapsed.Seconds(), err)
	}

	elapsed := time.Since(startTime)
	fmt.Printf("✓ Worker ready (%.0fs)\n\n", elapsed.Seconds())

	// Submit execution and stream output
	fmt.Println(style.CreateBanner("Executing Task", "🚀"))
	fmt.Println()

	_, err = ec.submitAndStreamExecution(ctx, plan, queueID)

	// Give worker time to exit gracefully in single execution mode
	// The Python worker will detect task completion and shutdown on its own
	workerExited := make(chan struct{})
	go func() {
		select {
		case <-workerErrChan:
			close(workerExited)
		case <-time.After(10 * time.Second):
			// Worker didn't exit gracefully within timeout
			close(workerExited)
		}
	}()

	// Wait for worker to exit or timeout
	<-workerExited

	// Check for worker errors
	select {
	case workerErr := <-workerErrChan:
		if err == nil && workerErr != nil && workerErr != context.Canceled {
			err = workerErr
		}
	default:
	}

	return err
}

// createEphemeralQueue creates a temporary queue with auto-cleanup
func (ec *ExecCommand) createEphemeralQueue(ctx context.Context, environmentID string) (*entities.WorkerQueue, error) {
	trueVal := true
	ttl := 300 // 5 minutes

	req := &entities.WorkerQueueCreateRequest{
		Name:                    fmt.Sprintf("local-exec-%s", time.Now().Format("20060102-150405")),
		EnvironmentID:           environmentID,
		Ephemeral:               &trueVal,
		SingleExecutionMode:     &trueVal,
		AutoCleanupAfterSeconds: &ttl,
	}

	queue, err := ec.client.CreateWorkerQueue(environmentID, req)
	if err != nil {
		return nil, fmt.Errorf("failed to create queue: %w", err)
	}

	return queue, nil
}

// waitForWorkerReady polls until worker appears in queue
func (ec *ExecCommand) waitForWorkerReady(ctx context.Context, queueID string, timeout time.Duration) error {
	ctx, cancel := context.WithTimeout(ctx, timeout)
	defer cancel()

	ticker := time.NewTicker(2 * time.Second) // Less frequent polling
	defer ticker.Stop()

	startTime := time.Now()
	lastMessageTime := startTime
	messageInterval := 10 * time.Second // Show progress every 10 seconds

	for {
		select {
		case <-ctx.Done():
			elapsed := time.Since(startTime)
			return fmt.Errorf("timeout waiting for worker after %.0fs (this may be due to slow dependency installation - try running again as dependencies will be cached)", elapsed.Seconds())

		case <-ticker.C:
			workers, err := ec.client.ListQueueWorkers(queueID)
			if err == nil && len(workers) > 0 {
				return nil
			}

			// Show progress message periodically
			elapsed := time.Since(startTime)
			if time.Since(lastMessageTime) >= messageInterval {
				fmt.Printf("   Still waiting... (%.0fs elapsed)\n", elapsed.Seconds())
				if elapsed > 30*time.Second && elapsed < 35*time.Second {
					fmt.Println("   Tip: First run may take longer while dependencies are installed")
				}
				lastMessageTime = time.Now()
			}
		}
	}
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

// resolveQueueIDs resolves queue IDs from either --queue or --queue-name flags
func (ec *ExecCommand) resolveQueueIDs(ctx context.Context) ([]string, error) {
	// Priority: IDs > Names
	if len(ec.QueueIDs) > 0 {
		return ec.validateQueueIDs(ctx, ec.QueueIDs)
	}

	if len(ec.QueueNames) > 0 {
		return ec.resolveQueueNames(ctx, ec.QueueNames)
	}

	return nil, fmt.Errorf("no queues specified")
}

// validateQueueIDs validates that all queue IDs exist and are accessible
func (ec *ExecCommand) validateQueueIDs(ctx context.Context, ids []string) ([]string, error) {
	for _, id := range ids {
		_, err := ec.client.GetWorkerQueue(id)
		if err != nil {
			return nil, fmt.Errorf("invalid queue %s: %w", id, err)
		}
	}
	return ids, nil
}

// resolveQueueNames converts queue names to IDs
func (ec *ExecCommand) resolveQueueNames(ctx context.Context, names []string) ([]string, error) {
	// Fetch all queues
	queues, err := ec.client.ListWorkerQueues()
	if err != nil {
		return nil, fmt.Errorf("failed to list worker queues: %w", err)
	}

	// Build name->ID map
	nameMap := make(map[string]string)
	for _, q := range queues {
		nameMap[q.Name] = q.ID
	}

	// Resolve names to IDs
	var ids []string
	for _, name := range names {
		id, ok := nameMap[name]
		if !ok {
			return nil, fmt.Errorf("queue not found: %s", name)
		}
		ids = append(ids, id)
	}

	return ids, nil
}

// QueueExecutionResult tracks the result of execution on a single queue
type QueueExecutionResult struct {
	QueueID   string
	QueueName string
	Success   bool
	Duration  time.Duration
	Error     error
}

// executeMultiQueue executes plan across multiple queues in parallel
func (ec *ExecCommand) executeMultiQueue(ctx context.Context, plan *kubiya.PlanResponse, queueIDs []string) error {
	fmt.Println()
	fmt.Println(style.CreateBanner(fmt.Sprintf("Multi-Queue Execution (%d queues)", len(queueIDs)), "🚀"))
	fmt.Println()

	// Fetch queue metadata for display
	queueMap := make(map[string]string) // id -> name
	for _, id := range queueIDs {
		queue, err := ec.client.GetWorkerQueue(id)
		if err != nil {
			queueMap[id] = id // Fallback to ID
		} else {
			queueMap[id] = queue.Name
		}
		fmt.Printf("  • %s\n", queueMap[id])
	}
	fmt.Println()

	// Execute in parallel goroutines
	var wg sync.WaitGroup
	resultChan := make(chan *QueueExecutionResult, len(queueIDs))

	for _, queueID := range queueIDs {
		wg.Add(1)
		go func(qid string) {
			defer wg.Done()

			result := &QueueExecutionResult{
				QueueID:   qid,
				QueueName: queueMap[qid],
			}

			start := time.Now()
			err := ec.executeSingleQueue(ctx, plan, qid)
			result.Duration = time.Since(start)
			result.Success = (err == nil)
			result.Error = err

			resultChan <- result
		}(queueID)
	}

	wg.Wait()
	close(resultChan)

	// Display summary
	fmt.Println()
	fmt.Println(style.CreateBanner("Execution Summary", "📊"))
	fmt.Println()

	successCount := 0
	var results []*QueueExecutionResult
	for result := range resultChan {
		results = append(results, result)
		if result.Success {
			successCount++
		}
	}

	// Sort results by duration for display
	sort.Slice(results, func(i, j int) bool {
		return results[i].Duration < results[j].Duration
	})

	for _, result := range results {
		if result.Success {
			fmt.Printf("  ✓ %s: completed (%.1fs)\n", result.QueueName, result.Duration.Seconds())
		} else {
			fmt.Printf("  ✗ %s: failed - %v\n", result.QueueName, result.Error)
		}
	}

	fmt.Println()
	fmt.Printf("Result: %d/%d succeeded\n", successCount, len(queueIDs))

	if successCount < len(queueIDs) {
		return fmt.Errorf("%d executions failed", len(queueIDs)-successCount)
	}

	return nil
}

// executeSingleQueue executes plan on a single queue
func (ec *ExecCommand) executeSingleQueue(ctx context.Context, plan *kubiya.PlanResponse, queueID string) error {
	// Submit and stream execution
	_, err := ec.submitAndStreamExecution(ctx, plan, queueID)
	return err
}

