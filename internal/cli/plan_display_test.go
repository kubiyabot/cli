package cli

import (
	"bytes"
	"encoding/json"
	"io"
	"os"
	"strings"
	"testing"
	"time"

	"github.com/kubiyabot/cli/internal/kubiya"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"gopkg.in/yaml.v3"
)

// createTestPlan creates a sample plan for testing
func createTestPlan() *kubiya.PlanResponse {
	agentName := "Test Agent"
	envName := "Test Environment"
	queueName := "Test Queue"

	return &kubiya.PlanResponse{
		PlanID:  "test-plan-123",
		Title:   "Test Deployment Plan",
		Summary: "Deploy application to production",
		Complexity: kubiya.ComplexityInfo{
			StoryPoints: 5,
			Confidence:  "high",
			Reasoning:   "Straightforward deployment",
		},
		RecommendedExecution: kubiya.RecommendedExecution{
			EntityType:                 "agent",
			EntityID:                   "agent-123",
			EntityName:                 "Deploy Agent",
			Reasoning:                  "Best suited for deployments",
			RecommendedEnvironmentID:   stringPtr("env-123"),
			RecommendedEnvironmentName: &envName,
			RecommendedWorkerQueueID:   stringPtr("queue-123"),
			RecommendedWorkerQueueName: &queueName,
		},
		CostEstimate: kubiya.CostEstimate{
			EstimatedCostUSD: 2.50,
			LLMCosts: []kubiya.LLMCost{
				{
					ModelID:               "gpt-4",
					EstimatedInputTokens:  1000,
					EstimatedOutputTokens: 500,
					CostPer1kInputTokens:  0.03,
					CostPer1kOutputTokens: 0.06,
					TotalCost:             0.06,
				},
			},
			ToolCosts: []kubiya.ToolCost{
				{
					Category:       "kubectl",
					EstimatedCalls: 10,
					CostPerCall:    0.01,
					TotalCost:      0.10,
				},
			},
		},
		RealizedSavings: kubiya.RealizedSavings{
			MoneySaved:     100.00,
			TimeSavedHours: 2.5,
		},
		TeamBreakdown: []kubiya.TeamBreakdownItem{
			{
				AgentName:           &agentName,
				TeamName:            "Deployment Team",
				Responsibilities:    []string{"Deploy application", "Verify health"},
				EstimatedTimeHours:  1.5,
				Tasks: []kubiya.TaskItem{
					{
						Title:    "Build Docker image",
						Status:   "pending",
						Priority: "high",
						Dependencies: []int{},
					},
					{
						Title:    "Deploy to Kubernetes",
						Status:   "pending",
						Priority: "medium",
						Subtasks: []kubiya.TaskItem{
							{
								Title:    "Apply manifests",
								Status:   "pending",
								Priority: "high",
							},
							{
								Title:    "Wait for rollout",
								Status:   "pending",
								Priority: "low",
							},
						},
					},
				},
			},
		},
		Risks:           []string{"Potential downtime during deployment", "Database migration may take longer than expected"},
		Prerequisites:   []string{"Code review completed", "Tests passing"},
		SuccessCriteria: []string{"Application responds to health checks", "Zero errors in logs"},
		CreatedAt:       time.Now(),
	}
}

func stringPtr(s string) *string {
	return &s
}

func TestNewPlanDisplayer(t *testing.T) {
	plan := createTestPlan()

	t.Run("CreateWithTextFormat", func(t *testing.T) {
		displayer := NewPlanDisplayer(plan, "text", true)
		assert.NotNil(t, displayer)
		assert.Equal(t, plan, displayer.plan)
		assert.Equal(t, "text", displayer.outputFormat)
		assert.True(t, displayer.interactive)
	})

	t.Run("CreateWithJSONFormat", func(t *testing.T) {
		displayer := NewPlanDisplayer(plan, "json", false)
		assert.NotNil(t, displayer)
		assert.Equal(t, "json", displayer.outputFormat)
		assert.False(t, displayer.interactive)
	})

	t.Run("CreateWithYAMLFormat", func(t *testing.T) {
		displayer := NewPlanDisplayer(plan, "yaml", false)
		assert.NotNil(t, displayer)
		assert.Equal(t, "yaml", displayer.outputFormat)
		assert.False(t, displayer.interactive)
	})
}

