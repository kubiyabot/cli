package cli

import (
	"encoding/json"
	"fmt"
	"os"
	"sort"
	"strings"
	"text/tabwriter"

	"github.com/kubiyabot/cli/internal/config"
	"github.com/kubiyabot/cli/internal/kubiya"
	"github.com/kubiyabot/cli/internal/style"
	"github.com/spf13/cobra"
	"gopkg.in/yaml.v3"
)

// newAgentIntegrationsCommand creates the command to manage agent integrations
func newAgentIntegrationsCommand(cfg *config.Config) *cobra.Command {
	cmd := &cobra.Command{
		Use:     "integrations",
		Aliases: []string{"integration", "int"},
		Short:   "🔌 Manage agent integrations",
		Long:    `Manage integrations for an agent.`,
	}

	cmd.AddCommand(
		newAgentIntegrationsListCommand(cfg),
		newAgentIntegrationsAddCommand(cfg),
		newAgentIntegrationsRemoveCommand(cfg),
	)

	return cmd
}

// newAgentIntegrationsListCommand lists agent integrations
func newAgentIntegrationsListCommand(cfg *config.Config) *cobra.Command {
	var outputFormat string

	cmd := &cobra.Command{
		Use:     "list [agent-uuid]",
		Aliases: []string{"ls", "get"},
		Short:   "📋 List agent integrations",
		Long:    `List all integrations attached to an agent.`,
		Example: `  # List integrations
  kubiya agent integrations list abc-123

  # List in JSON format
  kubiya agent integrations list abc-123 --output json`,
		Args: cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			agentUUID := args[0]
			client := kubiya.NewClient(cfg)

			agent, err := client.GetAgent(cmd.Context(), agentUUID)
			if err != nil {
				return fmt.Errorf("failed to get agent: %w", err)
			}

			switch outputFormat {
			case "json":
				return json.NewEncoder(os.Stdout).Encode(agent.Integrations)
			case "yaml":
				return yaml.NewEncoder(os.Stdout).Encode(agent.Integrations)
			default:
				fmt.Printf("%s Agent Integrations: %s\n\n",
					style.TitleStyle.Render("🔌"),
					style.HighlightStyle.Render(agent.Name))

				if len(agent.Integrations) == 0 {
					fmt.Printf("%s No integrations configured\n",
						style.DimStyle.Render("  •"))
					return nil
				}

				for i, integration := range agent.Integrations {
					fmt.Printf("  %s %s\n", 
						style.InfoStyle.Render(fmt.Sprintf("%d.", i+1)),
						style.HighlightStyle.Render(integration))
				}
				
				fmt.Printf("\n%s Total: %d integrations\n",
					style.SubtitleStyle.Render("📊"),
					len(agent.Integrations))
			}

			return nil
		},
	}

	cmd.Flags().StringVarP(&outputFormat, "output", "o", "text", "Output format (text|json|yaml)")
	return cmd
}

