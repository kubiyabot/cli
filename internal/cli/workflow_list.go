package cli

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"strconv"
	"strings"
	"text/tabwriter"
	"time"

	"github.com/kubiyabot/cli/internal/composer"
	"github.com/kubiyabot/cli/internal/config"
	"github.com/kubiyabot/cli/internal/style"
	"github.com/spf13/cobra"
)

type WorkflowListParams struct {
	Request    composer.WorkflowParams
	JsonOutput bool
	WorkflowID string
}

type WorkflowDetailsOutput struct {
	Name          string `json:"name"`
	Description   string `json:"description"`
	Status        string `json:"status"`
	LastExecution string `json:"last_execution"`
	CreatedBy     string `json:"created_by"`
	CreatedAt     string `json:"created_at"`
	UpdatedAt     string `json:"updated_at"`
}

func (o *WorkflowDetailsOutput) PrintJSON(w io.Writer) error {
	if w == nil {
		w = os.Stdout
	}
	enc := json.NewEncoder(w)
	enc.SetIndent("", "  ")
	return enc.Encode(o)
}

func (o *WorkflowDetailsOutput) PrintTable(w io.Writer) error {
	if w == nil {
		w = os.Stdout
	}
	fmt.Fprintf(w, "\n%s\n", style.TitleStyle.Render("📋 WORKFLOW DETAILS"))

	// Enhanced status styling
	statusText := o.Status
	switch strings.ToLower(statusText) {
	case "published":
		statusText = style.SuccessStyle.Render("✓ Published")
	case "draft":
		statusText = style.WarningStyle.Render("✎ Draft")
	case "archived":
		statusText = style.DimStyle.Render("📦 Archived")
	}

	// Format as key-value pairs instead of table
	fmt.Fprintf(w, "\n%s: %s\n", style.DimStyle.Render("Name"), style.HighlightStyle.Render(o.Name))
	fmt.Fprintf(w, "%s: %s\n", style.DimStyle.Render("Status"), statusText)
	if o.Description != "" {
		fmt.Fprintf(w, "%s: %s\n", style.DimStyle.Render("Description"), o.Description)
	}
	if o.CreatedBy != "" {
		fmt.Fprintf(w, "%s: %s\n", style.DimStyle.Render("Created By"), o.CreatedBy)
	}
	if o.CreatedAt != "" {
		fmt.Fprintf(w, "%s: %s\n", style.DimStyle.Render("Created"), formatDate(o.CreatedAt))
	}
	if o.UpdatedAt != "" {
		fmt.Fprintf(w, "%s: %s\n", style.DimStyle.Render("Updated"), formatDate(o.UpdatedAt))
	}
	if o.LastExecution != "" {
		fmt.Fprintf(w, "%s: %s\n", style.DimStyle.Render("Last Execution"), formatDate(o.LastExecution))
	} else {
		fmt.Fprintf(w, "%s: %s\n", style.DimStyle.Render("Last Execution"), style.DimStyle.Render("never"))
	}

	// Add helpful commands
	fmt.Fprintf(w, "\n%s Commands:\n", style.InfoStyle.Render("💡"))
	fmt.Fprintf(w, "  • %s - Execute this workflow\n", style.HighlightStyle.Render("kubiya workflow run \""+o.Name+"\""))
	fmt.Fprintf(w, "  • %s - View executions\n", style.HighlightStyle.Render("kubiya workflow execution list"))

	return nil
}

type WorkflowListOutput struct {
	Workflows  []WorkflowDetailsOutput `json:"workflows"`
	Total      int                     `json:"total"`
	Page       int                     `json:"page"`
	PageSize   int                     `json:"page_size"`
	TotalPages int                     `json:"total_pages"`
}

func (o *WorkflowListOutput) PrintJSON(w io.Writer) error {
	if w == nil {
		w = os.Stdout
	}
	enc := json.NewEncoder(w)
	enc.SetIndent("", "  ")
	return enc.Encode(o)
}

