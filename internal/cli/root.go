package cli

import (
	"github.com/kubiyabot/cli/internal/config"
	"github.com/kubiyabot/cli/internal/tui"
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
  • Chat:             kubiya chat --interactive
  • Browse sources:    kubiya browse  # Interactive source browser
  • Manage teammates: kubiya teammate list
  • List sources:     kubiya source list
  • Manage knowledge: kubiya knowledge list
  • Manage runners:   kubiya runner list
  • Manage webhooks:  kubiya webhook list

Need help? Visit: https://docs.kubiya.ai`,
	}

	// Add the browse command as a top-level alias for source interactive
	browseCmd := &cobra.Command{
		Use:     "browse",
		Aliases: []string{"b"},
		Short:   "🎮 Browse and execute tools interactively",
		RunE: func(cmd *cobra.Command, args []string) error {
			app := tui.NewSourceBrowser(cfg)
			return app.Run()
		},
	}
	rootCmd.AddCommand(browseCmd)

	// Add other subcommands
	rootCmd.AddCommand(
		newChatCommand(cfg),
		newTeammateCommand(cfg),
		newSourcesCommand(cfg),
		newToolsCommand(cfg),
		newKnowledgeCommand(cfg),
		newRunnersCommand(cfg),
		newWebhooksCommand(cfg),
		newSecretsCommand(cfg),
		newGenerateToolCommand(cfg),
	)

	return rootCmd.Execute()
}