// newAgentIntegrationsAddCommand adds integrations to an agent
func newAgentIntegrationsAddCommand(cfg *config.Config) *cobra.Command {
	var (
		yes          bool
		outputFormat string
	)

	cmd := &cobra.Command{
		Use:     "add [agent-uuid] [integration...]",
		Aliases: []string{"attach"},
		Short:   "➕ Add integration(s) to an agent",
		Long:    `Add one or more integrations to an agent.`,
		Example: `  # Add single integration
  kubiya agent integrations add abc-123 slack

  # Add multiple integrations
  kubiya agent integrations add abc-123 slack github teams

  # Add with confirmation skip
  kubiya agent integrations add abc-123 slack -y`,
		Args: cobra.MinimumNArgs(2),
		RunE: func(cmd *cobra.Command, args []string) error {
			agentUUID := args[0]
			integrationsToAdd := args[1:]

			client := kubiya.NewClient(cfg)
			
			// Get current agent
			agent, err := client.GetAgent(cmd.Context(), agentUUID)
			if err != nil {
				return fmt.Errorf("failed to get agent: %w", err)
			}

			// Check for duplicates
			var newIntegrations []string
			var duplicates []string
			
			for _, newIntegration := range integrationsToAdd {
				isDuplicate := false
				for _, existingIntegration := range agent.Integrations {
					if existingIntegration == newIntegration {
						duplicates = append(duplicates, newIntegration)
						isDuplicate = true
						break
					}
				}
				if !isDuplicate {
					newIntegrations = append(newIntegrations, newIntegration)
				}
			}

			// Report duplicates
			if len(duplicates) > 0 {
				fmt.Printf("%s Integrations already exist: %s\n", 
					style.WarningStyle.Render("⚠️"), 
					strings.Join(duplicates, ", "))
			}

			// Exit if no new integrations to add
			if len(newIntegrations) == 0 {
				return fmt.Errorf("no new integrations to add")
			}

			// Show what will be added
			fmt.Printf("%s Adding integrations to agent: %s\n\n", 
				style.InfoStyle.Render("➕"), 
				style.HighlightStyle.Render(agent.Name))
			
			for _, integration := range newIntegrations {
				fmt.Printf("  • %s\n", style.SuccessStyle.Render(integration))
			}
			fmt.Println()

			// Confirm addition
			if !yes {
				if !confirmYesNo(fmt.Sprintf("Add %d integration(s) to agent '%s'?", len(newIntegrations), agent.Name)) {
					return fmt.Errorf("integration addition cancelled")
				}
			}

			// Add integrations to agent
			updatedIntegrations := append(agent.Integrations, newIntegrations...)

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
				"integrations":          updatedIntegrations,
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

			fmt.Printf("%s Added %d integration(s) to agent '%s'\n\n",
				style.SuccessStyle.Render("✅"),
				len(newIntegrations),
				style.HighlightStyle.Render(result.Name))

			// Show updated integrations count
			fmt.Printf("%s Agent now has %d integrations\n",
				style.InfoStyle.Render("📊"),
				len(result.Integrations))

			return nil
		},
	}

	cmd.Flags().BoolVarP(&yes, "yes", "y", false, "Skip confirmation prompts")
	cmd.Flags().StringVarP(&outputFormat, "output", "o", "text", "Output format (text|json)")

	return cmd
}

