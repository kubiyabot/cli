package cli

import (
	"bufio"
	"fmt"
	"os"
	"strings"

	"github.com/kubiyabot/cli/internal/config"
	"github.com/kubiyabot/cli/internal/style"
	"github.com/spf13/cobra"
)

func newTriggerDeleteCommand(cfg *config.Config) *cobra.Command {
	var (
		force       bool
		// Provider credentials
		ddAPIKey    string
		ddAppKey    string
		ddSite      string
		githubToken string
		repository  string // For GitHub
	)

	cmd := &cobra.Command{
		Use:   "delete <provider> <webhook-name-or-id>",
		Short: "Delete a workflow trigger",
		Long: `Delete a workflow trigger and remove it from the external provider.

This command connects directly to the provider API to delete the webhook.

For Datadog: Use the webhook name
For GitHub: Use the webhook ID and specify --repository

This action cannot be undone.`,
		Example: `  # Delete a Datadog webhook
  kubiya trigger delete datadog my-webhook-name
  
  # Delete a GitHub webhook
  kubiya trigger delete github 12345 --repository myorg/myrepo
  
  # Delete without confirmation
  kubiya trigger delete datadog my-webhook-name --force`,
		Args: cobra.ExactArgs(2),
		RunE: func(cmd *cobra.Command, args []string) error {
			provider := TriggerProvider(strings.ToLower(args[0]))
			webhookID := args[1]
			return deleteTrigger(cfg, provider, webhookID, repository, force, ddAPIKey, ddAppKey, ddSite, githubToken)
		},
	}

	cmd.Flags().BoolVarP(&force, "force", "f", false, "Skip confirmation prompt")
	cmd.Flags().StringVar(&repository, "repository", "", "GitHub repository (required for GitHub provider)")
	
	// Provider credential flags
	cmd.Flags().StringVar(&ddAPIKey, "dd-api-key", "", "Datadog API key (alternative to DD_API_KEY env var)")
	cmd.Flags().StringVar(&ddAppKey, "dd-app-key", "", "Datadog application key (alternative to DD_APPLICATION_KEY env var)")
	cmd.Flags().StringVar(&ddSite, "dd-site", "", "Datadog site (alternative to DD_SITE env var)")
	cmd.Flags().StringVar(&githubToken, "github-token", "", "GitHub token (alternative to GITHUB_TOKEN env var)")

	return cmd
}

func deleteTrigger(cfg *config.Config, provider TriggerProvider, webhookID, repository string, force bool, ddAPIKey, ddAppKey, ddSite, githubToken string) error {
	fmt.Printf("%s\n", style.HeaderStyle.Render("🗑️ Delete Trigger"))

	// Validate provider
	if provider != ProviderDatadog && provider != ProviderGitHub {
		return fmt.Errorf("unsupported provider: %s (supported: datadog, github)", provider)
	}

	// Validate GitHub-specific requirements
	if provider == ProviderGitHub && repository == "" {
		return fmt.Errorf("--repository flag is required for GitHub provider")
	}

	if !force {
		fmt.Printf("\n%s\n", style.WarningStyle.Render("⚠️  WARNING: This action cannot be undone!"))
		fmt.Printf("\nThis will delete:\n")
		fmt.Printf("• Provider: %s\n", provider)
		fmt.Printf("• Webhook ID/Name: %s\n", webhookID)
		if provider == ProviderGitHub {
			fmt.Printf("• Repository: %s\n", repository)
		}

		fmt.Printf("\n%s", style.InfoStyle.Render("Are you sure you want to continue? (y/N): "))

		reader := bufio.NewReader(os.Stdin)
		response, err := reader.ReadString('\n')
		if err != nil {
			return fmt.Errorf("failed to read input: %w", err)
		}

		response = strings.TrimSpace(strings.ToLower(response))
		if response != "y" && response != "yes" {
			fmt.Printf("\n%s\n", style.InfoStyle.Render("❌ Deletion cancelled."))
			return nil
		}
	}

	fmt.Printf("\n%s Deleting webhook from %s...\n", style.InfoStyle.Render("🔄"), provider)

	// Delete based on provider
	switch provider {
	case ProviderDatadog:
		ddProvider, err := NewDatadogProviderWithCredentials(cfg, ddAPIKey, ddAppKey, ddSite)
		if err != nil {
			return fmt.Errorf("failed to initialize Datadog provider: %w", err)
		}

		if err := ddProvider.DeleteTrigger(webhookID); err != nil {
			return fmt.Errorf("failed to delete Datadog webhook: %w", err)
		}

	case ProviderGitHub:
		ghProvider, err := NewGitHubProviderWithCredentials(cfg, githubToken)
		if err != nil {
			return fmt.Errorf("failed to initialize GitHub provider: %w", err)
		}

		if err := ghProvider.DeleteTrigger(repository, webhookID); err != nil {
			return fmt.Errorf("failed to delete GitHub webhook: %w", err)
		}
	}

	fmt.Printf("\n%s\n", style.SuccessStyle.Render("✅ Webhook deleted successfully!"))

	fmt.Printf("\n%s\n", style.InfoStyle.Render("Cleanup Complete:"))
	fmt.Printf("• Webhook removed from %s\n", provider)
	fmt.Printf("• No further workflow executions will occur from this webhook\n")

	fmt.Printf("\n%s\n", style.InfoStyle.Render("💡 Next Steps:"))
	fmt.Printf("• List remaining webhooks: kubiya trigger list --provider %s\n", provider)
	fmt.Printf("• Create a new trigger: kubiya trigger create %s --workflow my-workflow.yaml --name my-trigger\n", provider)

	return nil
}
