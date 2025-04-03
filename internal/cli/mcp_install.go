package cli

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"io"
	"os"
	"os/exec"
	"reflect"
	"runtime"
	"strings"

	"github.com/AlecAivazis/survey/v2"
	"github.com/spf13/afero"
	"github.com/spf13/cobra"

	"github.com/kubiyabot/cli/internal/config"
	"github.com/kubiyabot/cli/internal/kubiya"
	"github.com/kubiyabot/cli/internal/mcp"
)

const (
	mcpGatewayRepo = "https://github.com/kubiyabot/mcp-gateway.git"
)

func newMcpInstallCommand(cfg *config.Config, fs afero.Fs) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "install",
		Short: "📥 Install the local Kubiya MCP server and apply default configurations",
		Long: fmt.Sprintf(`Clones the mcp-gateway repository (%s) into ~/.kubiya/mcp-gateway, 
checks dependencies, installs Python packages, and then attempts to automatically 
apply configurations for default providers (like Claude, Cursor) using selected teammates.

If run non-interactively, teammate selection will be skipped during auto-apply.`, mcpGatewayRepo),
		RunE: func(cmd *cobra.Command, args []string) error {
			cmd.SilenceUsage = true
			stdout := cmd.OutOrStdout()
			stderr := cmd.ErrOrStderr()

			fmt.Fprintln(stdout, "🚀 Starting MCP server installation...")

			// 1. Check for git
			fmt.Fprint(stdout, "Checking for git... ")
			gitExists, err := commandExists(fs, "git", stdout)
			if err != nil {
				fmt.Fprintf(stderr, "Error checking for git: %v\n", err)
				return err
			}
			if !gitExists {
				fmt.Fprintln(stdout, "❌ Not found.")
				fmt.Fprintln(stderr, "Error: 'git' command not found in PATH.")
				fmt.Fprintln(stderr, "Please install git and ensure it's available in your system's PATH.")
				return fmt.Errorf("git not found")
			}
			fmt.Fprintln(stdout, "✅ Found.")

			// 2. Get target directory
			gatewayDir, err := mcp.GetMcpGatewayDir()
			if err != nil {
				return fmt.Errorf("failed to determine mcp-gateway directory: %w", err)
			}
			fmt.Fprintf(stdout, "Target directory: %s\n", gatewayDir)

			// 3. Check if directory exists
			if _, err := fs.Stat(gatewayDir); err == nil {
				fmt.Fprintf(stdout, "🟡 Target directory already exists. Installation skipped.")
				fmt.Fprintf(stdout, " Use 'kubiya mcp update' to check for updates or 'kubiya mcp apply' to configure applications.")
				// TODO: Maybe add a --force flag later?
				return nil // Not an error, just already installed
			} else if !os.IsNotExist(err) {
				// Real error accessing the path
				return fmt.Errorf("failed to check target directory '%s': %w", gatewayDir, err)
			}

			// 4. Clone the repository
			fmt.Fprintf(stdout, "Cloning %s...\n", mcpGatewayRepo)
			cloneCmd := exec.Command("git", "clone", mcpGatewayRepo, gatewayDir)
			if err := runCommand(cloneCmd, stdout, stderr); err != nil {
				return fmt.Errorf("failed to clone repository: %w", err)
			}
			fmt.Fprintln(stdout, "✅ Repository cloned successfully.")

			// 5. Check for uv
			fmt.Fprint(stdout, "Checking for uv (Python package installer)... ")
			uvExists, err := commandExists(fs, "uv", stdout)
			if err != nil {
				fmt.Fprintf(stderr, "Error checking for uv: %v\n", err)
				return err
			}
			if !uvExists {
				fmt.Fprintln(stdout, "❌ Not found.")
				fmt.Fprintln(stderr, "Error: 'uv' command not found in PATH.")
				fmt.Fprintln(stderr, "uv is required to install the MCP server dependencies.")
				fmt.Fprintln(stderr, "Installation instructions: https://github.com/astral-sh/uv")
				// Attempt to provide OS-specific hints
				switch runtime.GOOS {
				case "linux", "darwin":
					fmt.Fprintln(stderr, "You might be able to install it via: curl -LsSf https://astral.sh/uv/install.sh | sh")
				case "windows":
					fmt.Fprintln(stderr, "You might be able to install it via: powershell -c \"irm https://astral.sh/uv/install.ps1 | iex\"")
				}
				return fmt.Errorf("uv not found")
			}
			fmt.Fprintln(stdout, "✅ Found.")

			// 6. Install dependencies
			fmt.Fprintln(stdout, "Installing Python dependencies using uv...")
			installCmd := exec.Command("uv", "sync")
			installCmd.Dir = gatewayDir
			if err := runCommand(installCmd, stdout, stderr); err != nil {
				return fmt.Errorf("failed to install dependencies: %w", err)
			}
			fmt.Fprintln(stdout, "✅ Dependencies installed successfully.")

			// 7. Get and store SHA
			fmt.Fprint(stdout, "Storing current version information... ")
			shaCmd := exec.Command("git", "rev-parse", "HEAD")
			shaCmd.Dir = gatewayDir
			var shaOut bytes.Buffer
			var shaErr bytes.Buffer
			shaCmd.Stdout = &shaOut
			shaCmd.Stderr = &shaErr
			if err := shaCmd.Run(); err != nil {
				fmt.Fprintln(stdout, "❌ Failed.")
				fmt.Fprintf(stderr, "Error getting git SHA: %v\nStderr: %s", err, shaErr.String())
				fmt.Fprintln(stderr, "Warning: Could not determine repository version. Update checks may not function correctly.")
			} else {
				currentSHA := strings.TrimSpace(shaOut.String())
				if err := mcp.StoreGatewaySHA(fs, currentSHA); err != nil {
					fmt.Fprintln(stdout, "❌ Failed.")
					fmt.Fprintf(stderr, "Error storing gateway SHA: %v\n", err)
					fmt.Fprintln(stderr, "Warning: Could not store repository version. Update checks may not function correctly.")
				} else {
					fmt.Fprintln(stdout, "✅ Stored.")
				}
			}

			// 8. Auto-apply default configurations
			fmt.Fprintln(stdout, "\n⚙️ Applying default MCP configurations...")

			// Check TTY for interactivity
			fileInfo, _ := os.Stdout.Stat()
			isTty := (fileInfo.Mode() & os.ModeCharDevice) != 0
			// TODO: Add a --non-interactive flag to explicitly disable this?
			isInteractive := isTty

			var selectedTeammateUUIDs []string

			if cfg.APIKey == "" {
				fmt.Fprintln(stdout, "🟡 Skipping teammate selection: Kubiya API key not configured.")
				fmt.Fprintln(stdout, "   Configurations will be applied without specific teammates.")
				fmt.Fprintln(stdout, "   Run 'kubiya config set api-key YOUR_API_KEY' and 'kubiya mcp apply <provider>' to customize.")
			} else {
				// Fetch Teammates (regardless of interactivity, needed for 'Select All')
				fmt.Fprint(stdout, "  Fetching teammates... ")
				client := kubiya.NewClient(cfg)
				teammates, err := client.ListTeammates(context.Background())
				if err != nil {
					fmt.Fprintln(stdout, "❌ Error fetching teammates.")
					fmt.Fprintf(stderr, "  Warning: Could not fetch teammates: %v\n", err)
					fmt.Fprintln(stderr, "  Configurations will be applied without specific teammates.")
				} else if len(teammates) == 0 {
					fmt.Fprintln(stdout, "🟡 No teammates found.")
					fmt.Fprintln(stderr, "  Warning: No teammates found in workspace. Configurations applied without specific teammates.")
				} else {
					fmt.Fprintf(stdout, "✅ Found %d teammates.\n", len(teammates))

					// Interactive Selection
					if isInteractive {
						fmt.Println("  Select teammates for default MCP configurations:")

						allUUIDs := make([]string, len(teammates))
						options := make([]string, len(teammates)+1)
						optionMap := make(map[string]string) // Map display string back to UUID
						options[0] = "(Select All)"          // Add Select All option
						for i, tm := range teammates {
							display := fmt.Sprintf("%s (%s)", tm.Name, tm.UUID)
							options[i+1] = display
							optionMap[display] = tm.UUID
							allUUIDs[i] = tm.UUID
						}

						var selectedOptions []string
						prompt := &survey.MultiSelect{
							Message:  "Teammates to enable:",
							Options:  options,
							Help:     `Selected teammates' context will be available via the local MCP server for default applications.`,
							PageSize: 15,
						}
						err = survey.AskOne(prompt, &selectedOptions, survey.WithKeepFilter(true))
						if err != nil {
							fmt.Fprintln(stdout, "🟡 Teammate selection cancelled or failed. Applying configs without specific teammates.")
							selectedTeammateUUIDs = []string{} // Ensure empty slice
						} else {
							// Process selection
							selectAllChosen := false
							for _, opt := range selectedOptions {
								if opt == "(Select All)" {
									selectAllChosen = true
									break
								}
							}

							if selectAllChosen {
								selectedTeammateUUIDs = allUUIDs
								fmt.Fprintf(stdout, "  Selected all %d teammates.\n", len(allUUIDs))
								// Optional: Add warning if len(allUUIDs) > 15 ?
							} else {
								for _, opt := range selectedOptions {
									if uuid, ok := optionMap[opt]; ok {
										selectedTeammateUUIDs = append(selectedTeammateUUIDs, uuid)
									}
								}
								fmt.Fprintf(stdout, "  Selected %d teammate(s).\n", len(selectedTeammateUUIDs))
							}
						}
					} else {
						// Non-interactive: Use empty list (no specific teammates)
						fmt.Fprintln(stdout, "  Running non-interactively. Applying configs without specific teammates.")
						selectedTeammateUUIDs = []string{}
					}
				}
			}

			// Apply configurations using the selected (or empty) teammate list
			mcpConfigDir, err := mcp.GetMcpConfigDir(fs)
			if err != nil {
				fmt.Fprintf(stderr, "  Warning: Could not access MCP config directory (%s) for auto-apply: %v\n", mcpConfigDir, err)
			} else {
				files, err := afero.ReadDir(fs, mcpConfigDir)
				if err != nil {
					fmt.Fprintf(stderr, "  Warning: Could not list files in MCP config directory (%s) for auto-apply: %v\n", mcpConfigDir, err)
				} else {
					fmt.Fprintln(stdout, "  Checking providers in", mcpConfigDir)
					appliedCount := 0
					for _, file := range files {
						if !file.IsDir() && strings.HasSuffix(file.Name(), ".yaml") {
							providerName := strings.TrimSuffix(file.Name(), ".yaml")
							// Call the refactored apply logic for each provider with selected teammates
							if err := applyProviderConfiguration(providerName, cfg, fs, stdout, stderr, selectedTeammateUUIDs); err != nil {
								fmt.Fprintf(stderr, "  Error applying config for %s: %v\n", providerName, err)
							} else {
								appliedCount++
							}
						}
					}
					if appliedCount == 0 {
						fmt.Fprintln(stdout, "  No applicable default provider configurations found or applied.")
						// Don't suggest manual apply here, as we just finished install
					}
				}
			}

			fmt.Fprintln(stdout, "\n✨ MCP server installation complete!")
			fmt.Fprintln(stdout, "   Default configurations (if applicable) have been automatically applied.")
			fmt.Fprintln(stdout, "   You can manage configurations further using 'kubiya mcp apply/edit/list'.")

			return nil
		},
	}
	// Add flags later if needed (e.g., --non-interactive, --force)
	return cmd
}