// newAgentIntegrationsRemoveCommand removes integrations from an agent
func newAgentIntegrationsRemoveCommand(cfg *config.Config) *cobra.Command {
	var (
		yes          bool
		outputFormat string
	)

	cmd := &cobra.Command{
		Use:     "remove [agent-uuid] [integration...]",
		Aliases: []string{"rm", "delete", "del", "detach"},
		Short:   "🗑️ Remove integration(s) from an agent",
		Long:    `Remove one or more integrations from an agent.`,
		Example: `  # Remove single integration
  kubiya agent integrations remove abc-123 slack

  # Remove multiple integrations
  kubiya agent integrations remove abc-123 slack github

  # Remove with confirmation skip
  kubiya agent integrations remove abc-123 slack -y`,
		Args: cobra.MinimumNArgs(2),
		RunE: func(cmd *cobra.Command, args []string) error {
			agentUUID := args[0]
			integrationsToRemove := args[1:]

			client := kubiya.NewClient(cfg)
			
			// Get current agent
			agent, err := client.GetAgent(cmd.Context(), agentUUID)
			if err != nil {
				return fmt.Errorf("failed to get agent: %w", err)
			}

			// Check which integrations exist on the agent
			var validIntegrations []string
			var notFound []string
			
			for _, integrationToRemove := range integrationsToRemove {
				found := false
				for _, existingIntegration := range agent.Integrations {
					if existingIntegration == integrationToRemove {
						validIntegrations = append(validIntegrations, integrationToRemove)
						found = true
						break
					}
				}
				if !found {
					notFound = append(notFound, integrationToRemove)
				}
			}

			// Report integrations not found
			if len(notFound) > 0 {
				fmt.Printf("%s Integrations not found on agent: %s\n", 
					style.WarningStyle.Render("⚠️"), 
					strings.Join(notFound, ", "))
			}

			// Exit if no valid integrations to remove
			if len(validIntegrations) == 0 {
				return fmt.Errorf("no valid integrations found to remove")
			}

			// Show what will be removed
			fmt.Printf("%s Removing integrations from agent: %s\n\n", 
				style.InfoStyle.Render("🗑️"), 
				style.HighlightStyle.Render(agent.Name))
			
			for _, integration := range validIntegrations {
				fmt.Printf("  • %s\n", style.ErrorStyle.Render(integration))
			}
			fmt.Println()

			// Confirm removal
			if !yes {
				if !confirmYesNo(fmt.Sprintf("Remove %d integration(s) from agent '%s'?", len(validIntegrations), agent.Name)) {
					return fmt.Errorf("integration removal cancelled")
				}
			}

			// Remove integrations from agent
			var updatedIntegrations []string
			for _, existingIntegration := range agent.Integrations {
				shouldRemove := false
				for _, integrationToRemove := range validIntegrations {
					if existingIntegration == integrationToRemove {
						shouldRemove = true
						break
					}
				}
				if !shouldRemove {
					updatedIntegrations = append(updatedIntegrations, existingIntegration)
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
				"integrations":          updatedIntegrations,
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

			fmt.Printf("%s Removed %d integration(s) from agent '%s'\n\n",
				style.SuccessStyle.Render("✅"),
				len(validIntegrations),
				style.HighlightStyle.Render(result.Name))

			// Show updated integrations count
			fmt.Printf("%s Agent now has %d integrations\n",
				style.InfoStyle.Render("📊"),
				len(result.Integrations))

			return nil
		},
	}

	cmd.Flags().BoolVarP(&yes, "yes", "y", false, "Skip confirmation prompts")
	cmd.Flags().StringVarP(&outputFormat, "output", "o", "text", "Output format (text|json)")

	return cmd
}

// newAgentEnvCommand creates the command to manage agent environment variables
func newAgentEnvCommand(cfg *config.Config) *cobra.Command {
	cmd := &cobra.Command{
		Use:     "env",
		Aliases: []string{"environment", "envvars"},
		Short:   "🌍 Manage agent environment variables",
		Long:    `Manage environment variables for an agent.`,
	}

	cmd.AddCommand(
		newAgentEnvListCommand(cfg),
		newAgentEnvSetCommand(cfg),
		newAgentEnvUnsetCommand(cfg),
	)

	return cmd
}

// newAgentEnvListCommand lists agent environment variables
func newAgentEnvListCommand(cfg *config.Config) *cobra.Command {
	var outputFormat string

	cmd := &cobra.Command{
		Use:     "list [agent-uuid]",
		Aliases: []string{"ls", "get"},
		Short:   "📋 List agent environment variables",
		Long:    `List all environment variables for an agent.`,
		Example: `  # List environment variables
  kubiya agent env list abc-123

  # List in JSON format
  kubiya agent env list abc-123 --output json`,
		Args: cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			agentUUID := args[0]
			client := kubiya.NewClient(cfg)

			agent, err := client.GetAgent(cmd.Context(), agentUUID)
			if err != nil {
				return fmt.Errorf("failed to get agent: %w", err)
			}

			switch outputFormat {
			case "json":
				return json.NewEncoder(os.Stdout).Encode(agent.Environment)
			case "yaml":
				return yaml.NewEncoder(os.Stdout).Encode(agent.Environment)
			default:
				fmt.Printf("%s Agent Environment Variables: %s\n\n",
					style.TitleStyle.Render("🌍"),
					style.HighlightStyle.Render(agent.Name))

				if len(agent.Environment) == 0 {
					fmt.Printf("%s No environment variables configured\n",
						style.DimStyle.Render("  •"))
					return nil
				}

				// Sort keys for consistent output
				keys := make([]string, 0, len(agent.Environment))
				for key := range agent.Environment {
					keys = append(keys, key)
				}
				sort.Strings(keys)

				w := tabwriter.NewWriter(os.Stdout, 0, 0, 3, ' ', 0)
				fmt.Fprintf(w, "  %s\t%s\n", 
					style.SubtitleStyle.Render("KEY"), 
					style.SubtitleStyle.Render("VALUE"))
				
				for _, key := range keys {
					value := agent.Environment[key]
					// Truncate long values for display
					if len(value) > 50 {
						value = value[:47] + "..."
					}
					fmt.Fprintf(w, "  %s\t%s\n", 
						style.HighlightStyle.Render(key),
						style.DimStyle.Render(value))
				}
				w.Flush()
				
				fmt.Printf("\n%s Total: %d environment variables\n",
					style.SubtitleStyle.Render("📊"),
					len(agent.Environment))
			}

			return nil
		},
	}

	cmd.Flags().StringVarP(&outputFormat, "output", "o", "text", "Output format (text|json|yaml)")
	return cmd
}

// newAgentEnvSetCommand sets environment variables for an agent
func newAgentEnvSetCommand(cfg *config.Config) *cobra.Command {
	var (
		yes          bool
		outputFormat string
		envVars      []string
	)

	cmd := &cobra.Command{
		Use:     "set [agent-uuid]",
		Aliases: []string{"add"},
		Short:   "➕ Set environment variable(s) for an agent",
		Long:    `Set one or more environment variables for an agent using KEY=VALUE format.`,
		Example: `  # Set single environment variable
  kubiya agent env set abc-123 --env LOG_LEVEL=debug

  # Set multiple environment variables
  kubiya agent env set abc-123 --env LOG_LEVEL=debug --env TIMEOUT=30 --env MODE=production

  # Set with confirmation skip
  kubiya agent env set abc-123 --env API_URL=https://api.example.com -y`,
		Args: cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			agentUUID := args[0]

			if len(envVars) == 0 {
				return fmt.Errorf("at least one environment variable must be specified using --env KEY=VALUE")
			}

			client := kubiya.NewClient(cfg)
			
			// Get current agent
			agent, err := client.GetAgent(cmd.Context(), agentUUID)
			if err != nil {
				return fmt.Errorf("failed to get agent: %w", err)
			}

			// Parse environment variables
			newEnvVars := make(map[string]string)
			var updates []string
			var additions []string

			for _, envVar := range envVars {
				parts := strings.SplitN(envVar, "=", 2)
				if len(parts) != 2 {
					return fmt.Errorf("invalid environment variable format: %s (expected KEY=VALUE)", envVar)
				}
				
				key, value := parts[0], parts[1]
				newEnvVars[key] = value

				if _, exists := agent.Environment[key]; exists {
					updates = append(updates, key)
				} else {
					additions = append(additions, key)
				}
			}

			// Show what will be changed
			fmt.Printf("%s Setting environment variables for agent: %s\n\n", 
				style.InfoStyle.Render("🌍"), 
				style.HighlightStyle.Render(agent.Name))
			
			if len(additions) > 0 {
				fmt.Printf("%s New variables:\n", style.SuccessStyle.Render("➕"))
				for _, key := range additions {
					fmt.Printf("  • %s=%s\n", 
						style.HighlightStyle.Render(key),
						style.DimStyle.Render(newEnvVars[key]))
				}
			}

			if len(updates) > 0 {
				fmt.Printf("%s Updated variables:\n", style.WarningStyle.Render("🔄"))
				for _, key := range updates {
					fmt.Printf("  • %s: %s → %s\n", 
						style.HighlightStyle.Render(key),
						style.DimStyle.Render(agent.Environment[key]),
						style.DimStyle.Render(newEnvVars[key]))
				}
			}
			fmt.Println()

			// Confirm changes
			if !yes {
				if !confirmYesNo(fmt.Sprintf("Set %d environment variable(s) for agent '%s'?", len(newEnvVars), agent.Name)) {
					return fmt.Errorf("environment variable update cancelled")
				}
			}

			// Merge environment variables
			updatedEnvironment := make(map[string]string)
			for k, v := range agent.Environment {
				updatedEnvironment[k] = v
			}
			for k, v := range newEnvVars {
				updatedEnvironment[k] = v
			}

			// Create update payload
			updateData := map[string]interface{}{
				"name":                  agent.Name,
				"description":           agent.Description,
				"instruction_type":      agent.InstructionType,
				"llm_model":             agent.LLMModel,
				"sources":               agent.Sources,
				"environment_variables": updatedEnvironment,
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
				"tools":                 agent.Tools,
				"tasks":                 agent.Tasks,
				"tags":                  agent.Tags,
			}

			// Update the agent
			result, err := client.UpdateAgentRaw(cmd.Context(), agentUUID, updateData)
			if err != nil {
				return fmt.Errorf("failed to update agent: %w", err)
			}

			fmt.Printf("%s Set %d environment variable(s) for agent '%s'\n\n",
				style.SuccessStyle.Render("✅"),
				len(newEnvVars),
				style.HighlightStyle.Render(result.Name))

			// Show updated environment variables count
			fmt.Printf("%s Agent now has %d environment variables\n",
				style.InfoStyle.Render("📊"),
				len(result.Environment))

			return nil
		},
	}

	cmd.Flags().BoolVarP(&yes, "yes", "y", false, "Skip confirmation prompts")
	cmd.Flags().StringVarP(&outputFormat, "output", "o", "text", "Output format (text|json)")
	cmd.Flags().StringArrayVar(&envVars, "env", []string{}, "Environment variables in KEY=VALUE format (can be used multiple times)")

	return cmd
}