func TestDisplayJSON(t *testing.T) {
	plan := createTestPlan()
	displayer := NewPlanDisplayer(plan, "json", false)

	t.Run("ValidJSONOutput", func(t *testing.T) {
		// Capture stdout
		old := os.Stdout
		r, w, _ := os.Pipe()
		os.Stdout = w

		err := displayer.displayJSON()
		require.NoError(t, err)

		w.Close()
		os.Stdout = old

		var buf bytes.Buffer
		io.Copy(&buf, r)
		output := buf.String()

		// Verify it's valid JSON
		var result kubiya.PlanResponse
		err = json.Unmarshal([]byte(output), &result)
		require.NoError(t, err)

		// Verify key fields
		assert.Equal(t, plan.PlanID, result.PlanID)
		assert.Equal(t, plan.Title, result.Title)
		assert.Equal(t, plan.Summary, result.Summary)
	})

	t.Run("JSONContainsAllFields", func(t *testing.T) {
		// Capture stdout
		old := os.Stdout
		r, w, _ := os.Pipe()
		os.Stdout = w

		err := displayer.displayJSON()
		require.NoError(t, err)

		w.Close()
		os.Stdout = old

		var buf bytes.Buffer
		io.Copy(&buf, r)
		output := buf.String()

		// Check for important fields in JSON
		assert.Contains(t, output, "test-plan-123")
		assert.Contains(t, output, "Test Deployment Plan")
		assert.Contains(t, output, "Deploy Agent")
		assert.Contains(t, output, "2.5")  // Cost
	})
}

func TestDisplayYAML(t *testing.T) {
	plan := createTestPlan()
	displayer := NewPlanDisplayer(plan, "yaml", false)

	t.Run("ValidYAMLOutput", func(t *testing.T) {
		// Capture stdout
		old := os.Stdout
		r, w, _ := os.Pipe()
		os.Stdout = w

		err := displayer.displayYAML()
		require.NoError(t, err)

		w.Close()
		os.Stdout = old

		var buf bytes.Buffer
		io.Copy(&buf, r)
		output := buf.String()

		// Verify it's valid YAML
		var result kubiya.PlanResponse
		err = yaml.Unmarshal([]byte(output), &result)
		require.NoError(t, err)

		// Verify key fields
		assert.Equal(t, plan.PlanID, result.PlanID)
		assert.Equal(t, plan.Title, result.Title)
	})

	t.Run("YAMLContainsAllFields", func(t *testing.T) {
		// Capture stdout
		old := os.Stdout
		r, w, _ := os.Pipe()
		os.Stdout = w

		err := displayer.displayYAML()
		require.NoError(t, err)

		w.Close()
		os.Stdout = old

		var buf bytes.Buffer
		io.Copy(&buf, r)
		output := buf.String()

		// Check for important fields in YAML
		assert.Contains(t, output, "test-plan-123")
		assert.Contains(t, output, "Test Deployment Plan")
		assert.Contains(t, output, "Deploy Agent")
	})
}

func TestGetTaskStatusIcon(t *testing.T) {
	plan := createTestPlan()
	displayer := NewPlanDisplayer(plan, "text", false)

	tests := []struct {
		name           string
		status         string
		expectedSymbol string
	}{
		{
			name:           "DoneStatus",
			status:         "done",
			expectedSymbol: "✓",
		},
		{
			name:           "InProgressStatus",
			status:         "in_progress",
			expectedSymbol: "⟳",
		},
		{
			name:           "PendingStatus",
			status:         "pending",
			expectedSymbol: "○",
		},
		{
			name:           "UnknownStatus",
			status:         "unknown",
			expectedSymbol: "○", // Default to pending
		},
		{
			name:           "EmptyStatus",
			status:         "",
			expectedSymbol: "○", // Default to pending
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			icon := displayer.getTaskStatusIcon(tt.status)
			assert.Contains(t, icon, tt.expectedSymbol)
		})
	}
}

func TestGetPriorityIcon(t *testing.T) {
	plan := createTestPlan()
	displayer := NewPlanDisplayer(plan, "text", false)

	tests := []struct {
		name           string
		priority       string
		expectedSymbol string
	}{
		{
			name:           "HighPriority",
			priority:       "high",
			expectedSymbol: "↑↑",
		},
		{
			name:           "MediumPriority",
			priority:       "medium",
			expectedSymbol: "↑",
		},
		{
			name:           "LowPriority",
			priority:       "low",
			expectedSymbol: "→",
		},
		{
			name:           "UnknownPriority",
			priority:       "unknown",
			expectedSymbol: "→", // Default to low
		},
		{
			name:           "EmptyPriority",
			priority:       "",
			expectedSymbol: "→", // Default to low
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			icon := displayer.getPriorityIcon(tt.priority)
			assert.Contains(t, icon, tt.expectedSymbol)
		})
	}
}

