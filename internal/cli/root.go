package cli

import (
	"github.com/kubiyabot/cli/internal/config"
	"github.com/spf13/cobra"
)

func Execute(cfg *config.Config) error {
	rootCmd := &cobra.Command{
		Use:   "kubiya",
		Short: "🤖 Kubiya CLI - Your DevOps Automation Companion",
		Long: `Welcome to Kubiya CLI! 👋

A powerful tool for interacting with your Kubiya teammates and managing your automation sources.
Use 'kubiya --help' to see all available commands.

Quick Start:
  • List teammates:    kubiya list
  • Chat:             kubiya chat --interactive
  • List sources:     kubiya source list
  • Manage knowledge: kubiya knowledge list
  • Manage runners:   kubiya runner list
  • Manage webhooks:  kubiya webhook list

Need help? Visit: https://docs.kubiya.ai`,
		Example: `  # Interactive chat
  kubiya chat --interactive

  # List sources
  kubiya source list

  # Manage knowledge
  kubiya knowledge list

  # Manage runners
  kubiya runner list

  # Manage webhooks
  kubiya webhook list`,
		SilenceUsage: true,
	}

	// Add subcommands
	rootCmd.AddCommand(
		newChatCommand(cfg),
		newListCommand(cfg),
		newSourcesCommand(cfg),
		newKnowledgeCommand(cfg),
		newRunnersCommand(cfg),
		newWebhooksCommand(cfg),
	)

	return rootCmd.Execute()
}
