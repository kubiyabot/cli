package entities

// Context represents contextual information for an entity
type Context struct {
	EntityType string      `json:"entity_type"` // environment, team, project, agent
	EntityID   string      `json:"entity_id"`
	Knowledge  []string    `json:"knowledge,omitempty"`  // Markdown knowledge base entries
	Resources  []string    `json:"resources,omitempty"`  // Resource references
	Policies   []string    `json:"policies,omitempty"`   // Policy IDs
	CreatedAt  *CustomTime `json:"created_at,omitempty"`
	UpdatedAt  *CustomTime `json:"updated_at,omitempty"`
}

// ContextRequest represents a request to set context
type ContextRequest struct {
	Knowledge []string `json:"knowledge,omitempty"`
	Resources []string `json:"resources,omitempty"`
	Policies  []string `json:"policies,omitempty"`
}

// ResolvedContext represents a context resolved with inheritance
type ResolvedContext struct {
	Knowledge  []string   `json:"knowledge,omitempty"`
	Resources  []string   `json:"resources,omitempty"`
	Policies   []string   `json:"policies,omitempty"`
	Sources    []string   `json:"sources,omitempty"` // Where each piece came from
}