func (o *WorkflowListOutput) PrintTable(w io.Writer) error {
	if w == nil {
		w = os.Stdout
	}
	if len(o.Workflows) == 0 {
		fmt.Fprintf(w, "\n%s No workflows found\n", style.DimStyle.Render("ℹ️"))
		fmt.Fprintf(w, "%s Create your first workflow at: %s\n",
			style.InfoStyle.Render("💡"),
			style.HighlightStyle.Render("https://compose.kubiya.ai"))
		return nil
	}

	// Print header with summary
	fmt.Fprintf(w, "\n%s\n", style.TitleStyle.Render("📋 WORKFLOWS"))
	fmt.Fprintf(w, "%s Found %s workflows (page %d of %d)\n\n",
		style.DimStyle.Render("ℹ️"),
		style.HighlightStyle.Render(fmt.Sprintf("%d", o.Total)),
		o.Page, o.TotalPages)

	// Enhanced table with better spacing and colors
	tw := tabwriter.NewWriter(w, 0, 0, 3, ' ', 0)
	fmt.Fprintln(tw, style.DimStyle.Render("NAME\tSTATUS\tLAST EXECUTION\tCREATED\tDESCRIPTION"))
	fmt.Fprintln(tw, style.DimStyle.Render("────\t──────\t──────────────\t───────\t───────────"))

	for _, wf := range o.Workflows {
		// Enhanced status styling
		statusText := wf.Status
		switch strings.ToLower(statusText) {
		case "published":
			statusText = style.SuccessStyle.Render("✓ published")
		case "draft":
			statusText = style.WarningStyle.Render("✎ draft")
		case "archived":
			statusText = style.DimStyle.Render("📦 archived")
		default:
			statusText = style.DimStyle.Render(statusText)
		}

		// Format dates more nicely
		createdAt := formatDate(wf.CreatedAt)
		lastExecution := formatDate(wf.LastExecution)
		if lastExecution == "" {
			lastExecution = style.DimStyle.Render("never")
		}

		// Truncate long descriptions
		description := wf.Description
		if len(description) > 50 {
			description = description[:47] + "..."
		}
		if description == "" {
			description = style.DimStyle.Render("(no description)")
		}

		fmt.Fprintf(tw, "%s\t%s\t%s\t%s\t%s\n",
			style.HighlightStyle.Render(wf.Name),
			statusText,
			lastExecution,
			createdAt,
			description)
	}

	tw.Flush()

	// Footer with helpful commands
	fmt.Fprintf(w, "\n%s Commands:\n", style.InfoStyle.Render("💡"))
	fmt.Fprintf(w, "  • %s - Execute a workflow\n", style.HighlightStyle.Render("kubiya workflow run <name-or-id>"))
	fmt.Fprintf(w, "  • %s - View executions\n", style.HighlightStyle.Render("kubiya workflow execution list"))
	fmt.Fprintf(w, "  • %s - Next page\n", style.DimStyle.Render("kubiya workflow list --page ")+style.HighlightStyle.Render(fmt.Sprintf("%d", o.Page+1)))

	return nil
}

func newWorkflowListCommand(cfg *config.Config) *cobra.Command {
	var params WorkflowListParams

	cmd := &cobra.Command{
		Use:   "list",
		Short: "List organization workflows (Composer)",
		Long:  `List all workflows in your organization using the Composer API.

The Composer API is automatically accessed at https://compose.kubiya.ai/api.
You can override this with the KUBIYA_COMPOSER_URL environment variable.

🔑 Authentication:
  Recommended: kubiya login (interactive setup)
  Alternative: export KUBIYA_API_KEY=your-key
  Get API key: https://compose.kubiya.ai/settings#apiKeys`,
		Example: `  # List workflows as a table
  kubiya workflow list

  # Output JSON with more items
  kubiya workflow list --json --limit 50

  # Filter by status or search by name
  kubiya workflow list --status published --search deploy`,
		RunE: func(cmd *cobra.Command, args []string) error {
			// Check for authentication before making API calls
			if err := checkAuthentication(cfg); err != nil {
				return err
			}

			ctx := context.Background()
			comp := composer.NewClient(cfg)
			params.Request.SetDefaults()
			if len(params.WorkflowID) == 0 {
				return listWorkflows(ctx, comp, &params, cmd.OutOrStdout())
			} else {
				return getWorkflow(ctx, comp, &params, cmd.OutOrStdout())
			}
		},
	}

	cmd.Flags().BoolVar(&params.JsonOutput, "json", false, "Output as JSON")
	cmd.Flags().IntVar(&params.Request.Page, "page", 1, "Page number")
	cmd.Flags().IntVar(&params.Request.Limit, "limit", 12, "Items per page")
	cmd.Flags().StringVar(&params.Request.Status, "status", "all", "Filter by status (all|draft|published)")
	cmd.Flags().StringVar(&params.Request.Search, "search", "", "Search by workflow name")
	cmd.Flags().StringVar(&params.Request.SortBy, "sort-by", "updated_at", "Sort by field")
	cmd.Flags().StringVar(&params.Request.SortOrder, "sort-order", "desc", "Sort order (asc|desc)")
	cmd.Flags().StringVar(&params.WorkflowID, "id", "", "Workflow ID to fetch a single workflow")

	return cmd
}