// commandExists checks if a command is available in the system's PATH.
func commandExists(fs afero.Fs, cmdName string, stdout io.Writer) (bool, error) {
	// Use the mockable LookPath function
	_, err := exec.LookPath(cmdName)
	if err != nil {
		if errors.Is(err, exec.ErrNotFound) {
			return false, nil // Command not found is not an error for this check
		}
		// Other errors (e.g., permission issues) should be reported
		return false, fmt.Errorf("failed to check for command '%s': %w", cmdName, err)
	}
	return true, nil
}

// runCommand executes a command and prints its output directly.
// Assumes cmd was created using the 'execCommand' variable.
func runCommand(cmd *exec.Cmd, stdout, stderr io.Writer) error {
	// Check if we are in mock mode by comparing function pointers
	isMocking := execCommand != nil && reflect.ValueOf(execCommand).Pointer() == reflect.ValueOf(mockExecCommand).Pointer()

	if isMocking {
		muCmdCalls.Lock()
		callKey := fmt.Sprintf("%s %s", cmd.Path, strings.Join(cmd.Args[1:], " "))
		recordedCmdCalls = append(recordedCmdCalls, commandCall{Name: cmd.Path, Args: cmd.Args[1:], Dir: cmd.Dir})

		mockErr, hasError := mockErrors[callKey]
		mockOut, _ := mockOutputs[callKey]
		muCmdCalls.Unlock()

		if hasError {
			// Maybe write mock output to stderr if error occurs?
			// For now, just return the error.
			return mockErr
		}
		if mockOut != "" && stdout != nil {
			_, _ = io.WriteString(stdout, mockOut)
		}
		return nil // Mocked execution succeeded
	}

	// --- Original Execution Logic ---
	cmd.Stdout = stdout
	cmd.Stderr = stderr

	// Start and Wait will be mocked during tests via cmd.Run override
	if err := cmd.Start(); err != nil {
		fmt.Fprintf(stderr, "Error starting command '%s': %v\n", cmd.String(), err)
		return err
	}

	if err := cmd.Wait(); err != nil {
		fmt.Fprintf(stderr, "Error running command '%s': %v\n", cmd.String(), err)
		return err
	}
	return nil
}

