package cli

import (
	"bufio"
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"os/signal"
	"regexp"
	"sort"
	"strings"
	"text/tabwriter"
	"time"

	"github.com/charmbracelet/lipgloss"
	"github.com/kubiyabot/cli/internal/config"
	"github.com/kubiyabot/cli/internal/kubiya"
	"github.com/kubiyabot/cli/internal/style"
	"github.com/spf13/cobra"
	"gopkg.in/yaml.v3"
)

func newWebhooksCommand(cfg *config.Config) *cobra.Command {
	cmd := &cobra.Command{
		Use:     "webhook",
		Aliases: []string{"webhooks"},
		Short:   "🔗 Manage webhooks",
		Long: `Create, read, update, and delete webhooks in your Kubiya workspace.

Webhooks allow you to trigger agents or workflows in response to external events.
Two types of webhooks are supported:

• Agent Webhooks: Trigger existing agents with custom prompts
• Workflow Webhooks: Execute workflows directly using inline agents

For detailed documentation, see:
• docs/webhook-guide.md - Complete webhook guide
• docs/webhook-examples.md - Practical examples
• docs/webhook-workflow-reference.md - Technical reference`,
	}

	cmd.AddCommand(
		newListWebhooksCommand(cfg),
		newDescribeWebhookCommand(cfg),
		newCreateWebhookCommand(cfg),
		newUpdateWebhookCommand(cfg),
		newDeleteWebhookCommand(cfg),
		newTestWebhookCommand(cfg),
		newImportWebhookCommand(cfg),
		newExportWebhookCommand(cfg),
		newWizardWebhookCommand(cfg),
	)

	return cmd
}

func newListWebhooksCommand(cfg *config.Config) *cobra.Command {
	var outputFormat string
	var limit int

	cmd := &cobra.Command{
		Use:     "list",
		Short:   "📋 List all webhooks",
		Example: "  kubiya webhook list\n  kubiya webhook list --output json\n  kubiya webhook list --limit 10",
		RunE: func(cmd *cobra.Command, args []string) error {
			client := kubiya.NewClient(cfg)
			webhooks, err := client.ListWebhooks(cmd.Context())
			if err != nil {
				return err
			}

			// Apply limit if specified
			if limit > 0 && limit < len(webhooks) {
				webhooks = webhooks[:limit]
			}

			switch outputFormat {
			case "json":
				return json.NewEncoder(os.Stdout).Encode(webhooks)
			case "yaml":
				yamlData, err := yaml.Marshal(webhooks)
				if err != nil {
					return fmt.Errorf("failed to marshal webhooks to YAML: %w", err)
				}
				fmt.Println(string(yamlData))
				return nil
			case "wide":
				// Enhanced detailed tabular output with better design
				fmt.Printf("╭─────────────────────────────────────────────────────────────────────────────────────────────────╮\n")
				fmt.Printf("│ 🔗 %s                                                                 │\n", style.TitleStyle.Render("WEBHOOKS - DETAILED VIEW"))
				fmt.Printf("╰─────────────────────────────────────────────────────────────────────────────────────────────────╯\n\n")

				w := tabwriter.NewWriter(os.Stdout, 0, 0, 2, ' ', 0)

				// Write colored header
				fmt.Fprintln(w, style.HeaderStyle.Render("ID")+"\t"+
					style.HeaderStyle.Render("NAME")+"\t"+
					style.HeaderStyle.Render("TYPE")+"\t"+
					style.HeaderStyle.Render("SOURCE")+"\t"+
					style.HeaderStyle.Render("DESTINATION")+"\t"+
					style.HeaderStyle.Render("METHOD")+"\t"+
					style.HeaderStyle.Render("FILTER")+"\t"+
					style.HeaderStyle.Render("CREATED BY")+"\t"+
					style.HeaderStyle.Render("MANAGED BY"))

				fmt.Fprintln(w, "───────────────────────────────\t"+
					"───────────────────────\t"+
					"───────────\t"+
					"───────────\t"+
					"───────────────────────\t"+
					"───────────\t"+
					"───────────\t"+
					"───────────\t"+
					"───────────")

				if len(webhooks) == 0 {
					fmt.Fprintln(w, style.DimStyle.Render("<no webhooks found>"))
				} else {
					for _, wh := range webhooks {
						filter := wh.Filter
						if filter == "" {
							filter = style.DimStyle.Render("<none>")
						}

						managedBy := wh.ManagedBy
						if managedBy == "" {
							managedBy = style.DimStyle.Render("<none>")
						}

						// Determine webhook type
						webhookType := "🤖 Agent"
						if wh.Workflow != "" {
							webhookType = "📋 Workflow"
						}

						// Truncate long values
						name := truncateString(wh.Name, 25)
						source := truncateString(wh.Source, 12)
						destination := formatDestination(wh.Communication.Destination, wh.Communication.Method, 30)
						filter = truncateString(filter, 15)
						createdBy := truncateString(wh.CreatedBy, 15)
						managedBy = truncateString(managedBy, 15)

						fmt.Fprintf(w, "%s\t%s\t%s\t%s\t%s\t%s\t%s\t%s\t%s\n",
							style.DimStyle.Render(wh.ID),
							style.HighlightStyle.Render(name),
							webhookType,
							style.SubtitleStyle.Render(source),
							destination,
							getMethodWithIcon(wh.Communication.Method),
							filter,
							createdBy,
							managedBy,
						)
					}
				}

				if err := w.Flush(); err != nil {
					return err
				}

				printListTips(len(webhooks))
			case "text":
				fallthrough
			default:
				// Header section with adequate width
				title := style.TitleStyle.Render("WEBHOOKS")
				fmt.Printf("╭────────────────────────────────────────────────────────────────────────────────────────╮\n")
				fmt.Printf("│ 🔗 %-85s │\n", title)
				fmt.Printf("╰────────────────────────────────────────────────────────────────────────────────────────╯\n\n")

				// Use tabwriter with fixed-width settings (padchar is a space)
				// 0 minwidth, 8 tabwidth, 2 padding, ' ' padchar, 0 flags
				w := tabwriter.NewWriter(os.Stdout, 0, 8, 2, ' ', 0)

				// Define header columns with fixed spacing
				headers := []string{
					style.HeaderStyle.Render("ID"),
					style.HeaderStyle.Render("NAME"),
					style.HeaderStyle.Render("SOURCE"),
					style.HeaderStyle.Render("DESTINATION"),
					style.HeaderStyle.Render("METHOD"),
				}

				// Define separator lines with matching lengths
				separators := []string{
					"─────────────────────────────",
					"───────────────────────",
					"──────────",
					"───────────────────────",
					"──────────",
				}

				// Print headers and separator lines
				fmt.Fprintf(w, " %s\t%s\t%s\t%s\t%s\n", headers[0], headers[1], headers[2], headers[3], headers[4])
				fmt.Fprintf(w, " %s\t%s\t%s\t%s\t%s\n", separators[0], separators[1], separators[2], separators[3], separators[4])

				if len(webhooks) == 0 {
					fmt.Fprintln(w, style.DimStyle.Render(" <no webhooks found>"))
				} else {
					for _, wh := range webhooks {
						// Truncate long values for better display
						name := truncateString(wh.Name, 25)
						source := truncateString(wh.Source, 12)
						destination := formatDestination(wh.Communication.Destination, wh.Communication.Method, 25)

						fmt.Fprintf(w, " %s\t%s\t%s\t%s\t%s\n",
							style.DimStyle.Render(wh.ID),
							style.HighlightStyle.Render(name),
							style.SubtitleStyle.Render(source),
							destination,
							getMethodWithIcon(wh.Communication.Method),
						)
					}
				}

				if err := w.Flush(); err != nil {
					return err
				}

				printListTips(len(webhooks))
				return nil
			}
			return nil
		},
	}

	cmd.Flags().StringVarP(&outputFormat, "output", "o", "text", "Output format (text|wide|json|yaml)")
	cmd.Flags().IntVarP(&limit, "limit", "l", 0, "Limit the number of webhooks to display")
	return cmd
}

// Helper function to print list command tips
func printListTips(webhookCount int) {
	fmt.Printf("\n%s\n", style.SubtitleStyle.Render("💡 Tips:"))
	fmt.Printf("  • %s to see detailed information\n", style.CommandStyle.Render("kubiya webhook describe <id>"))
	fmt.Printf("  • %s to see additional fields\n", style.CommandStyle.Render("--output wide"))
	fmt.Printf("  • %s or %s for machine-readable output\n",
		style.CommandStyle.Render("--output json"),
		style.CommandStyle.Render("--output yaml"))

	if webhookCount > 10 {
		fmt.Printf("  • %s to limit the displayed results\n",
			style.CommandStyle.Render("--limit <number>"))
	}
}

// formatDestination formats the destination based on the method type
func formatDestination(destination string, method string, maxLen int) string {
	switch strings.ToLower(method) {
	case "teams":
		if strings.HasPrefix(destination, "#{") && strings.HasSuffix(destination, "}") {
			// Extract team and channel from Teams destination
			jsonStr := destination[1:] // Remove the leading #
			var teamsConfig map[string]string
			if err := json.Unmarshal([]byte(jsonStr), &teamsConfig); err == nil {
				teamName := teamsConfig["team_name"]
				channelName := teamsConfig["channel_name"]
				if teamName != "" && channelName != "" {
					formatted := fmt.Sprintf("%s:%s", teamName, channelName)
					return style.HighlightStyle.Render(truncateString(formatted, maxLen))
				}
			}
		}
	case "slack":
		if strings.HasPrefix(destination, "#") {
			// Highlight Slack channels
			return style.SuccessStyle.Render(truncateString(destination, maxLen))
		} else if strings.HasPrefix(destination, "@") {
			// Highlight Slack users
			return style.HighlightStyle.Render(truncateString(destination, maxLen))
		} else if strings.Contains(destination, "@") {
			// Likely an email address
			return style.WarningStyle.Render(truncateString(destination, maxLen))
		}
	case "http":
		if destination == "" {
			// HTTP with no destination is SSE stream
			return style.WarningStyle.Render("HTTP SSE Stream")
		}
		return style.WarningStyle.Render(truncateString(destination, maxLen))
	}

	// Default formatting for other cases
	return style.DimStyle.Render(truncateString(destination, maxLen))
}

