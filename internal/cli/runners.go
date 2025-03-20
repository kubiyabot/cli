package cli

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"os/exec"
	"runtime"
	"strings"
	"time"

	"github.com/charmbracelet/lipgloss"
	"github.com/kubiyabot/cli/internal/config"
	"github.com/kubiyabot/cli/internal/kubiya"
	"github.com/spf13/cobra"
)

var runnerStyle = struct {
	TitleStyle     lipgloss.Style
	SubtitleStyle  lipgloss.Style
	HighlightStyle lipgloss.Style
	DimStyle       lipgloss.Style
	SuccessStyle   lipgloss.Style
	WarningStyle   lipgloss.Style
	ErrorStyle     lipgloss.Style
	CommandStyle   lipgloss.Style
}{
	TitleStyle:     lipgloss.NewStyle().Bold(true).Foreground(lipgloss.Color("205")),
	SubtitleStyle:  lipgloss.NewStyle().Foreground(lipgloss.Color("240")),
	HighlightStyle: lipgloss.NewStyle().Foreground(lipgloss.Color("212")),
	DimStyle:       lipgloss.NewStyle().Foreground(lipgloss.Color("244")),
	SuccessStyle:   lipgloss.NewStyle().Foreground(lipgloss.Color("40")),
	WarningStyle:   lipgloss.NewStyle().Foreground(lipgloss.Color("214")),
	ErrorStyle:     lipgloss.NewStyle().Foreground(lipgloss.Color("196")),
	CommandStyle:   lipgloss.NewStyle().Foreground(lipgloss.Color("39")),
}

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
		newInstallRunnerCommand(cfg),
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
		Example: "  kubiya runner list\n  kubiya runner ls --output json\n  kubiya runner list --debug",
		RunE: func(cmd *cobra.Command, args []string) error {
			client := kubiya.NewClient(cfg)

			// Set debug mode in client if --debug is passed
			if debug {
				cfg.Debug = true
			}

			// Define spinner frames for progress indication
			spinner := []string{"⠋", "⠙", "⠹", "⠸", "⠼", "⠴", "⠦", "⠧", "⠇", "⠏"}
			spinnerIdx := 0
			ticker := time.NewTicker(100 * time.Millisecond)
			defer ticker.Stop()

			// Start spinner in background
			ctx, cancel := context.WithCancel(cmd.Context())
			defer cancel()

			// Create a channel to receive the runners
			runnersChan := make(chan []kubiya.Runner, 1)
			errChan := make(chan error, 1)

			// Start fetching runners in background
			go func() {
				runners, err := client.ListRunners(ctx)
				if err != nil {
					errChan <- err
					return
				}
				runnersChan <- runners
			}()

			// Show loading animation
			go func() {
				for {
					select {
					case <-ctx.Done():
						return
					case <-ticker.C:
						spinnerIdx = (spinnerIdx + 1) % len(spinner)
						fmt.Printf("\r%s Loading runners...", spinner[spinnerIdx])
					}
				}
			}()

			// Wait for runners or error
			var runners []kubiya.Runner
			select {
			case runners = <-runnersChan:
				cancel()              // Stop spinner
				fmt.Print("\r\033[K") // Clear line
			case err := <-errChan:
				cancel()              // Stop spinner
				fmt.Print("\r\033[K") // Clear line
				return err
			}

			if debug {
				fmt.Printf("Debug: Found %d runners\n", len(runners))
			}

			switch outputFormat {
			case "json":
				return json.NewEncoder(os.Stdout).Encode(runners)
			case "text":
				// Print the title
				fmt.Println(runnerStyle.TitleStyle.Render("🏃 RUNNERS"))

				// Create a simple table with fixed column spacing
				fmt.Println("NAME                 TYPE                 VERSION    NAMESPACE     STATUS     HEALTH           DETAILS")
				fmt.Println("-------------------- -------------------- ---------- ------------- ---------- ---------------- ----------")

				if len(runners) == 0 {
					fmt.Println(runnerStyle.DimStyle.Render("No runners found"))
					return nil
				}

				healthyCount := 0
				unhealthyCount := 0
				unknownCount := 0

				// Print each row with fixed column widths
				for _, r := range runners {
					// Prepare name column (20 chars)
					name := r.Name
					if name == "" {
						name = "-"
					}
					if len(name) > 20 {
						name = name[:17] + "..."
					} else {
						name = fmt.Sprintf("%-20s", name)
					}

					// Prepare type column (20 chars)
					runnerType := r.RunnerType
					if runnerType == "" {
						runnerType = "-"
					}
					if len(runnerType) > 20 {
						runnerType = runnerType[:17] + "..."
					} else {
						runnerType = fmt.Sprintf("%-20s", runnerType)
					}

					// Prepare version column (10 chars)
					version := "v2"
					if r.Version == "0" || r.Version == "" {
						version = "v1"
					}
					version = fmt.Sprintf("%-10s", version)

					// Prepare namespace column (13 chars)
					namespace := r.Namespace
					if namespace == "" {
						namespace = "-"
					}
					if len(namespace) > 13 {
						namespace = namespace[:10] + "..."
					} else {
						namespace = fmt.Sprintf("%-13s", namespace)
					}

					// Prepare status column (10 chars)
					status := r.RunnerHealth.Status
					if status == "" {
						if r.RunnerHealth.Error != "" {
							status = "error"
						} else {
							status = "unknown"
						}
					}
					status = fmt.Sprintf("%-10s", status)

					// Prepare health column (16 chars)
					var health string
					if r.RunnerHealth.Error != "" {
						health = "❌ " + r.RunnerHealth.Error
						if len(health) > 16 {
							health = health[:13] + "..."
						}
						health = fmt.Sprintf("%-16s", health)
						unhealthyCount++
					} else if r.RunnerHealth.Health == "true" {
						health = fmt.Sprintf("%-16s", "✅ healthy")
						healthyCount++
					} else if r.RunnerHealth.Health == "false" {
						health = fmt.Sprintf("%-16s", "❌ unhealthy")
						unhealthyCount++
					} else if r.RunnerHealth.Status == "non-responsive" {
						health = fmt.Sprintf("%-16s", "❌ non-responsive")
						unhealthyCount++
					} else if r.RunnerHealth.Health != "" {
						health = "⚠️ " + r.RunnerHealth.Health
						if len(health) > 16 {
							health = health[:13] + "..."
						}
						health = fmt.Sprintf("%-16s", health)
						unknownCount++
					} else if r.RunnerHealth.Status != "" {
						health = "⚠️ " + r.RunnerHealth.Status
						if len(health) > 16 {
							health = health[:13] + "..."
						}
						health = fmt.Sprintf("%-16s", health)
						unknownCount++
					} else {
						health = fmt.Sprintf("%-16s", "unknown")
						unknownCount++
					}

					// Prepare details column (10 chars)
					var details string
					if r.RunnerHealth.Health == "true" && r.RunnerHealth.Version != "" {
						details = "v" + r.RunnerHealth.Version
					} else {
						details = "-"
					}
					details = fmt.Sprintf("%-10s", details)

					// Now colorize the fields after proper formatting
					name = runnerStyle.HighlightStyle.Render(name)
					version = runnerStyle.HighlightStyle.Render(version)

					if r.RunnerHealth.Status == "ok" {
						status = runnerStyle.SuccessStyle.Render(status)
					} else if r.RunnerHealth.Status == "unknown" || r.RunnerHealth.Status == "" {
						status = runnerStyle.DimStyle.Render(status)
					} else {
						status = runnerStyle.WarningStyle.Render(status)
					}

					if r.RunnerHealth.Error != "" || r.RunnerHealth.Health == "false" || r.RunnerHealth.Status == "non-responsive" {
						health = runnerStyle.ErrorStyle.Render(health)
					} else if r.RunnerHealth.Health == "true" {
						health = runnerStyle.SuccessStyle.Render(health)
					} else {
						health = runnerStyle.WarningStyle.Render(health)
					}

					if r.RunnerHealth.Health == "true" && r.RunnerHealth.Version != "" {
						details = runnerStyle.HighlightStyle.Render(details)
					}

					// Print the row with pre-formatted columns
					fmt.Printf("%s %s %s %s %s %s %s\n",
						name,
						runnerType,
						version,
						namespace,
						status,
						health,
						details)
				}

				// Print summary after the table
				fmt.Printf("\n%s\n", runnerStyle.SubtitleStyle.Render("Summary"))
				fmt.Printf("Total runners: %s\n", runnerStyle.HighlightStyle.Render(fmt.Sprintf("%d", len(runners))))
				fmt.Printf("Healthy: %s\n", runnerStyle.SuccessStyle.Render(fmt.Sprintf("%d", healthyCount)))
				fmt.Printf("Unhealthy: %s\n", runnerStyle.ErrorStyle.Render(fmt.Sprintf("%d", unhealthyCount)))
				fmt.Printf("Unknown: %s\n", runnerStyle.WarningStyle.Render(fmt.Sprintf("%d", unknownCount)))

				// Print helpful tips
				fmt.Printf("\n%s\n", runnerStyle.SubtitleStyle.Render("💡 Helpful Commands"))
				fmt.Printf("• Get runner manifest:\n  %s\n",
					runnerStyle.CommandStyle.Render("kubiya runner manifest <name>"))
				fmt.Printf("• Install a new runner:\n  %s\n",
					runnerStyle.CommandStyle.Render("kubiya runner install <name> [--namespace <ns>] [--wait]"))
				fmt.Printf("• Check runner logs:\n  %s\n",
					runnerStyle.CommandStyle.Render("kubectl logs -n <namespace> <pod-name>"))
				fmt.Printf("• Get detailed runner info:\n  %s\n",
					runnerStyle.CommandStyle.Render("kubiya runner manifest <name> --debug"))

				if unhealthyCount > 0 {
					fmt.Printf("\n%s\n", runnerStyle.ErrorStyle.Render("⚠️  Troubleshooting Tips"))
					fmt.Printf("1. %s\n", runnerStyle.HighlightStyle.Render("Check runner logs:"))
					fmt.Printf("   %s\n", runnerStyle.CommandStyle.Render("kubectl logs -n <namespace> <pod-name>"))
					fmt.Printf("2. %s\n", runnerStyle.HighlightStyle.Render("Verify runner configuration"))
					fmt.Printf("3. %s\n", runnerStyle.HighlightStyle.Render("Check Kubernetes events:"))
					fmt.Printf("   %s\n", runnerStyle.CommandStyle.Render("kubectl get events -n <namespace>"))
					fmt.Printf("4. %s\n", runnerStyle.HighlightStyle.Render("Try reinstalling the runner:"))
					fmt.Printf("   %s\n", runnerStyle.CommandStyle.Render("kubiya runner install <name>"))
				}

				if unknownCount > 0 {
					fmt.Printf("\n%s\n", runnerStyle.WarningStyle.Render("ℹ️  For unknown health status:"))
					fmt.Printf("1. %s\n", runnerStyle.HighlightStyle.Render("Enable debug mode:"))
					fmt.Printf("   %s\n", runnerStyle.CommandStyle.Render("kubiya runner list --debug"))
					fmt.Printf("2. %s\n", runnerStyle.HighlightStyle.Render("Check runner configuration"))
					fmt.Printf("3. %s\n", runnerStyle.HighlightStyle.Render("Verify network connectivity"))
				}

				return nil
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
				// Check if kubectl is installed
				if err := checkKubectlInstalled(); err != nil {
					return err
				}

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

func newInstallRunnerCommand(cfg *config.Config) *cobra.Command {
	var (
		namespace   string
		context     string
		autoApprove bool
		enableRBAC  bool
		installHelm bool
		wait        bool
		timeout     time.Duration
		followLogs  bool
		deploy      bool
	)

	cmd := &cobra.Command{
		Use:     "install [runner-name]",
		Aliases: []string{"i"},
		Short:   "🚀 Install a Kubiya runner on Kubernetes",
		Example: `  # Install a runner with interactive prompts
  kubiya runner install my-runner
  
  # Install with automatic approval and RBAC
  kubiya runner install my-runner -y --rbac
  
  # Install and deploy to current Kubernetes context
  kubiya runner install my-runner --deploy
  
  # Install, deploy, and wait for deployment
  kubiya runner install my-runner --deploy --wait --timeout 5m
  
  # Full installation with all options
  kubiya runner install my-runner --deploy --wait --rbac --namespace kubiya-system --context my-cluster`,
		Args: cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			// Check flag dependencies
			if followLogs && !wait {
				return fmt.Errorf("--follow-logs requires --wait to be specified")
			}

			// If deploy is true, set wait to true by default unless explicitly disabled
			if deploy && !cmd.Flags().Changed("wait") {
				wait = true
			}

			runnerName := args[0]
			client := kubiya.NewClient(cfg)

			// Validate API key is set
			if cfg.APIKey == "" {
				return fmt.Errorf("Kubiya API key is not set. Please run 'kubiya config' to set your API key")
			}

			// Verify connection to Kubiya API
			fmt.Println("🔍 Verifying connection to Kubiya API...")
			if _, err := client.ListRunners(cmd.Context()); err != nil {
				return fmt.Errorf("failed to connect to Kubiya API: %w", err)
			}

			// Get runner manifest from API
			fmt.Printf("🔍 Requesting manifest for runner '%s'...\n", runnerName)
			manifest, err := client.CreateRunnerManifest(cmd.Context(), args[0])
			if err != nil {
				return fmt.Errorf("failed to create runner manifest: %w", err)
			}

			fmt.Printf("✅ Runner '%s' created successfully!\n", runnerName)

			if !deploy {
				fmt.Println("\n💡 To deploy this runner to Kubernetes, run:")
				fmt.Printf("  kubiya runner install %s --deploy\n", runnerName)
				return nil
			}

			// Check if helm is installed
			if err := checkHelmInstalled(autoApprove, installHelm); err != nil {
				return err
			}

			// Download the manifest content
			fmt.Println("📥 Downloading runner manifest...")
			_, err = client.DownloadManifest(cmd.Context(), manifest.URL)
			if err != nil {
				return fmt.Errorf("failed to download manifest: %w", err)
			}

			// Add Helm repository
			fmt.Println("🔄 Adding Kubiya Helm repository...")
			if err := addHelmRepo(autoApprove); err != nil {
				return err
			}

			// Prepare namespace
			if namespace == "" {
				namespace = "kubiya"
			}

			// Check if kubectl is installed if we need to wait
			if wait {
				if err := checkKubectlInstalled(); err != nil {
					return err
				}
			}

			// Check Kubernetes connection
			if err := checkKubernetesConnection(context); err != nil {
				return fmt.Errorf("failed to connect to Kubernetes: %w", err)
			}

			// Install the chart
			fmt.Printf("🚀 Installing runner to namespace '%s'...\n", namespace)
			if err := installHelmChart(runnerName, namespace, context, enableRBAC, autoApprove); err != nil {
				return err
			}

			// Wait for deployment to complete if requested
			if wait {
				fmt.Printf("⏳ Waiting for deployment to complete (timeout: %s)...\n", timeout)
				if err := waitForDeployment(runnerName, namespace, context, timeout); err != nil {
					return fmt.Errorf("deployment did not complete successfully: %w", err)
				}
				fmt.Println("✅ Deployment completed successfully")

				// Verify runner is operational in Kubiya
				fmt.Println("🔍 Verifying runner is operational in Kubiya...")
				if err := verifyRunnerOperational(cmd.Context(), client, runnerName, timeout); err != nil {
					fmt.Printf("⚠️  Warning: %v\n", err)
					fmt.Println("    The deployment completed, but the runner might not be fully operational yet.")
					fmt.Println("    Run 'kubiya runner list' after a few minutes to check the status.")
				} else {
					fmt.Println("✅ Runner is operational in Kubiya")
				}

				// Follow logs if requested
				if followLogs {
					fmt.Println("\n📝 Following runner logs...")
					followRunnerLogs(runnerName, namespace, context)
				}
			}

			fmt.Printf("\n✅ Runner '%s' installed successfully!\n", runnerName)
			fmt.Println("\n🔍 To check the status of your runner, run:")
			fmt.Printf("  kubectl get pods -n %s\n", namespace)
			fmt.Println("\n📊 To view your runner in Kubiya:")
			fmt.Println("  kubiya runner list")

			return nil
		},
	}

	// Add flags
	cmd.Flags().StringVarP(&namespace, "namespace", "n", "", "Kubernetes namespace to install the runner to (default: kubiya)")
	cmd.Flags().StringVar(&context, "context", "", "Kubernetes context to use")
	cmd.Flags().BoolVarP(&autoApprove, "yes", "y", false, "Auto-approve all prompts")
	cmd.Flags().BoolVar(&enableRBAC, "rbac", false, "Enable RBAC for accessing other namespaces")
	cmd.Flags().BoolVar(&installHelm, "install-helm", false, "Install Helm if not already installed")
	cmd.Flags().BoolVarP(&wait, "wait", "w", false, "Wait for deployment to complete")
	cmd.Flags().DurationVar(&timeout, "timeout", 5*time.Minute, "Timeout for wait operation")
	cmd.Flags().BoolVarP(&followLogs, "follow-logs", "f", false, "Follow logs after installation (requires --wait)")
	cmd.Flags().BoolVarP(&deploy, "deploy", "d", false, "Deploy the runner to Kubernetes immediately")

	return cmd
}

// checkHelmInstalled checks if Helm is installed and offers to install it if not
func checkHelmInstalled(autoApprove, installHelm bool) error {
	_, err := exec.LookPath("helm")
	if err == nil {
		// Helm is installed
		return nil
	}

	// Helm is not installed
	fmt.Println("❌ Helm is not installed on your system")

	if !installHelm {
		// Only ask to install if --install-helm wasn't passed
		if !autoApprove {
			fmt.Print("Would you like to install Helm now? [y/N]: ")
			var response string
			fmt.Scanln(&response)
			response = strings.ToLower(response)
			installHelm = response == "y" || response == "yes"
		}
	}

	if !installHelm {
		return fmt.Errorf("Helm is required to install the Kubiya runner. Please install Helm manually or use --install-helm flag")
	}

	// Install Helm
	fmt.Println("📥 Installing Helm...")

	// Detect operating system and architecture
	var installCmd *exec.Cmd
	switch runtime.GOOS {
	case "darwin":
		// macOS - use Homebrew
		installCmd = exec.Command("brew", "install", "helm")
	case "linux":
		// Linux - use curl and install script
		installCmd = exec.Command("sh", "-c", "curl https://raw.githubusercontent.com/helm/helm/master/scripts/get-helm-3 | bash")
	default:
		return fmt.Errorf("automatic Helm installation is not supported on %s. Please install Helm manually: https://helm.sh/docs/intro/install/", runtime.GOOS)
	}

	installCmd.Stdout = os.Stdout
	installCmd.Stderr = os.Stderr
	if err := installCmd.Run(); err != nil {
		return fmt.Errorf("failed to install Helm: %w", err)
	}

	fmt.Println("✅ Helm installed successfully")
	return nil
}

// addHelmRepo adds the Kubiya Helm repository
func addHelmRepo(autoApprove bool) error {
	if !autoApprove {
		fmt.Print("Add Kubiya Helm repository? [Y/n]: ")
		var response string
		fmt.Scanln(&response)
		response = strings.ToLower(response)
		if response == "n" || response == "no" {
			return fmt.Errorf("Kubiya Helm repository is required to install the runner")
		}
	}

	cmd := exec.Command("helm", "repo", "add", "kubiya-helm-charts", "https://kubiyabot.github.io/helm-charts/")
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	if err := cmd.Run(); err != nil {
		return fmt.Errorf("failed to add Helm repository: %w", err)
	}

	// Update repos
	updateCmd := exec.Command("helm", "repo", "update")
	updateCmd.Stdout = os.Stdout
	updateCmd.Stderr = os.Stderr
	if err := updateCmd.Run(); err != nil {
		return fmt.Errorf("failed to update Helm repositories: %w", err)
	}

	return nil
}

// installHelmChart installs the Kubiya runner Helm chart
func installHelmChart(runnerName, namespace, context string, enableRBAC, autoApprove bool) error {
	if !autoApprove {
		fmt.Print("Install Kubiya runner Helm chart? [Y/n]: ")
		var response string
		fmt.Scanln(&response)
		response = strings.ToLower(response)
		if response == "n" || response == "no" {
			return fmt.Errorf("installation aborted")
		}
	}

	// Prepare Helm command
	args := []string{
		"upgrade",
		"--install",
		runnerName,
		"kubiya-helm-charts/kubiya-runner",
		"--set", "alloy.alloy.configMap.create=false",
		"--set", "alloy.alloy.configMap.key=config.alloy",
		"--set", "alloy.alloy.configMap.name=kubiya-runner-alloy-config",
		"--set", "alloy.alloy.extraEnv[0].name=AZURE_REMOTE_WRITE_URL",
		"--set", "alloy.alloy.extraEnv[0].value=https://runners-dce-metrics-prod-frkd.swedencentral-1.metrics.ingest.monitor.azure.com/dataCollectionRules/dcr-1ae3855c75624121bffa32679effcf2d/streams/Microsoft-PrometheusMetrics/api/v1/write?api-version=2023-04-24",
		"--set", "alloy.alloy.extraEnv[1].name=AZURE_CLIENT_ID",
		"--set", "alloy.alloy.extraEnv[1].value=e1768082-b511-41b2-9120-f4313d1d5245",
		"--set", "alloy.alloy.extraEnv[2].name=AZURE_CLIENT_SECRET",
		"--set", "alloy.alloy.extraEnv[2].value=xiv8Q~ZUYNrVFP8cMxUzMosYELxjwQWgzm58laQn",
		"--set", "alloy.alloy.extraEnv[3].name=AZURE_TOKEN_URL",
		"--set", "alloy.alloy.extraEnv[3].value=https://login.microsoftonline.com/84293e41-2381-47c9-afae-63eb789bce50/oauth2/v2.0/token",
		"--set", "alloy.alloy.securityContext.runAsGroup=473",
		"--set", "alloy.alloy.securityContext.runAsUser=473",
		"--set", "nats.jwt=LS0tLS1CRUdJTiBOQVRTIFVTRVIgSldULS0tLS0KZXlKMGVYQWlPaUpLVjFRaUxDSmhiR2NpT2lKbFpESTFOVEU1TFc1clpYa2lmUS5leUpxZEdraU9pSmFRelpDUlUxVlZWUklRMGRTUVRkVlFsbFBUbFpUU0U1WlRrazBWRlZGTmpkTlJUSlBURXhLVmtoUFIwbFZTVkZIU0ZCQklpd2lhV0YwSWpveE56UXlNemsxTURnNUxDSnBjM01pT2lKQlExUkJVVm8yVFZkU1YwdE9SRWxLUlVaTFRFSkpSakpNUkZCVlJrdFdTMU5PTlVkTVUwdEhVa1l6TjFKSU1rVlVWVmxOVTAxQ1ZDSXNJbTVoYldVaU9pSnJkV0pwZVdFdFlXa3VkR1Z6ZENJc0luTjFZaUk2SWxWRFN6WlRORWxRTkVoUVMxQk9RVlpIV0VsR1FrdE5WRTlITWtkVU4wZEdUVFEyUkZkUlZqSklXRFpPV2tVeU5sYzNOMXBIUzFOU0lpd2libUYwY3lJNmV5SndkV0lpT250OUxDSnpkV0lpT250OUxDSnBjM04xWlhKZllXTmpiM1Z1ZENJNklrRkJXRTlRVkRWTlIwcExWRXMyTmxBM1RWY3lVbEpKVEZGWVNFTTJSRnBMVkZSUE5raFhSRmhQUzBwUlRrVkpTVWxQVFZWYU5FSlRJaXdpZEhsd1pTSTZJblZ6WlhJaUxDSjJaWEp6YVc5dUlqb3lmWDAuaGFMZ0EwQlZ2X20wUGFndGx4RmFqdVdFWHk3ODJra0VtVXVJNUhUdEtGcTh1MFA2S1lqb0o4eHF5d0M2bGdpMTAwQVo1S0ltZjN6Um1Menk2YlJMRFEKLS0tLS0tRU5EIE5BVFMgVVNFUiBKV1QtLS0tLS0KCioqKioqKioqKioqKioqKioqKioqKioqKiogSU1QT1JUQU5UICoqKioqKioqKioqKioqKioqKioqKioqKioKTktFWSBTZWVkIHByaW50ZWQgYmVsb3cgY2FuIGJlIHVzZWQgdG8gc2lnbiBhbmQgcHJvdmUgaWRlbnRpdHkuCk5LRVlzIGFyZSBzZW5zaXRpdmUgYW5kIHNob3VsZCBiZSB0cmVhdGVkIGFzIHNlY3JldHMuCgotLS0tLUJFR0lOIFVTRVIgTktFWSBTRUVELS0tLS0KU1VBTUIzVE5QVURaS0FHWlZOUFgyR1BSWDJPNVVFSUlQUzJNR0FaTldINE1MUzJVM1FaSVBPT1BXSQotLS0tLS1FTkQgVVNFUiBOS0VZIFNFRUQtLS0tLS0KCioqKioqKioqKioqKioqKioqKioqKioqKioqKioqKioqKioqKioqKioqKioqKioqKioqKioqKioqKioqKioK",
		"--set", "nats.secondJwt=LS0tLS1CRUdJTiBOQVRTIFVTRVIgSldULS0tLS0KZXlKMGVYQWlPaUpLVjFRaUxDSmhiR2NpT2lKbFpESTFOVEU1TFc1clpYa2lmUS5leUpxZEdraU9pSkZUMHhNTXpkVFVVSk1OVTlEUWpZMVRGUklVRkZWV2t4UFZGcElTVkZFVFZRMk5rWlhXalpQVUVkSk1sbEtSRlJNUkZkQklpd2lhV0YwSWpveE56RTFOVEExT0RZNUxDSnBjM01pT2lKQlJFMDJOMUphVmxwUk5FaFBVMGhPTWxORVNsQlVRbFpZTTAwM1dWaEpUVE15TkRkS1JVeFZVRFJRVHpWT1dVbEpORmRLUjB0VVZpSXNJbTVoYldVaU9pSnJkV0pwZVdFdFlXa2lMQ0p6ZFdJaU9pSlZRVkpVV1ZVM1dGWkdSVmxaUzFnMlJqUTJOVWxSVUVOYVYwSTBXVkpaU1RKVVRFTkpNbGhXVkROTVZVNVNTVUZLVVZSRVJreE9RU0lzSW01aGRITWlPbnNpY0hWaUlqcDdmU3dpYzNWaUlqcDdmU3dpYVhOemRXVnlYMkZqWTI5MWJuUWlPaUpCUVZoUFVGUTFUVWRLUzFSTE5qWlFOMDFYTWxKU1NVeFJXRWhETmtSYVMxUlVUelpJVjBSWVQwdEtVVTVGU1VsSlQwMVZXalJDVXlJc0luUjVjR1VpT2lKMWMyVnlJaXdpZG1WeWMybHZiaUk2TW4xOS5yN1hDNU9SMEo5R0xqWDVGcXFNUUtDeEIxcHlEQTdvVG00NU5uNF9INm1FNjU0MFNQVTRoQnJtQlRyelNXMTdhZ3Q3MjBmeUdwV2tZYmFOWXBYajJBUQotLS0tLS1FTkQgTkFUUyBVU0VSIEpXVC0tLS0tLQoKKioqKioqKioqKioqKioqKioqKioqKioqKiBJTVBPUlRBTlQgKioqKioqKioqKioqKioqKioqKioqKioqKgpOS0VZIFNlZWQgcHJpbnRlZCBiZWxvdyBjYW4gYmUgdXNlZCB0byBzaWduIGFuZCBwcm92ZSBpZGVudGl0eS4KTktFWXMgYXJlIHNlbnNpdGl2ZSBhbmQgc2hvdWxkIGJlIHRyZWF0ZWQgYXMgc2VjcmV0cy4KCi0tLS0tQkVHSU4gVVNFUiBOS0VZIFNFRUQtLS0tLQpTVUFDM0dBMzNGNEdHN1FXMlNDM0FTQ0c1TU5RWkZGVUdIVVg2UzdLUTJINlpFRDdVWUlQWVRWTldNCi0tLS0tLUVORCBVU0VSIE5LRVkgU0VFRC0tLS0tLQoKKioqKioqKioqKioqKioqKioqKioqKioqKioqKioqKioqKioqKioqKioqKioqKioqKioqKioqKioqKioqKgo=",
		"--set", "nats.subject=kubiya-ai." + runnerName + ".incoming",
		"--set", "organization=kubiya-ai",
		"--set", "runner_name=" + runnerName,
		"--set", "user_id=2uXVpYlWeq9eZUf3YinINHRFJRg",
		"--set", "uuid=3df0598e-fce6-4178-8a9b-be040f11e130",
		"--create-namespace",
		"--namespace", namespace,
	}

	// Add RBAC settings if enabled
	if enableRBAC {
		args = append(args, "--set", "toolManager.adminClusterRole.create=true")
	}

	// Add context if specified
	if context != "" {
		args = append([]string{"--kube-context", context}, args...)
	}

	// Run the Helm command
	cmd := exec.Command("helm", args...)
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr

	return cmd.Run()
}

// waitForDeployment waits for runner deployment pods to be ready
func waitForDeployment(runnerName, namespace, context string, timeout time.Duration) error {
	// First, check if the deployment exists
	checkArgs := []string{
		"get",
		"deployment/" + runnerName + "-kubiya-runner",
		"-n", namespace,
		"--no-headers",
	}

	if context != "" {
		checkArgs = append([]string{"--context", context}, checkArgs...)
	}

	// Check if deployment exists
	checkCmd := exec.Command("kubectl", checkArgs...)
	if out, err := checkCmd.CombinedOutput(); err != nil {
		return fmt.Errorf("deployment not found: %s, %w", string(out), err)
	}

	// Build kubectl command to wait for deployment
	args := []string{
		"wait",
		"--for=condition=Available",
		"deployment/" + runnerName + "-kubiya-runner",
		"--timeout=" + timeout.String(),
		"-n", namespace,
	}

	// Add context if specified
	if context != "" {
		args = append([]string{"--context", context}, args...)
	}

	// Run the kubectl command
	fmt.Println("Waiting for main deployment...")
	cmd := exec.Command("kubectl", args...)
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	if err := cmd.Run(); err != nil {
		return fmt.Errorf("failed waiting for main deployment: %w", err)
	}

	// Check if there are any pods
	podCheckArgs := []string{
		"get",
		"pod",
		"-l", "app.kubernetes.io/instance=" + runnerName,
		"-n", namespace,
		"--no-headers",
	}

	if context != "" {
		podCheckArgs = append([]string{"--context", context}, podCheckArgs...)
	}

	podCheckCmd := exec.Command("kubectl", podCheckArgs...)
	podOutput, err := podCheckCmd.CombinedOutput()
	if err != nil || len(podOutput) == 0 {
		return fmt.Errorf("no pods found for the deployment, please check manually")
	}

	// Wait for all pods to be ready
	podArgs := []string{
		"wait",
		"--for=condition=Ready",
		"pod",
		"-l", "app.kubernetes.io/instance=" + runnerName,
		"--timeout=" + timeout.String(),
		"-n", namespace,
	}

	// Add context if specified
	if context != "" {
		podArgs = append([]string{"--context", context}, podArgs...)
	}

	// Run the kubectl command to wait for pods
	fmt.Println("Waiting for all pods to be ready...")
	podCmd := exec.Command("kubectl", podArgs...)
	podCmd.Stdout = os.Stdout
	podCmd.Stderr = os.Stderr

	return podCmd.Run()
}

// checkKubectlInstalled checks if kubectl is installed
func checkKubectlInstalled() error {
	_, err := exec.LookPath("kubectl")
	if err != nil {
		return fmt.Errorf("kubectl is required for this operation but not found in your PATH. Please install kubectl: https://kubernetes.io/docs/tasks/tools/")
	}
	return nil
}

// checkKubernetesConnection verifies that we can connect to the Kubernetes cluster
func checkKubernetesConnection(context string) error {
	// Check if kubectl is installed first
	if err := checkKubectlInstalled(); err != nil {
		return err
	}

	// Build command to check connection
	args := []string{"cluster-info"}

	if context != "" {
		args = append([]string{"--context", context}, args...)
	}

	// Run kubectl command silently to check connection
	cmd := exec.Command("kubectl", args...)
	cmd.Stdout = io.Discard
	cmd.Stderr = io.Discard

	// Execute the command
	if err := cmd.Run(); err != nil {
		if context != "" {
			return fmt.Errorf("could not connect to Kubernetes context '%s': %w", context, err)
		}
		return fmt.Errorf("could not connect to Kubernetes: %w", err)
	}

	return nil
}

// verifyRunnerOperational checks if the runner is operational via the Kubiya API
func verifyRunnerOperational(ctx context.Context, client *kubiya.Client, runnerName string, timeout time.Duration) error {
	// Set a timeout for the verification
	ctx, cancel := context.WithTimeout(ctx, timeout)
	defer cancel()

	// Poll with exponential backoff
	maxAttempts := 10
	initialBackoff := 5 * time.Second
	backoff := initialBackoff

	for attempt := 1; attempt <= maxAttempts; attempt++ {
		// Check if context is done
		select {
		case <-ctx.Done():
			return fmt.Errorf("timed out waiting for runner to be operational")
		default:
			// Continue
		}

		// Get runner details
		runner, err := client.GetRunner(ctx, runnerName)
		if err != nil {
			fmt.Printf("  Waiting for runner to appear in Kubiya (attempt %d/%d)...\n", attempt, maxAttempts)
			time.Sleep(backoff)
			backoff *= 2 // Exponential backoff
			continue
		}

		// Check runner health
		if runner.RunnerHealth.Health == "true" && runner.RunnerHealth.Status != "non-responsive" {
			return nil // Runner is healthy
		}

		// Runner exists but is not healthy yet
		fmt.Printf("  Runner found but not healthy yet. Status: %s (attempt %d/%d)\n",
			runner.RunnerHealth.Status, attempt, maxAttempts)

		time.Sleep(backoff)
		backoff *= 2 // Exponential backoff
	}

	return fmt.Errorf("runner exists but is not healthy after %d attempts", maxAttempts)
}

// followRunnerLogs follows the logs of the runner pods
func followRunnerLogs(runnerName, namespace, context string) error {
	// Get the pod name of the main runner container
	podArgs := []string{
		"get",
		"pods",
		"-l", "app.kubernetes.io/instance=" + runnerName + ",app.kubernetes.io/component=runner",
		"-n", namespace,
		"--no-headers",
		"-o", "custom-columns=:metadata.name",
	}

	if context != "" {
		podArgs = append([]string{"--context", context}, podArgs...)
	}

	podCmd := exec.Command("kubectl", podArgs...)
	podOutput, err := podCmd.Output()
	if err != nil {
		return fmt.Errorf("failed to get runner pod: %w", err)
	}

	podName := strings.TrimSpace(string(podOutput))
	if podName == "" {
		return fmt.Errorf("no runner pod found")
	}

	// Follow logs from the pod
	logArgs := []string{
		"logs",
		"-f",
		podName,
		"-n", namespace,
	}

	if context != "" {
		logArgs = append([]string{"--context", context}, logArgs...)
	}

	// Run in background and don't wait for it to complete
	logCmd := exec.Command("kubectl", logArgs...)
	logCmd.Stdout = os.Stdout
	logCmd.Stderr = os.Stderr

	// Start but don't wait for completion (user can Ctrl+C)
	return logCmd.Start()
}
