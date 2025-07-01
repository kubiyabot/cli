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

func newWorkflowGenerateCommand(cfg *config.Config) *cobra.Command {
	var (
		output           string
		enableClaudeCode bool
		anthropicKey     string
		workflowMode     string
		conversationID   string
		variables        []string
		generateOnly     bool
	)

	cmd := &cobra.Command{
		Use:   "generate [description]",
		Short: "Generate a workflow from natural language description",
		Long: `Generate a workflow definition from a natural language description using AI.

This command uses the Kubiya orchestration API to generate a workflow based on your description.
The generated workflow can be saved to a file for later execution or modification.`,
		Example: `  # Generate a workflow to deploy an application
  kubiya workflow generate "create a workflow to deploy my app to kubernetes"

  # Generate and save to file
  kubiya workflow generate "backup database and upload to S3" -o backup-workflow.yaml

  # Generate with Claude Code enabled
  kubiya workflow generate "complex data processing pipeline" --enable-claude-code`,
		Args: cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			client := kubiya.NewClient(cfg)
			ctx := context.Background()

			description := args[0]

			// Parse variables
			vars := make(map[string]interface{})
			for _, v := range variables {
				parts := strings.SplitN(v, "=", 2)
				if len(parts) == 2 {
					vars[parts[0]] = parts[1]
				}
			}

			// Build orchestration request
			options := kubiya.OrchestrateRequest{
				Format:           "sse",
				EnableClaudeCode: enableClaudeCode,
				AnthropicAPIKey:  anthropicKey,
				WorkflowMode:     workflowMode,
				ConversationID:   conversationID,
				Variables:        vars,
			}

			// Set workflow mode to "plan" if generate-only
			if generateOnly {
				options.WorkflowMode = "plan"
			}

			fmt.Printf("%s Generating workflow from description...\n", style.StatusStyle.Render("🤖"))
			fmt.Printf("%s %s\n\n", style.DimStyle.Render("Description:"), description)

			// Call the workflow generation API
			events, err := client.Workflow().GenerateWorkflow(ctx, description, options)
			if err != nil {
				// Check if it's a 404 error or timeout and offer local generation
				if strings.Contains(err.Error(), "orchestration API not found") ||
					strings.Contains(err.Error(), "orchestration API timeout") ||
					strings.Contains(err.Error(), "deadline exceeded") ||
					strings.Contains(err.Error(), "connection refused") ||
					strings.Contains(err.Error(), "INTERNAL_ERROR") {
					fmt.Printf("\n%s Orchestration API not available. Generating a template workflow locally...\n\n", style.WarningStyle.Render("⚠️"))

					// Generate a basic workflow template based on the description
					workflow := generateLocalWorkflowTemplate(description, vars)

					// Convert to YAML
					yamlData, err := yaml.Marshal(workflow)
					if err != nil {
						return fmt.Errorf("failed to generate workflow template: %w", err)
					}

					generatedWorkflow := string(yamlData)

					// Save to file if output is specified
					if output != "" {
						if err := os.WriteFile(output, yamlData, 0644); err != nil {
							return fmt.Errorf("failed to write workflow file: %w", err)
						}
						fmt.Printf("%s Workflow template saved to: %s\n",
							style.SuccessStyle.Render("✅"),
							style.HighlightStyle.Render(output))

						// Show next steps
						fmt.Printf("\n%s\n", style.SubtitleStyle.Render("Next Steps"))
						fmt.Printf("1. Edit the workflow template: %s\n", style.CommandStyle.Render(fmt.Sprintf("vi %s", output)))
						fmt.Printf("2. Test the workflow: %s\n", style.CommandStyle.Render(fmt.Sprintf("kubiya workflow test %s", output)))
						fmt.Printf("3. Execute the workflow: %s\n", style.CommandStyle.Render(fmt.Sprintf("kubiya workflow execute %s", output)))
					} else {
						// Display the generated workflow
						fmt.Printf("%s\n", style.SubtitleStyle.Render("Generated Workflow Template"))
						fmt.Printf("```yaml\n%s```\n", generatedWorkflow)
						fmt.Printf("\n%s This is a basic template. Please customize it according to your needs.\n", style.DimStyle.Render("Note:"))
					}

					return nil
				}
				return fmt.Errorf("failed to generate workflow: %w", err)
			}

			var generatedWorkflow string
			var inWorkflowBlock bool
			var workflowLines []string
			var hasWorkflow bool

			// Process streaming events
			for event := range events {
				switch event.Type {
				case "data":
					// Debug: print raw data when in verbose mode
					if strings.Contains(os.Getenv("KUBIYA_DEBUG"), "true") {
						fmt.Printf("[DEBUG] Event data: %s\n", event.Data)
					}

					// Look for workflow YAML or JSON in the response
					if strings.Contains(event.Data, "<workflow>") {
						inWorkflowBlock = true
						// Check if the workflow is on the same line
						if endIdx := strings.Index(event.Data, "</workflow>"); endIdx != -1 {
							// Extract workflow content from the same line
							startIdx := strings.Index(event.Data, "<workflow>") + len("<workflow>")
							workflowContent := strings.TrimSpace(event.Data[startIdx:endIdx])
							if workflowContent != "" {
								// Clean up any escape characters
								workflowContent = strings.ReplaceAll(workflowContent, "\\n", "\n")
								workflowContent = strings.ReplaceAll(workflowContent, "\\\"", "\"")
								workflowContent = strings.ReplaceAll(workflowContent, "\\\\", "\\")
								generatedWorkflow = workflowContent
								hasWorkflow = true
							}
							inWorkflowBlock = false
						}
						continue
					}
					if strings.Contains(event.Data, "</workflow>") && inWorkflowBlock {
						inWorkflowBlock = false
						// Join and clean up the workflow content
						generatedWorkflow = strings.Join(workflowLines, "\n")
						// Try to extract JSON from the workflow block
						if strings.TrimSpace(generatedWorkflow) != "" {
							// Clean up any escape characters
							generatedWorkflow = strings.ReplaceAll(generatedWorkflow, "\\n", "\n")
							generatedWorkflow = strings.ReplaceAll(generatedWorkflow, "\\\"", "\"")
							generatedWorkflow = strings.ReplaceAll(generatedWorkflow, "\\\\", "\\")
							hasWorkflow = true
						}
						workflowLines = []string{} // Reset
						continue
					}
					if inWorkflowBlock {
						workflowLines = append(workflowLines, event.Data)
					} else if strings.Contains(event.Data, "```yaml") {
						inWorkflowBlock = true
						hasWorkflow = true
						continue
					} else if strings.Contains(event.Data, "```") && inWorkflowBlock {
						inWorkflowBlock = false
						generatedWorkflow = strings.Join(workflowLines, "\n")
						continue
					} else if inWorkflowBlock {
						workflowLines = append(workflowLines, event.Data)
					} else {
						// Display other messages
						fmt.Print(event.Data)
					}
				case "error":
					// Don't immediately return error if we have a workflow
					if !hasWorkflow {
						// Check if error contains workflow data or is just a completion message
						if strings.Contains(event.Data, `"finishReason"`) {
							// This is a completion message, not an error
							// If we have a workflow, we're done successfully
							if generatedWorkflow != "" {
								hasWorkflow = true
							}
							continue
						}
						return fmt.Errorf("generation error: %s", event.Data)
					}
					fmt.Printf("\n%s Note: Workflow was generated but execution failed: %s\n",
						style.WarningStyle.Render("⚠️"), event.Data)
				}
			}

			// If we captured a workflow, use it
			if generatedWorkflow == "" && len(workflowLines) > 0 {
				generatedWorkflow = strings.Join(workflowLines, "\n")
			}

			if generatedWorkflow == "" {
				return fmt.Errorf("no workflow was generated")
			}

			// Convert JSON workflow to YAML if needed
			if strings.TrimSpace(generatedWorkflow)[0] == '{' {
				// Parse JSON workflow
				var jsonWorkflow map[string]interface{}
				if err := json.Unmarshal([]byte(generatedWorkflow), &jsonWorkflow); err != nil {
					return fmt.Errorf("failed to parse generated workflow JSON: %w", err)
				}

				// Convert to YAML
				yamlData, err := yaml.Marshal(jsonWorkflow)
				if err != nil {
					return fmt.Errorf("failed to convert workflow to YAML: %w", err)
				}
				generatedWorkflow = string(yamlData)
			}

			// Save to file if output is specified
			if output != "" {
				if err := os.WriteFile(output, []byte(generatedWorkflow), 0644); err != nil {
					return fmt.Errorf("failed to write workflow file: %w", err)
				}
				fmt.Printf("\n\n%s Workflow saved to: %s\n",
					style.SuccessStyle.Render("✅"),
					style.HighlightStyle.Render(output))

				// Show next steps
				fmt.Printf("\n%s\n", style.SubtitleStyle.Render("Next Steps"))
				fmt.Printf("1. Review and edit the workflow: %s\n", style.CommandStyle.Render(fmt.Sprintf("cat %s", output)))
				fmt.Printf("2. Test the workflow: %s\n", style.CommandStyle.Render(fmt.Sprintf("kubiya workflow test %s", output)))
				fmt.Printf("3. Execute the workflow: %s\n", style.CommandStyle.Render(fmt.Sprintf("kubiya workflow execute %s", output)))
			} else {
				// Display the generated workflow
				fmt.Printf("\n\n%s\n", style.SubtitleStyle.Render("Generated Workflow"))
				fmt.Printf("```yaml\n%s\n```\n", generatedWorkflow)
			}

			return nil
		},
	}

	cmd.Flags().StringVarP(&output, "output", "o", "", "Output file for the generated workflow")
	cmd.Flags().BoolVar(&enableClaudeCode, "enable-claude-code", false, "Enable Claude Code for advanced capabilities")
	cmd.Flags().StringVar(&anthropicKey, "anthropic-api-key", "", "Anthropic API key for Claude Code")
	cmd.Flags().StringVar(&workflowMode, "mode", "plan", "Workflow mode: 'act' or 'plan'")
	cmd.Flags().StringVar(&conversationID, "conversation-id", "", "Conversation ID for context continuity")
	cmd.Flags().StringArrayVar(&variables, "var", []string{}, "Variables in key=value format")
	cmd.Flags().BoolVar(&generateOnly, "generate-only", false, "Only generate the workflow without executing it")

	return cmd
}

