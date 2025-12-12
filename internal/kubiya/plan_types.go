package kubiya

import "time"

// PlanRequest represents a task planning request
type PlanRequest struct {
	Description  string              `json:"description"`
	Priority     string              `json:"priority"` // low, medium, high, critical
	Agents       []AgentInfo         `json:"agents"`
	Teams        []TeamInfo          `json:"teams"`
	Environments []EnvironmentInfo   `json:"environments"`
	WorkerQueues []WorkerQueueInfo   `json:"worker_queues"`
	OutputFormat string              `json:"output_format,omitempty"` // json, yaml, text
	QuickMode    bool                `json:"quick_mode,omitempty"`    // Fast planning for local execution - uses simplified LLM prompt
}

// PlanResponse represents a task plan
type PlanResponse struct {
	PlanID               string                 `json:"plan_id"`
	Title                string                 `json:"title"`
	Summary              string                 `json:"summary"`
	Complexity           ComplexityInfo         `json:"complexity"`
	TeamBreakdown        []TeamBreakdownItem    `json:"team_breakdown"`
	RecommendedExecution RecommendedExecution   `json:"recommended_execution"`
	CostEstimate         CostEstimate           `json:"cost_estimate"`
	RealizedSavings      RealizedSavings        `json:"realized_savings"`
	Risks                []string               `json:"risks"`
	Prerequisites        []string               `json:"prerequisites"`
	SuccessCriteria      []string               `json:"success_criteria"`
	CreatedAt            time.Time              `json:"created_at"`
}

// ComplexityInfo represents task complexity assessment
type ComplexityInfo struct {
	StoryPoints int     `json:"story_points"`
	Confidence  string  `json:"confidence"` // low, medium, high
	Reasoning   string  `json:"reasoning"`
}

// TeamBreakdownItem represents a phase of work assigned to a team or agent
type TeamBreakdownItem struct {
	TeamID              *string    `json:"team_id,omitempty"`
	TeamName            string     `json:"team_name,omitempty"`
	AgentID             *string    `json:"agent_id,omitempty"`
	AgentName           *string    `json:"agent_name,omitempty"`
	Responsibilities    []string   `json:"responsibilities"`
	Tasks               []TaskItem `json:"tasks"`
	EstimatedTimeHours  float64    `json:"estimated_time_hours"`
}

// TaskItem represents a single task or subtask
type TaskItem struct {
	Title        string     `json:"title"`
	Description  string     `json:"description,omitempty"`
	Status       string     `json:"status"` // pending, in_progress, done
	Priority     string     `json:"priority"` // low, medium, high
	Dependencies []int      `json:"dependencies,omitempty"` // Task IDs that must be completed first
	Subtasks     []TaskItem `json:"subtasks,omitempty"`
}

// RecommendedExecution contains the planner's recommendation for execution
type RecommendedExecution struct {
	EntityType                  string  `json:"entity_type"` // "agent" or "team"
	EntityID                    string  `json:"entity_id"`
	EntityName                  string  `json:"entity_name"`
	RecommendedEnvironmentID    *string `json:"recommended_environment_id,omitempty"`
	RecommendedEnvironmentName  *string `json:"recommended_environment_name,omitempty"`
	RecommendedWorkerQueueID    *string `json:"recommended_worker_queue_id,omitempty"`
	RecommendedWorkerQueueName  *string `json:"recommended_worker_queue_name,omitempty"`
	Reasoning                   string  `json:"reasoning"`
}

// CostEstimate represents estimated costs for execution
type CostEstimate struct {
	EstimatedCostUSD float64       `json:"estimated_cost_usd"`
	LLMCosts         []LLMCost     `json:"llm_costs"`
	ToolCosts        []ToolCost    `json:"tool_costs"`
}

// LLMCost represents cost for a specific LLM model
type LLMCost struct {
	ModelID                 string  `json:"model_id"`
	EstimatedInputTokens    int     `json:"estimated_input_tokens"`
	EstimatedOutputTokens   int     `json:"estimated_output_tokens"`
	CostPer1kInputTokens    float64 `json:"cost_per_1k_input_tokens"`
	CostPer1kOutputTokens   float64 `json:"cost_per_1k_output_tokens"`
	TotalCost               float64 `json:"total_cost"`
}

// ToolCost represents cost for tool usage
type ToolCost struct {
	Category       string  `json:"category"`
	EstimatedCalls int     `json:"estimated_calls"`
	CostPerCall    float64 `json:"cost_per_call"`
	TotalCost      float64 `json:"total_cost"`
}

// RealizedSavings represents time and cost savings from automation
type RealizedSavings struct {
	TimeSavedHours       float64 `json:"time_saved_hours"`
	MoneySaved           float64 `json:"money_saved"`
	ManualEffortAvoided  string  `json:"manual_effort_avoided"`
}

// AgentInfo represents agent metadata for planning
type AgentInfo struct {
	ID           string   `json:"id"`
	Name         string   `json:"name"`
	Description  string   `json:"description,omitempty"`
	Capabilities []string `json:"capabilities,omitempty"`
	ModelID      *string  `json:"model_id,omitempty"`
}

// TeamInfo represents team metadata for planning
type TeamInfo struct {
	ID          string   `json:"id"`
	Name        string   `json:"name"`
	Description string   `json:"description,omitempty"`
	Agents      []string `json:"agents,omitempty"` // Agent IDs
}

// EnvironmentInfo represents environment metadata for planning
type EnvironmentInfo struct {
	ID          string                 `json:"id"`
	Name        string                 `json:"name"`
	Description string                 `json:"description,omitempty"`
	Metadata    map[string]interface{} `json:"metadata,omitempty"`
}

// WorkerQueueInfo represents worker queue metadata for planning
type WorkerQueueInfo struct {
	ID            string `json:"id"`
	Name          string `json:"name"`
	EnvironmentID string `json:"environment_id"`
	Status        string `json:"status,omitempty"`
}

// PlanStreamEvent represents a streaming event from plan generation
type PlanStreamEvent struct {
	Type string                 `json:"type"` // progress, thinking, tool_call, tool_result, resources_summary, complete, error
	Data map[string]interface{} `json:"data"`
}
