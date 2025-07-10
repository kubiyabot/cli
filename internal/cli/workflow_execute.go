package cli

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
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
		saveTrace       bool
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
			
			// Debug logging for workflow parsing
			if cfg.Debug || verbose {
				fmt.Printf("[DEBUG] Parsed workflow: Name=%s, Steps=%d\n", req.Name, len(req.Steps))
				for i, step := range req.Steps {
					if stepMap, ok := step.(map[string]interface{}); ok {
						if name, ok := stepMap["name"].(string); ok {
							fmt.Printf("[DEBUG] Step %d: %s\n", i+1, name)
						}
					}
				}
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

			// Set the command field as required by the API
			req.Command = "execute_workflow"
			
			// Inject Kubiya API key into workflow environment
			if req.Env == nil {
				req.Env = make(map[string]interface{})
			}
			req.Env["KUBIYA_API_KEY"] = cfg.APIKey
			
			if cfg.Debug || verbose {
				fmt.Printf("[DEBUG] Injected KUBIYA_API_KEY into workflow environment\n")
			}

			// Show connection status
			runnerDisplayName := runner
			if runner == "kubiya-hosted" {
				runnerDisplayName = "Kubiya Hosted Runner"
			}
			fmt.Printf("%s Connecting to %s...", 
				style.InfoStyle.Render("🔌"), runnerDisplayName)
			
			// Execute workflow directly with simple client (like it worked 3 days ago!)
			workflowClient := client.Workflow()
			events, err := workflowClient.ExecuteWorkflow(ctx, req, runner)
			if err != nil {
				fmt.Printf(" %s\n", style.ErrorStyle.Render("failed!"))
				return fmt.Errorf("failed to execute workflow: %w", err)
			}
			
			fmt.Printf(" %s\n", style.SuccessStyle.Render("connected!"))

			// Process workflow events 
			var hasError bool
			var stepCount int
			var completedSteps int
			
			// Count total steps for progress tracking
			stepCount = len(req.Steps)
			
			fmt.Printf("\n%s Starting workflow execution...\n\n", 
				style.InfoStyle.Render("🚀"))
			
			// Show initial progress
			progressBar := generateProgressBar(completedSteps, stepCount)
			fmt.Printf("%s %s %s\n\n", 
				style.InfoStyle.Render("📊"),
				progressBar,
				style.HighlightStyle.Render(fmt.Sprintf("%d/%d steps completed", completedSteps, stepCount)))
			
			// Initialize workflow execution tracking
			var stepStartTimes = make(map[string]time.Time)
			var workflowTrace = NewWorkflowTrace(req.Name, stepCount)
			
			for event := range events {
				if event.Type == "data" {
					// Parse JSON data for workflow events
					var jsonData map[string]interface{}
					if err := json.Unmarshal([]byte(event.Data), &jsonData); err == nil {
						if eventType, ok := jsonData["type"].(string); ok {
							switch eventType {
							case "step_running":
								if step, ok := jsonData["step"].(map[string]interface{}); ok {
									if stepName, ok := step["name"].(string); ok {
										stepStartTimes[stepName] = time.Now()
										
										// Update workflow trace
										workflowTrace.StartStep(stepName)
										
										// Show step starting
										progress := fmt.Sprintf("[%d/%d]", completedSteps+1, stepCount)
										fmt.Printf("%s %s %s\n", 
											style.BulletStyle.Render("▶️"), 
											style.InfoStyle.Render(progress),
											style.ToolNameStyle.Render(stepName))
										fmt.Printf("  %s Running...\n", style.StatusStyle.Render("⏳"))
									}
								}
								
							case "step_complete":
								if step, ok := jsonData["step"].(map[string]interface{}); ok {
									if stepName, ok := step["name"].(string); ok {
										// Calculate duration
										var duration time.Duration
										if startTime, ok := stepStartTimes[stepName]; ok {
											duration = time.Since(startTime)
											delete(stepStartTimes, stepName)
										}
										
										// Extract step output if available
										var stepOutput string
										var stepStatus string = "finished"
										if output, ok := step["output"].(string); ok && output != "" {
											stepOutput = output
										}
										if status, ok := step["status"].(string); ok {
											stepStatus = status
										}
										
										// Show step completion with status
										if duration > 0 {
											fmt.Printf("  %s Step %s in %v\n", 
												style.SuccessStyle.Render("✅"), 
												stepStatus,
												duration.Round(time.Millisecond))
										} else {
											fmt.Printf("  %s Step %s\n", 
												style.SuccessStyle.Render("✅"),
												stepStatus)
										}
										
										// Update workflow trace
										workflowTrace.CompleteStep(stepName, stepStatus, stepOutput)
										
										// Show step output if available
										if stepOutput != "" {
											// Format output nicely
											fmt.Printf("  %s %s\n", 
												style.DimStyle.Render("📤 Output:"),
												style.ToolOutputStyle.Render(formatStepOutput(stepOutput)))
										}
										
										// Update progress
										completedSteps++
										progressBar := generateProgressBar(completedSteps, stepCount)
										fmt.Printf("  %s %s %s\n\n", 
											style.SuccessStyle.Render("📊"),
											progressBar,
											style.HighlightStyle.Render(fmt.Sprintf("%d/%d steps completed", completedSteps, stepCount)))
									}
								}
								
							case "workflow_complete":
								// Workflow finished
								if success, ok := jsonData["success"].(bool); ok && success {
									workflowTrace.Complete("completed")
									fmt.Printf("%s Workflow completed successfully!\n", 
										style.SuccessStyle.Render("🎉"))
								} else {
									workflowTrace.Complete("failed")
									fmt.Printf("%s Workflow execution failed\n", 
										style.ErrorStyle.Render("💥"))
									hasError = true
								}
								
								// Show workflow execution graph
								fmt.Print(workflowTrace.GenerateGraph())
								
								// Save trace to file if requested
								if saveTrace {
									if err := saveWorkflowTrace(workflowTrace, workflowFile); err != nil {
										fmt.Fprintf(os.Stderr, "Warning: Failed to save workflow trace: %v\n", err)
									}
								}
								
								return nil
							}
						}
					}
				} else if event.Type == "error" {
					fmt.Printf("%s %s\n", 
						style.ErrorStyle.Render("💀 Error:"), 
						event.Data)
					hasError = true
				} else if event.Type == "done" {
					// Stream ended
					break
				}
				
				// Show verbose SSE details if requested  
				if verbose {
					fmt.Printf("[VERBOSE] Event: %s, Data: %s\n", event.Type, event.Data)
				}
			}

			if hasError {
				workflowTrace.Complete("failed")
				fmt.Printf("\n%s Workflow execution failed. Check the logs above for details.\n", 
					style.ErrorStyle.Render("💥"))
				fmt.Print(workflowTrace.GenerateGraph())
				
				// Save trace to file if requested
				if saveTrace {
					if err := saveWorkflowTrace(workflowTrace, workflowFile); err != nil {
						fmt.Fprintf(os.Stderr, "Warning: Failed to save workflow trace: %v\n", err)
					}
				}
				
				return fmt.Errorf("workflow execution failed")
			}

			// If we reach here, the stream ended without explicit completion
			// This could mean the workflow completed successfully but didn't send the completion event
			fmt.Printf("\n%s Stream ended - checking workflow status...\n", 
				style.InfoStyle.Render("ℹ️"))
			
			if completedSteps >= stepCount && stepCount > 0 {
				workflowTrace.Complete("completed")
				fmt.Printf("%s Workflow appears to have completed successfully (%d/%d steps)\n", 
					style.SuccessStyle.Render("✅"), completedSteps, stepCount)
			} else {
				workflowTrace.Complete("incomplete")
				fmt.Printf("%s Workflow may be incomplete (%d/%d steps completed)\n", 
					style.WarningStyle.Render("⚠️"), completedSteps, stepCount)
				fmt.Print(workflowTrace.GenerateGraph())
				
				// Save trace to file if requested
				if saveTrace {
					if err := saveWorkflowTrace(workflowTrace, workflowFile); err != nil {
						fmt.Fprintf(os.Stderr, "Warning: Failed to save workflow trace: %v\n", err)
					}
				}
				
				return fmt.Errorf("workflow stream ended unexpectedly")
			}
			
			// Show final workflow execution graph
			fmt.Print(workflowTrace.GenerateGraph())
			
			// Save trace to file if requested
			if saveTrace {
				if err := saveWorkflowTrace(workflowTrace, workflowFile); err != nil {
					fmt.Fprintf(os.Stderr, "Warning: Failed to save workflow trace: %v\n", err)
				}
			}

			return nil
		},
	}

	cmd.Flags().StringVar(&runner, "runner", "", "Runner to use for execution")
	cmd.Flags().StringArrayVar(&variables, "var", []string{}, "Params in key=value format")
	cmd.Flags().BoolVarP(&watch, "watch", "w", true, "Watch execution output")
	cmd.Flags().BoolVar(&skipPolicyCheck, "skip-policy-check", false, "Skip policy validation before execution")
	cmd.Flags().BoolVarP(&verbose, "verbose", "v", false, "Enable verbose output with detailed SSE logs")
	cmd.Flags().BoolVar(&saveTrace, "save-trace", false, "Save workflow execution trace to JSON file")

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
		Command:     "execute_workflow",
		Name:        workflow.Name,
		Description: fmt.Sprintf("Execution of %s", workflow.Name),
		Steps:       steps,
		Params:   vars,
	}
}

