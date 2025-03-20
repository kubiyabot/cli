package cli

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"sort"
	"strings"
	"text/tabwriter"

	"github.com/kubiyabot/cli/internal/config"
	"github.com/kubiyabot/cli/internal/kubiya"
	"github.com/kubiyabot/cli/internal/style"
	"github.com/kubiyabot/cli/internal/tui"
	"github.com/spf13/cobra"
	"gopkg.in/yaml.v3"
)

func newTeammateCommand(cfg *config.Config) *cobra.Command {
	cmd := &cobra.Command{
		Use:     "teammate",
		Aliases: []string{"teammates", "tm"},
		Short:   "👥 Manage teammates",
		Long:    `Create, edit, delete, and list teammates in your Kubiya workspace.`,
	}

	cmd.AddCommand(
		newListTeammatesCommand(cfg),
		newCreateTeammateCommand(cfg),
		newEditTeammateCommand(cfg),
		newDeleteTeammateCommand(cfg),
		newGetTeammateCommand(cfg),
	)

	return cmd
}

func newCreateTeammateCommand(cfg *config.Config) *cobra.Command {
	var (
		name                string
		description         string
		interactive         bool
		llmModel            string
		instructionType     string
		inputFile           string
		inputFormat         string
		fromStdin           bool
		sources             []string
		secrets             []string
		integrations        []string
		envVars             []string
		inlineSourceFile    string
		inlineSourceStdin   bool
		webhooks            []string // Existing webhook IDs to attach
		webhookDestinations []string // Destinations for created webhooks (depends on method type)
		webhookMethod       string   // Webhook method (slack, teams, http, etc.)
		webhookPrompt       string   // Prompt for created webhooks
		webhookFile         string   // File containing webhook definitions
		knowledgeItems      []string // Existing knowledge item IDs to attach
		knowledgeFiles      []string // Files to create as knowledge items
		knowledgeLabels     []string // Labels for created knowledge items
	)

	cmd := &cobra.Command{
		Use:   "create",
		Short: "➕ Create new teammate",
		Example: `  # Create interactively with advanced form
  kubiya teammate create --interactive

  # Create from JSON/YAML file
  kubiya teammate create --file teammate.json
  kubiya teammate create --file teammate.yaml --format yaml

  # Create from stdin
  cat teammate.json | kubiya teammate create --stdin

  # Create with parameters
  kubiya teammate create --name "DevOps Bot" --desc "Handles DevOps tasks" \
    --source abc-123 --source def-456 \
    --secret DB_PASSWORD --env "LOG_LEVEL=debug" \
    --integration github

  # Create with webhook - system will generate a webhook URL as output
  kubiya teammate create --name "Slack Bot" \
    --webhook-dest "#alerts" --webhook-method slack \
    --webhook-prompt "Please analyze this alert"

  # Create HTTP webhook (system provides the webhook URL)
  kubiya teammate create --name "API Bot" --webhook-method http \
    --webhook-prompt "Process this API request"

  # Create with multiple webhook types
  kubiya teammate create --name "Notification Bot" \
    --webhook-method http \
    --webhook-dest "#dev-alerts" --webhook-method slack \
    --webhook-prompt "Process this notification"

  # Create with webhooks from file
  kubiya teammate create --name "WebhookBot" --webhook-file webhooks.json

  # Create with knowledge item
  kubiya teammate create --name "Docs Bot" --knowledge-file docs.md`,
		RunE: func(cmd *cobra.Command, args []string) error {
			client := kubiya.NewClient(cfg)

			var teammate kubiya.Teammate
			var err error
			var createdResources []string // Track resources created to show in summary

			// Handle inline sources first if provided
			if inlineSourceFile != "" || inlineSourceStdin {
				// We need to create a temporary stand-alone source using sources.go functionality
				var sourceName string

				if name != "" {
					sourceName = fmt.Sprintf("%s - Inline Source", name)
				} else {
					sourceName = "Inline Source for Teammate"
				}

				// Set up command to use sources functionality
				args := []string{"source", "add"}

				if inlineSourceFile != "" {
					fmt.Printf("📄 Using inline source from file: %s\n", inlineSourceFile)
					args = append(args, "--inline", inlineSourceFile)
				} else if inlineSourceStdin {
					fmt.Println("📥 Reading inline source from stdin...")
					args = append(args, "--inline-stdin")
				}

				// Add name
				args = append(args, "--name", sourceName)

				// Add yes flag to skip confirmation
				args = append(args, "--yes")

				// If runner is specified, add it
				if llmModel != "" {
					args = append(args, "--runner", llmModel)
				}

				// Capture the output to extract the UUID
				fmt.Printf("Creating inline source '%s'...\n", sourceName)
				output, err := captureCommandOutput("kubiya", args...)
				if err != nil {
					return fmt.Errorf("failed to create inline source: %w\nOutput: %s", err, output)
				}

				// Extract the UUID from the output
				sourceUUID := extractUUIDFromSourceOutput(output)
				if sourceUUID == "" {
					return fmt.Errorf("failed to extract UUID from source creation output: %s", output)
				}

				fmt.Printf("✅ Created inline source with UUID: %s\n", sourceUUID)
				createdResources = append(createdResources, fmt.Sprintf("Source: %s", sourceUUID))

				// Add the source UUID to the teammate
				teammate.Sources = append(teammate.Sources, sourceUUID)
			}

			// Process knowledge files if provided
			var knowledgeIds []string
			if len(knowledgeFiles) > 0 {
				for _, filename := range knowledgeFiles {
					// Read file content
					content, err := os.ReadFile(filename)
					if err != nil {
						return fmt.Errorf("failed to read knowledge file '%s': %w", filename, err)
					}

					// Create a name from the filename if not specified
					itemName := filepath.Base(filename)
					itemName = strings.TrimSuffix(itemName, filepath.Ext(itemName))

					// Create the knowledge item
					item := kubiya.Knowledge{
						Name:        itemName,
						Description: fmt.Sprintf("Knowledge for teammate: %s", name),
						Labels:      knowledgeLabels,
						Content:     string(content),
						Type:        "knowledge",
						Source:      "teammate_creation",
					}

					created, err := client.CreateKnowledge(cmd.Context(), item)
					if err != nil {
						return fmt.Errorf("failed to create knowledge item from file '%s': %w", filename, err)
					}

					fmt.Printf("✅ Created knowledge item: %s (UUID: %s)\n", created.Name, created.UUID)
					createdResources = append(createdResources, fmt.Sprintf("Knowledge: %s (%s)", created.Name, created.UUID))

					// Add the knowledge UUID to the list
					knowledgeIds = append(knowledgeIds, created.UUID)
				}
			}

			// Add existing knowledge items
			knowledgeIds = append(knowledgeIds, knowledgeItems...)

			// Process input based on flags
			switch {
			case inputFile != "":
				// Read from file (JSON or YAML)
				teammate, err = readTeammateFromFile(inputFile, inputFormat)
				if err != nil {
					return err
				}
				fmt.Printf("📄 Parsed teammate configuration from %s\n", inputFile)

			case fromStdin:
				// Read from stdin
				teammate, err = readTeammateFromStdin(inputFormat)
				if err != nil {
					return err
				}
				fmt.Printf("📥 Parsed teammate configuration from stdin\n")

			case interactive:
				// Use enhanced TUI form
				fmt.Println("🖥️ Starting interactive teammate creation form...")
				form := tui.NewTeammateForm(cfg)
				result, err := form.Run()
				if err != nil {
					return err
				}
				if result == nil {
					return fmt.Errorf("teammate creation cancelled")
				}
				teammate = *result

			default:
				// Use command line parameters
				if name == "" {
					return fmt.Errorf("name is required when not using --interactive, --file or --stdin")
				}
				teammate = kubiya.Teammate{
					Name:            name,
					Description:     description,
					LLMModel:        llmModel,
					InstructionType: instructionType,
					Sources:         sources,
					Secrets:         secrets,
					Integrations:    integrations,
				}

				// Parse environment variables
				if len(envVars) > 0 {
					teammate.Environment = make(map[string]string)
					for _, env := range envVars {
						parts := strings.SplitN(env, "=", 2)
						if len(parts) != 2 {
							return fmt.Errorf("invalid environment variable format: %s (should be KEY=VALUE)", env)
						}
						teammate.Environment[parts[0]] = parts[1]
					}
				}
			}

			// Add knowledge items to teammate's properties
			if len(knowledgeIds) > 0 {
				if teammate.Tags == nil {
					teammate.Tags = []string{}
				}
				for _, id := range knowledgeIds {
					teammate.Tags = append(teammate.Tags, fmt.Sprintf("knowledge:%s", id))
				}
			}

			// Augment the teammate with the proper structure expected by the API
			if teammate.Starters == nil {
				teammate.Starters = []interface{}{}
			}

			// Initialize empty slices if nil to ensure proper JSON serialization
			if teammate.Sources == nil {
				teammate.Sources = []string{}
			}
			if teammate.Secrets == nil {
				teammate.Secrets = []string{}
			}
			if teammate.Integrations == nil {
				teammate.Integrations = []string{}
			}
			if teammate.Runners == nil {
				teammate.Runners = []string{}
			}
			if teammate.AllowedGroups == nil {
				teammate.AllowedGroups = []string{}
			}
			if teammate.AllowedUsers == nil {
				teammate.AllowedUsers = []string{}
			}
			if teammate.Owners == nil {
				teammate.Owners = []string{}
			}
			if teammate.Tools == nil {
				teammate.Tools = []string{}
			}
			if teammate.Tasks == nil {
				teammate.Tasks = []string{}
			}
			if teammate.Tags == nil {
				teammate.Tags = []string{}
			}

			// Validate the teammate - this cannot be bypassed
			if err := validateTeammate(client, cmd.Context(), &teammate); err != nil {
				// Print a more helpful message with validation instructions
				fmt.Printf("\n%s\n", style.ErrorStyle.Render("❌ Validation Error"))
				fmt.Printf("%s\n\n", err.Error())

				return fmt.Errorf("validation failed")
			}

			// Show confirmation with details before proceeding
			fmt.Printf("\n%s\n\n", style.TitleStyle.Render(" 🤖 Creating Teammate "))
			fmt.Printf("  Name: %s\n", style.HighlightStyle.Render(teammate.Name))
			fmt.Printf("  Description: %s\n", teammate.Description)
			fmt.Printf("  LLM Model: %s\n", teammate.LLMModel)
			fmt.Printf("  Type: %s\n", teammate.InstructionType)

			if len(teammate.Sources) > 0 {
				fmt.Printf("  Sources: %d\n", len(teammate.Sources))
			}
			if len(teammate.Secrets) > 0 {
				fmt.Printf("  Secrets: %d\n", len(teammate.Secrets))
			}
			if len(teammate.Environment) > 0 {
				fmt.Printf("  Environment Variables: %d\n", len(teammate.Environment))
			}
			if len(teammate.Integrations) > 0 {
				fmt.Printf("  Integrations: %d\n", len(teammate.Integrations))
			}
			if len(knowledgeIds) > 0 {
				fmt.Printf("  Knowledge Items: %d\n", len(knowledgeIds))
			}
			fmt.Println()

			// Send to API
			created, err := client.CreateTeammate(cmd.Context(), teammate)
			if err != nil {
				// Enhanced error handling
				return fmt.Errorf("failed to create teammate: %w", err)
			}

			// Ensure we have a valid UUID before displaying
			if created.UUID == "" {
				fmt.Printf("\n%s\n", style.WarningStyle.Render("⚠️ Warning: API returned a teammate with empty UUID!"))
				fmt.Printf("The teammate may have been created, but the UUID was not returned by the API.\n")
				fmt.Printf("Please check the 'kubiya teammate list' command to find your teammate.\n\n")

				// Return with a more specific error to help debugging
				if created.ID != "" {
					fmt.Printf("Note: The API returned an ID field (%s) but not a UUID field.\n", style.HighlightStyle.Render(created.ID))
					fmt.Printf("This may indicate a mismatch between the CLI and API versions.\n\n")
				}
			}

			// Process webhooks if provided
			if len(webhooks) > 0 || len(webhookDestinations) > 0 || webhookFile != "" {
				// First, make sure we have a valid UUID
				if created.UUID == "" && created.ID != "" {
					created.UUID = created.ID
				}

				if created.UUID == "" {
					fmt.Printf("⚠️ Cannot attach webhooks: teammate UUID is empty\n")
				} else {
					// Attach existing webhooks
					for _, webhookID := range webhooks {
						if err := attachWebhookToTeammate(cmd.Context(), client, webhookID, created.UUID); err != nil {
							fmt.Printf("⚠️ Failed to attach webhook %s: %v\n", webhookID, err)
						} else {
							fmt.Printf("✅ Attached webhook %s to teammate\n", webhookID)
							createdResources = append(createdResources, fmt.Sprintf("Attached webhook: %s", webhookID))
						}
					}

					// Create new webhooks
					for _, dest := range webhookDestinations {
						method := webhookMethod
						if method == "" {
							method = "http" // Default method if not specified
						}

						prompt := webhookPrompt
						if prompt == "" {
							prompt = fmt.Sprintf("Default prompt for %s webhook", method)
						}

						webhook, err := createWebhook(cmd.Context(), client, created.UUID, dest, method, prompt)
						if err != nil {
							fmt.Printf("⚠️ Failed to create webhook for %s: %v\n", dest, err)
						} else {
							fmt.Printf("✅ Created %s webhook (ID: %s)\n", method, webhook.ID)
							if webhook.WebhookURL != "" {
								fmt.Printf("   📋 Webhook URL: %s\n", style.HighlightStyle.Render(webhook.WebhookURL))
								createdResources = append(createdResources, fmt.Sprintf("Created %s webhook: %s (URL: %s)",
									method, webhook.ID, webhook.WebhookURL))
							} else {
								createdResources = append(createdResources, fmt.Sprintf("Created %s webhook: %s",
									method, webhook.ID))
							}
						}
					}

					// Create HTTP webhook if specified without any destinations
					if webhookMethod == "http" && len(webhookDestinations) == 0 {
						prompt := webhookPrompt
						if prompt == "" {
							prompt = "Default prompt for HTTP webhook"
						}

						webhook, err := createWebhook(cmd.Context(), client, created.UUID, "", "http", prompt)
						if err != nil {
							fmt.Printf("⚠️ Failed to create HTTP webhook: %v\n", err)
						} else {
							fmt.Printf("✅ Created HTTP webhook (ID: %s)\n", webhook.ID)
							if webhook.WebhookURL != "" {
								fmt.Printf("   📋 Webhook URL: %s\n", style.HighlightStyle.Render(webhook.WebhookURL))
								fmt.Printf("   📊 Use the webhooks API or web interface to track webhook activity\n")
								createdResources = append(createdResources, fmt.Sprintf("Created HTTP webhook: %s (URL: %s)",
									webhook.ID, webhook.WebhookURL))
							} else {
								createdResources = append(createdResources, fmt.Sprintf("Created HTTP webhook: %s",
									webhook.ID))
							}
						}
					}

					// Process webhooks from file if specified
					if webhookFile != "" {
						// Read file content
						webhookData, err := os.ReadFile(webhookFile)
						if err != nil {
							fmt.Printf("⚠️ Failed to read webhook file: %v\n", err)
						} else {
							// Parse the webhooks (format depends on file extension)
							var webhooks []kubiya.Webhook
							ext := strings.ToLower(filepath.Ext(webhookFile))
							if ext == ".json" {
								if err := json.Unmarshal(webhookData, &webhooks); err != nil {
									// Try single webhook parse if array fails
									var singleWebhook kubiya.Webhook
									if err := json.Unmarshal(webhookData, &singleWebhook); err != nil {
										fmt.Printf("⚠️ Failed to parse webhook JSON: %v\n", err)
									} else {
										webhooks = []kubiya.Webhook{singleWebhook}
									}
								}
							} else if ext == ".yaml" || ext == ".yml" {
								if err := yaml.Unmarshal(webhookData, &webhooks); err != nil {
									// Try single webhook parse if array fails
									var singleWebhook kubiya.Webhook
									if err := yaml.Unmarshal(webhookData, &singleWebhook); err != nil {
										fmt.Printf("⚠️ Failed to parse webhook YAML: %v\n", err)
									} else {
										webhooks = []kubiya.Webhook{singleWebhook}
									}
								}
							} else {
								fmt.Printf("⚠️ Unsupported webhook file format: %s (use .json, .yaml, or .yml)\n", ext)
							}

							// Create each webhook
							for i, webhook := range webhooks {
								// Set the teammate ID
								webhook.AgentID = created.UUID

								// Set defaults if needed
								if webhook.Name == "" {
									webhook.Name = fmt.Sprintf("Webhook %d for %s", i+1, created.UUID)
								}
								if webhook.Communication.Method == "" {
									webhook.Communication.Method = "http"
								}
								if webhook.Prompt == "" {
									webhook.Prompt = fmt.Sprintf("Default prompt for %s webhook", webhook.Communication.Method)
								}

								// Create the webhook
								createdWebhook, err := client.CreateWebhook(cmd.Context(), webhook)
								if err != nil {
									fmt.Printf("⚠️ Failed to create webhook %d from file: %v\n", i+1, err)
									continue
								}

								fmt.Printf("✅ Created %s webhook from file (ID: %s)\n",
									webhook.Communication.Method, createdWebhook.ID)

								if createdWebhook.WebhookURL != "" {
									fmt.Printf("   📋 Webhook URL: %s\n", style.HighlightStyle.Render(createdWebhook.WebhookURL))
									createdResources = append(createdResources, fmt.Sprintf("Created %s webhook: %s (URL: %s)",
										webhook.Communication.Method, createdWebhook.ID, createdWebhook.WebhookURL))
								} else {
									createdResources = append(createdResources, fmt.Sprintf("Created %s webhook: %s",
										webhook.Communication.Method, createdWebhook.ID))
								}
							}
						}
					}
				}
			}

			// Display the teammate info
			fmt.Printf("%s Created teammate: %s (UUID: %s)\n\n",
				style.SuccessStyle.Render("✅"),
				style.HighlightStyle.Render(created.Name),
				style.DimStyle.Render(created.UUID))

			// Show created resources summary if any
			if len(createdResources) > 0 {
				fmt.Printf("%s\n", style.SubtitleStyle.Render("Created Resources"))
				for _, resource := range createdResources {
					fmt.Printf("• %s\n", resource)
				}
				fmt.Println()
			}

			// Helpful next steps
			fmt.Printf("%s\n", style.SubtitleStyle.Render("Next Steps"))
			if created.UUID != "" {
				fmt.Printf("• View details: %s\n",
					style.CommandStyle.Render(fmt.Sprintf("kubiya teammate get %s", created.UUID)))
				fmt.Printf("• Edit teammate: %s\n",
					style.CommandStyle.Render(fmt.Sprintf("kubiya teammate edit %s --interactive", created.UUID)))
			} else {
				fmt.Printf("• List all teammates: %s\n",
					style.CommandStyle.Render("kubiya teammate list"))
				fmt.Printf("• Create another teammate: %s\n",
					style.CommandStyle.Render("kubiya teammate create --interactive"))
			}

			return nil
		},
	}

	// Basic info flags
	cmd.Flags().StringVarP(&name, "name", "n", "", "Teammate name")
	cmd.Flags().StringVarP(&description, "desc", "d", "", "Teammate description")
	cmd.Flags().BoolVarP(&interactive, "interactive", "i", false, "Use interactive form")
	cmd.Flags().StringVar(&llmModel, "llm", "azure/gpt-4", "LLM model to use")
	cmd.Flags().StringVar(&instructionType, "type", "tools", "Instruction type")

	// Input file flags
	cmd.Flags().StringVarP(&inputFile, "file", "f", "", "File containing teammate configuration (JSON or YAML)")
	cmd.Flags().StringVar(&inputFormat, "format", "json", "Input format (json|yaml)")
	cmd.Flags().BoolVar(&fromStdin, "stdin", false, "Read configuration from stdin")

	// Component flags
	cmd.Flags().StringArrayVar(&sources, "source", []string{}, "Source UUID to attach (can be specified multiple times)")
	cmd.Flags().StringArrayVar(&secrets, "secret", []string{}, "Secret name to attach (can be specified multiple times)")
	cmd.Flags().StringArrayVar(&integrations, "integration", []string{}, "Integration to attach (can be specified multiple times)")
	cmd.Flags().StringArrayVar(&envVars, "env", []string{}, "Environment variable in KEY=VALUE format (can be specified multiple times)")

	// Add flags for inline sources
	cmd.Flags().StringVar(&inlineSourceFile, "inline-source", "", "File containing inline source tool definitions (YAML or JSON)")
	cmd.Flags().BoolVar(&inlineSourceStdin, "inline-source-stdin", false, "Read inline source tool definitions from stdin")

	// Add flags for webhooks
	cmd.Flags().StringArrayVar(&webhooks, "webhook", []string{}, "Existing webhook ID to attach (can be specified multiple times)")
	cmd.Flags().StringArrayVar(&webhookDestinations, "webhook-dest", []string{}, "Destination for new webhooks - required for slack/teams (not for http)")
	cmd.Flags().StringVar(&webhookMethod, "webhook-method", "http", "Webhook type (slack, teams, http) - determines destination format")
	cmd.Flags().StringVar(&webhookPrompt, "webhook-prompt", "", "Prompt for created webhooks")
	cmd.Flags().StringVar(&webhookFile, "webhook-file", "", "JSON or YAML file containing webhook definitions to create")

	// Add flags for knowledge
	cmd.Flags().StringArrayVar(&knowledgeItems, "knowledge", []string{}, "Knowledge item ID to attach (can be specified multiple times)")
	cmd.Flags().StringArrayVar(&knowledgeFiles, "knowledge-file", []string{}, "File to create as a knowledge item (can be specified multiple times)")
	cmd.Flags().StringArrayVar(&knowledgeLabels, "knowledge-labels", []string{}, "Labels for created knowledge items (can be specified multiple times)")

	// Add detailed description to the usage template
	cmd.Long += "\n\nWebhook Methods:\n" +
		"  - http: For HTTP webhooks. No destination needed - system provides a unique webhook URL.\n" +
		"  - slack: For Slack notifications. Destination should be a Slack channel or webhook URL.\n" +
		"  - teams: For Microsoft Teams. Destination should be a Teams webhook URL."

	return cmd
}

