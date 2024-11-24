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

func newSecretsCommand(cfg *config.Config) *cobra.Command {
	cmd := &cobra.Command{
		Use:     "secret",
		Aliases: []string{"secrets"},
		Short:   "🔒 Manage secrets",
		Long:    `Create, list, and delete secrets used by tools and teammates.`,
	}

	cmd.AddCommand(
		newListSecretsCommand(cfg),
		newSetSecretCommand(cfg),
		newDeleteSecretCommand(cfg),
	)

	return cmd
}

func newListSecretsCommand(cfg *config.Config) *cobra.Command {
	var outputFormat string

	cmd := &cobra.Command{
		Use:     "list",
		Short:   "📋 List all secrets",
		Example: "  kubiya secret list\n  kubiya secret list --output json",
		RunE: func(cmd *cobra.Command, args []string) error {
			client := kubiya.NewClient(cfg)
			secrets, err := client.ListSecrets(cmd.Context())
			if err != nil {
				return err
			}

			switch outputFormat {
			case "json":
				return json.NewEncoder(os.Stdout).Encode(secrets)
			case "text":
				w := tabwriter.NewWriter(os.Stdout, 0, 0, 2, ' ', 0)
				fmt.Fprintln(w, "🔒 SECRETS")
				fmt.Fprintln(w, "NAME\tCREATED BY\tCREATED AT\tDESCRIPTION")
				for _, s := range secrets {
					fmt.Fprintf(w, "%s\t%s\t%s\t%s\n",
						s.Name,
						s.CreatedBy,
						s.CreatedAt,
						s.Description,
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

func newSetSecretCommand(cfg *config.Config) *cobra.Command {
	var (
		value       string
		description string
		fromFile    string
	)

	cmd := &cobra.Command{
		Use:   "set [name]",
		Short: "➕ Set a secret value",
		Example: `  # Set from value
  kubiya secret set MY_SECRET --value "secret-value" --description "My secret"

  # Set from file
  kubiya secret set MY_SECRET --from-file ./secret.txt`,
		Args: cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			if fromFile != "" && value != "" {
				return fmt.Errorf("cannot use both --value and --from-file")
			}

			if fromFile != "" {
				data, err := os.ReadFile(fromFile)
				if err != nil {
					return fmt.Errorf("failed to read file: %w", err)
				}
				value = string(data)
			}

			if value == "" {
				return fmt.Errorf("secret value must be provided via --value or --from-file")
			}

			client := kubiya.NewClient(cfg)
			if err := client.SetSecret(cmd.Context(), args[0], value, description); err != nil {
				return err
			}

			fmt.Printf("✅ Secret %s set successfully\n", args[0])
			return nil
		},
	}

	cmd.Flags().StringVarP(&value, "value", "v", "", "Secret value")
	cmd.Flags().StringVarP(&description, "description", "d", "", "Secret description")
	cmd.Flags().StringVar(&fromFile, "from-file", "", "Read secret value from file")

	return cmd
}

func newDeleteSecretCommand(cfg *config.Config) *cobra.Command {
	var force bool

	cmd := &cobra.Command{
		Use:     "delete [name]",
		Short:   "🗑️ Delete a secret",
		Example: "  kubiya secret delete MY_SECRET\n  kubiya secret delete MY_SECRET --force",
		Args:    cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			if !force {
				fmt.Printf("⚠️  Are you sure you want to delete secret %s? [y/N] ", args[0])
				var confirm string
				fmt.Scanln(&confirm)
				if strings.ToLower(confirm) != "y" {
					return fmt.Errorf("operation cancelled")
				}
			}

			client := kubiya.NewClient(cfg)
			if err := client.DeleteSecret(cmd.Context(), args[0]); err != nil {
				return err
			}

			fmt.Printf("✅ Secret %s deleted successfully\n", args[0])
			return nil
		},
	}

	cmd.Flags().BoolVarP(&force, "force", "f", false, "Skip confirmation")
	return cmd
}
