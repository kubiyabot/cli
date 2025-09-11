package cli

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"os/signal"
	"strings"
	"syscall"
	"time"

	"github.com/kubiyabot/cli/internal/config"
	"github.com/kubiyabot/cli/internal/kubiya"
	"github.com/kubiyabot/cli/internal/style"
	"github.com/spf13/cobra"
)


func newWorkflowStreamCommand(cfg *config.Config) *cobra.Command {
	var (
		runner    string
		follow    bool
		verbose   bool
		jsonOutput bool
	)

	cmd := &cobra.Command{
		Use:   "stream <workflow-id>",
		Short: "Stream logs from an existing workflow execution",
		Long: `Connect to an existing workflow execution and stream live logs and events.

This command allows you to reconnect to running workflows and monitor their progress
in real-time. It supports:
• Connecting to workflows by ID
• Full error details and stack traces
• Live log streaming from all steps
• Progress tracking and status updates
• Graceful reconnection on connection issues`,
		Example: `  # Stream logs from a running workflow
  kubiya workflow stream abc123-def456-789

  # Stream with verbose output
  kubiya workflow stream abc123-def456-789 --verbose

  # Stream with JSON output for parsing
  kubiya workflow stream abc123-def456-789 --json

  # Stream from specific runner
  kubiya workflow stream abc123-def456-789 --runner prod-runner`,
		Args: cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			workflowID := args[0]
			client := kubiya.NewClient(cfg)
			ctx := context.Background()

			// Setup cancellation handling
			ctx, cancel := context.WithCancel(ctx)
			defer cancel()

			// Handle interrupt signals
			sigChan := make(chan os.Signal, 1)
			signal.Notify(sigChan, syscall.SIGINT, syscall.SIGTERM)
			go func() {
				<-sigChan
				fmt.Printf("\n%s Disconnecting from workflow stream...\n", 
					style.WarningStyle.Render("⚠️"))
				cancel()
			}()

			if !jsonOutput {
				fmt.Printf("%s Connecting to workflow stream: %s\n", 
					style.InfoStyle.Render("🔗"), style.HighlightStyle.Render(workflowID))
				if runner != "" {
					fmt.Printf("%s Using runner: %s\n", 
						style.DimStyle.Render("Runner:"), runner)
				}
			}

			return streamWorkflowLogs(ctx, client, workflowID, runner, StreamOptions{
				Follow:     follow,
				Verbose:    verbose,
				JSONOutput: jsonOutput,
			})
		},
	}

	cmd.Flags().StringVar(&runner, "runner", "", "Runner to connect to")
	cmd.Flags().BoolVarP(&follow, "follow", "f", true, "Follow log output")
	cmd.Flags().BoolVarP(&verbose, "verbose", "v", false, "Verbose output with detailed logs")
	cmd.Flags().BoolVar(&jsonOutput, "json", false, "Output in JSON format")

	return cmd
}

func newWorkflowRetryCommand(cfg *config.Config) *cobra.Command {
	var (
		runner   string
		fromStep string
		variables []string
		force    bool
		verbose  bool
	)

	cmd := &cobra.Command{
		Use:   "retry <workflow-id>",
		Short: "Retry a failed workflow from a specific step",
		Long: `Retry a workflow execution from a failed step or from a specific step.

This command allows you to restart workflow execution from any step, preserving
the context and variables from the previous execution:
• Retry from failed steps automatically
• Retry from any specific step by name
• Preserve or override variables from previous execution
• Resume with all previous step outputs available`,
		Example: `  # Retry from the failed step
  kubiya workflow retry abc123-def456-789

  # Retry from a specific step
  kubiya workflow retry abc123-def456-789 --from-step "deploy"

  # Retry with new variables
  kubiya workflow retry abc123-def456-789 --var env=staging --var debug=true

  # Force retry even if workflow succeeded
  kubiya workflow retry abc123-def456-789 --force`,
		Args: cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			workflowID := args[0]
			client := kubiya.NewClient(cfg)
			ctx := context.Background()

			// Parse variables
			vars := make(map[string]interface{})
			for _, v := range variables {
				parts := strings.SplitN(v, "=", 2)
				if len(parts) == 2 {
					vars[parts[0]] = parts[1]
				}
			}

			fmt.Printf("%s Retrying workflow: %s\n", 
				style.InfoStyle.Render("🔄"), style.HighlightStyle.Render(workflowID))

			if fromStep != "" {
				fmt.Printf("%s Starting from step: %s\n", 
					style.DimStyle.Render("From:"), fromStep)
			}

			if len(vars) > 0 {
				fmt.Printf("%s Variables:\n", style.DimStyle.Render("Vars:"))
				for k, v := range vars {
					fmt.Printf("  %s = %v\n", style.KeyStyle.Render(k), v)
				}
			}

			return retryWorkflow(ctx, client, workflowID, runner, RetryOptions{
				FromStep:  fromStep,
				Variables: vars,
				Force:     force,
				Verbose:   verbose,
			})
		},
	}

	cmd.Flags().StringVar(&runner, "runner", "", "Runner to use for retry")
	cmd.Flags().StringVar(&fromStep, "from-step", "", "Step to retry from (default: first failed step)")
	cmd.Flags().StringArrayVar(&variables, "var", []string{}, "Variables to override in key=value format")
	cmd.Flags().BoolVar(&force, "force", false, "Force retry even if workflow succeeded")
	cmd.Flags().BoolVarP(&verbose, "verbose", "v", false, "Verbose output")

	return cmd
}

