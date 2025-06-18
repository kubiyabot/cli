package cli

import (
	"context"
	"fmt"
	"os"
	"strings"

	"github.com/kubiyabot/cli/internal/config"
	"github.com/kubiyabot/cli/internal/kubiya"
	"github.com/kubiyabot/cli/internal/style"
	"github.com/spf13/cobra"
)

func newWorkflowGenerateCommand(cfg *config.Config) *cobra.Command {
	var (
		output           string
		enableClaudeCode bool
		anthropicKey     string
		workflowMode     string
		conversationID   string
		variables        []string
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

			fmt.Printf("%s Generating workflow from description...\n", style.StatusStyle.Render("🤖"))
			fmt.Printf("%s %s\n\n", style.DimStyle.Render("Description:"), description)

			// Call the workflow generation API
			events, err := client.Workflow().GenerateWorkflow(ctx, description, options)
			if err != nil {
				return fmt.Errorf("failed to generate workflow: %w", err)
			}

			var generatedWorkflow string
			var inWorkflowBlock bool
			var workflowLines []string

			// Process streaming events
			for event := range events {
				switch event.Type {
				case "data":
					// Look for workflow YAML in the response
					if strings.Contains(event.Data, "```yaml") {
						inWorkflowBlock = true
						continue
					}
					if strings.Contains(event.Data, "```") && inWorkflowBlock {
						inWorkflowBlock = false
						generatedWorkflow = strings.Join(workflowLines, "\n")
						continue
					}
					if inWorkflowBlock {
						workflowLines = append(workflowLines, event.Data)
					} else {
						// Display other messages
						fmt.Print(event.Data)
					}
				case "error":
					return fmt.Errorf("generation error: %s", event.Data)
				}
			}

			if generatedWorkflow == "" {
				return fmt.Errorf("no workflow was generated")
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

	return cmd
}