// newAgentEnvUnsetCommand unsets environment variables for an agent
func newAgentEnvUnsetCommand(cfg *config.Config) *cobra.Command {
	var (
		yes          bool
		outputFormat string
	)

	cmd := &cobra.Command{
		Use:     "unset [agent-uuid] [key...]",
		Aliases: []string{"remove", "rm", "delete", "del"},
		Short:   "🗑️ Unset environment variable(s) for an agent",
		Long:    `Remove one or more environment variables from an agent.`,
		Example: `  # Unset single environment variable
  kubiya agent env unset abc-123 LOG_LEVEL

  # Unset multiple environment variables
  kubiya agent env unset abc-123 LOG_LEVEL TIMEOUT MODE

  # Unset with confirmation skip
  kubiya agent env unset abc-123 DEPRECATED_VAR -y`,
		Args: cobra.MinimumNArgs(2),
		RunE: func(cmd *cobra.Command, args []string) error {
			agentUUID := args[0]
			keysToRemove := args[1:]

			client := kubiya.NewClient(cfg)
			
			// Get current agent
			agent, err := client.GetAgent(cmd.Context(), agentUUID)
			if err != nil {
				return fmt.Errorf("failed to get agent: %w", err)
			}

			// Check which keys exist
			var validKeys []string
			var notFound []string
			
			for _, key := range keysToRemove {
				if _, exists := agent.Environment[key]; exists {
					validKeys = append(validKeys, key)
				} else {
					notFound = append(notFound, key)
				}
			}

			// Report keys not found
			if len(notFound) > 0 {
				fmt.Printf("%s Environment variables not found: %s\n", 
					style.WarningStyle.Render("⚠️"), 
					strings.Join(notFound, ", "))
			}

			// Exit if no valid keys to remove
			if len(validKeys) == 0 {
				return fmt.Errorf("no valid environment variables found to remove")
			}

			// Show what will be removed
			fmt.Printf("%s Removing environment variables from agent: %s\n\n", 
				style.InfoStyle.Render("🗑️"), 
				style.HighlightStyle.Render(agent.Name))
			
			for _, key := range validKeys {
				fmt.Printf("  • %s=%s\n", 
					style.ErrorStyle.Render(key),
					style.DimStyle.Render(agent.Environment[key]))
			}
			fmt.Println()

			// Confirm removal
			if !yes {
				if !confirmYesNo(fmt.Sprintf("Remove %d environment variable(s) from agent '%s'?", len(validKeys), agent.Name)) {
					return fmt.Errorf("environment variable removal cancelled")
				}
			}

			// Remove environment variables
			updatedEnvironment := make(map[string]string)
			for k, v := range agent.Environment {
				shouldRemove := false
				for _, keyToRemove := range validKeys {
					if k == keyToRemove {
						shouldRemove = true
						break
					}
				}
				if !shouldRemove {
					updatedEnvironment[k] = v
				}
			}

			// Create update payload
			updateData := map[string]interface{}{
				"name":                  agent.Name,
				"description":           agent.Description,
				"instruction_type":      agent.InstructionType,
				"llm_model":             agent.LLMModel,
				"sources":               agent.Sources,
				"environment_variables": updatedEnvironment,
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
				"tools":                 agent.Tools,
				"tasks":                 agent.Tasks,
				"tags":                  agent.Tags,
			}

			// Update the agent
			result, err := client.UpdateAgentRaw(cmd.Context(), agentUUID, updateData)
			if err != nil {
				return fmt.Errorf("failed to update agent: %w", err)
			}

			fmt.Printf("%s Removed %d environment variable(s) from agent '%s'\n\n",
				style.SuccessStyle.Render("✅"),
				len(validKeys),
				style.HighlightStyle.Render(result.Name))

			// Show updated environment variables count
			fmt.Printf("%s Agent now has %d environment variables\n",
				style.InfoStyle.Render("📊"),
				len(result.Environment))

			return nil
		},
	}

	cmd.Flags().BoolVarP(&yes, "yes", "y", false, "Skip confirmation prompts")
	cmd.Flags().StringVarP(&outputFormat, "output", "o", "text", "Output format (text|json)")

	return cmd
}

