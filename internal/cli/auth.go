package cli

import (
	"fmt"

	"github.com/spf13/cobra"

	"github.com/kubiyabot/cli/internal/config"
)

// newAuthCommand creates a new auth command with subcommands
func newAuthCommand(cfg *config.Config) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "auth",
		Short: "Authentication management",
		Long:  "Manage authentication and view authentication status",
	}

	// Add subcommands
	cmd.AddCommand(newAuthStatusCommand(cfg))

	return cmd
}

// newAuthStatusCommand creates the auth status subcommand
func newAuthStatusCommand(cfg *config.Config) *cobra.Command {
	return &cobra.Command{
		Use:   "status",
		Short: "Check authentication status",
		Long:  "Display current authentication status including organization, user, and control plane URL",
		RunE: func(cmd *cobra.Command, args []string) error {
			return runAuthStatus(cfg)
		},
	}
}

// runAuthStatus displays the current authentication status
func runAuthStatus(cfg *config.Config) error {
	// Check if authenticated
	if cfg.APIKey == "" {
		fmt.Println("❌ Not authenticated")
		fmt.Println()
		fmt.Println("To authenticate, run:")
		fmt.Println("  kubiya login")
		fmt.Println()
		fmt.Println("Or set your API key:")
		fmt.Println("  export KUBIYA_API_KEY=your-api-key")
		fmt.Println()
		fmt.Println("Get your API key from: https://compose.kubiya.ai/settings#apiKeys")
		return nil
	}

	// Display authentication status
	fmt.Println("✅ Authenticated")

	if cfg.Org != "" {
		fmt.Printf("Organization: %s\n", cfg.Org)
	}

	if cfg.Email != "" {
		fmt.Printf("User: %s\n", cfg.Email)
	}

	if cfg.BaseURL != "" {
		fmt.Printf("Control Plane: %s\n", cfg.BaseURL)
	}

	// Show context if one is active
	if cfg.ContextName != "" {
		fmt.Printf("Context: %s\n", cfg.ContextName)
	}

	return nil
}
