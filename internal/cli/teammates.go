package cli

import (
	"encoding/json"
	"fmt"
	"io"
	"os"
	"os/exec"
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
		name            string
		description     string
		interactive     bool
		llmModel        string
		instructionType string
		inputFile       string
		inputFormat     string
		fromStdin       bool
		sources         []string
		secrets         []string
		integrations    []string
		envVars         []string
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
    --integration github`,
		RunE: func(cmd *cobra.Command, args []string) error {
			client := kubiya.NewClient(cfg)

			var teammate kubiya.Teammate
			var err error

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

			// Validate the teammate
			if err := validateTeammate(&teammate); err != nil {
				return err
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
			fmt.Println()

			// Send to API
			created, err := client.CreateTeammate(cmd.Context(), teammate)
			if err != nil {
				return fmt.Errorf("failed to create teammate: %w", err)
			}

			fmt.Printf("%s Created teammate: %s (UUID: %s)\n\n",
				style.SuccessStyle.Render("✅"),
				style.HighlightStyle.Render(created.Name),
				style.DimStyle.Render(created.UUID))

			// Helpful next steps
			fmt.Printf("%s\n", style.SubtitleStyle.Render("Next Steps"))
			fmt.Printf("• View details: %s\n",
				style.CommandStyle.Render(fmt.Sprintf("kubiya teammate get %s", created.UUID)))
			fmt.Printf("• Edit teammate: %s\n",
				style.CommandStyle.Render(fmt.Sprintf("kubiya teammate edit %s --interactive", created.UUID)))

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

	return cmd
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
func validateTeammate(teammate *kubiya.Teammate) error {
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

	return nil
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
  kubiya teammate edit abc-123 --add-env "DEBUG=true" --remove-integration github`,
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
				addEnvVars, removeEnvVars, addIntegrations, removeIntegrations) {

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

			if len(diff) == 0 {
				fmt.Println("No changes detected. Update cancelled.")
				return nil
			}

			fmt.Println("Changes:")
			for _, change := range diff {
				fmt.Printf("  • %s\n", change)
			}
			fmt.Println()

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