// getMethodWithIcon returns the method with an appropriate icon
func getMethodWithIcon(method string) string {
	switch strings.ToLower(method) {
	case "slack":
		return style.SuccessStyle.Render("💬 Slack")
	case "teams":
		return style.SubtitleStyle.Render("👥 Teams")
	case "http":
		return style.WarningStyle.Render("🌐 HTTP")
	default:
		return style.DimStyle.Render(method)
	}
}

// Helper function to truncate long strings
func truncateString(s string, maxLen int) string {
	if len(s) <= maxLen {
		return s
	}
	return s[:maxLen-3] + "..."
}

// Helper function to get color style based on method
func getMethodStyle(method string) lipgloss.Style {
	switch strings.ToLower(method) {
	case "slack":
		return style.SuccessStyle
	case "teams":
		return style.SubtitleStyle
	case "http":
		return style.WarningStyle
	default:
		return style.DimStyle
	}
}

func newDescribeWebhookCommand(cfg *config.Config) *cobra.Command {
	var outputFormat string

	cmd := &cobra.Command{
		Use:     "describe [id]",
		Aliases: []string{"get", "info", "show"},
		Short:   "📖 Show detailed webhook information",
		Example: "  kubiya webhook describe abc-123\n  kubiya webhook describe abc-123 --output json",
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
			case "yaml":
				yamlData, err := yaml.Marshal(webhook)
				if err != nil {
					return fmt.Errorf("failed to marshal webhook to YAML: %w", err)
				}
				fmt.Println(string(yamlData))
				return nil
			case "text":
				fallthrough
			default:
				// Enhanced text output with better formatting
				fmt.Printf("╭─────────────────────────────────────────────────╮\n")
				fmt.Printf("│ 🔗 Webhook: %-37s │\n", style.HighlightStyle.Render(webhook.Name))
				fmt.Printf("╰─────────────────────────────────────────────────╯\n\n")

				// Basic Information section
				fmt.Printf("%s\n", style.SubtitleStyle.Render("📋 Basic Information"))
				fmt.Printf("   ID:       %s\n", style.DimStyle.Render(webhook.ID))
				fmt.Printf("   Source:   %s\n", style.SubtitleStyle.Render(webhook.Source))
				
				// Type and target information
				if webhook.Workflow != "" {
					fmt.Printf("   Type:     %s\n", style.HighlightStyle.Render("📋 Workflow"))
					fmt.Printf("   Agent ID: %s\n", style.DimStyle.Render(webhook.AgentID))
					if webhook.Runner != "" {
						fmt.Printf("   Runner:   %s\n", style.SubtitleStyle.Render(webhook.Runner))
					}
				} else {
					fmt.Printf("   Type:     %s\n", style.HighlightStyle.Render("🤖 Agent"))
					fmt.Printf("   Agent ID: %s\n", style.DimStyle.Render(webhook.AgentID))
				}

				// Communication section
				fmt.Printf("\n%s\n", style.SubtitleStyle.Render("🔌 Communication"))
				fmt.Printf("   Method: %s\n", getMethodStyle(webhook.Communication.Method).Render(webhook.Communication.Method))

				// Handle different communication methods
				switch webhook.Communication.Method {
				case "Slack":
					fmt.Printf("   Channel: %s\n", style.HighlightStyle.Render(webhook.Communication.Destination))
				case "Teams":
					if strings.HasPrefix(webhook.Communication.Destination, "#{") && strings.HasSuffix(webhook.Communication.Destination, "}") {
						// Extract the JSON part
						jsonStr := webhook.Communication.Destination[1:] // Remove the leading #
						var teamsConfig map[string]string
						if err := json.Unmarshal([]byte(jsonStr), &teamsConfig); err == nil {
							fmt.Printf("   Team:    %s\n", style.HighlightStyle.Render(teamsConfig["team_name"]))
							fmt.Printf("   Channel: %s\n", style.HighlightStyle.Render(teamsConfig["channel_name"]))
						} else {
							fmt.Printf("   Destination: %s\n", webhook.Communication.Destination)
						}
					} else {
						fmt.Printf("   Destination: %s\n", webhook.Communication.Destination)
					}
				case "HTTP":
					if webhook.Communication.Destination != "" {
						fmt.Printf("   Endpoint: %s\n", style.HighlightStyle.Render(webhook.Communication.Destination))
					} else {
						fmt.Printf("   Endpoint: %s\n", style.DimStyle.Render("<direct HTTP response>"))
					}
				default:
					fmt.Printf("   Destination: %s\n", webhook.Communication.Destination)
				}

				fmt.Printf("   Hide Headers: %s\n", getBoolStyle(webhook.HideWebhookHeaders))

				// Filter section (if available)
				if webhook.Filter != "" {
					fmt.Printf("\n%s\n", style.SubtitleStyle.Render("🔍 Filter"))
					fmt.Printf("   %s\n", style.SubtitleStyle.Render(webhook.Filter))
				}

				// Management section (if available)
				if webhook.ManagedBy != "" || webhook.CreatedBy != "" {
					fmt.Printf("\n%s\n", style.SubtitleStyle.Render("👤 Management"))
					if webhook.ManagedBy != "" {
						fmt.Printf("   Managed By: %s\n", style.HighlightStyle.Render(webhook.ManagedBy))
						if webhook.TaskID != "" {
							fmt.Printf("   Task ID:    %s\n", style.DimStyle.Render(webhook.TaskID))
						}
					}
					if webhook.CreatedBy != "" {
						fmt.Printf("   Created By: %s\n", style.HighlightStyle.Render(webhook.CreatedBy))
					}
					if webhook.CreatedAt != "" && webhook.CreatedAt != "1970-01-01T00:00:00Z" && webhook.CreatedAt != "0001-01-01T00:00:00Z" {
						fmt.Printf("   Created At: %s\n", style.DimStyle.Render(webhook.CreatedAt))
					}
					if webhook.UpdatedAt != "" && webhook.UpdatedAt != "0001-01-01T00:00:00Z" {
						fmt.Printf("   Updated At: %s\n", style.DimStyle.Render(webhook.UpdatedAt))
					}
				}

				// Workflow Details section (only for workflow webhooks)
				if webhook.Workflow != "" {
					fmt.Printf("\n%s\n", style.SubtitleStyle.Render("📋 Workflow Details"))
					
					// Parse workflow to extract details
					var workflowData map[string]interface{}
					if err := json.Unmarshal([]byte(webhook.Workflow), &workflowData); err == nil {
						if name, ok := workflowData["name"].(string); ok {
							fmt.Printf("   Name:        %s\n", style.HighlightStyle.Render(name))
						}
						if desc, ok := workflowData["description"].(string); ok {
							fmt.Printf("   Description: %s\n", desc)
						}
						if steps, ok := workflowData["steps"].([]interface{}); ok {
							fmt.Printf("   Steps:       %d\n", len(steps))
						}
					}
				}

				// Prompt section
				fmt.Printf("\n%s\n", style.SubtitleStyle.Render("💬 Prompt"))
				promptLines := strings.Split(webhook.Prompt, "\n")
				for _, line := range promptLines {
					fmt.Printf("   %s\n", line)
				}

				// Template Variables section
				templateVars := extractTemplateVars(webhook.Prompt)
				if len(templateVars) > 0 {
					fmt.Printf("\n%s\n", style.SubtitleStyle.Render("🔤 Template Variables"))
					for _, v := range templateVars {
						fmt.Printf("   • %s\n", style.SubtitleStyle.Render(v))
					}
				}

				// URL and Testing section
				fmt.Printf("\n%s\n", style.SubtitleStyle.Render("🌐 Webhook URL"))
				fmt.Printf("   %s\n", style.HighlightStyle.Render(webhook.WebhookURL))

				fmt.Printf("\n%s\n", style.SubtitleStyle.Render("🧪 Test Command"))
				fmt.Printf("   %s\n",
					style.CommandStyle.Render(fmt.Sprintf("kubiya webhook test --id %s --data '{\"test\": true}'", webhook.ID)))

				return nil
			}
		},
	}

	cmd.Flags().StringVarP(&outputFormat, "output", "o", "text", "Output format (text|json|yaml)")
	return cmd
}

