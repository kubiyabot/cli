package cli

import (
	"context"
	"fmt"
	"strings"
	"time"

	"github.com/kubiyabot/cli/internal/config"
	"github.com/kubiyabot/cli/internal/kubiya"
	"github.com/kubiyabot/cli/internal/style"
	"github.com/spf13/cobra"
)

func newWorkflowResumeCommand(cfg *config.Config) *cobra.Command {
	var (
		listActive bool
		watch      bool
		verbose    bool
	)

	cmd := &cobra.Command{
		Use:   "resume [execution-id]",
		Short: "Resume an interrupted workflow execution",
		Long: `Resume monitoring a previously interrupted workflow execution.

This command allows you to reconnect to workflows that were interrupted due to
connection issues, CLI restarts, or other interruptions. The workflow state
is preserved locally and can be resumed at any time.`,
		Example: `  # List all active/interrupted executions
  kubiya workflow resume --list

  # Resume a specific execution
  kubiya workflow resume exec_1234567890_123456

  # Resume without watching output
  kubiya workflow resume exec_1234567890_123456 --no-watch`,
		Args: cobra.MaximumNArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			client := kubiya.NewClient(cfg)
			ctx := context.Background()

			// Create robust workflow client
			robustClient, err := kubiya.NewRobustWorkflowClient(client.Workflow(), cfg.Debug)
			if err != nil {
				return fmt.Errorf("failed to create workflow client: %w", err)
			}

			if listActive {
				return listActiveExecutions(robustClient)
			}

			if len(args) == 0 {
				return fmt.Errorf("execution ID is required. Use --list to see active executions")
			}

			executionID := args[0]

			// Check if execution exists and get its state
			state, err := robustClient.GetExecutionState(executionID)
			if err != nil {
				return fmt.Errorf("failed to get execution state: %w", err)
			}

			// Display execution info
			fmt.Printf("%s Resuming workflow execution\n", style.InfoStyle.Render("🔄"))
			fmt.Printf("%s Workflow: %s\n", style.DimStyle.Render("📋"), state.WorkflowName)
			fmt.Printf("%s Execution ID: %s\n", style.DimStyle.Render("🆔"), executionID)
			fmt.Printf("%s Status: %s\n", style.DimStyle.Render("📊"), state.Status)
			fmt.Printf("%s Progress: %d/%d steps completed\n", style.DimStyle.Render("📈"), state.CompletedSteps, state.TotalSteps)
			
			if state.ConnectionLost {
				fmt.Printf("%s Connection was lost, will attempt to reconnect\n", style.WarningStyle.Render("⚠️"))
			}
			
			if state.RetryCount > 0 {
				fmt.Printf("%s Previous retry attempts: %d\n", style.DimStyle.Render("🔄"), state.RetryCount)
			}

			duration := time.Since(state.StartTime)
			fmt.Printf("%s Running for: %v\n", style.DimStyle.Render("⏱️"), duration.Round(time.Second))
			fmt.Println()

			// Resume execution
			events, err := robustClient.ResumeExecution(ctx, executionID)
			if err != nil {
				return fmt.Errorf("failed to resume execution: %w", err)
			}

			// Process events (similar to execute command)
			return processResumedWorkflowEvents(events, watch, cfg.Debug || verbose)
		},
	}

	cmd.Flags().BoolVar(&listActive, "list", false, "List all active executions")
	cmd.Flags().BoolVarP(&watch, "watch", "w", true, "Watch execution output")
	cmd.Flags().BoolVarP(&verbose, "verbose", "v", false, "Enable verbose output with detailed logs")

	return cmd
}

