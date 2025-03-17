package main

import (
	"fmt"
	"os"

	"github.com/kubiyabot/cli/internal/cli"
	"github.com/kubiyabot/cli/internal/config"
	"github.com/kubiyabot/cli/internal/version"
)

// These variables are set via ldflags during build
var (
	versionStr = "dev"
	commit     = "unknown"
	date       = "unknown"
)

func main() {
	// Set version information from ldflags
	if versionStr != "dev" {
		version.Version = versionStr
		version.SetBuildInfo(commit, date, "goreleaser")
	}

	cfg, err := config.Load()
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error loading config: %v\n", err)
		os.Exit(1)
	}

	if err := cli.Execute(cfg); err != nil {
		fmt.Fprintf(os.Stderr, "Error: %v\n", err)
		os.Exit(1)
	}
}