func TestDisplayPlanRouting(t *testing.T) {
	plan := createTestPlan()

	t.Run("RoutesToJSON", func(t *testing.T) {
		displayer := NewPlanDisplayer(plan, "json", false)

		// Capture stdout
		old := os.Stdout
		r, w, _ := os.Pipe()
		os.Stdout = w

		err := displayer.DisplayPlan()
		require.NoError(t, err)

		w.Close()
		os.Stdout = old

		var buf bytes.Buffer
		io.Copy(&buf, r)
		output := buf.String()

		// Should be valid JSON
		assert.True(t, json.Valid([]byte(output)))
	})

	t.Run("RoutesToYAML", func(t *testing.T) {
		displayer := NewPlanDisplayer(plan, "yaml", false)

		// Capture stdout
		old := os.Stdout
		r, w, _ := os.Pipe()
		os.Stdout = w

		err := displayer.DisplayPlan()
		require.NoError(t, err)

		w.Close()
		os.Stdout = old

		var buf bytes.Buffer
		io.Copy(&buf, r)
		output := buf.String()

		// Should contain YAML markers
		assert.True(t, strings.Contains(output, "plan_id:") || strings.Contains(output, "planid:"))
	})

	t.Run("RoutesToTextNonInteractive", func(t *testing.T) {
		displayer := NewPlanDisplayer(plan, "text", false)

		// Capture stdout
		old := os.Stdout
		r, w, _ := os.Pipe()
		os.Stdout = w

		err := displayer.DisplayPlan()
		require.NoError(t, err)

		w.Close()
		os.Stdout = old

		var buf bytes.Buffer
		io.Copy(&buf, r)
		output := buf.String()

		// Should contain text output markers
		assert.Contains(t, output, "Task Execution Plan")
		assert.Contains(t, output, plan.Title)
	})
}

func TestDisplayText(t *testing.T) {
	plan := createTestPlan()
	displayer := NewPlanDisplayer(plan, "text", false)

	t.Run("TextOutputContainsKeyElements", func(t *testing.T) {
		// Capture stdout
		old := os.Stdout
		r, w, _ := os.Pipe()
		os.Stdout = w

		err := displayer.displayText()
		require.NoError(t, err)

		w.Close()
		os.Stdout = old

		var buf bytes.Buffer
		io.Copy(&buf, r)
		output := buf.String()

		// Verify key sections are present
		assert.Contains(t, output, "Task Execution Plan")
		assert.Contains(t, output, plan.Title)
		assert.Contains(t, output, plan.Summary)
		assert.Contains(t, output, "Recommended Execution")
		assert.Contains(t, output, "Cost Estimate")
		assert.Contains(t, output, "Task Breakdown")
	})

	t.Run("TextOutputContainsRisks", func(t *testing.T) {
		// Capture stdout
		old := os.Stdout
		r, w, _ := os.Pipe()
		os.Stdout = w

		err := displayer.displayText()
		require.NoError(t, err)

		w.Close()
		os.Stdout = old

		var buf bytes.Buffer
		io.Copy(&buf, r)
		output := buf.String()

		// Verify risks are displayed
		assert.Contains(t, output, "Identified Risks")
		assert.Contains(t, output, "Potential downtime")
	})

	t.Run("TextOutputContainsPrerequisites", func(t *testing.T) {
		// Capture stdout
		old := os.Stdout
		r, w, _ := os.Pipe()
		os.Stdout = w

		err := displayer.displayText()
		require.NoError(t, err)

		w.Close()
		os.Stdout = old

		var buf bytes.Buffer
		io.Copy(&buf, r)
		output := buf.String()

		// Verify prerequisites are displayed
		assert.Contains(t, output, "Prerequisites")
		assert.Contains(t, output, "Code review completed")
	})

	t.Run("TextOutputContainsSuccessCriteria", func(t *testing.T) {
		// Capture stdout
		old := os.Stdout
		r, w, _ := os.Pipe()
		os.Stdout = w

		err := displayer.displayText()
		require.NoError(t, err)

		w.Close()
		os.Stdout = old

		var buf bytes.Buffer
		io.Copy(&buf, r)
		output := buf.String()

		// Verify success criteria are displayed
		assert.Contains(t, output, "Success Criteria")
		assert.Contains(t, output, "Application responds to health checks")
	})
}