// Helper function to get color style based on boolean value
func getBoolStyle(value bool) string {
	if value {
		return style.SuccessStyle.Render("true")
	}
	return style.DimStyle.Render("false")
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
		hideHeaders bool
		fromFile    string
		fromStdin   bool
		interactive bool
		// New webhook target options
		target      string
		workflowDef string
		runner      string
	)

	cmd := &cobra.Command{
		Use:   "create",
		Short: "📝 Create new webhook",
		Long: `Create a new webhook in your Kubiya workspace.

Webhooks can trigger agents or workflows in response to external events.
Use --target to specify the webhook type:

• --target agent: Trigger existing agents with custom prompts
• --target workflow: Execute workflows directly using inline agents

WORKFLOW WEBHOOKS:
For workflow webhooks, the system automatically:
1. Creates/reuses an inline source with the execute_workflow tool
2. Creates an agent with the workflow definition in environment variables
3. Creates the webhook that triggers the agent

The workflow definition can be provided via:
• file://path/to/workflow.yaml - Local file path
• https://github.com/org/repo/blob/main/workflow.yaml - HTTPS URL
• JSON/YAML string directly - Inline definition

TEMPLATE VARIABLES:
Use template variables in prompts to extract data from webhook events:
• {{.event.repository.name}} - Repository name
• {{.event.pull_request.title}} - Pull request title
• {{.event.issue.number}} - Issue number

EVENT FILTERING:
Use JMESPath expressions to filter events:
• event.action == 'opened' - Only opened events
• event.pull_request.draft == false - Non-draft PRs
• contains(event.repository.name, 'backend') - Specific repos

For detailed documentation and examples, see:
• docs/webhook-guide.md - Complete guide
• docs/webhook-examples.md - Practical examples
• docs/webhook-workflow-reference.md - Technical reference`,
		Example: `  # Create an agent-based webhook
  kubiya webhook create \
    --name "GitHub PR" \
    --source "github" \
    --target "agent" \
    --agent-id "abc-123" \
    --method "Slack" \
    --destination "#devops" \
    --prompt "New PR: {{.event.pull_request.title}}"

  # Create a workflow-based webhook from file
  kubiya webhook create \
    --name "Deploy Pipeline" \
    --source "github" \
    --target "workflow" \
    --workflow "file://deploy-workflow.yaml" \
    --method "Slack" \
    --destination "#deployments"

  # Create a workflow-based webhook from URL
  kubiya webhook create \
    --name "CI Pipeline" \
    --source "github" \
    --target "workflow" \
    --workflow "https://github.com/kubiyabot/community-tools/blob/main/ci/workflow.yaml" \
    --method "Teams" \
    --destination "kubiya.ai:General"

  # Create webhook from JSON/YAML file
  kubiya webhook create --file webhook.json
  kubiya webhook create --file webhook.yaml

  # Create webhook from stdin
  cat webhook.json | kubiya webhook create --stdin`,
		RunE: func(cmd *cobra.Command, args []string) error {
			client := kubiya.NewClient(cfg)

			// Check if input is from file or stdin
			if fromFile != "" || fromStdin {
				var webhookData []byte
				var err error

				if fromFile != "" {
					webhookData, err = os.ReadFile(fromFile)
					if err != nil {
						return fmt.Errorf("failed to read file: %w", err)
					}
				} else if fromStdin {
					webhookData, err = io.ReadAll(os.Stdin)
					if err != nil {
						return fmt.Errorf("failed to read from stdin: %w", err)
					}
				}

				// Determine format
				isJSON := false
				if fromFile != "" {
					if strings.HasSuffix(strings.ToLower(fromFile), ".json") {
						isJSON = true
					}
				} else {
					// Try to determine from content
					trimmed := bytes.TrimSpace(webhookData)
					if len(trimmed) > 0 && (trimmed[0] == '{' || trimmed[0] == '[') {
						isJSON = true
					}
				}

				var webhook kubiya.Webhook
				if isJSON {
					if err := json.Unmarshal(webhookData, &webhook); err != nil {
						return fmt.Errorf("failed to parse JSON: %w", err)
					}
				} else {
					// Try YAML
					if err := yaml.Unmarshal(webhookData, &webhook); err != nil {
						return fmt.Errorf("failed to parse YAML: %w", err)
					}
				}

				// Clear ID and other server-assigned fields
				webhook.ID = ""
				webhook.CreatedAt = ""
				webhook.UpdatedAt = ""
				webhook.WebhookURL = ""

				// Create webhook
				created, err := client.CreateWebhook(cmd.Context(), webhook)
				if err != nil {
					return err
				}

				fmt.Printf("✅ Created webhook: %s (%s)\n", created.Name, created.ID)
				fmt.Printf("📎 Webhook URL: %s\n", created.WebhookURL)

				// Show template variables
				templateVars := extractTemplateVars(created.Prompt)
				if len(templateVars) > 0 {
					fmt.Printf("\n📝 Template Variables:\n")
					for _, v := range templateVars {
						fmt.Printf("- %s\n", v)
					}

					fmt.Printf("\n🧪 To test this webhook with these variables:\n")
					fmt.Printf("  kubiya webhook test --id %s --data '{", created.ID)

					for i, v := range templateVars {
						if i > 0 {
							fmt.Printf(", ")
						}
						fmt.Printf("\"%s\": \"example value\"", v)
					}

					fmt.Printf("}'\n")
				}

				return nil
			}

			// Handle interactive mode
			if interactive {
				return createWebhookInteractive(cmd.Context(), client, cfg)
			}

			// Validate target parameter
			if target == "" {
				target = "agent" // Default to agent for backward compatibility
			}

			if target != "agent" && target != "workflow" {
				return fmt.Errorf("invalid target '%s'. Must be 'agent' or 'workflow'", target)
			}

			// Handle workflow target
			if target == "workflow" {
				return createWorkflowWebhook(cmd.Context(), client, name, source, workflowDef, runner, method, destination, filter, hideHeaders)
			}

			// Handle agent target (existing behavior)
			return createAgentWebhook(cmd.Context(), client, name, source, agentID, method, destination, filter, prompt, hideHeaders)
		},
	}

	// Basic parameters
	cmd.Flags().StringVarP(&name, "name", "n", "", "Webhook name")
	cmd.Flags().StringVarP(&source, "source", "s", "", "Event source")
	cmd.Flags().StringVarP(&target, "target", "t", "agent", "Webhook target (agent|workflow)")
	cmd.Flags().StringVarP(&agentID, "agent-id", "a", "", "Agent ID (required for agent target)")
	cmd.Flags().StringVarP(&workflowDef, "workflow", "w", "", "Workflow definition (file:// or https:// URL, or JSON/YAML string, required for workflow target)")
	cmd.Flags().StringVarP(&runner, "runner", "r", "", "Runner name for workflow execution (optional, defaults to kubiya-hosted)")
	cmd.Flags().StringVarP(&method, "method", "m", "Slack", "Communication method (Slack|Teams|HTTP)")
	cmd.Flags().StringVarP(&destination, "destination", "d", "", "Communication destination (Slack: #channel, Teams: team:channel)")
	cmd.Flags().StringVarP(&filter, "filter", "f", "", "Event filter (JMESPath expression)")
	cmd.Flags().StringVarP(&prompt, "prompt", "p", "", "Agent prompt with template variables ({{.event.*}}, required for agent target)")
	cmd.Flags().BoolVar(&hideHeaders, "hide-headers", false, "Hide webhook headers in notifications")

	// File input flags
	cmd.Flags().StringVar(&fromFile, "file", "", "File containing webhook definition (JSON or YAML)")
	cmd.Flags().BoolVar(&fromStdin, "stdin", false, "Read webhook definition from stdin")
	cmd.Flags().BoolVarP(&interactive, "interactive", "i", false, "Interactive mode for parameter collection")

	// Try to mark flags required together if this Cobra version supports it
	cobra12OrHigher := true
	if cobra12OrHigher {
		cmd.MarkFlagsRequiredTogether("name", "source")
		cmd.MarkFlagsMutuallyExclusive("file", "stdin", "name")
	}

	return cmd
}

// createAgentWebhook creates a webhook that targets an agent
func createAgentWebhook(ctx context.Context, client *kubiya.Client, name, source, agentID, method, destination, filter, prompt string, hideHeaders bool) error {
	if agentID == "" {
		return fmt.Errorf("agent-id is required for agent target")
	}

	if prompt == "" {
		return fmt.Errorf("prompt is required for agent target")
	}

	// Create webhook from flags
	webhook := kubiya.Webhook{
		Name:               name,
		Source:             source,
		AgentID:            agentID,
		Filter:             filter,
		Prompt:             prompt,
		HideWebhookHeaders: hideHeaders,
		Communication:      kubiya.Communication{},
	}

	// Set method and destination based on specified method
	webhook.Communication.Method = method

	// Teams-specific processing
	if strings.EqualFold(method, "Teams") {
		if destination != "" {
			// Parse the destination in format "team:channel"
			parts := strings.Split(destination, ":")
			if len(parts) == 2 {
				// Use the parsed team and channel names directly in the destination format
				webhook.Communication.Method = "Teams"
				// Format Teams destination exactly as the API expects it
				webhook.Communication.Destination = fmt.Sprintf("#{\"team_name\": \"%s\", \"channel_name\": \"%s\"}", parts[0], parts[1])
			} else {
				// Try to use as-is (might be in JSON format already)
				webhook.Communication.Destination = destination
			}
		} else {
			return fmt.Errorf("for Teams webhooks, you must provide --destination in the format 'team:channel' (e.g., 'kubiya.ai:General')")
		}
	} else {
		webhook.Communication.Destination = destination
	}

	// HTTP doesn't require a destination
	if strings.EqualFold(method, "HTTP") && destination == "" {
		// Allow empty destination for HTTP (direct response)
		webhook.Communication.Method = "HTTP"
	}

	// Create the webhook
	created, err := client.CreateWebhook(ctx, webhook)
	if err != nil {
		return err
	}

	fmt.Printf("✅ Created agent webhook: %s (%s)\n", created.Name, created.ID)
	fmt.Printf("📎 Webhook URL: %s\n", created.WebhookURL)

	// Show template variables
	templateVars := extractTemplateVars(created.Prompt)
	if len(templateVars) > 0 {
		fmt.Printf("\n📝 Template Variables:\n")
		for _, v := range templateVars {
			fmt.Printf("- %s\n", v)
		}

		fmt.Printf("\n🧪 To test this webhook with these variables:\n")
		fmt.Printf("  kubiya webhook test --id %s --data '{", created.ID)

		for i, v := range templateVars {
			if i > 0 {
				fmt.Printf(", ")
			}
			fmt.Printf("\"%s\": \"example value\"", v)
		}

		fmt.Printf("}'\n")
	}

	return nil
}

