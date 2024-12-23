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
				return err
			}

			// Filter out v1 runners and add warning
			var v1Runners []string
			validRunners := make([]kubiya.Runner, 0, len(runners))
			for _, r := range runners {
				if r.Version == "v1" {
					v1Runners = append(v1Runners, r.Name)
				} else {
					validRunners = append(validRunners, r)
				}
			}

			// Show warning for v1 runners if any found
			if len(v1Runners) > 0 {
				fmt.Fprintf(os.Stderr, "⚠️  Warning: The following runners are using deprecated v1 version:\n")
				for _, name := range v1Runners {
					fmt.Fprintf(os.Stderr, "   - %s\n", name)
				}
				fmt.Fprintf(os.Stderr, "Please upgrade these runners to the latest version.\n\n")
			}

			switch outputFormat {
			case "json":
				return json.NewEncoder(os.Stdout).Encode(validRunners)
			case "text":
				w := tabwriter.NewWriter(os.Stdout, 0, 0, 2, ' ', 0)
				fmt.Fprintln(w, "🏃 RUNNERS")
				fmt.Fprintln(w, "NAME\tTYPE\tNAMESPACE\tSTATUS\tHEALTH")
				for _, r := range runners {
					fmt.Fprintf(w, "%s\t%s\t%s\t%s\t%s\n",
						r.Name,
						r.RunnerType,
						r.Version,
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

			// Check runner version first
			runner, err := client.GetRunner(cmd.Context(), args[0])
			if err != nil {
				return err
			}

			if runner.Version == "v1" {
				return fmt.Errorf("⚠️  runner '%s' is using deprecated v1 version. Please upgrade to the latest version", args[0])
			}

			// Get manifest URL
			manifest, err := client.GetRunnerManifest(cmd.Context(), args[0])
			if err != nil {
				return err
			}

			// Download manifest content
			content, err := client.DownloadManifest(cmd.Context(), manifest.URL)
			if err != nil {
				return err
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