// Helper function to attach a webhook to a teammate
func attachWebhookToTeammate(ctx context.Context, client *kubiya.Client, webhookID, teammateID string) error {
	// Get the webhook
	webhook, err := client.GetWebhook(ctx, webhookID)
	if err != nil {
		return fmt.Errorf("failed to get webhook: %w", err)
	}

	// Update the webhook's agent ID
	webhook.AgentID = teammateID

	// Save the updated webhook
	_, err = client.UpdateWebhook(ctx, webhookID, *webhook)
	if err != nil {
		return fmt.Errorf("failed to update webhook: %w", err)
	}

	return nil
}

// Helper function to create a webhook for a teammate
func createWebhook(ctx context.Context, client *kubiya.Client, teammateID, destination, method, prompt string) (*kubiya.Webhook, error) {
	// Create a new webhook
	webhook := kubiya.Webhook{
		Name:    fmt.Sprintf("%s webhook for %s", method, teammateID),
		AgentID: teammateID,
		Communication: kubiya.Communication{
			Method:      method,
			Destination: destination,
		},
		Prompt: prompt,
	}

	// For HTTP method with empty destination, just proceed (system will provide URL)
	if method == "http" && destination == "" {
		// Communication.Destination can be empty for HTTP webhooks
	} else if destination == "" {
		// For non-HTTP methods, destination is required
		return nil, fmt.Errorf("destination is required for %s webhook", method)
	}

	// Create webhook via the API
	createdWebhook, err := client.CreateWebhook(ctx, webhook)
	if err != nil {
		return nil, fmt.Errorf("failed to create webhook: %w", err)
	}

	return createdWebhook, nil
}

