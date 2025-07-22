package cli

import (
	"context"
	"fmt"
	"os"

	"github.com/spf13/cobra"

	"github.com/kubiyabot/cli/internal/config"
	"github.com/kubiyabot/cli/internal/sentry"
	"github.com/kubiyabot/cli/internal/tui"
	"github.com/kubiyabot/cli/internal/version"
)

func Execute(cfg *config.Config) error {
	// Create root context for Sentry tracing
	ctx := context.Background()

	return sentry.WithTransaction(ctx, "cli_root_execute", func(ctx context.Context) error {
		rootCmd := &cobra.Command{
			Use:   "kubiya",
			Short: "🤖 Kubiya CLI - Your Agentic AI Automation Companion",
			Long: `Welcome to Kubiya CLI! 👋

A powerful tool for interacting with your Kubiya agents and managing your automation sources.
Use 'kubiya --help' to see all available commands.

Quick Start:
  • Chat:             kubiya chat --interactive
  • Browse sources:    kubiya browse  # Interactive source browser
  • Manage agents: kubiya agent list
  • Manage tools:     kubiya tool list
  • Manage runners:   kubiya runner list
  • Manage webhooks:  kubiya webhook list
  • Manage workflows: kubiya workflow generate|test|execute|compose
  • Update CLI:       kubiya update
  • Initialize:       kubiya init tool|workflow  # Create new tools/workflows

Need help? Visit: https://docs.kubiya.ai`,
			Version: version.GetVersion(),
			PersistentPreRun: func(cmd *cobra.Command, args []string) {
				// Set transaction name based on command
				sentry.SetTransaction(fmt.Sprintf("kubiya_%s", cmd.Name()))

				// Add breadcrumb for command execution
				sentry.AddBreadcrumb("command", fmt.Sprintf("Executing command: %s", cmd.Name()), map[string]interface{}{
					"command": cmd.Name(),
					"args":    args,
				})

				// Skip update check for version and update commands
				if cmd.Name() == "version" || cmd.Name() == "update" {
					return
				}

				// Skip update check in automation mode
				if os.Getenv("KUBIYA_AUTOMATION") != "" {
					return
				}

				// Check for updates
				if msg := version.GetUpdateMessage(); msg != "" {
					fmt.Fprint(cmd.ErrOrStderr(), msg)
				}
			},
		}

		// Add the browse command as a top-level alias for source interactive
		browseCmd := &cobra.Command{
			Use:     "browse",
			Aliases: []string{"b"},
			Short:   "🎮 Browse and execute tools interactively",
			RunE: func(cmd *cobra.Command, args []string) error {
				return sentry.WithTransaction(cmd.Context(), "browse_command", func(ctx context.Context) error {
					app := tui.NewSourceBrowser(cfg)
					return app.Run()
				})
			},
		}
		rootCmd.AddCommand(browseCmd)

		// Add other subcommands
		rootCmd.AddCommand(
			newChatCommand(cfg),
			newAgentCommand(cfg),
			newSourcesCommand(cfg),
			newToolsCommand(cfg),
			newKnowledgeCommand(cfg),
			newRunnersCommand(cfg),
			newWebhooksCommand(cfg),
			newSecretsCommand(cfg),
			newGenerateToolCommand(cfg),
			newUpdateCommand(cfg),
			newVersionCommand(cfg),
			newIntegrationsCommand(cfg),
			newProjectCommand(cfg),
			newAuditCommand(cfg),
			newRunCommand(cfg),
			newInitCommand(cfg),
			newMcpCommand(cfg),
			newWorkflowCommand(cfg),
			newPolicyCommand(cfg),
			newUsersCommand(cfg),
		)

		return rootCmd.Execute()
	})
}