// createWorkflowWebhook creates a webhook that targets a workflow using an inline agent
func createWorkflowWebhook(ctx context.Context, client *kubiya.Client, name, source, workflowDef, runner, method, destination, filter string, hideHeaders bool) error {
	if workflowDef == "" {
		return fmt.Errorf("workflow definition is required for workflow target")
	}

	// Load workflow definition
	workflowData, err := loadWorkflowDefinition(workflowDef)
	if err != nil {
		return fmt.Errorf("failed to load workflow definition: %w", err)
	}

	// Parse workflow definition to validate it
	var workflow map[string]interface{}
	if err := json.Unmarshal(workflowData, &workflow); err != nil {
		return fmt.Errorf("failed to parse workflow definition: %w", err)
	}

	// Set default runner if not provided
	if runner == "" {
		runner = "kubiya-hosted"
	}

	// Create a unique agent name for this webhook
	agentName := fmt.Sprintf("workflow-webhook-%s", name)

	// Extract workflow parameters for templating
	workflowParams := extractWorkflowParameters(workflow)

	// Create or get the workflow execution source
	workflowSource, err := createOrGetWorkflowExecutionSource(ctx, client, runner)
	if err != nil {
		return fmt.Errorf("failed to create workflow execution source: %w", err)
	}

	fmt.Printf("📦 Using workflow execution source: %s (%s)\n", workflowSource.Name, workflowSource.UUID)

	// Create the agent payload using direct API call
	agentPayload := map[string]interface{}{
		"name":        agentName,
		"description": fmt.Sprintf("Inline agent for workflow webhook: %s", name),
		"ai_instructions": fmt.Sprintf("You are a workflow execution agent. Use the execute_workflow tool to run workflows when triggered by webhook events. The workflow definition is provided in the WORKFLOW_JSON environment variable. Workflow: %s", workflow["name"]),
		"sources":     []string{workflowSource.UUID},
		"environment_variables": map[string]string{
			"WORKFLOW_JSON": string(workflowData),
		},
		"runners": []string{runner},
	}

	// Create the agent using direct HTTP call
	agentData, err := json.Marshal(agentPayload)
	if err != nil {
		return fmt.Errorf("failed to marshal agent payload: %w", err)
	}

	fmt.Printf("Agent creation payload: %s\n", string(agentData))

	resp, err := client.PostRaw(ctx, "/agents", agentData)
	if err != nil {
		return fmt.Errorf("failed to create inline agent: %w", err)
	}
	defer resp.Body.Close()

	// Read response
	bodyBytes, err := io.ReadAll(resp.Body)
	if err != nil {
		return fmt.Errorf("failed to read response body: %w", err)
	}

	fmt.Printf("Agent creation response status: %d\n", resp.StatusCode)
	fmt.Printf("Agent creation response: %s\n", string(bodyBytes))

	// Check for error status codes
	if resp.StatusCode != 200 {
		return fmt.Errorf("agent creation failed with status %d: %s", resp.StatusCode, string(bodyBytes))
	}

	// Parse response
	var createdAgent struct {
		ID   string `json:"id"`
		UUID string `json:"uuid"`
		Name string `json:"name"`
	}
	if err := json.Unmarshal(bodyBytes, &createdAgent); err != nil {
		return fmt.Errorf("failed to unmarshal agent response: %w\nResponse body: %s", err, string(bodyBytes))
	}

	fmt.Printf("Created agent response: %+v\n", createdAgent)

	// Use ID if UUID is empty
	agentID := createdAgent.UUID
	if agentID == "" {
		agentID = createdAgent.ID
	}

	fmt.Printf("✅ Created inline agent: %s (%s)\n", createdAgent.Name, agentID)

	// Create webhook prompt with parameter templating
	webhookPrompt := createWebhookPrompt(workflowParams)

	if agentID == "" {
		return fmt.Errorf("failed to get agent ID from created agent")
	}

	// Create the webhook using the inline agent
	webhook := kubiya.Webhook{
		Name:               name,
		Source:             source,
		AgentID:            agentID,
		Workflow:           string(workflowData),
		Runner:             runner,
		Filter:             filter,
		HideWebhookHeaders: hideHeaders,
		Communication:      kubiya.Communication{},
		Prompt:             webhookPrompt,
	}

	// Set method and destination
	webhook.Communication.Method = method
	if strings.EqualFold(method, "Teams") {
		if destination != "" {
			parts := strings.Split(destination, ":")
			if len(parts) == 2 {
				webhook.Communication.Method = "Teams"
				webhook.Communication.Destination = fmt.Sprintf("#{\"team_name\": \"%s\", \"channel_name\": \"%s\"}", parts[0], parts[1])
			} else {
				webhook.Communication.Destination = destination
			}
		} else {
			return fmt.Errorf("for Teams webhooks, you must provide --destination in the format 'team:channel' (e.g., 'kubiya.ai:General')")
		}
	} else {
		webhook.Communication.Destination = destination
	}

	// Create the webhook using the same API as agent webhooks
	created, err := client.CreateWebhook(ctx, webhook)
	if err != nil {
		return err
	}

	fmt.Printf("✅ Created workflow webhook: %s (%s)\n", created.Name, created.ID)
	fmt.Printf("📎 Webhook URL: %s\n", created.WebhookURL)
	fmt.Printf("🔧 Workflow Runner: %s\n", runner)
	fmt.Printf("🤖 Agent ID: %s\n", agentID)

	// Show workflow info
	if workflowName, ok := workflow["name"]; ok {
		fmt.Printf("📄 Workflow Name: %s\n", workflowName)
	}
	if workflowDesc, ok := workflow["description"]; ok {
		fmt.Printf("📄 Workflow Description: %s\n", workflowDesc)
	}

	fmt.Printf("\n🧪 To test this webhook:\n")
	fmt.Printf("  kubiya webhook test --id %s --data '{\"test\": true}'\n", created.ID)

	return nil
}

// Helper function to safely get string values from workflow map
func getStringFromMap(m map[string]interface{}, key, defaultValue string) string {
	if val, ok := m[key]; ok {
		if str, ok := val.(string); ok {
			return str
		}
	}
	return defaultValue
}

// Helper function to get steps as JSON string
func getStepsAsJSON(workflow map[string]interface{}) string {
	if steps, ok := workflow["steps"]; ok {
		if stepsJSON, err := json.Marshal(steps); err == nil {
			return string(stepsJSON)
		}
	}
	return "[]"
}

// extractWorkflowParameters extracts parameter definitions from workflow
func extractWorkflowParameters(workflow map[string]interface{}) []WorkflowParameter {
	var params []WorkflowParameter
	
	// Check for params in the workflow definition
	if paramsInterface, ok := workflow["params"]; ok {
		if paramsList, ok := paramsInterface.([]interface{}); ok {
			for _, paramInterface := range paramsList {
				if param, ok := paramInterface.(map[string]interface{}); ok {
					workflowParam := WorkflowParameter{
						Name:        getStringFromMap(param, "name", ""),
						Type:        getStringFromMap(param, "type", "string"),
						Description: getStringFromMap(param, "description", ""),
						Required:    getBoolFromMap(param, "required", false),
					}
					if workflowParam.Name != "" {
						params = append(params, workflowParam)
					}
				}
			}
		}
	}
	
	// If no explicit params, extract from templated values in steps
	if len(params) == 0 {
		paramNames := extractTemplatedParameters(workflow)
		for _, name := range paramNames {
			params = append(params, WorkflowParameter{
				Name:        name,
				Type:        "string",
				Description: fmt.Sprintf("Parameter extracted from workflow: %s", name),
				Required:    false,
			})
		}
	}
	
	return params
}

// extractTemplatedParameters finds templated parameters in workflow steps
func extractTemplatedParameters(workflow map[string]interface{}) []string {
	var paramNames []string
	seen := make(map[string]bool)
	
	// Convert workflow to JSON string to search for template patterns
	if workflowJSON, err := json.Marshal(workflow); err == nil {
		// Find all {{.param_name}} patterns
		re := regexp.MustCompile(`\{\{\s*\.(\w+)\s*\}\}`)
		matches := re.FindAllStringSubmatch(string(workflowJSON), -1)
		
		for _, match := range matches {
			if len(match) > 1 {
				paramName := match[1]
				// Skip common webhook fields like 'event'
				if paramName != "event" && !seen[paramName] {
					paramNames = append(paramNames, paramName)
					seen[paramName] = true
				}
			}
		}
	}
	
	return paramNames
}

// createWorkflowSystemPrompt creates a system prompt for the workflow agent
func createWorkflowSystemPrompt(workflow map[string]interface{}, params []WorkflowParameter) string {
	workflowJSON, _ := json.MarshalIndent(workflow, "", "  ")
	
	var prompt strings.Builder
	prompt.WriteString("You are a workflow execution agent. Your task is to execute the following workflow with the provided event data.\n\n")
	
	prompt.WriteString("WORKFLOW DEFINITION:\n")
	prompt.WriteString("```json\n")
	prompt.WriteString(string(workflowJSON))
	prompt.WriteString("\n```\n\n")
	
	if len(params) > 0 {
		prompt.WriteString("EXPECTED PARAMETERS:\n")
		for _, param := range params {
			prompt.WriteString(fmt.Sprintf("- %s (%s): %s", param.Name, param.Type, param.Description))
			if param.Required {
				prompt.WriteString(" [REQUIRED]")
			}
			prompt.WriteString("\n")
		}
		prompt.WriteString("\n")
	}
	
	prompt.WriteString("INSTRUCTIONS:\n")
	prompt.WriteString("1. When you receive webhook event data, extract the relevant parameters\n")
	prompt.WriteString("2. Use the execute_workflow tool to run the workflow with the extracted parameters\n")
	prompt.WriteString("3. The workflow definition is embedded in this prompt for your reference\n")
	prompt.WriteString("4. Template variables in the workflow ({{.param_name}}) will be replaced with actual values\n")
	prompt.WriteString("5. Provide clear feedback about the workflow execution status\n")
	
	return prompt.String()
}

// Helper function to get bool values from map
func getBoolFromMap(m map[string]interface{}, key string, defaultValue bool) bool {
	if val, ok := m[key]; ok {
		if boolVal, ok := val.(bool); ok {
			return boolVal
		}
	}
	return defaultValue
}

// WorkflowParameter represents a parameter in a workflow
type WorkflowParameter struct {
	Name        string `json:"name"`
	Type        string `json:"type"`
	Description string `json:"description"`
	Required    bool   `json:"required"`
}

