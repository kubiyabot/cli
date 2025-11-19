package cli

import (
	"encoding/json"
	"fmt"
	"os"
	"text/tabwriter"

	"github.com/kubiyabot/cli/internal/config"
	"github.com/kubiyabot/cli/internal/controlplane"
	"github.com/kubiyabot/cli/internal/controlplane/entities"
	"github.com/kubiyabot/cli/internal/style"
	"github.com/spf13/cobra"
	"gopkg.in/yaml.v3"
)

func newSkillCommand(cfg *config.Config) *cobra.Command {
	cmd := &cobra.Command{
		Use:     "skill",
		Aliases: []string{"skills", "sk"},
		Short:   "🛠️  Manage skills (toolsets)",
		Long:    `Create, list, update, and delete skills in the control plane. Skills replace sources/tools from V1.`,
	}

	cmd.AddCommand(
		newCreateSkillCommand(cfg),
		newListSkillsCommand(cfg),
		newGetSkillCommand(cfg),
		newUpdateSkillCommand(cfg),
		newDeleteSkillCommand(cfg),
		newListSkillDefinitionsCommand(cfg),
		newAssociateSkillCommand(cfg),
		newListSkillAssociationsCommand(cfg),
	)

	return cmd
}

func newCreateSkillCommand(cfg *config.Config) *cobra.Command {
	var (
		name        string
		description string
		skillType   string
		variant     string
		enabled     bool
		configFile  string
		configJSON  string
		outputFormat string
	)

	cmd := &cobra.Command{
		Use:   "create",
		Short: "Create a new skill",
		Example: `  # Create a skill
  kubiya skill create --name "GitHub Tools" --type github --enabled

  # Create with configuration from file
  kubiya skill create --name "Slack Bot" --type slack --config config.json

  # Create with inline JSON config
  kubiya skill create --name "AWS CLI" --type aws --config-json '{"region":"us-east-1"}'`,
		RunE: func(cmd *cobra.Command, args []string) error {
			if name == "" {
				return fmt.Errorf("--name is required")
			}
			if skillType == "" {
				return fmt.Errorf("--type is required")
			}

			client, err := controlplane.New(cfg.APIKey, cfg.Debug)
			if err != nil {
				return fmt.Errorf("failed to create client: %w", err)
			}

			var configuration map[string]interface{}
			if configFile != "" {
				data, err := os.ReadFile(configFile)
				if err != nil {
					return fmt.Errorf("failed to read config file: %w", err)
				}
				if err := json.Unmarshal(data, &configuration); err != nil {
					return fmt.Errorf("failed to parse config file: %w", err)
				}
			} else if configJSON != "" {
				if err := json.Unmarshal([]byte(configJSON), &configuration); err != nil {
					return fmt.Errorf("failed to parse config JSON: %w", err)
				}
			} else {
				configuration = make(map[string]interface{})
			}

			req := &entities.SkillCreateRequest{
				Name:          name,
				SkillType:     skillType,
				Configuration: configuration,
				Enabled:       &enabled,
			}

			if description != "" {
				req.Description = &description
			}
			if variant != "" {
				req.Variant = &variant
			}

			skill, err := client.CreateSkill(req)
			if err != nil {
				return fmt.Errorf("failed to create skill: %w", err)
			}

			fmt.Printf("%s Skill %s created (ID: %s)\n",
				style.SuccessStyle.Render("✓"),
				style.HighlightStyle.Render(skill.Name),
				skill.ID)

			if outputFormat == "json" {
				data, _ := json.MarshalIndent(skill, "", "  ")
				fmt.Println(string(data))
			} else if outputFormat == "yaml" {
				data, _ := yaml.Marshal(skill)
				fmt.Print(string(data))
			}

			return nil
		},
	}

	cmd.Flags().StringVar(&name, "name", "", "Skill name (required)")
	cmd.Flags().StringVar(&description, "description", "", "Skill description")
	cmd.Flags().StringVar(&skillType, "type", "", "Skill type (required, use 'kubiya skill definitions' to see available types)")
	cmd.Flags().StringVar(&variant, "variant", "", "Skill variant")
	cmd.Flags().BoolVar(&enabled, "enabled", true, "Enable the skill")
	cmd.Flags().StringVar(&configFile, "config", "", "Configuration file (JSON)")
	cmd.Flags().StringVar(&configJSON, "config-json", "", "Configuration as inline JSON")
	cmd.Flags().StringVarP(&outputFormat, "output", "o", "", "Output format (json, yaml)")

	return cmd
}

