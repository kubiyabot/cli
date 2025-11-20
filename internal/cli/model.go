package cli

import (
	"encoding/json"
	"fmt"

	"github.com/kubiyabot/cli/internal/config"
	"github.com/kubiyabot/cli/internal/controlplane"
	"github.com/kubiyabot/cli/internal/formatter"
	"github.com/kubiyabot/cli/internal/style"
	"github.com/spf13/cobra"
	"gopkg.in/yaml.v3"
)

func newModelCommand(cfg *config.Config) *cobra.Command {
	cmd := &cobra.Command{
		Use:     "model",
		Aliases: []string{"models", "llm"},
		Short:   "🤖 Manage LLM models",
		Long:    `List and view available LLM models in the control plane.`,
	}

	cmd.AddCommand(
		newListModelsCommand(cfg),
		newGetModelCommand(cfg),
		newGetDefaultModelCommand(cfg),
		newListModelProvidersCommand(cfg),
	)

	return cmd
}

func newListModelsCommand(cfg *config.Config) *cobra.Command {
	var outputFormat string

	cmd := &cobra.Command{
		Use:     "list",
		Aliases: []string{"ls"},
		Short:   "List all available models",
		Example: `  # List models
  kubiya model list

  # List as JSON
  kubiya model list -o json

  # List as YAML
  kubiya model list -o yaml`,
		RunE: func(cmd *cobra.Command, args []string) error {
			client, err := controlplane.New(cfg.APIKey, cfg.Debug)
			if err != nil {
				return fmt.Errorf("failed to create client: %w", err)
			}

			models, err := client.ListModels()
			if err != nil {
				return fmt.Errorf("failed to list models: %w", err)
			}

			if len(models) == 0 {
				formatter.EmptyListMessage("models")
				return nil
			}

			switch outputFormat {
			case "json":
				data, err := json.MarshalIndent(models, "", "  ")
				if err != nil {
					return err
				}
				fmt.Println(string(data))
			case "yaml":
				data, err := yaml.Marshal(models)
				if err != nil {
					return err
				}
				fmt.Print(string(data))
			default:
				formatter.ListOutput("Models", "🤖", len(models), func() {
					table := formatter.NewTable("VALUE", "LABEL", "PROVIDER", "ENABLED", "RECOMMENDED")
					for _, model := range models {
						recommended := ""
						if model.Recommended {
							recommended = style.HighlightStyle.Render("★")
						}
						table.AddRow(
							formatter.StyledValue(model.Value),
							formatter.StyledName(model.Label),
							formatter.StyledDim(model.Provider),
							formatter.FormatBoolean(model.Enabled),
							recommended,
						)
					}
					table.Render()
				})
			}

			return nil
		},
	}

	cmd.Flags().StringVarP(&outputFormat, "output", "o", "", "Output format (json, yaml)")

	return cmd
}

func newGetModelCommand(cfg *config.Config) *cobra.Command {
	var outputFormat string

	cmd := &cobra.Command{
		Use:     "get MODEL_ID",
		Aliases: []string{"describe", "show"},
		Short:   "Get model details",
		Args:    cobra.ExactArgs(1),
		Example: `  # Get model details
  kubiya model get gpt-4

  # Get as JSON
  kubiya model get gpt-4 -o json`,
		RunE: func(cmd *cobra.Command, args []string) error {
			modelID := args[0]

			client, err := controlplane.New(cfg.APIKey, cfg.Debug)
			if err != nil {
				return fmt.Errorf("failed to create client: %w", err)
			}

			model, err := client.GetModel(modelID)
			if err != nil {
				return fmt.Errorf("failed to get model: %w", err)
			}

			switch outputFormat {
			case "json":
				data, err := json.MarshalIndent(model, "", "  ")
				if err != nil {
					return err
				}
				fmt.Println(string(data))
			case "yaml":
				data, err := yaml.Marshal(model)
				if err != nil {
					return err
				}
				fmt.Print(string(data))
			default:
				fields := map[string]string{
					"ID":       model.ID,
					"Value":    model.Value,
					"Label":    model.Label,
					"Provider": model.Provider,
					"Enabled":  formatter.FormatBooleanYesNo(model.Enabled),
				}

				if model.Logo != nil {
					fields["Logo"] = *model.Logo
				}

				if model.Description != nil {
					fields["Description"] = *model.Description
				}

				if model.Recommended {
					fields["Recommended"] = style.HighlightStyle.Render("Yes ★")
				}

				if len(model.CompatibleRuntimes) > 0 {
					fields["Compatible Runtimes"] = fmt.Sprintf("%v", model.CompatibleRuntimes)
				}

				if model.CreatedAt != nil {
					fields["Created"] = model.CreatedAt.Format("2006-01-02 15:04:05")
				}

				formatter.DetailOutput("Model Details", "🤖", fields)
			}

			return nil
		},
	}

	cmd.Flags().StringVarP(&outputFormat, "output", "o", "", "Output format (json, yaml)")

	return cmd
}