func TestDisplayTaskList(t *testing.T) {
	plan := createTestPlan()
	displayer := NewPlanDisplayer(plan, "text", false)

	t.Run("DisplaysTaskHierarchy", func(t *testing.T) {
		// Capture stdout
		old := os.Stdout
		r, w, _ := os.Pipe()
		os.Stdout = w

		tasks := plan.TeamBreakdown[0].Tasks
		displayer.displayTaskList(tasks, 1)

		w.Close()
		os.Stdout = old

		var buf bytes.Buffer
		io.Copy(&buf, r)
		output := buf.String()

		// Verify tasks are displayed
		assert.Contains(t, output, "Build Docker image")
		assert.Contains(t, output, "Deploy to Kubernetes")
		assert.Contains(t, output, "Apply manifests") // Subtask
	})

	t.Run("DisplaysDependencies", func(t *testing.T) {
		// Capture stdout
		old := os.Stdout
		r, w, _ := os.Pipe()
		os.Stdout = w

		tasks := plan.TeamBreakdown[0].Tasks
		displayer.displayTaskList(tasks, 1)

		w.Close()
		os.Stdout = old

		var buf bytes.Buffer
		io.Copy(&buf, r)
		output := buf.String()

		// Verify dependencies are shown
		assert.Contains(t, output, "Depends on")
		assert.Contains(t, output, "code-review")
	})

	t.Run("DisplaysStatusIcons", func(t *testing.T) {
		// Capture stdout
		old := os.Stdout
		r, w, _ := os.Pipe()
		os.Stdout = w

		tasks := plan.TeamBreakdown[0].Tasks
		displayer.displayTaskList(tasks, 1)

		w.Close()
		os.Stdout = old

		var buf bytes.Buffer
		io.Copy(&buf, r)
		output := buf.String()

		// Verify status icons are present (at least the pending icon)
		assert.Contains(t, output, "○")
	})

	t.Run("DisplaysPriorityIcons", func(t *testing.T) {
		// Capture stdout
		old := os.Stdout
		r, w, _ := os.Pipe()
		os.Stdout = w

		tasks := plan.TeamBreakdown[0].Tasks
		displayer.displayTaskList(tasks, 1)

		w.Close()
		os.Stdout = old

		var buf bytes.Buffer
		io.Copy(&buf, r)
		output := buf.String()

		// Verify priority indicators are present
		assert.True(t, strings.Contains(output, "↑↑") || strings.Contains(output, "↑") || strings.Contains(output, "→"))
	})
}

func TestDisplayCostEstimate(t *testing.T) {
	plan := createTestPlan()
	displayer := NewPlanDisplayer(plan, "text", false)

	t.Run("DisplaysTotalCost", func(t *testing.T) {
		// Capture stdout
		old := os.Stdout
		r, w, _ := os.Pipe()
		os.Stdout = w

		displayer.displayCostEstimate()

		w.Close()
		os.Stdout = old

		var buf bytes.Buffer
		io.Copy(&buf, r)
		output := buf.String()

		// Verify cost is displayed
		assert.Contains(t, output, "Cost Estimate")
		assert.Contains(t, output, "$2.50")
	})

	t.Run("DisplaysLLMCosts", func(t *testing.T) {
		// Capture stdout
		old := os.Stdout
		r, w, _ := os.Pipe()
		os.Stdout = w

		displayer.displayCostEstimate()

		w.Close()
		os.Stdout = old

		var buf bytes.Buffer
		io.Copy(&buf, r)
		output := buf.String()

		// Verify LLM costs are shown
		assert.Contains(t, output, "LLM Costs")
	})

	t.Run("DisplaysToolCosts", func(t *testing.T) {
		// Capture stdout
		old := os.Stdout
		r, w, _ := os.Pipe()
		os.Stdout = w

		displayer.displayCostEstimate()

		w.Close()
		os.Stdout = old

		var buf bytes.Buffer
		io.Copy(&buf, r)
		output := buf.String()

		// Verify tool costs are shown
		assert.Contains(t, output, "Tool Costs")
	})

	t.Run("DisplaysRealizedSavings", func(t *testing.T) {
		// Capture stdout
		old := os.Stdout
		r, w, _ := os.Pipe()
		os.Stdout = w

		displayer.displayCostEstimate()

		w.Close()
		os.Stdout = old

		var buf bytes.Buffer
		io.Copy(&buf, r)
		output := buf.String()

		// Verify realized savings are shown
		assert.Contains(t, output, "Realized Savings")
		assert.Contains(t, output, "$100.00")
		assert.Contains(t, output, "2.5 hours saved")
	})
}

