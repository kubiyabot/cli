package context

// Config represents the kubectl-style configuration file
type Config struct {
	APIVersion     string            `yaml:"apiVersion"`
	Kind           string            `yaml:"kind"`
	CurrentContext string            `yaml:"current-context"`
	Contexts       []NamedContext    `yaml:"contexts"`
	Users          []NamedUser       `yaml:"users"`
	Organizations  []NamedOrganization `yaml:"organizations,omitempty"`
}

// NamedContext represents a named context
type NamedContext struct {
	Name    string  `yaml:"name"`
	Context Context `yaml:"context"`
}

// Context represents a context configuration
type Context struct {
	APIURL       string              `yaml:"api-url"`
	Organization string              `yaml:"organization"`
	User         string              `yaml:"user"`
	UseV1API     bool                `yaml:"use-v1-api,omitempty"`
	LiteLLMProxy *LiteLLMProxyConfig `yaml:"litellm-proxy,omitempty"`
}

// LiteLLMProxyConfig represents local LiteLLM proxy configuration
type LiteLLMProxyConfig struct {
	Enabled    bool   `yaml:"enabled"`
	ConfigFile string `yaml:"config-file,omitempty"` // Path to config file
	ConfigJSON string `yaml:"config-json,omitempty"` // Inline JSON config
}

// NamedUser represents a named user
type NamedUser struct {
	Name string `yaml:"name"`
	User User   `yaml:"user"`
}

// User represents user credentials
type User struct {
	Token string `yaml:"token"`
}

// NamedOrganization represents a named organization
type NamedOrganization struct {
	Name         string       `yaml:"name"`
	Organization Organization `yaml:"organization"`
}

// Organization represents organization details
type Organization struct {
	ID   string `yaml:"id"`
	Name string `yaml:"name"`
}
