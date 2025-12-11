package cli

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"

	"github.com/kubiyabot/cli/internal/composer"
	"github.com/kubiyabot/cli/internal/config"
	"github.com/kubiyabot/cli/internal/style"
	"github.com/spf13/cobra"
)

// newWorkflowRunCommand creates a command to execute stored workflows by ID/name
func newWorkflowRunCommand(cfg *config.Config) *cobra.Command {
	var (
		variables []string
		runner    string
		verbose   bool
		jsonOutput bool
	)

	cmd := &cobra.Command{
		Use:   "run [workflow-id-or-name]",
		Short: "Execute a stored workflow by ID or name",
		Long: `Execute a workflow that is stored in the Kubiya system by its ID or name.

This command retrieves the workflow from the composer API and executes it,
similar to clicking the "test" button in the Composer UI. The workflow
must already exist in your organization.

The Composer API is automatically accessed at https://compose.kubiya.ai/api.
You can override this with the KUBIYA_COMPOSER_URL environment variable.

🔑 Authentication:
  Recommended: kubiya login (interactive setup)
  Alternative: export KUBIYA_API_KEY=your-key
  Get API key: https://compose.kubiya.ai/settings#apiKeys`,
		Example: `  # Execute a workflow by ID
  kubiya workflow run abc123-def456-789

  # Execute a workflow by name (if unique)
  kubiya workflow run "My Deploy Workflow"

  # Execute with input parameters
  kubiya workflow run deploy-prod --var env=production --var version=v1.2.3

  # Execute with verbose output
  kubiya workflow run my-workflow --verbose

  # Execute and output JSON
  kubiya workflow run my-workflow --json`,
		Args: cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			// Check for authentication before making API calls
			if err := checkAuthentication(cfg); err != nil {
				return err
			}

			ctx := context.Background()
			comp := composer.NewClient(cfg)
			w := cmd.OutOrStdout()

			workflowIdentifier := args[0]

			// Parse variables
			input := make(map[string]interface{})
			for _, v := range variables {
				parts := strings.SplitN(v, "=", 2)
				if len(parts) == 2 {
					input[parts[0]] = parts[1]
				}
			}

			// Find workflow by ID or name
			workflow, err := findWorkflowByIdentifier(ctx, comp, workflowIdentifier)
			if err != nil {
				return err
			}

			if !jsonOutput {
				fmt.Fprintf(w, "%s Executing workflow: %s\n",
					style.InfoStyle.Render("🚀"), style.HighlightStyle.Render(workflow.Name))
				if workflow.Description != "" {
					fmt.Fprintf(w, "%s Description: %s\n",
						style.DimStyle.Render("ℹ️"), workflow.Description)
				}
				if len(input) > 0 {
					fmt.Fprintf(w, "%s Input parameters:\n", style.DimStyle.Render("📝"))
					for k, v := range input {
						fmt.Fprintf(w, "  • %s = %v\n", style.HighlightStyle.Render(k), v)
					}
				}
				fmt.Fprintln(w)
			}

			// Execute the workflow
			execResp, err := comp.ExecuteWorkflow(ctx, workflow.ID, composer.WorkflowExecuteParams{
				Input:  input,
				Runner: runner, // Will be set to kubiya-hosted in the composer client
			})
			if err != nil {
				// Provide helpful authentication guidance
				if strings.Contains(err.Error(), "401") || strings.Contains(err.Error(), "Unauthorized") {
					return fmt.Errorf(`authentication failed - you need to set up authentication first!

🔑 Recommended: Use interactive authentication
   kubiya login

💻 Alternative: Set API key manually
   export KUBIYA_API_KEY=your-api-key

🌐 Get your API key from: https://compose.kubiya.ai/settings#apiKeys

Note: For automation/CI, use KUBIYA_API_KEY environment variable
For interactive use, 'kubiya login' provides a better experience`)
				}
				return fmt.Errorf("failed to execute workflow: %w", err)
			}

			if jsonOutput {
				// Output execution response as JSON
				enc := json.NewEncoder(w)
				enc.SetIndent("", "  ")
				return enc.Encode(map[string]interface{}{
					"workflow_id":    workflow.ID,
					"workflow_name":  workflow.Name,
					"execution_id":   execResp.ExecutionID,
					"request_id":     execResp.RequestID,
					"status":         execResp.Status,
					"message":        execResp.Message,
					"stream_url":     execResp.StreamURL,
					"status_url":     execResp.StatusURL,
					"input":          input,
				})
			}

			// Display execution information
			fmt.Fprintf(w, "%s Workflow execution started successfully!\n",
				style.SuccessStyle.Render("✅"))
			fmt.Fprintf(w, "  • Execution ID: %s\n", style.HighlightStyle.Render(execResp.ExecutionID))
			if execResp.RequestID != "" {
				fmt.Fprintf(w, "  • Request ID: %s\n", style.HighlightStyle.Render(execResp.RequestID))
			}
			fmt.Fprintf(w, "  • Status: %s\n", execResp.Status)
			fmt.Fprintf(w, "  • Runner: kubiya-hosted\n")

			if verbose {
				fmt.Fprintf(w, "\n%s Execution Details:\n", style.TitleStyle.Render("📊"))
				fmt.Fprintf(w, "  • Stream URL: %s\n", execResp.StreamURL)
				fmt.Fprintf(w, "  • Status URL: %s\n", execResp.StatusURL)
			}

			fmt.Fprintf(w, "\n%s To monitor execution progress:\n", style.InfoStyle.Render("👀"))
			fmt.Fprintf(w, "  kubiya workflow execution list --id %s\n", workflow.ID)

			return nil
		},
	}

	cmd.Flags().StringArrayVar(&variables, "var", nil, "Input parameters in key=value format")
	cmd.Flags().StringVar(&runner, "runner", "", "Runner to use (will be overridden to kubiya-hosted)")
	cmd.Flags().BoolVarP(&verbose, "verbose", "v", false, "Enable verbose output")
	cmd.Flags().BoolVar(&jsonOutput, "json", false, "Output execution details as JSON")

	return cmd
}