// generateProgressBar creates a visual progress bar for workflow execution
func generateProgressBar(completed, total int) string {
	if total == 0 {
		return "[-]"
	}
	
	barLength := 20
	completedLength := (completed * barLength) / total
	
	bar := "["
	for i := 0; i < barLength; i++ {
		if i < completedLength {
			bar += "█"
		} else {
			bar += "░"
		}
	}
	bar += "]"
	
	return bar
}

// formatStepOutput formats step output for better display
func formatStepOutput(output string) string {
	// Limit output length for readability
	maxLength := 500
	if len(output) > maxLength {
		return output[:maxLength] + "... (truncated)"
	}
	
	// Clean up common escape sequences and whitespace
	formatted := strings.TrimSpace(output)
	formatted = strings.ReplaceAll(formatted, "\\n", "\n")
	formatted = strings.ReplaceAll(formatted, "\\t", "\t")
	
	// If it looks like JSON, try to format it
	if strings.HasPrefix(formatted, "{") && strings.HasSuffix(formatted, "}") {
		var jsonObj interface{}
		if err := json.Unmarshal([]byte(formatted), &jsonObj); err == nil {
			if prettyBytes, err := json.MarshalIndent(jsonObj, "    ", "  "); err == nil {
				return string(prettyBytes)
			}
		}
	}
	
	return formatted
}

