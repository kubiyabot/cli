package cli

import (
	"bufio"
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"regexp"
	"sort"
	"strings"
	"text/tabwriter"
	"time"

	"github.com/kubiyabot/cli/internal/config"
	"github.com/kubiyabot/cli/internal/kubiya"
	"github.com/spf13/cobra"
	"gopkg.in/yaml.v3"
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
		newTestWebhookCommand(cfg),
		newImportWebhookCommand(cfg),
		newExportWebhookCommand(cfg),
		newWizardWebhookCommand(cfg),
	)

	return cmd
}

func newListWebhooksCommand(cfg *config.Config) *cobra.Command {
	var outputFormat string

	cmd := &cobra.Command{
		Use:     "list",
		Short:   "📋 List all webhooks",
		Example: "  kubiya webhook list\n  kubiya webhook list --output json\n  kubiya webhook list --output yaml",
		RunE: func(cmd *cobra.Command, args []string) error {
			client := kubiya.NewClient(cfg)
			webhooks, err := client.ListWebhooks(cmd.Context())
			if err != nil {
				return err
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
				// Detailed tabular output
				w := tabwriter.NewWriter(os.Stdout, 0, 0, 2, ' ', 0)
				fmt.Fprintln(w, "🔗 WEBHOOKS (DETAILED VIEW)")
				fmt.Fprintln(w, "ID\tNAME\tSOURCE\tDESTINATION\tMETHOD\tFILTER\tCREATED BY\tMANAGED BY")
				for _, wh := range webhooks {
					filter := wh.Filter
					if filter == "" {
						filter = "<none>"
					}

					managedBy := wh.ManagedBy
					if managedBy == "" {
						managedBy = "<none>"
					}

					fmt.Fprintf(w, "%s\t%s\t%s\t%s\t%s\t%s\t%s\t%s\n",
						wh.ID,
						wh.Name,
						wh.Source,
						wh.Communication.Destination,
						wh.Communication.Method,
						filter,
						wh.CreatedBy,
						managedBy,
					)
				}
				return w.Flush()
			case "text":
				fallthrough
			default:
				// Standard tabular output
				w := tabwriter.NewWriter(os.Stdout, 0, 0, 2, ' ', 0)
				fmt.Fprintln(w, "🔗 WEBHOOKS")
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

				// Add helpful tip at the bottom
				if err := w.Flush(); err != nil {
					return err
				}

				fmt.Println("\n📌 Tips:")
				fmt.Println("- Use 'kubiya webhook get <id>' to see more details")
				fmt.Println("- Use '--output wide' to see additional fields")
				fmt.Println("- Use '--output json' or '--output yaml' for machine-readable output")

				return nil
			}
		},
	}

	cmd.Flags().StringVarP(&outputFormat, "output", "o", "text", "Output format (text|wide|json|yaml)")
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

				// Print communication details with appropriate formatting
				fmt.Printf("Communication:\n")
				switch webhook.Communication.Method {
				case "Slack":
					fmt.Printf("  Method: %s\n", webhook.Communication.Method)
					fmt.Printf("  Destination: %s\n", webhook.Communication.Destination)
				case "Teams":
					fmt.Printf("  Method: %s\n", webhook.Communication.Method)
					// For Teams, try to parse and pretty print the JSON destination
					if strings.HasPrefix(webhook.Communication.Destination, "#{") && strings.HasSuffix(webhook.Communication.Destination, "}") {
						// Extract the JSON part
						jsonStr := webhook.Communication.Destination[1:] // Remove the leading #
						var teamsConfig map[string]string
						if err := json.Unmarshal([]byte(jsonStr), &teamsConfig); err == nil {
							fmt.Printf("  Team: %s\n", teamsConfig["team_name"])
							fmt.Printf("  Channel: %s\n", teamsConfig["channel_name"])
						} else {
							fmt.Printf("  Destination: %s\n", webhook.Communication.Destination)
						}
					} else {
						fmt.Printf("  Destination: %s\n", webhook.Communication.Destination)
					}
				case "HTTP":
					fmt.Printf("  Method: %s\n", webhook.Communication.Method)
					if webhook.Communication.Destination != "" {
						fmt.Printf("  Destination: %s\n", webhook.Communication.Destination)
					} else {
						fmt.Printf("  Destination: <direct HTTP response>\n")
					}
				default:
					fmt.Printf("  Method: %s\n", webhook.Communication.Method)
					fmt.Printf("  Destination: %s\n", webhook.Communication.Destination)
				}

				fmt.Printf("  Hide Headers: %t\n", webhook.Communication.HideHeaders)

				// Print filter details
				if webhook.Filter != "" {
					fmt.Printf("Filter: %s\n", webhook.Filter)
				} else {
					fmt.Printf("Filter: <none>\n")
				}

				// Print management details
				if webhook.ManagedBy != "" {
					fmt.Printf("Managed By: %s\n", webhook.ManagedBy)
					if webhook.TaskID != "" {
						fmt.Printf("Task ID: %s\n", webhook.TaskID)
					}
				}

				// Print timestamps
				if webhook.CreatedAt != "" && webhook.CreatedAt != "1970-01-01T00:00:00Z" && webhook.CreatedAt != "0001-01-01T00:00:00Z" {
					fmt.Printf("Created At: %s\n", webhook.CreatedAt)
				}
				if webhook.UpdatedAt != "" && webhook.UpdatedAt != "0001-01-01T00:00:00Z" {
					fmt.Printf("Updated At: %s\n", webhook.UpdatedAt)
				}
				if webhook.CreatedBy != "" {
					fmt.Printf("Created By: %s\n", webhook.CreatedBy)
				}

				// Extract template variables from the prompt
				fmt.Printf("\nPrompt: \n%s\n", webhook.Prompt)

				templateVars := extractTemplateVars(webhook.Prompt)
				if len(templateVars) > 0 {
					fmt.Printf("\n📝 Template Variables:\n")
					for _, v := range templateVars {
						fmt.Printf("- %s\n", v)
					}
				}

				fmt.Printf("\n📎 Webhook URL: %s\n", webhook.WebhookURL)

				// Print example usage for testing
				fmt.Printf("\n🧪 Test this webhook:\n")
				fmt.Printf("  kubiya webhook test --id %s --data '{\"test\": true}'\n", webhook.ID)

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
		hideHeaders bool
		fromFile    string
		fromStdin   bool
		teamName    string
		channelName string
	)

	cmd := &cobra.Command{
		Use:   "create",
		Short: "📝 Create new webhook",
		Example: `  # Create a webhook with basic parameters
  kubiya webhook create \
    --name "GitHub PR" \
    --source "github" \
    --agent-id "abc-123" \
    --method "Slack" \
    --destination "#devops" \
    --prompt "New PR: {{.event.pull_request.title}}"

  # Create a Teams webhook
  kubiya webhook create \
    --name "Jira Issues" \
    --source "jira" \
    --agent-id "abc-123" \
    --method "Teams" \
    --team-name "kubiya.ai" \
    --channel-name "General" \
    --prompt "New issue: {{.event.issue.key}}"

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

			// Create webhook from flags
			webhook := kubiya.Webhook{
				Name:    name,
				Source:  source,
				AgentID: agentID,
				Communication: kubiya.Communication{
					HideHeaders: hideHeaders,
				},
				Filter: filter,
				Prompt: prompt,
			}

			// Set method and destination based on specified method
			webhook.Communication.Method = method

			// Teams-specific processing
			if strings.EqualFold(method, "Teams") {
				// If team and channel provided, format as Teams destination
				if teamName != "" && channelName != "" {
					webhook.Communication.Method = "Teams"
					webhook.Communication.Destination = fmt.Sprintf("#{\"team_name\": \"%s\", \"channel_name\": \"%s\"}", teamName, channelName)
				} else if destination == "" {
					return fmt.Errorf("for Teams webhooks, you must provide either --destination or both --team-name and --channel-name")
				} else {
					webhook.Communication.Destination = destination
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
		},
	}

	// Basic parameters
	cmd.Flags().StringVarP(&name, "name", "n", "", "Webhook name")
	cmd.Flags().StringVarP(&source, "source", "s", "", "Event source")
	cmd.Flags().StringVarP(&agentID, "agent-id", "a", "", "Agent ID")
	cmd.Flags().StringVarP(&method, "method", "m", "Slack", "Communication method (Slack|Teams|HTTP)")
	cmd.Flags().StringVarP(&destination, "destination", "d", "", "Communication destination (channel for Slack, JSON for Teams)")
	cmd.Flags().StringVarP(&filter, "filter", "f", "", "Event filter (JMESPath expression)")
	cmd.Flags().StringVarP(&prompt, "prompt", "p", "", "Agent prompt with template variables ({{.event.*}})")
	cmd.Flags().BoolVar(&hideHeaders, "hide-headers", false, "Hide webhook headers in notifications")

	// Teams-specific flags
	cmd.Flags().StringVar(&teamName, "team-name", "", "Teams team name (when method is Teams)")
	cmd.Flags().StringVar(&channelName, "channel-name", "", "Teams channel name (when method is Teams)")

	// File input flags
	cmd.Flags().StringVar(&fromFile, "file", "", "File containing webhook definition (JSON or YAML)")
	cmd.Flags().BoolVar(&fromStdin, "stdin", false, "Read webhook definition from stdin")

	// Only require name, source, agent-id, and prompt if not using file/stdin
	cmd.MarkFlagsRequiredTogether("name", "source", "agent-id", "prompt")
	cmd.MarkFlagsMutuallyExclusive("file", "stdin", "name")
	cmd.MarkFlagsMutuallyExclusive("team-name", "destination")
	cmd.MarkFlagsRequiredTogether("team-name", "channel-name")

	return cmd
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
				existing.Communication.HideHeaders = hideHeaders
			}

			// Show changes
			fmt.Println("\n📝 Review changes:")
			fmt.Printf("Name: %s\n", existing.Name)
			fmt.Printf("Source: %s\n", existing.Source)
			fmt.Printf("Agent ID: %s\n", existing.AgentID)
			fmt.Printf("Communication Method: %s\n", existing.Communication.Method)
			fmt.Printf("Communication Destination: %s\n", existing.Communication.Destination)
			fmt.Printf("Hide Headers: %t\n", existing.Communication.HideHeaders)
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
  kubiya webhook test --id abc-123 --data '{"key": "value"}' --wait`,
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
			} else if dataFile != "" {
				// Data provided in file
				data, err := os.ReadFile(dataFile)
				if err != nil {
					return fmt.Errorf("failed to read data file: %w", err)
				}

				if err := json.Unmarshal(data, &testData); err != nil {
					return fmt.Errorf("invalid JSON in data file: %w", err)
				}
			} else {
				// Use default test data with template variable examples if webhook was retrieved
				if webhook != nil {
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
					// Simple default test data if we don't have the webhook
					testData = map[string]interface{}{
						"test":      true,
						"timestamp": time.Now().Format(time.RFC3339),
						"message":   "Test webhook from Kubiya CLI",
					}
				}
			}

			// Print the payload we're sending
			fmt.Println("📤 Sending test data to webhook...")
			prettyJSON, _ := json.MarshalIndent(testData, "", "  ")
			fmt.Printf("Payload:\n%s\n\n", string(prettyJSON))

			// Send the test with response handling
			if waitForResponse {
				fmt.Println("⏳ Waiting for webhook to process...")
				resp, err := client.TestWebhookWithResponse(cmd.Context(), webhookURL, testData)
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
	cmd.Flags().BoolVar(&waitForResponse, "wait", false, "Wait for webhook response")
	cmd.Flags().BoolVar(&verbose, "verbose", false, "Show detailed information")

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
		Name:    "example-webhook",
		Source:  "github",
		AgentID: "AGENT_ID_HERE",
		Communication: kubiya.Communication{
			Method:      "Slack",
			Destination: "#channel-name",
			HideHeaders: false,
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
			teammates, err := client.GetTeammates(cmd.Context())
			if err != nil {
				fmt.Println("⚠️ Could not fetch agents. You'll need to enter the agent ID manually.")
			} else {
				w := tabwriter.NewWriter(os.Stdout, 0, 0, 2, ' ', 0)
				fmt.Fprintln(w, "ID\tNAME\tDESCRIPTION")
				for _, t := range teammates {
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
				teamName := readInput("Teams team name", "")
				channelName := readInput("Teams channel name", "")
				if teamName != "" && channelName != "" {
					webhook.Communication.Destination = fmt.Sprintf("#{\"team_name\": \"%s\", \"channel_name\": \"%s\"}", teamName, channelName)
				} else {
					return fmt.Errorf("team name and channel name are required for Teams communication")
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
			webhook.Communication.HideHeaders = strings.ToLower(hideHeadersChoice) == "y"

			// Review webhook details
			fmt.Println("\n📝 Review webhook details:")
			fmt.Printf("Name: %s\n", webhook.Name)
			fmt.Printf("Source: %s\n", webhook.Source)
			fmt.Printf("Agent ID: %s\n", webhook.AgentID)
			fmt.Printf("Communication Method: %s\n", webhook.Communication.Method)
			fmt.Printf("Communication Destination: %s\n", webhook.Communication.Destination)
			fmt.Printf("Hide Headers: %t\n", webhook.Communication.HideHeaders)
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
