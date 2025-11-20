package entities

// Model represents an LLM model in the control plane
type Model struct {
	ID                 string                 `json:"id,omitempty"`
	Value              string                 `json:"value"`              // Model identifier (e.g., "kubiya/claude-sonnet-4")
	Label              string                 `json:"label"`              // Display name (e.g., "Claude Sonnet 4")
	Provider           string                 `json:"provider"`           // Provider name (e.g., "Anthropic", "OpenAI")
	Logo               *string                `json:"logo,omitempty"`     // Logo path or URL
	Description        *string                `json:"description,omitempty"`
	Enabled            bool                   `json:"enabled"`
	Recommended        bool                   `json:"recommended"`         // Whether model is recommended/default
	CompatibleRuntimes []string               `json:"compatible_runtimes"` // Compatible runtime types
	Capabilities       map[string]interface{} `json:"capabilities"`        // Model capabilities (vision, function_calling, etc.)
	Pricing            map[string]interface{} `json:"pricing,omitempty"`   // Pricing information
	DisplayOrder       int                    `json:"display_order"`       // Display order (lower = shown first)
	CreatedAt          *CustomTime            `json:"created_at,omitempty"`
	UpdatedAt          *CustomTime            `json:"updated_at,omitempty"`
}

// ModelCreateRequest represents the request to create a model
type ModelCreateRequest struct {
	Value              string                 `json:"value"`
	Label              string                 `json:"label"`
	Provider           string                 `json:"provider"`
	Logo               *string                `json:"logo,omitempty"`
	Description        *string                `json:"description,omitempty"`
	Enabled            *bool                  `json:"enabled,omitempty"`
	Recommended        *bool                  `json:"recommended,omitempty"`
	CompatibleRuntimes []string               `json:"compatible_runtimes,omitempty"`
	Capabilities       map[string]interface{} `json:"capabilities,omitempty"`
	Pricing            map[string]interface{} `json:"pricing,omitempty"`
	DisplayOrder       *int                   `json:"display_order,omitempty"`
}

// ModelUpdateRequest represents the request to update a model
type ModelUpdateRequest struct {
	Value              *string                `json:"value,omitempty"`
	Label              *string                `json:"label,omitempty"`
	Provider           *string                `json:"provider,omitempty"`
	Logo               *string                `json:"logo,omitempty"`
	Description        *string                `json:"description,omitempty"`
	Enabled            *bool                  `json:"enabled,omitempty"`
	Recommended        *bool                  `json:"recommended,omitempty"`
	CompatibleRuntimes []string               `json:"compatible_runtimes,omitempty"`
	Capabilities       map[string]interface{} `json:"capabilities,omitempty"`
	Pricing            map[string]interface{} `json:"pricing,omitempty"`
	DisplayOrder       *int                   `json:"display_order,omitempty"`
}
