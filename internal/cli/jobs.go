package cli

import (
	"encoding/json"
	"fmt"
	"text/tabwriter"
	"time"

	"github.com/kubiyabot/cli/internal/config"
	"github.com/kubiyabot/cli/internal/controlplane"
	"github.com/kubiyabot/cli/internal/controlplane/entities"
	"github.com/kubiyabot/cli/internal/style"
	"github.com/spf13/cobra"
)

func newJobCommand(cfg *config.Config) *cobra.Command {
	cmd := &cobra.Command{
		Use:     "job",
		Aliases: []string{"jobs", "j"},
		Short:   "⏰ Manage scheduled jobs",
		Long: `Manage scheduled and recurring jobs for agents and teams.

Jobs allow you to schedule tasks to run automatically at specified intervals
or trigger them manually via webhooks.`,
		Example: `  # List all jobs
  kubiya job list

  # Create a new job
  kubiya job create --name "daily-report" --agent <agent-id> --schedule "0 9 * * *" --prompt "Generate daily report"

  # Get job details
  kubiya job get <job-id>

  # Trigger a job manually
  kubiya job trigger <job-id>

  # Enable/disable a job
  kubiya job enable <job-id>
  kubiya job disable <job-id>

  # View job executions
  kubiya job executions <job-id>`,
	}

	cmd.AddCommand(
		newListJobsCommand(cfg),
		newGetJobCommand(cfg),
		newCreateJobCommand(cfg),
		newUpdateJobCommand(cfg),
		newDeleteJobCommand(cfg),
		newTriggerJobCommand(cfg),
		newEnableJobCommand(cfg),
		newDisableJobCommand(cfg),
		newJobExecutionsCommand(cfg),
	)

	return cmd
}

func newListJobsCommand(cfg *config.Config) *cobra.Command {
	var outputFormat string

	cmd := &cobra.Command{
		Use:     "list",
		Aliases: []string{"ls", "l"},
		Short:   "📋 List all jobs",
		Example: `  # List all jobs
  kubiya job list

  # List all jobs in JSON format
  kubiya job list -o json`,
		RunE: func(cmd *cobra.Command, args []string) error {
			client, err := controlplane.New(cfg.APIKey, cfg.Debug)
			if err != nil {
				return fmt.Errorf("failed to create client: %w", err)
			}

			jobs, err := client.ListJobs()
			if err != nil {
				return fmt.Errorf("failed to list jobs: %w", err)
			}

			if len(jobs) == 0 {
				if outputFormat == "json" {
					fmt.Println("[]")
					return nil
				}
				fmt.Println(style.CreateHelpBox("No jobs found"))
				return nil
			}

			// Handle output format
			switch outputFormat {
			case "json":
				encoder := json.NewEncoder(cmd.OutOrStdout())
				encoder.SetIndent("", "  ")
				return encoder.Encode(jobs)
			default:
				// Beautiful header
				fmt.Println()
				fmt.Println(style.CreateBanner(fmt.Sprintf("Scheduled Jobs (%d)", len(jobs)), "⏰"))
				fmt.Println()

				// Display in table format
				w := tabwriter.NewWriter(cmd.OutOrStdout(), 0, 0, 3, ' ', 0)
				fmt.Fprintln(w, style.TableHeaderStyle.Render("ID\tNAME\tENTITY\tSCHEDULE\tENABLED\tNEXT RUN"))
				fmt.Fprintln(w, style.CreateDivider(100))

				for _, job := range jobs {
					id := truncateID(job.ID)
					name := job.Name
					if len(name) > 30 {
						name = name[:27] + "..."
					}

					// Determine entity type and ID
					entity := "-"
					if job.AgentID != nil {
						entity = "agent:" + truncateID(*job.AgentID)
					} else if job.TeamID != nil {
						entity = "team:" + truncateID(*job.TeamID)
					}

					schedule := "-"
					if job.Schedule != nil {
						schedule = *job.Schedule
						if len(schedule) > 20 {
							schedule = schedule[:17] + "..."
						}
					}

					enabled := "❌"
					if job.Enabled {
						enabled = "✅"
					}

					nextRun := "-"
					if job.NextRunAt != nil {
						nextRun = job.NextRunAt.Format("2006-01-02 15:04")
					}

					fmt.Fprintf(w, "%s\t%s\t%s\t%s\t%s\t%s\n",
						style.ValueStyle.Render(id),
						style.InfoStyle.Render(name),
						style.DimStyle.Render(entity),
						style.MetadataValueStyle.Render(schedule),
						enabled,
						style.DimStyle.Render(nextRun))
				}

				w.Flush()
				fmt.Println()

				return nil
			}
		},
	}

	cmd.Flags().StringVarP(&outputFormat, "output", "o", "text", "Output format (text|json)")

	return cmd
}