func listWorkflows(ctx context.Context, comp *composer.Client, params *WorkflowListParams, w io.Writer) error {
	resp, err := comp.ListWorkflows(ctx, params.Request)
	if err != nil {
		// Provide helpful authentication guidance for common errors
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

		if strings.Contains(err.Error(), "connection") || strings.Contains(err.Error(), "timeout") || strings.Contains(err.Error(), "network") {
			return fmt.Errorf(`network connection failed - check your internet connection

🌐 Trying to connect to: https://compose.kubiya.ai/api
💡 If you're using a corporate network, you may need to configure proxy settings`)
		}

		return fmt.Errorf("failed to list workflows: %w", err)
	}

	output := WorkflowListOutput{
		Workflows:  make([]WorkflowDetailsOutput, len(resp.Workflows)),
		Total:      resp.Total,
		Page:       resp.Page,
		PageSize:   resp.PageSize,
		TotalPages: resp.TotalPages,
	}
	for i, wf := range resp.Workflows {
		output.Workflows[i] = WorkflowDetailsOutput{
			Name:          wf.Name,
			Description:   wf.Description,
			Status:        wf.Status,
			LastExecution: wf.LastExecution(),
			CreatedBy:     wf.UserName,
			CreatedAt:     wf.CreatedAt,
			UpdatedAt:     wf.UpdatedAt,
		}
	}

	if params.JsonOutput {
		return output.PrintJSON(w)
	} else {
		return output.PrintTable(w)
	}
}

func getWorkflow(ctx context.Context, comp *composer.Client, params *WorkflowListParams, w io.Writer) error {
	wf, err := comp.GetWorkflow(ctx, params.WorkflowID)
	if err != nil {
		return fmt.Errorf("failed to get workflow: %w", err)
	}

	output := WorkflowDetailsOutput{
		Name:          wf.Name,
		Description:   wf.Description,
		Status:        wf.Status,
		LastExecution: wf.LastExecution(),
		CreatedBy:     wf.UserName,
		CreatedAt:     wf.CreatedAt,
		UpdatedAt:     wf.UpdatedAt,
	}

	if params.JsonOutput {
		return output.PrintJSON(w)
	} else {
		return output.PrintTable(w)
	}
}

// formatDate formats a date string for display
func formatDate(dateStr string) string {
	if dateStr == "" {
		return ""
	}

	// Try to parse the date
	t, err := time.Parse(time.RFC3339, dateStr)
	if err != nil {
		// Try alternative format
		t, err = time.Parse("2006-01-02T15:04:05.000000+00:00", dateStr)
		if err != nil {
			return dateStr // Return original if can't parse
		}
	}

	now := time.Now()
	diff := now.Sub(t)

	// Format based on how long ago
	switch {
	case diff < time.Hour:
		return fmt.Sprintf("%dm ago", int(diff.Minutes()))
	case diff < 24*time.Hour:
		return fmt.Sprintf("%dh ago", int(diff.Hours()))
	case diff < 7*24*time.Hour:
		return fmt.Sprintf("%dd ago", int(diff.Hours()/24))
	case diff < 30*24*time.Hour:
		return fmt.Sprintf("%dw ago", int(diff.Hours()/(24*7)))
	default:
		return t.Format("Jan 02, 2006")
	}
}

// checkAuthentication verifies that the user has proper authentication set up
func checkAuthentication(cfg *config.Config) error {
	if cfg.APIKey == "" {
		return fmt.Errorf(`no authentication configured!

🔑 Recommended: Set up interactive authentication
   kubiya login

💻 Alternative: Set API key manually
   export KUBIYA_API_KEY=your-api-key

🌐 Get your API key from: https://compose.kubiya.ai/settings#apiKeys

📖 Quick start:
   1. Run 'kubiya login' for interactive setup (recommended)
   2. Or visit https://compose.kubiya.ai/settings#apiKeys to get your API key
   3. Then export KUBIYA_API_KEY=your-key

Note: For automation/CI environments, use the KUBIYA_API_KEY environment variable
For interactive use, 'kubiya login' provides the best experience with automatic token refresh`)
	}
	return nil
}

