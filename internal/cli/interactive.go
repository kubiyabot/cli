package cli

import (
	"github.com/kubiyabot/cli/internal/config"
	"github.com/kubiyabot/cli/internal/tui"
	"github.com/spf13/cobra"
)

func newInteractiveCommand(cfg *config.Config) *cobra.Command {
	return &cobra.Command{
		Use:   "interactive",
		Short: "Start interactive mode",
		RunE: func(cmd *cobra.Command, args []string) error {
			app := tui.New(cfg)
			return app.Run()
		},
	}
}
