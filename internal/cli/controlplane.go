package cli

import (
	"github.com/kubiyabot/cli/internal/config"
	"github.com/spf13/cobra"
)

// newControlplaneCommand creates the control-plane command with subcommands
func newControlplaneCommand(cfg *config.Config) *cobra.Command {
	controlplaneCmd := &cobra.Command{
		Use:   "control-plane",
		Short: "🎯 Manage Kubiya Control Plane",
		Long: `Start and manage the Kubiya Control Plane API server locally.

The Control Plane is the central orchestration hub for agents, workflows,
and executions. It provides a REST API and manages connections to Temporal
workflows and database storage.`,
		Example: `  # Start control plane with default settings
  kubiya control-plane start

  # Start with custom database
  kubiya control-plane start --database-url=postgresql://localhost/control_plane

  # Start with Supabase
  kubiya control-plane start --supabase-url=https://xxx.supabase.co --supabase-key=xxx

  # Start on custom port
  kubiya control-plane start --port=8888`,
	}

	// Add subcommands
	controlplaneCmd.AddCommand(
		newControlPlaneStartCommand(cfg),
	)

	return controlplaneCmd
}