type WorkflowExecutionListParams struct {
	Request    composer.WorkflowExecutionParams
	RunnerID   string
	JsonOutput bool
	StartTime  string
	EndTime    string
}

type WorkflowExecution struct {
	Name       string `json:"name"`
	Status     string `json:"status"`
	Runner     string `json:"runner"`
	StartedAt  string `json:"started_at"`
	Duration   string `json:"duration"`
	StepsDone  int    `json:"steps_completed"`
	StepsTotal int    `json:"steps_total"`
}

type WorkflowExecutionListOutput struct {
	WorkflowExecutions []WorkflowExecution
}

func (o *WorkflowExecutionListOutput) PrintJSON(w io.Writer) error {
	if w == nil {
		w = os.Stdout
	}
	enc := json.NewEncoder(w)
	enc.SetIndent("", "  ")
	return enc.Encode(o)
}

func (o *WorkflowExecutionListOutput) PrintTable(w io.Writer) error {
	if w == nil {
		w = os.Stdout
	}
	if len(o.WorkflowExecutions) == 0 {
		fmt.Fprintf(w, "\n%s No recent executions found\n", style.DimStyle.Render("ℹ️"))
		fmt.Fprintf(w, "%s Execute a workflow: %s\n",
			style.InfoStyle.Render("💡"),
			style.HighlightStyle.Render("kubiya workflow run <name-or-id>"))
		return nil
	}

	fmt.Fprintf(w, "\n%s\n", style.TitleStyle.Render("🕒 WORKFLOW EXECUTIONS (last 24h)"))
	fmt.Fprintf(w, "%s Found %d recent executions\n\n",
		style.DimStyle.Render("ℹ️"), len(o.WorkflowExecutions))

	tw := tabwriter.NewWriter(w, 0, 0, 3, ' ', 0)
	fmt.Fprintln(tw, style.DimStyle.Render("WORKFLOW\tSTATUS\tRUNNER\tSTARTED\tDURATION\tPROGRESS"))
	fmt.Fprintln(tw, style.DimStyle.Render("────────\t──────\t──────\t───────\t────────\t────────"))

	for _, ex := range o.WorkflowExecutions {
		// Enhanced status styling
		statusText := ex.Status
		switch strings.ToLower(statusText) {
		case "completed", "success":
			statusText = style.SuccessStyle.Render("✓ completed")
		case "running":
			statusText = style.InfoStyle.Render("▶ running")
		case "failed", "error":
			statusText = style.WarningStyle.Render("✗ failed")
		case "pending":
			statusText = style.WarningStyle.Render("⏳ pending")
		case "cancelled", "canceled":
			statusText = style.DimStyle.Render("⊘ cancelled")
		default:
			statusText = style.DimStyle.Render(statusText)
		}

		// Progress bar visualization
		progressText := fmt.Sprintf("%d/%d", ex.StepsDone, ex.StepsTotal)
		if ex.StepsTotal > 0 {
			progress := float64(ex.StepsDone) / float64(ex.StepsTotal)
			if progress == 1.0 {
				progressText = style.SuccessStyle.Render(progressText)
			} else if progress > 0.5 {
				progressText = style.InfoStyle.Render(progressText)
			} else {
				progressText = style.WarningStyle.Render(progressText)
			}
		}

		fmt.Fprintf(tw, "%s\t%s\t%s\t%s\t%s\t%s\n",
			style.HighlightStyle.Render(ex.Name),
			statusText,
			ex.Runner,
			formatDate(ex.StartedAt),
			ex.Duration,
			progressText)
	}

	tw.Flush()

	fmt.Fprintf(w, "\n%s Commands:\n", style.InfoStyle.Render("💡"))
	fmt.Fprintf(w, "  • %s - Filter by status\n", style.HighlightStyle.Render("kubiya workflow execution list --status running"))
	fmt.Fprintf(w, "  • %s - Filter by workflow\n", style.HighlightStyle.Render("kubiya workflow execution list --id <workflow-id>"))

	return nil
}

