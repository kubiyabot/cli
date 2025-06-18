package cli

import (
	"github.com/kubiyabot/cli/internal/config"
	"github.com/spf13/cobra"
)

// newWorkflowCommand creates the workflow command group
func newWorkflowCommand(cfg *config.Config) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "workflow",
		Short: "Manage Kubiya workflows",
		Long: `Manage Kubiya workflows including generation, testing, execution, and composition.

This command provides comprehensive workflow management capabilities:
• Generate workflows from natural language descriptions
• Test workflows with streaming output
• Execute workflows from files or inline
• Compose complex workflows from simpler ones`,
		Example: `  # Generate a workflow from description
  kubiya workflow generate "create a workflow to deploy an app"
  
  # Test a workflow
  kubiya workflow test my-workflow.yaml
  
  # Execute a workflow
  kubiya workflow execute my-workflow.yaml
  
  # Compose workflows
  kubiya workflow compose --from deploy.yaml --from notify.yaml --output pipeline.yaml`,
	}

	// Add subcommands
	cmd.AddCommand(
		newWorkflowGenerateCommand(cfg),
		newWorkflowTestCommand(cfg),
		newWorkflowExecuteCommand(cfg),
		newWorkflowComposeCommand(cfg),
	)

	return cmd
}