func newGetDefaultModelCommand(cfg *config.Config) *cobra.Command {
	var outputFormat string

	cmd := &cobra.Command{
		Use:     "default",
		Aliases: []string{"recommended"},
		Short:   "Get the default/recommended model",
		Example: `  # Get default model
  kubiya model default

  # Get as JSON
  kubiya model default -o json`,
		RunE: func(cmd *cobra.Command, args []string) error {
			client, err := controlplane.New(cfg.APIKey, cfg.Debug)
			if err != nil {
				return fmt.Errorf("failed to create client: %w", err)
			}

			model, err := client.GetDefaultModel()
			if err != nil {
				return fmt.Errorf("failed to get default model: %w", err)
			}

			switch outputFormat {
			case "json":
				data, err := json.MarshalIndent(model, "", "  ")
				if err != nil {
					return err
				}
				fmt.Println(string(data))
			case "yaml":
				data, err := yaml.Marshal(model)
				if err != nil {
					return err
				}
				fmt.Print(string(data))
			default:
				fields := map[string]string{
					"ID":       model.ID,
					"Value":    model.Value,
					"Label":    model.Label,
					"Provider": model.Provider,
				}

				if len(model.CompatibleRuntimes) > 0 {
					fields["Compatible Runtimes"] = fmt.Sprintf("%v", model.CompatibleRuntimes)
				}

				if model.Description != nil {
					fields["Description"] = *model.Description
				}

				formatter.DetailOutput("★ Default Model", "🤖", fields)
			}

			return nil
		},
	}

	cmd.Flags().StringVarP(&outputFormat, "output", "o", "", "Output format (json, yaml)")

	return cmd
}

func newListModelProvidersCommand(cfg *config.Config) *cobra.Command {
	var outputFormat string

	cmd := &cobra.Command{
		Use:     "providers",
		Aliases: []string{"provider"},
		Short:   "List unique model providers",
		Example: `  # List providers
  kubiya model providers

  # List as JSON
  kubiya model providers -o json`,
		RunE: func(cmd *cobra.Command, args []string) error {
			client, err := controlplane.New(cfg.APIKey, cfg.Debug)
			if err != nil {
				return fmt.Errorf("failed to create client: %w", err)
			}

			providers, err := client.ListModelProviders()
			if err != nil {
				return fmt.Errorf("failed to list providers: %w", err)
			}

			if len(providers) == 0 {
				formatter.EmptyListMessage("providers")
				return nil
			}

			switch outputFormat {
			case "json":
				data, err := json.MarshalIndent(providers, "", "  ")
				if err != nil {
					return err
				}
				fmt.Println(string(data))
			case "yaml":
				data, err := yaml.Marshal(providers)
				if err != nil {
					return err
				}
				fmt.Print(string(data))
			default:
				formatter.ListOutput("Model Providers", "🤖", len(providers), func() {
					for _, provider := range providers {
						fmt.Printf("  • %s\n", style.HighlightStyle.Render(provider))
					}
				})
			}

			return nil
		},
	}

	cmd.Flags().StringVarP(&outputFormat, "output", "o", "", "Output format (json, yaml)")

	return cmd
}
