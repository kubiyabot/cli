package cli

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/google/uuid"
	"github.com/kubiyabot/cli/internal/config"
	"github.com/kubiyabot/cli/internal/style"
	"github.com/spf13/cobra"
)

func newTriggerCreateCommand(cfg *config.Config) *cobra.Command {
	var (
		workflowFile  string
		name          string
		webhookName   string
		customHeaders string
		payload       string
		encodeAs      string
		runner        string
		// Provider credentials (alternatives to env vars)
		ddAPIKey     string
		ddAppKey     string
		ddSite       string
		githubToken  string
		// GitHub specific
		repository string
		events     []string
		secret     string
	)

	cmd := &cobra.Command{
		Use:   "create <provider>",
		Short: "Create a new workflow trigger",
		Long: `Create a new trigger that will execute a workflow when external events occur.

Currently supported providers:
• datadog - Create Datadog webhook triggers for alerts and incidents  
• github - Create GitHub webhook triggers for repository events

Each provider requires specific configuration and environment variables.`,
		Example: `  # Create a Datadog webhook trigger
  kubiya trigger create datadog \
    --workflow my-workflow.yaml \
    --name "incident-response" \
    --webhook-name "kubiya-incident-webhook"
  
  # Create a GitHub webhook trigger
  kubiya trigger create github \
    --workflow deploy-on-push.yaml \
    --name "auto-deploy" \
    --repository "myorg/myrepo" \
    --events "push,pull_request"
  
  # Create with custom runner and payload
  kubiya trigger create datadog \
    --workflow emergency-response.yaml \
    --name "critical-alert" \
    --webhook-name "kubiya-critical-webhook" \
    --runner "production-runner" \
    --payload '{"alert": "$EVENT_MSG", "severity": "$PRIORITY"}'`,
		Args: cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			provider := TriggerProvider(strings.ToLower(args[0]))

			// Validate provider
			if provider != ProviderDatadog && provider != ProviderGitHub {
				return fmt.Errorf("unsupported provider: %s (supported: datadog, github)", provider)
			}

			// Validate required flags
			if workflowFile == "" {
				return fmt.Errorf("--workflow flag is required")
			}

			if name == "" {
				return fmt.Errorf("--name flag is required")
			}

			// Provider-specific validation
			if provider == ProviderGitHub {
				if repository == "" {
					return fmt.Errorf("--repository flag is required for GitHub provider")
				}
				if len(events) == 0 {
					return fmt.Errorf("--events flag is required for GitHub provider")
				}
			}

			// Validate workflow file exists
			if !filepath.IsAbs(workflowFile) {
				// Convert relative path to absolute
				abs, err := filepath.Abs(workflowFile)
				if err != nil {
					return fmt.Errorf("failed to resolve workflow path: %w", err)
				}
				workflowFile = abs
			}

			if _, err := os.Stat(workflowFile); os.IsNotExist(err) {
				return fmt.Errorf("workflow file does not exist: %s", workflowFile)
			}

			return createTrigger(cfg, provider, workflowFile, name, webhookName, customHeaders, payload, encodeAs, runner, repository, events, secret, ddAPIKey, ddAppKey, ddSite, githubToken)
		},
	}

	cmd.Flags().StringVarP(&workflowFile, "workflow", "w", "", "Path to the workflow file (required)")
	cmd.Flags().StringVarP(&name, "name", "n", "", "Human-readable name for the trigger (required)")
	cmd.Flags().StringVar(&webhookName, "webhook-name", "", "Name for the webhook in the external provider (defaults to trigger name)")
	cmd.Flags().StringVar(&customHeaders, "custom-headers", "", "Custom headers for the webhook (newline-separated)")
	cmd.Flags().StringVar(&payload, "payload", "", "Custom payload template for the webhook")
	cmd.Flags().StringVar(&encodeAs, "encode-as", "json", "Encoding format for the webhook payload")
	cmd.Flags().StringVar(&runner, "runner", "", "Runner to use for workflow execution (uses default if not specified)")
	
	// Provider credential flags (alternatives to environment variables)
	cmd.Flags().StringVar(&ddAPIKey, "dd-api-key", "", "Datadog API key (alternative to DD_API_KEY env var)")
	cmd.Flags().StringVar(&ddAppKey, "dd-app-key", "", "Datadog application key (alternative to DD_APPLICATION_KEY env var)")
	cmd.Flags().StringVar(&ddSite, "dd-site", "", "Datadog site (alternative to DD_SITE env var)")
	cmd.Flags().StringVar(&githubToken, "github-token", "", "GitHub token (alternative to GITHUB_TOKEN env var)")
	
	// GitHub specific flags
	cmd.Flags().StringVar(&repository, "repository", "", "GitHub repository in format 'owner/repo' (required for GitHub)")
	cmd.Flags().StringSliceVar(&events, "events", []string{"push"}, "GitHub events to trigger on (comma-separated)")
	cmd.Flags().StringVar(&secret, "secret", "", "Webhook secret for GitHub verification (optional)")

	return cmd
}

