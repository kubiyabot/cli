package main

import (
	"fmt"
	"os"
	"path/filepath"

	"github.com/kubiyabot/cli/internal/cli"
	"github.com/kubiyabot/cli/internal/config"
)

func main() {
	// Ensure config directory exists
	configDir, err := os.UserConfigDir()
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error getting config directory: %v\n", err)
		os.Exit(1)
	}

	kubiyaDir := filepath.Join(configDir, "kubiya")
	if err := os.MkdirAll(kubiyaDir, 0755); err != nil {
		fmt.Fprintf(os.Stderr, "Error creating config directory: %v\n", err)
		os.Exit(1)
	}

	// Load the configuration
	cfg, err := config.Load()
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error loading config: %v\n", err)
		os.Exit(1)
	}

	// Execute with config
	if err := cli.Execute(cfg); err != nil {
		fmt.Fprintf(os.Stderr, "Error: %v\n", err)
		os.Exit(1)
	}
}
