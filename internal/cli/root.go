package cli

import (
	"fmt"
	"os"

	"github.com/spf13/cobra"

	"github.com/kubiyabot/cli/internal/config"
	"github.com/kubiyabot/cli/internal/version"
)

func Execute(cfg *config.Config) error {
	rootCmd := &cobra.Command{
		Use:   "kubiya",
		Short: "🤖 Kubiya CLI - Your Agentic AI Automation Companion",
		Long: `Welcome to Kubiya CLI! 👋

A powerful tool for interacting with your Kubiya agents and managing your automation.

🔑 First Time Setup:
  kubiya login                          # Interactive authentication (recommended)

🌐 Get API Key: https://compose.kubiya.ai/settings#apiKeys
💻 For CI/Automation: export KUBIYA_API_KEY=your-api-key

Quick Start:
  • Authenticate:     kubiya login
  • Chat:             kubiya agent chat <agent-id>
  • Manage agents:    kubiya agent list
  • Manage teams:     kubiya team list
  • Manage skills:    kubiya skill list
  • Manage policies:  kubiya policy list
  • View executions:  kubiya execution list
  • Schedule jobs:    kubiya job list
  • Manage workflows: kubiya workflow list|run|execute|compose
  • Update CLI:       kubiya update

Need help? Visit: https://docs.kubiya.ai`,
		Version:       version.GetVersion(),
		SilenceUsage:  true,  // Never show usage on errors - errors are formatted by handleError in main.go
		SilenceErrors: false, // Let errors propagate to main.go for proper handling
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

	// V2 Control Plane Commands
	rootCmd.AddCommand(
		// Core V2 Commands
		NewExecCommand(cfg),        // V2: Smart exec with auto-planning
		newAgentCommand(cfg),       // V2: Agents
		newTeamCommand(cfg),        // V2: Teams
		newExecutionCommand(cfg),   // V2: Executions (one-time task runs)
		newJobCommand(cfg),         // V2: Jobs (scheduled/recurring tasks)
		newModelCommand(cfg),       // V2: Models
		newSkillCommand(cfg),       // V2: Skills
		newPolicyCommand(cfg),      // V2: Policies
		newEnvironmentCommand(cfg), // V2: Environments
		newProjectCommand(cfg),     // V2: Projects
		newWorkerCommand(cfg),      // V2: Worker management
		newGraphCommand(cfg),       // V2: Context Graph (includes intelligent search)
		newMemoryCommand(cfg),      // V2: Cognitive memory management

		// V1 Legacy Commands (still on api.kubiya.ai)
		newWorkflowCommand(cfg), // V1: Workflows
		newUsersCommand(cfg),    // V1: User management
		newSecretsCommand(cfg),  // V1: Secrets
		newKnowledgeCommand(cfg), // V1: Knowledge service

		// System Commands
		newAuthCommand(cfg),  // Authentication management
		newLoginCommand(cfg),
		newUpdateCommand(cfg),
		newVersionCommand(cfg),
		NewConfigCmd(),       // Context management
		newMcpCommand(cfg),   // MCP server management
	)

	return rootCmd.Execute()
}

// showAuthHintIfNeeded displays a helpful authentication message for commands that require auth
func showAuthHintIfNeeded(cmd *cobra.Command, cfg *config.Config) {
	// Commands that require authentication
	authRequiredCommands := map[string]bool{
		"workflow":  true,
		"agent":     true,
		"team":      true,
		"execution": true,
		"job":       true,
		"model":     true,
		"skill":     true,
		"policy":    true,
		"secret":    true,
		"knowledge": true,
		"graph":     true,
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