// newAgentSecretsCommand creates the command to manage agent secrets
func newAgentSecretsCommand(cfg *config.Config) *cobra.Command {
	cmd := &cobra.Command{
		Use:     "secrets",
		Aliases: []string{"secret"},
		Short:   "🔐 Manage agent secrets",
		Long:    `Manage secrets for an agent.`,
	}

	cmd.AddCommand(
		newAgentSecretsListCommand(cfg),
		newAgentSecretsAddCommand(cfg),
		newAgentSecretsRemoveCommand(cfg),
	)

	return cmd
}

// newAgentSecretsListCommand lists agent secrets
func newAgentSecretsListCommand(cfg *config.Config) *cobra.Command {
	var outputFormat string

	cmd := &cobra.Command{
		Use:     "list [agent-uuid]",
		Aliases: []string{"ls", "get"},
		Short:   "📋 List agent secrets",
		Long:    `List all secrets attached to an agent.`,
		Example: `  # List secrets
  kubiya agent secrets list abc-123

  # List in JSON format
  kubiya agent secrets list abc-123 --output json`,
		Args: cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			agentUUID := args[0]
			client := kubiya.NewClient(cfg)

			agent, err := client.GetAgent(cmd.Context(), agentUUID)
			if err != nil {
				return fmt.Errorf("failed to get agent: %w", err)
			}

			switch outputFormat {
			case "json":
				return json.NewEncoder(os.Stdout).Encode(agent.Secrets)
			case "yaml":
				return yaml.NewEncoder(os.Stdout).Encode(agent.Secrets)
			default:
				fmt.Printf("%s Agent Secrets: %s\n\n",
					style.TitleStyle.Render("🔐"),
					style.HighlightStyle.Render(agent.Name))

				if len(agent.Secrets) == 0 {
					fmt.Printf("%s No secrets configured\n",
						style.DimStyle.Render("  •"))
					return nil
				}

				for i, secret := range agent.Secrets {
					fmt.Printf("  %s %s\n", 
						style.InfoStyle.Render(fmt.Sprintf("%d.", i+1)),
						style.HighlightStyle.Render(secret))
				}
				
				fmt.Printf("\n%s Total: %d secrets\n",
					style.SubtitleStyle.Render("📊"),
					len(agent.Secrets))
			}

			return nil
		},
	}

	cmd.Flags().StringVarP(&outputFormat, "output", "o", "text", "Output format (text|json|yaml)")
	return cmd
}

