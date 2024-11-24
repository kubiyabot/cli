package cli

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"os"
	"strings"
	"time"

	"github.com/kubiyabot/cli/internal/config"
	"github.com/kubiyabot/cli/internal/kubectl"
	"github.com/kubiyabot/cli/internal/kubiya"
	"github.com/kubiyabot/cli/internal/portforward"
	"github.com/kubiyabot/cli/internal/style"
	"github.com/spf13/cobra"
)

func newExecuteCommand(cfg *config.Config) *cobra.Command {
	var (
		sourceUUID string

		teammateID     string
		kubeContext    string
		async          bool
		follow         bool
		interactive    bool
		rawOutput      bool
		timeout        time.Duration
		argValues      []string
		envValues      []string
		autoApprove    bool
		useLocalEnv    bool
		skipEnvCheck   bool
		envSource      string // "local", "teammate", "manual"
		nonInteractive bool
	)

	cmd := &cobra.Command{
		Use:   "execute [tool-name]",
		Short: "🚀 Execute a tool",
		RunE: func(cmd *cobra.Command, args []string) error {
			if len(args) == 0 {
				return fmt.Errorf("tool name is required")
			}
			toolName := args[0]

			// Setup client
			client := kubiya.NewClient(cfg)

			// Get source and tool
			var tool *kubiya.Tool
			var source *kubiya.Source

			fmt.Printf("🔍 Looking for tool '%s'...\n", toolName)

			if sourceUUID != "" {
				var err error
				source, err = client.GetSourceMetadata(cmd.Context(), sourceUUID)
				if err != nil {
					return fmt.Errorf("failed to get source: %w", err)
				}

				// Find tool in source
				for _, t := range source.Tools {
					if t.Name == toolName {
						tool = &t
						break
					}
				}
			} else {
				// Search all sources
				sources, err := client.ListSources(cmd.Context())
				if err != nil {
					return err
				}

				for _, s := range sources {
					metadata, err := client.GetSourceMetadata(cmd.Context(), s.UUID)
					if err != nil {
						continue
					}
					for _, t := range metadata.Tools {
						if t.Name == toolName {
							tool = &t
							source = metadata
							break
						}
					}
					if tool != nil {
						break
					}
				}
			}

			if tool == nil {
				return fmt.Errorf("tool '%s' not found", toolName)
			}

			fmt.Printf("✅ Found tool '%s' in source '%s'\n", toolName, source.Name)

			// Handle Kubernetes context
			if kubeContext == "" {
				currentCtx, err := kubectl.GetCurrentContext()
				if err != nil {
					return fmt.Errorf("failed to get current context: %w", err)
				}
				kubeContext = currentCtx
			}

			// Validate context
			if err := kubectl.ValidateContext(kubeContext); err != nil {
				return fmt.Errorf("invalid context: %w", err)
			}

			fmt.Printf("🔄 Using Kubernetes context: %s\n", kubeContext)

			// Setup port-forward
			fmt.Printf("⚡ Setting up connection to tool manager...\n")

			pf, err := portforward.NewPortForwarder("kubiya", "tool-manager", 80, 80)
			if err != nil {
				return fmt.Errorf("failed to create port-forwarder: %w", err)
			}

			if err := pf.SetContext(kubeContext); err != nil {
				return fmt.Errorf("failed to set context: %w", err)
			}

			pfCtx, pfCancel := context.WithTimeout(cmd.Context(), 30*time.Second)
			defer pfCancel()

			fmt.Println("Starting port forward...")
			if err := pf.Start(pfCtx); err != nil {
				return fmt.Errorf("failed to start port-forward: %w", err)
			}
			defer pf.Stop()

			fmt.Println("Waiting for port forward to be ready...")
			if err := pf.WaitUntilReady(pfCtx); err != nil {
				return fmt.Errorf("port-forward failed: %w", err)
			}

			fmt.Printf("✅ Connection established\n")

			// Collect arguments and environment variables
			argMap := make(map[string]string)
			envMap := make(map[string]string)

			// Parse provided arguments
			for _, arg := range argValues {
				parts := strings.SplitN(arg, "=", 2)
				if len(parts) != 2 {
					return fmt.Errorf("invalid argument format: %s (expected key=value)", arg)
				}
				argMap[parts[0]] = parts[1]
			}

			// Interactive mode for missing required arguments
			if interactive || len(argMap) < len(tool.Args) {
				fmt.Printf("\n%s\n\n", style.TitleStyle.Render(fmt.Sprintf(" 🛠️  Execute: %s ", tool.Name)))

				if tool.Description != "" {
					fmt.Printf("Description: %s\n\n", tool.Description)
				}

				// Collect missing arguments
				for _, arg := range tool.Args {
					if value, exists := argMap[arg.Name]; exists {
						fmt.Printf("✓ %s: %s\n", arg.Name, value)
						continue
					}

					if arg.Required {
						fmt.Printf("Enter value for %s", style.HighlightStyle.Render(arg.Name))
						if arg.Description != "" {
							fmt.Printf(" (%s)", arg.Description)
						}
						fmt.Printf(": ")

						var value string
						fmt.Scanln(&value)

						if value == "" && arg.Required {
							return fmt.Errorf("required argument '%s' cannot be empty", arg.Name)
						}

						argMap[arg.Name] = value
					}
				}
				fmt.Println()
			}

			// Environment variable handling
			for _, env := range tool.Env {
				var envValue string
				var source string

				// Check local environment if allowed
				if useLocalEnv {
					if val, exists := os.LookupEnv(env); exists {
						envValue = val
						source = "local environment"
					}
				}

				// Check teammate if specified
				if envValue == "" && teammateID != "" {
					teammate, err := client.GetTeammate(cmd.Context(), teammateID)
					if err != nil {
						return fmt.Errorf("failed to get teammate: %w", err)
					}
					if val, exists := teammate.Environment[env]; exists {
						envValue = val
						source = fmt.Sprintf("teammate %s", teammate.Name)
					}
				}

				// Check manual override
				if val, exists := getEnvFromValues(env, envValues); exists {
					envValue = val
					source = "manual override"
				}

				if envValue == "" {
					if !skipEnvCheck {
						fmt.Printf("⚠️  Warning: Missing environment variable: %s\n", env)
					}
				} else {
					envMap[env] = envValue
					if !nonInteractive {
						fmt.Printf("✓ Using %s from %s\n", env, source)
					}
				}
			}

			// Prepare execution request
			execReq := struct {
				ToolName  string            `json:"tool_name"`
				SourceURL string            `json:"source_url"`
				ArgMap    map[string]string `json:"arg_map"`
				EnvVars   map[string]string `json:"env_vars"`
				Async     bool              `json:"async"`
			}{
				ToolName:  tool.Name,
				SourceURL: source.URL,
				ArgMap:    argMap,
				EnvVars:   envMap,
				Async:     async,
			}

			// Execute the tool
			fmt.Printf("🚀 Executing %s...\n", tool.Name)

			jsonData, err := json.Marshal(execReq)
			if err != nil {
				return fmt.Errorf("failed to marshal request: %w", err)
			}

			httpClient := &http.Client{Timeout: timeout}
			resp, err := httpClient.Post("http://localhost:80/tool/execute", "application/json", bytes.NewBuffer(jsonData))
			if err != nil {
				return fmt.Errorf("failed to execute tool: %w", err)
			}
			defer resp.Body.Close()

			var execResp struct {
				Status      string `json:"status"`
				ExecutionID string `json:"execution_id,omitempty"`
				Output      string `json:"output,omitempty"`
				Error       string `json:"error,omitempty"`
			}

			if err := json.NewDecoder(resp.Body).Decode(&execResp); err != nil {
				return fmt.Errorf("failed to decode response: %w", err)
			}

			if execResp.Error != "" {
				return fmt.Errorf("execution failed: %s", execResp.Error)
			}

			if async {
				fmt.Printf("✅ Execution started (ID: %s)\n", execResp.ExecutionID)
				if follow {
					fmt.Println("📝 Following output:")
					if err := followExecution(httpClient, execResp.ExecutionID, rawOutput); err != nil {
						return fmt.Errorf("failed to follow execution: %w", err)
					}
				}
			} else {
				if rawOutput {
					fmt.Print(execResp.Output)
				} else {
					fmt.Printf("\n📝 Output:\n%s\n", execResp.Output)
				}
			}

			return nil
		},
	}

	cmd.Flags().StringVarP(&sourceUUID, "source", "s", "", "Source UUID")
	cmd.Flags().StringVarP(&teammateID, "teammate", "t", "", "Teammate ID")
	cmd.Flags().StringVarP(&kubeContext, "context", "c", "", "Kubernetes context")
	cmd.Flags().BoolVarP(&async, "async", "a", false, "Execute asynchronously")
	cmd.Flags().BoolVarP(&follow, "follow", "f", false, "Follow async execution")
	cmd.Flags().BoolVarP(&interactive, "interactive", "i", false, "Interactive mode")
	cmd.Flags().BoolVar(&rawOutput, "raw", false, "Raw output")
	cmd.Flags().DurationVar(&timeout, "timeout", 30*time.Second, "Execution timeout")
	cmd.Flags().StringArrayVar(&argValues, "arg", []string{}, "Tool arguments (key=value)")
	cmd.Flags().StringArrayVar(&envValues, "env", []string{}, "Environment variables (key=value)")
	cmd.Flags().BoolVar(&autoApprove, "yes", false, "Auto approve warnings")
	cmd.Flags().BoolVar(&useLocalEnv, "use-local-env", false, "Use local environment")
	cmd.Flags().BoolVar(&skipEnvCheck, "skip-env-check", false, "Skip environment checks")
	cmd.Flags().StringVar(&envSource, "env-source", "", "Environment source")
	cmd.Flags().BoolVarP(&nonInteractive, "non-interactive", "n", false, "Non-interactive mode")

	return cmd
}

