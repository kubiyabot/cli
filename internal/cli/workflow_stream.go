package cli

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"os/signal"
	"strings"
	"syscall"
	"text/tabwriter"
	"time"

	"github.com/kubiyabot/cli/internal/config"
	"github.com/kubiyabot/cli/internal/composer"
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
        page       int
        limit      int
        status     string
        search     string
        sortBy     string
        sortOrder  string
        jsonOutput bool
        workflowID string
    )

    cmd := &cobra.Command{
        Use:   "list",
        Short: "List organization workflows (Composer)",
        Long:  "List all workflows in your organization using the Composer API.",
        Example: `  # List workflows as a table
  kubiya workflow list

  # Output JSON with more items
  kubiya workflow list --json --limit 50

  # Filter by status or search by name
  kubiya workflow list --status published --search deploy`,
        RunE: func(cmd *cobra.Command, args []string) error {
            ctx := context.Background()
            comp := composer.NewClient(cfg)

            // Defaults per API docs
            if page <= 0 { page = 1 }
            if limit <= 0 { limit = 12 }
            if sortBy == "" { sortBy = "updated_at" }
            if sortOrder == "" { sortOrder = "desc" }

            // If --id provided, fetch a specific workflow and present it
            if workflowID != "" {
                wf, err := comp.GetWorkflow(ctx, workflowID)
                if err != nil {
                    return fmt.Errorf("failed to get workflow: %w", err)
                }

                // Assemble single row and print
                lastExec := ""
                if len(wf.RecentExecutions) > 0 {
                    if wf.RecentExecutions[0].FinishedAt != "" {
                        lastExec = wf.RecentExecutions[0].FinishedAt
                    } else {
                        lastExec = wf.RecentExecutions[0].StartedAt
                    }
                }
                totalExecs, _ := comp.CountWorkflowExecutions(ctx, wf.ID)

                if jsonOutput {
                    out := map[string]interface{}{
                        "name": wf.Name,
                        "description": wf.Description,
                        "status": wf.Status,
                        "last_execution": lastExec,
                        "created_by": wf.UserName,
                        "created_at": wf.CreatedAt,
                        "updated_at": wf.UpdatedAt,
                        "execution_count": totalExecs,
                    }
                    enc := json.NewEncoder(os.Stdout)
                    enc.SetIndent("", "  ")
                    return enc.Encode(out)
                }

                w := tabwriter.NewWriter(os.Stdout, 0, 0, 2, ' ', 0)
                fmt.Fprintln(w, style.TitleStyle.Render("📋 WORKFLOW"))
                fmt.Fprintln(w, "NAME\tSTATUS\tLAST EXECUTION\tCREATED BY\tCREATED AT\tUPDATED AT\tEXECUTIONS\tDESCRIPTION")
                fmt.Fprintf(w, "%s\t%s\t%s\t%s\t%s\t%s\t%d\t%s\n",
                    style.HighlightStyle.Render(wf.Name), wf.Status, lastExec, wf.UserName, wf.CreatedAt, wf.UpdatedAt, totalExecs, wf.Description)
                return w.Flush()
            }

            resp, err := comp.ListWorkflows(ctx, page, limit, status, search, "", sortBy, sortOrder)
            if err != nil {
                return fmt.Errorf("failed to list workflows: %w", err)
            }

            // Build rows with execution count and last execution
            type Row struct {
                Name        string
                Description string
                Status      string
                LastExec    string
                CreatedBy   string
                CreatedAt   string
                UpdatedAt   string
                ExecCount   int
            }
            rows := make([]Row, 0, len(resp.Workflows))

            for _, wf := range resp.Workflows {
                lastExec := ""
                if len(wf.RecentExecutions) > 0 {
                    if wf.RecentExecutions[0].FinishedAt != "" {
                        lastExec = wf.RecentExecutions[0].FinishedAt
                    } else {
                        lastExec = wf.RecentExecutions[0].StartedAt
                    }
                }

                totalExecs, _ := comp.CountWorkflowExecutions(ctx, wf.ID)

                rows = append(rows, Row{
                    Name:        wf.Name,
                    Description: wf.Description,
                    Status:      wf.Status,
                    LastExec:    lastExec,
                    CreatedBy:   wf.UserName,
                    CreatedAt:   wf.CreatedAt,
                    UpdatedAt:   wf.UpdatedAt,
                    ExecCount:   totalExecs,
                })
            }

            if jsonOutput {
                type OutItem struct {
                    Name           string `json:"name"`
                    Description    string `json:"description"`
                    Status         string `json:"status"`
                    LastExecution  string `json:"last_execution"`
                    CreatedBy      string `json:"created_by"`
                    CreatedAt      string `json:"created_at"`
                    UpdatedAt      string `json:"updated_at"`
                    ExecutionCount int    `json:"execution_count"`
                }
                out := struct {
                    Workflows []OutItem `json:"workflows"`
                    Total     int       `json:"total"`
                    Page      int       `json:"page"`
                    PageSize  int       `json:"page_size"`
                }{Total: resp.Total, Page: resp.Page, PageSize: resp.PageSize}
                out.Workflows = make([]OutItem, 0, len(rows))
                for _, r := range rows {
                    out.Workflows = append(out.Workflows, OutItem{
                        Name: r.Name, Description: r.Description, Status: r.Status,
                        LastExecution: r.LastExec, CreatedBy: r.CreatedBy,
                        CreatedAt: r.CreatedAt, UpdatedAt: r.UpdatedAt,
                        ExecutionCount: r.ExecCount,
                    })
                }
                enc := json.NewEncoder(os.Stdout)
                enc.SetIndent("", "  ")
                return enc.Encode(out)
            }

            // Table output by default
            w := tabwriter.NewWriter(os.Stdout, 0, 0, 2, ' ', 0)
            fmt.Fprintln(w, style.TitleStyle.Render("📋 WORKFLOWS"))
            fmt.Fprintln(w, "NAME\tSTATUS\tLAST EXECUTION\tCREATED BY\tCREATED AT\tUPDATED AT\tEXECUTIONS\tDESCRIPTION")
            for _, r := range rows {
                name := style.HighlightStyle.Render(r.Name)
                statusText := r.Status
                if strings.EqualFold(statusText, "published") {
                    statusText = style.SuccessStyle.Render("published")
                } else if strings.EqualFold(statusText, "draft") {
                    statusText = style.DimStyle.Render("draft")
                }
                fmt.Fprintf(w, "%s\t%s\t%s\t%s\t%s\t%s\t%d\t%s\n",
                    name, statusText, r.LastExec, r.CreatedBy, r.CreatedAt, r.UpdatedAt, r.ExecCount, r.Description)
            }
            return w.Flush()
        },
    }

    cmd.Flags().IntVar(&page, "page", 1, "Page number")
    cmd.Flags().IntVar(&limit, "limit", 12, "Items per page")
    cmd.Flags().StringVar(&status, "status", "all", "Filter by status (all|draft|published)")
    cmd.Flags().StringVar(&search, "search", "", "Search by workflow name")
    cmd.Flags().StringVar(&sortBy, "sort-by", "updated_at", "Sort by field")
    cmd.Flags().StringVar(&sortOrder, "sort-order", "desc", "Sort order (asc|desc)")
    cmd.Flags().BoolVar(&jsonOutput, "json", false, "Output as JSON")
    cmd.Flags().StringVar(&workflowID, "id", "", "Workflow ID to fetch a single workflow")

    return cmd
}

