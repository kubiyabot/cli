package cli

import (
	"fmt"

	"github.com/kubiyabot/cli/internal/config"
	"github.com/kubiyabot/cli/internal/version"
	"github.com/spf13/cobra"
)

func newVersionCommand(cfg *config.Config) *cobra.Command {
	return &cobra.Command{
		Use:   "version",
		Short: "📋 Show CLI version information",
		Run: func(cmd *cobra.Command, args []string) {
			fmt.Printf("Kubiya CLI %s\n", version.GetVersion())

			// Check for updates
			if latest, hasUpdate, err := version.CheckForUpdate(); err == nil && hasUpdate {
				fmt.Printf("\n📢 Update available! Latest version: %s\n", latest)
				fmt.Println("Run 'kubiya update' to update to the latest version")
			}
		},
	}
}
