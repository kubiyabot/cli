package cli

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"strings"

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

  # Execute and watch output (auto-detects format)
  kubiya workflow execute long-running --watch

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

			// Execute workflow
			events, err := client.Workflow().ExecuteWorkflow(ctx, req, runner)
			if err != nil {
				return fmt.Errorf("failed to execute workflow: %w", err)
			}

			// Process streaming events
			var hasError bool
			var executionID string
			for event := range events {
				switch event.Type {
				case "data":
					if watch {
						// Try to parse as JSON for structured output
						var jsonData map[string]interface{}
						if err := json.Unmarshal([]byte(event.Data), &jsonData); err == nil {
							// Handle different event types from the API
							if eventType, ok := jsonData["type"].(string); ok {
								switch eventType {
								case "step_running":
									if step, ok := jsonData["step"].(map[string]interface{}); ok {
										if stepName, ok := step["name"].(string); ok {
											fmt.Printf("\n%s Step: %s\n", style.BulletStyle.Render("▶"), style.ToolNameStyle.Render(stepName))
											fmt.Printf("  %s Running...\n", style.DimStyle.Render("⏳"))
										}
									}
								case "step_complete":
									if step, ok := jsonData["step"].(map[string]interface{}); ok {
										if output, ok := step["output"].(string); ok && output != "" {
											fmt.Printf("  %s Output: %s\n", style.DimStyle.Render("📤"), style.ToolOutputStyle.Render(output))
										}
										if status, ok := step["status"].(string); ok {
											if status == "finished" {
												fmt.Printf("  %s Step completed successfully\n", style.SuccessStyle.Render("✓"))
											} else if status == "failed" {
												fmt.Printf("  %s Step failed\n", style.ErrorStyle.Render("✗"))
												hasError = true
											}
										}
									}
								case "workflow_complete":
									if requestId, ok := jsonData["requestId"].(string); ok {
										executionID = requestId
									}
									if status, ok := jsonData["status"].(string); ok {
										if status == "finished" && jsonData["success"] == true {
											fmt.Printf("\n%s Workflow executed successfully!\n", style.SuccessStyle.Render("✅"))
										} else {
											fmt.Printf("\n%s Workflow execution failed\n", style.ErrorStyle.Render("❌"))
											hasError = true
										}
									}
									if executionID != "" {
										fmt.Printf("%s Execution ID: %s\n", style.DimStyle.Render("📋"), executionID)
									}
								}
							}
						} else {
							// Plain text output
							if event.Data != "" {
								fmt.Println(event.Data)
							}
						}
					}
				case "error":
					fmt.Printf("%s Error: %s\n", style.ErrorStyle.Render("✗"), event.Data)
					hasError = true
				case "done":
					// Workflow execution finished
					break
				}
			}

			if hasError {
				return fmt.Errorf("workflow execution failed")
			}

			return nil
		},
	}

	cmd.Flags().StringVar(&runner, "runner", "", "Runner to use for execution")
	cmd.Flags().StringArrayVar(&variables, "var", []string{}, "Params in key=value format")
	cmd.Flags().BoolVarP(&watch, "watch", "w", true, "Watch execution output")
	cmd.Flags().BoolVar(&skipPolicyCheck, "skip-policy-check", false, "Skip policy validation before execution")

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