// readTeammateFromFile reads and parses a teammate configuration from a file
func readTeammateFromFile(filepath, format string) (kubiya.Teammate, error) {
	data, err := os.ReadFile(filepath)
	if err != nil {
		return kubiya.Teammate{}, fmt.Errorf("failed to read file: %w", err)
	}

	return parseTeammateData(data, format)
}

// readTeammateFromStdin reads and parses a teammate configuration from stdin
func readTeammateFromStdin(format string) (kubiya.Teammate, error) {
	data, err := io.ReadAll(os.Stdin)
	if err != nil {
		return kubiya.Teammate{}, fmt.Errorf("failed to read from stdin: %w", err)
	}

	return parseTeammateData(data, format)
}

// parseTeammateData parses teammate data from JSON or YAML
func parseTeammateData(data []byte, format string) (kubiya.Teammate, error) {
	var teammate kubiya.Teammate

	switch format {
	case "json", "":
		if err := json.Unmarshal(data, &teammate); err != nil {
			return kubiya.Teammate{}, fmt.Errorf("invalid JSON: %w", err)
		}
	case "yaml", "yml":
		if err := yaml.Unmarshal(data, &teammate); err != nil {
			return kubiya.Teammate{}, fmt.Errorf("invalid YAML: %w", err)
		}
	default:
		return kubiya.Teammate{}, fmt.Errorf("unsupported format: %s", format)
	}

	return teammate, nil
}

// validateTeammate performs basic validation on a teammate
func validateTeammate(client *kubiya.Client, ctx context.Context, teammate *kubiya.Teammate) error {
	if teammate.Name == "" {
		return fmt.Errorf("teammate name cannot be empty")
	}

	// Set defaults if not provided
	if teammate.LLMModel == "" {
		teammate.LLMModel = "azure/gpt-4"
	}

	if teammate.InstructionType == "" {
		teammate.InstructionType = "tools"
	}

	// Ensure we have at least one owner
	// If no owners specified, use the current user ID (if available)
	if len(teammate.Owners) == 0 {
		// For now, we'll leave this empty and let the API handle it
		// The API should set the current user as owner
	}

	// Validate all sources have non-empty UUIDs and match the expected UUID format
	for i, sourceID := range teammate.Sources {
		if sourceID == "" {
			return fmt.Errorf("source at index %d has empty UUID", i)
		}

		// Basic UUID format validation (simple regex check)
		if !isValidUUID(sourceID) {
			return fmt.Errorf("source at index %d has invalid UUID format: %s", i, sourceID)
		}

		// Optional: Validate that the source exists
		// This requires an API call, so it might be expensive
	}

	// Validate all secrets have non-empty names
	for i, secret := range teammate.Secrets {
		if secret == "" {
			return fmt.Errorf("secret at index %d has empty name", i)
		}

		// Optionally validate that the secret exists
		// This requires an API call, so it might be expensive
	}

	// Validate all integrations have non-empty names and actually exist
	if len(teammate.Integrations) > 0 {
		// Get all available integrations
		integrations, err := client.ListIntegrations(ctx)
		if err != nil {
			return fmt.Errorf("failed to validate integrations: %w", err)
		}

		availableIntegrations := make(map[string]bool)
		for _, integration := range integrations {
			availableIntegrations[integration.Name] = true
		}

		for i, integration := range teammate.Integrations {
			if integration == "" {
				return fmt.Errorf("integration at index %d has empty name", i)
			}

			// Check if integration exists in the system
			if !availableIntegrations[integration] {
				return fmt.Errorf("integration '%s' does not exist in the system", integration)
			}
		}
	}

	// Validate environment variables have non-empty keys and values
	for key, value := range teammate.Environment {
		if key == "" {
			return fmt.Errorf("environment variable has empty key")
		}
		if value == "" {
			return fmt.Errorf("environment variable '%s' has empty value", key)
		}
	}

	// Ensure the starters field is initialized properly
	if teammate.Starters == nil {
		teammate.Starters = []interface{}{}
	}

	return nil
}

// isValidUUID performs a simple check to determine if a string looks like a UUID
// This is a basic validation, not a comprehensive UUID check
func isValidUUID(id string) bool {
	// Simple check for common UUID formats:
	// - 8-4-4-4-12 format (standard UUID)
	// - 32 hex characters (no dashes)
	// - Formats like "abc-123" that are sometimes used in the system

	// Check if it contains only valid hex chars and dashes
	for _, c := range id {
		if !((c >= '0' && c <= '9') || (c >= 'a' && c <= 'f') || (c >= 'A' && c <= 'F') || c == '-') {
			return false
		}
	}

	// Length check (with or without dashes)
	return len(id) > 8 && len(id) <= 36
}