func newListSkillsCommand(cfg *config.Config) *cobra.Command {
	var outputFormat string

	cmd := &cobra.Command{
		Use:     "list",
		Aliases: []string{"ls"},
		Short:   "List all skills",
		Example: `  # List skills
  kubiya skill list

  # List as JSON
  kubiya skill list -o json`,
		RunE: func(cmd *cobra.Command, args []string) error {
			client, err := controlplane.New(cfg.APIKey, cfg.Debug)
			if err != nil {
				return fmt.Errorf("failed to create client: %w", err)
			}

			skills, err := client.ListSkills()
			if err != nil {
				return fmt.Errorf("failed to list skills: %w", err)
			}

			if len(skills) == 0 {
				fmt.Println("No skills found.")
				return nil
			}

			switch outputFormat {
			case "json":
				data, err := json.MarshalIndent(skills, "", "  ")
				if err != nil {
					return err
				}
				fmt.Println(string(data))
			case "yaml":
				data, err := yaml.Marshal(skills)
				if err != nil {
					return err
				}
				fmt.Print(string(data))
			default:
				w := tabwriter.NewWriter(os.Stdout, 0, 0, 3, ' ', 0)
				fmt.Fprintln(w, "ID\tNAME\tTYPE\tVARIANT\tENABLED\tCREATED")
				for _, skill := range skills {
					variant := "-"
					if skill.Variant != nil {
						variant = *skill.Variant
					}
					created := "-"
					if skill.CreatedAt != nil {
						created = skill.CreatedAt.Format("2006-01-02")
					}
					enabled := "false"
					if skill.Enabled {
						enabled = "true"
					}
					fmt.Fprintf(w, "%s\t%s\t%s\t%s\t%s\t%s\n",
						skill.ID,
						skill.Name,
						skill.SkillType,
						variant,
						enabled,
						created,
					)
				}
				w.Flush()
			}

			return nil
		},
	}

	cmd.Flags().StringVarP(&outputFormat, "output", "o", "", "Output format (json, yaml)")

	return cmd
}

func newGetSkillCommand(cfg *config.Config) *cobra.Command {
	var outputFormat string

	cmd := &cobra.Command{
		Use:     "get SKILL_ID",
		Aliases: []string{"describe", "show"},
		Short:   "Get skill details",
		Args:    cobra.ExactArgs(1),
		Example: `  # Get skill details
  kubiya skill get skill-123

  # Get as JSON
  kubiya skill get skill-123 -o json`,
		RunE: func(cmd *cobra.Command, args []string) error {
			skillID := args[0]

			client, err := controlplane.New(cfg.APIKey, cfg.Debug)
			if err != nil {
				return fmt.Errorf("failed to create client: %w", err)
			}

			skill, err := client.GetSkill(skillID)
			if err != nil {
				return fmt.Errorf("failed to get skill: %w", err)
			}

			switch outputFormat {
			case "json":
				data, err := json.MarshalIndent(skill, "", "  ")
				if err != nil {
					return err
				}
				fmt.Println(string(data))
			case "yaml":
				data, err := yaml.Marshal(skill)
				if err != nil {
					return err
				}
				fmt.Print(string(data))
			default:
				fmt.Printf("%s: %s\n", style.BoldStyle.Render("ID"), skill.ID)
				fmt.Printf("%s: %s\n", style.BoldStyle.Render("Name"), skill.Name)
				if skill.Description != nil {
					fmt.Printf("%s: %s\n", style.BoldStyle.Render("Description"), *skill.Description)
				}
				fmt.Printf("%s: %s\n", style.BoldStyle.Render("Type"), skill.SkillType)
				if skill.Variant != nil {
					fmt.Printf("%s: %s\n", style.BoldStyle.Render("Variant"), *skill.Variant)
				}
				fmt.Printf("%s: %t\n", style.BoldStyle.Render("Enabled"), skill.Enabled)
				if len(skill.Configuration) > 0 {
					configJSON, _ := json.MarshalIndent(skill.Configuration, "", "  ")
					fmt.Printf("%s:\n%s\n", style.BoldStyle.Render("Configuration"), string(configJSON))
				}
				if skill.CreatedAt != nil {
					fmt.Printf("%s: %s\n", style.BoldStyle.Render("Created"), skill.CreatedAt.Format("2006-01-02 15:04:05"))
				}
			}

			return nil
		},
	}

	cmd.Flags().StringVarP(&outputFormat, "output", "o", "", "Output format (json, yaml)")

	return cmd
}