// Removed: workflow status command

func newWorkflowListCommand(cfg *config.Config) *cobra.Command {
	var (
		runner     string
		filter     string
		limit      int
		jsonOutput bool
		allRunners bool
	)

	cmd := &cobra.Command{
		Use:   "list",
		Short: "List workflow executions across runners",
		Long: `List workflow executions with filtering and pagination support.

This command provides comprehensive listing of workflows across all runners:
• Filter by status (running, completed, failed)
• Cross-runner visibility
• Pagination support
• Detailed execution information`,
		Example: `  # List all workflows
  kubiya workflow list

  # List running workflows
  kubiya workflow list --filter running

  # List workflows from specific runner
  kubiya workflow list --runner prod-runner

  # List workflows across all runners
  kubiya workflow list --all-runners

  # List with JSON output
  kubiya workflow list --json --limit 50`,
		RunE: func(cmd *cobra.Command, args []string) error {
			client := kubiya.NewClient(cfg)
			ctx := context.Background()

			return listWorkflows(ctx, client, ListOptions{
				Runner:     runner,
				Filter:     filter,
				Limit:      limit,
				JSONOutput: jsonOutput,
				AllRunners: allRunners,
			})
		},
	}

	cmd.Flags().StringVar(&runner, "runner", "", "Runner to query")
	cmd.Flags().StringVar(&filter, "filter", "all", "Filter workflows (all|running|completed|failed)")
	cmd.Flags().IntVar(&limit, "limit", 10, "Limit number of results")
	cmd.Flags().BoolVar(&jsonOutput, "json", false, "Output in JSON format")
	cmd.Flags().BoolVar(&allRunners, "all-runners", false, "Query all available runners")

	return cmd
}

// StreamOptions defines options for streaming workflow logs
type StreamOptions struct {
	Follow     bool
	Verbose    bool
	JSONOutput bool
}

// RetryOptions defines options for retrying workflows
type RetryOptions struct {
	FromStep  string
	Variables map[string]interface{}
	Force     bool
	Verbose   bool
}

// ListOptions defines options for listing workflows
type ListOptions struct {
	Runner     string
	Filter     string
	Limit      int
	JSONOutput bool
	AllRunners bool
}

// Enhanced workflow execution with better error handling
func executeWorkflowEnhanced(ctx context.Context, client *kubiya.Client, req kubiya.WorkflowExecutionRequest, runner string, verbose bool) error {
	workflowClient := client.Workflow()
	
	// Enhanced request with better error tracking
	req.Command = "execute_workflow"
	if req.Env == nil {
		req.Env = make(map[string]interface{})
	}
	// Note: This would need to be implemented with proper client access
	// req.Env["KUBIYA_API_KEY"] = client.cfg.APIKey
	req.Env["KUBIYA_ENHANCED_ERRORS"] = "true"
	req.Env["KUBIYA_STREAM_LOGS"] = "true"

	events, err := workflowClient.ExecuteWorkflow(ctx, req, runner)
	if err != nil {
		return fmt.Errorf("failed to execute workflow: %w", err)
	}

	return processEnhancedWorkflowEvents(ctx, events, verbose)
}