// createOrGetWorkflowExecutionSource creates or gets an existing workflow execution source
func createOrGetWorkflowExecutionSource(ctx context.Context, client *kubiya.Client, runner string) (*kubiya.Source, error) {
	sourceName := "workflow-execution-tools"
	
	// Check if source already exists
	sources, err := client.ListSources(ctx)
	if err != nil {
		return nil, fmt.Errorf("failed to list sources: %w", err)
	}

	// Look for existing workflow execution source
	for _, source := range sources {
		if source.Name == sourceName && source.Type == "inline" {
			// Check if it has the execute_workflow tool
			sourceWithTools, err := client.GetSourceMetadata(ctx, source.UUID)
			if err != nil {
				continue // Skip if we can't get metadata
			}
			
			// Check if it has the execute_workflow tool
			for _, tool := range sourceWithTools.Tools {
				if tool.Name == "execute_workflow" {
					return sourceWithTools, nil
				}
			}
		}
	}

	// Create the inline source using direct API call
	fmt.Printf("Creating inline source with name: %s\n", sourceName)
	
	sourcePayload := map[string]interface{}{
		"name": sourceName,
		"url":  "",
		"dynamic_config": map[string]interface{}{},
		"inline_tools": []map[string]interface{}{
			{
				"name":    "execute_workflow",
				"image":   "python:latest",
				"content": fmt.Sprintf(`#!/bin/bash
# Workflow execution tool
# This tool executes a workflow using the Kubiya workflow API

WORKFLOW_JSON="$1"
EVENT_PARAMS="$2"

echo "🔄 Executing workflow with parameters..."
echo "Runner: %s"

# Execute the workflow via curl
curl -X POST "https://api.kubiya.ai/api/v1/workflow?runner=%s&operation=execute_workflow" \
    -H "Authorization: UserKey $KUBIYA_API_KEY" \
    -H "Content-Type: application/json" \
    -H "Accept: text/event-stream" \
    -d "{
        \"workflow_spec\": $WORKFLOW_JSON,
        \"params\": $EVENT_PARAMS
    }"
`, runner, runner),
			},
		},
	}

	// Create the source using direct HTTP call
	sourceData, err := json.Marshal(sourcePayload)
	if err != nil {
		return nil, fmt.Errorf("failed to marshal source payload: %w", err)
	}

	fmt.Printf("Source creation payload: %s\n", string(sourceData))

	resp, err := client.PostRaw(ctx, "/sources", sourceData)
	if err != nil {
		return nil, fmt.Errorf("failed to create workflow execution source: %w", err)
	}
	defer resp.Body.Close()

	// Read response
	bodyBytes, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("failed to read response body: %w", err)
	}

	fmt.Printf("Source creation response status: %d\n", resp.StatusCode)
	fmt.Printf("Source creation response: %s\n", string(bodyBytes))

	// Check for error status codes
	if resp.StatusCode != 200 {
		return nil, fmt.Errorf("source creation failed with status %d: %s", resp.StatusCode, string(bodyBytes))
	}

	// Parse response
	var createdSource kubiya.Source
	if err := json.Unmarshal(bodyBytes, &createdSource); err != nil {
		return nil, fmt.Errorf("failed to unmarshal source response: %w\nResponse body: %s", err, string(bodyBytes))
	}

	fmt.Printf("Created source response: %+v\n", &createdSource)
	return &createdSource, nil
}

// createWebhookPrompt creates a webhook prompt that extracts parameters from event data
func createWebhookPrompt(params []WorkflowParameter) string {
	var prompt strings.Builder
	
	prompt.WriteString("Execute the workflow with the following event data and extracted parameters:\n\n")
	prompt.WriteString("Event Data: {{.event}}\n\n")
	
	if len(params) > 0 {
		prompt.WriteString("Extracted Parameters:\n")
		for _, param := range params {
			prompt.WriteString(fmt.Sprintf("- %s: {{.event.%s}}\n", param.Name, param.Name))
		}
		prompt.WriteString("\n")
	}
	
	prompt.WriteString("Please execute the workflow with these parameters using the execute_workflow tool.")
	
	return prompt.String()
}

// loadWorkflowDefinition loads a workflow definition from various sources
func loadWorkflowDefinition(def string) ([]byte, error) {
	var data []byte
	var err error
	
	if strings.HasPrefix(def, "file://") {
		// Load from file
		filePath := strings.TrimPrefix(def, "file://")
		data, err = os.ReadFile(filePath)
		if err != nil {
			return nil, err
		}
	} else if strings.HasPrefix(def, "https://") || strings.HasPrefix(def, "http://") {
		// Load from URL
		resp, err := http.Get(def)
		if err != nil {
			return nil, err
		}
		defer resp.Body.Close()
		data, err = io.ReadAll(resp.Body)
		if err != nil {
			return nil, err
		}
	} else {
		// Assume it's a JSON/YAML string
		data = []byte(def)
	}
	
	// Convert YAML to JSON if needed
	data = bytes.TrimSpace(data)
	if len(data) > 0 && data[0] != '{' && data[0] != '[' {
		// Likely YAML, convert to JSON
		var yamlData interface{}
		if err := yaml.Unmarshal(data, &yamlData); err != nil {
			return nil, fmt.Errorf("failed to parse YAML: %w", err)
		}
		
		jsonData, err := json.Marshal(yamlData)
		if err != nil {
			return nil, fmt.Errorf("failed to convert YAML to JSON: %w", err)
		}
		return jsonData, nil
	}
	
	return data, nil
}

// createWebhookInteractive creates a webhook in interactive mode
func createWebhookInteractive(ctx context.Context, client *kubiya.Client, cfg *config.Config) error {
	fmt.Println("🔗 Interactive Webhook Creation")
	fmt.Println("==============================")
	
	// Get webhook name
	var name string
	fmt.Print("Enter webhook name: ")
	fmt.Scanln(&name)
	
	// Get source
	var source string
	fmt.Print("Enter event source (e.g., github, slack, custom): ")
	fmt.Scanln(&source)
	
	// Get target type
	var target string
	fmt.Print("Select target type (agent/workflow) [agent]: ")
	fmt.Scanln(&target)
	if target == "" {
		target = "agent"
	}
	
	// Get communication method
	var method string
	fmt.Print("Select communication method (Slack/Teams/HTTP) [Slack]: ")
	fmt.Scanln(&method)
	if method == "" {
		method = "Slack"
	}
	
	// Get destination
	var destination string
	if !strings.EqualFold(method, "HTTP") {
		if strings.EqualFold(method, "Teams") {
			fmt.Print("Enter Teams destination (team:channel): ")
		} else {
			fmt.Print("Enter Slack destination (#channel): ")
		}
		fmt.Scanln(&destination)
	}
	
	// Get filter
	var filter string
	fmt.Print("Enter event filter (JMESPath, optional): ")
	fmt.Scanln(&filter)
	
	if target == "workflow" {
		return createWorkflowWebhookInteractive(ctx, client, name, source, method, destination, filter)
	} else {
		return createAgentWebhookInteractive(ctx, client, name, source, method, destination, filter)
	}
}

// createWorkflowWebhookInteractive creates a workflow webhook interactively
func createWorkflowWebhookInteractive(ctx context.Context, client *kubiya.Client, name, source, method, destination, filter string) error {
	fmt.Println("\n📋 Workflow Configuration")
	fmt.Println("========================")
	
	// Get workflow definition
	var workflowDef string
	fmt.Print("Enter workflow definition (file:// path, https:// URL, or JSON): ")
	fmt.Scanln(&workflowDef)
	
	// Get runner
	var runner string
	fmt.Print("Enter runner name [kubiya-hosted]: ")
	fmt.Scanln(&runner)
	if runner == "" {
		runner = "kubiya-hosted"
	}
	
	// Load and validate workflow
	workflowData, err := loadWorkflowDefinition(workflowDef)
	if err != nil {
		return fmt.Errorf("failed to load workflow definition: %w", err)
	}
	
	var workflow map[string]interface{}
	if err := json.Unmarshal(workflowData, &workflow); err != nil {
		return fmt.Errorf("failed to parse workflow definition: %w", err)
	}
	
	// Extract and show parameters
	workflowParams := extractWorkflowParameters(workflow)
	
	if len(workflowParams) > 0 {
		fmt.Println("\n📝 Detected Workflow Parameters:")
		for _, param := range workflowParams {
			fmt.Printf("  - %s (%s): %s", param.Name, param.Type, param.Description)
			if param.Required {
				fmt.Print(" [REQUIRED]")
			}
			fmt.Println()
		}
		
		fmt.Println("\nThese parameters will be automatically extracted from webhook event data.")
		fmt.Println("Example usage patterns:")
		for _, param := range workflowParams {
			fmt.Printf("  - {{.event.%s}} -> %s\n", param.Name, param.Description)
		}
	}
	
	// Show preview
	fmt.Println("\n🔍 Webhook Preview:")
	fmt.Printf("  Name: %s\n", name)
	fmt.Printf("  Source: %s\n", source)
	fmt.Printf("  Target: workflow\n")
	fmt.Printf("  Method: %s\n", method)
	fmt.Printf("  Destination: %s\n", destination)
	fmt.Printf("  Filter: %s\n", filter)
	fmt.Printf("  Runner: %s\n", runner)
	fmt.Printf("  Workflow: %s\n", getStringFromMap(workflow, "name", "Unnamed Workflow"))
	
	// Confirm creation
	var confirm string
	fmt.Print("\nCreate this webhook? (y/N): ")
	fmt.Scanln(&confirm)
	if strings.ToLower(confirm) != "y" {
		fmt.Println("❌ Webhook creation cancelled")
		return nil
	}
	
	// Create the webhook
	return createWorkflowWebhook(ctx, client, name, source, workflowDef, runner, method, destination, filter, false)
}

// createAgentWebhookInteractive creates an agent webhook interactively
func createAgentWebhookInteractive(ctx context.Context, client *kubiya.Client, name, source, method, destination, filter string) error {
	fmt.Println("\n🤖 Agent Configuration")
	fmt.Println("======================")
	
	// List available agents
	agents, err := client.GetAgents(ctx)
	if err != nil {
		return fmt.Errorf("failed to list agents: %w", err)
	}
	
	if len(agents) == 0 {
		return fmt.Errorf("no agents found. Please create an agent first.")
	}
	
	fmt.Println("Available agents:")
	for i, agent := range agents {
		fmt.Printf("  %d. %s (%s) - %s\n", i+1, agent.Name, agent.UUID, agent.Description)
	}
	
	// Get agent selection
	var agentIndex int
	fmt.Printf("Select agent (1-%d): ", len(agents))
	fmt.Scanln(&agentIndex)
	
	if agentIndex < 1 || agentIndex > len(agents) {
		return fmt.Errorf("invalid agent selection")
	}
	
	selectedAgent := agents[agentIndex-1]
	
	// Get prompt
	var prompt string
	fmt.Print("Enter agent prompt (use {{.event.*}} for template variables): ")
	fmt.Scanln(&prompt)
	
	// Show preview
	fmt.Println("\n🔍 Webhook Preview:")
	fmt.Printf("  Name: %s\n", name)
	fmt.Printf("  Source: %s\n", source)
	fmt.Printf("  Target: agent\n")
	fmt.Printf("  Agent: %s (%s)\n", selectedAgent.Name, selectedAgent.UUID)
	fmt.Printf("  Method: %s\n", method)
	fmt.Printf("  Destination: %s\n", destination)
	fmt.Printf("  Filter: %s\n", filter)
	fmt.Printf("  Prompt: %s\n", prompt)
	
	// Confirm creation
	var confirm string
	fmt.Print("\nCreate this webhook? (y/N): ")
	fmt.Scanln(&confirm)
	if strings.ToLower(confirm) != "y" {
		fmt.Println("❌ Webhook creation cancelled")
		return nil
	}
	
	// Create the webhook
	return createAgentWebhook(ctx, client, name, source, selectedAgent.UUID, method, destination, filter, prompt, false)
}

