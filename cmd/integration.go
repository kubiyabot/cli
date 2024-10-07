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

var integrationCmd = &cobra.Command{
	Use:   "integration",
	Short: "Manage integration",
	Long:  `Manage integration in the Kubiya platform.`,
}

func init() {
	rootCmd.AddCommand(integrationCmd)
}


// get_integrations_vendorCmd represents the get_integrations_vendor command
var get_integrations_vendorCmd = &cobra.Command{
	Use:   "get_integrations_vendor",
	Short: "Get vendor integration",
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

		
		
		
		vendorValue, _ := cmd.Flags().GetString("vendor")
		
		

		
		
		
		params := kubiya.IntegrationApiOpts{}
		
		
		
		result, httpResp, err = client.IntegrationApi.(
			ctx,
			
			
			vendorValue,
			
			
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
	integrationCmd.AddCommand(get_integrations_vendorCmd)

	// Capture the command name
	

	
	
	
	get_integrations_vendorCmd.Flags().String("vendor", "", "Vendor name")
	get_integrations_vendorCmd.MarkFlagRequired("vendor")
	
	
}

// delete_integrations_vendorCmd represents the delete_integrations_vendor command
var delete_integrations_vendorCmd = &cobra.Command{
	Use:   "delete_integrations_vendor",
	Short: "Delete vendor integration",
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

		
		
		
		vendorValue, _ := cmd.Flags().GetString("vendor")
		
		

		
		
		result, httpResp, err = client.IntegrationApi.(
			ctx,
			
			
			vendorValue,
			
			
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
	integrationCmd.AddCommand(delete_integrations_vendorCmd)

	// Capture the command name
	

	
	
	
	delete_integrations_vendorCmd.Flags().String("vendor", "", "Vendor name")
	delete_integrations_vendorCmd.MarkFlagRequired("vendor")
	
	
}

// create_integrations_vendor_statusCmd represents the create_integrations_vendor_status command
var create_integrations_vendor_statusCmd = &cobra.Command{
	Use:   "create_integrations_vendor_status",
	Short: "Update vendor integration status",
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

		
		
		
		vendorValue, _ := cmd.Flags().GetString("vendor")
		
		
		
		bodyValue, _ := cmd.Flags().GetString("body")
		
		

		
		
		// Prepare request body
		
		requestBody := kubiya.Request{
			
			
			
			
			
			
			
		}
		
		result, httpResp, err = client.IntegrationApi.(
			ctx,
			
			
			
			vendorValue,
			
			
			
			
			
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
	integrationCmd.AddCommand(create_integrations_vendor_statusCmd)

	// Capture the command name
	

	
	
	
	create_integrations_vendor_statusCmd.Flags().String("vendor", "", "Vendor name")
	create_integrations_vendor_statusCmd.MarkFlagRequired("vendor")
	
	
	
	
	create_integrations_vendor_statusCmd.Flags().String("body", "", "Status update")
	create_integrations_vendor_statusCmd.MarkFlagRequired("body")
	
	
}

// list_integrationsCmd represents the list_integrations command
var list_integrationsCmd = &cobra.Command{
	Use:   "list_integrations",
	Short: "Get integrations",
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

		
		

		
		
		
		params := kubiya.IntegrationApiOpts{}
		
		result, httpResp, err = client.IntegrationApi.(
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
	integrationCmd.AddCommand(list_integrationsCmd)

	// Capture the command name
	

	
	
}