// newWorkflowExecutionListCommand lists executions from the last 24 hours
func newWorkflowExecutionListCommand(cfg *config.Config) *cobra.Command {
	var params WorkflowExecutionListParams

	cmd := &cobra.Command{
		Use:   "execution list",
		Short: "List workflow executions from the last 24 hours",
		RunE: func(cmd *cobra.Command, args []string) error {
			ctx := context.Background()
			comp := composer.NewClient(cfg)
			params.Request.SetDefaults()
			return listWorkflowExecutions(ctx, comp, &params, cmd.OutOrStdout())
		},
	}

	cmd.Flags().BoolVar(&params.JsonOutput, "json", false, "Output as JSON")
	cmd.Flags().IntVar(&params.Request.Limit, "limit", 100, "Max number of executions to return")
	cmd.Flags().StringVar(&params.Request.Status, "status", "", "Filter by status (running|canceled|pending|completed|failed)")
	cmd.Flags().StringVar(&params.Request.WorkflowID, "id", "", "Filter by workflow ID")
	cmd.Flags().StringVar(&params.RunnerID, "runner", "", "Filter by runner ID")

	return cmd
}

func listWorkflowExecutions(ctx context.Context, comp *composer.Client, params *WorkflowExecutionListParams, w io.Writer) error {
	// Set default time range if not provided
	startTime := params.StartTime
	endTime := params.EndTime

	if startTime == "" {
		// Default to 24 hours ago
		startTime = time.Now().Add(-24 * time.Hour).Format(time.RFC3339)
	}

	// Parse time bounds for filtering
	var startTimeParsed, endTimeParsed time.Time
	var err error

	if startTime != "" {
		startTimeParsed, err = time.Parse(time.RFC3339, startTime)
		if err != nil {
			return fmt.Errorf("invalid start time format, please use RFC3339 format (e.g., 2023-04-01T00:00:00Z): %w", err)
		}
	}

	if endTime != "" {
		endTimeParsed, err = time.Parse(time.RFC3339, endTime)
		if err != nil {
			return fmt.Errorf("invalid end time format, please use RFC3339 format (e.g., 2023-04-01T00:00:00Z): %w", err)
		}
	}

	var (
		page      = 1
		collected int
		output    WorkflowExecutionListOutput
	)

	for collected < params.Request.Limit {
		resp, err := comp.ListWorkflowExecutions(ctx, params.Request)
		if err != nil {
			return fmt.Errorf("failed to list executions: %w", err)
		}
		if len(resp.Executions) == 0 {
			break
		}

		for _, ex := range resp.Executions {
			started, err := time.Parse(time.RFC3339, ex.StartedAt)
			if err != nil {
				continue
			}

			// Apply time range filtering
			if !startTimeParsed.IsZero() && started.Before(startTimeParsed) {
				collected = params.Request.Limit
				break
			}
			if !endTimeParsed.IsZero() && started.After(endTimeParsed) {
				continue
			}

			var (
				name                  string
				stepsTotal, stepsDone int
				runner                = ex.Runner
			)
			if ex.Workflow != nil {
				name = ex.Workflow.Name
			}
			if ex.Runner != "" {
				runner = ex.Runner
			}
			stepsTotal = len(ex.Steps)
			for _, st := range ex.Steps {
				if strings.EqualFold(st.Status, "completed") || strings.EqualFold(st.Status, "success") {
					stepsDone++
				}
			}

			// Filter by runner if requested
			if params.RunnerID != "" && !strings.EqualFold(runner, params.RunnerID) {
				continue
			}

			output.WorkflowExecutions = append(output.WorkflowExecutions, WorkflowExecution{
				Name:       name,
				Status:     ex.Status,
				Runner:     runner,
				StartedAt:  ex.StartedAt,
				Duration:   ex.Duration(),
				StepsDone:  stepsDone,
				StepsTotal: stepsTotal,
			})
			collected++
			if collected >= params.Request.Limit {
				break
			}
		}

		if collected >= params.Request.Limit {
			break
		}
		page++
	}

	if params.JsonOutput {
		return output.PrintJSON(w)
	} else {
		return output.PrintTable(w)
	}
}

