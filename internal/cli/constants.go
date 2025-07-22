package cli

// Default group IDs that should always have access to agents
const (
	// AdminGroupID is the default admin group that should always have access to agents
	// This should be updated with the actual Admin group UUID from your Kubiya workspace
	AdminGroupID = "admin-group-id-placeholder"
	
	// Common group IDs that are typically used in Kubiya workspaces
	// Update these with actual group UUIDs from your environment
	OperatorGroupID = "operator-group-id-placeholder"
)

// GetDefaultAllowedGroups returns the default groups that should always have access to agents
func GetDefaultAllowedGroups() []string {
	// Only include groups that are not placeholders
	var defaultGroups []string
	
	if AdminGroupID != "admin-group-id-placeholder" {
		defaultGroups = append(defaultGroups, AdminGroupID)
	}
	
	if OperatorGroupID != "operator-group-id-placeholder" {
		defaultGroups = append(defaultGroups, OperatorGroupID)
	}
	
	return defaultGroups
}

// EnsureAdminGroupAccess ensures that admin groups are included in the allowed groups list
func EnsureAdminGroupAccess(allowedGroups []string) []string {
	defaultGroups := GetDefaultAllowedGroups()
	
	// Create a map to track existing groups
	existingGroups := make(map[string]bool)
	for _, group := range allowedGroups {
		existingGroups[group] = true
	}
	
	// Add default groups if they don't already exist
	for _, defaultGroup := range defaultGroups {
		if !existingGroups[defaultGroup] {
			allowedGroups = append(allowedGroups, defaultGroup)
		}
	}
	
	return allowedGroups
}

// RemoveAdminGroupFromRestrictions removes admin groups from a restriction list
// This is used when clearing access but we want to preserve admin access
func RemoveAdminGroupFromRestrictions(allowedGroups []string) []string {
	defaultGroups := GetDefaultAllowedGroups()
	defaultGroupsMap := make(map[string]bool)
	for _, group := range defaultGroups {
		defaultGroupsMap[group] = true
	}
	
	var filteredGroups []string
	for _, group := range allowedGroups {
		// Keep groups that are NOT default admin groups
		if !defaultGroupsMap[group] {
			filteredGroups = append(filteredGroups, group)
		}
	}
	
	return filteredGroups
}