func newEditTeammateCommand(cfg *config.Config) *cobra.Command {
	var (
		interactive        bool
		editor             bool
		name               string
		description        string
		llmModel           string
		instructions       string
		addSources         []string
		removeSources      []string
		addSecrets         []string
		removeSecrets      []string
		addEnvVars         []string
		removeEnvVars      []string
		addIntegrations    []string
		removeIntegrations []string
		outputFormat       string
		// New webhook-related variables
		addWebhooks         []string
		removeWebhooks      []string
		webhookDestinations []string
		webhookMethod       string
		webhookPrompt       string
		webhookFile         string
	)

	cmd := &cobra.Command{
		Use:   "edit [uuid]",
		Short: "✏️ Edit teammate",
		Example: `  # Edit interactively with form
  kubiya teammate edit abc-123 --interactive

  # Edit using JSON editor
  kubiya teammate edit abc-123 --editor

  # Edit specific fields
  kubiya teammate edit abc-123 --name "New Name" --desc "Updated description"
  
  # Add or remove components
  kubiya teammate edit abc-123 --add-source def-456 --remove-secret DB_PASSWORD
  kubiya teammate edit abc-123 --add-env "DEBUG=true" --remove-integration github
  
  # Add webhooks to teammate
  kubiya teammate edit abc-123 --webhook-method slack --webhook-dest "#alerts" --webhook-prompt "Alert received"
  
  # Add HTTP webhook (system generates the URL)
  kubiya teammate edit abc-123 --webhook-method http --webhook-prompt "Process this API request"
  
  # Add existing webhook by ID
  kubiya teammate edit abc-123 --add-webhook webhook-123-456
  
  # Remove webhook from teammate
  kubiya teammate edit abc-123 --remove-webhook webhook-789-012`,
		Args: cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			client := kubiya.NewClient(cfg)
			uuid := args[0]

			// Get existing teammate
			teammate, err := client.GetTeammate(cmd.Context(), uuid)
			if err != nil {
				return fmt.Errorf("failed to get teammate: %w", err)
			}

			var updated kubiya.Teammate
			var createdResources []string // Track resources created to show in summary

			if interactive {
				// Use TUI form
				fmt.Println("🖥️ Starting interactive teammate editing form...")
				form := tui.NewTeammateForm(cfg)
				form.SetDefaults(teammate)
				result, err := form.Run()
				if err != nil {
					return err
				}
				if result == nil {
					return fmt.Errorf("teammate editing cancelled")
				}
				updated = *result
			} else if editor {
				// Use JSON editor
				updated, err = editTeammateJSON(teammate)
				if err != nil {
					return err
				}
			} else if hasCommandLineChanges(name, description, llmModel, instructions,
				addSources, removeSources, addSecrets, removeSecrets,
				addEnvVars, removeEnvVars, addIntegrations, removeIntegrations) ||
				hasWebhookChanges(addWebhooks, removeWebhooks, webhookDestinations, webhookMethod, webhookPrompt) {

				// Apply command-line changes
				updated = *teammate

				// Update basic fields if provided
				if name != "" {
					updated.Name = name
				}
				if description != "" {
					updated.Description = description
				}
				if llmModel != "" {
					updated.LLMModel = llmModel
				}
				if instructions != "" {
					updated.AIInstructions = instructions
				}

				// Handle sources
				for _, source := range addSources {
					// Check if already exists
					exists := false
					for _, s := range updated.Sources {
						if s == source {
							exists = true
							break
						}
					}
					if !exists {
						updated.Sources = append(updated.Sources, source)
					}
				}
				for _, source := range removeSources {
					var newSources []string
					for _, s := range updated.Sources {
						if s != source {
							newSources = append(newSources, s)
						}
					}
					updated.Sources = newSources
				}

				// Handle secrets
				for _, secret := range addSecrets {
					// Check if already exists
					exists := false
					for _, s := range updated.Secrets {
						if s == secret {
							exists = true
							break
						}
					}
					if !exists {
						updated.Secrets = append(updated.Secrets, secret)
					}
				}
				for _, secret := range removeSecrets {
					var newSecrets []string
					for _, s := range updated.Secrets {
						if s != secret {
							newSecrets = append(newSecrets, s)
						}
					}
					updated.Secrets = newSecrets
				}

				// Handle environment variables
				if len(addEnvVars) > 0 && updated.Environment == nil {
					updated.Environment = make(map[string]string)
				}
				for _, env := range addEnvVars {
					parts := strings.SplitN(env, "=", 2)
					if len(parts) != 2 {
						return fmt.Errorf("invalid environment variable format: %s (should be KEY=VALUE)", env)
					}
					updated.Environment[parts[0]] = parts[1]
				}
				for _, key := range removeEnvVars {
					delete(updated.Environment, key)
				}

				// Handle integrations
				for _, integration := range addIntegrations {
					// Check if already exists
					exists := false
					for _, i := range updated.Integrations {
						if i == integration {
							exists = true
							break
						}
					}
					if !exists {
						updated.Integrations = append(updated.Integrations, integration)
					}
				}
				for _, integration := range removeIntegrations {
					var newIntegrations []string
					for _, i := range updated.Integrations {
						if i != integration {
							newIntegrations = append(newIntegrations, i)
						}
					}
					updated.Integrations = newIntegrations
				}
			} else {
				return fmt.Errorf("must specify either --interactive, --editor, or specific fields to change")
			}

			// Generate a diff for display
			diff := generateTeammateDiff(teammate, &updated)

			// Show changes
			fmt.Printf("\n%s\n\n", style.TitleStyle.Render(" 🔄 Updating Teammate "))

			if len(diff) == 0 && !hasWebhookChanges(addWebhooks, removeWebhooks, webhookDestinations, webhookMethod, webhookPrompt) {
				fmt.Println("No changes detected. Update cancelled.")
				return nil
			}

			if len(diff) > 0 {
				fmt.Println("Changes:")
				for _, change := range diff {
					fmt.Printf("  • %s\n", change)
				}
				fmt.Println()
			}

			// Confirm update with user
			if !confirmYesNo("Proceed with these changes?") {
				return fmt.Errorf("update cancelled")
			}

			// Update the teammate
			result, err := client.UpdateTeammate(cmd.Context(), uuid, updated)
			if err != nil {
				return fmt.Errorf("failed to update teammate: %w", err)
			}

			fmt.Printf("%s Updated teammate: %s\n\n",
				style.SuccessStyle.Render("✅"),
				style.HighlightStyle.Render(result.Name))

			// Process webhook changes after the teammate update
			// Process existing webhook attachments/detachments
			for _, webhookID := range addWebhooks {
				if err := attachWebhookToTeammate(cmd.Context(), client, webhookID, uuid); err != nil {
					fmt.Printf("⚠️ Failed to attach webhook %s: %v\n", webhookID, err)
				} else {
					fmt.Printf("✅ Attached webhook %s to teammate\n", webhookID)
					createdResources = append(createdResources, fmt.Sprintf("Attached webhook: %s", webhookID))
				}
			}

			for _, webhookID := range removeWebhooks {
				// Detach webhook - we need to get it, set agent_id to empty, and update it
				webhook, err := client.GetWebhook(cmd.Context(), webhookID)
				if err != nil {
					fmt.Printf("⚠️ Failed to get webhook %s: %v\n", webhookID, err)
					continue
				}

				// Only detach if it's actually attached to this teammate
				if webhook.AgentID == uuid {
					webhook.AgentID = ""
					if _, err := client.UpdateWebhook(cmd.Context(), webhookID, *webhook); err != nil {
						fmt.Printf("⚠️ Failed to detach webhook %s: %v\n", webhookID, err)
					} else {
						fmt.Printf("✅ Detached webhook %s from teammate\n", webhookID)
						createdResources = append(createdResources, fmt.Sprintf("Detached webhook: %s", webhookID))
					}
				} else {
					fmt.Printf("⚠️ Webhook %s is not attached to this teammate\n", webhookID)
				}
			}

			// Create new webhooks from command line arguments
			if len(webhookDestinations) > 0 || (webhookMethod == "http" && webhookPrompt != "") {
				// Create webhooks for each destination
				for _, dest := range webhookDestinations {
					method := webhookMethod
					if method == "" {
						method = "http" // Default method if not specified
					}

					prompt := webhookPrompt
					if prompt == "" {
						prompt = fmt.Sprintf("Default prompt for %s webhook", method)
					}

					webhook, err := createWebhook(cmd.Context(), client, uuid, dest, method, prompt)
					if err != nil {
						fmt.Printf("⚠️ Failed to create webhook for %s: %v\n", dest, err)
					} else {
						fmt.Printf("✅ Created %s webhook (ID: %s)\n", method, webhook.ID)
						if webhook.WebhookURL != "" {
							fmt.Printf("   📋 Webhook URL: %s\n", style.HighlightStyle.Render(webhook.WebhookURL))
							createdResources = append(createdResources, fmt.Sprintf("Created %s webhook: %s (URL: %s)",
								method, webhook.ID, webhook.WebhookURL))
						} else {
							createdResources = append(createdResources, fmt.Sprintf("Created %s webhook: %s",
								method, webhook.ID))
						}
					}
				}

				// Create HTTP webhook if specified without any destinations
				if webhookMethod == "http" && len(webhookDestinations) == 0 && webhookPrompt != "" {
					webhook, err := createWebhook(cmd.Context(), client, uuid, "", "http", webhookPrompt)
					if err != nil {
						fmt.Printf("⚠️ Failed to create HTTP webhook: %v\n", err)
					} else {
						fmt.Printf("✅ Created HTTP webhook (ID: %s)\n", webhook.ID)
						if webhook.WebhookURL != "" {
							fmt.Printf("   📋 Webhook URL: %s\n", style.HighlightStyle.Render(webhook.WebhookURL))
							fmt.Printf("   📊 Use the webhooks API or web interface to track webhook activity\n")
							createdResources = append(createdResources, fmt.Sprintf("Created HTTP webhook: %s (URL: %s)",
								webhook.ID, webhook.WebhookURL))
						} else {
							createdResources = append(createdResources, fmt.Sprintf("Created HTTP webhook: %s",
								webhook.ID))
						}
					}
				}
			}

			// Create webhooks from file if specified
			if webhookFile != "" {
				// Read file content
				webhookData, err := os.ReadFile(webhookFile)
				if err != nil {
					return fmt.Errorf("failed to read webhook file: %w", err)
				}

				// Parse the webhooks (format depends on file extension)
				var webhooks []kubiya.Webhook
				ext := strings.ToLower(filepath.Ext(webhookFile))
				if ext == ".json" {
					if err := json.Unmarshal(webhookData, &webhooks); err != nil {
						// Try single webhook parse if array fails
						var singleWebhook kubiya.Webhook
						if err := json.Unmarshal(webhookData, &singleWebhook); err != nil {
							return fmt.Errorf("failed to parse webhook JSON: %w", err)
						}
						webhooks = []kubiya.Webhook{singleWebhook}
					}
				} else if ext == ".yaml" || ext == ".yml" {
					if err := yaml.Unmarshal(webhookData, &webhooks); err != nil {
						// Try single webhook parse if array fails
						var singleWebhook kubiya.Webhook
						if err := yaml.Unmarshal(webhookData, &singleWebhook); err != nil {
							return fmt.Errorf("failed to parse webhook YAML: %w", err)
						}
						webhooks = []kubiya.Webhook{singleWebhook}
					}
				} else {
					return fmt.Errorf("unsupported webhook file format: %s (use .json, .yaml, or .yml)", ext)
				}

				// Create each webhook
				for i, webhook := range webhooks {
					// Set the teammate ID
					webhook.AgentID = uuid

					// Set defaults if needed
					if webhook.Name == "" {
						webhook.Name = fmt.Sprintf("Webhook %d for %s", i+1, uuid)
					}
					if webhook.Communication.Method == "" {
						webhook.Communication.Method = "http"
					}
					if webhook.Prompt == "" {
						webhook.Prompt = fmt.Sprintf("Default prompt for %s webhook", webhook.Communication.Method)
					}

					// Create the webhook
					createdWebhook, err := client.CreateWebhook(cmd.Context(), webhook)
					if err != nil {
						fmt.Printf("⚠️ Failed to create webhook %d from file: %v\n", i+1, err)
						continue
					}

					fmt.Printf("✅ Created %s webhook from file (ID: %s)\n",
						webhook.Communication.Method, createdWebhook.ID)

					if createdWebhook.WebhookURL != "" {
						fmt.Printf("   📋 Webhook URL: %s\n", style.HighlightStyle.Render(createdWebhook.WebhookURL))
						createdResources = append(createdResources, fmt.Sprintf("Created %s webhook: %s (URL: %s)",
							webhook.Communication.Method, createdWebhook.ID, createdWebhook.WebhookURL))
					} else {
						createdResources = append(createdResources, fmt.Sprintf("Created %s webhook: %s",
							webhook.Communication.Method, createdWebhook.ID))
					}
				}
			}

			// Show created resources summary if any
			if len(createdResources) > 0 {
				fmt.Printf("%s\n", style.SubtitleStyle.Render("Created/Updated Resources"))
				for _, resource := range createdResources {
					fmt.Printf("• %s\n", resource)
				}
				fmt.Println()
			}

			// Provide next steps
			fmt.Printf("%s\n", style.SubtitleStyle.Render("Next Steps"))
			fmt.Printf("• View details: %s\n",
				style.CommandStyle.Render(fmt.Sprintf("kubiya teammate get %s", uuid)))
			fmt.Printf("• List teammates: %s\n",
				style.CommandStyle.Render("kubiya teammate list"))

			return nil
		},
	}

	// Edit mode flags
	cmd.Flags().BoolVarP(&interactive, "interactive", "i", false, "Use interactive form")
	cmd.Flags().BoolVarP(&editor, "editor", "e", false, "Use JSON editor")
	cmd.Flags().StringVarP(&outputFormat, "output", "o", "text", "Output format (text|json)")

	// Basic field flags
	cmd.Flags().StringVarP(&name, "name", "n", "", "Update teammate name")
	cmd.Flags().StringVarP(&description, "desc", "d", "", "Update teammate description")
	cmd.Flags().StringVar(&llmModel, "llm", "", "Update LLM model")
	cmd.Flags().StringVar(&instructions, "instructions", "", "Update custom AI instructions")

	// Component flags - add
	cmd.Flags().StringArrayVar(&addSources, "add-source", []string{}, "Add source UUID (can be specified multiple times)")
	cmd.Flags().StringArrayVar(&addSecrets, "add-secret", []string{}, "Add secret name (can be specified multiple times)")
	cmd.Flags().StringArrayVar(&addEnvVars, "add-env", []string{}, "Add environment variable in KEY=VALUE format (can be specified multiple times)")
	cmd.Flags().StringArrayVar(&addIntegrations, "add-integration", []string{}, "Add integration (can be specified multiple times)")

	// Component flags - remove
	cmd.Flags().StringArrayVar(&removeSources, "remove-source", []string{}, "Remove source UUID (can be specified multiple times)")
	cmd.Flags().StringArrayVar(&removeSecrets, "remove-secret", []string{}, "Remove secret name (can be specified multiple times)")
	cmd.Flags().StringArrayVar(&removeEnvVars, "remove-env", []string{}, "Remove environment variable key (can be specified multiple times)")
	cmd.Flags().StringArrayVar(&removeIntegrations, "remove-integration", []string{}, "Remove integration (can be specified multiple times)")

	// Webhook flags
	cmd.Flags().StringArrayVar(&addWebhooks, "add-webhook", []string{}, "Add existing webhook by ID (can be specified multiple times)")
	cmd.Flags().StringArrayVar(&removeWebhooks, "remove-webhook", []string{}, "Remove webhook by ID (can be specified multiple times)")
	cmd.Flags().StringArrayVar(&webhookDestinations, "webhook-dest", []string{}, "Destination for new webhooks (depends on method type)")
	cmd.Flags().StringVar(&webhookMethod, "webhook-method", "http", "Webhook type (slack, teams, http) - determines destination format")
	cmd.Flags().StringVar(&webhookPrompt, "webhook-prompt", "", "Prompt for created webhooks")
	cmd.Flags().StringVar(&webhookFile, "webhook-file", "", "JSON or YAML file containing webhook definitions to create")

	return cmd
}

