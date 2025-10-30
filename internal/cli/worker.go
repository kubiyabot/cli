package cli

import (
	"github.com/kubiyabot/cli/internal/config"
	"github.com/spf13/cobra"
)

// newWorkerCommand creates the worker command
func newWorkerCommand(cfg *config.Config) *cobra.Command {
	workerCmd := &cobra.Command{
		Use:   "worker",
		Short: "🔧 Manage agent workers",
		Long:  `Start and manage agent workers for executing tasks`,
	}

	workerCmd.AddCommand(
		newWorkerStartCommand(cfg),
		newWorkerStatusCommand(cfg),
		newWorkerStopCommand(cfg),
	)

	return workerCmd
}
