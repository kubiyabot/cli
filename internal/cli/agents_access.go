package cli

import (
	"encoding/json"
	"fmt"
	"os"

	"github.com/kubiyabot/cli/internal/config"
	"github.com/kubiyabot/cli/internal/kubiya"
	"github.com/kubiyabot/cli/internal/style"
	"github.com/spf13/cobra"
)

// newAgentAccessCommand creates the command to manage agent access control
func newAgentAccessCommand(cfg *config.Config) *cobra.Command {
	cmd := &cobra.Command{
		Use:     "access",
		Aliases: []string{"permissions", "acl"},
		Short:   "🔐 Manage agent access control",
		Long:    `Manage access control settings for an agent, including allowed users and groups.`,
	}

	cmd.AddCommand(
		newAgentAccessShowCommand(cfg),
		newAgentAccessClearCommand(cfg),
		newAgentAccessAddUserCommand(cfg),
		newAgentAccessRemoveUserCommand(cfg),
		newAgentAccessAddGroupCommand(cfg),
		newAgentAccessRemoveGroupCommand(cfg),
	)

	return cmd
}

// newAgentAccessShowCommand shows current access control settings
func newAgentAccessShowCommand(cfg *config.Config) *cobra.Command {
	var outputFormat string

	cmd := &cobra.Command{
		Use:     "show [agent-uuid]",
		Aliases: []string{"get", "list"},
		Short:   "👁️ Show agent access control settings",
		Long:    `Display current access control settings for an agent.`,
		Args:    cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			agentUUID := args[0]
			client := kubiya.NewClient(cfg)

			agent, err := client.GetAgent(cmd.Context(), agentUUID)
			if err != nil {
				return fmt.Errorf("failed to get agent: %w", err)
			}

			switch outputFormat {
			case "json":
				accessInfo := map[string]interface{}{
					"agent_uuid":     agent.UUID,
					"agent_name":     agent.Name,
					"owners":         agent.Owners,
					"allowed_users":  agent.AllowedUsers,
					"allowed_groups": agent.AllowedGroups,
				}
				return json.NewEncoder(os.Stdout).Encode(accessInfo)
			default:
				fmt.Printf("%s Access Control: %s\n\n",
					style.TitleStyle.Render("🔐"),
					style.HighlightStyle.Render(agent.Name))

				fmt.Printf("%s Owners:\n", style.SubtitleStyle.Render("👑"))
				if len(agent.Owners) == 0 {
					fmt.Printf("  %s No owners configured\n", style.DimStyle.Render("•"))
				} else {
					for i, owner := range agent.Owners {
						fmt.Printf("  %s %s\n", 
							style.InfoStyle.Render(fmt.Sprintf("%d.", i+1)),
							style.HighlightStyle.Render(owner))
					}
				}
				fmt.Println()

				fmt.Printf("%s Allowed Users:\n", style.SubtitleStyle.Render("👤"))
				if len(agent.AllowedUsers) == 0 {
					fmt.Printf("  %s No specific users (open access or group-based)\n", style.DimStyle.Render("•"))
				} else {
					for i, user := range agent.AllowedUsers {
						fmt.Printf("  %s %s\n", 
							style.InfoStyle.Render(fmt.Sprintf("%d.", i+1)),
							style.HighlightStyle.Render(user))
					}
				}
				fmt.Println()

				fmt.Printf("%s Allowed Groups:\n", style.SubtitleStyle.Render("👥"))
				if len(agent.AllowedGroups) == 0 {
					fmt.Printf("  %s No group restrictions (open access)\n", style.SuccessStyle.Render("•"))
				} else {
					for i, group := range agent.AllowedGroups {
						fmt.Printf("  %s %s\n", 
							style.InfoStyle.Render(fmt.Sprintf("%d.", i+1)),
							style.WarningStyle.Render(group))
					}
				}

				fmt.Printf("\n%s Access Status: ", style.SubtitleStyle.Render("🚦"))
				if len(agent.AllowedGroups) == 0 && len(agent.AllowedUsers) == 0 {
					fmt.Printf("%s (anyone can use this agent)\n", style.SuccessStyle.Render("Open Access"))
				} else {
					fmt.Printf("%s (only specific users/groups can use this agent)\n", style.WarningStyle.Render("Restricted"))
				}
			}

			return nil
		},
	}

	cmd.Flags().StringVarP(&outputFormat, "output", "o", "text", "Output format (text|json)")
	return cmd
}

