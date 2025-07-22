package main

import (
	"fmt"
	"os"
	"path/filepath"
	"time"

	"github.com/kubiyabot/cli/internal/cli"
	"github.com/kubiyabot/cli/internal/config"
	"github.com/kubiyabot/cli/internal/sentry"
	"github.com/kubiyabot/cli/internal/version"
)

func main() {
	// Initialize Sentry for error and performance monitoring
	if err := sentry.Initialize(version.GetVersion()); err != nil {
		// Log but don't fail if Sentry initialization fails
		fmt.Fprintf(os.Stderr, "Warning: Failed to initialize Sentry: %v\n", err)
	}
	// Ensure all events are sent before program exits
	defer sentry.Flush(2 * time.Second)

	// Ensure config directory exists
	configDir, err := os.UserConfigDir()
	if err != nil {
		sentry.CaptureError(err, map[string]string{
			"operation": "get_config_directory",
		}, map[string]interface{}{
			"config_dir": configDir,
		})
		fmt.Fprintf(os.Stderr, "Error getting config directory: %v\n", err)
		os.Exit(1)
	}

	kubiyaDir := filepath.Join(configDir, "kubiya")
	if err := os.MkdirAll(kubiyaDir, 0755); err != nil {
		sentry.CaptureError(err, map[string]string{
			"operation": "create_config_directory",
		}, map[string]interface{}{
			"kubiya_dir": kubiyaDir,
		})
		fmt.Fprintf(os.Stderr, "Error creating config directory: %v\n", err)
		os.Exit(1)
	}

	// Load the configuration
	cfg, err := config.Load()
	if err != nil {
		sentry.CaptureError(err, map[string]string{
			"operation": "load_config",
		}, map[string]interface{}{
			"config_dir": kubiyaDir,
		})
		fmt.Fprintf(os.Stderr, "Error loading config: %v\n", err)
		os.Exit(1)
	}

	// Execute with config and capture any top-level errors
	if err := cli.Execute(cfg); err != nil {
		sentry.CaptureError(err, map[string]string{
			"operation": "cli_execute",
		}, map[string]interface{}{
			"args": os.Args,
		})
		fmt.Fprintf(os.Stderr, "Error: %v\n", err)
		os.Exit(1)
	}
}
