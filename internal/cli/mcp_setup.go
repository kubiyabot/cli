package cli

import (
	"fmt"
	"os"
	"path/filepath"

	"github.com/kubiyabot/cli/internal/config"
	"github.com/kubiyabot/cli/internal/mcp"
	"github.com/spf13/afero"
	"github.com/spf13/cobra"
)

func newMcpSetupCommand(cfg *config.Config, fs afero.Fs) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "setup",
		Short: "🚀 Initialize the MCP configuration directory and default providers",
		Long: `Creates the necessary MCP configuration directory (~/.kubiya/mcp)
and populates it with default configuration files for known applications
(e.g., Claude Desktop on macOS, Cursor IDE).

If configuration files already exist, they will not be overwritten.`,
		RunE: func(cmd *cobra.Command, args []string) error {
			cmd.SilenceUsage = true // Prevent usage from printing on error

			mcpDir, err := mcp.GetMcpConfigDir(fs)
			if err != nil {
				return fmt.Errorf("failed to get or create MCP config directory: %w", err)
			}
			fmt.Fprintf(cmd.OutOrStdout(), "✅ Ensured MCP configuration directory exists: %s\n", mcpDir)

			defaultConfigs := mcp.GetDefaultConfigs()
			if len(defaultConfigs) == 0 {
				fmt.Fprintln(cmd.OutOrStdout(), "ℹ️ No default MCP configurations applicable for your operating system.")
				return nil
			}

			var createdCount int
			for filename, content := range defaultConfigs {
				targetPath := filepath.Join(mcpDir, filename)

				if _, err := fs.Stat(targetPath); err == nil {
					fmt.Fprintf(cmd.OutOrStdout(), "🟡 Skipping existing configuration: %s\n", filename)
					continue
				} else if !os.IsNotExist(err) {
					fmt.Fprintf(cmd.ErrOrStderr(), "⚠️ Error checking file %s: %v. Skipping.\n", filename, err)
					continue
				}

				err = afero.WriteFile(fs, targetPath, []byte(content), 0640)
				if err != nil {
					fmt.Fprintf(cmd.ErrOrStderr(), "❌ Error writing default config %s: %v\n", filename, err)
					continue
				}
				fmt.Fprintf(cmd.OutOrStdout(), "➕ Created default configuration: %s\n", filename)
				createdCount++
			}

			if createdCount > 0 {
				fmt.Fprintf(cmd.OutOrStdout(), "\n✨ MCP setup complete. You can now list configurations with 'kubiya mcp list'.\n")
			} else {
				fmt.Fprintf(cmd.OutOrStdout(), "\n✨ MCP setup checked. All applicable default configurations already exist.\n")
			}

			return nil
		},
	}
	return cmd
}