// Check if any command-line changes were specified
func hasCommandLineChanges(name, description, llmModel, instructions string,
	addSources, removeSources, addSecrets, removeSecrets,
	addEnvVars, removeEnvVars, addIntegrations, removeIntegrations []string) bool {

	if name != "" || description != "" || llmModel != "" || instructions != "" {
		return true
	}

	if len(addSources) > 0 || len(removeSources) > 0 {
		return true
	}

	if len(addSecrets) > 0 || len(removeSecrets) > 0 {
		return true
	}

	if len(addEnvVars) > 0 || len(removeEnvVars) > 0 {
		return true
	}

	if len(addIntegrations) > 0 || len(removeIntegrations) > 0 {
		return true
	}

	return false
}

// Check if any webhook-related changes were specified
func hasWebhookChanges(addWebhooks, removeWebhooks, webhookDestinations []string, webhookMethod, webhookPrompt string) bool {
	if len(addWebhooks) > 0 || len(removeWebhooks) > 0 {
		return true
	}

	if len(webhookDestinations) > 0 {
		return true
	}

	// For HTTP webhooks, only a prompt is required (no destination)
	if webhookMethod == "http" && webhookPrompt != "" {
		return true
	}

	// For other webhook types, both method and destination are needed
	if webhookMethod != "" && webhookMethod != "http" && webhookPrompt != "" {
		return true
	}

	return false
}

// Generate a human-readable diff between two teammates
func generateTeammateDiff(original, updated *kubiya.Teammate) []string {
	var changes []string

	// Compare basic fields
	if original.Name != updated.Name {
		changes = append(changes, fmt.Sprintf("Name: %s → %s",
			style.DimStyle.Render(original.Name),
			style.HighlightStyle.Render(updated.Name)))
	}

	if original.Description != updated.Description {
		changes = append(changes, fmt.Sprintf("Description updated"))
	}

	if original.LLMModel != updated.LLMModel {
		changes = append(changes, fmt.Sprintf("LLM Model: %s → %s",
			style.DimStyle.Render(original.LLMModel),
			style.HighlightStyle.Render(updated.LLMModel)))
	}

	if original.InstructionType != updated.InstructionType {
		changes = append(changes, fmt.Sprintf("Type: %s → %s",
			style.DimStyle.Render(original.InstructionType),
			style.HighlightStyle.Render(updated.InstructionType)))
	}

	if original.AIInstructions != updated.AIInstructions {
		changes = append(changes, "AI Instructions updated")
	}

	// Compare sources
	addedSources, removedSources := diffStringSlices(original.Sources, updated.Sources)
	if len(addedSources) > 0 {
		changes = append(changes, fmt.Sprintf("Added %d source(s)", len(addedSources)))
	}
	if len(removedSources) > 0 {
		changes = append(changes, fmt.Sprintf("Removed %d source(s)", len(removedSources)))
	}

	// Compare secrets
	addedSecrets, removedSecrets := diffStringSlices(original.Secrets, updated.Secrets)
	if len(addedSecrets) > 0 {
		changes = append(changes, fmt.Sprintf("Added %d secret(s)", len(addedSecrets)))
	}
	if len(removedSecrets) > 0 {
		changes = append(changes, fmt.Sprintf("Removed %d secret(s)", len(removedSecrets)))
	}

	// Compare env vars
	addedEnvVars, removedEnvVars, changedEnvVars := diffEnvVars(original.Environment, updated.Environment)
	if len(addedEnvVars) > 0 {
		changes = append(changes, fmt.Sprintf("Added %d environment variable(s)", len(addedEnvVars)))
	}
	if len(changedEnvVars) > 0 {
		changes = append(changes, fmt.Sprintf("Changed %d environment variable(s)", len(changedEnvVars)))
	}
	if len(removedEnvVars) > 0 {
		changes = append(changes, fmt.Sprintf("Removed %d environment variable(s)", len(removedEnvVars)))
	}

	// Compare integrations
	addedIntegrations, removedIntegrations := diffStringSlices(original.Integrations, updated.Integrations)
	if len(addedIntegrations) > 0 {
		changes = append(changes, fmt.Sprintf("Added %d integration(s)", len(addedIntegrations)))
	}
	if len(removedIntegrations) > 0 {
		changes = append(changes, fmt.Sprintf("Removed %d integration(s)", len(removedIntegrations)))
	}

	return changes
}