// newAgentSecretsAddCommand adds secrets to an agent
func newAgentSecretsAddCommand(cfg *config.Config) *cobra.Command {
	var (
		yes          bool
		outputFormat string
	)

	cmd := &cobra.Command{
		Use:     "add [agent-uuid] [secret...]",
		Aliases: []string{"attach"},
		Short:   "➕ Add secret(s) to an agent",
		Long:    `Add one or more secrets to an agent.`,
		Example: `  # Add single secret
  kubiya agent secrets add abc-123 DATABASE_PASSWORD

  # Add multiple secrets
  kubiya agent secrets add abc-123 DATABASE_PASSWORD API_KEY JWT_SECRET

  # Add with confirmation skip
  kubiya agent secrets add abc-123 GITHUB_TOKEN -y`,
		Args: cobra.MinimumNArgs(2),
		RunE: func(cmd *cobra.Command, args []string) error {
			agentUUID := args[0]
			secretsToAdd := args[1:]

			client := kubiya.NewClient(cfg)
			
			// Get current agent
			agent, err := client.GetAgent(cmd.Context(), agentUUID)
			if err != nil {
				return fmt.Errorf("failed to get agent: %w", err)
			}

			// Check for duplicates
			var newSecrets []string
			var duplicates []string
			
			for _, newSecret := range secretsToAdd {
				isDuplicate := false
				for _, existingSecret := range agent.Secrets {
					if existingSecret == newSecret {
						duplicates = append(duplicates, newSecret)
						isDuplicate = true
						break
					}
				}
				if !isDuplicate {
					newSecrets = append(newSecrets, newSecret)
				}
			}

			// Report duplicates
			if len(duplicates) > 0 {
				fmt.Printf("%s Secrets already exist: %s\n", 
					style.WarningStyle.Render("⚠️"), 
					strings.Join(duplicates, ", "))
			}

			// Exit if no new secrets to add
			if len(newSecrets) == 0 {
				return fmt.Errorf("no new secrets to add")
			}

			// Show what will be added
			fmt.Printf("%s Adding secrets to agent: %s\n\n", 
				style.InfoStyle.Render("🔐"), 
				style.HighlightStyle.Render(agent.Name))
			
			for _, secret := range newSecrets {
				fmt.Printf("  • %s\n", style.SuccessStyle.Render(secret))
			}
			fmt.Println()

			// Confirm addition
			if !yes {
				if !confirmYesNo(fmt.Sprintf("Add %d secret(s) to agent '%s'?", len(newSecrets), agent.Name)) {
					return fmt.Errorf("secret addition cancelled")
				}
			}

			// Add secrets to agent
			updatedSecrets := append(agent.Secrets, newSecrets...)

			// Create update payload
			updateData := map[string]interface{}{
				"name":                  agent.Name,
				"description":           agent.Description,
				"instruction_type":      agent.InstructionType,
				"llm_model":             agent.LLMModel,
				"sources":               agent.Sources,
				"environment_variables": agent.Environment,
				"secrets":               updatedSecrets,
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
				"tools":                 agent.Tools,
				"tasks":                 agent.Tasks,
				"tags":                  agent.Tags,
			}

			// Update the agent
			result, err := client.UpdateAgentRaw(cmd.Context(), agentUUID, updateData)
			if err != nil {
				return fmt.Errorf("failed to update agent: %w", err)
			}

			fmt.Printf("%s Added %d secret(s) to agent '%s'\n\n",
				style.SuccessStyle.Render("✅"),
				len(newSecrets),
				style.HighlightStyle.Render(result.Name))

			// Show updated secrets count
			fmt.Printf("%s Agent now has %d secrets\n",
				style.InfoStyle.Render("📊"),
				len(result.Secrets))

			return nil
		},
	}

	cmd.Flags().BoolVarP(&yes, "yes", "y", false, "Skip confirmation prompts")
	cmd.Flags().StringVarP(&outputFormat, "output", "o", "text", "Output format (text|json)")

	return cmd
}

