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
		configFile                                                     string
		disablePlatformAPIs                                            bool
		productionMode                                                 bool
		whitelistedTools                                               []string
		disableRunners                                                 bool
		enableOPAPolicies                                              bool
		requireAuth                                                    bool
		sessionTimeout                                                 int
		serverName                                                     string
		serverVersion                                                  string
		disableDynamicTools, enableVerboseLogging, enableDocumentation bool
	)

	cmd := &cobra.Command{
		Use:   "serve",
		Short: "Start the Kubiya MCP server",
		Long:  `Start the Kubiya Model Context Protocol (MCP) server that exposes Kubiya tools to MCP clients.`,
		RunE: func(cmd *cobra.Command, args []string) error {
			// Load config
			cfg, err := config.Load()
			if err != nil {
				return fmt.Errorf("failed to load config: %w", err)
			}
			if cfg.APIKey == "" {
				return fmt.Errorf("API key not configured. Set KUBIYA_API_KEY environment variable or run 'kubiya init' first")
			}

			// Initialize Sentry if configured
			if err := sentryutil.Initialize("kubiya-cli"); err != nil {
				// Log but don't fail if Sentry initialization fails
			}
			defer sentryutil.Flush(2 * time.Second)

			if productionMode {
				// Load production configuration
				serverConfig, err := mcp.LoadProductionConfig(afero.NewOsFs(), configFile, disablePlatformAPIs, whitelistedTools)
				if err != nil {
					return fmt.Errorf("failed to load production config: %w", err)
				}

				// Override config with command line flags
				if disableRunners {
					serverConfig.EnableRunners = false
				}
				if enableOPAPolicies {
					serverConfig.EnableOPAPolicies = true
				}
				if requireAuth {
					serverConfig.RequireAuth = true
				}
				if sessionTimeout > 0 {
					serverConfig.SessionTimeout = sessionTimeout
				}
				if serverName != "" {
					serverConfig.ServerName = serverName
				}
				if serverVersion != "" {
					serverConfig.ServerVersion = serverVersion
				}
				if disableDynamicTools {
					serverConfig.AllowDynamicTools = false
				}
				if enableVerboseLogging {
					serverConfig.VerboseLogging = true
				}
				if enableDocumentation {
					serverConfig.EnableDocumentation = true
				}

				// Create Kubiya client
				kubiyaClient := kubiya.NewClient(cfg)

				// Set org ID from user config
				serverConfig.OrgID = cfg.Org

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
				serverConfig, err := mcp.LoadConfiguration(afero.NewOsFs(), configFile, disablePlatformAPIs, whitelistedTools)
				if err != nil {
					return fmt.Errorf("failed to load config: %w", err)
				}

				// Override config with command line flags
				if disableRunners {
					serverConfig.EnableRunners = false
				}
				if enableOPAPolicies {
					serverConfig.EnableOPAPolicies = true
				}
				if disableDynamicTools {
					serverConfig.AllowDynamicTools = false
				}
				if enableVerboseLogging {
					serverConfig.VerboseLogging = true
				}
				if enableDocumentation {
					serverConfig.EnableDocumentation = true
				}
				// Create and start server
				server := mcp.NewServer(cfg, serverConfig)
				return server.Start()
			}
		},
		Example: `  # Start MCP server with default settings (platform APIs enabled by default)
  kubiya mcp serve

  # Start MCP server with custom configuration
  kubiya mcp serve --config ~/my-mcp-config.json

  # Start MCP server with platform APIs disabled
  kubiya mcp serve --disable-platform-apis

  # Start MCP server with specific whitelisted tools
  kubiya mcp serve --whitelist-tools kubectl,helm,terraform

  # Start production MCP server with all features
  kubiya mcp serve --production --require-auth --session-timeout 3600

  # Start server with custom name and version
  kubiya mcp serve --server-name "My Kubiya Server" --server-version "2.0.0"

  # Disable runners and enable OPA policies
  kubiya mcp serve --disable-runners --enable-opa-policies

  # Enable kubiya documentation search
  kubiya mcp serve --enable-documentation

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

	// Configuration flags
	cmd.Flags().StringVarP(&configFile, "config", "c", "", "Path to MCP server configuration file")
	cmd.Flags().BoolVar(&productionMode, "production", false, "Run in production mode with session management, middleware, and hooks")

	// Core functionality flags
	cmd.Flags().BoolVar(&disablePlatformAPIs, "disable-platform-apis", false, "Disable access to platform management APIs (platform APIs are enabled by default)")
	cmd.Flags().BoolVar(&disableRunners, "disable-runners", false, "Disable tool runners")
	cmd.Flags().BoolVar(&enableOPAPolicies, "enable-opa-policies", false, "Enable OPA policy enforcement")

	// Tool configuration flags
	cmd.Flags().StringSliceVar(&whitelistedTools, "whitelist-tools", []string{}, "Comma-separated list of tools to whitelist (e.g., kubectl,helm,terraform)")
	cmd.Flags().BoolVar(&disableDynamicTools, "disable-dynamic-tools", false, "Disable dynamic tool generation (dynamic tools enabled by default)")

	// Logging and debugging flags
	cmd.Flags().BoolVar(&enableVerboseLogging, "verbose", false, "Enable verbose logging output")

	// Documentation flags
	cmd.Flags().BoolVar(&enableDocumentation, "enable-documentation", false, "Enable documentation features (experimental)")

	// Production mode specific flags
	cmd.Flags().BoolVar(&requireAuth, "require-auth", false, "Require authentication for MCP connections")
	cmd.Flags().IntVar(&sessionTimeout, "session-timeout", 0, "Session timeout in seconds (default: 1800)")
	cmd.Flags().StringVar(&serverName, "server-name", "", "Custom server name")
	cmd.Flags().StringVar(&serverVersion, "server-version", "", "Custom server version")

	return cmd
}