func createTrigger(cfg *config.Config, provider TriggerProvider, workflowFile, name, webhookName, customHeaders, payload, encodeAs, runner, repository string, events []string, secret, ddAPIKey, ddAppKey, ddSite, githubToken string) error {
	// Generate unique trigger ID
	triggerID := uuid.New().String()

	// Use name as webhook name if not specified
	if webhookName == "" {
		webhookName = strings.ReplaceAll(strings.ToLower(name), " ", "-")
	}

	// Create trigger configuration
	trigger := &Trigger{
		ID:          triggerID,
		Name:        name,
		Provider:    provider,
		WorkflowRef: workflowFile,
		Status:      StatusActive,
		CreatedAt:   time.Now(),
		UpdatedAt:   time.Now(),
		CreatedBy:   "cli-user", // TODO: Get actual user from config
	}

	// Configure provider-specific settings
	switch provider {
	case ProviderDatadog:
		trigger.Config = map[string]interface{}{
			"webhook_name":   webhookName,
			"custom_headers": customHeaders,
			"payload":        payload,
			"encode_as":      encodeAs,
		}

		// Add runner if specified
		if runner != "" {
			if trigger.Config["environment"] == nil {
				trigger.Config["environment"] = make(map[string]string)
			}
			trigger.Config["environment"].(map[string]string)["KUBIYA_RUNNER"] = runner
		}

		// Create the Datadog provider and trigger with credentials
		ddProvider, err := NewDatadogProviderWithCredentials(cfg, ddAPIKey, ddAppKey, ddSite)
		if err != nil {
			return fmt.Errorf("failed to initialize Datadog provider: %w", err)
		}

		// Validate configuration
		if err := ddProvider.ValidateConfig(trigger.Config); err != nil {
			return fmt.Errorf("invalid configuration: %w", err)
		}

		// Show warning about Datadog configuration
		fmt.Printf("\n%s\n", style.WarningStyle.Render("⚠️  IMPORTANT: Datadog Integration"))
		fmt.Printf("This will create a webhook in your Datadog account: %s\n", webhookName)
		fmt.Printf("Make sure you have the required environment variables set:\n")
		for _, envVar := range ddProvider.GetRequiredEnvVars() {
			if os.Getenv(envVar) == "" {
				fmt.Printf("  ❌ %s (not set)\n", envVar)
			} else {
				fmt.Printf("  ✅ %s (set)\n", envVar)
			}
		}
		fmt.Printf("Required Datadog permissions: Webhook management\n")

		// Create the trigger
		fmt.Printf("\nCreating Datadog webhook trigger...\n")
		if err := ddProvider.CreateTrigger(trigger); err != nil {
			return fmt.Errorf("failed to create trigger: %w", err)
		}

		// TODO: Store trigger configuration locally for management
		// For now, just display success message

		fmt.Printf("\n%s\n", style.SuccessStyle.Render("✅ Trigger created successfully!"))
		fmt.Printf("\n%s\n", style.InfoStyle.Render("Trigger Details:"))
		fmt.Printf("• ID: %s\n", triggerID)
		fmt.Printf("• Name: %s\n", name)
		fmt.Printf("• Provider: %s\n", provider)
		fmt.Printf("• Workflow: %s\n", workflowFile)
		fmt.Printf("• Webhook Name: %s\n", webhookName)

		if runner != "" {
			fmt.Printf("• Runner: %s\n", runner)
		}

		fmt.Printf("\n%s\n", style.InfoStyle.Render("Next Steps:"))
		fmt.Printf("1. Your Datadog webhook has been created and configured\n")
		fmt.Printf("2. You can now attach this webhook to Datadog monitors or alerting rules\n")
		fmt.Printf("3. Test the trigger with: kubiya trigger test %s\n", triggerID)
		fmt.Printf("4. View trigger details with: kubiya trigger describe %s\n", triggerID)

		return nil

	case ProviderGitHub:
		// Convert events slice to interface slice for validation
		eventInterfaces := make([]interface{}, len(events))
		for i, event := range events {
			eventInterfaces[i] = event
		}
		
		trigger.Config = map[string]interface{}{
			"repository": repository,
			"events":     eventInterfaces,
			"secret":     secret,
		}

		// Create the GitHub provider and trigger with credentials
		ghProvider, err := NewGitHubProviderWithCredentials(cfg, githubToken)
		if err != nil {
			return fmt.Errorf("failed to initialize GitHub provider: %w", err)
		}

		// Validate configuration
		if err := ghProvider.ValidateConfig(trigger.Config); err != nil {
			return fmt.Errorf("invalid configuration: %w", err)
		}

		// Show warning about permissions
		fmt.Printf("\n%s\n", style.WarningStyle.Render("⚠️  IMPORTANT: GitHub Integration"))
		fmt.Printf("This will create a webhook in the repository: %s\n", repository)
		fmt.Printf("Make sure you have admin access to this repository.\n")
		fmt.Printf("Required GitHub permissions: repo (full control of private repositories)\n")
		
		// Create the trigger
		fmt.Printf("\nCreating GitHub webhook trigger...\n")
		if err := ghProvider.CreateTrigger(trigger); err != nil {
			return fmt.Errorf("failed to create trigger: %w", err)
		}

		fmt.Printf("\n%s\n", style.SuccessStyle.Render("✅ Trigger created successfully!"))
		fmt.Printf("\n%s\n", style.InfoStyle.Render("Trigger Details:"))
		fmt.Printf("• ID: %s\n", triggerID)
		fmt.Printf("• Name: %s\n", name)
		fmt.Printf("• Provider: %s\n", provider)
		fmt.Printf("• Workflow: %s\n", workflowFile)
		fmt.Printf("• Repository: %s\n", repository)
		fmt.Printf("• Events: %s\n", strings.Join(events, ", "))
		
		if runner != "" {
			fmt.Printf("• Runner: %s\n", runner)
		}

		fmt.Printf("\n%s\n", style.InfoStyle.Render("Next Steps:"))
		fmt.Printf("1. Your GitHub webhook has been created and configured\n")
		fmt.Printf("2. The webhook will trigger on these events: %s\n", strings.Join(events, ", "))
		fmt.Printf("3. Test the trigger with: kubiya trigger test %s\n", triggerID)
		fmt.Printf("4. View trigger details with: kubiya trigger describe %s\n", triggerID)

		return nil

	default:
		return fmt.Errorf("provider %s is not yet implemented", provider)
	}
}