// WorkflowTrace tracks the execution of a workflow for visualization
type WorkflowTrace struct {
	Name       string        `json:"name"`
	StartTime  time.Time     `json:"start_time"`
	EndTime    *time.Time    `json:"end_time,omitempty"`
	Duration   time.Duration `json:"duration"`
	TotalSteps int           `json:"total_steps"`
	Steps      []StepTrace   `json:"steps"`
	Status     string        `json:"status"` // "running", "completed", "failed"
}

// StepTrace tracks the execution of a single step
type StepTrace struct {
	Name        string        `json:"name"`
	StartTime   *time.Time    `json:"start_time,omitempty"`
	EndTime     *time.Time    `json:"end_time,omitempty"`
	Duration    time.Duration `json:"duration"`
	Status      string        `json:"status"` // "pending", "running", "completed", "failed"
	Output      string        `json:"output,omitempty"`
	OutputVars  map[string]interface{} `json:"output_vars,omitempty"`
	Description string        `json:"description,omitempty"`
}

// NewWorkflowTrace creates a new workflow trace
func NewWorkflowTrace(name string, totalSteps int) *WorkflowTrace {
	return &WorkflowTrace{
		Name:       name,
		StartTime:  time.Now(),
		TotalSteps: totalSteps,
		Steps:      make([]StepTrace, 0, totalSteps),
		Status:     "running",
	}
}

// AddStep adds a step to the workflow trace
func (wt *WorkflowTrace) AddStep(name, description string) {
	step := StepTrace{
		Name:        name,
		Status:      "pending", 
		Description: description,
		OutputVars:  make(map[string]interface{}),
	}
	wt.Steps = append(wt.Steps, step)
}