func newUpdateSkillCommand(cfg *config.Config) *cobra.Command {
	var (
		name        string
		description string
		variant     string
		enabled     *bool
		configFile  string
		configJSON  string
	)

	cmd := &cobra.Command{
		Use:   "update SKILL_ID",
		Short: "Update a skill",
		Args:  cobra.ExactArgs(1),
		Example: `  # Update skill name
  kubiya skill update skill-123 --name "New Name"

  # Disable a skill
  kubiya skill update skill-123 --enabled=false`,
		RunE: func(cmd *cobra.Command, args []string) error {
			skillID := args[0]

			client, err := controlplane.New(cfg.APIKey, cfg.Debug)
			if err != nil {
				return fmt.Errorf("failed to create client: %w", err)
			}

			req := &entities.SkillUpdateRequest{}

			if name != "" {
				req.Name = &name
			}
			if description != "" {
				req.Description = &description
			}
			if variant != "" {
				req.Variant = &variant
			}
			if enabled != nil {
				req.Enabled = enabled
			}

			if configFile != "" || configJSON != "" {
				var configuration map[string]interface{}
				if configFile != "" {
					data, err := os.ReadFile(configFile)
					if err != nil {
						return fmt.Errorf("failed to read config file: %w", err)
					}
					if err := json.Unmarshal(data, &configuration); err != nil {
						return fmt.Errorf("failed to parse config file: %w", err)
					}
				} else {
					if err := json.Unmarshal([]byte(configJSON), &configuration); err != nil {
						return fmt.Errorf("failed to parse config JSON: %w", err)
					}
				}
				req.Configuration = configuration
			}

			skill, err := client.UpdateSkill(skillID, req)
			if err != nil {
				return fmt.Errorf("failed to update skill: %w", err)
			}

			fmt.Printf("%s Skill %s updated\n",
				style.SuccessStyle.Render("✓"),
				style.HighlightStyle.Render(skill.Name))

			return nil
		},
	}

	cmd.Flags().StringVar(&name, "name", "", "Skill name")
	cmd.Flags().StringVar(&description, "description", "", "Skill description")
	cmd.Flags().StringVar(&variant, "variant", "", "Skill variant")
	enabledBool := cmd.Flags().Bool("enabled", false, "Enable/disable the skill")
	enabled = enabledBool
	cmd.Flags().StringVar(&configFile, "config", "", "Configuration file (JSON)")
	cmd.Flags().StringVar(&configJSON, "config-json", "", "Configuration as inline JSON")

	return cmd
}

func newDeleteSkillCommand(cfg *config.Config) *cobra.Command {
	cmd := &cobra.Command{
		Use:     "delete SKILL_ID",
		Aliases: []string{"remove", "rm"},
		Short:   "Delete a skill",
		Args:    cobra.ExactArgs(1),
		Example: `  # Delete a skill
  kubiya skill delete skill-123`,
		RunE: func(cmd *cobra.Command, args []string) error {
			skillID := args[0]

			client, err := controlplane.New(cfg.APIKey, cfg.Debug)
			if err != nil {
				return fmt.Errorf("failed to create client: %w", err)
			}

			// Get skill name before deleting
			skill, err := client.GetSkill(skillID)
			if err != nil {
				return fmt.Errorf("failed to get skill: %w", err)
			}

			if err := client.DeleteSkill(skillID); err != nil {
				return fmt.Errorf("failed to delete skill: %w", err)
			}

			fmt.Printf("%s Skill %s deleted\n",
				style.SuccessStyle.Render("✓"),
				style.HighlightStyle.Render(skill.Name))

			return nil
		},
	}

	return cmd
}

