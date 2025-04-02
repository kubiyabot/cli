package cli

import (
	"fmt"
	"os"
	"runtime"
	"strings"
	"text/tabwriter"

	"github.com/kubiyabot/cli/internal/config"
	"github.com/kubiyabot/cli/internal/mcp"
	"github.com/spf13/afero"
	"github.com/spf13/cobra"
)

// Accept fs afero.Fs
func newMcpListCommand(cfg *config.Config, fs afero.Fs) *cobra.Command {
	// fs is now passed in
	// fs := afero.NewOsFs()

	cmd := &cobra.Command{
		Use:     "list",
		Aliases: []string{"ls"},
		Short:   "📋 List configured MCP providers",
		Long:    `Scans the MCP configuration directory (~/.kubiya/mcp) and lists all found provider configurations.`,
		RunE: func(cmd *cobra.Command, args []string) error {
			cmd.SilenceUsage = true

			providers, err := mcp.ListProviders(fs)
			if err != nil {
				// The ListProviders function itself now checks for directory existence.
				// If it returned an error, it's likely a real read error, not just non-existence.
				// However, we might want to provide a friendlier message if the directory is empty/doesn't exist.

				// Let's check existence explicitly here for the user message.
				mcpDir, pathErr := mcp.GetMcpConfigDir(fs) // Get path, ignore creation error
				if pathErr == nil {                        // If we can get the path
					if _, statErr := fs.Stat(mcpDir); os.IsNotExist(statErr) {
						fmt.Fprintln(cmd.OutOrStdout(), " MCP configuration directory not found.")
						fmt.Fprintln(cmd.OutOrStdout(), " Run 'kubiya mcp setup' to initialize it.")
						return nil // Treat as non-error for user
					}
				}
				// If we got here, it's either a path error or a read error from ListProviders
				return fmt.Errorf("failed to list MCP providers: %w", err)
			}

			if len(providers) == 0 {
				fmt.Fprintln(cmd.OutOrStdout(), "ℹ️ No MCP provider configurations found in ~/.kubiya/mcp.")
				fmt.Fprintln(cmd.OutOrStdout(), " Run 'kubiya mcp setup' to add defaults, or create your own .yaml files.")
				return nil
			}

			// Print table header
			w := tabwriter.NewWriter(cmd.OutOrStdout(), 0, 0, 2, ' ', 0)
			fmt.Fprintln(w, "NAME\tOS\tSTATUS\tCONFIG FILE")
			fmt.Fprintln(w, "----\t--\t------\t----------- ") // Separator adjusted slightly

			currentOS := runtime.GOOS
			for _, p := range providers {
				status := "-" // Placeholder for now
				if p.OS != "" && p.OS != currentOS {
					status = fmt.Sprintf("Inactive (Requires %s)", p.OS)
				} else if p.OS == currentOS || p.OS == "" {
					// TODO: Add real status check here (e.g., is mcp-gateway installed? is target file generated?)
					status = "Pending"
				}

				// Clean up OS display (e.g., "darwin" -> "macOS")
				displayOS := p.OS
				if displayOS == "darwin" {
					displayOS = "macOS"
				}
				if displayOS == "" {
					displayOS = "any"
				} // If OS field is omitted
				displayOS = strings.Title(displayOS)

				fmt.Fprintf(w, "%s\t%s\t%s\t%s\n",
					p.Name,
					displayOS,
					status,
					p.Filename,
				)
			}

			w.Flush()
			return nil
		},
	}
	return cmd
}
