package cli

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"strings"
	"time"

	"github.com/kubiyabot/cli/internal/config"
	"github.com/kubiyabot/cli/internal/kubiya"
	"github.com/kubiyabot/cli/internal/style"
	"github.com/spf13/cobra"
	"gopkg.in/yaml.v3"
)

func newWorkflowExecuteCommand(cfg *config.Config) *cobra.Command {
	var (
		runner          string
		variables       []string
		watch           bool
		skipPolicyCheck bool
		verbose         bool
		startFromStep   string
		noRetry         bool
		maxRetries      int
		retryDelay      int
		interactive     bool
	)

	cmd := &cobra.Command{
		Use:   "execute [workflow-file]",
		Short: "Execute a workflow from a file",
		Long: `Execute a workflow defined in a YAML or JSON file.

This command loads a workflow from a file and executes it using the Kubiya API.
The file can be in YAML (.yaml, .yml) or JSON (.json) format, or the format will be auto-detected.
You can provide variables and choose the runner for execution.`,
		Example: `  # Execute a YAML workflow
  kubiya workflow execute deploy.yaml

  # Execute a JSON workflow
  kubiya workflow execute backup.json

  # Execute with variables
  kubiya workflow execute backup.yaml --var env=production --var retention=30

  # Execute with specific runner
  kubiya workflow execute migrate.json --runner prod-runner

  # Execute with verbose SSE logging
  kubiya workflow execute long-running --verbose

  # Start from a specific step
  kubiya workflow execute deploy.yaml --start-from-step "deploy-app"

  # Skip policy validation
  kubiya workflow execute deploy.yaml --skip-policy-check`,
		Args: cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			client := kubiya.NewClient(cfg)
			ctx := context.Background()

			// Read and parse workflow file
			workflowFile := args[0]
			
			// Parse variables first
			vars := make(map[string]interface{})
			for _, v := range variables {
				parts := strings.SplitN(v, "=", 2)
				if len(parts) == 2 {
					vars[parts[0]] = parts[1]
				}
			}

			// Parse workflow file (supports both JSON and YAML with auto-detection)
			workflow, workflowReq, format, err := parseWorkflowFile(workflowFile, vars)
			if err != nil {
				return err
			}

			var req kubiya.WorkflowExecutionRequest
			if workflowReq != nil {
				// Already in WorkflowExecutionRequest format
				req = *workflowReq
			} else if workflow != nil {
				// Convert from Workflow struct
				req = buildExecutionRequest(*workflow, vars, runner)
			} else {
				return fmt.Errorf("failed to parse workflow file")
			}

			// Show format detection info if helpful
			if format != "" {
				fmt.Printf("%s Detected format: %s\n", style.DimStyle.Render("📄"), format)
			}

			fmt.Printf("%s Executing workflow: %s\n", style.StatusStyle.Render("🚀"), style.HighlightStyle.Render(req.Name))
			fmt.Printf("%s %s\n", style.DimStyle.Render("File:"), workflowFile)
			if runner != "" {
				fmt.Printf("%s %s\n", style.DimStyle.Render("Runner:"), runner)
			}
			if len(req.Params) > 0 {
				fmt.Printf("%s\n", style.DimStyle.Render("Params:"))
				for k, v := range req.Params {
					fmt.Printf("  %s = %v\n", style.KeyStyle.Render(k), v)
				}
			}
			fmt.Println()

			// Policy validation (if enabled)
			if !skipPolicyCheck {
				opaEnforce := os.Getenv("KUBIYA_OPA_ENFORCE")
				if opaEnforce == "true" || opaEnforce == "1" {
					fmt.Printf("%s Validating workflow execution permissions...\n", style.InfoStyle.Render("🛡️"))
					
					// Convert workflow to map for validation
					workflowDef := map[string]interface{}{
						"name":        req.Name,
						"description": req.Description,
						"steps":       req.Steps,
					}
					
					allowed, issues, err := client.ValidateWorkflowExecution(ctx, workflowDef, req.Params, runner)
					if err != nil {
						return fmt.Errorf("workflow permission validation failed: %w", err)
					}
					
					if !allowed {
						fmt.Printf("%s Workflow execution denied by policy:\n", style.ErrorStyle.Render("❌"))
						for _, issue := range issues {
							fmt.Printf("  • %s\n", issue)
						}
						return fmt.Errorf("workflow execution denied by policy")
					}
					
					if len(issues) > 0 {
						fmt.Printf("%s Workflow execution permitted with warnings:\n", style.WarningStyle.Render("⚠️"))
						for _, issue := range issues {
							fmt.Printf("  • %s\n", issue)
						}
					} else {
						fmt.Printf("%s Workflow execution permissions validated\n", style.SuccessStyle.Render("✅"))
					}
					fmt.Println()
				}
			}

			// Create workflow client with verbose mode
			workflowClient, err := kubiya.NewRobustWorkflowClient(client.Workflow(), cfg.Debug || verbose)
			if err != nil {
				return fmt.Errorf("failed to create workflow client: %w", err)
			}

			// Configure retry options
			options := kubiya.RobustExecutionOptions{
				NoRetry:     noRetry,
				MaxRetries:  maxRetries,
				RetryDelay:  time.Duration(retryDelay) * time.Second,
				Verbose:     verbose,
			}
			
			// Execute workflow 
			var events <-chan kubiya.RobustWorkflowEvent
			if startFromStep != "" {
				// Start from specific step
				events, err = workflowClient.ExecuteWorkflowFromStepWithOptions(ctx, req, runner, startFromStep, options)
			} else {
				// Normal execution
				events, err = workflowClient.ExecuteWorkflowRobustWithOptions(ctx, req, runner, options)
			}
			if err != nil {
				return fmt.Errorf("failed to execute workflow: %w", err)
			}

			// Process workflow events 
			var hasError bool
			var executionID string
			
			// Use interactive mode if requested
			if interactive {
				renderer := NewInteractiveWorkflowRenderer(req.Name, req.Steps)
				
				for event := range events {
					renderer.ProcessEvent(event)
					
					// Track execution state
					if event.ExecutionID != "" {
						executionID = event.ExecutionID
					}
					
					if event.Type == "error" || (event.Type == "complete" && event.State != nil && event.State.Status == "failed") {
						hasError = true
					}
				}
			} else {
				// Standard mode processing
				var stepStartTimes = make(map[string]time.Time)
				var stepOutputs = make(map[string]string)
				var isReconnecting bool
				
				fmt.Printf("\n%s Starting workflow execution...\n\n", 
					style.InfoStyle.Render("🚀"))
			
			for event := range events {
				// Show verbose SSE details if requested
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
					// Initial state or state updates
					executionID = event.ExecutionID
					if event.State != nil {
						fmt.Printf("%s %s\n", 
							style.InfoStyle.Render("📊"), 
							event.Message)
						fmt.Printf("%s Execution ID: %s\n", 
							style.DimStyle.Render("🆔"), 
							executionID)
						fmt.Printf("%s Progress: %d/%d steps completed\n\n", 
							style.DimStyle.Render("📊"), 
							event.State.CompletedSteps, 
							event.State.TotalSteps)
					}
					
				case "step":
					// Step status updates
					if event.StepStatus == "running" {
						stepStartTimes[event.StepName] = time.Now()
						
						// Enhanced step display with progress
						if event.State != nil {
							progress := fmt.Sprintf("[%d/%d]", event.State.CompletedSteps+1, event.State.TotalSteps)
							fmt.Printf("%s %s %s\n", 
								style.BulletStyle.Render("▶️"), 
								style.DimStyle.Render(progress),
								style.ToolNameStyle.Render(event.StepName))
						}
						
						fmt.Printf("  %s Running...\n", style.DimStyle.Render("⏳"))
						
					} else if event.StepStatus == "completed" || event.StepStatus == "finished" {
						// Calculate step duration
						var duration time.Duration
						if startTime, ok := stepStartTimes[event.StepName]; ok {
							duration = time.Since(startTime)
							delete(stepStartTimes, event.StepName)
						}
						
						if event.Data != "" {
							// Truncate long outputs for better display
							displayOutput := event.Data
							if len(event.Data) > 200 {
								displayOutput = event.Data[:200] + "..."
							}
							fmt.Printf("  %s %s\n", 
								style.DimStyle.Render("📤 Output:"), 
								style.ToolOutputStyle.Render(displayOutput))
							
							// Store full output for summary
							stepOutputs[event.StepName] = event.Data
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
						// Calculate step duration
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
						// Check if it's a structured log or plain text
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
						fmt.Printf("%s Execution ID: %s\n", 
							style.DimStyle.Render("🆔"), 
							executionID)
						
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
			}

			// Clean up old executions (keep for 24 hours)
			if err := workflowClient.CleanupOldExecutions(24 * time.Hour); err != nil {
				if cfg.Debug {
					fmt.Printf("[DEBUG] Failed to cleanup old executions: %v\n", err)
				}
			}

			if hasError {
				fmt.Printf("\n%s Workflow execution failed. Check the logs above for details.\n", 
					style.ErrorStyle.Render("💥"))
				os.Exit(1)
			}

			return nil
		},
	}

	cmd.Flags().StringVar(&runner, "runner", "", "Runner to use for execution")
	cmd.Flags().StringArrayVar(&variables, "var", []string{}, "Params in key=value format")
	cmd.Flags().BoolVarP(&watch, "watch", "w", true, "Watch execution output")
	cmd.Flags().BoolVar(&skipPolicyCheck, "skip-policy-check", false, "Skip policy validation before execution")
	cmd.Flags().BoolVarP(&verbose, "verbose", "v", false, "Enable verbose output with detailed SSE logs")
	cmd.Flags().StringVar(&startFromStep, "start-from-step", "", "Start execution from a specific step (resume from step)")
	cmd.Flags().BoolVar(&noRetry, "no-retry", false, "Disable connection retry on failures")
	cmd.Flags().IntVar(&maxRetries, "max-retries", 10, "Maximum number of connection retry attempts")
	cmd.Flags().IntVar(&retryDelay, "retry-delay", 2, "Initial retry delay in seconds (exponential backoff)")
	cmd.Flags().BoolVarP(&interactive, "interactive", "i", false, "Enable interactive mode with workflow visualization")

	return cmd
}

// parseWorkflowFile parses a workflow file that can be in JSON or YAML format
// It returns either a Workflow struct or a WorkflowExecutionRequest, along with format info
func parseWorkflowFile(filePath string, vars map[string]interface{}) (*Workflow, *kubiya.WorkflowExecutionRequest, string, error) {
	data, err := os.ReadFile(filePath)
	if err != nil {
		return nil, nil, "", fmt.Errorf("failed to read workflow file: %w", err)
	}

	// First, try to determine format by file extension
	isJSON := strings.HasSuffix(strings.ToLower(filePath), ".json")
	isYAML := strings.HasSuffix(strings.ToLower(filePath), ".yaml") || strings.HasSuffix(strings.ToLower(filePath), ".yml")

	// If no clear extension, try to auto-detect format
	if !isJSON && !isYAML {
		// Try JSON first by looking for typical JSON markers
		trimmed := strings.TrimSpace(string(data))
		if strings.HasPrefix(trimmed, "{") && strings.HasSuffix(trimmed, "}") {
			isJSON = true
		} else {
			isYAML = true // Default to YAML
		}
	}

	if isJSON {
		return parseJSONWorkflow(data, vars)
	}
	return parseYAMLWorkflow(data, vars)
}

// parseJSONWorkflow parses JSON workflow data
func parseJSONWorkflow(data []byte, vars map[string]interface{}) (*Workflow, *kubiya.WorkflowExecutionRequest, string, error) {
	// Try to parse as Workflow struct first
	var workflow Workflow
	if err := json.Unmarshal(data, &workflow); err == nil {
		// Successfully parsed as Workflow struct
		return &workflow, nil, "json-workflow", nil
	}

	// Try to parse as WorkflowExecutionRequest
	var req kubiya.WorkflowExecutionRequest
	if err := json.Unmarshal(data, &req); err != nil {
		return nil, nil, "", fmt.Errorf("failed to parse JSON as either Workflow or WorkflowExecutionRequest: %w", err)
	}

	// Merge variables into the request
	if req.Params == nil {
		req.Params = vars
	} else {
		for k, v := range vars {
			req.Params[k] = v
		}
	}

	return nil, &req, "json-request", nil
}

// parseYAMLWorkflow parses YAML workflow data
func parseYAMLWorkflow(data []byte, vars map[string]interface{}) (*Workflow, *kubiya.WorkflowExecutionRequest, string, error) {
	// Try to parse as Workflow struct first
	var workflow Workflow
	if err := yaml.Unmarshal(data, &workflow); err == nil && workflow.Name != "" {
		// Successfully parsed as Workflow struct
		return &workflow, nil, "yaml-workflow", nil
	}

	// Try to parse as WorkflowExecutionRequest
	var req kubiya.WorkflowExecutionRequest
	if err := yaml.Unmarshal(data, &req); err != nil {
		return nil, nil, "", fmt.Errorf("failed to parse YAML as either Workflow or WorkflowExecutionRequest: %w", err)
	}

	// Merge variables into the request
	if req.Params == nil {
		req.Params = vars
	} else {
		for k, v := range vars {
			req.Params[k] = v
		}
	}

	return nil, &req, "yaml-request", nil
}

// buildExecutionRequest converts a Workflow to WorkflowExecutionRequest
func buildExecutionRequest(workflow Workflow, vars map[string]interface{}, runner string) kubiya.WorkflowExecutionRequest {
	// Convert WorkflowStep to interface{} for the API
	steps := make([]interface{}, len(workflow.Steps))
	for i, step := range workflow.Steps {
		stepMap := map[string]interface{}{
			"name": step.Name,
		}
		if step.Description != "" {
			stepMap["description"] = step.Description
		}
		if step.Command != "" {
			stepMap["command"] = step.Command
		}
		if step.Executor.Type != "" {
			stepMap["executor"] = map[string]interface{}{
				"type":   step.Executor.Type,
				"config": step.Executor.Config,
			}
		}
		if step.Output != "" {
			stepMap["output"] = step.Output
		}
		if len(step.Depends) > 0 {
			stepMap["depends"] = step.Depends
		}
		steps[i] = stepMap
	}

	return kubiya.WorkflowExecutionRequest{
		Name:        workflow.Name,
		Description: fmt.Sprintf("Execution of %s", workflow.Name),
		Steps:       steps,
		Params:   vars,
	}
}