// generateLocalWorkflowTemplate creates a basic workflow template based on the description
func generateLocalWorkflowTemplate(description string, vars map[string]interface{}) Workflow {
	// Normalize description for analysis
	desc := strings.ToLower(description)

	// Create workflow name from description
	name := strings.ReplaceAll(description, " ", "-")
	if len(name) > 50 {
		name = name[:50]
	}
	name = strings.Trim(name, "-")

	steps := []WorkflowStep{}

	// Detect common patterns and create appropriate steps
	if strings.Contains(desc, "deploy") {
		if strings.Contains(desc, "k8s") || strings.Contains(desc, "kubernetes") {
			steps = append(steps, WorkflowStep{
				Name:        "validate-deployment",
				Description: "Validate Kubernetes deployment configuration",
				Command:     "kubectl --dry-run=client -f deployment.yaml",
				Output:      "VALIDATION_RESULT",
			})
			steps = append(steps, WorkflowStep{
				Name:        "apply-deployment",
				Description: "Apply Kubernetes deployment",
				Command:     "kubectl apply -f deployment.yaml",
				Output:      "DEPLOYMENT_RESULT",
				Depends:     []string{"validate-deployment"},
			})
			steps = append(steps, WorkflowStep{
				Name:        "check-rollout-status",
				Description: "Check deployment rollout status",
				Command:     "kubectl rollout status deployment/{{.app_name}}",
				Output:      "ROLLOUT_STATUS",
				Depends:     []string{"apply-deployment"},
			})
		} else if strings.Contains(desc, "docker") {
			steps = append(steps, WorkflowStep{
				Name:        "build-image",
				Description: "Build Docker image",
				Command:     "docker build -t {{.image_name}}:{{.tag}} .",
				Output:      "BUILD_RESULT",
			})
			steps = append(steps, WorkflowStep{
				Name:        "push-image",
				Description: "Push Docker image to registry",
				Command:     "docker push {{.image_name}}:{{.tag}}",
				Output:      "PUSH_RESULT",
				Depends:     []string{"build-image"},
			})
		} else {
			// Generic deployment steps
			steps = append(steps, WorkflowStep{
				Name:        "prepare-deployment",
				Description: "Prepare deployment environment",
				Command:     "echo 'Preparing deployment for {{.app_name}}'",
				Output:      "PREP_RESULT",
			})
			steps = append(steps, WorkflowStep{
				Name:        "deploy-application",
				Description: "Deploy the application",
				Command:     "echo 'Deploying {{.app_name}} to {{.environment}}'",
				Output:      "DEPLOY_RESULT",
				Depends:     []string{"prepare-deployment"},
			})
		}
	} else if strings.Contains(desc, "backup") {
		steps = append(steps, WorkflowStep{
			Name:        "create-backup",
			Description: "Create backup",
			Command:     "echo 'Creating backup of {{.target}}'",
			Output:      "BACKUP_FILE",
		})
		steps = append(steps, WorkflowStep{
			Name:        "upload-backup",
			Description: "Upload backup to storage",
			Command:     "echo 'Uploading backup to {{.storage_location}}'",
			Output:      "UPLOAD_RESULT",
			Depends:     []string{"create-backup"},
		})
	} else if strings.Contains(desc, "test") || strings.Contains(desc, "check") {
		steps = append(steps, WorkflowStep{
			Name:        "run-tests",
			Description: "Run tests or checks",
			Command:     "echo 'Running tests/checks'",
			Output:      "TEST_RESULT",
		})
		steps = append(steps, WorkflowStep{
			Name:        "report-results",
			Description: "Report test results",
			Command:     "echo 'Test results: ${TEST_RESULT}'",
			Depends:     []string{"run-tests"},
		})
	} else {
		// Default generic steps
		steps = append(steps, WorkflowStep{
			Name:        "step-1",
			Description: "First step - customize this",
			Command:     "echo 'Executing step 1'",
			Output:      "STEP1_OUTPUT",
		})
		steps = append(steps, WorkflowStep{
			Name:        "step-2",
			Description: "Second step - customize this",
			Command:     "echo 'Step 1 output: ${STEP1_OUTPUT}'",
			Depends:     []string{"step-1"},
		})
	}

	// Add notification step if mentioned
	if strings.Contains(desc, "notify") || strings.Contains(desc, "slack") || strings.Contains(desc, "email") {
		notifyStep := WorkflowStep{
			Name:        "send-notification",
			Description: "Send notification about workflow completion",
		}
		if strings.Contains(desc, "slack") {
			notifyStep.Executor = WorkflowExecutor{
				Type: "agent",
				Config: map[string]interface{}{
					"agent_name": "{{.agent_name}}",
					"message":       "Send workflow completion notification to Slack channel {{.slack_channel}}",
				},
			}
		} else {
			notifyStep.Command = "echo 'Workflow completed successfully'"
		}
		// Make it depend on all previous steps
		if len(steps) > 0 {
			notifyStep.Depends = []string{steps[len(steps)-1].Name}
		}
		steps = append(steps, notifyStep)
	}

	return Workflow{
		Name:  name,
		Steps: steps,
	}
}
