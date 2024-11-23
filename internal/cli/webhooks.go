package cli

import (
	"encoding/json"
	"fmt"
	"os"
	"strings"
	"text/tabwriter"

	"github.com/kubiyabot/cli/internal/config"
	"github.com/kubiyabot/cli/internal/kubiya"
	"github.com/spf13/cobra"
)

func newWebhooksCommand(cfg *config.Config) *cobra.Command {
	cmd := &cobra.Command{
		Use:     "webhook",
		Aliases: []string{"webhooks"},
		Short:   "🔗 Manage webhooks",
		Long:    `Create, read, update, and delete webhooks in your Kubiya workspace.`,
	}

	cmd.AddCommand(
		newListWebhooksCommand(cfg),
		newGetWebhookCommand(cfg),
		newCreateWebhookCommand(cfg),
		newUpdateWebhookCommand(cfg),
		newDeleteWebhookCommand(cfg),
	)

	return cmd
}

func newListWebhooksCommand(cfg *config.Config) *cobra.Command {
	var outputFormat string

	cmd := &cobra.Command{
		Use:     "list",
		Short:   "📋 List all webhooks",
		Example: "  kubiya webhook list\n  kubiya webhook list --output json",
		RunE: func(cmd *cobra.Command, args []string) error {
			client := kubiya.NewClient(cfg)
			webhooks, err := client.ListWebhooks(cmd.Context())
			if err != nil {
				return err
			}

			switch outputFormat {
			case "json":
				return json.NewEncoder(os.Stdout).Encode(webhooks)
			case "text":
				w := tabwriter.NewWriter(os.Stdout, 0, 0, 2, ' ', 0)
				fmt.Fprintln(w, "🔗 WEBHOOKS\n")
				fmt.Fprintln(w, "ID\tNAME\tSOURCE\tDESTINATION\tMETHOD")
				for _, wh := range webhooks {
					fmt.Fprintf(w, "%s\t%s\t%s\t%s\t%s\n",
						wh.ID,
						wh.Name,
						wh.Source,
						wh.Communication.Destination,
						wh.Communication.Method,
					)
				}
				return w.Flush()
			default:
				return fmt.Errorf("unknown output format: %s", outputFormat)
			}
		},
	}

	cmd.Flags().StringVarP(&outputFormat, "output", "o", "text", "Output format (text|json)")
	return cmd
}

func newGetWebhookCommand(cfg *config.Config) *cobra.Command {
	var outputFormat string

	cmd := &cobra.Command{
		Use:     "get [id]",
		Short:   "📖 Get webhook details",
		Example: "  kubiya webhook get abc-123\n  kubiya webhook get abc-123 --output json",
		Args:    cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			client := kubiya.NewClient(cfg)
			webhook, err := client.GetWebhook(cmd.Context(), args[0])
			if err != nil {
				return err
			}

			switch outputFormat {
			case "json":
				return json.NewEncoder(os.Stdout).Encode(webhook)
			case "text":
				fmt.Printf("🔗 Webhook: %s\n\n", webhook.Name)
				fmt.Printf("ID: %s\n", webhook.ID)
				fmt.Printf("Source: %s\n", webhook.Source)
				fmt.Printf("Agent ID: %s\n", webhook.AgentID)
				fmt.Printf("Communication:\n")
				fmt.Printf("  Method: %s\n", webhook.Communication.Method)
				fmt.Printf("  Destination: %s\n", webhook.Communication.Destination)
				fmt.Printf("Filter: %s\n", webhook.Filter)
				fmt.Printf("Prompt: %s\n", webhook.Prompt)
				fmt.Printf("URL: %s\n", webhook.WebhookURL)
				return nil
			default:
				return fmt.Errorf("unknown output format: %s", outputFormat)
			}
		},
	}

	cmd.Flags().StringVarP(&outputFormat, "output", "o", "text", "Output format (text|json)")
	return cmd
}

func newCreateWebhookCommand(cfg *config.Config) *cobra.Command {
	var (
		name        string
		source      string
		agentID     string
		method      string
		destination string
		filter      string
		prompt      string
	)

	cmd := &cobra.Command{
		Use:   "create",
		Short: "📝 Create new webhook",
		Example: `  kubiya webhook create \
    --name "GitHub PR" \
    --source "github" \
    --agent-id "abc-123" \
    --method "Slack" \
    --destination "#devops" \
    --prompt "New PR: {{.event.pull_request.title}}"`,
		RunE: func(cmd *cobra.Command, args []string) error {
			webhook := kubiya.Webhook{
				Name:    name,
				Source:  source,
				AgentID: agentID,
				Communication: kubiya.Communication{
					Method:      method,
					Destination: destination,
				},
				Filter: filter,
				Prompt: prompt,
			}

			client := kubiya.NewClient(cfg)
			created, err := client.CreateWebhook(cmd.Context(), webhook)
			if err != nil {
				return err
			}

			fmt.Printf("✅ Created webhook: %s (%s)\n", created.Name, created.ID)
			fmt.Printf("📎 Webhook URL: %s\n", created.WebhookURL)
			return nil
		},
	}

	cmd.Flags().StringVarP(&name, "name", "n", "", "Webhook name")
	cmd.Flags().StringVarP(&source, "source", "s", "", "Event source")
	cmd.Flags().StringVarP(&agentID, "agent-id", "a", "", "Agent ID")
	cmd.Flags().StringVarP(&method, "method", "m", "Slack", "Communication method")
	cmd.Flags().StringVarP(&destination, "destination", "d", "", "Communication destination")
	cmd.Flags().StringVarP(&filter, "filter", "f", "", "Event filter")
	cmd.Flags().StringVarP(&prompt, "prompt", "p", "", "Agent prompt")

	cmd.MarkFlagRequired("name")
	cmd.MarkFlagRequired("source")
	cmd.MarkFlagRequired("agent-id")
	cmd.MarkFlagRequired("destination")
	cmd.MarkFlagRequired("prompt")

	return cmd
}

