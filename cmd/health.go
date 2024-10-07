package cmd

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"strconv"

	"github.com/spf13/cobra"
	"github.com/spf13/viper"

	"github.com/kubiyabot/cli/kubiya"
)

var healthCmd = &cobra.Command{
	Use:   "health",
	Short: "Manage health",
	Long:  `Manage health in the Kubiya platform.`,
}

func init() {
	rootCmd.AddCommand(healthCmd)
}


// get_versionCmd represents the get_version command
var get_versionCmd = &cobra.Command{
	Use:   "get_version",
	Short: "Get API version",
	Long:  ``,
	Run: func(cmd *cobra.Command, args []string) {
		cfg := kubiya.NewConfiguration()
		cfg.Host = viper.GetString("api-url")

		apiKey := viper.GetString("api-key")
		if apiKey == "" {
			fmt.Println("API Key is required. Set it using the --api-key flag or KUBIYA_API_KEY environment variable.")
			return
		}
		cfg.AddDefaultHeader("Authorization", "Api-Key "+apiKey)

		client := kubiya.NewAPIClient(cfg)

		var result interface{}
		var httpResp *http.Response
		var err error

		// Prepare the context
		ctx := context.Background()

		
		

		
		
		
		params := kubiya.HealthApiOpts{}
		
		result, httpResp, err = client.HealthApi.(
			ctx,
			
			&params,
		)
		

		if err != nil {
			fmt.Printf("Error: %v\n", err)
			return
		}

		if httpResp != nil {
			defer httpResp.Body.Close()
		}

		jsonResult, _ := json.MarshalIndent(result, "", "  ")
		fmt.Printf("Response: %s\n", string(jsonResult))
	},
}

func init() {
	healthCmd.AddCommand(get_versionCmd)

	// Capture the command name
	

	
	
}

// get_whoamiCmd represents the get_whoami command
var get_whoamiCmd = &cobra.Command{
	Use:   "get_whoami",
	Short: "Get user information",
	Long:  ``,
	Run: func(cmd *cobra.Command, args []string) {
		cfg := kubiya.NewConfiguration()
		cfg.Host = viper.GetString("api-url")

		apiKey := viper.GetString("api-key")
		if apiKey == "" {
			fmt.Println("API Key is required. Set it using the --api-key flag or KUBIYA_API_KEY environment variable.")
			return
		}
		cfg.AddDefaultHeader("Authorization", "Api-Key "+apiKey)

		client := kubiya.NewAPIClient(cfg)

		var result interface{}
		var httpResp *http.Response
		var err error

		// Prepare the context
		ctx := context.Background()

		
		

		
		
		
		params := kubiya.HealthApiOpts{}
		
		result, httpResp, err = client.HealthApi.(
			ctx,
			
			&params,
		)
		

		if err != nil {
			fmt.Printf("Error: %v\n", err)
			return
		}

		if httpResp != nil {
			defer httpResp.Body.Close()
		}

		jsonResult, _ := json.MarshalIndent(result, "", "  ")
		fmt.Printf("Response: %s\n", string(jsonResult))
	},
}

func init() {
	healthCmd.AddCommand(get_whoamiCmd)

	// Capture the command name
	

	
	
}
