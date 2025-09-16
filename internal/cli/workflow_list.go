package cli

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
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

func (o *WorkflowDetailsOutput) PrintJSON() error {
	enc := json.NewEncoder(os.Stdout)
	enc.SetIndent("", "  ")
	return enc.Encode(o)
}

func (o *WorkflowDetailsOutput) PrintTable() error {
	w := tabwriter.NewWriter(os.Stdout, 0, 0, 2, ' ', 0)
	fmt.Fprintln(w, style.TitleStyle.Render("📋 WORKFLOW"))
	fmt.Fprintln(w, "NAME\tSTATUS\tLAST EXECUTION\tCREATED BY\tCREATED AT\tUPDATED AT\tDESCRIPTION")
	fmt.Fprintf(w, "%s\t%s\t%s\t%s\t%s\t%s\t%s\n",
		style.HighlightStyle.Render(o.Name),
		o.Status, o.LastExecution, o.CreatedBy, o.CreatedAt, o.UpdatedAt, o.Description)
	return w.Flush()
}

type WorkflowListOutput struct {
	Workflows []WorkflowDetailsOutput `json:"workflows"`
	Total     int                     `json:"total"`
	Page      int                     `json:"page"`
	PageSize  int                     `json:"page_size"`
}

func (o *WorkflowListOutput) PrintJSON() error {
	enc := json.NewEncoder(os.Stdout)
	enc.SetIndent("", "  ")
	return enc.Encode(o)
}

func (o *WorkflowListOutput) PrintTable() error {
	w := tabwriter.NewWriter(os.Stdout, 0, 0, 2, ' ', 0)
	fmt.Fprintln(w, style.TitleStyle.Render("📋 WORKFLOWS"))
	fmt.Fprintln(w, "NAME\tSTATUS\tLAST EXECUTION\tCREATED BY\tCREATED AT\tUPDATED AT\tDESCRIPTION")
	for _, wf := range o.Workflows {
		statusText := wf.Status
		if strings.EqualFold(statusText, "published") {
			statusText = style.SuccessStyle.Render("published")
		} else if strings.EqualFold(statusText, "draft") {
			statusText = style.DimStyle.Render("draft")
		}
		fmt.Fprintf(w, "%s\t%s\t%s\t%s\t%s\t%s\t%s\n",
			style.HighlightStyle.Render(wf.Name),
			statusText, wf.LastExecution, wf.CreatedBy, wf.CreatedAt, wf.UpdatedAt, wf.Description)
	}
	return w.Flush()
}

func newWorkflowListCommand(cfg *config.Config) *cobra.Command {
	var params WorkflowListParams

	cmd := &cobra.Command{
		Use:   "list",
		Short: "List organization workflows (Composer)",
		Long:  "List all workflows in your organization using the Composer API.",
		Example: `  # List workflows as a table
  kubiya workflow list

  # Output JSON with more items
  kubiya workflow list --json --limit 50

  # Filter by status or search by name
  kubiya workflow list --status published --search deploy`,
		RunE: func(cmd *cobra.Command, args []string) error {
			ctx := context.Background()
			comp := composer.NewClient(cfg)
			params.Request.SetDefaults()
			if len(params.WorkflowID) == 0 {
				return listWorkflows(ctx, comp, &params)
			} else {
				return getWorkflow(ctx, comp, &params)
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

func listWorkflows(ctx context.Context, comp *composer.Client, params *WorkflowListParams) error {
	resp, err := comp.ListWorkflows(ctx, params.Request)
	if err != nil {
		return fmt.Errorf("failed to list workflows: %w", err)
	}

	output := WorkflowListOutput{
		Workflows: make([]WorkflowDetailsOutput, len(resp.Workflows)),
		Total:     resp.Total,
		Page:      resp.Page,
		PageSize:  resp.PageSize,
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
		return output.PrintJSON()
	} else {
		return output.PrintTable()
	}
}

func getWorkflow(ctx context.Context, comp *composer.Client, params *WorkflowListParams) error {
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
		return output.PrintJSON()
	} else {
		return output.PrintTable()
	}
}

type WorkflowExecutionListParams struct {
	Request    composer.WorkflowExecutionParams
	RunnerID   string
	JsonOutput bool
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

func (o *WorkflowExecutionListOutput) PrintJSON() error {
	enc := json.NewEncoder(os.Stdout)
	enc.SetIndent("", "  ")
	return enc.Encode(o)
}

func (o *WorkflowExecutionListOutput) PrintTable() error {
	w := tabwriter.NewWriter(os.Stdout, 0, 0, 2, ' ', 0)
	fmt.Fprintln(w, style.TitleStyle.Render("🕒 EXECUTIONS (last 24h)"))
	fmt.Fprintln(w, "NAME\tSTATUS\tRUNNER\tSTARTED AT\tDURATION\tSTEPS (DONE/TOTAL)")
	for _, ex := range o.WorkflowExecutions {
		fmt.Fprintf(w, "%s\t%s\t%s\t%s\t%s\t%d/%d\n",
			style.HighlightStyle.Render(ex.Name),
			ex.Status, ex.Runner, ex.StartedAt, ex.Duration, ex.StepsDone, ex.StepsTotal)
	}
	return w.Flush()
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
			return listWorkflowExecutions(ctx, comp, &params)
		},
	}

	cmd.Flags().BoolVar(&params.JsonOutput, "json", false, "Output as JSON")
	cmd.Flags().IntVar(&params.Request.Limit, "limit", 100, "Max number of executions to return")
	cmd.Flags().StringVar(&params.Request.Status, "status", "", "Filter by status (running|canceled|pending|completed|failed)")
	cmd.Flags().StringVar(&params.Request.WorkflowID, "id", "", "Filter by workflow ID")
	cmd.Flags().StringVar(&params.RunnerID, "runner", "", "Filter by runner ID")

	return cmd
}

func listWorkflowExecutions(ctx context.Context, comp *composer.Client, params *WorkflowExecutionListParams) error {
	var (
		page      = 1
		collected int
		cutoff    = time.Now().Add(-24 * time.Hour)
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
			if started.Before(cutoff) {
				collected = params.Request.Limit
				break
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
		return output.PrintJSON()
	} else {
		return output.PrintTable()
	}
}
