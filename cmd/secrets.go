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

var secretsCmd = &cobra.Command{
	Use:   "secrets",
	Short: "Manage secrets",
	Long:  `Manage secrets in the Kubiya platform.`,
}

func init() {
	rootCmd.AddCommand(secretsCmd)
}


// list_secretsCmd represents the list_secrets command
var list_secretsCmd = &cobra.Command{
	Use:   "list_secrets",
	Short: "Get secrets",
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

		
		

		
		
		
		params := kubiya.SecretsApiOpts{}
		
		result, httpResp, err = client.SecretsApi.(
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
	secretsCmd.AddCommand(list_secretsCmd)

	// Capture the command name
	

	
	
}

// create_secretsCmd represents the create_secrets command
var create_secretsCmd = &cobra.Command{
	Use:   "create_secrets",
	Short: "Create secrets",
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

		
		
		
		bodyValue, _ := cmd.Flags().GetString("body")
		
		

		
		
		// Prepare request body
		
		requestBody := kubiya.Request{
			
			
			
			
			
		}
		
		result, httpResp, err = client.SecretsApi.(
			ctx,
			
			requestBody,
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
	secretsCmd.AddCommand(create_secretsCmd)

	// Capture the command name
	

	
	
	
	
	create_secretsCmd.Flags().String("body", "", "")
	create_secretsCmd.MarkFlagRequired("body")
	
	
}
