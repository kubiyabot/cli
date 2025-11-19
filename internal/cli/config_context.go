package cli

import (
	"fmt"
	"os"

	"github.com/kubiyabot/cli/internal/context"
	"github.com/kubiyabot/cli/internal/style"
	"github.com/spf13/cobra"
	"gopkg.in/yaml.v3"
)

func NewConfigCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "config",
		Short: "Manage CLI configuration and contexts",
		Long:  "Manage CLI configuration and contexts (similar to kubectl config)",
	}

	cmd.AddCommand(
		newConfigGetContextsCmd(),
		newConfigCurrentContextCmd(),
		newConfigUseContextCmd(),
		newConfigSetContextCmd(),
		newConfigDeleteContextCmd(),
		newConfigRenameContextCmd(),
		newConfigViewCmd(),
	)

	return cmd
}

func newConfigGetContextsCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "get-contexts",
		Short: "List all configured contexts",
		RunE: func(cmd *cobra.Command, args []string) error {
			contexts, current, err := context.ListContexts()
			if err != nil {
				return fmt.Errorf("failed to list contexts: %w", err)
			}

			if len(contexts) == 0 {
				fmt.Println("No contexts configured. Use 'kubiya config set-context' to create one.")
				return nil
			}

			fmt.Println("CURRENT   NAME                API-URL                              ORGANIZATION    USER")
			for _, nc := range contexts {
				currentMarker := " "
				if nc.Name == current {
					currentMarker = style.SuccessStyle.Render("*")
				}

				v1Flag := ""
				if nc.Context.UseV1API {
					v1Flag = style.WarningStyle.Render(" (V1)")
				}

				fmt.Printf("%-9s %-19s %-36s %-15s %s%s\n",
					currentMarker,
					nc.Name,
					nc.Context.APIURL,
					nc.Context.Organization,
					nc.Context.User,
					v1Flag,
				)
			}

			return nil
		},
	}
}

func newConfigCurrentContextCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "current-context",
		Short: "Display the current context",
		RunE: func(cmd *cobra.Command, args []string) error {
			_, name, err := context.GetCurrentContext()
			if err != nil {
				return fmt.Errorf("failed to get current context: %w", err)
			}

			fmt.Println(name)
			return nil
		},
	}
}

func newConfigUseContextCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "use-context CONTEXT_NAME",
		Short: "Switch to a different context",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			contextName := args[0]

			if err := context.SetCurrentContext(contextName); err != nil {
				return fmt.Errorf("failed to switch context: %w", err)
			}

			fmt.Printf("%s Switched to context %s\n",
				style.SuccessStyle.Render("✓"),
				style.HighlightStyle.Render(contextName))
			return nil
		},
	}
}

func newConfigSetContextCmd() *cobra.Command {
	var (
		apiURL       string
		organization string
		user         string
		token        string
		useV1API     bool
	)

	cmd := &cobra.Command{
		Use:   "set-context CONTEXT_NAME",
		Short: "Create or update a context",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			contextName := args[0]

			// Check if context already exists
			existingCtx, _, _ := context.GetCurrentContext()
			if existingCtx != nil {
				// Update mode - use existing values as defaults
				if apiURL == "" {
					apiURL = existingCtx.APIURL
				}
				if organization == "" {
					organization = existingCtx.Organization
				}
				if user == "" {
					user = existingCtx.User
				}
			}

			// Validate required fields
			if apiURL == "" {
				apiURL = "https://control-plane.kubiya.ai"
			}
			if organization == "" {
				return fmt.Errorf("--organization is required")
			}
			if user == "" {
				return fmt.Errorf("--user is required")
			}

			// Create context
			ctx := context.Context{
				APIURL:       apiURL,
				Organization: organization,
				User:         user,
				UseV1API:     useV1API,
			}

			if err := context.CreateContext(contextName, ctx); err != nil {
				return fmt.Errorf("failed to create context: %w", err)
			}

			// Set or update user if token is provided
			if token != "" {
				if err := context.SetUser(user, context.User{Token: token}); err != nil {
					return fmt.Errorf("failed to set user: %w", err)
				}
			}

			fmt.Printf("%s Context %s created/updated\n",
				style.SuccessStyle.Render("✓"),
				style.HighlightStyle.Render(contextName))
			return nil
		},
	}

	cmd.Flags().StringVar(&apiURL, "api-url", "", "API URL (default: https://control-plane.kubiya.ai)")
	cmd.Flags().StringVar(&organization, "organization", "", "Organization name (required)")
	cmd.Flags().StringVar(&user, "user", "", "User name/email (required)")
	cmd.Flags().StringVar(&token, "token", "", "API token (if not provided, will use existing)")
	cmd.Flags().BoolVar(&useV1API, "use-v1-api", false, "Use V1 API (api.kubiya.ai) instead of control plane")

	return cmd
}

func newConfigDeleteContextCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "delete-context CONTEXT_NAME",
		Short: "Delete a context",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			contextName := args[0]

			if err := context.DeleteContext(contextName); err != nil {
				return fmt.Errorf("failed to delete context: %w", err)
			}

			fmt.Printf("%s Context %s deleted\n",
				style.SuccessStyle.Render("✓"),
				style.HighlightStyle.Render(contextName))
			return nil
		},
	}
}

func newConfigRenameContextCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "rename-context OLD_NAME NEW_NAME",
		Short: "Rename a context",
		Args:  cobra.ExactArgs(2),
		RunE: func(cmd *cobra.Command, args []string) error {
			oldName := args[0]
			newName := args[1]

			if err := context.RenameContext(oldName, newName); err != nil {
				return fmt.Errorf("failed to rename context: %w", err)
			}

			fmt.Printf("%s Context renamed from %s to %s\n",
				style.SuccessStyle.Render("✓"),
				style.HighlightStyle.Render(oldName),
				style.HighlightStyle.Render(newName))
			return nil
		},
	}
}

func newConfigViewCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "view",
		Short: "Display the full configuration file",
		RunE: func(cmd *cobra.Command, args []string) error {
			config, err := context.LoadConfig()
			if err != nil {
				return fmt.Errorf("failed to load config: %w", err)
			}

			// Marshal to YAML for display
			data, err := yaml.Marshal(config)
			if err != nil {
				return fmt.Errorf("failed to marshal config: %w", err)
			}

			fmt.Print(string(data))
			return nil
		},
	}
}

func init() {
	// Initialize config file if it doesn't exist
	if err := context.InitConfig(); err != nil {
		fmt.Fprintf(os.Stderr, "Warning: failed to initialize config: %v\n", err)
	}
}
