package cli

import (
	"errors"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"strings"

	"github.com/kubiyabot/cli/internal/config"
	"github.com/kubiyabot/cli/internal/mcp"
	"github.com/spf13/afero"
	"github.com/spf13/cobra"
)

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

// Accept fs afero.Fs (for consistency, though not directly used in RunE)
func newMcpEditCommand(cfg *config.Config, fs afero.Fs) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "edit <provider_name>",
		Short: "✏️ Edit the YAML configuration file for an MCP provider",
		Long: `Opens the specified provider's YAML configuration file 
(located in ~/.kubiya/mcp/) in your default editor ($EDITOR).`,
		Args: cobra.ExactArgs(1), // Requires provider name
		RunE: func(cmd *cobra.Command, args []string) error {
			cmd.SilenceUsage = true
			stdout := cmd.OutOrStdout()
			stderr := cmd.ErrOrStderr()
			providerName := args[0]

			// 1. Find the config file path (uses mcp.GetMcpConfigDir, needs fs)
			mcpDir, err := mcp.GetMcpConfigDir(fs) // Pass fs
			if err != nil {
				return fmt.Errorf("failed to get MCP config directory: %w", err)
			}
			providerName = strings.TrimSuffix(providerName, ".yaml") // Allow name with or without .yaml
			configFilename := providerName + ".yaml"
			configPath := filepath.Join(mcpDir, configFilename)

			// 2. Check if the config file exists (use fs.Stat)
			if _, err := fs.Stat(configPath); os.IsNotExist(err) { // Use fs
				fmt.Fprintf(stderr, "Error: Configuration file for provider '%s' not found at %s\n", providerName, configPath)
				fmt.Fprintln(stderr, "Available providers can be listed with 'kubiya mcp list'.")
				return fmt.Errorf("provider config file not found: %s", configFilename)
			} else if err != nil {
				return fmt.Errorf("failed checking provider config file '%s': %w", configPath, err)
			}

			// 3. Get the editor
			editor := os.Getenv("EDITOR")
			if editor == "" {
				// Try common defaults if EDITOR is not set
				vimExists, err := commandExists(fs, "vim", stdout) // Pass fs, stdout; get bool & err
				if err != nil {
					fmt.Fprintf(stderr, "Warning: Failed to check for vim: %v\n", err)
				} else if vimExists {
					editor = "vim"
				} else {
					nanoExists, err := commandExists(fs, "nano", stdout) // Pass fs, stdout; get bool & err
					if err != nil {
						fmt.Fprintf(stderr, "Warning: Failed to check for nano: %v\n", err)
					} else if nanoExists {
						editor = "nano"
					} else {
						codeExists, err := commandExists(fs, "code", stdout) // Pass fs, stdout; get bool & err
						if err != nil {
							fmt.Fprintf(stderr, "Warning: Failed to check for code: %v\n", err)
						} else if codeExists {
							editor = "code"
						}
					}
				}
				// Add more editors? (emacs, etc.)

				if editor == "" {
					fmt.Fprintln(stderr, "Error: $EDITOR environment variable is not set, and no default editor (vim, nano, code) found.")
					fmt.Fprintln(stderr, "Please set the EDITOR environment variable to your preferred text editor.")
					fmt.Fprintln(stderr, "Example: export EDITOR=vim")
					return fmt.Errorf("$EDITOR not set and no default found")
				}
				fmt.Fprintf(stdout, "$EDITOR not set, using default editor: %s\n", editor)
			}

			// 4. Execute the editor command
			fmt.Fprintf(stdout, "Opening %s with %s...\n", configPath, editor)
			editCmd := exec.Command(editor, configPath)

			// Connect the command directly to the terminal streams for interactive editing
			editCmd.Stdin = os.Stdin
			editCmd.Stdout = os.Stdout
			editCmd.Stderr = os.Stderr

			if err := editCmd.Run(); err != nil { // Use Run() directly
				// Error message is usually printed by the editor itself to stderr
				// We just need to return a generic error
				// fmt.Fprintf(stderr, "Editor command '%s' failed: %v\n", editor, err)
				return fmt.Errorf("editor command failed")
			}

			fmt.Fprintln(stdout, "Editor closed.")
			return nil
		},
	}
	return cmd
}
