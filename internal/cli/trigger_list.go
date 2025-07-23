package cli

import (
	"fmt"
	"os"
	"strings"
	"text/tabwriter"

	"github.com/kubiyabot/cli/internal/config"
	"github.com/kubiyabot/cli/internal/style"
	"github.com/spf13/cobra"
)

func newTriggerListCommand(cfg *config.Config) *cobra.Command {
	var (
		provider   string
		kubiyaOnly bool
		// Provider credentials
		ddAPIKey    string
		ddAppKey    string
		ddSite      string
		githubToken string
		repository  string // For GitHub
	)

	cmd := &cobra.Command{
		Use:   "list",
		Short: "List all workflow triggers",
		Long: `List all configured workflow triggers across all providers.

This command connects to the actual provider APIs to list webhooks and identifies
which ones are configured to call Kubiya API endpoints.

You can filter by provider and show only Kubiya-related webhooks.`,
		Example: `  # List all triggers from all providers
  kubiya trigger list
  
  # List only Datadog webhooks
  kubiya trigger list --provider datadog
  
  # List only Kubiya-related webhooks
  kubiya trigger list --kubiya-only
  
  # List GitHub webhooks for a specific repository
  kubiya trigger list --provider github --repository myorg/myrepo`,
		RunE: func(cmd *cobra.Command, args []string) error {
			return listTriggers(cfg, provider, kubiyaOnly, repository, ddAPIKey, ddAppKey, ddSite, githubToken)
		},
	}

	cmd.Flags().StringVar(&provider, "provider", "", "Filter by provider (datadog, github)")
	cmd.Flags().BoolVar(&kubiyaOnly, "kubiya-only", false, "Show only webhooks that point to Kubiya API")
	cmd.Flags().StringVar(&repository, "repository", "", "GitHub repository (required for GitHub provider)")
	
	// Provider credential flags
	cmd.Flags().StringVar(&ddAPIKey, "dd-api-key", "", "Datadog API key (alternative to DD_API_KEY env var)")
	cmd.Flags().StringVar(&ddAppKey, "dd-app-key", "", "Datadog application key (alternative to DD_APPLICATION_KEY env var)")
	cmd.Flags().StringVar(&ddSite, "dd-site", "", "Datadog site (alternative to DD_SITE env var)")
	cmd.Flags().StringVar(&githubToken, "github-token", "", "GitHub token (alternative to GITHUB_TOKEN env var)")

	return cmd
}

func listTriggers(cfg *config.Config, providerFilter string, kubiyaOnly bool, repository, ddAPIKey, ddAppKey, ddSite, githubToken string) error {
	fmt.Printf("%s\n", style.HeaderStyle.Render("📋 Workflow Triggers"))
	
	var allWebhooks []WebhookInfo
	
	// List Datadog webhooks if requested or no provider filter
	if providerFilter == "" || providerFilter == "datadog" {
		fmt.Printf("\n🔍 Checking Datadog webhooks...\n")
		ddProvider, err := NewDatadogProviderWithCredentials(cfg, ddAPIKey, ddAppKey, ddSite)
		if err != nil {
			fmt.Printf("⚠️  Skipping Datadog: %v\n", err)
		} else {
			webhooks, err := ddProvider.ListWebhooks()
			if err != nil {
				fmt.Printf("❌ Failed to list Datadog webhooks: %v\n", err)
			} else {
				allWebhooks = append(allWebhooks, webhooks...)
				fmt.Printf("✅ Found %d Datadog webhooks\n", len(webhooks))
			}
		}
	}
	
	// List GitHub webhooks if requested or no provider filter
	if providerFilter == "" || providerFilter == "github" {
		if repository == "" && providerFilter == "github" {
			return fmt.Errorf("--repository flag is required when filtering by GitHub provider")
		}
		
		if repository != "" {
			fmt.Printf("\n🔍 Checking GitHub webhooks for %s...\n", repository)
			ghProvider, err := NewGitHubProviderWithCredentials(cfg, githubToken)
			if err != nil {
				fmt.Printf("⚠️  Skipping GitHub: %v\n", err)
			} else {
				webhooks, err := ghProvider.ListWebhooks(repository)
				if err != nil {
					fmt.Printf("❌ Failed to list GitHub webhooks: %v\n", err)
				} else {
					allWebhooks = append(allWebhooks, webhooks...)
					fmt.Printf("✅ Found %d GitHub webhooks\n", len(webhooks))
				}
			}
		}
	}

	// Filter by Kubiya-only if requested
	if kubiyaOnly {
		var kubiyaWebhooks []WebhookInfo
		for _, webhook := range allWebhooks {
			if webhook.IsKubiya {
				kubiyaWebhooks = append(kubiyaWebhooks, webhook)
			}
		}
		allWebhooks = kubiyaWebhooks
	}

	if len(allWebhooks) == 0 {
		fmt.Printf("\n%s\n", style.InfoStyle.Render("No webhooks found."))
		if kubiyaOnly {
			fmt.Printf("No Kubiya-related webhooks found. Create your first trigger with:\n")
		} else {
			fmt.Printf("No webhooks found. Create your first trigger with:\n")
		}
		fmt.Printf("kubiya trigger create datadog --workflow my-workflow.yaml --name my-trigger\n")
		return nil
	}

	// Create table writer
	fmt.Printf("\n")
	w := tabwriter.NewWriter(os.Stdout, 0, 0, 2, ' ', 0)
	fmt.Fprintln(w, "PROVIDER\tNAME/ID\tURL\tKUBIYA\tEVENTS")

	for _, webhook := range allWebhooks {
		kubiyaIcon := "❌"
		if webhook.IsKubiya {
			kubiyaIcon = "✅"
		}

		nameOrID := webhook.Name
		if nameOrID == "" || nameOrID == "web" {
			nameOrID = webhook.ID
		}

		events := "N/A"
		if len(webhook.Events) > 0 {
			events = strings.Join(webhook.Events, ", ")
		}

		// Truncate long URLs for display
		displayURL := webhook.URL
		if len(displayURL) > 50 {
			displayURL = displayURL[:47] + "..."
		}

		fmt.Fprintf(w, "%s\t%s\t%s\t%s\t%s\n",
			webhook.Provider,
			nameOrID,
			displayURL,
			kubiyaIcon,
			events,
		)
	}

	w.Flush()

	// Show summary
	kubiyaCount := 0
	for _, webhook := range allWebhooks {
		if webhook.IsKubiya {
			kubiyaCount++
		}
	}

	fmt.Printf("\n%s\n", style.InfoStyle.Render("📊 Summary"))
	fmt.Printf("• Total webhooks: %d\n", len(allWebhooks))
	fmt.Printf("• Kubiya-related: %d\n", kubiyaCount)
	fmt.Printf("• Other webhooks: %d\n", len(allWebhooks)-kubiyaCount)

	fmt.Printf("\n%s\n", style.InfoStyle.Render("💡 Next Steps:"))
	fmt.Printf("• Delete webhook: kubiya trigger delete <provider> <name/id>\n")
	fmt.Printf("• Create new trigger: kubiya trigger create <provider> --workflow my-workflow.yaml --name my-trigger\n")

	return nil
}
