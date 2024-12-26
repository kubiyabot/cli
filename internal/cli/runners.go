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
		Aliases: []string{"runners", "r"},
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
	var (
		outputFormat string
		debug        bool
	)

	cmd := &cobra.Command{
		Use:     "list",
		Aliases: []string{"ls", "l"},
		Short:   "📋 List all runners",
		Example: "  kubiya runner list\n  kubiya runner ls --output json",
		RunE: func(cmd *cobra.Command, args []string) error {
			client := kubiya.NewClient(cfg)
			runners, err := client.ListRunners(cmd.Context())
			if err != nil {
				return err
			}

			if debug {
				fmt.Printf("Debug: Found %d runners\n", len(runners))
				for i, r := range runners {
					fmt.Printf("Runner %d:\n", i+1)
					fmt.Printf("  Name: %q\n", r.Name)
					fmt.Printf("  Type: %q\n", r.RunnerType)
					fmt.Printf("  Version: %q\n", r.Version)
					fmt.Printf("  Namespace: %q\n", r.Namespace)
					fmt.Printf("  Health Status: %q\n", r.RunnerHealth.Status)
					fmt.Printf("  Health: %q\n", r.RunnerHealth.Health)
					fmt.Printf("  Error: %q\n", r.RunnerHealth.Error)
				}
				fmt.Println()
			}

			switch outputFormat {
			case "json":
				return json.NewEncoder(os.Stdout).Encode(runners)
			case "text":
				w := tabwriter.NewWriter(os.Stdout, 0, 0, 2, ' ', 0)
				fmt.Fprintln(w, "🏃 RUNNERS")
				fmt.Fprintln(w, "NAME\tTYPE\tVERSION\tNAMESPACE\tSTATUS\tHEALTH")

				if len(runners) == 0 {
					fmt.Fprintln(w, "No runners found")
					return w.Flush()
				}

				for _, r := range runners {
					// Handle empty fields with appropriate placeholders
					name := r.Name
					if name == "" {
						name = "-"
					}

					runnerType := r.RunnerType
					if runnerType == "" {
						runnerType = "-"
					}

					version := "v2"
					if r.Version == "0" || r.Version == "" {
						version = "v1"
					}

					namespace := r.Namespace
					if namespace == "" {
						namespace = "-"
					}

					status := r.RunnerHealth.Status
					if status == "" {
						if r.RunnerHealth.Error != "" {
							status = "error"
						} else {
							status = "unknown"
						}
					}

					var health string
					if r.RunnerHealth.Error != "" {
						health = "❌ " + r.RunnerHealth.Error
					} else if r.RunnerHealth.Health == "true" {
						health = "✅ healthy"
					} else if r.RunnerHealth.Health == "false" {
						health = "❌ unhealthy"
					} else if r.RunnerHealth.Status == "non-responsive" {
						health = "❌ non-responsive"
					} else {
						health = "unknown"
					}

					fmt.Fprintf(w, "%s\t%s\t%s\t%s\t%s\t%s\n",
						name,
						runnerType,
						version,
						namespace,
						status,
						health,
					)
				}
				return w.Flush()
			default:
				return fmt.Errorf("unknown output format: %s", outputFormat)
			}
		},
	}

	cmd.Flags().StringVarP(&outputFormat, "output", "o", "text", "Output format (text|json)")
	cmd.Flags().BoolVarP(&debug, "debug", "d", false, "Show debug information")
	return cmd
}

func newGetRunnerManifestCommand(cfg *config.Config) *cobra.Command {
	var (
		outputFile string
		apply      bool
		context    string
	)

	cmd := &cobra.Command{
		Use:     "manifest [runner-name]",
		Aliases: []string{"m", "man"},
		Short:   "📜 Get runner's Kubernetes manifest",
		Example: `  # Save manifest to file
  kubiya runner manifest my-runner -o manifest.yaml
  
  # Short form
  kubiya r m my-runner -o manifest.yaml

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
