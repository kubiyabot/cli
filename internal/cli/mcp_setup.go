package cli

import (
	"fmt"
	"os"
	"strings"

	"github.com/kubiyabot/cli/internal/config"
	"github.com/kubiyabot/cli/internal/mcp_helpers"
	"github.com/spf13/afero"
	"github.com/spf13/cobra"
)

func newMcpSetupCommand(cfg *config.Config, fs afero.Fs) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "setup",
		Short: "🚀 Initialize the MCP configuration directory and default providers",
		Long: `Creates the necessary MCP configuration file (~/.kubiya/mcp)
and populates it with default 


If configuration files already exist, they will not be overwritten.`,
		PersistentPreRunE: func(cmd *cobra.Command, args []string) error {
			// Check if API key is configured before running any teammate command
			return requireAPIKey(cmd, cfg)
		},
		RunE: func(cmd *cobra.Command, args []string) error {
			cfg, err := config.Load()
			cobra.CheckErr(err)

			teammateIds := os.Getenv("TEAMMATE_UUIDS")
			if strings.TrimSpace(teammateIds) == "" {
				fmt.Println("please set the TEAMMATE_UUIDS (comma separated)")
				os.Exit(0)
			}
			err = mcp_helpers.SaveMcpConfig(fs, cfg.APIKey, strings.Split(teammateIds, ","))
			if err != nil {
				return err
			}
			executable, err := os.Executable()
			cobra.CheckErr(err)

			fmt.Println("here's your mcp servers")
			fmt.Printf(`{
  "mcpServers": {
    "kubiya": {
      "command": "%s",
      "args": ["mcp", "serve"]
    }
  }
}

`, executable)
			return nil
		},
	}
	return cmd
}