func listActiveExecutions(robustClient *kubiya.RobustWorkflowClient) error {
	executions, err := robustClient.ListActiveExecutions()
	if err != nil {
		return fmt.Errorf("failed to list active executions: %w", err)
	}

	if len(executions) == 0 {
		fmt.Printf("%s No active workflow executions found\n", style.InfoStyle.Render("ℹ️"))
		return nil
	}

	fmt.Printf("%s Active workflow executions:\n\n", style.InfoStyle.Render("📋"))

	for _, exec := range executions {
		status := exec.Status
		statusStyle := style.InfoStyle
		
		if exec.ConnectionLost {
			status += " (connection lost)"
			statusStyle = style.WarningStyle
		} else if exec.Status == "failed" {
			statusStyle = style.ErrorStyle
		} else if exec.Status == "completed" {
			statusStyle = style.SuccessStyle
		}

		duration := time.Since(exec.StartTime)
		if exec.EndTime != nil {
			duration = exec.EndTime.Sub(exec.StartTime)
		}

		fmt.Printf("%s %s\n", style.BulletStyle.Render("▶"), style.ToolNameStyle.Render(exec.WorkflowName))
		fmt.Printf("  %s %s\n", style.DimStyle.Render("🆔 ID:"), exec.ExecutionID)
		fmt.Printf("  %s %s\n", style.DimStyle.Render("📊 Status:"), statusStyle.Render(status))
		fmt.Printf("  %s %d/%d\n", style.DimStyle.Render("📈 Progress:"), exec.CompletedSteps, exec.TotalSteps)
		fmt.Printf("  %s %v\n", style.DimStyle.Render("⏱️ Duration:"), duration.Round(time.Second))
		
		if exec.CurrentStep != "" {
			fmt.Printf("  %s %s\n", style.DimStyle.Render("⚡ Current:"), exec.CurrentStep)
		}
		
		if exec.RetryCount > 0 {
			fmt.Printf("  %s %d\n", style.DimStyle.Render("🔄 Retries:"), exec.RetryCount)
		}

		// Show step summary
		if len(exec.StepHistory) > 0 {
			var completed, running, failed, pending int
			for _, step := range exec.StepHistory {
				switch step.Status {
				case "completed":
					completed++
				case "running":
					running++
				case "failed":
					failed++
				default:
					pending++
				}
			}
			
			fmt.Printf("  %s ", style.DimStyle.Render("📝 Steps:"))
			if completed > 0 {
				fmt.Printf("%s %d", style.SuccessStyle.Render("✅"), completed)
			}
			if running > 0 {
				if completed > 0 { fmt.Printf(", ") }
				fmt.Printf("%s %d", style.InfoStyle.Render("⏳"), running)
			}
			if failed > 0 {
				if completed > 0 || running > 0 { fmt.Printf(", ") }
				fmt.Printf("%s %d", style.ErrorStyle.Render("❌"), failed)
			}
			if pending > 0 {
				if completed > 0 || running > 0 || failed > 0 { fmt.Printf(", ") }
				fmt.Printf("%s %d", style.DimStyle.Render("⏳"), pending)
			}
			fmt.Println()
		}

		fmt.Println()
	}

	fmt.Printf("%s To resume an execution: %s\n", 
		style.DimStyle.Render("💡 Tip:"), 
		style.HighlightStyle.Render("kubiya workflow resume <execution-id>"))

	return nil
}