func newListSkillDefinitionsCommand(cfg *config.Config) *cobra.Command {
	var outputFormat string

	cmd := &cobra.Command{
		Use:     "definitions",
		Aliases: []string{"types", "defs"},
		Short:   "List available skill types and definitions",
		Example: `  # List skill definitions
  kubiya skill definitions

  # List as JSON
  kubiya skill definitions -o json`,
		RunE: func(cmd *cobra.Command, args []string) error {
			client, err := controlplane.New(cfg.APIKey, cfg.Debug)
			if err != nil {
				return fmt.Errorf("failed to create client: %w", err)
			}

			definitions, err := client.GetSkillDefinitions()
			if err != nil {
				return fmt.Errorf("failed to get skill definitions: %w", err)
			}

			switch outputFormat {
			case "json":
				data, err := json.MarshalIndent(definitions, "", "  ")
				if err != nil {
					return err
				}
				fmt.Println(string(data))
			case "yaml":
				data, err := yaml.Marshal(definitions)
				if err != nil {
					return err
				}
				fmt.Print(string(data))
			default:
				w := tabwriter.NewWriter(os.Stdout, 0, 0, 3, ' ', 0)
				fmt.Fprintln(w, "TYPE\tNAME\tDESCRIPTION\tVARIANTS")
				for _, def := range definitions {
					variants := "-"
					if len(def.Variants) > 0 {
						variants = fmt.Sprintf("%d", len(def.Variants))
					}
					fmt.Fprintf(w, "%s\t%s\t%s\t%s\n",
						def.Type,
						def.Name,
						def.Description,
						variants,
					)
				}
				w.Flush()
			}

			return nil
		},
	}

	cmd.Flags().StringVarP(&outputFormat, "output", "o", "", "Output format (json, yaml)")

	return cmd
}

func newAssociateSkillCommand(cfg *config.Config) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "associate ENTITY_TYPE ENTITY_ID SKILL_ID",
		Short: "Associate a skill with an entity (agent, team, or environment)",
		Args:  cobra.ExactArgs(3),
		Example: `  # Associate skill with agent
  kubiya skill associate agent agent-123 skill-456

  # Associate skill with team
  kubiya skill associate team team-789 skill-456`,
		RunE: func(cmd *cobra.Command, args []string) error {
			entityType := args[0]
			entityID := args[1]
			skillID := args[2]

			client, err := controlplane.New(cfg.APIKey, cfg.Debug)
			if err != nil {
				return fmt.Errorf("failed to create client: %w", err)
			}

			_, err = client.AssociateSkill(entityType, entityID, skillID)
			if err != nil {
				return fmt.Errorf("failed to associate skill: %w", err)
			}

			fmt.Printf("%s Skill associated with %s %s\n",
				style.SuccessStyle.Render("✓"),
				entityType,
				entityID)

			return nil
		},
	}

	return cmd
}

func newListSkillAssociationsCommand(cfg *config.Config) *cobra.Command {
	var outputFormat string

	cmd := &cobra.Command{
		Use:     "list-associations ENTITY_TYPE ENTITY_ID",
		Aliases: []string{"associations"},
		Short:   "List skills associated with an entity",
		Args:    cobra.ExactArgs(2),
		Example: `  # List skills for an agent
  kubiya skill list-associations agent agent-123

  # List skills for a team
  kubiya skill list-associations team team-789`,
		RunE: func(cmd *cobra.Command, args []string) error {
			entityType := args[0]
			entityID := args[1]

			client, err := controlplane.New(cfg.APIKey, cfg.Debug)
			if err != nil {
				return fmt.Errorf("failed to create client: %w", err)
			}

			skills, err := client.ListSkillAssociations(entityType, entityID)
			if err != nil {
				return fmt.Errorf("failed to list skill associations: %w", err)
			}

			if len(skills) == 0 {
				fmt.Printf("No skills associated with %s %s\n", entityType, entityID)
				return nil
			}

			switch outputFormat {
			case "json":
				data, err := json.MarshalIndent(skills, "", "  ")
				if err != nil {
					return err
				}
				fmt.Println(string(data))
			case "yaml":
				data, err := yaml.Marshal(skills)
				if err != nil {
					return err
				}
				fmt.Print(string(data))
			default:
				w := tabwriter.NewWriter(os.Stdout, 0, 0, 3, ' ', 0)
				fmt.Fprintln(w, "ID\tNAME\tTYPE\tENABLED")
				for _, skill := range skills {
					enabled := "false"
					if skill.Enabled {
						enabled = "true"
					}
					fmt.Fprintf(w, "%s\t%s\t%s\t%s\n",
						skill.ID,
						skill.Name,
						skill.SkillType,
						enabled,
					)
				}
				w.Flush()
			}

			return nil
		},
	}

	cmd.Flags().StringVarP(&outputFormat, "output", "o", "", "Output format (json, yaml)")

	return cmd
}
