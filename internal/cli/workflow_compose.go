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

func newWorkflowComposeCommand(cfg *config.Config) *cobra.Command {
	var (
		workflowMode     string
		conversationID   string
		variables        []string
		enableClaudeCode bool
		anthropicKey     string
		orgID            string
		userID           string
		output           string
		watch            bool
	)

	cmd := &cobra.Command{
		Use:   "compose [description]",
		Short: "Compose and execute a workflow from natural language",
		Long: `Compose and execute a workflow from a natural language description.

This command uses the Kubiya orchestration API to generate and immediately execute
a workflow based on your description. It combines generation and execution in one step.`,
		Example: `  # Compose and execute a simple workflow
  kubiya workflow compose "check kubernetes cluster health and send results to slack"

  # Compose with act mode (direct execution)
  kubiya workflow compose "restart the web service" --mode act

  # Compose with plan mode (terraform-like planning)
  kubiya workflow compose "deploy application to production" --mode plan

  # Compose with conversation context
  kubiya workflow compose "now scale it to 5 replicas" --conversation-id abc123

  # Compose with variables
  kubiya workflow compose "backup database {{db_name}}" --var db_name=users

  # Compose with Claude Code enabled
  kubiya workflow compose "analyze logs and create report" --enable-claude-code`,
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
				WorkflowMode:     workflowMode,
				ConversationID:   conversationID,
				Variables:        vars,
				EnableClaudeCode: enableClaudeCode,
				AnthropicAPIKey:  anthropicKey,
				OrgID:            orgID,
				UserID:           userID,
			}

			fmt.Printf("%s Composing and executing workflow...\n", style.StatusStyle.Render("🎼"))
			fmt.Printf("%s %s\n", style.DimStyle.Render("Description:"), description)
			fmt.Printf("%s %s\n", style.DimStyle.Render("Mode:"), style.HighlightStyle.Render(workflowMode))
			if conversationID != "" {
				fmt.Printf("%s %s\n", style.DimStyle.Render("Conversation:"), conversationID)
			}
			if len(vars) > 0 {
				fmt.Printf("%s\n", style.DimStyle.Render("Variables:"))
				for k, v := range vars {
					fmt.Printf("  %s = %v\n", style.KeyStyle.Render(k), v)
				}
			}
			fmt.Println()

			// Call the orchestration API
			events, err := client.Workflow().ComposeAndExecute(ctx, description, options)
			if err != nil {
				return fmt.Errorf("failed to compose workflow: %w", err)
			}

			var hasError bool
			var workflowContent strings.Builder
			var inWorkflowBlock bool
			var outputBuffer strings.Builder

			// Process streaming events
			for event := range events {
				switch event.Type {
				case "data":
					data := event.Data

					// Capture workflow YAML if requested
					if output != "" {
						if strings.Contains(data, "```yaml") {
							inWorkflowBlock = true
							continue
						}
						if strings.Contains(data, "```") && inWorkflowBlock {
							inWorkflowBlock = false
							continue
						}
						if inWorkflowBlock {
							workflowContent.WriteString(data)
							workflowContent.WriteString("\n")
						}
					}

					// Display output if watching
					if watch {
						// Check for special markers in the data
						if strings.Contains(data, "🔧") || strings.Contains(data, "Tool:") {
							fmt.Printf("\n%s %s\n", style.ToolStyle.Render("🔧"), data)
						} else if strings.Contains(data, "✅") || strings.Contains(data, "success") {
							fmt.Printf("%s %s\n", style.SuccessStyle.Render("✓"), data)
						} else if strings.Contains(data, "❌") || strings.Contains(data, "error") || strings.Contains(data, "failed") {
							fmt.Printf("%s %s\n", style.ErrorStyle.Render("✗"), data)
							hasError = true
						} else if strings.Contains(data, "📊") || strings.Contains(data, "Result:") {
							fmt.Printf("\n%s\n", style.HeadingStyle.Render("Results"))
							fmt.Printf("%s\n", data)
						} else {
							fmt.Print(data)
						}
					} else {
						outputBuffer.WriteString(data)
					}

				case "error":
					fmt.Printf("\n%s Error: %s\n", style.ErrorStyle.Render("✗"), event.Data)
					hasError = true

				case "tool_call":
					if watch {
						fmt.Printf("\n%s Executing tool...\n", style.ToolStyle.Render("🔧"))
					}

				case "complete", "done":
					if !hasError {
						fmt.Printf("\n\n%s Workflow composed and executed successfully!\n", style.SuccessStyle.Render("✅"))
					} else {
						fmt.Printf("\n\n%s Workflow execution completed with errors\n", style.ErrorStyle.Render("❌"))
					}
				}
			}

			// Save workflow if output file specified
			if output != "" && workflowContent.Len() > 0 {
				if err := os.WriteFile(output, []byte(workflowContent.String()), 0644); err != nil {
					return fmt.Errorf("failed to save workflow: %w", err)
				}
				fmt.Printf("\n%s Workflow saved to: %s\n",
					style.DimStyle.Render("💾"),
					style.HighlightStyle.Render(output))
			}

			// If not watching, show the complete output at the end
			if !watch && outputBuffer.Len() > 0 {
				fmt.Printf("\n%s\n", style.HeadingStyle.Render("Execution Output"))
				fmt.Println(outputBuffer.String())
			}

			if hasError {
				return fmt.Errorf("workflow execution failed")
			}

			return nil
		},
	}

	cmd.Flags().StringVar(&workflowMode, "mode", "act", "Workflow mode: 'act' (direct execution) or 'plan' (with planning)")
	cmd.Flags().StringVar(&conversationID, "conversation-id", "", "Conversation ID for context continuity")
	cmd.Flags().StringArrayVar(&variables, "var", []string{}, "Variables in key=value format")
	cmd.Flags().BoolVar(&enableClaudeCode, "enable-claude-code", false, "Enable Claude Code for advanced capabilities")
	cmd.Flags().StringVar(&anthropicKey, "anthropic-api-key", "", "Anthropic API key for Claude Code")
	cmd.Flags().StringVar(&orgID, "org-id", "", "Organization ID")
	cmd.Flags().StringVar(&userID, "user-id", "", "User ID")
	cmd.Flags().StringVarP(&output, "output", "o", "", "Save generated workflow to file")
	cmd.Flags().BoolVarP(&watch, "watch", "w", true, "Watch execution output in real-time")

	return cmd
}