func newGetJobCommand(cfg *config.Config) *cobra.Command {
	cmd := &cobra.Command{
		Use:     "get <job-id>",
		Aliases: []string{"show", "describe"},
		Short:   "🔍 Get job details",
		Args:    cobra.ExactArgs(1),
		Example: `  # Get job details
  kubiya job get <job-id>`,
		RunE: func(cmd *cobra.Command, args []string) error {
			jobID := args[0]

			client, err := controlplane.New(cfg.APIKey, cfg.Debug)
			if err != nil {
				return fmt.Errorf("failed to create client: %w", err)
			}

			job, err := client.GetJob(jobID)
			if err != nil {
				return fmt.Errorf("failed to get job: %w", err)
			}

			// Beautiful banner
			fmt.Println()
			fmt.Println(style.CreateBanner("Job Details", "⏰"))
			fmt.Println()

			// Basic info
			info := map[string]string{
				"ID":      job.ID,
				"Name":    job.Name,
				"Enabled": fmt.Sprintf("%t", job.Enabled),
				"Prompt":  truncateString(job.Prompt, 60),
			}

			if job.Description != nil {
				info["Description"] = *job.Description
			}

			if job.AgentID != nil {
				info["Agent ID"] = *job.AgentID
			}

			if job.TeamID != nil {
				info["Team ID"] = *job.TeamID
			}

			if job.Schedule != nil {
				info["Schedule"] = *job.Schedule
			}

			if job.Timezone != nil {
				info["Timezone"] = *job.Timezone
			}

			if job.WebhookPath != nil {
				info["Webhook Path"] = *job.WebhookPath
			}

			if job.CreatedAt != nil {
				info["Created"] = job.CreatedAt.Format(time.RFC3339)
			}

			if job.UpdatedAt != nil {
				info["Updated"] = job.UpdatedAt.Format(time.RFC3339)
			}

			if job.LastRunAt != nil {
				info["Last Run"] = job.LastRunAt.Format(time.RFC3339)
			}

			if job.NextRunAt != nil {
				info["Next Run"] = job.NextRunAt.Format(time.RFC3339)
			}

			fmt.Println(style.CreateMetadataBox(info))
			fmt.Println()

			return nil
		},
	}

	return cmd
}

