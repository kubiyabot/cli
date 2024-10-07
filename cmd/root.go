package cmd

import (
    "fmt"
    "os"

    "github.com/spf13/cobra"
)

var rootCmd = &cobra.Command{
    Use:   "kubiya-cli",
    Short: "Kubiya CLI",
    Long:  `Kubiya CLI is a command line interface for interacting with the Kubiya API.`,
}

func Execute() {
    if err := rootCmd.Execute(); err != nil {
        fmt.Println(err)
        os.Exit(1)
    }
}

func Root() *cobra.Command {
    return rootCmd
}