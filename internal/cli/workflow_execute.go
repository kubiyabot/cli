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
		runner    string
		variables []string
		watch     bool
	)

	cmd := &cobra.Command{
		Use:   "execute [workflow-file]",
		Short: "Execute a workflow from a file",
		Long: `Execute a workflow defined in a YAML file.

This command loads a workflow from a file and executes it using the Kubiya API.
You can provide variables and choose the runner for execution.`,
		Example: `  # Execute a workflow
  kubiya workflow execute deploy.yaml

  # Execute with variables
  kubiya workflow execute backup.yaml --var env=production --var retention=30

  # Execute with specific runner
  kubiya workflow execute migrate.yaml --runner prod-runner

  # Execute and watch output
  kubiya workflow execute long-running.yaml --watch`,
		Args: cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			client := kubiya.NewClient(cfg)
			ctx := context.Background()

			// Read workflow file
			workflowFile := args[0]
			data, err := os.ReadFile(workflowFile)
			if err != nil {
				return fmt.Errorf("failed to read workflow file: %w", err)
			}

			// Parse workflow
			var workflow Workflow
			if err := yaml.Unmarshal(data, &workflow); err != nil {
				return fmt.Errorf("failed to parse workflow YAML: %w", err)
			}

			// Parse variables
			vars := make(map[string]interface{})
			for _, v := range variables {
				parts := strings.SplitN(v, "=", 2)
				if len(parts) == 2 {
					vars[parts[0]] = parts[1]
				}
			}

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

			// Build execution request
			req := kubiya.WorkflowExecutionRequest{
				Name:        workflow.Name,
				Description: fmt.Sprintf("Execution of %s", workflow.Name),
				Steps:       steps,
				Variables:   vars,
				Runner:      runner,
			}

			fmt.Printf("%s Executing workflow: %s\n", style.StatusStyle.Render("🚀"), style.HighlightStyle.Render(workflow.Name))
			fmt.Printf("%s %s\n", style.DimStyle.Render("File:"), workflowFile)
			if runner != "" {
				fmt.Printf("%s %s\n", style.DimStyle.Render("Runner:"), runner)
			}
			if len(vars) > 0 {
				fmt.Printf("%s\n", style.DimStyle.Render("Variables:"))
				for k, v := range vars {
					fmt.Printf("  %s = %v\n", style.KeyStyle.Render(k), v)
				}
			}
			fmt.Println()

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
	cmd.Flags().StringArrayVar(&variables, "var", []string{}, "Variables in key=value format")
	cmd.Flags().BoolVarP(&watch, "watch", "w", true, "Watch execution output")

	return cmd
}