func newUpdateWebhookCommand(cfg *config.Config) *cobra.Command {
	var (
		name        string
		source      string
		agentID     string
		method      string
		destination string
		filter      string
		prompt      string
	)

	cmd := &cobra.Command{
		Use:   "update [id]",
		Short: "✏️ Update webhook",
		Example: `  kubiya webhook update abc-123 \
    --name "Updated Name" \
    --prompt "New prompt: {{.event}}"`,
		Args: cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			client := kubiya.NewClient(cfg)

			// Get existing webhook
			existing, err := client.GetWebhook(cmd.Context(), args[0])
			if err != nil {
				return err
			}

			// Update fields if provided
			if name != "" {
				existing.Name = name
			}
			if source != "" {
				existing.Source = source
			}
			if agentID != "" {
				existing.AgentID = agentID
			}
			if method != "" {
				existing.Communication.Method = method
			}
			if destination != "" {
				existing.Communication.Destination = destination
			}
			if filter != "" {
				existing.Filter = filter
			}
			if prompt != "" {
				existing.Prompt = prompt
			}

			// Show changes
			fmt.Println("\n📝 Review changes:")
			fmt.Printf("Name: %s\n", existing.Name)
			fmt.Printf("Source: %s\n", existing.Source)
			fmt.Printf("Agent ID: %s\n", existing.AgentID)
			fmt.Printf("Communication Method: %s\n", existing.Communication.Method)
			fmt.Printf("Communication Destination: %s\n", existing.Communication.Destination)
			fmt.Printf("Filter: %s\n", existing.Filter)
			fmt.Printf("Prompt: %s\n", existing.Prompt)

			fmt.Print("\nDo you want to proceed? [y/N] ")
			var confirm string
			fmt.Scanln(&confirm)
			if strings.ToLower(confirm) != "y" {
				return fmt.Errorf("update cancelled")
			}

			updated, err := client.UpdateWebhook(cmd.Context(), args[0], *existing)
			if err != nil {
				return err
			}

			fmt.Printf("✅ Updated webhook: %s\n", updated.Name)
			return nil
		},
	}

	cmd.Flags().StringVarP(&name, "name", "n", "", "New name")
	cmd.Flags().StringVarP(&source, "source", "s", "", "New event source")
	cmd.Flags().StringVarP(&agentID, "agent-id", "a", "", "New agent ID")
	cmd.Flags().StringVarP(&method, "method", "m", "", "New communication method")
	cmd.Flags().StringVarP(&destination, "destination", "d", "", "New communication destination")
	cmd.Flags().StringVarP(&filter, "filter", "f", "", "New event filter")
	cmd.Flags().StringVarP(&prompt, "prompt", "p", "", "New agent prompt")

	return cmd
}

func newDeleteWebhookCommand(cfg *config.Config) *cobra.Command {
	var force bool

	cmd := &cobra.Command{
		Use:     "delete [id]",
		Short:   "🗑️ Delete webhook",
		Example: "  kubiya webhook delete abc-123\n  kubiya webhook delete abc-123 --force",
		Args:    cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			client := kubiya.NewClient(cfg)

			// Get webhook details first for better feedback
			webhook, err := client.GetWebhook(cmd.Context(), args[0])
			if err != nil {
				return fmt.Errorf("failed to get webhook details: %w", err)
			}

			if !force {
				fmt.Printf("About to delete webhook:\n")
				fmt.Printf("  Name: %s\n", webhook.Name)
				fmt.Printf("  Source: %s\n", webhook.Source)
				fmt.Printf("  Destination: %s\n", webhook.Communication.Destination)
				fmt.Print("\nAre you sure you want to delete this webhook? [y/N] ")
				var confirm string
				fmt.Scanln(&confirm)
				if strings.ToLower(confirm) != "y" {
					return fmt.Errorf("deletion cancelled")
				}
			}

			if err := client.DeleteWebhook(cmd.Context(), args[0]); err != nil {
				return fmt.Errorf("failed to delete webhook: %w", err)
			}

			fmt.Printf("✅ Successfully deleted webhook: %s (%s)\n", webhook.Name, args[0])
			return nil
		},
	}

	cmd.Flags().BoolVarP(&force, "force", "f", false, "Skip confirmation prompt")
	return cmd
}
