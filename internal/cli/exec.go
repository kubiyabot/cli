package cli

import (
	"bufio"
	"context"
	"fmt"
	"os"
	"os/signal"
	"sort"
	"strings"
	"sync"
	"syscall"
	"time"

	"github.com/kubiyabot/cli/internal/config"
	"github.com/kubiyabot/cli/internal/controlplane"
	"github.com/kubiyabot/cli/internal/controlplane/entities"
	"github.com/kubiyabot/cli/internal/kubiya"
	"github.com/kubiyabot/cli/internal/output"
	"github.com/kubiyabot/cli/internal/streaming"
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

		// Streaming flags
		noStream     bool   // Disable live event streaming (streaming is enabled by default)
		streamFormat string // Output format: auto, text, json
		verbose      bool   // Show detailed tool inputs/outputs

		// Output control flags (enhanced streaming UX)
		outputLines int  // Maximum lines per tool output
		fullOutput  bool // Show complete outputs without truncation
		compact     bool // Minimal output mode (tool names + status only)

		// New execution mode flags
		local            bool
		queueIDs         []string
		queueNames       []string
		environment      string
		parentExecution  string
		packageSource    string // For local mode: specify worker package source
		localWheel       string // For local mode: path to local wheel file
		cwd              string // Working directory for execution
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

  # Streaming is enabled by default - see live tool calls and agent reasoning
  kubiya exec "Deploy my app"                       # Formatted text output with tool inputs/outputs
  kubiya exec "Deploy my app" --output-lines=5      # Show max 5 lines per tool output
  kubiya exec "Deploy my app" --full-output         # Show complete tool outputs
  kubiya exec "Deploy my app" --compact             # Minimal output for CI
  kubiya exec "Deploy my app" --verbose             # Include even more detail
  kubiya exec "Deploy my app" --stream-format=json  # NDJSON for parsing
  kubiya exec "Deploy my app" --no-stream           # Disable streaming (legacy mode)

  # CI/CD usage (NDJSON output auto-detected)
  kubiya exec "Deploy to staging" --yes 2>&1 | jq 'select(.type == "tool_completed")'

  # Local execution with ephemeral worker
  kubiya exec "task" --local
  kubiya exec "task" --local --environment <env-id>
  kubiya exec "task" --local --package-source=0.5.0
  kubiya exec "task" --local --package-source=kubiyabot/control-plane-api@feature-branch
  kubiya exec "task" --local --local-wheel=/path/to/wheel.whl

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

			// Create progress manager for UI
			progressManager := output.NewProgressManager()

			executor := &ExecCommand{
				cfg:             cfg,
				client:          client,
				autoApprove:     autoApprove,
				outputFormat:    outputFormat,
				savePlanPath:    savePlanPath,
				nonInteractive:  nonInteractive,
				priority:        priority,
				progressManager: progressManager,
				NoStream:        noStream,
				StreamFormat:    streamFormat,
				Verbose:         verbose,
				OutputLines:     outputLines,
				FullOutput:      fullOutput,
				Compact:         compact,
				Local:           local,
				QueueIDs:        queueIDs,
				QueueNames:      queueNames,
				Environment:     environment,
				ParentExecution: parentExecution,
				PackageSource:   packageSource,
				LocalWheel:      localWheel,
				Cwd:             cwd,
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
	cmd.Flags().StringVar(&planFile, "plan-file", "", "Execute from plan file (local path, URL, or GitHub: user/repo//path)")
	cmd.Flags().BoolVarP(&autoApprove, "yes", "y", false, "Auto-approve plan without confirmation")
	cmd.Flags().StringVarP(&outputFormat, "output", "o", "text", "Output format: text, json, yaml")
	cmd.Flags().StringVar(&savePlanPath, "save-plan", "", "Custom path to save plan (default: ~/.kubiya/plans/<plan-id>.json)")
	cmd.Flags().BoolVar(&nonInteractive, "non-interactive", false, "Non-interactive mode (skip all prompts)")
	cmd.Flags().StringVar(&priority, "priority", "medium", "Task priority: low, medium, high, critical")

	// Streaming flags (streaming is enabled by default)
	cmd.Flags().BoolVar(&noStream, "no-stream", false, "Disable live event streaming (streaming is enabled by default)")
	cmd.Flags().StringVar(&streamFormat, "stream-format", "auto", "Stream output format: auto, text, json (default: text for all environments)")
	cmd.Flags().BoolVarP(&verbose, "verbose", "v", false, "Show even more detail (50 lines per output instead of 10)")

	// Output control flags (enhanced streaming UX)
	cmd.Flags().IntVar(&outputLines, "output-lines", 10, "Maximum lines to show per tool output (0 for default)")
	cmd.Flags().BoolVar(&fullOutput, "full-output", false, "Show complete tool outputs without truncation")
	cmd.Flags().BoolVar(&compact, "compact", false, "Minimal output: tool names and status only (useful for CI)")

	// Execution mode flags
	cmd.Flags().BoolVar(&local, "local", false, "Run execution locally with ephemeral worker (uses fast planning)")
	cmd.Flags().StringSliceVar(&queueIDs, "queue", nil, "Worker queue ID(s) - comma-separated for parallel execution")
	cmd.Flags().StringSliceVar(&queueNames, "queue-name", nil, "Worker queue name(s) - alternative to IDs")
	cmd.Flags().StringVar(&environment, "environment", "", "Environment ID for execution (required for --local, auto-detected otherwise)")
	cmd.Flags().StringVar(&parentExecution, "parent-execution", "", "Parent execution ID for conversation continuation")
	cmd.Flags().StringVar(&packageSource, "package-source", "", "Worker package source for local mode: PyPI version, local file, git URL, or GitHub shorthand (owner/repo@ref)")
	cmd.Flags().StringVar(&localWheel, "local-wheel", "", "Path to local wheel file for local mode (for development/testing)")
	cmd.Flags().StringVar(&cwd, "cwd", "", "Working directory for execution (overrides default workspace)")

	return cmd
}

// ExecCommand handles execution with auto-planning
type ExecCommand struct {
	cfg             *config.Config
	client          *controlplane.Client
	autoApprove     bool
	outputFormat    string
	savePlanPath    string
	nonInteractive  bool
	priority        string
	currentPlan     *kubiya.PlanResponse
	progressManager *output.ProgressManager

	// Streaming options (streaming is enabled by default)
	NoStream     bool   // --no-stream: Disable live event streaming
	StreamFormat string // --stream-format: Output format (auto, text, json)
	Verbose      bool   // --verbose: Show detailed tool inputs/outputs

	// Output control options (enhanced streaming UX)
	OutputLines int  // --output-lines: Maximum lines per tool output
	FullOutput  bool // --full-output: Show complete outputs
	Compact     bool // --compact: Minimal output mode

	// New execution mode flags
	Local           bool     // --local: Run with ephemeral local worker (uses fast planning)
	QueueIDs        []string // --queue: Explicit queue selection (comma-separated)
	QueueNames      []string // --queue-name: Select by name instead of ID
	Environment     string   // --environment: Environment ID for execution
	ParentExecution string   // --parent-execution: Parent execution ID for conversation continuation
	PackageSource   string   // --package-source: Worker package source for local mode
	LocalWheel      string   // --local-wheel: Path to local wheel file for local mode
	Cwd             string   // --cwd: Working directory for execution
}

// ExecuteWithPlanning runs the full planning workflow
func (ec *ExecCommand) ExecuteWithPlanning(ctx context.Context, prompt string) error {
	// Show task header prominently at the start
	fmt.Println()
	fmt.Println(style.CreateTaskHeader(prompt))
	fmt.Println()

	// Show execution mode info cleanly
	if ec.Local {
		fmt.Println("💻 Local Execution Mode")
		fmt.Println("   • Running with ephemeral local worker")
		fmt.Println("   • Using fast planning mode")
		fmt.Println()
	}

	// Fetch resources with spinner
	var resources *PlanningResources
	var err error
	if !ec.nonInteractive {
		spinner := ec.progressManager.Spinner("Discovering available resources")
		spinner.Start()
		resources, err = NewResourceFetcher(ec.client).FetchAllResources(ctx)
		spinner.Stop()
		if err != nil {
			ec.progressManager.Error(fmt.Sprintf("Failed to fetch resources: %v", err))
			return fmt.Errorf("failed to fetch resources: %w", err)
		}
		ec.progressManager.Success(fmt.Sprintf("Found %d agents, %d teams, %d environments",
			len(resources.Agents), len(resources.Teams), len(resources.Environments)))
		fmt.Println()
	} else {
		// Non-interactive mode
		resources, err = NewResourceFetcher(ec.client).FetchAllResources(ctx)
		if err != nil {
			return fmt.Errorf("failed to fetch resources: %w", err)
		}
	}

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
		plan, err = ec.streamPlanGeneration(ctx, plannerClient, planReq, resources)
	} else {
		plan, err = plannerClient.CreatePlan(ctx, planReq)
	}

	if err != nil {
		return fmt.Errorf("failed to create plan: %w", err)
	}

	ec.currentPlan = plan

	// Debug: Log plan details in local mode
	if ec.Local && plan != nil {
		fmt.Printf("\n[DEBUG] Planner recommendation:\n")
		fmt.Printf("  Entity Type: %s\n", plan.RecommendedExecution.EntityType)
		fmt.Printf("  Entity ID:   %s\n", plan.RecommendedExecution.EntityID)
		fmt.Printf("  Entity Name: %s\n", plan.RecommendedExecution.EntityName)
	}

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
	// Displayer should be non-interactive if either --yes or --non-interactive is specified
	displayer := NewPlanDisplayer(plan, ec.outputFormat, !ec.nonInteractive && !ec.autoApprove)

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

// streamPlanGeneration streams plan with progress updates using bubbletea TUI
func (ec *ExecCommand) streamPlanGeneration(ctx context.Context, client *kubiya.PlannerClient, req *kubiya.PlanRequest, resources *PlanningResources) (*kubiya.PlanResponse, error) {
	eventChan, errChan := client.StreamPlanProgress(ctx, req)

	// Run bubbletea TUI for real-time progress display
	plan, err := runPlanProgressTUI(ctx, eventChan, errChan, resources)
	if err != nil {
		return nil, err
	}

	return plan, nil
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
	fmt.Println(style.CreateDivider(80))
	fmt.Println()
	fmt.Printf("🚀 %s\n", style.BoldStyle.Render("Executing Plan"))
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

	// Debug: Log what we're submitting
	fmt.Printf("\n[DEBUG] Submitting to: /api/v1/%ss/%s/execute\n",
		rec.EntityType, rec.EntityID)

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

	// Set execution environment if cwd is provided
	var execEnv *entities.ExecutionEnvironmentOverride
	if ec.Cwd != "" {
		execEnv = &entities.ExecutionEnvironmentOverride{
			WorkingDir: ec.Cwd,
		}
	}

	if rec.EntityType == "agent" {
		req := &entities.ExecuteAgentRequest{
			Prompt:               plan.Summary,
			WorkerQueueID:        queuePtr,
			ParentExecutionID:    parentExecPtr,
			Stream:               &streamFlag,
			ExecutionEnvironment: execEnv,
		}
		execution, err = ec.client.ExecuteAgentV2(rec.EntityID, req)
	} else {
		req := &entities.ExecuteTeamRequest{
			Prompt:               plan.Summary,
			WorkerQueueID:        queuePtr,
			ParentExecutionID:    parentExecPtr,
			Stream:               &streamFlag,
			ExecutionEnvironment: execEnv,
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
	// Get environment with smart fallback logic:
	// 1. CLI --environment flag (highest priority)
	// 2. Planner's recommendation (if available)
	// 3. First available environment (auto-fetch fallback)
	var envID string
	var envName string

	if ec.Environment != "" {
		// Priority 1: User explicitly specified environment via --environment flag
		envID = ec.Environment
		fmt.Printf("📍 Using environment from CLI flag: %s\n", envID)
	} else if plan.RecommendedExecution.RecommendedEnvironmentID != nil {
		// Priority 2: Use planner's recommendation
		envID = *plan.RecommendedExecution.RecommendedEnvironmentID
		if plan.RecommendedExecution.RecommendedEnvironmentName != nil {
			envName = *plan.RecommendedExecution.RecommendedEnvironmentName
		}
		fmt.Printf("📍 Using environment from planner: %s", envID)
		if envName != "" {
			fmt.Printf(" (%s)", envName)
		}
		fmt.Println()
	} else {
		// Priority 3: Fallback - fetch first available environment
		fmt.Println("⚠️  No environment specified, fetching first available environment...")
		environments, err := ec.client.ListEnvironments()
		if err != nil {
			return fmt.Errorf("no environment specified and failed to list environments: %w", err)
		}
		if len(environments) == 0 {
			return fmt.Errorf("no environments available in organization. Please create an environment first")
		}

		firstEnv := environments[0]
		envID = firstEnv.ID
		envName = firstEnv.Name
		fmt.Printf("📍 Auto-selected first available environment: %s (%s)\n", envID, envName)
	}

	// Section divider
	fmt.Println()
	fmt.Println(style.CreateDivider(80))
	fmt.Println()

	// Create ephemeral queue
	fmt.Println("📦 Creating ephemeral worker queue...")
	queue, err := ec.createEphemeralQueue(ctx, envID)
	if err != nil {
		return fmt.Errorf("failed to create ephemeral queue: %w", err)
	}
	queueID := queue.ID
	fmt.Printf("✓ Queue created: %s\n", queueID)
	fmt.Println()

	// Track cleanup state to prevent double cleanup
	cleanupDone := false
	var cleanupMutex sync.Mutex

	// Cleanup function - ensures queue is deleted
	cleanup := func() {
		cleanupMutex.Lock()
		defer cleanupMutex.Unlock()

		if cleanupDone {
			return // Already cleaned up
		}
		cleanupDone = true

		// Delete ephemeral queue with retry logic
		// Worker may need time to fully unregister from queue
		fmt.Println("🧹 Cleaning up ephemeral queue...")

		maxRetries := 5
		retryDelay := 2 * time.Second

		for i := 0; i < maxRetries; i++ {
			// Check if there are still workers registered
			workers, err := ec.client.ListQueueWorkers(queueID)
			if err == nil && len(workers) > 0 {
				if i == 0 {
					fmt.Println("   Waiting for worker to unregister...")
				} else {
					fmt.Printf("   Still waiting... (%d workers active)\n", len(workers))
				}
				time.Sleep(retryDelay)
				continue
			}

			// Try to delete queue
			if err := ec.client.DeleteWorkerQueue(queueID); err != nil {
				if i < maxRetries-1 {
					// Retry
					time.Sleep(retryDelay)
					continue
				}
				// Final attempt failed
				fmt.Printf("⚠️  Warning: Failed to delete queue after %d attempts: %v\n", maxRetries, err)
				return
			}

			// Success!
			fmt.Println("✓ Queue cleaned up")
			return
		}

		fmt.Println("⚠️  Warning: Queue cleanup timed out (will auto-cleanup after TTL)")
	}

	// Ensure cleanup runs on exit (normal, error, or panic)
	defer cleanup()

	// Start local worker
	fmt.Println("🚀 Starting local worker...")
	// Check if streaming is enabled (for quiet mode)
	streamingOpts := ec.GetStreamingOptions()

	workerOpts := &WorkerStartOptions{
		QueueID:             queueID,
		DeploymentType:      "local",
		DaemonMode:          false,
		SingleExecutionMode: true, // Exit gracefully after completing the task
		PackageSource:       ec.PackageSource,
		LocalWheel:          ec.LocalWheel,
		QuietMode:           streamingOpts.Enabled, // Suppress worker output when streaming
		cfg:                 ec.cfg,
	}

	// Setup signal handling for graceful shutdown
	sigChan := make(chan os.Signal, 1)
	signal.Notify(sigChan, os.Interrupt, syscall.SIGTERM)

	// Shutdown channel to coordinate graceful exit
	shutdownChan := make(chan string, 1)

	// Create cancellable context for worker
	workerCtx, cancelWorker := context.WithCancel(ctx)
	defer cancelWorker()

	// Start worker in background goroutine
	workerErrChan := make(chan error, 1)
	go func() {
		err := workerOpts.Run(workerCtx)
		// Always signal worker exit (nil for success, error for failure)
		// Don't signal if context was canceled (cleanup already in progress)
		if err != context.Canceled {
			workerErrChan <- err
		}
	}()

	// Handle signals for graceful shutdown
	// Note: The worker process is managed by workerOpts.Run() which handles its own signal processing
	// We just need to cancel the context and let the worker handle cleanup
	go func() {
		<-sigChan
		fmt.Println("\n\n⚠️  Interrupt received, shutting down gracefully...")
		fmt.Println("   Stopping worker process...")
		fmt.Println("   (Press Ctrl+C again to force shutdown)")
		cancelWorker() // Cancel worker context - this will trigger cleanup in runLocalForeground

		// Wait a moment for worker to start cleanup
		time.Sleep(500 * time.Millisecond)
		shutdownChan <- "interrupted"

		// Listen for second interrupt to force kill
		// On second interrupt, we exit immediately without waiting
		<-sigChan
		fmt.Println("\n\n⚠️  Force shutdown requested...")
		shutdownChan <- "force-killed"
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

	// Validate and fix agent selection before submission
	if err := ec.validateAndFixAgentSelection(ctx, plan); err != nil {
		return fmt.Errorf("failed to validate agent selection: %w", err)
	}

	// Submit execution and stream output
	fmt.Println()
	fmt.Println(style.CreateDivider(80))
	fmt.Println()
	fmt.Printf("🚀 %s\n", style.BoldStyle.Render("Executing Task"))
	fmt.Println()

	_, err = ec.submitAndStreamExecution(ctx, plan, queueID)

	// Give worker time to exit gracefully in single execution mode
	// The Python worker monitors execution status (checks every 3s) and shuts down automatically
	// Expected shutdown time: 2-5 seconds after execution completes
	fmt.Println()
	fmt.Println("⏳ Waiting for worker to shut down...")

	workerExited := make(chan struct{})
	go func() {
		select {
		case <-workerErrChan:
			close(workerExited)
		case <-time.After(20 * time.Second):
			// Worker should exit within 15-20 seconds after execution completes:
			// - 9s for 3 consecutive completion checks (3 checks × 3s)
			// - 5s grace period for SSE stream completion
			// - Buffer for final cleanup and process exit
			// If it doesn't, proceed with cleanup anyway
			close(workerExited)
		}
	}()

	// Wait for worker to exit, timeout, or interrupt
	select {
	case <-workerExited:
		fmt.Println("✓ Worker shut down")
		// Brief pause to allow worker to fully unregister from queue
		// Reduced from 3s to 1s since Python worker handles cleanup
		time.Sleep(1 * time.Second)
	case reason := <-shutdownChan:
		if reason == "force-killed" {
			// Force kill was triggered - give worker process a brief moment to be killed
			// then exit immediately without waiting for full cleanup
			fmt.Println("   ⚠️  Forcing immediate shutdown...")
			forceTimer := time.NewTimer(1 * time.Second)
			select {
			case <-workerErrChan:
				forceTimer.Stop()
				fmt.Println("   ✓ Worker process terminated")
			case <-forceTimer.C:
				fmt.Println("   ⚠️  Exiting without waiting for worker cleanup")
			}
			return fmt.Errorf("execution force terminated")
		}

		fmt.Printf("⚠️  Shutdown triggered: %s\n", reason)

		// Wait for worker goroutine to complete cleanup (with shorter timeout)
		fmt.Println("   Waiting for worker cleanup to complete...")
		workerCleanupTimer := time.NewTimer(5 * time.Second)
		select {
		case <-workerErrChan:
			// Worker goroutine completed
			workerCleanupTimer.Stop()
			fmt.Println("   ✓ Worker cleanup completed")
		case <-workerCleanupTimer.C:
			// Timeout - worker didn't clean up in time
			fmt.Println("   ⚠️  Worker cleanup timed out (5s), proceeding with queue cleanup")
		}

		return fmt.Errorf("execution interrupted")
	}

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

// validateAndFixAgentSelection validates the planner's agent selection and fixes if invalid
func (ec *ExecCommand) validateAndFixAgentSelection(ctx context.Context, plan *kubiya.PlanResponse) error {
	rec := &plan.RecommendedExecution

	// Check if the selected agent/team exists
	var exists bool
	var err error

	if rec.EntityType == "agent" {
		// Try to fetch the specific agent
		_, err = ec.client.GetAgent(rec.EntityID)
		exists = (err == nil)

		if !exists {
			fmt.Printf("⚠️  Planner selected agent '%s' (ID: %s) which doesn't exist in your organization\n",
				rec.EntityName, rec.EntityID)
			fmt.Println("   Looking for an alternative agent...")

			// List all available agents
			agents, listErr := ec.client.ListAgents()
			if listErr != nil {
				return fmt.Errorf("agent '%s' not found and failed to list alternatives: %w", rec.EntityID, listErr)
			}

			if len(agents) == 0 {
				return fmt.Errorf("agent '%s' not found and no agents available in organization", rec.EntityID)
			}

			// Try to find a matching agent by name
			var matchedAgent *entities.Agent
			for _, agent := range agents {
				if agent.Name == rec.EntityName {
					matchedAgent = agent
					break
				}
			}

			// If no name match, use first available agent
			if matchedAgent == nil {
				matchedAgent = agents[0]
				fmt.Printf("   No agent named '%s' found, using '%s' instead\n",
					rec.EntityName, matchedAgent.Name)
			} else {
				fmt.Printf("   Found matching agent '%s' in your organization\n", matchedAgent.Name)
			}

			// Update the plan with the correct agent
			rec.EntityID = matchedAgent.ID
			rec.EntityName = matchedAgent.Name
			fmt.Printf("   ✓ Using agent: %s (ID: %s)\n\n", matchedAgent.Name, matchedAgent.ID)
		}

	} else if rec.EntityType == "team" {
		// Similar logic for teams
		_, err = ec.client.GetTeam(rec.EntityID)
		exists = (err == nil)

		if !exists {
			fmt.Printf("⚠️  Planner selected team '%s' (ID: %s) which doesn't exist in your organization\n",
				rec.EntityName, rec.EntityID)
			fmt.Println("   Looking for an alternative team...")

			teams, listErr := ec.client.ListTeams()
			if listErr != nil {
				return fmt.Errorf("team '%s' not found and failed to list alternatives: %w", rec.EntityID, listErr)
			}

			if len(teams) == 0 {
				return fmt.Errorf("team '%s' not found and no teams available in organization", rec.EntityID)
			}

			// Try to find a matching team by name
			var matchedTeam *entities.Team
			for _, team := range teams {
				if team.Name == rec.EntityName {
					matchedTeam = team
					break
				}
			}

			// If no name match, use first available team
			if matchedTeam == nil {
				matchedTeam = teams[0]
				fmt.Printf("   No team named '%s' found, using '%s' instead\n",
					rec.EntityName, matchedTeam.Name)
			} else {
				fmt.Printf("   Found matching team '%s' in your organization\n", matchedTeam.Name)
			}

			// Update the plan with the correct team
			rec.EntityID = matchedTeam.ID
			rec.EntityName = matchedTeam.Name
			fmt.Printf("   ✓ Using team: %s (ID: %s)\n\n", matchedTeam.Name, matchedTeam.ID)
		}
	}

	return nil
}

// createEphemeralQueue creates a temporary queue with auto-cleanup
func (ec *ExecCommand) createEphemeralQueue(ctx context.Context, environmentID string) (*entities.WorkerQueue, error) {
	trueVal := true
	ttl := 3600 // 1 hour - queue auto-cleanup after execution completes

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
	// Check if enhanced streaming is enabled (--stream flag or CI auto-detection)
	opts := ec.GetStreamingOptions()
	if opts.Enabled {
		return ec.streamExecutionWithPipeline(ctx, executionID, opts)
	}

	// Default streaming behavior (backward compatible)
	return ec.streamExecutionDefault(ctx, executionID)
}

// streamExecutionDefault is the original streaming implementation
func (ec *ExecCommand) streamExecutionDefault(ctx context.Context, executionID string) error {
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
			case entities.StreamEventTypeChunk:
				// Stream content in real-time
				fmt.Print(style.OutputStyle.Render(event.Content))
				fullResponse.WriteString(event.Content)
			case entities.StreamEventTypeError:
				// Show error
				fmt.Println()
				fmt.Println()
				fmt.Println(style.CreateErrorBox(event.Content))
				return fmt.Errorf("execution error: %s", event.Content)
			case entities.StreamEventTypeComplete:
				// Completion
				fmt.Println()
				fmt.Println()
				fmt.Println(style.CreateSuccessBox("Execution completed successfully"))
				return nil
			case entities.StreamEventTypeStatus:
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

// streamExecutionWithEnhancedPipeline streams execution using the new streaming pipeline
// This provides tool call visibility, NDJSON output, and CI-friendly formatting
func (ec *ExecCommand) streamExecutionWithEnhancedPipeline(ctx context.Context, executionID string, opts StreamingOptions) error {
	executor := NewStreamingExecutor(opts)
	defer executor.Close()

	// Print streaming mode info in text mode
	if opts.Format == streaming.StreamFormatText && !output.IsCI() {
		printStreamingInfo(opts)
	}

	// Send connected event
	executor.ProcessStreamEvent(streaming.NewConnectedEvent(executionID))

	// Stream execution output from the control plane
	eventChan, errChan := ec.client.StreamExecutionOutput(ctx, executionID)

	// Process events through the streaming executor
	err := executor.StreamExecution(ctx, eventChan, errChan)
	if err != nil {
		return err
	}

	// Send done event
	executor.ProcessStreamEvent(streaming.NewDoneEvent())

	// Flush any remaining output
	if flushErr := executor.Flush(); flushErr != nil {
		return flushErr
	}

	// Print completion message in text mode (JSON mode already has done event)
	if opts.Format == streaming.StreamFormatText && !output.IsCI() {
		fmt.Println()
		fmt.Println(style.CreateSuccessBox("Execution completed successfully"))
	}

	return nil
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
	// Displayer should be non-interactive if either --yes or --non-interactive is specified
	displayer := NewPlanDisplayer(savedPlan.Plan, ec.outputFormat, !ec.autoApprove && !ec.nonInteractive)

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

	// 6. Execute - route based on --local flag
	if ec.Local {
		return ec.executeLocal(ctx, savedPlan.Plan)
	}

	return ec.executeFromPlan(ctx, savedPlan.Plan, savedPlan)
}

// ExecuteDirect executes directly without planning
func (ec *ExecCommand) ExecuteDirect(ctx context.Context, entityType, entityID, prompt string) error {
	fmt.Println()
	fmt.Println(style.CreateTaskHeader(prompt))
	fmt.Println()
	fmt.Println(style.CreateDivider(80))
	fmt.Println()
	fmt.Printf("⚡ %s\n", style.BoldStyle.Render("Direct Execution"))
	fmt.Println()

	entityIcon := "🤖"
	if entityType == "team" {
		entityIcon = "👥"
	}
	fmt.Printf("%s Using %s: %s\n\n", entityIcon, entityType, style.HighlightStyle.Render(entityID))

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
	fmt.Println(style.CreateDivider(80))
	fmt.Println()
	fmt.Printf("🚀 %s (%d queues)\n", style.BoldStyle.Render("Multi-Queue Execution"), len(queueIDs))
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
	fmt.Println(style.CreateDivider(80))
	fmt.Println()
	fmt.Printf("📊 %s\n", style.BoldStyle.Render("Execution Summary"))
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

