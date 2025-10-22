package cli

import (
	"fmt"
	"os"

	"github.com/spf13/cobra"

	"github.com/kubiyabot/cli/internal/config"
	"github.com/kubiyabot/cli/internal/tui"
	"github.com/kubiyabot/cli/internal/version"
)

func Execute(cfg *config.Config) error {
	rootCmd := &cobra.Command{
		Use:   "kubiya",
		Short: "🤖 Kubiya CLI - Your Agentic AI Automation Companion",
		Long: `Welcome to Kubiya CLI! 👋

A powerful tool for interacting with your Kubiya agents and managing your automation sources.

🔑 First Time Setup:
  kubiya login                          # Interactive authentication (recommended)

🌐 Get API Key: https://compose.kubiya.ai/settings#apiKeys
💻 For CI/Automation: export KUBIYA_API_KEY=your-api-key

Quick Start:
  • Authenticate:     kubiya login
  • Chat:             kubiya chat --interactive
  • Browse sources:   kubiya browse              # Interactive source browser
  • Manage agents:    kubiya agent list
  • Manage tools:     kubiya tool list
  • Manage runners:   kubiya runner list
  • Manage webhooks:  kubiya webhook list
  • Manage workflows: kubiya workflow list|run|execute|compose
  • Update CLI:       kubiya update
  • Initialize:       kubiya init tool|workflow  # Create new tools/workflows

Need help? Visit: https://docs.kubiya.ai`,
		Version: version.GetVersion(),
		PersistentPreRun: func(cmd *cobra.Command, args []string) {
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

			// Show authentication hint for commands that need auth if not configured
			showAuthHintIfNeeded(cmd, cfg)
		},

		RunE: func(cmd *cobra.Command, args []string) error {
			// If no subcommand specified and no auth, show helpful message
			if cfg.APIKey == "" {
				fmt.Printf(`Welcome to Kubiya CLI! 🤖

To get started, you need to authenticate first:

🔑 Recommended: Interactive setup
   kubiya login

💻 Alternative: Manual API key setup
   export KUBIYA_API_KEY=your-api-key

🌐 Get your API key from: https://compose.kubiya.ai/settings#apiKeys

Once authenticated, try:
   kubiya workflow list          # List your workflows
   kubiya agent list             # List your agents
   kubiya chat                   # Start a chat session

Need help? Visit: https://docs.kubiya.ai
`)
				return nil
			}

			// Show help if no subcommand
			return cmd.Help()
		},
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
		newAgentCommand(cfg),
		newSourcesCommand(cfg),
		newToolsCommand(cfg),
		newDocumentationCommand(cfg),
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
		newTriggerCommand(cfg),
		newPolicyCommand(cfg),
		newUsersCommand(cfg),
		newLoginCommand(cfg),
		newWorkerCommand(cfg),
	)

	return rootCmd.Execute()
}

// showAuthHintIfNeeded displays a helpful authentication message for commands that require auth
func showAuthHintIfNeeded(cmd *cobra.Command, cfg *config.Config) {
	// Commands that require authentication
	authRequiredCommands := map[string]bool{
		"workflow": true,
		"agent":    true,
		"chat":     true,
		"browse":   true,
		"tool":     true,
		"runner":   true,
		"webhook":  true,
		"source":   true,
		"secret":   true,
	}

	// Check if this command or its parent requires auth
	currentCmd := cmd
	for currentCmd != nil {
		if authRequiredCommands[currentCmd.Name()] {
			if cfg.APIKey == "" {
				fmt.Fprintf(os.Stderr, `
⚠️  Authentication required for '%s' command

🔑 Quick setup: kubiya login
🌐 Get API key: https://compose.kubiya.ai/settings#apiKeys

`, currentCmd.Name())
			}
			break
		}
		currentCmd = currentCmd.Parent()
	}
}
