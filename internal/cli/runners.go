package cli

import (
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"text/tabwriter"

	"github.com/kubiyabot/cli/internal/config"
	"github.com/kubiyabot/cli/internal/kubiya"
	"github.com/spf13/cobra"
)

func newRunnersCommand(cfg *config.Config) *cobra.Command {
	cmd := &cobra.Command{
		Use:     "runner",
		Aliases: []string{"runners"},
		Short:   "🏃 Manage runners",
		Long:    `Work with Kubiya runners - list and manage your runners.`,
	}

	cmd.AddCommand(
		newListRunnersCommand(cfg),
		newGetRunnerManifestCommand(cfg),
	)

	return cmd
}

func newListRunnersCommand(cfg *config.Config) *cobra.Command {
	var outputFormat string

	cmd := &cobra.Command{
		Use:     "list",
		Short:   "📋 List all runners",
		Example: "  kubiya runner list\n  kubiya runner list --output json",
		RunE: func(cmd *cobra.Command, args []string) error {
			client := kubiya.NewClient(cfg)
			runners, err := client.ListRunners(cmd.Context())
			if err != nil {
				return fmt.Errorf("failed to list runners: %w", err)
			}

			switch outputFormat {
			case "json":
				return json.NewEncoder(os.Stdout).Encode(runners)
			case "text":
				w := tabwriter.NewWriter(os.Stdout, 0, 0, 2, ' ', 0)
				fmt.Fprintln(w, "🏃 RUNNERS\n")
				fmt.Fprintln(w, "NAME\tTYPE\tNAMESPACE\tSTATUS\tHEALTH")
				for _, r := range runners {
					fmt.Fprintf(w, "%s\t%s\t%s\t%s\t%s\n",
						r.Name,
						r.RunnerType,
						r.Namespace,
						r.RunnerHealth.Status,
						r.RunnerHealth.Health,
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

func newGetRunnerManifestCommand(cfg *config.Config) *cobra.Command {
	var (
		outputFile string
		apply      bool
		context    string
	)

	cmd := &cobra.Command{
		Use:   "manifest [runner-name]",
		Short: "📜 Get runner's Kubernetes manifest",
		Example: `  # Save manifest to file
  kubiya runner manifest my-runner -o manifest.yaml

  # Apply manifest directly to current kubectl context
  kubiya runner manifest my-runner --apply

  # Apply to specific context
  kubiya runner manifest my-runner --apply --context my-context`,
		Args: cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			client := kubiya.NewClient(cfg)

			// Get manifest URL
			manifest, err := client.GetRunnerManifest(cmd.Context(), args[0])
			if err != nil {
				return fmt.Errorf("failed to get manifest URL: %w", err)
			}

			// Download manifest content
			content, err := client.DownloadManifest(cmd.Context(), manifest.URL)
			if err != nil {
				return fmt.Errorf("failed to download manifest: %w", err)
			}

			if outputFile != "" {
				if err := os.WriteFile(outputFile, content, 0644); err != nil {
					return fmt.Errorf("failed to write manifest: %w", err)
				}
				fmt.Printf("✅ Manifest saved to: %s\n", outputFile)
			}

			if apply {
				// Create temporary file for kubectl
				tmpfile, err := os.CreateTemp("", "kubiya-*.yaml")
				if err != nil {
					return fmt.Errorf("failed to create temp file: %w", err)
				}
				defer os.Remove(tmpfile.Name())

				if _, err := tmpfile.Write(content); err != nil {
					return fmt.Errorf("failed to write temp file: %w", err)
				}
				tmpfile.Close()

				// Build kubectl command
				args := []string{"apply", "-f", tmpfile.Name()}
				if context != "" {
					args = append([]string{"--context", context}, args...)
				}

				// Run kubectl
				cmd := exec.Command("kubectl", args...)
				cmd.Stdout = os.Stdout
				cmd.Stderr = os.Stderr

				if err := cmd.Run(); err != nil {
					return fmt.Errorf("failed to apply manifest: %w", err)
				}
				fmt.Println("✅ Manifest applied successfully")
			}

			return nil
		},
	}

	cmd.Flags().StringVarP(&outputFile, "output", "o", "", "Save manifest to file")
	cmd.Flags().BoolVar(&apply, "apply", false, "Apply manifest to Kubernetes")
	cmd.Flags().StringVar(&context, "context", "", "Kubernetes context to use")

	return cmd
}