func newCreateJobCommand(cfg *config.Config) *cobra.Command {
	var (
		name        string
		description string
		agentID     string
		teamID      string
		schedule    string
		timezone    string
		prompt      string
		enabled     bool
		webhookPath string
	)

	cmd := &cobra.Command{
		Use:   "create",
		Short: "➕ Create a new job",
		Example: `  # Create a scheduled job for an agent
  kubiya job create --name "daily-report" --agent <agent-id> --schedule "0 9 * * *" --prompt "Generate daily report"

  # Create a webhook-triggered job
  kubiya job create --name "on-demand" --team <team-id> --webhook "/my-webhook" --prompt "Process webhook data"

  # Create a disabled job
  kubiya job create --name "future-job" --agent <agent-id> --schedule "0 0 * * 0" --prompt "Weekly task" --enabled=false`,
		RunE: func(cmd *cobra.Command, args []string) error {
			if name == "" {
				return fmt.Errorf("name is required (--name)")
			}
			if prompt == "" {
				return fmt.Errorf("prompt is required (--prompt)")
			}
			if agentID == "" && teamID == "" {
				return fmt.Errorf("either --agent or --team must be specified")
			}
			if agentID != "" && teamID != "" {
				return fmt.Errorf("cannot specify both --agent and --team")
			}

			client, err := controlplane.New(cfg.APIKey, cfg.Debug)
			if err != nil {
				return fmt.Errorf("failed to create client: %w", err)
			}

			req := &entities.CreateJobRequest{
				Name:    name,
				Prompt:  prompt,
				Enabled: &enabled,
			}

			if description != "" {
				req.Description = &description
			}

			if agentID != "" {
				req.AgentID = &agentID
			}

			if teamID != "" {
				req.TeamID = &teamID
			}

			if schedule != "" {
				req.Schedule = &schedule
			}

			if timezone != "" {
				req.Timezone = &timezone
			}

			if webhookPath != "" {
				req.WebhookPath = &webhookPath
			}

			job, err := client.CreateJob(req)
			if err != nil {
				return fmt.Errorf("failed to create job: %w", err)
			}

			fmt.Println()
			fmt.Println(style.CreateSuccessBox(fmt.Sprintf("Job created successfully: %s", job.ID)))
			fmt.Println()

			// Show job details
			info := map[string]string{
				"ID":      job.ID,
				"Name":    job.Name,
				"Enabled": fmt.Sprintf("%t", job.Enabled),
			}

			if job.Schedule != nil {
				info["Schedule"] = *job.Schedule
			}

			if job.NextRunAt != nil {
				info["Next Run"] = job.NextRunAt.Format(time.RFC3339)
			}

			fmt.Println(style.CreateMetadataBox(info))
			fmt.Println()

			return nil
		},
	}

	cmd.Flags().StringVar(&name, "name", "", "Job name (required)")
	cmd.Flags().StringVar(&description, "description", "", "Job description")
	cmd.Flags().StringVar(&agentID, "agent", "", "Agent ID (mutually exclusive with --team)")
	cmd.Flags().StringVar(&teamID, "team", "", "Team ID (mutually exclusive with --agent)")
	cmd.Flags().StringVar(&schedule, "schedule", "", "Cron schedule expression (e.g., '0 9 * * *')")
	cmd.Flags().StringVar(&timezone, "timezone", "", "Timezone for schedule (e.g., 'America/New_York')")
	cmd.Flags().StringVar(&prompt, "prompt", "", "Task prompt to execute (required)")
	cmd.Flags().BoolVar(&enabled, "enabled", true, "Enable job immediately")
	cmd.Flags().StringVar(&webhookPath, "webhook", "", "Webhook path for triggering job")

	return cmd
}

