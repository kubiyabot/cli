package filter

import (
	"context"
	"os"
	"strings"
	"time"

	"github.com/kubiyabot/cli/internal/mcp/session"
	"github.com/mark3labs/mcp-go/mcp"
)

// ToolFilter is a function that filters tools based on context
type ToolFilter func(ctx context.Context, tools []mcp.Tool) []mcp.Tool

// Chain chains multiple filters together
func Chain(filters ...ToolFilter) ToolFilter {
	return func(ctx context.Context, tools []mcp.Tool) []mcp.Tool {
		result := tools
		for _, filter := range filters {
			result = filter(ctx, result)
		}
		return result
	}
}

// PermissionFilter filters tools based on session permissions
type PermissionFilter struct {
	toolPermissions map[string][]string // tool name -> required permissions
}

// NewPermissionFilter creates a new permission filter
func NewPermissionFilter(toolPermissions map[string][]string) *PermissionFilter {
	if toolPermissions == nil {
		toolPermissions = make(map[string][]string)
	}
	return &PermissionFilter{
		toolPermissions: toolPermissions,
	}
}

// Apply filters tools based on permissions
func (f *PermissionFilter) Apply(ctx context.Context, tools []mcp.Tool) []mcp.Tool {
	sess, ok := session.SessionFromContext(ctx)
	if !ok {
		// No session, return only public tools (no permission requirements)
		return f.filterPublicTools(tools)
	}

	var filtered []mcp.Tool
	for _, tool := range tools {
		if f.hasPermissionForTool(sess, tool.Name) {
			filtered = append(filtered, tool)
		}
	}

	return filtered
}

// filterPublicTools returns only tools that don't require permissions
func (f *PermissionFilter) filterPublicTools(tools []mcp.Tool) []mcp.Tool {
	var filtered []mcp.Tool
	for _, tool := range tools {
		if requiredPerms, exists := f.toolPermissions[tool.Name]; !exists || len(requiredPerms) == 0 {
			filtered = append(filtered, tool)
		}
	}
	return filtered
}

// hasPermissionForTool checks if session has required permissions for a tool
func (f *PermissionFilter) hasPermissionForTool(sess *session.State, toolName string) bool {
	requiredPerms, exists := f.toolPermissions[toolName]
	if !exists || len(requiredPerms) == 0 {
		// No specific permissions required
		return true
	}

	// Check if user has any of the required permissions
	for _, required := range requiredPerms {
		if sess.HasPermission(required) {
			return true
		}
	}

	return false
}

// EnvironmentFilter filters tools based on environment
type EnvironmentFilter struct {
	devOnlyTools  map[string]bool
	prodOnlyTools map[string]bool
}

// NewEnvironmentFilter creates a new environment filter
func NewEnvironmentFilter() *EnvironmentFilter {
	return &EnvironmentFilter{
		devOnlyTools: map[string]bool{
			"debug_session":    true,
			"dump_state":       true,
			"test_integration": true,
			"mock_data":        true,
			"reset_database":   true,
		},
		prodOnlyTools: map[string]bool{
			"backup_production": true,
			"scale_service":     true,
			"rotate_secrets":    true,
		},
	}
}

// Apply filters tools based on current environment
func (f *EnvironmentFilter) Apply(ctx context.Context, tools []mcp.Tool) []mcp.Tool {
	env := strings.ToLower(os.Getenv("ENVIRONMENT"))
	if env == "" {
		env = "production" // Default to production for safety
	}

	var filtered []mcp.Tool
	for _, tool := range tools {
		// Skip dev-only tools in production
		if f.devOnlyTools[tool.Name] && env != "development" {
			continue
		}

		// Skip prod-only tools in non-production
		if f.prodOnlyTools[tool.Name] && env != "production" {
			continue
		}

		filtered = append(filtered, tool)
	}

	return filtered
}

// TimeBasedFilter filters tools based on time restrictions
type TimeBasedFilter struct {
	maintenanceTools map[string]bool
	businessHours    struct {
		StartHour int
		EndHour   int
		Weekdays  []time.Weekday
	}
}

// NewTimeBasedFilter creates a new time-based filter
func NewTimeBasedFilter() *TimeBasedFilter {
	return &TimeBasedFilter{
		maintenanceTools: map[string]bool{
			"backup_database":   true,
			"cleanup_logs":      true,
			"restart_service":   true,
			"optimize_database": true,
			"purge_cache":       true,
		},
		businessHours: struct {
			StartHour int
			EndHour   int
			Weekdays  []time.Weekday
		}{
			StartHour: 9,
			EndHour:   17,
			Weekdays: []time.Weekday{
				time.Monday,
				time.Tuesday,
				time.Wednesday,
				time.Thursday,
				time.Friday,
			},
		},
	}
}

