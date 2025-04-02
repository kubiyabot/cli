package cli

import (
	"github.com/spf13/afero"
	"github.com/spf13/cobra"

	"github.com/kubiyabot/cli/internal/config"
)

// newMcpCommand creates the `mcp` command group.
func newMcpCommand(cfg *config.Config) *cobra.Command {
	// Create the filesystem interface for real OS operations
	fs := afero.NewOsFs()

	cmd := &cobra.Command{
		Use:     "mcp",
		Aliases: []string{},
		Short:   "💻 Manage Model Context Protocol (MCP) integrations",
		Long: `Integrate Kubiya with local applications using the Model Context Protocol (MCP).

This command group allows you to set up, configure, and manage MCP integrations
for applications like Claude Desktop, Cursor IDE, and more.

It helps bridge your Kubiya context (like teammates and API keys) with the
configuration files of these local tools.`,
		// PersistentPreRunE: // Add authentication/config loading if needed globally for mcp commands
		RunE: func(cmd *cobra.Command, args []string) error {
			// Show help if no subcommand is provided
			return cmd.Help()
		},
	}

	// Add subcommands
	cmd.AddCommand(newMcpSetupCommand(cfg, fs))
	cmd.AddCommand(newMcpListCommand(cfg, fs))
	cmd.AddCommand(newMcpInstallCommand(cfg, fs))
	cmd.AddCommand(newMcpUpdateCommand(cfg, fs))
	cmd.AddCommand(newMcpApplyCommand(cfg, fs))
	cmd.AddCommand(newMcpEditCommand(cfg, fs))

	return cmd
}
