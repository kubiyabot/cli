package cli

import (
	"github.com/kubiyabot/cli/internal/config"
	"github.com/kubiyabot/cli/internal/mcp"
	"github.com/spf13/afero"
	"github.com/spf13/cobra"
)

func newMcpServeCommand(cfg *config.Config, fs afero.Fs) *cobra.Command {
	var allowPlatformAPIs bool
	var configFile string

	cmd := &cobra.Command{
		Use:   "serve",
		Short: "🚀 Start the Kubiya MCP server",
		Long: `Start the Kubiya MCP (Model Context Protocol) server.

This server provides AI tools with access to Kubiya platform capabilities including:
- Tool execution with live streaming
- Platform management (runners, agents, integrations, sources)  
- Knowledge base access
- Workflow automation prompts
- Live resource browsing

The server communicates via stdio and can be used with AI tools like Claude Desktop and Cursor.`,
		PersistentPreRunE: func(cmd *cobra.Command, args []string) error {
			return requireAPIKey(cmd, cfg)
		},
		RunE: func(cmd *cobra.Command, args []string) error {
			// Load MCP server configuration
			serverConfig, err := mcp.LoadConfiguration(fs, configFile, allowPlatformAPIs)
			if err != nil {
				return err
			}

			// Create and start MCP server
			server := mcp.NewServer(cfg, serverConfig)
			return server.Start()
		},
	}

	cmd.Flags().BoolVar(&allowPlatformAPIs, "allow-platform-apis", false, "Enable platform APIs (runners, agents, integrations management)")
	cmd.Flags().StringVar(&configFile, "config", "", "MCP server config file path")

	return cmd
}