func newUpdateWebhookCommand(cfg *config.Config) *cobra.Command {
	var (
		name           string
		source         string
		agentID        string
		method         string
		destination    string
		filter         string
		prompt         string
		hideHeaders    bool
		hideHeadersSet bool
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

			// Check if headers-visibility was set
			hideHeadersSet = cmd.Flags().Changed("headers-visibility")

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
			if hideHeadersSet {
				existing.HideWebhookHeaders = hideHeaders
			}

			// Show changes
			fmt.Println("\n📝 Review changes:")
			fmt.Printf("Name: %s\n", existing.Name)
			fmt.Printf("Source: %s\n", existing.Source)
			fmt.Printf("Agent ID: %s\n", existing.AgentID)
			fmt.Printf("Communication Method: %s\n", existing.Communication.Method)
			fmt.Printf("Communication Destination: %s\n", existing.Communication.Destination)
			fmt.Printf("Hide Headers: %t\n", existing.HideWebhookHeaders)
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
	cmd.Flags().StringVarP(&method, "method", "m", "", "New communication method (Slack|Teams|HTTP)")
	cmd.Flags().StringVarP(&destination, "destination", "d", "", "New communication destination")
	cmd.Flags().StringVarP(&filter, "filter", "f", "", "New event filter (JMESPath expression)")
	cmd.Flags().StringVarP(&prompt, "prompt", "p", "", "New agent prompt with template variables")

	// Add headers-visibility flag
	cmd.Flags().BoolVar(&hideHeaders, "headers-visibility", false, "Control webhook headers visibility (hide|show)")
	cmd.Flags().Lookup("headers-visibility").NoOptDefVal = "true"

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
				return err
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
					return fmt.Errorf("operation cancelled")
				}
			}

			if err := client.DeleteWebhook(cmd.Context(), args[0]); err != nil {
				return err
			}

			fmt.Printf("✅ Successfully deleted webhook: %s (%s)\n", webhook.Name, args[0])
			return nil
		},
	}

	cmd.Flags().BoolVarP(&force, "force", "f", false, "Skip confirmation prompt")
	return cmd
}

// newTestWebhookCommand creates a new command to test a webhook
func newTestWebhookCommand(cfg *config.Config) *cobra.Command {
	var (
		webhookID       string
		webhookURL      string
		dataFile        string
		dataJSON        string
		waitForResponse bool
		verbose         bool
		autoGenerate    bool
		monitorEvents   bool
	)

	cmd := &cobra.Command{
		Use:   "test",
		Short: "🧪 Test a webhook",
		Example: `  # Test with webhook ID
  kubiya webhook test --id abc-123 --data '{"key": "value"}'
  
  # Test with webhook URL directly
  kubiya webhook test --url https://webhook-url --data-file test-payload.json
  
  # Test with auto-generated data based on template variables
  kubiya webhook test --id abc-123 --auto-generate
  
  # Wait for the webhook to complete and show response
  kubiya webhook test --id abc-123 --data '{"key": "value"}' --wait
  
  # Monitor webhook events in real-time
  kubiya webhook test --id abc-123 --data '{"key": "value"}' --wait --monitor`,
		RunE: func(cmd *cobra.Command, args []string) error {
			client := kubiya.NewClient(cfg)

			// Get webhook URL if ID is provided
			var webhook *kubiya.Webhook
			var err error

			if webhookID != "" && webhookURL == "" {
				webhook, err = client.GetWebhook(cmd.Context(), webhookID)
				if err != nil {
					return fmt.Errorf("failed to get webhook: %w", err)
				}
				webhookURL = webhook.WebhookURL
				fmt.Printf("📎 Using webhook URL: %s\n", webhookURL)
			}

			if webhookURL == "" {
				return fmt.Errorf("either --id or --url must be provided")
			}

			// Parse the test data
			var testData interface{}

			if dataJSON != "" {
				// Data provided directly in command
				if err := json.Unmarshal([]byte(dataJSON), &testData); err != nil {
					return fmt.Errorf("invalid JSON data: %w", err)
				}

				// Check if we need to convert dot notation to nested objects
				if mapData, ok := testData.(map[string]interface{}); ok {
					testData = convertDotNotationToNested(mapData)

					if verbose {
						fmt.Println("🔄 Converting flat keys to nested structure:")
						prettyJSON, _ := json.MarshalIndent(testData, "", "  ")
						fmt.Printf("%s\n\n", string(prettyJSON))
					}
				}
			} else if dataFile != "" {
				// Data provided in file
				data, err := os.ReadFile(dataFile)
				if err != nil {
					return fmt.Errorf("failed to read data file: %w", err)
				}

				if err := json.Unmarshal(data, &testData); err != nil {
					return fmt.Errorf("invalid JSON in data file: %w", err)
				}

				// Check if we need to convert dot notation to nested objects
				if mapData, ok := testData.(map[string]interface{}); ok {
					testData = convertDotNotationToNested(mapData)

					if verbose {
						fmt.Println("🔄 Converting flat keys to nested structure:")
						prettyJSON, _ := json.MarshalIndent(testData, "", "  ")
						fmt.Printf("%s\n\n", string(prettyJSON))
					}
				}
			} else if autoGenerate && webhook != nil {
				// Use auto-generated test data based on template variables
				templateVars := extractTemplateVars(webhook.Prompt)
				testData = make(map[string]interface{})

				if len(templateVars) > 0 {
					// Add example values for each template variable
					data := testData.(map[string]interface{})
					for _, v := range templateVars {
						// Handle nested variables like event.issue.key
						parts := strings.Split(v, ".")

						// Start with the root object
						current := data

						// Create nested objects for each part except the last
						for i, part := range parts {
							if i == len(parts)-1 {
								// For the last part, set a sample value
								current[part] = fmt.Sprintf("sample-%s", part)
							} else {
								// For intermediate parts, create a nested object if needed
								if _, exists := current[part]; !exists {
									current[part] = make(map[string]interface{})
								}
								current = current[part].(map[string]interface{})
							}
						}
					}
				}

				// Add default test metadata
				data := testData.(map[string]interface{})
				data["_test"] = map[string]interface{}{
					"timestamp": time.Now().Format(time.RFC3339),
					"message":   "Test webhook from Kubiya CLI",
				}
			} else {
				// Simple default test data
				testData = map[string]interface{}{
					"test":      true,
					"timestamp": time.Now().Format(time.RFC3339),
					"message":   "Test webhook from Kubiya CLI",
				}
			}

			// Print the payload we're sending
			fmt.Println("📤 Sending test data to webhook...")
			prettyJSON, _ := json.MarshalIndent(testData, "", "  ")
			fmt.Printf("Payload:\n%s\n\n", string(prettyJSON))

			// If we have monitor flag explicitly set to true, enable it regardless of wait flag
			// If monitor flag is not set but wait is set, enable monitoring by default
			if monitorEvents || (waitForResponse && webhookID != "" && webhook != nil && !cmd.Flags().Changed("monitor")) {
				monitorEvents = true
			}

			// Send the test with response handling
			if waitForResponse {
				// Create a cancellable context for the entire operation
				ctx, cancel := context.WithCancel(cmd.Context())
				defer cancel()

				// Set up signal handling for graceful cancellation
				sigChan := make(chan os.Signal, 1)
				signal.Notify(sigChan, os.Interrupt)

				// Channel to signal when we should exit
				exitChan := make(chan struct{})

				go func() {
					select {
					case <-sigChan:
						fmt.Println("\n⚠️ Interrupt received, stopping webhook test...")
						cancel()
					case <-exitChan:
						// Normal exit, do nothing
						return
					}
				}()

				fmt.Println("⏳ Waiting for webhook to process...")

				// Start a webhook test monitor if we have the webhook ID and name
				var webhookTest *kubiya.WebhookTest
				var monitorDone chan struct{}

				// Only start monitoring if --monitor flag is set and we have webhook details
				if monitorEvents && webhookID != "" && webhook != nil {
					// Create a new webhook test session
					webhookTest = kubiya.NewWebhookTest(client, webhook.Name)

					// Channel to signal when monitoring is done
					monitorDone = make(chan struct{})

					// Start monitoring in a goroutine
					go func() {
						defer close(monitorDone)
						if err := webhookTest.StartTest(ctx); err != nil && ctx.Err() == nil {
							fmt.Printf("Error monitoring webhook: %v\n", err)
						}
					}()
				}

				// Send the webhook request and wait for HTTP response
				resp, err := client.TestWebhookWithResponse(ctx, webhookURL, testData)
				if err != nil {
					if ctx.Err() != context.Canceled {
						return fmt.Errorf("webhook test failed: %w", err)
					}
					return fmt.Errorf("webhook test was canceled")
				}

				// Print response status
				fmt.Printf("✅ Webhook test successful (Status: %d)\n", resp.StatusCode)

				// If we have a response body, print it
				if resp.Body != nil && resp.ContentLength > 0 {
					fmt.Println("\n📬 Response:")

					// Try to parse and pretty print the response if it's JSON
					var respData interface{}
					responseBody, _ := io.ReadAll(resp.Body)

					if err := json.Unmarshal(responseBody, &respData); err == nil {
						// It's valid JSON, pretty print it
						prettyResp, _ := json.MarshalIndent(respData, "", "  ")
						fmt.Println(string(prettyResp))
					} else {
						// Not JSON, print as is
						fmt.Println(string(responseBody))
					}
				}

				// If webhook test monitor is active, wait for completion or interrupt
				if monitorEvents && webhookID != "" && webhookTest != nil && monitorDone != nil {
					fmt.Println("\n📡 Monitoring webhook execution events in real-time. Press Ctrl+C to stop...")
					fmt.Println("   Note: The webhook has been triggered and is being processed by the server.")
					fmt.Println("   We are now monitoring for any events or actions it performs.")

					if verbose {
						fmt.Println("\n🔍 Debug Information:")
						fmt.Printf("   - Webhook ID: %s\n", webhookID)
						fmt.Printf("   - Webhook Name: %s\n", webhook.Name)
						fmt.Printf("   - Webhook Source: %s\n", webhook.Source)
						fmt.Printf("   - Webhook Method: %s\n", webhook.Communication.Method)

						// Print the payload we sent in dot notation format for easy matching with template vars
						fmt.Println("   - Payload in dot notation (for template matching):")
						printMapInDotNotation(testData, "     ", "")
					}

					// Just wait for user to press Ctrl+C
					<-ctx.Done()
					fmt.Println("\nWebhook monitoring stopped.")
				} else {
					// If not monitoring, signal to exit
					close(exitChan)
				}
			} else {
				// Just send without waiting for response
				if err := client.TestWebhook(cmd.Context(), webhookURL, testData); err != nil {
					return fmt.Errorf("webhook test failed: %w", err)
				}

				fmt.Println("✅ Webhook test successful")
			}

			return nil
		},
	}

	cmd.Flags().StringVar(&webhookID, "id", "", "Webhook ID")
	cmd.Flags().StringVar(&webhookURL, "url", "", "Webhook URL")
	cmd.Flags().StringVar(&dataJSON, "data", "", "JSON data to send")
	cmd.Flags().StringVar(&dataFile, "data-file", "", "File containing JSON data to send")
	cmd.Flags().BoolVar(&waitForResponse, "wait", false, "Wait for HTTP response (automatically enables monitoring with --id)")
	cmd.Flags().BoolVar(&verbose, "verbose", false, "Show detailed information")
	cmd.Flags().BoolVar(&autoGenerate, "auto-generate", false, "Auto-generate test data based on template variables")
	cmd.Flags().BoolVar(&monitorEvents, "monitor", false, "Monitor webhook events in real-time (requires --id)")

	// Make sure only one of id or url is provided
	cmd.MarkFlagsMutuallyExclusive("id", "url")

	return cmd
}