func newUpdateJobCommand(cfg *config.Config) *cobra.Command {
	var (
		name        string
		description string
		schedule    string
		timezone    string
		prompt      string
	)

	cmd := &cobra.Command{
		Use:   "update <job-id>",
		Short: "✏️  Update a job",
		Args:  cobra.ExactArgs(1),
		Example: `  # Update job schedule
  kubiya job update <job-id> --schedule "0 10 * * *"

  # Update job prompt
  kubiya job update <job-id> --prompt "New task description"

  # Update job name
  kubiya job update <job-id> --name "New Job Name"`,
		RunE: func(cmd *cobra.Command, args []string) error {
			jobID := args[0]

			client, err := controlplane.New(cfg.APIKey, cfg.Debug)
			if err != nil {
				return fmt.Errorf("failed to create client: %w", err)
			}

			req := &entities.UpdateJobRequest{}

			if name != "" {
				req.Name = &name
			}

			if description != "" {
				req.Description = &description
			}

			if schedule != "" {
				req.Schedule = &schedule
			}

			if timezone != "" {
				req.Timezone = &timezone
			}

			if prompt != "" {
				req.Prompt = &prompt
			}

			job, err := client.UpdateJob(jobID, req)
			if err != nil {
				return fmt.Errorf("failed to update job: %w", err)
			}

			fmt.Println()
			fmt.Println(style.CreateSuccessBox("Job updated successfully"))
			fmt.Println()

			// Show updated job info
			info := map[string]string{
				"ID":      job.ID,
				"Name":    job.Name,
				"Enabled": fmt.Sprintf("%t", job.Enabled),
			}

			if job.Schedule != nil {
				info["Schedule"] = *job.Schedule
			}

			if job.NextRunAt != nil {
				info["Next Run"] = job.NextRunAt.Format(time.RFC3339)
			}

			fmt.Println(style.CreateMetadataBox(info))
			fmt.Println()

			return nil
		},
	}

	cmd.Flags().StringVar(&name, "name", "", "Job name")
	cmd.Flags().StringVar(&description, "description", "", "Job description")
	cmd.Flags().StringVar(&schedule, "schedule", "", "Cron schedule expression")
	cmd.Flags().StringVar(&timezone, "timezone", "", "Timezone for schedule")
	cmd.Flags().StringVar(&prompt, "prompt", "", "Task prompt to execute")

	return cmd
}

func newDeleteJobCommand(cfg *config.Config) *cobra.Command {
	cmd := &cobra.Command{
		Use:     "delete <job-id>",
		Aliases: []string{"del", "rm"},
		Short:   "🗑️  Delete a job",
		Args:    cobra.ExactArgs(1),
		Example: `  # Delete a job
  kubiya job delete <job-id>`,
		RunE: func(cmd *cobra.Command, args []string) error {
			jobID := args[0]

			client, err := controlplane.New(cfg.APIKey, cfg.Debug)
			if err != nil {
				return fmt.Errorf("failed to create client: %w", err)
			}

			if err := client.DeleteJob(jobID); err != nil {
				return fmt.Errorf("failed to delete job: %w", err)
			}

			fmt.Println()
			fmt.Println(style.CreateSuccessBox(fmt.Sprintf("Job deleted: %s", jobID)))
			fmt.Println()

			return nil
		},
	}

	return cmd
}

func newTriggerJobCommand(cfg *config.Config) *cobra.Command {
	cmd := &cobra.Command{
		Use:     "trigger <job-id>",
		Aliases: []string{"run", "execute"},
		Short:   "▶️  Manually trigger a job",
		Args:    cobra.ExactArgs(1),
		Example: `  # Trigger a job manually
  kubiya job trigger <job-id>`,
		RunE: func(cmd *cobra.Command, args []string) error {
			jobID := args[0]

			client, err := controlplane.New(cfg.APIKey, cfg.Debug)
			if err != nil {
				return fmt.Errorf("failed to create client: %w", err)
			}

			response, err := client.TriggerJob(jobID)
			if err != nil {
				return fmt.Errorf("failed to trigger job: %w", err)
			}

			fmt.Println()
			fmt.Println(style.CreateSuccessBox("Job triggered successfully"))
			fmt.Println()

			info := map[string]string{
				"Job ID":       response.JobID,
				"Execution ID": response.ExecutionID,
				"Status":       response.Status,
			}

			fmt.Println(style.CreateMetadataBox(info))
			fmt.Println()

			fmt.Println(style.CreateHelpBox(fmt.Sprintf("💡 View execution logs:\n   kubiya execution logs %s", response.ExecutionID)))
			fmt.Println()

			return nil
		},
	}

	return cmd
}

