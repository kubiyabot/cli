package main

import (
	"context"
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
	// Initialize Sentry early for comprehensive tracing
	if err := sentry.Initialize(version.GetVersion()); err != nil {
		fmt.Fprintf(os.Stderr, "Warning: Failed to initialize Sentry: %v\n", err)
		// Continue execution even if Sentry fails
	}

	// Ensure Sentry events are flushed on exit
	defer sentry.Flush(2 * time.Second)

	// Recover from any panics and report to Sentry
	defer sentry.RecoverWithSentry(context.Background(), map[string]interface{}{
		"main": "kubiya_cli_main",
	})

	// Add breadcrumb for CLI startup
	sentry.AddBreadcrumb("cli.startup", "Kubiya CLI starting", map[string]interface{}{
		"version": version.GetVersion(),
		"args":    os.Args,
	})

	// Ensure config directory exists
	configDir, err := os.UserConfigDir()
	if err != nil {
		sentry.CaptureError(err, map[string]string{
			"error.type": "config_directory",
		}, map[string]interface{}{
			"operation": "get_user_config_dir",
		})
		fmt.Fprintf(os.Stderr, "Error getting config directory: %v\n", err)
		os.Exit(1)
	}

	kubiyaDir := filepath.Join(configDir, "kubiya")
	if err := os.MkdirAll(kubiyaDir, 0755); err != nil {
		sentry.CaptureError(err, map[string]string{
			"error.type": "config_directory_creation",
		}, map[string]interface{}{
			"config_dir": kubiyaDir,
		})
		fmt.Fprintf(os.Stderr, "Error creating config directory: %v\n", err)
		os.Exit(1)
	}

	// Load the configuration
	cfg, err := config.Load()
	if err != nil {
		sentry.CaptureError(err, map[string]string{
			"error.type": "config_load",
		}, map[string]interface{}{
			"config_dir": kubiyaDir,
		})
		fmt.Fprintf(os.Stderr, "Error loading config: %v\n", err)
		os.Exit(1)
	}

	// Execute with config - wrapped in Sentry transaction
	ctx := context.Background()
	err = sentry.WithCLICommand(ctx, "main", os.Args[1:], func(ctx context.Context) error {
		return cli.Execute(cfg)
	})

	if err != nil {
		sentry.CaptureError(err, map[string]string{
			"error.type": "cli_execution",
		}, map[string]interface{}{
			"command": os.Args,
		})
		fmt.Fprintf(os.Stderr, "Error: %v\n", err)
		os.Exit(1)
	}
}