// StartStep marks a step as started
func (wt *WorkflowTrace) StartStep(name string) {
	for i := range wt.Steps {
		if wt.Steps[i].Name == name {
			now := time.Now()
			wt.Steps[i].StartTime = &now
			wt.Steps[i].Status = "running"
			return
		}
	}
	// If step doesn't exist, add it
	now := time.Now()
	step := StepTrace{
		Name:       name,
		StartTime:  &now,
		Status:     "running",
		OutputVars: make(map[string]interface{}),
	}
	wt.Steps = append(wt.Steps, step)
}

// CompleteStep marks a step as completed
func (wt *WorkflowTrace) CompleteStep(name, status, output string) {
	for i := range wt.Steps {
		if wt.Steps[i].Name == name {
			now := time.Now()
			wt.Steps[i].EndTime = &now
			wt.Steps[i].Status = status
			wt.Steps[i].Output = output
			
			if wt.Steps[i].StartTime != nil {
				wt.Steps[i].Duration = now.Sub(*wt.Steps[i].StartTime)
			}
			return
		}
	}
}

// Complete marks the workflow as completed
func (wt *WorkflowTrace) Complete(status string) {
	now := time.Now()
	wt.EndTime = &now
	wt.Duration = now.Sub(wt.StartTime)
	wt.Status = status
}

// GenerateGraph creates a visual representation of the workflow execution
func (wt *WorkflowTrace) GenerateGraph() string {
	var graph strings.Builder
	
	graph.WriteString(fmt.Sprintf("\n%s Workflow Execution Graph\n", 
		style.HeaderStyle.Render("📊")))
	graph.WriteString(fmt.Sprintf("%s %s\n", 
		style.DimStyle.Render("Name:"), wt.Name))
	graph.WriteString(fmt.Sprintf("%s %s\n", 
		style.DimStyle.Render("Status:"), getStatusEmoji(wt.Status)))
	
	if wt.EndTime != nil {
		graph.WriteString(fmt.Sprintf("%s %v\n", 
			style.DimStyle.Render("Duration:"), wt.Duration.Round(time.Millisecond)))
	}
	
	graph.WriteString("\n")
	
	// Generate step graph
	for i, step := range wt.Steps {
		// Step connector
		if i == 0 {
			graph.WriteString("┌─")
		} else {
			graph.WriteString("├─")
		}
		
		// Step info
		statusEmoji := getStatusEmoji(step.Status)
		graph.WriteString(fmt.Sprintf(" %s %s", statusEmoji, step.Name))
		
		if step.Duration > 0 {
			graph.WriteString(fmt.Sprintf(" (%v)", step.Duration.Round(time.Millisecond)))
		}
		graph.WriteString("\n")
		
		// Show output if available and not too long
		if step.Output != "" && len(step.Output) < 100 {
			if i == len(wt.Steps)-1 {
				graph.WriteString("  └─ 📤 ")
			} else {
				graph.WriteString("│ └─ 📤 ")
			}
			graph.WriteString(style.DimStyle.Render(step.Output))
			graph.WriteString("\n")
		}
	}
	
	return graph.String()
}

// getStatusEmoji returns an emoji for the given status
func getStatusEmoji(status string) string {
	switch status {
	case "pending":
		return "⏳ Pending"
	case "running":
		return "🔄 Running"
	case "completed", "finished":
		return "✅ Completed"
	case "failed":
		return "❌ Failed"
	default:
		return "❓ " + status
	}
}

// saveWorkflowTrace saves the workflow trace to a JSON file
func saveWorkflowTrace(trace *WorkflowTrace, workflowFile string) error {
	// Generate trace filename based on workflow file and timestamp
	baseFilename := strings.TrimSuffix(workflowFile, filepath.Ext(workflowFile))
	timestamp := trace.StartTime.Format("20060102-150405")
	traceFilename := fmt.Sprintf("%s-trace-%s.json", baseFilename, timestamp)
	
	// Marshal trace to JSON
	traceData, err := json.MarshalIndent(trace, "", "  ")
	if err != nil {
		return fmt.Errorf("failed to marshal trace: %w", err)
	}
	
	// Write to file
	if err := os.WriteFile(traceFilename, traceData, 0644); err != nil {
		return fmt.Errorf("failed to write trace file: %w", err)
	}
	
	fmt.Printf("\n%s Workflow trace saved to: %s\n", 
		style.InfoStyle.Render("💾"), traceFilename)
	
	return nil
}