// newImportWebhookCommand creates a new command to import a webhook from JSON/YAML
func newImportWebhookCommand(cfg *config.Config) *cobra.Command {
	var (
		filePath string
		format   string
		example  bool
	)

	cmd := &cobra.Command{
		Use:   "import",
		Short: "📥 Import webhook from file",
		Example: `  kubiya webhook import --file webhook.json
  kubiya webhook import --file webhook.yaml --format yaml
  kubiya webhook import --example > webhook_template.json`,
		RunE: func(cmd *cobra.Command, args []string) error {
			if example {
				template := generateWebhookTemplate("json")
				fmt.Println(template)
				return nil
			}

			if filePath == "" {
				return fmt.Errorf("--file is required unless --example is specified")
			}

			data, err := os.ReadFile(filePath)
			if err != nil {
				return fmt.Errorf("failed to read file: %w", err)
			}

			var webhookData []byte

			// Auto-detect format if not specified
			if format == "" {
				if strings.HasSuffix(strings.ToLower(filePath), ".yaml") ||
					strings.HasSuffix(strings.ToLower(filePath), ".yml") {
					format = "yaml"
				} else {
					format = "json"
				}
			}

			// Convert YAML to JSON if needed
			if format == "yaml" {
				// First unmarshal YAML
				var yamlObj interface{}
				if err := yaml.Unmarshal(data, &yamlObj); err != nil {
					return fmt.Errorf("invalid YAML: %w", err)
				}

				// Then marshal back to JSON
				webhookData, err = json.Marshal(yamlObj)
				if err != nil {
					return fmt.Errorf("failed to convert YAML to JSON: %w", err)
				}
			} else {
				webhookData = data
			}

			client := kubiya.NewClient(cfg)
			webhook, err := client.ImportWebhookFromJSON(cmd.Context(), webhookData)
			if err != nil {
				return fmt.Errorf("failed to import webhook: %w", err)
			}

			fmt.Printf("✅ Imported webhook: %s (%s)\n", webhook.Name, webhook.ID)
			fmt.Printf("📎 Webhook URL: %s\n", webhook.WebhookURL)
			return nil
		},
	}

	cmd.Flags().StringVarP(&filePath, "file", "f", "", "Path to JSON/YAML file")
	cmd.Flags().StringVar(&format, "format", "", "File format (json|yaml)")
	cmd.Flags().BoolVar(&example, "example", false, "Generate an example webhook template")

	return cmd
}

// newExportWebhookCommand creates a new command to export a webhook to JSON/YAML
func newExportWebhookCommand(cfg *config.Config) *cobra.Command {
	var (
		webhookID string
		outFile   string
		format    string
		example   bool
	)

	cmd := &cobra.Command{
		Use:   "export [id]",
		Short: "📤 Export webhook to file",
		Example: `  kubiya webhook export abc-123 --output webhook.json
  kubiya webhook export abc-123 --output webhook.yaml --format yaml
  kubiya webhook export --example --format yaml > webhook_template.yaml`,
		RunE: func(cmd *cobra.Command, args []string) error {
			if example {
				template := generateWebhookTemplate(format)
				fmt.Println(template)
				return nil
			}

			if len(args) == 0 {
				return fmt.Errorf("webhook ID is required unless --example is specified")
			}

			webhookID = args[0]

			client := kubiya.NewClient(cfg)
			webhook, err := client.GetWebhook(cmd.Context(), webhookID)
			if err != nil {
				return fmt.Errorf("failed to get webhook: %w", err)
			}

			var data []byte

			// Auto-detect format if not specified
			if format == "" {
				if strings.HasSuffix(strings.ToLower(outFile), ".yaml") ||
					strings.HasSuffix(strings.ToLower(outFile), ".yml") {
					format = "yaml"
				} else {
					format = "json"
				}
			}

			if format == "yaml" {
				data, err = yaml.Marshal(webhook)
				if err != nil {
					return fmt.Errorf("failed to marshal webhook to YAML: %w", err)
				}
			} else {
				data, err = json.MarshalIndent(webhook, "", "  ")
				if err != nil {
					return fmt.Errorf("failed to marshal webhook to JSON: %w", err)
				}
			}

			if outFile != "" {
				if err := os.WriteFile(outFile, data, 0644); err != nil {
					return fmt.Errorf("failed to write file: %w", err)
				}
				fmt.Printf("✅ Exported webhook to %s\n", outFile)
			} else {
				fmt.Println(string(data))
			}

			return nil
		},
	}

	cmd.Flags().StringVarP(&outFile, "output", "o", "", "Output file path (defaults to stdout)")
	cmd.Flags().StringVar(&format, "format", "", "Output format (json|yaml)")
	cmd.Flags().BoolVar(&example, "example", false, "Generate an example webhook template")

	return cmd
}

// generateWebhookTemplate creates an example webhook template in the specified format
func generateWebhookTemplate(format string) string {
	// Create example webhook
	webhook := kubiya.Webhook{
		Name:               "example-webhook",
		Source:             "github",
		AgentID:            "AGENT_ID_HERE",
		HideWebhookHeaders: false,
		Communication: kubiya.Communication{
			Method:      "Slack",
			Destination: "#channel-name",
		},
		Filter: "pull_request[?state == 'open']",
		Prompt: "# GitHub Pull Request\n\nPlease analyze the following PR details:\n\n- Title: {{.event.pull_request.title}}\n- Author: {{.event.pull_request.user.login}}\n- Description: {{.event.pull_request.body}}",
	}

	// Add example comments
	var result string
	if format == "yaml" {
		// Convert to YAML
		data, _ := yaml.Marshal(webhook)

		// Add YAML comments
		yamlComments := "# Example webhook template in YAML format\n" +
			"# To use this template:\n" +
			"# 1. Replace AGENT_ID_HERE with your agent ID\n" +
			"# 2. Customize the prompt and other fields as needed\n" +
			"# 3. Import with: kubiya webhook import --file webhook.yaml\n\n"

		result = yamlComments + string(data)
	} else {
		// Convert to JSON
		data, _ := json.MarshalIndent(webhook, "", "  ")

		// Add JSON comments as a leading comment block
		jsonComments := "// Example webhook template in JSON format\n" +
			"// To use this template:\n" +
			"// 1. Replace AGENT_ID_HERE with your agent ID\n" +
			"// 2. Customize the prompt and other fields as needed\n" +
			"// 3. Import with: kubiya webhook import --file webhook.json\n\n"

		result = jsonComments + string(data)
	}

	return result
}

