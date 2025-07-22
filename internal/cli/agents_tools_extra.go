package cli

import (
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

// newAgentToolRemoveCommand creates the command to remove tools from an agent
func newAgentToolRemoveCommand(cfg *config.Config) *cobra.Command {
	var (
		yes          bool
		outputFormat string
	)

	cmd := &cobra.Command{
		Use:     "remove [agent-uuid] [tool-name...]",
		Aliases: []string{"rm", "delete", "del"},
		Short:   "🗑️ Remove tool(s) from an agent",
		Long:    `Remove one or more tools from an agent.`,
		Example: `  # Remove single tool
  kubiya agent tool remove abc-123 python_script_runner

  # Remove multiple tools
  kubiya agent tool remove abc-123 tool1 tool2 tool3

  # Remove with confirmation skip
  kubiya agent tool remove abc-123 unwanted_tool -y`,
		Args: cobra.MinimumNArgs(2),
		RunE: func(cmd *cobra.Command, args []string) error {
			agentUUID := args[0]
			toolsToRemove := args[1:]

			client := kubiya.NewClient(cfg)
			
			// Get current agent
			agent, err := client.GetAgent(cmd.Context(), agentUUID)
			if err != nil {
				return fmt.Errorf("failed to get agent: %w", err)
			}

			// Check which tools exist on the agent
			var validTools []string
			var notFound []string
			
			for _, toolToRemove := range toolsToRemove {
				found := false
				for _, existingTool := range agent.Tools {
					if existingTool == toolToRemove {
						validTools = append(validTools, toolToRemove)
						found = true
						break
					}
				}
				if !found {
					notFound = append(notFound, toolToRemove)
				}
			}

			// Report tools not found
			if len(notFound) > 0 {
				fmt.Printf("%s Tools not found on agent: %s\n", 
					style.WarningStyle.Render("⚠️"), 
					strings.Join(notFound, ", "))
			}

			// Exit if no valid tools to remove
			if len(validTools) == 0 {
				return fmt.Errorf("no valid tools found to remove")
			}

			// Show what will be removed
			fmt.Printf("%s Removing tools from agent: %s\n\n", 
				style.InfoStyle.Render("🗑️"), 
				style.HighlightStyle.Render(agent.Name))
			
			for _, tool := range validTools {
				fmt.Printf("  • %s\n", style.ErrorStyle.Render(tool))
			}
			fmt.Println()

			// Confirm removal
			if !yes {
				if !confirmYesNo(fmt.Sprintf("Remove %d tool(s) from agent '%s'?", len(validTools), agent.Name)) {
					return fmt.Errorf("tool removal cancelled")
				}
			}

			// Remove tools from agent
			var updatedTools []string
			for _, existingTool := range agent.Tools {
				shouldRemove := false
				for _, toolToRemove := range validTools {
					if existingTool == toolToRemove {
						shouldRemove = true
						break
					}
				}
				if !shouldRemove {
					updatedTools = append(updatedTools, existingTool)
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
				"runners":               agent.Runners,
				"is_debug_mode":         agent.IsDebugMode,
				"ai_instructions":       agent.AIInstructions,
				"image":                 agent.Image,
				"managed_by":            agent.ManagedBy,
				"integrations":          agent.Integrations,
				"links":                 agent.Links,
				"tools":                 updatedTools,
				"tasks":                 agent.Tasks,
				"tags":                  agent.Tags,
			}

			// Update the agent
			result, err := client.UpdateAgentRaw(cmd.Context(), agentUUID, updateData)
			if err != nil {
				return fmt.Errorf("failed to update agent: %w", err)
			}

			fmt.Printf("%s Removed %d tool(s) from agent '%s'\n\n",
				style.SuccessStyle.Render("✅"),
				len(validTools),
				style.HighlightStyle.Render(result.Name))

			// Show updated tools count
			fmt.Printf("%s Agent now has %d tools\n",
				style.InfoStyle.Render("📊"),
				len(result.Tools))

			return nil
		},
	}

	cmd.Flags().BoolVarP(&yes, "yes", "y", false, "Skip confirmation prompts")
	cmd.Flags().StringVarP(&outputFormat, "output", "o", "text", "Output format (text|json)")

	return cmd
}

// newAgentToolDescribeCommand creates the command to describe/show tool details
func newAgentToolDescribeCommand(cfg *config.Config) *cobra.Command {
	var outputFormat string

	cmd := &cobra.Command{
		Use:     "describe [agent-uuid] [tool-name]",
		Aliases: []string{"desc", "get", "show", "info"},
		Short:   "📄 Describe a specific tool on an agent",
		Long:    `Show detailed information about a specific tool attached to an agent.`,
		Example: `  # Describe a tool
  kubiya agent tool describe abc-123 python_script_runner

  # Get tool info in JSON format
  kubiya agent tool get abc-123 create_file --output json`,
		Args: cobra.ExactArgs(2),
		RunE: func(cmd *cobra.Command, args []string) error {
			agentUUID := args[0]
			toolName := args[1]

			client := kubiya.NewClient(cfg)
			
			// Get current agent
			agent, err := client.GetAgent(cmd.Context(), agentUUID)
			if err != nil {
				return fmt.Errorf("failed to get agent: %w", err)
			}

			// Check if tool exists on agent
			toolExists := false
			for _, existingTool := range agent.Tools {
				if existingTool == toolName {
					toolExists = true
					break
				}
			}

			if !toolExists {
				return fmt.Errorf("tool '%s' not found on agent '%s'", toolName, agent.Name)
			}

			switch outputFormat {
			case "json":
				toolInfo := map[string]interface{}{
					"name":       toolName,
					"agent_uuid": agentUUID,
					"agent_name": agent.Name,
					"attached":   true,
				}
				return json.NewEncoder(os.Stdout).Encode(toolInfo)
			case "yaml":
				toolInfo := map[string]interface{}{
					"name":       toolName,
					"agent_uuid": agentUUID,
					"agent_name": agent.Name,
					"attached":   true,
				}
				return yaml.NewEncoder(os.Stdout).Encode(toolInfo)
			default:
				fmt.Printf("%s Tool Details\n\n", style.TitleStyle.Render("📄"))
				fmt.Printf("Tool Name: %s\n", style.HighlightStyle.Render(toolName))
				fmt.Printf("Agent: %s (%s)\n", style.HighlightStyle.Render(agent.Name), agent.UUID)
				fmt.Printf("Status: %s\n", style.SuccessStyle.Render("✅ Attached"))
				
				// Show position in tools list
				for i, tool := range agent.Tools {
					if tool == toolName {
						fmt.Printf("Position: %s of %d\n", style.InfoStyle.Render(fmt.Sprintf("#%d", i+1)), len(agent.Tools))
						break
					}
				}
				
				fmt.Printf("\n%s Tool Management Commands:\n", style.SubtitleStyle.Render("🛠️"))
				fmt.Printf("  • Remove: %s\n", style.DimStyle.Render(fmt.Sprintf("kubiya agent tool remove %s %s", agentUUID, toolName)))
				fmt.Printf("  • List all: %s\n", style.DimStyle.Render(fmt.Sprintf("kubiya agent tools list %s", agentUUID)))
				
			}

			return nil
		},
	}

	cmd.Flags().StringVarP(&outputFormat, "output", "o", "text", "Output format (text|json|yaml)")

	return cmd
}