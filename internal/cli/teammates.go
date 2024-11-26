package cli

import (
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"strings"
	"text/tabwriter"

	"github.com/kubiyabot/cli/internal/config"
	"github.com/kubiyabot/cli/internal/kubiya"
	"github.com/kubiyabot/cli/internal/style"
	"github.com/kubiyabot/cli/internal/tui"
	"github.com/spf13/cobra"
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
		jsonFile        string
	)

	cmd := &cobra.Command{
		Use:   "create",
		Short: "➕ Create new teammate",
		Example: `  # Create interactively
  kubiya teammate create --interactive

  # Create from JSON file
  kubiya teammate create --file teammate.json

  # Create with basic parameters
  kubiya teammate create --name "DevOps Bot" --description "Handles DevOps tasks"`,
		RunE: func(cmd *cobra.Command, args []string) error {
			client := kubiya.NewClient(cfg)

			var teammate kubiya.Teammate

			if jsonFile != "" {
				// Read from JSON file
				data, err := os.ReadFile(jsonFile)
				if err != nil {
					return fmt.Errorf("failed to read file: %w", err)
				}
				if err := json.Unmarshal(data, &teammate); err != nil {
					return fmt.Errorf("invalid JSON: %w", err)
				}
			} else if interactive {
				// Use TUI form
				form := tui.NewTeammateForm(cfg)
				result, err := form.Run()
				if err != nil {
					return err
				}
				teammate = *result
			} else {
				// Use command line parameters
				if name == "" {
					return fmt.Errorf("name is required when not using --interactive or --file")
				}
				teammate = kubiya.Teammate{
					Name:            name,
					Description:     description,
					LLMModel:        llmModel,
					InstructionType: instructionType,
				}
			}

			created, err := client.CreateTeammate(cmd.Context(), teammate)
			if err != nil {
				return err
			}

			fmt.Printf("✅ Created teammate: %s (UUID: %s)\n", created.Name, created.UUID)
			return nil
		},
	}

	cmd.Flags().StringVarP(&name, "name", "n", "", "Teammate name")
	cmd.Flags().StringVarP(&description, "desc", "d", "", "Teammate description")
	cmd.Flags().BoolVarP(&interactive, "interactive", "i", false, "Use interactive form")
	cmd.Flags().StringVar(&llmModel, "llm", "azure/gpt-4", "LLM model to use")
	cmd.Flags().StringVar(&instructionType, "type", "tools", "Instruction type")
	cmd.Flags().StringVarP(&jsonFile, "file", "f", "", "JSON file containing teammate configuration")

	return cmd
}

