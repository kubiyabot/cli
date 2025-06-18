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

func newWorkflowTestCommand(cfg *config.Config) *cobra.Command {
	var (
		runner    string
		variables []string
	)

	cmd := &cobra.Command{
		Use:   "test [workflow-file]",
		Short: "Test a workflow by executing it",
		Long: `Test a workflow by executing it and streaming the results.

This command loads a workflow from a YAML file and executes it using the Kubiya API.
It provides real-time streaming output so you can monitor the workflow execution.`,
		Example: `  # Test a workflow from file
  kubiya workflow test my-workflow.yaml

  # Test with specific runner
  kubiya workflow test deploy.yaml --runner core-testing-2

  # Test with variables
  kubiya workflow test backup.yaml --var env=staging --var bucket=backup-staging`,
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
				Description: fmt.Sprintf("Test execution of %s", workflow.Name),
				Steps:       steps,
				Variables:   vars,
				Runner:      runner,
			}

			fmt.Printf("%s Testing workflow: %s\n", style.StatusStyle.Render("🧪"), style.HighlightStyle.Render(workflow.Name))
			fmt.Printf("%s %s\n\n", style.DimStyle.Render("File:"), workflowFile)

			// Execute workflow
			events, err := client.Workflow().TestWorkflow(ctx, req, runner)
			if err != nil {
				return fmt.Errorf("failed to test workflow: %w", err)
			}

			// Process streaming events
			var hasError bool
			for event := range events {
				switch event.Type {
				case "data":
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
									if stepName, ok := step["name"].(string); ok && stepName != "" {
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
								}
							case "workflow_complete":
								if status, ok := jsonData["status"].(string); ok {
									if status == "finished" && jsonData["success"] == true {
										fmt.Printf("\n%s Workflow test completed successfully!\n", style.SuccessStyle.Render("✅"))
									} else {
										fmt.Printf("\n%s Workflow test failed\n", style.ErrorStyle.Render("❌"))
										hasError = true
									}
								}
							}
						}
					} else {
						// Plain text output
						if event.Data != "" {
							fmt.Println(event.Data)
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
				return fmt.Errorf("workflow test failed")
			}

			return nil
		},
	}

	cmd.Flags().StringVar(&runner, "runner", "", "Runner to use for execution (default: core-testing-2)")
	cmd.Flags().StringArrayVar(&variables, "var", []string{}, "Variables in key=value format")

	return cmd
}