// findWorkflowByIdentifier finds a workflow by ID or name
func findWorkflowByIdentifier(ctx context.Context, comp *composer.Client, identifier string) (*composer.Workflow, error) {
	// First try to get by ID directly
	workflow, err := comp.GetWorkflow(ctx, identifier)
	if err == nil {
		return workflow, nil
	}

	// If that fails, search by name
	workflows, err := comp.ListWorkflows(ctx, composer.WorkflowParams{
		Search: identifier,
		Limit:  100, // Search more broadly
	})
	if err != nil {
		return nil, fmt.Errorf("failed to search for workflow: %w", err)
	}

	// Look for exact name match
	var matches []*composer.Workflow
	for i := range workflows.Workflows {
		if workflows.Workflows[i].Name == identifier {
			matches = append(matches, &workflows.Workflows[i])
		}
	}

	// If no exact match, look for partial matches
	if len(matches) == 0 {
		for i := range workflows.Workflows {
			if strings.Contains(strings.ToLower(workflows.Workflows[i].Name), strings.ToLower(identifier)) {
				matches = append(matches, &workflows.Workflows[i])
			}
		}
	}

	if len(matches) == 0 {
		return nil, fmt.Errorf("no workflow found with ID or name '%s'. Use 'kubiya workflow list' to see available workflows", identifier)
	}

	if len(matches) > 1 {
		fmt.Printf("%s Multiple workflows match '%s':\n", style.WarningStyle.Render("⚠️"), identifier)
		for _, wf := range matches {
			fmt.Printf("  • %s (%s) - %s\n",
				style.HighlightStyle.Render(wf.Name),
				wf.ID,
				wf.Status)
		}
		return nil, fmt.Errorf("multiple workflows found. Please use the exact workflow ID")
	}

	return matches[0], nil
}