func TestDisplayRecommendedExecution(t *testing.T) {
	plan := createTestPlan()
	displayer := NewPlanDisplayer(plan, "text", false)

	t.Run("DisplaysEntityInformation", func(t *testing.T) {
		// Capture stdout
		old := os.Stdout
		r, w, _ := os.Pipe()
		os.Stdout = w

		displayer.displayRecommendedExecution()

		w.Close()
		os.Stdout = old

		var buf bytes.Buffer
		io.Copy(&buf, r)
		output := buf.String()

		// Verify entity info is displayed
		assert.Contains(t, output, "Recommended Execution")
		assert.Contains(t, output, "Deploy Agent")
		assert.Contains(t, output, "Agent")
	})

	t.Run("DisplaysEnvironment", func(t *testing.T) {
		// Capture stdout
		old := os.Stdout
		r, w, _ := os.Pipe()
		os.Stdout = w

		displayer.displayRecommendedExecution()

		w.Close()
		os.Stdout = old

		var buf bytes.Buffer
		io.Copy(&buf, r)
		output := buf.String()

		// Verify environment is shown
		assert.Contains(t, output, "Environment")
		assert.Contains(t, output, "Test Environment")
	})

	t.Run("DisplaysWorkerQueue", func(t *testing.T) {
		// Capture stdout
		old := os.Stdout
		r, w, _ := os.Pipe()
		os.Stdout = w

		displayer.displayRecommendedExecution()

		w.Close()
		os.Stdout = old

		var buf bytes.Buffer
		io.Copy(&buf, r)
		output := buf.String()

		// Verify worker queue is shown
		assert.Contains(t, output, "Worker Queue")
		assert.Contains(t, output, "Test Queue")
	})

	t.Run("DisplaysReasoning", func(t *testing.T) {
		// Capture stdout
		old := os.Stdout
		r, w, _ := os.Pipe()
		os.Stdout = w

		displayer.displayRecommendedExecution()

		w.Close()
		os.Stdout = old

		var buf bytes.Buffer
		io.Copy(&buf, r)
		output := buf.String()

		// Verify reasoning is shown
		assert.Contains(t, output, "Best suited for deployments")
	})
}

func TestDisplayWithEmptyFields(t *testing.T) {
	// Create a minimal plan with empty optional fields
	minimalPlan := &kubiya.PlanResponse{
		PlanID:  "minimal-plan",
		Title:   "Minimal Plan",
		Summary: "Basic plan",
		Complexity: kubiya.ComplexityInfo{
			StoryPoints: 1,
			Confidence:  "low",
		},
		RecommendedExecution: kubiya.RecommendedExecution{
			EntityType: "agent",
			EntityID:   "agent-1",
			EntityName: "Agent",
			Reasoning:  "Simple task",
		},
		CostEstimate: kubiya.CostEstimate{
			EstimatedCostUSD: 0.10,
		},
		RealizedSavings: kubiya.RealizedSavings{},
		TeamBreakdown:   []kubiya.TeamBreakdownItem{},
		Risks:           []string{},
		Prerequisites:   []string{},
		SuccessCriteria: []string{},
	}

	displayer := NewPlanDisplayer(minimalPlan, "text", false)

	t.Run("HandlesEmptyRisks", func(t *testing.T) {
		// Should not panic with empty risks
		old := os.Stdout
		r, w, _ := os.Pipe()
		os.Stdout = w

		err := displayer.displayText()
		require.NoError(t, err)

		w.Close()
		os.Stdout = old

		var buf bytes.Buffer
		io.Copy(&buf, r)
		output := buf.String()

		// Should still display but with empty sections
		assert.NotEmpty(t, output)
	})

	t.Run("HandlesZeroSavings", func(t *testing.T) {
		// Capture stdout
		old := os.Stdout
		r, w, _ := os.Pipe()
		os.Stdout = w

		displayer.displayCostEstimate()

		w.Close()
		os.Stdout = old

		var buf bytes.Buffer
		io.Copy(&buf, r)
		output := buf.String()

		// Should not display realized savings if zero
		assert.NotContains(t, output, "Realized Savings")
	})
}
