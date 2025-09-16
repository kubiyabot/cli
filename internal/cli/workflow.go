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
		Long: `Manage Kubiya workflows including generation, testing, execution, composition, and recovery.

This command provides comprehensive workflow management capabilities:
• Generate workflows from natural language descriptions
• Describe workflows with detailed information and beautiful visualization
• Test workflows with streaming output
• Execute workflows from files or inline with reliable connection handling
• Run stored workflows by ID or name (like composer-ui "test" button)
• Resume interrupted workflow executions
• Compose complex workflows from simpler ones`,
		Example: `  # Generate a workflow from description
  kubiya workflow generate "create a workflow to deploy an app"
  
  # Describe a workflow with detailed information
  kubiya workflow describe my-workflow.yaml
  kubiya workflow describe my-workflow.yaml --steps --env --deps --outputs
  
  # Test a workflow
  kubiya workflow test my-workflow.yaml
  
  # Execute a workflow from file with reliable connection handling
  kubiya workflow execute my-workflow.yaml

  # Run a stored workflow by ID or name
  kubiya workflow run "My Deploy Workflow" --var env=prod
  kubiya workflow run abc123-def456-789

  # List interrupted executions and resume
  kubiya workflow resume --list
  kubiya workflow resume exec_1234567890_123456

  # View workflow executions (like Composer UI executions page)
  kubiya workflow executions
  kubiya workflow executions --status running --limit 50

  # Compose workflows
  kubiya workflow compose --from deploy.yaml --from notify.yaml --output pipeline.yaml`,
	}

	// Add subcommands
	cmd.AddCommand(
		newWorkflowGenerateCommand(cfg),
		newWorkflowDescribeCommand(cfg),
		newWorkflowTestCommand(cfg),
		newWorkflowExecuteCommand(cfg),
		newWorkflowRunCommand(cfg), // Execute stored workflows by ID/name
		newWorkflowComposeCommand(cfg),
		newWorkflowResumeCommand(cfg),
		// Enhanced streaming and management commands
		newWorkflowStreamCommand(cfg),
		newWorkflowRetryCommand(cfg),
		newWorkflowListCommand(cfg),
		newWorkflowExecutionsCommand(cfg), // List executions with configurable time range
	)

	return cmd
}
