package cli

import (
	"encoding/json"
	"fmt"
	"os"
	"strings"
	"text/tabwriter"

	"github.com/kubiyabot/cli/internal/config"
	"github.com/kubiyabot/cli/internal/controlplane"
	"github.com/kubiyabot/cli/internal/controlplane/entities"
	"github.com/kubiyabot/cli/internal/style"
	"github.com/spf13/cobra"
	"gopkg.in/yaml.v3"
)

func newEnvironmentCommand(cfg *config.Config) *cobra.Command {
	cmd := &cobra.Command{
		Use:     "environment",
		Aliases: []string{"env", "environments"},
		Short:   "🌍 Manage environments",
		Long:    `Create, list, update, and delete environments in the control plane.`,
	}

	cmd.AddCommand(
		newCreateEnvironmentCommand(cfg),
		newListEnvironmentsCommand(cfg),
		newGetEnvironmentCommand(cfg),
		newUpdateEnvironmentCommand(cfg),
		newDeleteEnvironmentCommand(cfg),
		newGetEnvironmentWorkerCommandCommand(cfg),
	)

	return cmd
}

func newCreateEnvironmentCommand(cfg *config.Config) *cobra.Command {
	var (
		name          string
		description   string
		variables     []string
		secrets       []string
		integrations  []string
		outputFormat  string
	)

	cmd := &cobra.Command{
		Use:   "create",
		Short: "Create a new environment",
		Example: `  # Create an environment
  kubiya environment create --name "Production" --description "Production environment"

  # Create with variables
  kubiya environment create --name "Dev" --var "LOG_LEVEL=debug" --var "REGION=us-east-1"

  # Create with secrets and integrations
  kubiya environment create --name "Staging" --secret "DB_PASSWORD" --integration "github"`,
		RunE: func(cmd *cobra.Command, args []string) error {
			if name == "" {
				return fmt.Errorf("--name is required")
			}

			client, err := controlplane.New(cfg.APIKey, cfg.Debug)
			if err != nil {
				return fmt.Errorf("failed to create client: %w", err)
			}

			vars := make(map[string]string)
			for _, v := range variables {
				parts := strings.SplitN(v, "=", 2)
				if len(parts) != 2 {
					return fmt.Errorf("invalid variable format: %s (use KEY=VALUE)", v)
				}
				vars[parts[0]] = parts[1]
			}

			req := &entities.EnvironmentCreateRequest{
				Name:         name,
				Variables:    vars,
				Secrets:      secrets,
				Integrations: integrations,
			}

			if description != "" {
				req.Description = &description
			}

			environment, err := client.CreateEnvironment(req)
			if err != nil {
				return fmt.Errorf("failed to create environment: %w", err)
			}

			fmt.Printf("%s Environment %s created (ID: %s)\n",
				style.SuccessStyle.Render("✓"),
				style.HighlightStyle.Render(environment.Name),
				environment.ID)

			if outputFormat == "json" {
				data, _ := json.MarshalIndent(environment, "", "  ")
				fmt.Println(string(data))
			} else if outputFormat == "yaml" {
				data, _ := yaml.Marshal(environment)
				fmt.Print(string(data))
			}

			return nil
		},
	}

	cmd.Flags().StringVar(&name, "name", "", "Environment name (required)")
	cmd.Flags().StringVar(&description, "description", "", "Environment description")
	cmd.Flags().StringSliceVar(&variables, "var", []string{}, "Environment variables (KEY=VALUE, can be specified multiple times)")
	cmd.Flags().StringSliceVar(&secrets, "secret", []string{}, "Secret names (can be specified multiple times)")
	cmd.Flags().StringSliceVar(&integrations, "integration", []string{}, "Integration IDs (can be specified multiple times)")
	cmd.Flags().StringVarP(&outputFormat, "output", "o", "", "Output format (json, yaml)")

	return cmd
}

