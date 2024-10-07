package main

import (
	"fmt"
	"os"

	"github.com/kubiyabot/cli/cmd"
	"github.com/spf13/cobra"
	"github.com/spf13/viper"
)

var (
	version = "1.0.0"
)

func main() {
	// Add global flags
	cmd.Root().PersistentFlags().String("api-key", "", "API key for authentication")
	cmd.Root().PersistentFlags().String("api-url", "https://api.kubiya.com", "Base URL for the Kubiya API")

	// Bind flags to viper
	viper.BindPFlag("api-key", cmd.Root().PersistentFlags().Lookup("api-key"))
	viper.BindPFlag("api-url", cmd.Root().PersistentFlags().Lookup("api-url"))

	// Add a version command
	cmd.Root().AddCommand(&cobra.Command{
		Use:   "version",
		Short: "Print the version number of Kubiya CLI",
		Run: func(cmd *cobra.Command, args []string) {
			fmt.Printf("Kubiya CLI v%s\n", version)
		},
	})

	if err := cmd.Root().Execute(); err != nil {
		fmt.Printf("Error: %v\n", err)
		os.Exit(1)
	}
}