// followExecution follows the output of an async execution
func followExecution(httpClient *http.Client, executionID string, rawOutput bool) error {
	spinner := []string{"⠋", "⠙", "⠹", "⠸", "⠼", "⠴", "⠦", "⠧", "⠇", "⠏"}
	spinnerIdx := 0
	lastOutput := ""

	for {
		time.Sleep(1 * time.Second)

		resp, err := httpClient.Get(fmt.Sprintf("http://localhost:80/tool/status?id=%s", executionID))
		if err != nil {
			return fmt.Errorf("failed to get execution status: %w", err)
		}

		var status struct {
			Status string `json:"status"`
			Output string `json:"output,omitempty"`
			Error  string `json:"error,omitempty"`
		}

		if err := json.NewDecoder(resp.Body).Decode(&status); err != nil {
			resp.Body.Close()
			return fmt.Errorf("failed to decode status: %w", err)
		}
		resp.Body.Close()

		// Only print new output
		if status.Output != "" && status.Output != lastOutput {
			if rawOutput {
				fmt.Print(status.Output[len(lastOutput):])
			} else {
				fmt.Printf("%s\n", status.Output[len(lastOutput):])
			}
			lastOutput = status.Output
		}

		if status.Status == "completed" || status.Status == "failed" {
			if !rawOutput {
				if status.Status == "completed" {
					fmt.Fprintf(os.Stderr, "\n%s\n", style.HighlightStyle.Render("✅ Execution completed"))
				} else {
					fmt.Fprintf(os.Stderr, "\n%s\n", style.HighlightStyle.Render("❌ Execution failed"))
				}
			}
			if status.Status == "failed" {
				return fmt.Errorf("execution failed: %s", status.Error)
			}
			break
		} else if !rawOutput {
			// Show spinner for in-progress executions
			fmt.Printf("\r%s Execution in progress... %s",
				style.HighlightStyle.Render("⏳"),
				spinner[spinnerIdx])
			spinnerIdx = (spinnerIdx + 1) % len(spinner)
		}
	}
	return nil
}

// Helper function to get environment variable from values
func getEnvFromValues(key string, values []string) (string, bool) {
	prefix := key + "="
	for _, v := range values {
		if strings.HasPrefix(v, prefix) {
			return strings.TrimPrefix(v, prefix), true
		}
	}
	return "", false
}

// Add this function to handle secret resolution
func resolveSecrets(ctx context.Context, client *kubiya.Client, tool *kubiya.Tool) (map[string]string, error) {
	secrets := make(map[string]string)

	// Check tool environment variables for any secret references
	for _, env := range tool.Env {
		if strings.HasPrefix(env, "SECRET_") {
			fmt.Printf("🔐 Fetching secret value for %s...\n", env)
			value, err := client.GetSecretValue(ctx, env)
			if err != nil {
				return nil, fmt.Errorf("failed to get secret %s: %w", env, err)
			}
			secrets[env] = value
			fmt.Printf("✓ Secret %s retrieved successfully\n", env)
		}
	}

	return secrets, nil
}