func newEnableJobCommand(cfg *config.Config) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "enable <job-id>",
		Short: "✅ Enable a job",
		Args:  cobra.ExactArgs(1),
		Example: `  # Enable a job
  kubiya job enable <job-id>`,
		RunE: func(cmd *cobra.Command, args []string) error {
			jobID := args[0]

			client, err := controlplane.New(cfg.APIKey, cfg.Debug)
			if err != nil {
				return fmt.Errorf("failed to create client: %w", err)
			}

			job, err := client.EnableJob(jobID)
			if err != nil {
				return fmt.Errorf("failed to enable job: %w", err)
			}

			fmt.Println()
			fmt.Println(style.CreateSuccessBox(fmt.Sprintf("Job enabled: %s", job.Name)))
			fmt.Println()

			if job.NextRunAt != nil {
				fmt.Println(style.CreateHelpBox(fmt.Sprintf("⏰ Next run: %s", job.NextRunAt.Format(time.RFC3339))))
				fmt.Println()
			}

			return nil
		},
	}

	return cmd
}

func newDisableJobCommand(cfg *config.Config) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "disable <job-id>",
		Short: "❌ Disable a job",
		Args:  cobra.ExactArgs(1),
		Example: `  # Disable a job
  kubiya job disable <job-id>`,
		RunE: func(cmd *cobra.Command, args []string) error {
			jobID := args[0]

			client, err := controlplane.New(cfg.APIKey, cfg.Debug)
			if err != nil {
				return fmt.Errorf("failed to create client: %w", err)
			}

			job, err := client.DisableJob(jobID)
			if err != nil {
				return fmt.Errorf("failed to disable job: %w", err)
			}

			fmt.Println()
			fmt.Println(style.CreateSuccessBox(fmt.Sprintf("Job disabled: %s", job.Name)))
			fmt.Println()

			return nil
		},
	}

	return cmd
}

func newJobExecutionsCommand(cfg *config.Config) *cobra.Command {
	cmd := &cobra.Command{
		Use:     "executions <job-id>",
		Aliases: []string{"exec", "runs"},
		Short:   "📊 View job execution history",
		Args:    cobra.ExactArgs(1),
		Example: `  # View executions for a job
  kubiya job executions <job-id>`,
		RunE: func(cmd *cobra.Command, args []string) error {
			jobID := args[0]

			client, err := controlplane.New(cfg.APIKey, cfg.Debug)
			if err != nil {
				return fmt.Errorf("failed to create client: %w", err)
			}

			response, err := client.GetJobExecutions(jobID)
			if err != nil {
				return fmt.Errorf("failed to get job executions: %w", err)
			}

			if len(response.Executions) == 0 {
				fmt.Println(style.CreateHelpBox("No executions found for this job"))
				return nil
			}

			// Beautiful header
			fmt.Println()
			fmt.Println(style.CreateBanner(fmt.Sprintf("Job Executions (%d)", response.Total), "📊"))
			fmt.Println()

			// Display in table format
			w := tabwriter.NewWriter(cmd.OutOrStdout(), 0, 0, 3, ' ', 0)
			fmt.Fprintln(w, style.TableHeaderStyle.Render("EXECUTION ID\tSTATUS\tCREATED\tCOMPLETED"))
			fmt.Fprintln(w, style.CreateDivider(80))

			for _, exec := range response.Executions {
				id := truncateID(exec.GetID())

				createdAt := "N/A"
				if exec.CreatedAt != nil {
					createdAt = exec.CreatedAt.Format("2006-01-02 15:04")
				}

				completedAt := "-"
				if exec.CompletedAt != nil {
					completedAt = exec.CompletedAt.Format("2006-01-02 15:04")
				}

				fmt.Fprintf(w, "%s\t%s\t%s\t%s\n",
					style.ValueStyle.Render(id),
					style.CreateStatusBadge(string(exec.Status)),
					style.DimStyle.Render(createdAt),
					style.DimStyle.Render(completedAt))
			}

			w.Flush()
			fmt.Println()

			return nil
		},
	}

	return cmd
}