// Apply filters tools based on current time
func (f *TimeBasedFilter) Apply(ctx context.Context, tools []mcp.Tool) []mcp.Tool {
	now := time.Now()
	hour := now.Hour()
	weekday := now.Weekday()

	// Check if we're in business hours
	inBusinessHours := false
	for _, wd := range f.businessHours.Weekdays {
		if weekday == wd && hour >= f.businessHours.StartHour && hour < f.businessHours.EndHour {
			inBusinessHours = true
			break
		}
	}

	var filtered []mcp.Tool
	for _, tool := range tools {
		// Maintenance tools only allowed outside business hours
		if f.maintenanceTools[tool.Name] && inBusinessHours {
			continue
		}

		filtered = append(filtered, tool)
	}

	return filtered
}

// FeatureFlagFilter filters tools based on feature flags
type FeatureFlagFilter struct {
	featureFlags map[string]bool
	toolFlags    map[string]string // tool name -> required feature flag
}

// NewFeatureFlagFilter creates a new feature flag filter
func NewFeatureFlagFilter(featureFlags map[string]bool) *FeatureFlagFilter {
	return &FeatureFlagFilter{
		featureFlags: featureFlags,
		toolFlags: map[string]string{
			"ai_assistant":        "feature_ai",
			"advanced_analytics":  "feature_analytics",
			"workflow_automation": "feature_automation",
			"custom_integrations": "feature_integrations",
		},
	}
}

// Apply filters tools based on feature flags
func (f *FeatureFlagFilter) Apply(ctx context.Context, tools []mcp.Tool) []mcp.Tool {
	var filtered []mcp.Tool
	for _, tool := range tools {
		// Check if tool requires a feature flag
		if requiredFlag, exists := f.toolFlags[tool.Name]; exists {
			// Check if feature is enabled
			if !f.featureFlags[requiredFlag] {
				continue
			}
		}

		filtered = append(filtered, tool)
	}

	return filtered
}

// UsageQuotaFilter filters tools based on usage quotas
type UsageQuotaFilter struct {
	quotaChecker func(sessionID string, toolName string) bool
}

// NewUsageQuotaFilter creates a new usage quota filter
func NewUsageQuotaFilter(quotaChecker func(sessionID string, toolName string) bool) *UsageQuotaFilter {
	return &UsageQuotaFilter{
		quotaChecker: quotaChecker,
	}
}

// Apply filters tools based on usage quotas
func (f *UsageQuotaFilter) Apply(ctx context.Context, tools []mcp.Tool) []mcp.Tool {
	sess, ok := session.SessionFromContext(ctx)
	if !ok {
		// No session, can't check quotas
		return tools
	}

	var filtered []mcp.Tool
	for _, tool := range tools {
		// Check if user has quota for this tool
		if f.quotaChecker != nil && !f.quotaChecker(sess.ID, tool.Name) {
			continue
		}

		filtered = append(filtered, tool)
	}

	return filtered
}

// CustomFilter allows custom filtering logic
type CustomFilter struct {
	filterFunc ToolFilter
}

// NewCustomFilter creates a new custom filter
func NewCustomFilter(filterFunc ToolFilter) *CustomFilter {
	return &CustomFilter{
		filterFunc: filterFunc,
	}
}

// Apply applies the custom filter
func (f *CustomFilter) Apply(ctx context.Context, tools []mcp.Tool) []mcp.Tool {
	if f.filterFunc == nil {
		return tools
	}
	return f.filterFunc(ctx, tools)
}

// DefaultToolPermissions returns default tool permissions
func DefaultToolPermissions() map[string][]string {
	return map[string][]string{
		// Admin-only tools
		"delete_user":       {"admin"},
		"modify_system":     {"admin"},
		"manage_runners":    {"admin"},
		"rotate_secrets":    {"admin"},
		"backup_production": {"admin"},

		// Admin or operator tools
		"create_integration": {"admin", "operator"},
		"manage_sources":     {"admin", "operator"},
		"scale_service":      {"admin", "operator"},
		"restart_service":    {"admin", "operator"},

		// Tools available to all authenticated users
		"list_runners":     {"admin", "operator", "user"},
		"execute_tool":     {"admin", "operator", "user"},
		"list_sources":     {"admin", "operator", "user"},
		"search_kb":        {"admin", "operator", "user"},
		"execute_workflow": {"admin", "operator", "user"},
	}
}
