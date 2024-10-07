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

var runnersCmd = &cobra.Command{
	Use:   "runners",
	Short: "Manage runners",
	Long:  `Manage runners in the Kubiya platform.`,
}

func init() {
	rootCmd.AddCommand(runnersCmd)
}


// list_runnersCmd represents the list_runners command
var list_runnersCmd = &cobra.Command{
	Use:   "list_runners",
	Short: "Get runners",
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

		
		

		
		
		
		params := kubiya.RunnersApiOpts{}
		
		result, httpResp, err = client.RunnersApi.(
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
	runnersCmd.AddCommand(list_runnersCmd)

	// Capture the command name
	

	
	
}

// get_runners_runner_healthCmd represents the get_runners_runner_health command
var get_runners_runner_healthCmd = &cobra.Command{
	Use:   "get_runners_runner_health",
	Short: "Get runner health",
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

		
		
		
		runnerValue, _ := cmd.Flags().GetString("runner")
		
		

		
		
		
		params := kubiya.RunnersApiOpts{}
		
		
		
		result, httpResp, err = client.RunnersApi.(
			ctx,
			
			
			runnerValue,
			
			
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
	runnersCmd.AddCommand(get_runners_runner_healthCmd)

	// Capture the command name
	

	
	
	
	get_runners_runner_healthCmd.Flags().String("runner", "", "Runner name")
	get_runners_runner_healthCmd.MarkFlagRequired("runner")
	
	
}