// newWorkflowExecutionListCommand lists executions from the last 24 hours
func newWorkflowExecutionListCommand(cfg *config.Config) *cobra.Command {
    var (
        limit      int
        status     string
        jsonOutput bool
    )

    cmd := &cobra.Command{
        Use:   "execution list",
        Short: "List workflow executions from the last 24 hours",
        RunE: func(cmd *cobra.Command, args []string) error {
            ctx := context.Background()
            comp := composer.NewClient(cfg)

            if limit <= 0 { limit = 100 }

            // We will page through executions until we either run out or hit older than 24h
            const pageSize = 50
            page := 1
            collected := 0
            cutoff := time.Now().Add(-24 * time.Hour)

            type Row struct {
                Name        string
                Status      string
                Runner      string
                StartedAt   time.Time
                Duration    time.Duration
                StepsDone   int
                StepsTotal  int
            }
            var rows []Row

            for collected < limit {
                resp, err := comp.ListExecutions(ctx, page, pageSize, status, "")
                if err != nil {
                    return fmt.Errorf("failed to list executions: %w", err)
                }
                if len(resp.Executions) == 0 {
                    break
                }

                for _, ex := range resp.Executions {
                    // Parse start time
                    started, err := time.Parse(time.RFC3339, ex.StartedAt)
                    if err != nil {
                        continue
                    }
                    if started.Before(cutoff) {
                        // Since API is sorted by recency, we can stop
                        collected = limit
                        break
                    }

                    // Fetch details for step counts and workflow name when needed
                    details, _ := comp.GetExecution(ctx, ex.ID)
                    name := ""
                    stepsTotal := 0
                    stepsDone := 0
                    runner := ex.Runner
                    if details != nil {
                        if details.Workflow != nil {
                            name = details.Workflow.Name
                        }
                        if details.Runner != "" {
                            runner = details.Runner
                        }
                        stepsTotal = len(details.Steps)
                        for _, st := range details.Steps {
                            if strings.EqualFold(st.Status, "completed") || strings.EqualFold(st.Status, "success") {
                                stepsDone++
                            }
                        }
                    }

                    // Duration
                    dur := time.Duration(ex.DurationMs) * time.Millisecond

                    rows = append(rows, Row{
                        Name:       name,
                        Status:     ex.Status,
                        Runner:     runner,
                        StartedAt:  started,
                        Duration:   dur,
                        StepsDone:  stepsDone,
                        StepsTotal: stepsTotal,
                    })
                    collected++
                    if collected >= limit {
                        break
                    }
                }

                if collected >= limit {
                    break
                }
                page++
            }

            if jsonOutput {
                type OutItem struct {
                    Name        string `json:"name"`
                    Status      string `json:"status"`
                    Runner      string `json:"runner"`
                    StartedAt   string `json:"started_at"`
                    Duration    string `json:"duration"`
                    StepsDone   int    `json:"steps_completed"`
                    StepsTotal  int    `json:"steps_total"`
                }
                out := make([]OutItem, 0, len(rows))
                for _, r := range rows {
                    out = append(out, OutItem{
                        Name: r.Name, Status: r.Status, Runner: r.Runner,
                        StartedAt: r.StartedAt.Format(time.RFC3339),
                        Duration: r.Duration.String(),
                        StepsDone: r.StepsDone, StepsTotal: r.StepsTotal,
                    })
                }
                enc := json.NewEncoder(os.Stdout)
                enc.SetIndent("", "  ")
                return enc.Encode(out)
            }

            // Table output by default
            w := tabwriter.NewWriter(os.Stdout, 0, 0, 2, ' ', 0)
            fmt.Fprintln(w, style.TitleStyle.Render("🕒 EXECUTIONS (last 24h)"))
            fmt.Fprintln(w, "NAME\tSTATUS\tRUNNER\tSTARTED AT\tDURATION\tSTEPS (DONE/TOTAL)")
            for _, r := range rows {
                started := r.StartedAt.Format(time.RFC3339)
                fmt.Fprintf(w, "%s\t%s\t%s\t%s\t%s\t%d/%d\n",
                    style.HighlightStyle.Render(r.Name), r.Status, r.Runner, started, r.Duration.String(), r.StepsDone, r.StepsTotal)
            }
            return w.Flush()
        },
    }

    cmd.Flags().IntVar(&limit, "limit", 100, "Max number of executions to return")
    cmd.Flags().StringVar(&status, "status", "", "Filter by status (running|completed|failed)")
    cmd.Flags().BoolVar(&jsonOutput, "json", false, "Output as JSON")

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

// Deprecated: listWorkflows (runner-based) removed in favor of Composer API implementation