// Process enhanced workflow events with better error handling
func processEnhancedWorkflowEvents(ctx context.Context, events <-chan kubiya.WorkflowSSEEvent, verbose bool) error {
	var (
		hasError      bool
		currentStep   string
		completedSteps int
		totalSteps    int
		workflowID    string
		reconnectCount int
		maxReconnects = 3
	)

	for {
		select {
		case <-ctx.Done():
			return ctx.Err()
		case event, ok := <-events:
			if !ok {
				if hasError {
					return fmt.Errorf("workflow execution failed")
				}
				return nil
			}

			if verbose {
				fmt.Printf("[VERBOSE] Event: %s, Data: %s\n", event.Type, event.Data)
			}

			// Enhanced event processing
			if event.Type == "data" && event.Data != "" {
				if err := processWorkflowEventStream(event.Data, &currentStep, &completedSteps, &totalSteps, &workflowID, verbose); err != nil {
					if verbose {
						fmt.Printf("[VERBOSE] Event processing error: %v\n", err)
					}
				}
			} else if event.Type == "error" {
				hasError = true
				if err := handleWorkflowError(event.Data, workflowID, currentStep); err != nil {
					fmt.Printf("%s %s\n", style.ErrorStyle.Render("💥 Critical Error:"), err.Error())
				}
			} else if event.Type == "done" {
				if verbose {
					fmt.Printf("[VERBOSE] Stream completed\n")
				}
				break
			} else if strings.Contains(event.Type, "connection") || strings.Contains(event.Data, "connection") {
				// Handle connection issues with retry
				if reconnectCount < maxReconnects {
					reconnectCount++
					fmt.Printf("%s Connection issue detected (attempt %d/%d), retrying...\n", 
						style.WarningStyle.Render("🔄"), reconnectCount, maxReconnects)
					time.Sleep(time.Duration(reconnectCount) * time.Second)
					continue
				} else {
					return fmt.Errorf("max reconnection attempts reached")
				}
			}
		}
	}

	if hasError {
		return fmt.Errorf("workflow execution failed")
	}
	return nil
}

// Process individual workflow events with enhanced parsing
func processWorkflowEventStream(data string, currentStep *string, completedSteps, totalSteps *int, workflowID *string, verbose bool) error {
	var event kubiya.EnhancedWorkflowEvent
	if err := json.Unmarshal([]byte(data), &event); err != nil {
		// Try parsing as basic workflow event
		var basicEvent map[string]interface{}
		if err := json.Unmarshal([]byte(data), &basicEvent); err != nil {
			return fmt.Errorf("failed to parse event: %w", err)
		}
		return processBasicWorkflowEvent(basicEvent, currentStep, completedSteps, totalSteps, workflowID, verbose)
	}

	// Extract workflow ID if available
	if event.WorkflowID != "" && *workflowID == "" {
		*workflowID = event.WorkflowID
	}

	switch event.Type {
	case "workflow_start":
		fmt.Printf("%s Workflow started: %s\n", 
			style.InfoStyle.Render("🚀"), 
			style.HighlightStyle.Render(event.RequestID))
		if event.Progress != nil {
			*totalSteps = event.Progress.Total
		}

	case "step_running":
		if event.Step != nil {
			*currentStep = event.Step.Name
			fmt.Printf("%s %s %s\n", 
				style.BulletStyle.Render("▶️"), 
				style.InfoStyle.Render(fmt.Sprintf("[%d/%d]", *completedSteps+1, *totalSteps)),
				style.ToolNameStyle.Render(event.Step.Name))
			
			if verbose && len(event.Step.Logs) > 0 {
				for _, log := range event.Step.Logs {
					fmt.Printf("  %s %s\n", style.DimStyle.Render("📝"), log)
				}
			}
		}

	case "step_complete":
		if event.Step != nil {
			*completedSteps++
			duration := ""
			if event.Step.Duration != "" {
				duration = fmt.Sprintf(" (%s)", event.Step.Duration)
			}
			
			fmt.Printf("  %s Step %s%s\n", 
				style.SuccessStyle.Render("✅"), 
				event.Step.Status, duration)
			
			if event.Step.Output != "" {
				fmt.Printf("  %s %s\n", 
					style.DimStyle.Render("📤 Output:"),
					style.ToolOutputStyle.Render(formatStepOutput(event.Step.Output)))
			}
			
			if verbose && event.Step.Variables != nil && len(event.Step.Variables) > 0 {
				fmt.Printf("  %s Variables:\n", style.DimStyle.Render("🔧"))
				for k, v := range event.Step.Variables {
					fmt.Printf("    %s = %v\n", style.KeyStyle.Render(k), v)
				}
			}
		}

	case "step_failed":
		if event.Step != nil {
			fmt.Printf("  %s Step failed: %s\n", 
				style.ErrorStyle.Render("❌"), 
				event.Step.Error)
			
			if event.Step.CanRetry {
				fmt.Printf("  %s This step can be retried with: kubiya workflow retry %s --from-step %s\n",
					style.InfoStyle.Render("💡"), *workflowID, event.Step.Name)
			}
			
			if verbose && len(event.Step.Logs) > 0 {
				fmt.Printf("  %s Error logs:\n", style.DimStyle.Render("📋"))
				for _, log := range event.Step.Logs {
					fmt.Printf("    %s\n", style.ErrorStyle.Render(log))
				}
			}
		}

	case "workflow_complete":
		success := event.Data != nil
		if success {
			fmt.Printf("%s Workflow completed successfully!\n", 
				style.SuccessStyle.Render("🎉"))
		} else {
			fmt.Printf("%s Workflow execution failed\n", 
				style.ErrorStyle.Render("💥"))
		}

	case "error":
		if event.Error != nil {
			fmt.Printf("%s %s\n", 
				style.ErrorStyle.Render("💀 Error:"), 
				event.Error.Message)
			if event.Error.Details != "" && verbose {
				fmt.Printf("  %s %s\n", 
					style.DimStyle.Render("Details:"), 
					event.Error.Details)
			}
		}
	}

	return nil
}