// runCommandCapture executes a command and captures its stdout and stderr.
// Assumes cmd was created using the 'execCommand' variable.
func runCommandCapture(cmd *exec.Cmd) (string, string, error) {
	// Check if we are in mock mode
	isMocking := execCommand != nil && reflect.ValueOf(execCommand).Pointer() == reflect.ValueOf(mockExecCommand).Pointer()

	if isMocking {
		muCmdCalls.Lock()
		callKey := fmt.Sprintf("%s %s", cmd.Path, strings.Join(cmd.Args[1:], " "))
		recordedCmdCalls = append(recordedCmdCalls, commandCall{Name: cmd.Path, Args: cmd.Args[1:], Dir: cmd.Dir})

		mockErr, hasError := mockErrors[callKey]
		mockOut, _ := mockOutputs[callKey]
		// Mock stderr isn't explicitly handled yet, return empty for now
		muCmdCalls.Unlock()

		if hasError {
			// Return mock output (if any) and mock error, similar to real execution failure
			stderrMsg := ""
			if mockErr != nil {
				stderrMsg = mockErr.Error()
			}
			return mockOut, stderrMsg, mockErr
		}
		// Success case for mock
		return mockOut, "", nil
	}

	// --- Original Execution Logic ---
	var stdoutBuf, stderrBuf bytes.Buffer
	cmd.Stdout = &stdoutBuf
	cmd.Stderr = &stderrBuf

	// cmd.Run() will be the mocked version during tests
	err := cmd.Run()

	stdoutStr := stdoutBuf.String()
	stderrStr := stderrBuf.String()

	if err != nil {
		// Return captured stdout/stderr even on error
		return stdoutStr, stderrStr, fmt.Errorf("command '%s' failed: %w\nstderr: %s", cmd.String(), err, stderrStr)
	}

	return stdoutStr, stderrStr, nil
}
