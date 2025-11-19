package cli

import (
	"context"
	"fmt"
	"text/tabwriter"
	"time"

	"github.com/kubiyabot/cli/internal/config"
	"github.com/kubiyabot/cli/internal/controlplane"
	"github.com/kubiyabot/cli/internal/style"
	"github.com/spf13/cobra"
)

func newExecutionCommand(cfg *config.Config) *cobra.Command {
	cmd := &cobra.Command{
		Use:     "execution",
		Aliases: []string{"executions", "exec"},
		Short:   "📊 Manage agent and team executions",
		Long: `View, monitor, and manage agent and team executions (jobs).

Executions represent tasks that have been submitted to agents or teams.
You can list all executions, view details, stream logs, and cancel running executions.`,
		Example: `  # List all executions
  kubiya execution list

  # List executions with filters
  kubiya execution list --status running
  kubiya execution list --agent <agent-id>
  kubiya execution list --limit 50

  # Get execution details
  kubiya execution get <execution-id>

  # Stream execution logs in real-time
  kubiya execution logs <execution-id>

  # Cancel a running execution
  kubiya execution cancel <execution-id>`,
	}

	cmd.AddCommand(
		newListExecutionsCommand(cfg),
		newGetExecutionCommand(cfg),
		newExecutionLogsCommand(cfg),
		newCancelExecutionCommand(cfg),
	)

	return cmd
}

func newListExecutionsCommand(cfg *config.Config) *cobra.Command {
	var (
		status        string
		agentID       string
		teamID        string
		executionType string
		limit         int
	)

	cmd := &cobra.Command{
		Use:     "list",
		Aliases: []string{"ls", "l"},
		Short:   "📋 List all executions",
		Long: `List all agent and team executions with optional filters.

Executions can be filtered by status, agent, team, or execution type.`,
		Example: `  # List all executions
  kubiya execution list

  # List running executions
  kubiya execution list --status running

  # List executions for specific agent
  kubiya execution list --agent 8064f4c8-fb5c-4a52-99f8-9075521500a3

  # List team executions
  kubiya execution list --type team

  # Limit results
  kubiya execution list --limit 20`,
		RunE: func(cmd *cobra.Command, args []string) error {
			client, err := controlplane.New(cfg.APIKey, cfg.Debug)
			if err != nil {
				return fmt.Errorf("failed to create client: %w", err)
			}

			// Build filters
			filters := make(map[string]string)
			if status != "" {
				filters["status"] = status
			}
			if agentID != "" {
				filters["agent_id"] = agentID
			}
			if teamID != "" {
				filters["team_id"] = teamID
			}
			if executionType != "" {
				filters["execution_type"] = executionType
			}
			if limit > 0 {
				filters["limit"] = fmt.Sprintf("%d", limit)
			}

			executions, err := client.ListExecutions(filters)
			if err != nil {
				return fmt.Errorf("failed to list executions: %w", err)
			}

			if len(executions) == 0 {
				fmt.Println(style.CreateHelpBox("No executions found matching the criteria"))
				return nil
			}

			// Beautiful header
			fmt.Println()
			fmt.Println(style.CreateBanner(fmt.Sprintf("Executions (%d)", len(executions)), "📊"))
			fmt.Println()

			// Display in table format
			w := tabwriter.NewWriter(cmd.OutOrStdout(), 0, 0, 3, ' ', 0)
			fmt.Fprintln(w, style.TableHeaderStyle.Render("ID\tTYPE\tENTITY\tSTATUS\tCREATED"))
			fmt.Fprintln(w, style.CreateDivider(80))

			for _, exec := range executions {
				// Truncate IDs for display
				id := truncateID(exec.GetID())
				entityID := truncateID(exec.EntityID)

				// Format creation time
				createdAt := "N/A"
				if exec.CreatedAt != nil {
					createdAt = exec.CreatedAt.Format("2006-01-02 15:04")
				}

				// Display row
				fmt.Fprintf(w, "%s\t%s\t%s\t%s\t%s\n",
					style.ValueStyle.Render(id),
					style.InfoStyle.Render(exec.ExecutionType),
					style.DimStyle.Render(entityID),
					style.CreateStatusBadge(string(exec.Status)),
					style.DimStyle.Render(createdAt))
			}

			w.Flush()
			fmt.Println()

			return nil
		},
	}

	cmd.Flags().StringVarP(&status, "status", "s", "", "Filter by status (pending, running, completed, failed)")
	cmd.Flags().StringVar(&agentID, "agent", "", "Filter by agent ID")
	cmd.Flags().StringVar(&teamID, "team", "", "Filter by team ID")
	cmd.Flags().StringVar(&executionType, "type", "", "Filter by type (agent, team)")
	cmd.Flags().IntVar(&limit, "limit", 50, "Limit number of results")

	return cmd
}

