package cli

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"time"

	"github.com/google/uuid"
	"github.com/kubiyabot/cli/internal/kubiya"
)

// PlanStorageManager manages plan file storage and lifecycle
type PlanStorageManager struct {
	planDir string
}

// NewPlanStorageManager creates a new plan storage manager
func NewPlanStorageManager() (*PlanStorageManager, error) {
	homeDir, err := os.UserHomeDir()
	if err != nil {
		return nil, fmt.Errorf("failed to get home directory: %w", err)
	}

	planDir := filepath.Join(homeDir, ".kubiya", "plans")
	if err := os.MkdirAll(planDir, 0755); err != nil {
		return nil, fmt.Errorf("failed to create plans directory: %w", err)
	}

	return &PlanStorageManager{planDir: planDir}, nil
}

// SavedPlan wraps a plan with lifecycle metadata
type SavedPlan struct {
	Plan        *kubiya.PlanResponse `json:"plan"`
	SavedAt     time.Time            `json:"saved_at"`
	Prompt      string               `json:"prompt"`
	Approved    bool                 `json:"approved"`
	ApprovedAt  *time.Time           `json:"approved_at,omitempty"`
	ExecutedAt  *time.Time           `json:"executed_at,omitempty"`
	ExecutionID string               `json:"execution_id,omitempty"`
	FilePath    string               `json:"file_path"`
}

// SavePlan saves a plan to disk
func (psm *PlanStorageManager) SavePlan(plan *kubiya.PlanResponse, prompt string) (*SavedPlan, error) {
	saved := &SavedPlan{
		Plan:    plan,
		SavedAt: time.Now(),
		Prompt:  prompt,
	}

	// Generate a fallback plan ID if empty
	planID := plan.PlanID
	if planID == "" {
		planID = uuid.New().String()
		// Update the plan with the generated ID
		plan.PlanID = planID
	}

	filename := fmt.Sprintf("%s.json", planID)
	filePath := filepath.Join(psm.planDir, filename)
	saved.FilePath = filePath

	data, err := json.MarshalIndent(saved, "", "  ")
	if err != nil {
		return nil, fmt.Errorf("failed to marshal plan: %w", err)
	}

	if err := os.WriteFile(filePath, data, 0644); err != nil {
		return nil, fmt.Errorf("failed to write plan file: %w", err)
	}

	return saved, nil
}

// LoadPlan loads a plan from disk
func (psm *PlanStorageManager) LoadPlan(path string) (*SavedPlan, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("failed to read plan file: %w", err)
	}

	var saved SavedPlan
	if err := json.Unmarshal(data, &saved); err != nil {
		return nil, fmt.Errorf("failed to unmarshal plan: %w", err)
	}

	saved.FilePath = path
	return &saved, nil
}

// MarkApproved marks a plan as approved
func (psm *PlanStorageManager) MarkApproved(plan *SavedPlan) error {
	now := time.Now()
	plan.Approved = true
	plan.ApprovedAt = &now

	data, err := json.MarshalIndent(plan, "", "  ")
	if err != nil {
		return fmt.Errorf("failed to marshal plan: %w", err)
	}

	return os.WriteFile(plan.FilePath, data, 0644)
}

// MarkExecuted marks a plan as executed
func (psm *PlanStorageManager) MarkExecuted(plan *SavedPlan, executionID string) error {
	now := time.Now()
	plan.ExecutedAt = &now
	plan.ExecutionID = executionID

	data, err := json.MarshalIndent(plan, "", "  ")
	if err != nil {
		return fmt.Errorf("failed to marshal plan: %w", err)
	}

	return os.WriteFile(plan.FilePath, data, 0644)
}

// ListPlans lists all saved plans
func (psm *PlanStorageManager) ListPlans() ([]*SavedPlan, error) {
	files, err := os.ReadDir(psm.planDir)
	if err != nil {
		return nil, fmt.Errorf("failed to read plans directory: %w", err)
	}

	var plans []*SavedPlan
	for _, file := range files {
		if !file.IsDir() && filepath.Ext(file.Name()) == ".json" {
			path := filepath.Join(psm.planDir, file.Name())
			plan, err := psm.LoadPlan(path)
			if err != nil {
				// Skip invalid plan files
				continue
			}
			plans = append(plans, plan)
		}
	}

	// Sort by saved time (newest first)
	sort.Slice(plans, func(i, j int) bool {
		return plans[i].SavedAt.After(plans[j].SavedAt)
	})

	return plans, nil
}

// GetPlanPath returns the full path for a plan file
func (psm *PlanStorageManager) GetPlanPath(planID string) string {
	return filepath.Join(psm.planDir, fmt.Sprintf("%s.json", planID))
}

// PlanExists checks if a plan file exists
func (psm *PlanStorageManager) PlanExists(planID string) bool {
	path := psm.GetPlanPath(planID)
	_, err := os.Stat(path)
	return err == nil
}

// DeletePlan deletes a plan file
func (psm *PlanStorageManager) DeletePlan(planID string) error {
	path := psm.GetPlanPath(planID)
	if err := os.Remove(path); err != nil {
		return fmt.Errorf("failed to delete plan file: %w", err)
	}
	return nil
}

// GetPlansDirectory returns the plans directory path
func (psm *PlanStorageManager) GetPlansDirectory() string {
	return psm.planDir
}