// Compare string slices and return added and removed items
func diffStringSlices(original, updated []string) (added, removed []string) {
	originalMap := make(map[string]bool)
	updatedMap := make(map[string]bool)

	for _, item := range original {
		originalMap[item] = true
	}

	for _, item := range updated {
		updatedMap[item] = true

		if !originalMap[item] {
			added = append(added, item)
		}
	}

	for _, item := range original {
		if !updatedMap[item] {
			removed = append(removed, item)
		}
	}

	return added, removed
}

// Compare environment variables and return added, removed, and changed items
func diffEnvVars(original, updated map[string]string) (added, removed, changed []string) {
	if original == nil {
		original = make(map[string]string)
	}

	if updated == nil {
		updated = make(map[string]string)
	}

	for key, updatedValue := range updated {
		if originalValue, exists := original[key]; exists {
			if originalValue != updatedValue {
				changed = append(changed, key)
			}
		} else {
			added = append(added, key)
		}
	}

	for key := range original {
		if _, exists := updated[key]; !exists {
			removed = append(removed, key)
		}
	}

	return added, removed, changed
}

func editTeammateJSON(teammate *kubiya.Teammate) (kubiya.Teammate, error) {
	// Create temporary file
	tmpfile, err := os.CreateTemp("", "kubiya-*.json")
	if err != nil {
		return kubiya.Teammate{}, fmt.Errorf("failed to create temp file: %w", err)
	}
	defer os.Remove(tmpfile.Name())

	// Write current teammate as JSON
	data, err := json.MarshalIndent(teammate, "", "  ")
	if err != nil {
		return kubiya.Teammate{}, fmt.Errorf("failed to marshal JSON: %w", err)
	}

	if _, err := tmpfile.Write(data); err != nil {
		return kubiya.Teammate{}, fmt.Errorf("failed to write temp file: %w", err)
	}
	tmpfile.Close()

	// Open in editor
	editor := os.Getenv("EDITOR")
	if editor == "" {
		editor = "vim"
	}

	cmd := exec.Command(editor, tmpfile.Name())
	cmd.Stdin = os.Stdin
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr

	if err := cmd.Run(); err != nil {
		return kubiya.Teammate{}, fmt.Errorf("editor failed: %w", err)
	}

	// Read updated content
	content, err := os.ReadFile(tmpfile.Name())
	if err != nil {
		return kubiya.Teammate{}, fmt.Errorf("failed to read updated file: %w", err)
	}

	var updated kubiya.Teammate
	if err := json.Unmarshal(content, &updated); err != nil {
		return kubiya.Teammate{}, fmt.Errorf("invalid JSON: %w", err)
	}

	return updated, nil
}

func newDeleteTeammateCommand(cfg *config.Config) *cobra.Command {
	var force bool

	cmd := &cobra.Command{
		Use:     "delete [uuid]",
		Short:   "🗑️ Delete teammate",
		Example: "  kubiya teammate delete abc-123\n  kubiya teammate delete abc-123 --force",
		Args:    cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			client := kubiya.NewClient(cfg)

			// Get teammate details first
			teammate, err := client.GetTeammate(cmd.Context(), args[0])
			if err != nil {
				return err
			}

			if !force {
				fmt.Printf("About to delete teammate:\n")
				fmt.Printf("  Name: %s\n", teammate.Name)
				fmt.Printf("  Description: %s\n", teammate.Description)
				fmt.Print("\nAre you sure? [y/N] ")
				var confirm string
				fmt.Scanln(&confirm)
				if strings.ToLower(confirm) != "y" {
					return fmt.Errorf("operation cancelled")
				}
			}

			if err := client.DeleteTeammate(cmd.Context(), args[0]); err != nil {
				return err
			}

			fmt.Printf("✅ Deleted teammate: %s\n", teammate.Name)
			return nil
		},
	}

	cmd.Flags().BoolVarP(&force, "force", "f", false, "Skip confirmation")
	return cmd
}

func newListTeammatesCommand(cfg *config.Config) *cobra.Command {
	var (
		outputFormat string
		showAll      bool
		sortBy       string
		filter       string
		limit        int
		showActive   bool
	)

	cmd := &cobra.Command{
		Use:   "list",
		Short: "📋 List all teammates",
		Example: `  # List teammates
  kubiya teammate list

  # Show all details including capabilities
  kubiya teammate list --all

  # Show only active teammates
  kubiya teammate list --active

  # Filter teammates (supports partial matching)
  kubiya teammate list --filter "kubernetes"

  # Sort by name, creation date, or last updated
  kubiya teammate list --sort name
  kubiya teammate list --sort created
  kubiya teammate list --sort updated

  # Output in JSON format
  kubiya teammate list --output json`,
		RunE: func(cmd *cobra.Command, args []string) error {
			client := kubiya.NewClient(cfg)
			teammates, err := client.ListTeammates(cmd.Context())
			if err != nil {
				return fmt.Errorf("failed to fetch teammates: %w", err)
			}

			// Get all sources to map IDs to names
			sources, err := client.ListSources(cmd.Context())
			if err != nil {
				return fmt.Errorf("failed to fetch source details: %w", err)
			}

			sourceMap := make(map[string]kubiya.Source)
			for _, s := range sources {
				sourceMap[s.UUID] = s
			}

			// Filter teammates if requested
			if filter != "" {
				filterLower := strings.ToLower(filter)
				var filtered []kubiya.Teammate
				for _, t := range teammates {
					// Match against name, description, type, etc.
					if strings.Contains(strings.ToLower(t.Name), filterLower) ||
						strings.Contains(strings.ToLower(t.Description), filterLower) ||
						strings.Contains(strings.ToLower(t.InstructionType), filterLower) ||
						strings.Contains(strings.ToLower(t.LLMModel), filterLower) {
						filtered = append(filtered, t)
						continue
					}

					// Match against sources
					for _, sourceID := range t.Sources {
						if source, ok := sourceMap[sourceID]; ok {
							if strings.Contains(strings.ToLower(source.Name), filterLower) {
								filtered = append(filtered, t)
								break
							}
						}
					}

					// Match against integrations
					for _, integration := range t.Integrations {
						if strings.Contains(strings.ToLower(integration), filterLower) {
							filtered = append(filtered, t)
							break
						}
					}
				}
				teammates = filtered
			}

			// Filter active teammates if requested
			if showActive {
				var active []kubiya.Teammate
				for _, t := range teammates {
					status := getTeammateStatus(t)
					if !strings.Contains(status, "inactive") {
						active = append(active, t)
					}
				}
				teammates = active
			}

			// Sort teammates if requested
			switch strings.ToLower(sortBy) {
			case "name":
				sort.Slice(teammates, func(i, j int) bool {
					return teammates[i].Name < teammates[j].Name
				})
			case "created":
				sort.Slice(teammates, func(i, j int) bool {
					return teammates[i].Metadata.CreatedAt > teammates[j].Metadata.CreatedAt
				})
			case "updated":
				sort.Slice(teammates, func(i, j int) bool {
					return teammates[i].Metadata.LastUpdated > teammates[j].Metadata.LastUpdated
				})
			}

			// Limit results if requested
			if limit > 0 && limit < len(teammates) {
				teammates = teammates[:limit]
			}

			switch outputFormat {
			case "json":
				return json.NewEncoder(os.Stdout).Encode(teammates)
			case "text":
				// Count active teammates
				activeCount := 0
				for _, t := range teammates {
					if !strings.Contains(getTeammateStatus(t), "inactive") {
						activeCount++
					}
				}

				// Create a tabwriter for aligned output
				w := tabwriter.NewWriter(os.Stdout, 0, 0, 2, ' ', 0)

				// Show title with counts
				fmt.Fprintln(w, style.TitleStyle.Render(fmt.Sprintf(" 👥 Teammates (%d total, %d active) ", len(teammates), activeCount)))

				// Change the header based on display mode
				if showAll {
					fmt.Fprintln(w, "UUID\tNAME\tTYPE\tSTATUS\tMODEL\tSOURCES\tRUNNERS\tINTEGRATIONS\tDESCRIPTION")
				} else {
					fmt.Fprintln(w, "UUID\tNAME\tTYPE\tSTATUS\tDESCRIPTION")
				}

				// Add debug output if debug mode is enabled
				for _, t := range teammates {
					if cfg.Debug {
						debugTeammate(t)
					}

					// Basic info
					uuid := style.DimStyle.Render(t.UUID)
					name := style.HighlightStyle.Render(t.Name)
					typeIcon := getTeammateTypeIcon(t.InstructionType)
					status := getTeammateStatus(t)
					description := truncateDescription(t.Description, 50)

					// Extended info for "all" mode
					if showAll {
						// LLM model
						model := t.LLMModel
						if model == "" {
							model = style.DimStyle.Render("default")
						}

						// Sources
						sourcesList := ""
						if len(t.Sources) > 0 {
							var sourceNames []string
							for _, sourceID := range t.Sources {
								if source, ok := sourceMap[sourceID]; ok && source.Name != "" {
									sourceNames = append(sourceNames, source.Name)
								} else {
									sourceNames = append(sourceNames, fmt.Sprintf("ID:%s", sourceID))
								}
							}
							if len(sourceNames) > 2 {
								sourcesList = fmt.Sprintf("%s +%d more",
									strings.Join(sourceNames[:2], ", "), len(sourceNames)-2)
							} else {
								sourcesList = strings.Join(sourceNames, ", ")
							}
						} else {
							sourcesList = style.DimStyle.Render("none")
						}

						// Runners
						runnersList := ""
						if len(t.Runners) > 0 {
							if len(t.Runners) > 2 {
								runnersList = fmt.Sprintf("%s +%d more",
									strings.Join(t.Runners[:2], ", "), len(t.Runners)-2)
							} else {
								runnersList = strings.Join(t.Runners, ", ")
							}
						} else {
							runnersList = style.DimStyle.Render("none")
						}

						// Integrations
						integrationsList := ""
						if len(t.Integrations) > 0 {
							if len(t.Integrations) > 2 {
								integrationsList = fmt.Sprintf("%s +%d more",
									strings.Join(t.Integrations[:2], ", "), len(t.Integrations)-2)
							} else {
								integrationsList = strings.Join(t.Integrations, ", ")
							}
						} else {
							integrationsList = style.DimStyle.Render("none")
						}

						// Print row with extended info
						fmt.Fprintf(w, "%s\t%s\t%s\t%s\t%s\t%s\t%s\t%s\t%s\n",
							uuid, name, typeIcon, status, model, sourcesList, runnersList, integrationsList, description)
					} else {
						// Print row with basic info
						fmt.Fprintf(w, "%s\t%s\t%s\t%s\t%s\n",
							uuid, name, typeIcon, status, description)
					}
				}

				// Add footer with helpful tips
				err := w.Flush()
				if err != nil {
					return err
				}

				if len(teammates) == 0 {
					fmt.Println("\nNo teammates found. Create one with: kubiya teammate create --interactive")
				} else {
					fmt.Printf("\n%s\n", style.SubtitleStyle.Render("Helpful Commands"))
					fmt.Printf("• Create a new teammate: %s\n",
						style.CommandStyle.Render("kubiya teammate create --interactive"))
					fmt.Printf("• View teammate details: %s\n",
						style.CommandStyle.Render("kubiya teammate get <uuid>"))
					fmt.Printf("• Show detailed listing: %s\n",
						style.CommandStyle.Render("kubiya teammate list --all"))

					if !showActive && activeCount > 0 {
						fmt.Printf("• Show only active teammates: %s\n",
							style.CommandStyle.Render("kubiya teammate list --active"))
					}
				}

				return nil
			default:
				return fmt.Errorf("unknown output format: %s", outputFormat)
			}
		},
	}

	cmd.Flags().StringVarP(&outputFormat, "output", "o", "text", "Output format (text|json)")
	cmd.Flags().BoolVarP(&showAll, "all", "a", false, "Show detailed information for all teammates")
	cmd.Flags().BoolVar(&showActive, "active", false, "Show only active teammates")
	cmd.Flags().StringVarP(&sortBy, "sort", "s", "", "Sort by field (name|created|updated)")
	cmd.Flags().StringVarP(&filter, "filter", "f", "", "Filter teammates by name, description, or type")
	cmd.Flags().IntVarP(&limit, "limit", "l", 0, "Limit number of results")

	return cmd
}

