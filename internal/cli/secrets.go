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
	"github.com/spf13/cobra"
)

func newSecretsCommand(cfg *config.Config) *cobra.Command {
	cmd := &cobra.Command{
		Use:     "secret",
		Aliases: []string{"secrets"},
		Short:   "🔒 Manage secrets",
		Long:    `Create, read, update, and delete secrets used by tools and teammates.`,
	}

	cmd.AddCommand(
		newListSecretsCommand(cfg),
		newGetSecretCommand(cfg),
		newGetSecretValueCommand(cfg),
		newCreateSecretCommand(cfg),
		newUpdateSecretCommand(cfg),
		newDeleteSecretCommand(cfg),
		newEditSecretCommand(cfg),
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
				fmt.Fprintln(w, "🔒 SECRETS\n")
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

func newGetSecretValueCommand(cfg *config.Config) *cobra.Command {
	var outputFormat string

	cmd := &cobra.Command{
		Use:   "value [name]",
		Short: "🔑 Get secret value",
		Example: `  # Get secret value
  kubiya secret value MY_SECRET
  
  # Get value in JSON format
  kubiya secret value MY_SECRET --output json`,
		Args: cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			client := kubiya.NewClient(cfg)
			value, err := client.GetSecretValue(cmd.Context(), args[0])
			if err != nil {
				return err
			}

			switch outputFormat {
			case "json":
				return json.NewEncoder(os.Stdout).Encode(map[string]string{"value": value})
			default:
				fmt.Printf("Value: %s\n", value)
				return nil
			}
		},
	}

	cmd.Flags().StringVarP(&outputFormat, "output", "o", "text", "Output format (text|json)")
	return cmd
}

func newGetSecretCommand(cfg *config.Config) *cobra.Command {
	var outputFormat string

	cmd := &cobra.Command{
		Use:   "get [name]",
		Short: "🔍 Get secret details",
		Example: `  # Get secret details
  kubiya secret get MY_SECRET
  
  # Get details in JSON format
  kubiya secret get MY_SECRET --output json`,
		Args: cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			client := kubiya.NewClient(cfg)

			secret, err := client.GetSecret(cmd.Context(), args[0])
			if err != nil {
				return err
			}

			switch outputFormat {
			case "json":
				return json.NewEncoder(os.Stdout).Encode(secret)
			default:
				fmt.Printf("Name: %s\n", secret.Name)
				fmt.Printf("Created By: %s\n", secret.CreatedBy)
				fmt.Printf("Created At: %s\n", secret.CreatedAt)
				if secret.Description != "" {
					fmt.Printf("Description: %s\n", secret.Description)
				}
				return nil
			}
		},
	}

	cmd.Flags().StringVarP(&outputFormat, "output", "o", "text", "Output format (text|json)")
	return cmd
}

func newCreateSecretCommand(cfg *config.Config) *cobra.Command {
	var (
		value       string
		description string
		fromFile    string
	)

	cmd := &cobra.Command{
		Use:   "create [name]",
		Short: "➕ Create new secret",
		Example: `  # Create from value
  kubiya secret create MY_SECRET --value "secret-value" --description "My secret"

  # Create from file
  kubiya secret create MY_SECRET --from-file ./secret.txt`,
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
			if err := client.CreateSecret(cmd.Context(), args[0], value, description); err != nil {
				return err
			}

			fmt.Printf("✅ Created secret: %s\n", args[0])
			return nil
		},
	}

	cmd.Flags().StringVarP(&value, "value", "v", "", "Secret value")
	cmd.Flags().StringVarP(&description, "description", "d", "", "Secret description")
	cmd.Flags().StringVar(&fromFile, "from-file", "", "Read value from file")

	return cmd
}

func newEditSecretCommand(cfg *config.Config) *cobra.Command {
	return &cobra.Command{
		Use:     "edit [name]",
		Short:   "✏️ Edit secret value in your default editor",
		Example: "  kubiya secret edit MY_SECRET",
		Args:    cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			client := kubiya.NewClient(cfg)
			name := args[0]

			// Get current value
			value, err := client.GetSecretValue(cmd.Context(), name)
			if err != nil {
				return err
			}

			// Create temporary file
			tmpfile, err := os.CreateTemp("", "kubiya-*.secret")
			if err != nil {
				return fmt.Errorf("failed to create temp file: %w", err)
			}
			defer os.Remove(tmpfile.Name())

			// Write current value
			if _, err := tmpfile.WriteString(value); err != nil {
				return fmt.Errorf("failed to write to temp file: %w", err)
			}
			tmpfile.Close()

			// Open in editor
			editor := os.Getenv("EDITOR")
			if editor == "" {
				editor = "vim"
			}

			editorCmd := exec.Command(editor, tmpfile.Name())
			editorCmd.Stdin = os.Stdin
			editorCmd.Stdout = os.Stdout
			editorCmd.Stderr = os.Stderr
			if err := editorCmd.Run(); err != nil {
				return fmt.Errorf("editor failed: %w", err)
			}

			// Read updated content
			content, err := os.ReadFile(tmpfile.Name())
			if err != nil {
				return fmt.Errorf("failed to read updated content: %w", err)
			}

			// Update the secret
			if err := client.UpdateSecret(cmd.Context(), name, string(content), ""); err != nil {
				return err
			}

			fmt.Printf("✅ Updated secret: %s\n", name)
			return nil
		},
	}
}

func newUpdateSecretCommand(cfg *config.Config) *cobra.Command {
	var (
		value       string
		description string
		fromFile    string
	)

	cmd := &cobra.Command{
		Use:   "update [name]",
		Short: "🔄 Update secret value",
		Example: `  # Update from value
  kubiya secret update MY_SECRET --value "new-secret-value"

  # Update from file
  kubiya secret update MY_SECRET --from-file ./secret.txt`,
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
			if err := client.UpdateSecret(cmd.Context(), args[0], value, description); err != nil {
				return err
			}

			fmt.Printf("✅ Secret %s updated successfully\n", args[0])
			return nil
		},
	}

	cmd.Flags().StringVarP(&value, "value", "v", "", "New secret value")
	cmd.Flags().StringVarP(&description, "description", "d", "", "New description")
	cmd.Flags().StringVar(&fromFile, "from-file", "", "Read new value from file")

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
