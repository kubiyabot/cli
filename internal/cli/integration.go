package cli

import (
	"context"
	"fmt"
	"strings"

	"github.com/spf13/cobra"

	"github.com/kubiyabot/cli/internal/config"
	"github.com/kubiyabot/cli/internal/kubiya"
)

func newIntegrationsCommand(cfg *config.Config) *cobra.Command {
	cmd := &cobra.Command{
		Use:     "integration",
		Aliases: []string{"integrations"},
		Short:   "🔒 Manage integrations",
		Long:    `Create, read, update, and delete integrations used by tools and teammates.`,
	}

	cmd.AddCommand(
		newActivateIntegrationCommand(cfg),
	)

	return cmd
}

func newActivateIntegrationCommand(cfg *config.Config) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "activate [name]",
		Short: "➕ Activate Github App integration",
		Example: `  # Activate Github App Integration
  kubiya integration activate github_app`,
		Args: cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			switch args[0] {
			case "github_app":
				installUrl, err := githubApp(cmd.Context(), cfg)
				if err != nil {
					return err
				}

				go openUrl(installUrl)
				fmt.Printf("✅ GitHub App integration activated successfully!\nInstallation URL: %s\nPlease open this URL in your browser to complete the installation.", installUrl)
			default:
				return fmt.Errorf("integration type %s is not supported only: %s",
					args[0], strings.Join([]string{"github_app"}, ", "))
			}

			return nil
		},
	}

	return cmd
}

func githubApp(ctx context.Context, cfg *config.Config) (string, error) {
	const (
		name              = "github_app"
		alreadyExistError = "integration github app already exist"
		createError       = "failed to create github app integration"
	)

	c := kubiya.NewClient(cfg)
	item, err := c.GetIntegration(ctx, name)
	if err != nil {
		return "", err
	}

	if item != nil && item.Name == name {
		return "", fmt.Errorf(alreadyExistError)
	}

	installUrl, err := c.CreateGithubIntegration(ctx)
	if err != nil {
		return "", err
	}

	if len(installUrl) <= 0 {
		return "", fmt.Errorf(createError)
	}

	return installUrl, err
}
