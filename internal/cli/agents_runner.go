package cli

import (
	"fmt"
	"strings"

	"github.com/kubiyabot/cli/internal/config"
	"github.com/kubiyabot/cli/internal/kubiya"
	"github.com/kubiyabot/cli/internal/style"
	"github.com/spf13/cobra"
)

// newAgentRunnerCommand creates the command to manage agent runners
func newAgentRunnerCommand(cfg *config.Config) *cobra.Command {
	var (
		yes          bool
		outputFormat string
	)

	cmd := &cobra.Command{
		Use:     "runner [agent-uuid] [runner-name]",
		Aliases: []string{"set-runner"},
		Short:   "🏃 Set agent runner",
		Long:    `Set the runner for an agent. The runner determines where the agent executes.`,
		Example: `  # Set runner to gke-poc-kubiya (common for working agents)
  kubiya agent runner abc-123 gke-poc-kubiya

  # Set runner to gke-integration
  kubiya agent runner abc-123 gke-integration

  # Set runner to kubiya-hosted  
  kubiya agent runner abc-123 kubiya-hosted

  # Set with confirmation skip
  kubiya agent runner abc-123 gke-poc-kubiya -y`,
		Args: cobra.ExactArgs(2),
		RunE: func(cmd *cobra.Command, args []string) error {
			agentUUID := args[0]
			newRunner := args[1]

			client := kubiya.NewClient(cfg)
			
			// Get current agent
			agent, err := client.GetAgent(cmd.Context(), agentUUID)
			if err != nil {
				return fmt.Errorf("failed to get agent: %w", err)
			}

			// Check if runner is already set
			currentRunners := strings.Join(agent.Runners, ", ")
			if len(agent.Runners) == 1 && agent.Runners[0] == newRunner {
				fmt.Printf("%s Agent '%s' is already using runner: %s\n",
					style.InfoStyle.Render("ℹ️"),
					style.HighlightStyle.Render(agent.Name),
					style.HighlightStyle.Render(newRunner))
				return nil
			}

			// Show what will be changed
			fmt.Printf("%s Updating runner for agent: %s\n\n", 
				style.InfoStyle.Render("🏃"), 
				style.HighlightStyle.Render(agent.Name))
			
			fmt.Printf("  Current: %s\n", 
				style.DimStyle.Render(currentRunners))
			fmt.Printf("  New:     %s\n\n", 
				style.SuccessStyle.Render(newRunner))

			// Confirm change
			if !yes {
				if !confirmYesNo(fmt.Sprintf("Update runner for agent '%s' to '%s'?", agent.Name, newRunner)) {
					return fmt.Errorf("runner update cancelled")
				}
			}

			// Create update payload
			updateData := map[string]interface{}{
				"name":                  agent.Name,
				"description":           agent.Description,
				"instruction_type":      agent.InstructionType,
				"llm_model":             agent.LLMModel,
				"sources":               agent.Sources,
				"environment_variables": agent.Environment,
				"secrets":               agent.Secrets,
				"allowed_groups":        agent.AllowedGroups,
				"allowed_users":         agent.AllowedUsers,
				"owners":                agent.Owners,
				"runners":               []string{newRunner},
				"is_debug_mode":         agent.IsDebugMode,
				"ai_instructions":       agent.AIInstructions,
				"image":                 agent.Image,
				"managed_by":            agent.ManagedBy,
				"integrations":          agent.Integrations,
				"links":                 agent.Links,
				"tools":                 agent.Tools,
				"tasks":                 agent.Tasks,
				"tags":                  agent.Tags,
			}

			// Update the agent
			result, err := client.UpdateAgentRaw(cmd.Context(), agentUUID, updateData)
			if err != nil {
				return fmt.Errorf("failed to update agent: %w", err)
			}

			fmt.Printf("%s Updated runner for agent '%s' to: %s\n",
				style.SuccessStyle.Render("✅"),
				style.HighlightStyle.Render(result.Name),
				style.HighlightStyle.Render(newRunner))

			return nil
		},
	}

	cmd.Flags().BoolVarP(&yes, "yes", "y", false, "Skip confirmation prompts")
	cmd.Flags().StringVarP(&outputFormat, "output", "o", "text", "Output format (text|json)")

	return cmd
}