func newListEnvironmentsCommand(cfg *config.Config) *cobra.Command {
	var outputFormat string

	cmd := &cobra.Command{
		Use:     "list",
		Aliases: []string{"ls"},
		Short:   "List all environments",
		Example: `  # List environments
  kubiya environment list

  # List as JSON
  kubiya environment list -o json`,
		RunE: func(cmd *cobra.Command, args []string) error {
			client, err := controlplane.New(cfg.APIKey, cfg.Debug)
			if err != nil {
				return fmt.Errorf("failed to create client: %w", err)
			}

			environments, err := client.ListEnvironments()
			if err != nil {
				return fmt.Errorf("failed to list environments: %w", err)
			}

			if len(environments) == 0 {
				fmt.Println("No environments found.")
				return nil
			}

			switch outputFormat {
			case "json":
				data, err := json.MarshalIndent(environments, "", "  ")
				if err != nil {
					return err
				}
				fmt.Println(string(data))
			case "yaml":
				data, err := yaml.Marshal(environments)
				if err != nil {
					return err
				}
				fmt.Print(string(data))
			default:
				w := tabwriter.NewWriter(os.Stdout, 0, 0, 3, ' ', 0)
				fmt.Fprintln(w, "ID\tNAME\tVARIABLES\tSECRETS\tINTEGRATIONS\tCREATED")
				for _, env := range environments {
					created := "-"
					if env.CreatedAt != nil {
						created = env.CreatedAt.Format("2006-01-02")
					}
					fmt.Fprintf(w, "%s\t%s\t%d\t%d\t%d\t%s\n",
						env.ID,
						env.Name,
						len(env.Variables),
						len(env.Secrets),
						len(env.Integrations),
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

func newGetEnvironmentCommand(cfg *config.Config) *cobra.Command {
	var outputFormat string

	cmd := &cobra.Command{
		Use:     "get ENV_ID",
		Aliases: []string{"describe", "show"},
		Short:   "Get environment details",
		Args:    cobra.ExactArgs(1),
		Example: `  # Get environment details
  kubiya environment get env-123

  # Get as JSON
  kubiya environment get env-123 -o json`,
		RunE: func(cmd *cobra.Command, args []string) error {
			envID := args[0]

			client, err := controlplane.New(cfg.APIKey, cfg.Debug)
			if err != nil {
				return fmt.Errorf("failed to create client: %w", err)
			}

			environment, err := client.GetEnvironment(envID)
			if err != nil {
				return fmt.Errorf("failed to get environment: %w", err)
			}

			switch outputFormat {
			case "json":
				data, err := json.MarshalIndent(environment, "", "  ")
				if err != nil {
					return err
				}
				fmt.Println(string(data))
			case "yaml":
				data, err := yaml.Marshal(environment)
				if err != nil {
					return err
				}
				fmt.Print(string(data))
			default:
				fmt.Printf("%s: %s\n", style.BoldStyle.Render("ID"), environment.ID)
				fmt.Printf("%s: %s\n", style.BoldStyle.Render("Name"), environment.Name)
				if environment.Description != nil {
					fmt.Printf("%s: %s\n", style.BoldStyle.Render("Description"), *environment.Description)
				}
				if len(environment.Variables) > 0 {
					fmt.Printf("%s:\n", style.BoldStyle.Render("Variables"))
					for k, v := range environment.Variables {
						fmt.Printf("  %s = %s\n", k, v)
					}
				}
				if len(environment.Secrets) > 0 {
					fmt.Printf("%s: %v\n", style.BoldStyle.Render("Secrets"), environment.Secrets)
				}
				if len(environment.Integrations) > 0 {
					fmt.Printf("%s: %v\n", style.BoldStyle.Render("Integrations"), environment.Integrations)
				}
				if environment.CreatedAt != nil {
					fmt.Printf("%s: %s\n", style.BoldStyle.Render("Created"), environment.CreatedAt.Format("2006-01-02 15:04:05"))
				}
			}

			return nil
		},
	}

	cmd.Flags().StringVarP(&outputFormat, "output", "o", "", "Output format (json, yaml)")

	return cmd
}

func newUpdateEnvironmentCommand(cfg *config.Config) *cobra.Command {
	var (
		name          string
		description   string
		variables     []string
		secrets       []string
		integrations  []string
	)

	cmd := &cobra.Command{
		Use:   "update ENV_ID",
		Short: "Update an environment",
		Args:  cobra.ExactArgs(1),
		Example: `  # Update environment name
  kubiya environment update env-123 --name "New Name"

  # Update variables
  kubiya environment update env-123 --var "NEW_VAR=value"

  # Update secrets
  kubiya environment update env-123 --secret "API_KEY"`,
		RunE: func(cmd *cobra.Command, args []string) error {
			envID := args[0]

			client, err := controlplane.New(cfg.APIKey, cfg.Debug)
			if err != nil {
				return fmt.Errorf("failed to create client: %w", err)
			}

			req := &entities.EnvironmentUpdateRequest{}

			if name != "" {
				req.Name = &name
			}
			if description != "" {
				req.Description = &description
			}
			if len(variables) > 0 {
				vars := make(map[string]string)
				for _, v := range variables {
					parts := strings.SplitN(v, "=", 2)
					if len(parts) != 2 {
						return fmt.Errorf("invalid variable format: %s (use KEY=VALUE)", v)
					}
					vars[parts[0]] = parts[1]
				}
				req.Variables = vars
			}
			if len(secrets) > 0 {
				req.Secrets = secrets
			}
			if len(integrations) > 0 {
				req.Integrations = integrations
			}

			environment, err := client.UpdateEnvironment(envID, req)
			if err != nil {
				return fmt.Errorf("failed to update environment: %w", err)
			}

			fmt.Printf("%s Environment %s updated\n",
				style.SuccessStyle.Render("✓"),
				style.HighlightStyle.Render(environment.Name))

			return nil
		},
	}

	cmd.Flags().StringVar(&name, "name", "", "Environment name")
	cmd.Flags().StringVar(&description, "description", "", "Environment description")
	cmd.Flags().StringSliceVar(&variables, "var", []string{}, "Environment variables (KEY=VALUE, replaces existing)")
	cmd.Flags().StringSliceVar(&secrets, "secret", []string{}, "Secret names (replaces existing)")
	cmd.Flags().StringSliceVar(&integrations, "integration", []string{}, "Integration IDs (replaces existing)")

	return cmd
}

func newDeleteEnvironmentCommand(cfg *config.Config) *cobra.Command {
	cmd := &cobra.Command{
		Use:     "delete ENV_ID",
		Aliases: []string{"remove", "rm"},
		Short:   "Delete an environment",
		Args:    cobra.ExactArgs(1),
		Example: `  # Delete an environment
  kubiya environment delete env-123`,
		RunE: func(cmd *cobra.Command, args []string) error {
			envID := args[0]

			client, err := controlplane.New(cfg.APIKey, cfg.Debug)
			if err != nil {
				return fmt.Errorf("failed to create client: %w", err)
			}

			// Get environment name before deleting
			environment, err := client.GetEnvironment(envID)
			if err != nil {
				return fmt.Errorf("failed to get environment: %w", err)
			}

			if err := client.DeleteEnvironment(envID); err != nil {
				return fmt.Errorf("failed to delete environment: %w", err)
			}

			fmt.Printf("%s Environment %s deleted\n",
				style.SuccessStyle.Render("✓"),
				style.HighlightStyle.Render(environment.Name))

			return nil
		},
	}

	return cmd
}

func newGetEnvironmentWorkerCommandCommand(cfg *config.Config) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "worker-command ENV_ID",
		Short: "Get the worker registration command for an environment",
		Args:  cobra.ExactArgs(1),
		Example: `  # Get worker command
  kubiya environment worker-command env-123`,
		RunE: func(cmd *cobra.Command, args []string) error {
			envID := args[0]

			client, err := controlplane.New(cfg.APIKey, cfg.Debug)
			if err != nil {
				return fmt.Errorf("failed to create client: %w", err)
			}

			command, err := client.GetEnvironmentWorkerCommand(envID)
			if err != nil {
				return fmt.Errorf("failed to get worker command: %w", err)
			}

			fmt.Println(style.BoldStyle.Render("Worker Registration Command:"))
			fmt.Println()
			fmt.Println(command)

			return nil
		},
	}

	return cmd
}