// newAgentAccessClearCommand clears all access restrictions
func newAgentAccessClearCommand(cfg *config.Config) *cobra.Command {
	var yes bool

	cmd := &cobra.Command{
		Use:   "clear [agent-uuid]",
		Short: "🔓 Clear all access restrictions (make agent publicly accessible)",
		Long: `Clear all user and group access restrictions, making the agent accessible to anyone.
This sets both allowed_users and allowed_groups to empty arrays.`,
		Args: cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			agentUUID := args[0]
			client := kubiya.NewClient(cfg)

			agent, err := client.GetAgent(cmd.Context(), agentUUID)
			if err != nil {
				return fmt.Errorf("failed to get agent: %w", err)
			}

			// Check if already open
			if len(agent.AllowedGroups) == 0 && len(agent.AllowedUsers) == 0 {
				fmt.Printf("%s Agent '%s' already has open access\n",
					style.InfoStyle.Render("ℹ️"),
					style.HighlightStyle.Render(agent.Name))
				return nil
			}

			// Show what will be cleared
			fmt.Printf("%s Clearing access restrictions for agent: %s\n\n", 
				style.InfoStyle.Render("🔓"), 
				style.HighlightStyle.Render(agent.Name))
			
			if len(agent.AllowedUsers) > 0 {
				fmt.Printf("%s Removing %d allowed users\n", 
					style.WarningStyle.Render("👤"),
					len(agent.AllowedUsers))
			}
			if len(agent.AllowedGroups) > 0 {
				fmt.Printf("%s Removing %d allowed groups\n", 
					style.WarningStyle.Render("👥"),
					len(agent.AllowedGroups))
			}
			fmt.Printf("%s Agent will become publicly accessible\n\n",
				style.SuccessStyle.Render("🌐"))

			// Confirm action
			if !yes {
				if !confirmYesNo(fmt.Sprintf("Clear all access restrictions for agent '%s'?", agent.Name)) {
					return fmt.Errorf("access restriction clearing cancelled")
				}
			}

			// Create update payload with empty access lists
			updateData := map[string]interface{}{
				"name":                  agent.Name,
				"description":           agent.Description,
				"instruction_type":      agent.InstructionType,
				"llm_model":             agent.LLMModel,
				"sources":               agent.Sources,
				"environment_variables": agent.Environment,
				"secrets":               agent.Secrets,
				"allowed_groups":        []string{},
				"allowed_users":         []string{},
				"owners":                agent.Owners,
				"runners":               agent.Runners,
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

			fmt.Printf("%s Cleared all access restrictions for agent '%s'\n",
				style.SuccessStyle.Render("✅"),
				style.HighlightStyle.Render(result.Name))
			fmt.Printf("%s Agent is now publicly accessible\n",
				style.SuccessStyle.Render("🌐"))

			return nil
		},
	}

	cmd.Flags().BoolVarP(&yes, "yes", "y", false, "Skip confirmation prompts")
	return cmd
}

// Helper commands for adding/removing users and groups (simplified implementations)

func newAgentAccessAddUserCommand(cfg *config.Config) *cobra.Command {
	var yes bool

	return &cobra.Command{
		Use:   "add-user [agent-uuid] [user-id...]",
		Short: "➕ Add allowed users to agent",
		Args:  cobra.MinimumNArgs(2),
		RunE: func(cmd *cobra.Command, args []string) error {
			agentUUID := args[0]
			usersToAdd := args[1:]

			client := kubiya.NewClient(cfg)
			agent, err := client.GetAgent(cmd.Context(), agentUUID)
			if err != nil {
				return fmt.Errorf("failed to get agent: %w", err)
			}

			// Add users (with duplicate checking)
			updatedUsers := append([]string{}, agent.AllowedUsers...)
			var newUsers []string
			
			for _, user := range usersToAdd {
				found := false
				for _, existing := range updatedUsers {
					if existing == user {
						found = true
						break
					}
				}
				if !found {
					updatedUsers = append(updatedUsers, user)
					newUsers = append(newUsers, user)
				}
			}

			if len(newUsers) == 0 {
				return fmt.Errorf("no new users to add")
			}

			if !yes {
				if !confirmYesNo(fmt.Sprintf("Add %d user(s) to agent '%s'?", len(newUsers), agent.Name)) {
					return fmt.Errorf("user addition cancelled")
				}
			}

			updateData := map[string]interface{}{
				"name":                  agent.Name,
				"description":           agent.Description,
				"instruction_type":      agent.InstructionType,
				"llm_model":             agent.LLMModel,
				"sources":               agent.Sources,
				"environment_variables": agent.Environment,
				"secrets":               agent.Secrets,
				"allowed_groups":        agent.AllowedGroups,
				"allowed_users":         updatedUsers,
				"owners":                agent.Owners,
				"runners":               agent.Runners,
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

			_, err = client.UpdateAgentRaw(cmd.Context(), agentUUID, updateData)
			if err != nil {
				return fmt.Errorf("failed to update agent: %w", err)
			}

			fmt.Printf("%s Added %d user(s) to agent '%s'\n",
				style.SuccessStyle.Render("✅"),
				len(newUsers),
				style.HighlightStyle.Render(agent.Name))

			return nil
		},
	}
}

func newAgentAccessRemoveUserCommand(cfg *config.Config) *cobra.Command {
	return &cobra.Command{
		Use:   "remove-user [agent-uuid] [user-id...]",
		Short: "🗑️ Remove allowed users from agent",
		Args:  cobra.MinimumNArgs(2),
		RunE: func(cmd *cobra.Command, args []string) error {
			// Similar implementation to add-user but removes users
			return fmt.Errorf("not implemented yet - use 'kubiya agent access clear' to remove all restrictions")
		},
	}
}

func newAgentAccessAddGroupCommand(cfg *config.Config) *cobra.Command {
	return &cobra.Command{
		Use:   "add-group [agent-uuid] [group-id...]",
		Short: "➕ Add allowed groups to agent",
		Args:  cobra.MinimumNArgs(2),
		RunE: func(cmd *cobra.Command, args []string) error {
			// Similar implementation to add-user but for groups
			return fmt.Errorf("not implemented yet - use 'kubiya agent access clear' to remove all restrictions")
		},
	}
}

func newAgentAccessRemoveGroupCommand(cfg *config.Config) *cobra.Command {
	return &cobra.Command{
		Use:   "remove-group [agent-uuid] [group-id...]",
		Short: "🗑️ Remove allowed groups from agent", 
		Args:  cobra.MinimumNArgs(2),
		RunE: func(cmd *cobra.Command, args []string) error {
			// Similar implementation to add-user but removes groups
			return fmt.Errorf("not implemented yet - use 'kubiya agent access clear' to remove all restrictions")
		},
	}
}