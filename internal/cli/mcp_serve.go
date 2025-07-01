package cli

import (
	"context"
	"fmt"
	"time"

	"github.com/kubiyabot/cli/internal/config"
	"github.com/kubiyabot/cli/internal/kubiya"
	"github.com/kubiyabot/cli/internal/mcp"
	sentryutil "github.com/kubiyabot/cli/internal/sentry"
	"github.com/spf13/afero"
	"github.com/spf13/cobra"
)

func NewMCPServeCmd() *cobra.Command {
	var (
		configFile        string
		allowPlatformAPIs bool
		productionMode    bool
	)

	cmd := &cobra.Command{
		Use:   "serve",
		Short: "Start the Kubiya MCP server",
		Long:  `Start the Kubiya Model Context Protocol (MCP) server that exposes Kubiya tools to MCP clients.`,
		RunE: func(cmd *cobra.Command, args []string) error {
			// Load config
			cfg, _ := config.Load()
			if cfg.APIKey == "" {
				return fmt.Errorf("API key not configured. Run 'kubiya init' first")
			}

			// Initialize Sentry if configured
			if err := sentryutil.Initialize("kubiya-cli"); err != nil {
				// Log but don't fail if Sentry initialization fails
			}
			defer sentryutil.Flush(2 * time.Second)

			if productionMode {
				// Load production configuration
				serverConfig, err := mcp.LoadProductionConfig(afero.NewOsFs(), configFile, allowPlatformAPIs)
				if err != nil {
					return fmt.Errorf("failed to load production config: %w", err)
				}

				// Create Kubiya client
				kubiyaClient := kubiya.NewClient(cfg)

				// Create production server
				server, err := mcp.NewProductionServer(kubiyaClient, serverConfig)
				if err != nil {
					return fmt.Errorf("failed to create production server: %w", err)
				}

				// Start server
				ctx := context.Background()
				return server.Start(ctx)
			} else {
				// Use simple server for backward compatibility
				serverConfig, err := mcp.LoadConfiguration(afero.NewOsFs(), configFile, allowPlatformAPIs)
				if err != nil {
					return fmt.Errorf("failed to load config: %w", err)
				}

				// Create and start server
				server := mcp.NewServer(cfg, serverConfig)
				return server.Start()
			}
		},
		Example: `  # Start MCP server with default settings
  kubiya mcp serve

  # Start MCP server with custom configuration
  kubiya mcp serve --config ~/my-mcp-config.json

  # Start MCP server with platform APIs enabled
  kubiya mcp serve --allow-platform-apis

  # Start production MCP server with all features
  kubiya mcp serve --production

  # Configure in Claude Desktop (~/Library/Application Support/Claude/claude_desktop_config.json):
  {
    "mcpServers": {
      "kubiya": {
        "command": "kubiya",
        "args": ["mcp", "serve"],
        "env": {
          "KUBIYA_API_KEY": "your-api-key"
        }
      }
    }
  }`,
	}

	cmd.Flags().StringVarP(&configFile, "config", "c", "", "Path to MCP server configuration file")
	cmd.Flags().BoolVar(&allowPlatformAPIs, "allow-platform-apis", false, "Allow access to platform management APIs (create/update/delete operations)")
	cmd.Flags().BoolVar(&productionMode, "production", false, "Run in production mode with session management, middleware, and hooks")

	return cmd
}