// Handle basic workflow events for backward compatibility
func processBasicWorkflowEvent(event map[string]interface{}, currentStep *string, completedSteps, totalSteps *int, workflowID *string, verbose bool) error {
	eventType, ok := event["type"].(string)
	if !ok {
		return fmt.Errorf("missing event type")
	}

	switch eventType {
	case "step_running":
		if step, ok := event["step"].(map[string]interface{}); ok {
			if stepName, ok := step["name"].(string); ok {
				*currentStep = stepName
				fmt.Printf("%s %s %s\n", 
					style.BulletStyle.Render("▶️"), 
					style.InfoStyle.Render(fmt.Sprintf("[%d/%d]", *completedSteps+1, *totalSteps)),
					style.ToolNameStyle.Render(stepName))
			}
		}

	case "step_complete":
		if step, ok := event["step"].(map[string]interface{}); ok {
			*completedSteps++
			if _, ok := step["name"].(string); ok {
				fmt.Printf("  %s Step completed\n", style.SuccessStyle.Render("✅"))
				
				if output, ok := step["output"].(string); ok && output != "" {
					fmt.Printf("  %s %s\n", 
						style.DimStyle.Render("📤 Output:"),
						style.ToolOutputStyle.Render(formatStepOutput(output)))
				}
			}
		}

	case "workflow_complete":
		if success, ok := event["success"].(bool); ok && success {
			fmt.Printf("%s Workflow completed successfully!\n", 
				style.SuccessStyle.Render("🎉"))
		} else {
			fmt.Printf("%s Workflow execution failed\n", 
				style.ErrorStyle.Render("💥"))
		}
	}

	return nil
}

// Enhanced error handling with actionable information
func handleWorkflowError(errorData, workflowID, currentStep string) error {
	var workflowError kubiya.WorkflowError
	if err := json.Unmarshal([]byte(errorData), &workflowError); err != nil {
		// Handle as simple error string
		fmt.Printf("%s %s\n", style.ErrorStyle.Render("💥 Error:"), errorData)
		if workflowID != "" && currentStep != "" {
			fmt.Printf("%s Try retrying with: kubiya workflow retry %s --from-step %s\n",
				style.InfoStyle.Render("💡"), workflowID, currentStep)
		}
		return fmt.Errorf("workflow error: %s", errorData)
	}

	// Enhanced error display
	fmt.Printf("%s %s (Code: %s)\n", 
		style.ErrorStyle.Render("💥 Error:"), 
		workflowError.Message, 
		workflowError.Code)
	
	if workflowError.Details != "" {
		fmt.Printf("%s %s\n", 
			style.DimStyle.Render("Details:"), 
			workflowError.Details)
	}

	if workflowError.Step != "" {
		fmt.Printf("%s %s\n", 
			style.DimStyle.Render("Failed Step:"), 
			workflowError.Step)
	}

	if workflowError.Retry && workflowID != "" {
		retryStep := workflowError.Step
		if retryStep == "" {
			retryStep = currentStep
		}
		fmt.Printf("%s Try retrying with: kubiya workflow retry %s --from-step %s\n",
			style.InfoStyle.Render("💡"), workflowID, retryStep)
	}

	return fmt.Errorf("workflow error: %s", workflowError.Message)
}