func newGetExecutionCommand(cfg *config.Config) *cobra.Command {
	cmd := &cobra.Command{
		Use:     "get <execution-id>",
		Aliases: []string{"show", "describe"},
		Short:   "🔍 Get execution details",
		Long:    `Display detailed information about a specific execution.`,
		Example: `  # Get execution details
  kubiya execution get abc123-def456-ghi789

  # Get with full execution ID
  kubiya execution get 8064f4c8-fb5c-4a52-99f8-9075521500a3`,
		Args: cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			executionID := args[0]

			client, err := controlplane.New(cfg.APIKey, cfg.Debug)
			if err != nil {
				return fmt.Errorf("failed to create client: %w", err)
			}

			execution, err := client.GetExecution(executionID)
			if err != nil {
				return fmt.Errorf("failed to get execution: %w", err)
			}

			// Beautiful banner
			fmt.Println()
			fmt.Println(style.CreateBanner("Execution Details", "📊"))
			fmt.Println()

			// Basic info
			info := map[string]string{
				"ID":             execution.GetID(),
				"Type":           execution.ExecutionType,
				"Entity ID":      execution.EntityID,
				"Status":         string(execution.Status),
				"Prompt":         truncateString(execution.Prompt, 60),
			}

			if execution.WorkflowID != nil {
				info["Workflow ID"] = *execution.WorkflowID
			}

			if execution.CreatedAt != nil {
				info["Created"] = execution.CreatedAt.Format(time.RFC3339)
			}

			if execution.StartedAt != nil {
				info["Started"] = execution.StartedAt.Format(time.RFC3339)
			}

			if execution.CompletedAt != nil {
				info["Completed"] = execution.CompletedAt.Format(time.RFC3339)
			}

			fmt.Println(style.CreateMetadataBox(info))
			fmt.Println()

			// Response (if available)
			if execution.Response != nil && *execution.Response != "" {
				fmt.Println(style.SectionStyle.Render("Response:"))
				fmt.Println(style.ResponseContainerStyle.Render(*execution.Response))
				fmt.Println()
			}

			// Error (if any)
			if execution.ErrorMessage != nil && *execution.ErrorMessage != "" {
				fmt.Println(style.CreateErrorBox(*execution.ErrorMessage))
				fmt.Println()
			}

			// Usage stats (if available)
			if execution.Usage != nil && len(execution.Usage) > 0 {
				fmt.Println(style.SectionStyle.Render("Usage Stats:"))
				for key, value := range execution.Usage {
					fmt.Printf("  %s %v\n",
						style.MetadataKeyStyle.Render(key+":"),
						style.MetadataValueStyle.Render(fmt.Sprint(value)))
				}
				fmt.Println()
			}

			return nil
		},
	}

	return cmd
}

func newExecutionLogsCommand(cfg *config.Config) *cobra.Command {
	cmd := &cobra.Command{
		Use:     "logs <execution-id>",
		Aliases: []string{"log", "stream", "watch"},
		Short:   "📜 Stream execution logs in real-time",
		Long: `Stream execution logs and events in real-time.

This command connects to the execution stream and displays output as it happens.
Use Ctrl+C to stop streaming.`,
		Example: `  # Stream execution logs
  kubiya execution logs abc123-def456

  # Watch execution progress
  kubiya execution logs 8064f4c8-fb5c-4a52-99f8-9075521500a3`,
		Args: cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			executionID := args[0]

			client, err := controlplane.New(cfg.APIKey, cfg.Debug)
			if err != nil {
				return fmt.Errorf("failed to create client: %w", err)
			}

			// Get execution info first
			execution, err := client.GetExecution(executionID)
			if err != nil {
				return fmt.Errorf("failed to get execution: %w", err)
			}

			// Beautiful header
			fmt.Println()
			fmt.Println(style.CreateBanner(fmt.Sprintf("Streaming Logs: %s", truncateID(executionID)), "📜"))
			fmt.Println()

			// Show execution info
			info := map[string]string{
				"Type":   execution.ExecutionType,
				"Status": string(execution.Status),
			}
			fmt.Println(style.CreateMetadataBox(info))
			fmt.Println()
			fmt.Println(style.CreateDivider(80))
			fmt.Println()

			// Stream logs
			ctx, cancel := context.WithCancel(cmd.Context())
			defer cancel()

			eventChan, errChan := client.StreamExecutionOutput(ctx, executionID)

			for {
				select {
				case event, ok := <-eventChan:
					if !ok {
						fmt.Println()
						fmt.Println(style.CreateSuccessBox("Stream closed"))
						return nil
					}

					switch event.Type {
					case "chunk":
						fmt.Print(style.OutputStyle.Render(event.Content))
					case "error":
						fmt.Println()
						fmt.Println(style.CreateErrorBox(event.Content))
						return nil
					case "complete":
						fmt.Println()
						fmt.Println(style.CreateSuccessBox("Execution completed"))
						return nil
					case "status":
						if event.Status != nil {
							fmt.Printf(" %s ", style.CreateStatusBadge(string(*event.Status)))
						}
					}

				case err := <-errChan:
					if err != nil {
						fmt.Println()
						fmt.Println(style.CreateErrorBox(fmt.Sprintf("Stream error: %v", err)))
						return err
					}

				case <-ctx.Done():
					fmt.Println()
					fmt.Println(style.CreateWarningBox("Streaming interrupted"))
					return nil
				}
			}
		},
	}

	return cmd
}

func newCancelExecutionCommand(cfg *config.Config) *cobra.Command {
	cmd := &cobra.Command{
		Use:     "cancel <execution-id>",
		Aliases: []string{"stop", "abort", "kill"},
		Short:   "🛑 Cancel a running execution",
		Long: `Cancel a running execution.

This will attempt to gracefully stop the execution. Note that some executions
may take time to fully terminate.`,
		Example: `  # Cancel execution
  kubiya execution cancel abc123-def456

  # Stop running execution
  kubiya execution cancel 8064f4c8-fb5c-4a52-99f8-9075521500a3`,
		Args: cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			executionID := args[0]

			fmt.Println()
			fmt.Println(style.CreateWarningBox(fmt.Sprintf("Cancellation requested for execution: %s", executionID)))
			fmt.Println()
			fmt.Println(style.CreateHelpBox("Note: The Control Plane API does not currently support execution cancellation.\nTo stop an execution, you can interrupt it at the worker level or wait for timeout."))
			fmt.Println()

			// TODO: Implement when API supports it
			// For now, just show a message

			return nil
		},
	}

	return cmd
}

// Helper functions

func truncateID(id string) string {
	if len(id) <= 12 {
		return id
	}
	return id[:12] + "..."
}
