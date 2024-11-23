package main

import (
	"fmt"
	"os"

	"github.com/kubiyabot/cli/internal/cli"
	"github.com/kubiyabot/cli/internal/config"
)

func main() {
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