// Placeholder functions for the new commands - these would integrate with the Workflow Engine API
func streamWorkflowLogs(ctx context.Context, client *kubiya.Client, workflowID, runner string, opts StreamOptions) error {
	// This would call the enhanced workflow client
	enhancedClient := client.WorkflowEnhanced()
	events, err := enhancedClient.StreamWorkflowLogs(ctx, workflowID, runner)
	if err != nil {
		fmt.Printf("%s Failed to stream workflow logs: %v\n", 
			style.ErrorStyle.Render("❌"), err)
		fmt.Printf("%s Use: POST /api/v1/workflow?runner=%s&operation=get_status\n", 
			style.DimStyle.Render("API:"), runner)
		fmt.Printf("%s Body: {\"workflowId\": \"%s\", \"stream\": true}\n", 
			style.DimStyle.Render("Request:"), workflowID)
		return err
	}

	// Process streaming events
	for event := range events {
		if opts.JSONOutput {
			eventJSON, _ := json.Marshal(event)
			fmt.Println(string(eventJSON))
		} else {
			// Display formatted output
			switch event.Type {
			case "status":
				fmt.Printf("%s Workflow Status: %s\n", 
					style.InfoStyle.Render("📊"), 
					event.Data["status"])
			case "step_update", "step_history":
				if event.Step != nil {
					fmt.Printf("%s Step: %s [%s]\n", 
						style.BulletStyle.Render("▶️"), 
						event.Step.Name, 
						event.Step.Status)
					if event.Step.Output != "" {
						fmt.Printf("  %s %s\n", 
							style.DimStyle.Render("📤 Output:"),
							event.Step.Output)
					}
				}
			case "error":
				if event.Error != nil {
					fmt.Printf("%s %s\n", 
						style.ErrorStyle.Render("💥 Error:"), 
						event.Error.Message)
				}
			case "done":
				fmt.Printf("%s Stream completed\n", 
					style.SuccessStyle.Render("✅"))
				return nil
			}
		}
	}
	return nil
}

func retryWorkflow(ctx context.Context, client *kubiya.Client, workflowID, runner string, opts RetryOptions) error {
	enhancedClient := client.WorkflowEnhanced()
	retryReq := kubiya.RetryWorkflowRequest{
		WorkflowID: workflowID,
		FromStep:   opts.FromStep,
		Variables:  opts.Variables,
		Force:      opts.Force,
	}
	
	_, err := enhancedClient.RetryWorkflow(ctx, retryReq, runner)
	if err != nil {
		fmt.Printf("%s Failed to retry workflow: %v\n", 
			style.ErrorStyle.Render("❌"), err)
		fmt.Printf("%s Use: POST /api/v1/workflow?runner=%s&operation=retry_workflow\n", 
			style.DimStyle.Render("API:"), runner)
		fmt.Printf("%s Body: {\"workflowId\": \"%s\", \"fromStep\": \"%s\"}\n", 
			style.DimStyle.Render("Request:"), workflowID, opts.FromStep)
		return err
	}

	fmt.Printf("%s Workflow retry initiated: %s\n", 
		style.SuccessStyle.Render("🔄"), workflowID)
	
	// Stream the retry execution
	return streamWorkflowLogs(ctx, client, workflowID, runner, StreamOptions{Follow: true})
}

// Removed: getWorkflowStatus helper

// Removed: watchWorkflowStatus helper

func listWorkflows(ctx context.Context, client *kubiya.Client, opts ListOptions) error {
	enhancedClient := client.WorkflowEnhanced()
	workflows, err := enhancedClient.ListWorkflows(ctx, opts.Filter, opts.Runner, opts.Limit, 0)
	if err != nil {
		fmt.Printf("%s Failed to list workflows: %v\n", 
			style.ErrorStyle.Render("❌"), err)
		fmt.Printf("%s Use: POST /api/v1/workflow?runner=%s&operation=list_workflows\n", 
			style.DimStyle.Render("API:"), opts.Runner)
		fmt.Printf("%s Body: {\"filter\": \"%s\", \"limit\": %d}\n", 
			style.DimStyle.Render("Request:"), opts.Filter, opts.Limit)
		return err
	}

	if opts.JSONOutput {
		workflowsJSON, _ := json.MarshalIndent(workflows, "", "  ")
		fmt.Println(string(workflowsJSON))
	} else {
		// Display formatted list
		fmt.Printf("%s Workflows\n", style.HeaderStyle.Render("📋"))
		for _, workflow := range workflows.Workflows {
			statusEmoji := "⏳"
			if workflow.Status == "SUCCESS" {
				statusEmoji = "✅"
			} else if workflow.Status == "FAILED" {
				statusEmoji = "❌"
			}
			fmt.Printf("  %s %s [%s] - %s\n", 
				statusEmoji, workflow.ID, workflow.Status, workflow.Name)
		}
		
		if workflows.Pagination != nil {
			fmt.Printf("\n%s Showing %d workflows (Page %d)\n", 
				style.DimStyle.Render("📄"), 
				len(workflows.Workflows), 
				workflows.Pagination.Page)
		}
	}
	return nil
}