package cli

import (
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/kubiyabot/cli/internal/kubiya"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestPlanStorageManager(t *testing.T) {
	// Create temp directory for testing
	tempDir := t.TempDir()

	// Create storage manager with custom directory
	psm := &PlanStorageManager{
		planDir: filepath.Join(tempDir, "plans"),
	}

	// Create plans directory
	err := os.MkdirAll(psm.planDir, 0755)
	require.NoError(t, err)

	t.Run("SavePlan", func(t *testing.T) {
		plan := &kubiya.PlanResponse{
			PlanID:  "test-plan-123",
			Title:   "Test Plan",
			Summary: "This is a test plan",
			Complexity: kubiya.ComplexityInfo{
				StoryPoints: 5,
				Confidence:  "high",
				Reasoning:   "Simple task",
			},
			CostEstimate: kubiya.CostEstimate{
				EstimatedCostUSD: 1.50,
			},
			RecommendedExecution: kubiya.RecommendedExecution{
				EntityType: "agent",
				EntityID:   "agent-123",
				EntityName: "Test Agent",
				Reasoning:  "Best suited for the task",
			},
			CreatedAt: time.Now(),
		}

		prompt := "Deploy my application"
		savedPlan, err := psm.SavePlan(plan, prompt)
		require.NoError(t, err)
		assert.NotNil(t, savedPlan)
		assert.Equal(t, plan.PlanID, savedPlan.Plan.PlanID)
		assert.Equal(t, prompt, savedPlan.Prompt)
		assert.False(t, savedPlan.Approved)

		// Verify file exists
		_, err = os.Stat(savedPlan.FilePath)
		assert.NoError(t, err)
	})

	t.Run("LoadPlan", func(t *testing.T) {
		// First save a plan
		plan := &kubiya.PlanResponse{
			PlanID:  "test-plan-456",
			Title:   "Load Test Plan",
			Summary: "Test loading functionality",
		}

		savedPlan, err := psm.SavePlan(plan, "Test prompt")
		require.NoError(t, err)

		// Now load it
		loadedPlan, err := psm.LoadPlan(savedPlan.FilePath)
		require.NoError(t, err)
		assert.Equal(t, plan.PlanID, loadedPlan.Plan.PlanID)
		assert.Equal(t, plan.Title, loadedPlan.Plan.Title)
		assert.Equal(t, "Test prompt", loadedPlan.Prompt)
	})

	t.Run("MarkApproved", func(t *testing.T) {
		plan := &kubiya.PlanResponse{
			PlanID: "test-plan-789",
			Title:  "Approval Test Plan",
		}

		savedPlan, err := psm.SavePlan(plan, "Test prompt")
		require.NoError(t, err)

		// Mark as approved
		err = psm.MarkApproved(savedPlan)
		require.NoError(t, err)
		assert.True(t, savedPlan.Approved)
		assert.NotNil(t, savedPlan.ApprovedAt)

		// Reload and verify
		reloaded, err := psm.LoadPlan(savedPlan.FilePath)
		require.NoError(t, err)
		assert.True(t, reloaded.Approved)
		assert.NotNil(t, reloaded.ApprovedAt)
	})

	t.Run("MarkExecuted", func(t *testing.T) {
		plan := &kubiya.PlanResponse{
			PlanID: "test-plan-101",
			Title:  "Execution Test Plan",
		}

		savedPlan, err := psm.SavePlan(plan, "Test prompt")
		require.NoError(t, err)

		// Mark as executed
		executionID := "exec-12345"
		err = psm.MarkExecuted(savedPlan, executionID)
		require.NoError(t, err)
		assert.NotNil(t, savedPlan.ExecutedAt)
		assert.Equal(t, executionID, savedPlan.ExecutionID)

		// Reload and verify
		reloaded, err := psm.LoadPlan(savedPlan.FilePath)
		require.NoError(t, err)
		assert.NotNil(t, reloaded.ExecutedAt)
		assert.Equal(t, executionID, reloaded.ExecutionID)
	})

	t.Run("ListPlans", func(t *testing.T) {
		// Save multiple plans
		for i := 0; i < 3; i++ {
			plan := &kubiya.PlanResponse{
				PlanID: "list-test-" + string(rune('a'+i)),
				Title:  "List Test Plan " + string(rune('A'+i)),
			}
			_, err := psm.SavePlan(plan, "Test prompt")
			require.NoError(t, err)
			time.Sleep(10 * time.Millisecond) // Ensure different timestamps
		}

		// List all plans
		plans, err := psm.ListPlans()
		require.NoError(t, err)
		assert.GreaterOrEqual(t, len(plans), 3)

		// Verify they're sorted by saved time (newest first)
		if len(plans) >= 2 {
			assert.True(t, plans[0].SavedAt.After(plans[1].SavedAt) || plans[0].SavedAt.Equal(plans[1].SavedAt))
		}
	})

	t.Run("PlanExists", func(t *testing.T) {
		planID := "exists-test-123"
		plan := &kubiya.PlanResponse{
			PlanID: planID,
			Title:  "Existence Test Plan",
		}

		// Should not exist initially
		assert.False(t, psm.PlanExists(planID))

		// Save the plan
		_, err := psm.SavePlan(plan, "Test prompt")
		require.NoError(t, err)

		// Should exist now
		assert.True(t, psm.PlanExists(planID))
	})

	t.Run("DeletePlan", func(t *testing.T) {
		planID := "delete-test-456"
		plan := &kubiya.PlanResponse{
			PlanID: planID,
			Title:  "Delete Test Plan",
		}

		savedPlan, err := psm.SavePlan(plan, "Test prompt")
		require.NoError(t, err)

		// Verify it exists
		assert.True(t, psm.PlanExists(planID))

		// Delete it
		err = psm.DeletePlan(planID)
		require.NoError(t, err)

		// Verify it's gone
		assert.False(t, psm.PlanExists(planID))

		// Loading should fail
		_, err = psm.LoadPlan(savedPlan.FilePath)
		assert.Error(t, err)
	})

	t.Run("GetPlansDirectory", func(t *testing.T) {
		dir := psm.GetPlansDirectory()
		assert.NotEmpty(t, dir)
		assert.Contains(t, dir, "plans")
	})
}

func TestNewPlanStorageManager(t *testing.T) {
	t.Run("CreatesDirectory", func(t *testing.T) {
		// Use temp home for testing
		tempHome := t.TempDir()
		originalHome := os.Getenv("HOME")
		os.Setenv("HOME", tempHome)
		defer os.Setenv("HOME", originalHome)

		psm, err := NewPlanStorageManager()
		require.NoError(t, err)
		assert.NotNil(t, psm)

		// Verify directory was created
		_, err = os.Stat(psm.planDir)
		assert.NoError(t, err)
	})
}