func newEditTeammateCommand(cfg *config.Config) *cobra.Command {
	var (
		interactive bool
		editor      bool
	)

	cmd := &cobra.Command{
		Use:   "edit [uuid]",
		Short: "✏️ Edit teammate",
		Example: `  # Edit interactively with form
  kubiya teammate edit abc-123 --interactive

  # Edit using JSON editor
  kubiya teammate edit abc-123 --editor`,
		Args: cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			client := kubiya.NewClient(cfg)

			// Get existing teammate
			teammate, err := client.GetTeammate(cmd.Context(), args[0])
			if err != nil {
				return err
			}

			var updated kubiya.Teammate

			if interactive {
				// Use TUI form
				form := tui.NewTeammateForm(cfg)
				form.SetDefaults(teammate)
				result, err := form.Run()
				if err != nil {
					return err
				}
				updated = *result
			} else if editor {
				// Use JSON editor
				updated, err = editTeammateJSON(teammate)
				if err != nil {
					return err
				}
			} else {
				return fmt.Errorf("must specify either --interactive or --editor")
			}

			// Update the teammate
			result, err := client.UpdateTeammate(cmd.Context(), args[0], updated)
			if err != nil {
				return err
			}

			fmt.Printf("✅ Updated teammate: %s\n", result.Name)
			return nil
		},
	}

	cmd.Flags().BoolVarP(&interactive, "interactive", "i", false, "Use interactive form")
	cmd.Flags().BoolVarP(&editor, "editor", "e", false, "Use JSON editor")

	return cmd
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
	var outputFormat string

	cmd := &cobra.Command{
		Use:   "list",
		Short: "📋 List all teammates",
		Example: `  # List teammates
  kubiya teammate list

  # Output in JSON format
  kubiya teammate list --output json`,
		RunE: func(cmd *cobra.Command, args []string) error {
			client := kubiya.NewClient(cfg)
			teammates, err := client.ListTeammates(cmd.Context())
			if err != nil {
				return err
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

			switch outputFormat {
			case "json":
				return json.NewEncoder(os.Stdout).Encode(teammates)
			case "text":
				w := tabwriter.NewWriter(os.Stdout, 0, 0, 2, ' ', 0)
				fmt.Fprintln(w, style.TitleStyle.Render(" 👥 Teammates "))
				fmt.Fprintln(w, "UUID\tNAME\tTYPE\tSTATUS\tDESCRIPTION")

				for _, t := range teammates {
					// Add debug output if debug mode is enabled
					if cfg.Debug {
						debugTeammate(t)
					}

					fmt.Fprintf(w, "%s\t%s\t%s\t%s\t%s\n",
						style.DimStyle.Render(t.UUID),
						style.HighlightStyle.Render(t.Name),
						getTeammateTypeIcon(t.InstructionType),
						getTeammateStatus(t),
						truncateDescription(t.Description, 50),
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
	var indicators []string

	// Check LLM model
	if t.LLMModel != "" {
		if strings.Contains(strings.ToLower(t.LLMModel), "gpt-4") {
			indicators = append(indicators, "🧠")
		} else {
			indicators = append(indicators, "🤖")
		}
	}

	// Add source/tools indicators
	if len(t.Sources) > 0 || len(t.Tools) > 0 {
		indicators = append(indicators, "🛠️")
	}

	// Add integration indicator
	if len(t.Integrations) > 0 {
		indicators = append(indicators, "🔌")
	}

	// Add env/secrets indicator
	if len(t.Environment) > 0 || len(t.Secrets) > 0 {
		indicators = append(indicators, "🔧")
	}

	if len(indicators) == 0 {
		return style.DimStyle.Render("inactive")
	}

	return strings.Join(indicators, " ")
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
				return err
			}

			switch outputFormat {
			case "json":
				return json.NewEncoder(os.Stdout).Encode(teammate)
			case "text":
				fmt.Printf("\n%s\n\n", style.TitleStyle.Render(fmt.Sprintf(" 👤 Teammate: %s ", teammate.Name)))

				fmt.Printf("%s\n", style.SubtitleStyle.Render("Basic Information"))
				fmt.Printf("  UUID: %s\n", teammate.UUID)
				fmt.Printf("  Description: %s\n", teammate.Description)
				fmt.Printf("  Type: %s\n", teammate.InstructionType)
				fmt.Printf("  LLM Model: %s\n", teammate.LLMModel)
				fmt.Println()

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
							if source.URL != "" {
								fmt.Printf("    URL: %s\n", style.DimStyle.Render(source.URL))
							}
							if source.Description != "" {
								fmt.Printf("    %s\n", source.Description)
							}
						} else {
							fmt.Printf("  • %s (ID: %s)\n", style.DimStyle.Render("Unknown source"), sourceID)
						}
						fmt.Println()
					}
				}

				if len(teammate.Environment) > 0 {
					fmt.Printf("%s\n", style.SubtitleStyle.Render("Environment Variables"))
					w := tabwriter.NewWriter(os.Stdout, 0, 0, 2, ' ', 0)
					for k, v := range teammate.Environment {
						fmt.Fprintf(w, "  • %s:\t%s\n",
							style.HighlightStyle.Render(k),
							v)
					}
					w.Flush()
					fmt.Println()
				}

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

				if len(teammate.Integrations) > 0 {
					fmt.Printf("%s\n", style.SubtitleStyle.Render("Integrations"))
					for _, integration := range teammate.Integrations {
						fmt.Printf("  • %s\n", integration)
					}
					fmt.Println()
				}

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

				return nil
			default:
				return fmt.Errorf("unknown output format: %s", outputFormat)
			}
		},
	}

	cmd.Flags().StringVarP(&outputFormat, "output", "o", "text", "Output format (text|json)")
	return cmd
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