// Helper functions to improve the display

func getTeammateTypeIcon(instructionType string) string {
	switch strings.ToLower(instructionType) {
	case "tools":
		return "🛠️ Tools"
	case "chat":
		return "💬 Chat"
	case "workflow":
		return "📋 Workflow"
	default:
		return instructionType
	}
}

func getTeammateStatus(t kubiya.Teammate) string {
	// Force all teammates to be active regardless of properties
	return style.ActiveStyle.Render("active")
}

func getTeammateCapabilities(t kubiya.Teammate, sourceMap map[string]kubiya.Source) string {
	var capabilities []string

	// Debug output if needed
	// fmt.Printf("Debug - Teammate %s:\nSources: %v\nTools: %v\nEnv: %v\nSecrets: %v\nIntegrations: %v\n",
	//     t.Name, t.Sources, t.Tools, t.Environment, t.Secrets, t.Integrations)

	// Check direct tools
	if len(t.Tools) > 0 {
		capabilities = append(capabilities, fmt.Sprintf("🛠️ %d tools", len(t.Tools)))
	}

	// Check sources and their tools
	var sourceTools int
	var sourceNames []string
	for _, sourceID := range t.Sources {
		if source, ok := sourceMap[sourceID]; ok {
			sourceTools += source.ConnectedToolsCount
			if source.Name != "" {
				sourceNames = append(sourceNames, source.Name)
			}
		}
	}
	if sourceTools > 0 {
		capabilities = append(capabilities, fmt.Sprintf("📦 %d source tools", sourceTools))
	}
	if len(sourceNames) > 0 {
		capabilities = append(capabilities, fmt.Sprintf("Sources: %s", strings.Join(sourceNames, ", ")))
	}

	// Check integrations with icons
	if len(t.Integrations) > 0 {
		var integrationIcons []string
		for _, integration := range t.Integrations {
			switch strings.ToLower(integration) {
			case "github", "github_admin":
				integrationIcons = append(integrationIcons, "GitHub 🐙")
			case "aws", "aws_admin":
				integrationIcons = append(integrationIcons, "AWS ☁️")
			case "kubernetes", "k8s":
				integrationIcons = append(integrationIcons, "K8s ⎈")
			case "slack":
				integrationIcons = append(integrationIcons, "Slack 💬")
			case "databricks":
				integrationIcons = append(integrationIcons, "Databricks 📊")
			case "terraform":
				integrationIcons = append(integrationIcons, "Terraform 🏗️")
			default:
				integrationIcons = append(integrationIcons, integration)
			}
		}
		if len(integrationIcons) > 0 {
			capabilities = append(capabilities, fmt.Sprintf("🔌 %s", strings.Join(integrationIcons, ", ")))
		}
	}

	// Check environment variables
	if len(t.Environment) > 0 {
		capabilities = append(capabilities, fmt.Sprintf("🔧 %d env vars", len(t.Environment)))
	}

	// Check secrets
	if len(t.Secrets) > 0 {
		capabilities = append(capabilities, fmt.Sprintf("🔒 %d secrets", len(t.Secrets)))
	}

	// Check runners
	if len(t.Runners) > 0 {
		capabilities = append(capabilities, fmt.Sprintf("🏃 %d runners", len(t.Runners)))
	}

	// Check allowed groups
	if len(t.AllowedGroups) > 0 {
		capabilities = append(capabilities, fmt.Sprintf("👥 %d groups", len(t.AllowedGroups)))
	}

	// Check LLM model
	if t.LLMModel != "" {
		switch {
		case strings.Contains(strings.ToLower(t.LLMModel), "gpt-4"):
			capabilities = append(capabilities, "🧠 GPT-4")
		case strings.Contains(strings.ToLower(t.LLMModel), "gpt-3"):
			capabilities = append(capabilities, "🤖 GPT-3")
		default:
			capabilities = append(capabilities, "🤖 "+t.LLMModel)
		}
	}

	if len(capabilities) == 0 {
		return style.DimStyle.Render("no active capabilities")
	}

	return strings.Join(capabilities, " | ")
}

func summarizeIntegrations(integrations []string) string {
	if len(integrations) == 0 {
		return style.DimStyle.Render("none")
	}

	// Map common integrations to icons
	integrationIcons := map[string]string{
		"github":     "🐙",
		"gitlab":     "🦊",
		"aws":        "☁️",
		"azure":      "☁️",
		"gcp":        "☁️",
		"kubernetes": "⎈",
		"slack":      "💬",
		"jira":       "📋",
		"jenkins":    "🔧",
	}

	var icons []string
	for _, integration := range integrations {
		lowered := strings.ToLower(integration)
		for key, icon := range integrationIcons {
			if strings.Contains(lowered, key) {
				icons = append(icons, icon)
				break
			}
		}
	}

	if len(icons) == 0 {
		return fmt.Sprintf("%d custom", len(integrations))
	}
	return strings.Join(icons, " ")
}

func truncateDescription(desc string, maxLen int) string {
	if len(desc) <= maxLen {
		return desc
	}
	return desc[:maxLen-3] + "..."
}