// newAgentSecretsRemoveCommand removes secrets from an agent
func newAgentSecretsRemoveCommand(cfg *config.Config) *cobra.Command {
	var (
		yes          bool
		outputFormat string
	)

	cmd := &cobra.Command{
		Use:     "remove [agent-uuid] [secret...]",
		Aliases: []string{"rm", "delete", "del", "detach"},
		Short:   "🗑️ Remove secret(s) from an agent",
		Long:    `Remove one or more secrets from an agent.`,
		Example: `  # Remove single secret
  kubiya agent secrets remove abc-123 OLD_API_KEY

  # Remove multiple secrets
  kubiya agent secrets remove abc-123 OLD_API_KEY DEPRECATED_SECRET

  # Remove with confirmation skip
  kubiya agent secrets remove abc-123 UNUSED_TOKEN -y`,
		Args: cobra.MinimumNArgs(2),
		RunE: func(cmd *cobra.Command, args []string) error {
			agentUUID := args[0]
			secretsToRemove := args[1:]

			client := kubiya.NewClient(cfg)
			
			// Get current agent
			agent, err := client.GetAgent(cmd.Context(), agentUUID)
			if err != nil {
				return fmt.Errorf("failed to get agent: %w", err)
			}

			// Check which secrets exist on the agent
			var validSecrets []string
			var notFound []string
			
			for _, secretToRemove := range secretsToRemove {
				found := false
				for _, existingSecret := range agent.Secrets {
					if existingSecret == secretToRemove {
						validSecrets = append(validSecrets, secretToRemove)
						found = true
						break
					}
				}
				if !found {
					notFound = append(notFound, secretToRemove)
				}
			}

			// Report secrets not found
			if len(notFound) > 0 {
				fmt.Printf("%s Secrets not found on agent: %s\n", 
					style.WarningStyle.Render("⚠️"), 
					strings.Join(notFound, ", "))
			}

			// Exit if no valid secrets to remove
			if len(validSecrets) == 0 {
				return fmt.Errorf("no valid secrets found to remove")
			}

			// Show what will be removed
			fmt.Printf("%s Removing secrets from agent: %s\n\n", 
				style.InfoStyle.Render("🗑️"), 
				style.HighlightStyle.Render(agent.Name))
			
			for _, secret := range validSecrets {
				fmt.Printf("  • %s\n", style.ErrorStyle.Render(secret))
			}
			fmt.Println()

			// Confirm removal
			if !yes {
				if !confirmYesNo(fmt.Sprintf("Remove %d secret(s) from agent '%s'?", len(validSecrets), agent.Name)) {
					return fmt.Errorf("secret removal cancelled")
				}
			}

			// Remove secrets from agent
			var updatedSecrets []string
			for _, existingSecret := range agent.Secrets {
				shouldRemove := false
				for _, secretToRemove := range validSecrets {
					if existingSecret == secretToRemove {
						shouldRemove = true
						break
					}
				}
				if !shouldRemove {
					updatedSecrets = append(updatedSecrets, existingSecret)
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
				"secrets":               updatedSecrets,
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
				"tools":                 agent.Tools,
				"tasks":                 agent.Tasks,
				"tags":                  agent.Tags,
			}

			// Update the agent
			result, err := client.UpdateAgentRaw(cmd.Context(), agentUUID, updateData)
			if err != nil {
				return fmt.Errorf("failed to update agent: %w", err)
			}

			fmt.Printf("%s Removed %d secret(s) from agent '%s'\n\n",
				style.SuccessStyle.Render("✅"),
				len(validSecrets),
				style.HighlightStyle.Render(result.Name))

			// Show updated secrets count
			fmt.Printf("%s Agent now has %d secrets\n",
				style.InfoStyle.Render("📊"),
				len(result.Secrets))

			return nil
		},
	}

	cmd.Flags().BoolVarP(&yes, "yes", "y", false, "Skip confirmation prompts")
	cmd.Flags().StringVarP(&outputFormat, "output", "o", "text", "Output format (text|json)")

	return cmd
}

// newAgentModelCommand creates the command to manage agent LLM model
func newAgentModelCommand(cfg *config.Config) *cobra.Command {
	var (
		yes          bool
		outputFormat string
	)

	// Supported LLM models
	supportedModels := []string{
		"claude-4-sonnet",
		"claude-4-opus", 
		"gpt-4o",
	}

	cmd := &cobra.Command{
		Use:     "model [agent-uuid] [model]",
		Aliases: []string{"llm", "llm-model"},
		Short:   "🧠 Set agent LLM model",
		Long:    fmt.Sprintf(`Set the LLM model for an agent.

Supported models:
  • claude-4-sonnet - Claude Sonnet (faster, efficient)
  • claude-4-opus   - Claude Opus (most capable)  
  • gpt-4o          - GPT-4 Omni (OpenAI)`),
		Example: `  # Set model to Claude Sonnet
  kubiya agent model abc-123 claude-4-sonnet

  # Set model to Claude Opus
  kubiya agent model abc-123 claude-4-opus

  # Set model to GPT-4o
  kubiya agent model abc-123 gpt-4o

  # Set with confirmation skip
  kubiya agent model abc-123 claude-4-sonnet -y`,
		Args: cobra.ExactArgs(2),
		RunE: func(cmd *cobra.Command, args []string) error {
			agentUUID := args[0]
			newModel := args[1]

			// Validate model
			validModel := false
			for _, supported := range supportedModels {
				if supported == newModel {
					validModel = true
					break
				}
			}
			if !validModel {
				return fmt.Errorf("unsupported model: %s\nSupported models: %s", 
					newModel, strings.Join(supportedModels, ", "))
			}

			client := kubiya.NewClient(cfg)
			
			// Get current agent
			agent, err := client.GetAgent(cmd.Context(), agentUUID)
			if err != nil {
				return fmt.Errorf("failed to get agent: %w", err)
			}

			// Check if model is already set
			if agent.LLMModel == newModel {
				fmt.Printf("%s Agent '%s' is already using model: %s\n",
					style.InfoStyle.Render("ℹ️"),
					style.HighlightStyle.Render(agent.Name),
					style.HighlightStyle.Render(newModel))
				return nil
			}

			// Show what will be changed
			fmt.Printf("%s Updating LLM model for agent: %s\n\n", 
				style.InfoStyle.Render("🧠"), 
				style.HighlightStyle.Render(agent.Name))
			
			fmt.Printf("  Current: %s\n", 
				style.DimStyle.Render(agent.LLMModel))
			fmt.Printf("  New:     %s\n\n", 
				style.SuccessStyle.Render(newModel))

			// Confirm change
			if !yes {
				if !confirmYesNo(fmt.Sprintf("Update LLM model for agent '%s' to '%s'?", agent.Name, newModel)) {
					return fmt.Errorf("model update cancelled")
				}
			}

			// Create update payload
			updateData := map[string]interface{}{
				"name":                  agent.Name,
				"description":           agent.Description,
				"instruction_type":      agent.InstructionType,
				"llm_model":             newModel,
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
				"tools":                 agent.Tools,
				"tasks":                 agent.Tasks,
				"tags":                  agent.Tags,
			}

			// Update the agent
			result, err := client.UpdateAgentRaw(cmd.Context(), agentUUID, updateData)
			if err != nil {
				return fmt.Errorf("failed to update agent: %w", err)
			}

			fmt.Printf("%s Updated LLM model for agent '%s' to: %s\n",
				style.SuccessStyle.Render("✅"),
				style.HighlightStyle.Render(result.Name),
				style.HighlightStyle.Render(newModel))

			return nil
		},
	}

	cmd.Flags().BoolVarP(&yes, "yes", "y", false, "Skip confirmation prompts")
	cmd.Flags().StringVarP(&outputFormat, "output", "o", "text", "Output format (text|json)")

	return cmd
}