func processResumedWorkflowEvents(events <-chan kubiya.RobustWorkflowEvent, watch bool, verbose bool) error {
	var hasError bool
	var stepStartTimes = make(map[string]time.Time)
	var isReconnecting bool

	for event := range events {
		// Show verbose details if requested
		if verbose {
			fmt.Printf("[VERBOSE] Event: %s, ExecutionID: %s, Message: %s\n", 
				event.Type, event.ExecutionID, event.Message)
			if event.Data != "" {
				fmt.Printf("[VERBOSE] Data: %s\n", event.Data)
			}
		}
		
		if !watch {
			continue
		}

		switch event.Type {
		case "state":
			// State updates
			if event.State != nil {
				fmt.Printf("%s %s\n", 
					style.InfoStyle.Render("📊"), 
					event.Message)
				fmt.Printf("%s Progress: %d/%d steps completed\n", 
					style.DimStyle.Render("📈"), 
					event.State.CompletedSteps, 
					event.State.TotalSteps)
				fmt.Println()
			}

		case "step":
			// Step status updates
			if event.StepStatus == "running" {
				stepStartTimes[event.StepName] = time.Now()
				
				if event.State != nil {
					progress := fmt.Sprintf("[%d/%d]", event.State.CompletedSteps+1, event.State.TotalSteps)
					fmt.Printf("%s %s %s\n", 
						style.BulletStyle.Render("▶️"), 
						style.DimStyle.Render(progress),
						style.ToolNameStyle.Render(event.StepName))
				}
				
				fmt.Printf("  %s Running...\n", style.DimStyle.Render("⏳"))
				
			} else if event.StepStatus == "completed" || event.StepStatus == "finished" {
				var duration time.Duration
				if startTime, ok := stepStartTimes[event.StepName]; ok {
					duration = time.Since(startTime)
					delete(stepStartTimes, event.StepName)
				}
				
				if event.Data != "" {
					displayOutput := event.Data
					if len(event.Data) > 200 {
						displayOutput = event.Data[:200] + "..."
					}
					fmt.Printf("  %s %s\n", 
						style.DimStyle.Render("📤 Output:"), 
						style.ToolOutputStyle.Render(displayOutput))
				}
				
				if duration > 0 {
					fmt.Printf("  %s Step completed in %v\n\n", 
						style.SuccessStyle.Render("✅"), 
						duration.Round(time.Millisecond))
				} else {
					fmt.Printf("  %s Step completed\n\n", 
						style.SuccessStyle.Render("✅"))
				}
				
			} else if event.StepStatus == "failed" {
				var duration time.Duration
				if startTime, ok := stepStartTimes[event.StepName]; ok {
					duration = time.Since(startTime)
					delete(stepStartTimes, event.StepName)
				}
				
				if duration > 0 {
					fmt.Printf("  %s Step failed after %v\n\n", 
						style.ErrorStyle.Render("❌"), 
						duration.Round(time.Millisecond))
				} else {
					fmt.Printf("  %s Step failed\n\n", 
						style.ErrorStyle.Render("❌"))
				}
				hasError = true
			}

		case "data":
			// Raw data output
			if event.Data != "" {
				if strings.Contains(event.Data, ":") {
					fmt.Printf("  %s %s\n", style.DimStyle.Render("📝"), event.Data)
				} else {
					fmt.Println(event.Data)
				}
			}

		case "reconnect":
			// Connection recovery
			if event.Reconnect {
				if !isReconnecting {
					fmt.Printf("\n%s Connection lost, attempting to reconnect...\n", 
						style.WarningStyle.Render("🔄"))
					isReconnecting = true
				}
				fmt.Printf("  %s %s\n", 
					style.DimStyle.Render("⏳"), 
					event.Message)
			} else {
				fmt.Printf("  %s %s\n\n", 
					style.SuccessStyle.Render("✅"), 
					event.Message)
				isReconnecting = false
			}

		case "complete":
			// Workflow completion
			if event.State != nil {
				totalDuration := time.Since(event.State.StartTime)
				if event.State.EndTime != nil {
					totalDuration = event.State.EndTime.Sub(event.State.StartTime)
				}
				
				if event.State.Status == "completed" {
					fmt.Printf("%s Workflow completed successfully!\n", 
						style.SuccessStyle.Render("🎉"))
				} else {
					fmt.Printf("%s Workflow execution failed\n", 
						style.ErrorStyle.Render("💥"))
					hasError = true
				}
				
				fmt.Printf("%s Total duration: %v\n", 
					style.DimStyle.Render("⏱️"), 
					totalDuration.Round(time.Millisecond))
				fmt.Printf("%s Steps completed: %d/%d\n", 
					style.DimStyle.Render("📊"), 
					event.State.CompletedSteps, 
					event.State.TotalSteps)
				
				if event.State.RetryCount > 0 {
					fmt.Printf("%s Connection retries: %d\n", 
						style.DimStyle.Render("🔄"), 
						event.State.RetryCount)
				}
			}

		case "error":
			// Error events
			if event.Error != "" {
				fmt.Printf("%s %s\n", 
					style.ErrorStyle.Render("💀 Error:"), 
					event.Error)
				hasError = true
			}
		}
	}

	if hasError {
		return fmt.Errorf("workflow execution failed")
	}

	return nil
}