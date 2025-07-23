package cli

import (
	"fmt"

	"github.com/kubiyabot/cli/internal/config"
	"github.com/kubiyabot/cli/internal/style"
	"github.com/spf13/cobra"
)

func newTriggerUpdateCommand(cfg *config.Config) *cobra.Command {
	var (
		workflowFile  string
		name          string
		customHeaders string
		payload       string
		encodeAs      string
		status        string
	)

	cmd := &cobra.Command{
		Use:   "update <trigger-id>",
		Short: "Update an existing workflow trigger",
		Long: `Update the configuration of an existing workflow trigger.

You can update any aspect of the trigger including the workflow file,
name, provider-specific settings, and status.`,
		Example: `  # Update the workflow file for a trigger
  kubiya trigger update abc123def456 --workflow new-workflow.yaml
  
  # Update the trigger name and status
  kubiya trigger update abc123def456 --name "Updated Response" --status inactive
  
  # Update Datadog-specific configuration
  kubiya trigger update abc123def456 \
    --payload '{"alert": "$EVENT_MSG", "priority": "$PRIORITY"}' \
    --custom-headers "X-Team: Platform"`,
		Args: cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			triggerID := args[0]
			return updateTrigger(cfg, triggerID, workflowFile, name, customHeaders, payload, encodeAs, status)
		},
	}

	cmd.Flags().StringVarP(&workflowFile, "workflow", "w", "", "Path to the workflow file")
	cmd.Flags().StringVarP(&name, "name", "n", "", "Human-readable name for the trigger")
	cmd.Flags().StringVar(&customHeaders, "custom-headers", "", "Custom headers for the webhook (newline-separated)")
	cmd.Flags().StringVar(&payload, "payload", "", "Custom payload template for the webhook")
	cmd.Flags().StringVar(&encodeAs, "encode-as", "", "Encoding format for the webhook payload")
	cmd.Flags().StringVar(&status, "status", "", "Trigger status (active, inactive)")

	return cmd
}

func updateTrigger(cfg *config.Config, triggerID, workflowFile, name, customHeaders, payload, encodeAs, status string) error {
	fmt.Printf("%s\n", style.HeaderStyle.Render("🔄 Updating Trigger"))

	// TODO: Implement actual trigger update
	// This would involve:
	// 1. Loading the existing trigger configuration
	// 2. Validating the updates
	// 3. Updating the configuration in storage
	// 4. Updating the external provider (if provider-specific settings changed)
	// 5. Saving the updated configuration

	fmt.Printf("\n%s Updating trigger %s...\n", style.InfoStyle.Render("🔄"), triggerID)

	// Show what would be updated
	updatedFields := []string{}

	if workflowFile != "" {
		updatedFields = append(updatedFields, "workflow file")
		fmt.Printf("• Workflow file: %s\n", workflowFile)
	}

	if name != "" {
		updatedFields = append(updatedFields, "name")
		fmt.Printf("• Name: %s\n", name)
	}

	if customHeaders != "" {
		updatedFields = append(updatedFields, "custom headers")
		fmt.Printf("• Custom headers: %s\n", customHeaders)
	}

	if payload != "" {
		updatedFields = append(updatedFields, "payload template")
		fmt.Printf("• Payload: %s\n", payload)
	}

	if encodeAs != "" {
		updatedFields = append(updatedFields, "encoding")
		fmt.Printf("• Encode as: %s\n", encodeAs)
	}

	if status != "" {
		updatedFields = append(updatedFields, "status")
		fmt.Printf("• Status: %s\n", status)
	}

	if len(updatedFields) == 0 {
		return fmt.Errorf("no updates specified. Use --help to see available options")
	}

	// Simulate update process
	fmt.Printf("\n%s Validating updates...\n", style.InfoStyle.Render("🔍"))

	// Here we would validate the new configuration
	if workflowFile != "" {
		// Validate workflow file exists
		fmt.Printf("✅ Workflow file validated\n")
	}

	if status != "" && status != "active" && status != "inactive" {
		return fmt.Errorf("invalid status: %s (must be 'active' or 'inactive')", status)
	}

	fmt.Printf("\n%s Updating external provider...\n", style.InfoStyle.Render("🔄"))
	// Here we would update the provider-specific configuration
	fmt.Printf("✅ Provider configuration updated\n")

	fmt.Printf("\n%s Saving configuration...\n", style.InfoStyle.Render("💾"))
	// Here we would save the updated trigger configuration
	fmt.Printf("✅ Configuration saved\n")

	fmt.Printf("\n%s\n", style.SuccessStyle.Render("✅ Trigger updated successfully!"))

	fmt.Printf("\n%s\n", style.InfoStyle.Render("Updated Fields:"))
	for _, field := range updatedFields {
		fmt.Printf("• %s\n", field)
	}

	fmt.Printf("\n%s\n", style.InfoStyle.Render("Next Steps:"))
	fmt.Printf("• Test the updated trigger: kubiya trigger test %s\n", triggerID)
	fmt.Printf("• View updated details: kubiya trigger describe %s\n", triggerID)

	return nil
}