func newGetTeammateCommand(cfg *config.Config) *cobra.Command {
	var outputFormat string

	cmd := &cobra.Command{
		Use:     "get [uuid]",
		Short:   "🔍 Get teammate details",
		Example: "  kubiya teammate get abc-123\n  kubiya teammate get abc-123 --output json",
		Args:    cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			client := kubiya.NewClient(cfg)
			teammate, err := client.GetTeammate(cmd.Context(), args[0])
			if err != nil {
				return fmt.Errorf("failed to get teammate: %w", err)
			}

			switch outputFormat {
			case "json":
				return json.NewEncoder(os.Stdout).Encode(teammate)

			case "text":
				fmt.Printf("\n%s\n\n", style.TitleStyle.Render(fmt.Sprintf(" 👤 Teammate: %s ", teammate.Name)))

				fmt.Printf("%s\n", style.SubtitleStyle.Render("Basic Information"))
				fmt.Printf("  UUID: %s\n", style.DimStyle.Render(teammate.UUID))
				fmt.Printf("  Name: %s\n", style.HighlightStyle.Render(teammate.Name))
				fmt.Printf("  Description: %s\n", teammate.Description)
				fmt.Printf("  Type: %s\n", getTeammateTypeIcon(teammate.InstructionType))
				fmt.Printf("  LLM Model: %s\n", getModelWithIcon(teammate.LLMModel))
				if teammate.Image != "" {
					fmt.Printf("  Image: %s\n", teammate.Image)
				}
				if teammate.ManagedBy != "" {
					fmt.Printf("  Managed By: %s\n", teammate.ManagedBy)
				}
				if teammate.IsDebugMode {
					fmt.Printf("  Debug Mode: %s\n", style.WarningStyle.Render("Enabled"))
				}
				fmt.Println()

				// Get sources with details
				if len(teammate.Sources) > 0 {
					fmt.Printf("%s\n", style.SubtitleStyle.Render("Sources"))

					// Get all sources to map IDs to names
					sources, err := client.ListSources(cmd.Context())
					if err != nil {
						return fmt.Errorf("failed to fetch source details: %w", err)
					}

					sourceMap := make(map[string]kubiya.Source)
					for _, s := range sources {
						sourceMap[s.UUID] = s
					}

					for _, sourceID := range teammate.Sources {
						if source, ok := sourceMap[sourceID]; ok {
							fmt.Printf("  • %s\n", style.HighlightStyle.Render(source.Name))
							fmt.Printf("    UUID: %s\n", style.DimStyle.Render(source.UUID))
							if source.URL != "" {
								fmt.Printf("    URL: %s\n", style.DimStyle.Render(source.URL))
							}
							if source.Description != "" {
								fmt.Printf("    %s\n", source.Description)
							}

							// Show source metrics
							total := len(source.Tools) + len(source.InlineTools)
							if total > 0 {
								fmt.Printf("    Total Tools: %d (%d regular, %d inline)\n",
									total, len(source.Tools), len(source.InlineTools))
							}
						} else {
							fmt.Printf("  • %s (ID: %s)\n", style.DimStyle.Render("Unknown source"), sourceID)
						}
						fmt.Println()
					}
				}

				// Get all direct tools
				if len(teammate.Tools) > 0 {
					fmt.Printf("%s\n", style.SubtitleStyle.Render("Direct Tools"))
					for _, toolID := range teammate.Tools {
						fmt.Printf("  • %s\n", style.HighlightStyle.Render(toolID))
						// Try to fetch tool details if possible
						// This would depend on client.GetTool implementation
						// tool, err := client.GetTool(cmd.Context(), toolID)
						fmt.Println()
					}
				}

				// Environment variables with improved display
				if len(teammate.Environment) > 0 {
					fmt.Printf("%s\n", style.SubtitleStyle.Render("Environment Variables"))
					w := tabwriter.NewWriter(os.Stdout, 0, 0, 2, ' ', 0)
					fmt.Fprintln(w, "  KEY\tVALUE")
					for k, v := range teammate.Environment {
						maskedValue := v
						// Mask sensitive values
						if strings.Contains(strings.ToLower(k), "password") ||
							strings.Contains(strings.ToLower(k), "token") ||
							strings.Contains(strings.ToLower(k), "secret") ||
							strings.Contains(strings.ToLower(k), "key") {
							maskedValue = "••••••••••••••"
						}

						fmt.Fprintf(w, "  %s\t%s\n",
							style.HighlightStyle.Render(k),
							maskedValue)
					}
					w.Flush()
					fmt.Println()
				}

				// Secrets with descriptions
				if len(teammate.Secrets) > 0 {
					fmt.Printf("%s\n", style.SubtitleStyle.Render("Secrets"))
					for _, secret := range teammate.Secrets {
						// Try to get secret details
						secretInfo, err := client.GetSecret(cmd.Context(), secret)
						if err == nil && secretInfo.Description != "" {
							fmt.Printf("  • %s: %s\n",
								style.HighlightStyle.Render(secret),
								style.DimStyle.Render(secretInfo.Description))
						} else {
							fmt.Printf("  • %s\n", style.HighlightStyle.Render(secret))
						}
					}
					fmt.Println()
				}

				// Integrations with icons
				if len(teammate.Integrations) > 0 {
					fmt.Printf("%s\n", style.SubtitleStyle.Render("Integrations"))
					for _, integration := range teammate.Integrations {
						icon := getIntegrationIcon(integration)
						fmt.Printf("  • %s %s\n", icon, integration)
					}
					fmt.Println()
				}

				// Access control
				if len(teammate.AllowedGroups) > 0 || len(teammate.AllowedUsers) > 0 || len(teammate.Owners) > 0 {
					fmt.Printf("%s\n", style.SubtitleStyle.Render("Access Control"))

					if len(teammate.Owners) > 0 {
						fmt.Printf("  Owners: %s\n", strings.Join(teammate.Owners, ", "))
					}

					if len(teammate.AllowedGroups) > 0 {
						fmt.Printf("  Allowed Groups: %s\n", strings.Join(teammate.AllowedGroups, ", "))
					}

					if len(teammate.AllowedUsers) > 0 {
						fmt.Printf("  Allowed Users: %s\n", strings.Join(teammate.AllowedUsers, ", "))
					}

					fmt.Println()
				}

				// Runners
				if len(teammate.Runners) > 0 {
					fmt.Printf("%s\n", style.SubtitleStyle.Render("Runners"))
					for _, runner := range teammate.Runners {
						fmt.Printf("  • %s\n", runner)
					}
					fmt.Println()
				}

				// Timestamps
				if teammate.Metadata.CreatedAt != "" || teammate.Metadata.LastUpdated != "" {
					fmt.Printf("%s\n", style.SubtitleStyle.Render("Timestamps"))
					if teammate.Metadata.CreatedAt != "" {
						fmt.Printf("  Created: %s\n", teammate.Metadata.CreatedAt)
					}
					if teammate.Metadata.LastUpdated != "" {
						fmt.Printf("  Updated: %s\n", teammate.Metadata.LastUpdated)
					}
					fmt.Println()
				}

				// AI Instructions (if available and not empty)
				if teammate.AIInstructions != "" {
					fmt.Printf("%s\n", style.SubtitleStyle.Render("AI Instructions"))
					fmt.Printf("  %s\n\n", teammate.AIInstructions)
				}

				// Helpful commands
				fmt.Printf("%s\n", style.SubtitleStyle.Render("Helpful Commands"))
				fmt.Printf("  • Edit: %s\n",
					style.CommandStyle.Render(fmt.Sprintf("kubiya teammate edit %s --interactive", teammate.UUID)))
				fmt.Printf("  • Delete: %s\n",
					style.CommandStyle.Render(fmt.Sprintf("kubiya teammate delete %s", teammate.UUID)))
				fmt.Println()

				return nil

			default:
				return fmt.Errorf("unknown output format: %s", outputFormat)
			}
		},
	}

	cmd.Flags().StringVarP(&outputFormat, "output", "o", "text", "Output format (text|json)")
	return cmd
}

// getModelWithIcon returns the LLM model with an appropriate icon
func getModelWithIcon(model string) string {
	modelLower := strings.ToLower(model)

	switch {
	case strings.Contains(modelLower, "gpt-4"):
		return fmt.Sprintf("🧠 %s", model)
	case strings.Contains(modelLower, "claude"):
		return fmt.Sprintf("🦚 %s", model)
	case strings.Contains(modelLower, "llama"):
		return fmt.Sprintf("🦙 %s", model)
	default:
		return fmt.Sprintf("🤖 %s", model)
	}
}

// getIntegrationIcon returns an appropriate icon for the integration
func getIntegrationIcon(integration string) string {
	integrationLower := strings.ToLower(integration)

	switch {
	case strings.Contains(integrationLower, "github"):
		return "🐙"
	case strings.Contains(integrationLower, "gitlab"):
		return "🦊"
	case strings.Contains(integrationLower, "aws"):
		return "☁️"
	case strings.Contains(integrationLower, "azure"):
		return "☁️"
	case strings.Contains(integrationLower, "gcp"):
		return "☁️"
	case strings.Contains(integrationLower, "kubernetes"), strings.Contains(integrationLower, "k8s"):
		return "⎈"
	case strings.Contains(integrationLower, "slack"):
		return "💬"
	case strings.Contains(integrationLower, "jira"):
		return "📋"
	case strings.Contains(integrationLower, "jenkins"):
		return "🔧"
	case strings.Contains(integrationLower, "docker"):
		return "🐳"
	case strings.Contains(integrationLower, "terraform"):
		return "🏗️"
	default:
		return "🔌"
	}
}

// Add this debug function
func debugTeammate(t kubiya.Teammate) {
	fmt.Printf("\nDebug - Teammate: %s\n", t.Name)
	fmt.Printf("Sources: %v\n", t.Sources)
	fmt.Printf("Tools: %v\n", t.Tools)
	fmt.Printf("Environment: %d vars\n", len(t.Environment))
	fmt.Printf("Secrets: %v\n", t.Secrets)
	fmt.Printf("Integrations: %v\n", t.Integrations)
	fmt.Printf("LLM Model: %s\n", t.LLMModel)
	fmt.Printf("Instruction Type: %s\n", t.InstructionType)
	fmt.Printf("Runners: %v\n", t.Runners)
	fmt.Printf("AllowedGroups: %v\n", t.AllowedGroups)
	fmt.Printf("--------------------\n")
}

// confirmYesNo asks the user for confirmation
func confirmYesNo(prompt string) bool {
	fmt.Printf("%s [y/N] ", prompt)
	var confirm string
	fmt.Scanln(&confirm)
	return strings.ToLower(confirm) == "y"
}

// captureCommandOutput runs a command and returns its output
func captureCommandOutput(command string, args ...string) (string, error) {
	cmd := exec.Command(command, args...)
	output, err := cmd.CombinedOutput()
	if err != nil {
		return "", fmt.Errorf("command failed: %s, error: %w", string(output), err)
	}
	return string(output), nil
}

// extractUUIDFromSourceOutput extracts the UUID from the source creation output
func extractUUIDFromSourceOutput(output string) string {
	// Look for lines containing "UUID: <uuid>"
	lines := strings.Split(output, "\n")
	for _, line := range lines {
		if strings.Contains(line, "UUID:") {
			parts := strings.SplitN(line, "UUID:", 2)
			if len(parts) == 2 {
				return strings.TrimSpace(parts[1])
			}
		}
	}
	return ""
}
