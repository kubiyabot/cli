package cli

import (
	"fmt"
	"os"
	"os/exec"
	"runtime"
	"strings"

	"github.com/spf13/cobra"

	"github.com/kubiyabot/cli/internal/config"
	"github.com/kubiyabot/cli/internal/style"
)

// requireAPIKey checks if the API key is present in the config.
// If not, it guides the user or prompts to save if found in env.
// It returns an error if the key is missing and not provided/saved.
func requireAPIKey(cmd *cobra.Command, cfg *config.Config) error {
	if cfg.APIKey != "" {
		return nil // API Key exists, all good.
	}

	// API Key is missing from config, check environment variable
	envAPIKey := os.Getenv("KUBIYA_API_KEY")
	if envAPIKey != "" {
		// Found in environment, but not in config file (or config file doesn't exist yet)
		configPath, _ := config.GetConfigFilePath() // Ignore error getting path for prompt
		fmt.Fprintf(cmd.OutOrStdout(), "🔑 API Key found in environment variable KUBIYA_API_KEY.\n")

		// Check if running interactively (simplistic check for now)
		// A better check might involve checking if stdin is a TTY
		isInteractive := true // Assume interactive unless a specific flag is set?
		if nonInteractiveFlag := cmd.Flags().Lookup("non-interactive"); nonInteractiveFlag != nil {
			if nonInteractiveFlag.Value.String() == "true" {
				isInteractive = false
			}
		}

		if isInteractive {
			fmt.Fprintf(cmd.OutOrStdout(), "💾 Do you want to save it to %s? (Recommended) [Y/n] ", configPath)
			var confirm string
			fmt.Scanln(&confirm)
			if strings.ToLower(confirm) == "n" {
				fmt.Fprintln(cmd.OutOrStdout(), " R Using API key from environment variable for this session only.")
				cfg.APIKey = envAPIKey // Use it for this run
				return nil
			} else {
				// Save the key (using the placeholder function for now)
				if err := config.SaveAPIKey(envAPIKey); err != nil {
					fmt.Fprintf(os.Stderr, " Error saving API Key: %v\n", err)
					fmt.Fprintln(cmd.OutOrStdout(), " R Continuing with API key from environment variable for this session.")
					cfg.APIKey = envAPIKey // Still use it even if save failed
					return nil
				}
				// Successfully saved
				cfg.APIKey = envAPIKey
				return nil
			}
		} else {
			// Non-interactive mode, just use the env var
			fmt.Fprintln(cmd.OutOrStdout(), " R Using API key from environment variable KUBIYA_API_KEY (non-interactive mode).")
			cfg.APIKey = envAPIKey
			return nil
		}
	}

	// API Key is missing entirely
	fmt.Fprintln(os.Stderr, style.ErrorStyle.Render(" Error: Kubiya API Key is required for this command."))
	fmt.Fprintln(os.Stderr, " Please set the KUBIYA_API_KEY environment variable or configure it in ~/.kubiya/config.yaml")
	fmt.Fprintln(os.Stderr, " You can generate an API key at: https://app.kubiya.ai/settings/api-keys")
	return fmt.Errorf("API key not configured")
}

// openUrl opens the specified URL in the default browser based on the OS.
func openUrl(uri string) {
	switch runtime.GOOS {
	case "linux":
		// Try xdg-open first
		if err := exec.Command("xdg-open", uri).Start(); err == nil {
			return
		}
		// Fallback for systems without xdg-open (e.g., WSL without X server)
		_ = exec.Command("sensible-browser", uri).Start()
	case "darwin":
		_ = exec.Command("open", uri).Start()
	case "windows":
		_ = exec.Command("rundll32", "url.dll,FileProtocolHandler", uri).Start()
	default:
		fmt.Printf("Unsupported OS for opening URL automatically: %s\nPlease open manually: %s\n", runtime.GOOS, uri)
	}
}
