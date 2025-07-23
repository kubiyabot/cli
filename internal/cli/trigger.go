package cli

import (
	"github.com/kubiyabot/cli/internal/config"
	"github.com/spf13/cobra"
)

// newTriggerCommand creates the trigger command group
func newTriggerCommand(cfg *config.Config) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "trigger",
		Short: "🔗 Manage workflow triggers",
		Long: `Manage external triggers that can execute workflows automatically.

Triggers allow you to integrate with external systems to automatically execute workflows
when certain events occur. Supported providers include:

• Datadog: Create webhooks for alerts, monitors, and incidents
• GitHub: Create webhooks for repository events (coming soon)

Each provider has its own configuration requirements and capabilities.`,
		Example: `  # Create a Datadog webhook trigger
  kubiya trigger create datadog --workflow my-workflow.yaml --name "incident-response"
  
  # List all triggers
  kubiya trigger list
  
  # Test a trigger
  kubiya trigger test my-trigger-id
  
  # Update an existing trigger
  kubiya trigger update my-trigger-id --workflow updated-workflow.yaml
  
  # Delete a trigger
  kubiya trigger delete my-trigger-id`,
	}

	// Add subcommands
	cmd.AddCommand(
		newTriggerCreateCommand(cfg),
		newTriggerListCommand(cfg),
		newTriggerDescribeCommand(cfg),
		newTriggerUpdateCommand(cfg),
		newTriggerDeleteCommand(cfg),
		newTriggerTestCommand(cfg),
	)

	return cmd
}