// newWizardWebhookCommand creates a new interactive webhook creation wizard
func newWizardWebhookCommand(cfg *config.Config) *cobra.Command {
	cmd := &cobra.Command{
		Use:     "wizard",
		Short:   "🧙 Create webhook using interactive wizard",
		Example: "  kubiya webhook wizard",
		RunE: func(cmd *cobra.Command, args []string) error {
			client := kubiya.NewClient(cfg)

			webhook := kubiya.Webhook{
				Communication: kubiya.Communication{
					Method: "Slack", // Default method
				},
			}

			// Function to read input with prompt
			readInput := func(prompt string, defaultValue string) string {
				if defaultValue != "" {
					fmt.Printf("%s [%s]: ", prompt, defaultValue)
				} else {
					fmt.Printf("%s: ", prompt)
				}
				var input string
				fmt.Scanln(&input)
				if input == "" {
					return defaultValue
				}
				return input
			}

			// Get webhook name
			webhook.Name = readInput("Webhook name", "")
			if webhook.Name == "" {
				return fmt.Errorf("webhook name is required")
			}

			// Get event source
			webhook.Source = readInput("Event source (e.g., github, jira)", "")
			if webhook.Source == "" {
				return fmt.Errorf("event source is required")
			}

			// Get agent ID
			fmt.Println("\n📋 Available agents:")
			agents, err := client.GetAgents(cmd.Context())
			if err != nil {
				fmt.Println("⚠️ Could not fetch agents. You'll need to enter the agent ID manually.")
			} else {
				w := tabwriter.NewWriter(os.Stdout, 0, 0, 2, ' ', 0)
				fmt.Fprintln(w, "ID\tNAME\tDESCRIPTION")
				for _, t := range agents {
					// Truncate description if it's too long
					description := t.Description
					if len(description) > 40 {
						description = description[:37] + "..."
					}
					fmt.Fprintf(w, "%s\t%s\t%s\n", t.UUID, t.Name, description)
				}
				w.Flush()
				fmt.Println()
			}

			webhook.AgentID = readInput("Agent ID", "")
			if webhook.AgentID == "" {
				return fmt.Errorf("agent ID is required")
			}

			// Get communication method
			fmt.Println("\n📡 Available communication methods:")
			fmt.Println("1. Slack (default)")
			fmt.Println("2. Teams (Microsoft Teams)")
			fmt.Println("3. HTTP (Direct HTTP response)")

			methodChoice := readInput("Choose communication method [1-3]", "1")
			switch methodChoice {
			case "1":
				webhook.Communication.Method = "Slack"
				webhook.Communication.Destination = readInput("Slack channel or user (e.g., #channel, @user)", "")
				if webhook.Communication.Destination == "" {
					return fmt.Errorf("Slack channel or user is required")
				}
			case "2":
				webhook.Communication.Method = "Teams"
				webhook.Communication.Destination = readInput("Teams destination (e.g., kubiya.ai:General)", "")
				if webhook.Communication.Destination == "" {
					return fmt.Errorf("Teams destination is required")
				}
			case "3":
				webhook.Communication.Method = "HTTP"
				webhook.Communication.Destination = readInput("HTTP destination (optional)", "")
				// HTTP doesn't require a destination - will use direct response if empty
			default:
				return fmt.Errorf("invalid choice: %s", methodChoice)
			}

			// Get event filter
			fmt.Println("\n🔍 Event Filter (JMESPath expression, optional)")
			fmt.Println("Example: pull_request.requested_reviewers[?login == 'username']")
			fmt.Println("Leave empty to receive all events without filtering")
			webhook.Filter = readInput("Filter", "")

			// Get agent prompt
			fmt.Println("\n💬 Agent Prompt")
			fmt.Println("You can use template variables like {{.event.field}} to reference event data.")
			fmt.Println("Examples:")
			fmt.Println("- New PR: {{.event.pull_request.title}}")
			fmt.Println("- Issue: {{.event.issue.key}} - {{.event.issue.fields.summary}}")
			fmt.Println("\nEnter your prompt (type 'done' on a new line to finish):")
			scanner := bufio.NewScanner(os.Stdin)
			var promptBuilder strings.Builder
			for scanner.Scan() {
				line := scanner.Text()
				if line == "done" {
					break
				}
				promptBuilder.WriteString(line)
				promptBuilder.WriteString("\n")
			}
			webhook.Prompt = strings.TrimSpace(promptBuilder.String())
			if webhook.Prompt == "" {
				return fmt.Errorf("agent prompt is required")
			}

			// Get header visibility
			hideHeadersChoice := readInput("Hide webhook headers in notifications? (y/N)", "n")
			webhook.HideWebhookHeaders = strings.ToLower(hideHeadersChoice) == "y"

			// Review webhook details
			fmt.Println("\n📝 Review webhook details:")
			fmt.Printf("Name: %s\n", webhook.Name)
			fmt.Printf("Source: %s\n", webhook.Source)
			fmt.Printf("Agent ID: %s\n", webhook.AgentID)
			fmt.Printf("Communication Method: %s\n", webhook.Communication.Method)
			fmt.Printf("Communication Destination: %s\n", webhook.Communication.Destination)
			fmt.Printf("Hide Headers: %t\n", webhook.HideWebhookHeaders)
			fmt.Printf("Filter: %s\n", webhook.Filter)
			fmt.Printf("Prompt: \n%s\n", webhook.Prompt)

			confirmCreate := readInput("\nCreate webhook? (y/N)", "n")
			if strings.ToLower(confirmCreate) != "y" {
				return fmt.Errorf("webhook creation cancelled")
			}

			// Create the webhook
			created, err := client.CreateWebhook(cmd.Context(), webhook)
			if err != nil {
				return fmt.Errorf("failed to create webhook: %w", err)
			}

			fmt.Printf("\n✅ Created webhook: %s (%s)\n", created.Name, created.ID)
			fmt.Printf("📎 Webhook URL: %s\n", created.WebhookURL)

			// Extract and show template variables
			templateVars := extractTemplateVars(created.Prompt)
			if len(templateVars) > 0 {
				fmt.Printf("\n📝 Template Variables:\n")
				for _, v := range templateVars {
					fmt.Printf("- %s\n", v)
				}
			}

			// Ask if they want to test the webhook
			testWebhook := readInput("\nTest the webhook now? (y/N)", "n")
			if strings.ToLower(testWebhook) == "y" {
				var testData map[string]interface{}

				if len(templateVars) > 0 {
					// Create test data with example values for template variables
					testData = make(map[string]interface{})

					// Use nested objects for variables like event.issue.key
					for _, v := range templateVars {
						parts := strings.Split(v, ".")

						// Start with the root object
						current := testData

						// Create nested objects for each part except the last
						for i, part := range parts {
							if i == len(parts)-1 {
								// For the last part, set a sample value
								current[part] = fmt.Sprintf("sample-%s", part)
							} else {
								// For intermediate parts, create a nested object if needed
								if _, exists := current[part]; !exists {
									current[part] = make(map[string]interface{})
								}
								current = current[part].(map[string]interface{})
							}
						}
					}

					// Add test metadata
					testData["_test"] = map[string]interface{}{
						"timestamp": time.Now().Format(time.RFC3339),
						"message":   "Test webhook from Kubiya CLI wizard",
					}
				} else {
					// Simple default test data
					testData = map[string]interface{}{
						"test":      true,
						"timestamp": time.Now().Format(time.RFC3339),
						"message":   "Test webhook from Kubiya CLI wizard",
					}
				}

				fmt.Println("📤 Sending test data to webhook...")
				prettyJSON, _ := json.MarshalIndent(testData, "", "  ")
				fmt.Printf("Payload:\n%s\n\n", string(prettyJSON))

				// Ask if they want to wait for a response
				waitForResponse := readInput("Wait for response? (y/N)", "n")

				if strings.ToLower(waitForResponse) == "y" {
					fmt.Println("⏳ Waiting for webhook to process...")
					resp, err := client.TestWebhookWithResponse(cmd.Context(), created.WebhookURL, testData)
					if err != nil {
						return fmt.Errorf("webhook test failed: %w", err)
					}

					// Print response status
					fmt.Printf("✅ Webhook test successful (Status: %d)\n", resp.StatusCode)

					// If we have a response body, print it
					if resp.Body != nil && resp.ContentLength > 0 {
						fmt.Println("\n📬 Response:")

						// Try to parse and pretty print the response if it's JSON
						var respData interface{}
						responseBody, _ := io.ReadAll(resp.Body)

						if err := json.Unmarshal(responseBody, &respData); err == nil {
							// It's valid JSON, pretty print it
							prettyResp, _ := json.MarshalIndent(respData, "", "  ")
							fmt.Println(string(prettyResp))
						} else {
							// Not JSON, print as is
							fmt.Println(string(responseBody))
						}
					}
				} else {
					if err := client.TestWebhook(cmd.Context(), created.WebhookURL, testData); err != nil {
						return fmt.Errorf("webhook test failed: %w", err)
					}
					fmt.Println("✅ Webhook test successful")
				}
			}

			return nil
		},
	}

	return cmd
}

// extractTemplateVars extracts Go template variables from the prompt string
func extractTemplateVars(prompt string) []string {
	// Regular expression to find template variables like {{.event.field}}
	varRegex := regexp.MustCompile(`{{[\s]*\.([^{}]+)[\s]*}}`)
	matches := varRegex.FindAllStringSubmatch(prompt, -1)

	// Deduplicate the variables
	varsMap := make(map[string]bool)
	for _, match := range matches {
		if len(match) > 1 {
			varsMap[match[1]] = true
		}
	}

	// Convert map to slice
	var vars []string
	for v := range varsMap {
		vars = append(vars, v)
	}

	// Sort the variables for consistent output
	sort.Strings(vars)
	return vars
}

// printMapInDotNotation prints a map in dot notation format
func printMapInDotNotation(data interface{}, indent string, prefix string) {
	switch v := data.(type) {
	case map[string]interface{}:
		for k, val := range v {
			key := k
			if prefix != "" {
				key = prefix + "." + k
			}

			switch child := val.(type) {
			case map[string]interface{}:
				fmt.Printf("%s%s:\n", indent, key)
				printMapInDotNotation(child, indent+"  ", key)
			case []interface{}:
				fmt.Printf("%s%s: (array with %d items)\n", indent, key, len(child))
				for i, item := range child {
					arrayKey := fmt.Sprintf("%s[%d]", key, i)
					fmt.Printf("%s  %s:\n", indent, arrayKey)
					printMapInDotNotation(item, indent+"    ", "")
				}
			default:
				fmt.Printf("%s%s: %v\n", indent, key, val)
			}
		}
	case []interface{}:
		fmt.Printf("%s(array with %d items)\n", indent, len(v))
		for i, item := range v {
			fmt.Printf("%s  [%d]:\n", indent, i)
			printMapInDotNotation(item, indent+"    ", "")
		}
	default:
		if prefix != "" {
			fmt.Printf("%s%s: %v\n", indent, prefix, v)
		} else {
			fmt.Printf("%s%v\n", indent, v)
		}
	}
}

// convertDotNotationToNested transforms keys with dots like "event.issue.key"
// into nested objects {"event":{"issue":{"key":"value"}}}
func convertDotNotationToNested(flatMap map[string]interface{}) map[string]interface{} {
	result := make(map[string]interface{})

	for key, value := range flatMap {
		// Skip keys that don't contain dots
		if !strings.Contains(key, ".") {
			result[key] = value
			continue
		}

		// Split the key by dots
		parts := strings.Split(key, ".")

		// Start with the root of the result
		current := result

		// For each part except the last one, create nested maps as needed
		for i := 0; i < len(parts)-1; i++ {
			part := parts[i]

			// If this part doesn't exist yet in the current map, create it
			if _, exists := current[part]; !exists {
				current[part] = make(map[string]interface{})
			}

			// If it exists but is not a map, we have a conflict
			nextMap, isMap := current[part].(map[string]interface{})
			if !isMap {
				// Handle conflict by creating a new map and copying the old value
				// under a special key
				oldValue := current[part]
				nextMap = make(map[string]interface{})
				nextMap["_value"] = oldValue
				current[part] = nextMap
			}

			// Move down into the next level
			current = nextMap
		}

		// Set the value at the final part
		current[parts[len(parts)-1]] = value
	}

	return result
}