// newWorkflowExecutionsCommand creates a shorter alias for listing executions
func newWorkflowExecutionsCommand(cfg *config.Config) *cobra.Command {
	var params WorkflowExecutionListParams

	cmd := &cobra.Command{
		Use:     "executions",
		Aliases: []string{"exec", "ex"},
		Short:   "List workflow executions with configurable time range",
		Long: `List workflow executions with filtering options and configurable time ranges.

This command provides a view similar to the Composer UI executions page,
showing workflow runs with their status, progress, and timing information.

Time Range:
• Default: Last 24 hours if no time filters specified
• Custom: Use --start-time and --end-time for specific ranges
• Format: RFC3339 (e.g., 2023-04-01T00:00:00Z or 2023-04-01T00:00:00-07:00)

🔑 Authentication:
  Recommended: kubiya login (interactive setup)
  Alternative: export KUBIYA_API_KEY=your-key
  Get API key: https://compose.kubiya.ai/settings#apiKeys`,
		Example: `  # List executions from last 24 hours (default)
  kubiya workflow executions

  # List executions from last 5 minutes
  kubiya workflow executions --from 5m

  # List executions from last 2 hours
  kubiya workflow executions --from 2h

  # List executions from last 3 days
  kubiya workflow executions --from 3d

  # List executions from last week
  kubiya workflow executions --from 1w

  # Filter by workflow and status with time range
  kubiya workflow executions --id workflow-123 --status running --from 6h

  # Filter by runner with time range
  kubiya workflow executions --runner kubiya-hosted --from 1d

  # Output as JSON with time range
  kubiya workflow executions --json --limit 50 --from 12h

  # Use absolute time ranges (advanced)
  kubiya workflow executions --start-time "2024-01-01T00:00:00Z" --end-time "2024-01-02T00:00:00Z"`,
		RunE: func(cmd *cobra.Command, args []string) error {
			// Check for authentication before making API calls
			if err := checkAuthentication(cfg); err != nil {
				return err
			}

			ctx := context.Background()
			comp := composer.NewClient(cfg)
			params.Request.SetDefaults()
			return listWorkflowExecutions(ctx, comp, &params, cmd.OutOrStdout())
		},
	}

	// Add flags
	cmd.Flags().BoolVar(&params.JsonOutput, "json", false, "Output as JSON")
	cmd.Flags().IntVar(&params.Request.Limit, "limit", 100, "Max number of executions to return")
	cmd.Flags().StringVar(&params.Request.Status, "status", "", "Filter by status (running|canceled|pending|completed|failed)")
	cmd.Flags().StringVar(&params.Request.WorkflowID, "id", "", "Filter by workflow ID")
	cmd.Flags().StringVar(&params.RunnerID, "runner", "", "Filter by runner ID")

	// Time range flags
	cmd.Flags().StringVar(&params.StartTime, "start-time", "", "Start time (RFC3339 format: 2023-04-01T00:00:00Z)")
	cmd.Flags().StringVar(&params.EndTime, "end-time", "", "End time (RFC3339 format: 2023-04-01T00:00:00Z)")

	// Convenient relative time flag
	var fromDuration string
	cmd.Flags().StringVar(&fromDuration, "from", "", "Show executions from X time ago (e.g., 5m, 2h, 1d, 1w)")

	// Pre-run hook to convert --from to --start-time
	cmd.PreRunE = func(cmd *cobra.Command, args []string) error {
		if fromDuration != "" {
			duration, err := parseDuration(fromDuration)
			if err != nil {
				return fmt.Errorf("invalid duration format '%s': %w\nSupported formats: 5m, 2h, 1d, 1w (m=minutes, h=hours, d=days, w=weeks)", fromDuration, err)
			}
			params.StartTime = time.Now().Add(-duration).Format(time.RFC3339)
		}
		return nil
	}

	return cmd
}

// parseDuration parses user-friendly duration strings like "5m", "2h", "1d", "1w"
func parseDuration(s string) (time.Duration, error) {
	if s == "" {
		return 0, fmt.Errorf("empty duration")
	}

	// Handle simple Go durations first (covers m, h, s, ms, us, ns)
	if duration, err := time.ParseDuration(s); err == nil {
		return duration, nil
	}

	// Parse custom formats for days and weeks
	if len(s) < 2 {
		return 0, fmt.Errorf("duration too short")
	}

	unit := s[len(s)-1:]
	valueStr := s[:len(s)-1]

	value, err := strconv.Atoi(valueStr)
	if err != nil {
		return 0, fmt.Errorf("invalid number '%s'", valueStr)
	}

	if value < 0 {
		return 0, fmt.Errorf("duration must be positive")
	}

	switch unit {
	case "d":
		return time.Duration(value) * 24 * time.Hour, nil
	case "w":
		return time.Duration(value) * 7 * 24 * time.Hour, nil
	default:
		return 0, fmt.Errorf("unsupported unit '%s'. Supported: m (minutes), h (hours), d (days), w (weeks)", unit)
	}
}
