package cli

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"

	"github.com/kubiyabot/cli/internal/config"
	"github.com/kubiyabot/cli/internal/mcp"
	"github.com/spf13/afero"
	"github.com/spf13/cobra"
)

func newMcpSetupCommand(cfg *config.Config, fs afero.Fs) *cobra.Command {
	var allowPlatformAPIs bool
	var configFile string

	cmd := &cobra.Command{
		Use:   "setup",
		Short: "🚀 Setup MCP server configuration",
		Long: `Setup Kubiya MCP server configuration for use with AI tools like Claude Desktop, Cursor, and others.

This command creates:
- MCP server configuration file
- Sample client configurations
- Environment setup instructions`,
		PersistentPreRunE: func(cmd *cobra.Command, args []string) error {
			return requireAPIKey(cmd, cfg)
		},
		RunE: func(cmd *cobra.Command, args []string) error {
			executable, err := os.Executable()
			if err != nil {
				return fmt.Errorf("failed to get executable path: %w", err)
			}

			// Create default MCP server config if it doesn't exist
			homeDir, err := os.UserHomeDir()
			if err != nil {
				return fmt.Errorf("failed to get home directory: %w", err)
			}

			kubiyaDir := filepath.Join(homeDir, ".kubiya")
			if err := fs.MkdirAll(kubiyaDir, 0755); err != nil {
				return fmt.Errorf("failed to create .kubiya directory: %w", err)
			}

			mcpConfigPath := filepath.Join(kubiyaDir, "mcp-server.json")
			
			// Create default MCP server configuration
			defaultConfig := &mcp.Configuration{
				EnableRunners:     true,
				AllowPlatformAPIs: allowPlatformAPIs,
				WhitelistedTools: []mcp.WhitelistedTool{
					{
						Name:        "kubectl",
						Description: "Kubernetes command-line tool",
						ToolName:    "kubectl",
					},
					{
						Name:        "aws-cli",
						Description: "AWS command-line interface",
						ToolName:    "aws",
					},
				},
			}

			configData, err := json.MarshalIndent(defaultConfig, "", "  ")
			if err != nil {
				return fmt.Errorf("failed to marshal config: %w", err)
			}

			if err := afero.WriteFile(fs, mcpConfigPath, configData, 0644); err != nil {
				return fmt.Errorf("failed to write config file: %w", err)
			}

			fmt.Printf("✅ Created MCP server configuration at: %s\n\n", mcpConfigPath)

			// Display Claude Desktop configuration
			fmt.Println("📋 Claude Desktop Configuration")
			fmt.Println("Add this to your Claude Desktop settings:")
			fmt.Printf(`{
  "mcpServers": {
    "kubiya": {
      "command": "%s",
      "args": ["mcp", "serve"]
    }
  }
}

`, executable)

			// Display Cursor IDE configuration
			fmt.Println("📋 Cursor IDE Configuration")
			fmt.Println("Add this to your Cursor settings:")
			fmt.Printf(`{
  "mcpServers": {
    "kubiya": {
      "command": "%s",
      "args": ["mcp", "serve"]
    }
  }
}

`, executable)

			// Display environment variables
			fmt.Println("🔧 Environment Variables")
			fmt.Printf("Set these environment variables:\n")
			fmt.Printf("export KUBIYA_API_KEY=%s\n", cfg.APIKey)
			if allowPlatformAPIs {
				fmt.Printf("export KUBIYA_MCP_ALLOW_PLATFORM_APIS=true\n")
			}
			fmt.Printf("export KUBIYA_MCP_ENABLE_RUNNERS=true\n\n")

			// Display available tools info
			fmt.Println("🛠️  Available MCP Tools:")
			fmt.Println("- execute_tool: Execute Kubiya tools with live streaming")
			if allowPlatformAPIs {
				fmt.Println("- create_runner: Create new runners")
				fmt.Println("- delete_runner: Delete runners")
				fmt.Println("- list_runners: List all runners")
				fmt.Println("- create_integration: Create integrations")
				fmt.Println("- list_integrations: List integrations")
				fmt.Println("- list_agents: List agents")
				fmt.Println("- chat_with_agent: Chat with specific agents")
				fmt.Println("- list_sources: List tool sources")
				fmt.Println("- create_source: Create new tool sources")
				fmt.Println("- And more platform management tools...")
			}
			fmt.Println()

			fmt.Println("🎯 MCP Prompts Available:")
			fmt.Println("- kubernetes_troubleshooting: Kubernetes troubleshooting workflows")
			fmt.Println("- deployment_automation: Deployment automation workflows")
			fmt.Println("- agent_communication: Inter-agent communication workflows")
			fmt.Println()

			fmt.Println("📊 MCP Resources Available:")
			fmt.Println("- runners_list: Live list of runners")
			fmt.Println("- agents_list: Live list of agents")
			fmt.Println("- integrations_list: Live list of integrations")
			fmt.Println()

			fmt.Println("🚀 Next Steps:")
			fmt.Println("1. Restart your AI tool (Claude Desktop/Cursor)")
			fmt.Println("2. Test the connection with: kubiya mcp serve")
			fmt.Printf("3. Configure tools at: %s\n", mcpConfigPath)

			return nil
		},
	}

	cmd.Flags().BoolVar(&allowPlatformAPIs, "allow-platform-apis", false, "Enable platform APIs (runners, agents, integrations management)")
	cmd.Flags().StringVar(&configFile, "config", "", "MCP server config file path")
	
	return cmd
}
