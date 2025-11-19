package context

import (
	"fmt"
	"os"
)

// GetCurrentContext returns the current context
func GetCurrentContext() (*Context, string, error) {
	config, err := LoadConfig()
	if err != nil {
		return nil, "", err
	}

	// Check for environment variable override
	if envContext := os.Getenv("KUBIYA_CONTEXT"); envContext != "" {
		for _, nc := range config.Contexts {
			if nc.Name == envContext {
				return &nc.Context, nc.Name, nil
			}
		}
		return nil, "", fmt.Errorf("context %q not found (from KUBIYA_CONTEXT)", envContext)
	}

	if config.CurrentContext == "" {
		return nil, "", fmt.Errorf("no current context set")
	}

	for _, nc := range config.Contexts {
		if nc.Name == config.CurrentContext {
			return &nc.Context, nc.Name, nil
		}
	}

	return nil, "", fmt.Errorf("current context %q not found", config.CurrentContext)
}

// SetCurrentContext sets the current context
func SetCurrentContext(name string) error {
	config, err := LoadConfig()
	if err != nil {
		return err
	}

	// Verify context exists
	found := false
	for _, nc := range config.Contexts {
		if nc.Name == name {
			found = true
			break
		}
	}

	if !found {
		return fmt.Errorf("context %q not found", name)
	}

	config.CurrentContext = name
	return SaveConfig(config)
}

// ListContexts returns all contexts
func ListContexts() ([]NamedContext, string, error) {
	config, err := LoadConfig()
	if err != nil {
		return nil, "", err
	}

	return config.Contexts, config.CurrentContext, nil
}

// CreateContext creates a new context
func CreateContext(name string, ctx Context) error {
	config, err := LoadConfig()
	if err != nil {
		return err
	}

	// Check if context already exists
	for i, nc := range config.Contexts {
		if nc.Name == name {
			// Update existing context
			config.Contexts[i].Context = ctx
			return SaveConfig(config)
		}
	}

	// Add new context
	config.Contexts = append(config.Contexts, NamedContext{
		Name:    name,
		Context: ctx,
	})

	// If this is the first context, set it as current
	if config.CurrentContext == "" {
		config.CurrentContext = name
	}

	return SaveConfig(config)
}

// DeleteContext deletes a context
func DeleteContext(name string) error {
	config, err := LoadConfig()
	if err != nil {
		return err
	}

	// Find and remove context
	found := false
	for i, nc := range config.Contexts {
		if nc.Name == name {
			config.Contexts = append(config.Contexts[:i], config.Contexts[i+1:]...)
			found = true
			break
		}
	}

	if !found {
		return fmt.Errorf("context %q not found", name)
	}

	// If we deleted the current context, clear it
	if config.CurrentContext == name {
		config.CurrentContext = ""
		// Set to first available context if any
		if len(config.Contexts) > 0 {
			config.CurrentContext = config.Contexts[0].Name
		}
	}

	return SaveConfig(config)
}

// RenameContext renames a context
func RenameContext(oldName, newName string) error {
	config, err := LoadConfig()
	if err != nil {
		return err
	}

	// Find and rename context
	found := false
	for i, nc := range config.Contexts {
		if nc.Name == oldName {
			config.Contexts[i].Name = newName
			found = true
			break
		}
	}

	if !found {
		return fmt.Errorf("context %q not found", oldName)
	}

	// Update current context if it was renamed
	if config.CurrentContext == oldName {
		config.CurrentContext = newName
	}

	return SaveConfig(config)
}

// GetUser gets a user by name
func GetUser(name string) (*User, error) {
	config, err := LoadConfig()
	if err != nil {
		return nil, err
	}

	for _, nu := range config.Users {
		if nu.Name == name {
			return &nu.User, nil
		}
	}

	return nil, fmt.Errorf("user %q not found", name)
}

// SetUser sets or updates a user
func SetUser(name string, user User) error {
	config, err := LoadConfig()
	if err != nil {
		return err
	}

	// Check if user already exists
	for i, nu := range config.Users {
		if nu.Name == name {
			// Update existing user
			config.Users[i].User = user
			return SaveConfig(config)
		}
	}

	// Add new user
	config.Users = append(config.Users, NamedUser{
		Name: name,
		User: user,
	})

	return SaveConfig(config)
}

// GetAPIKey gets the API key for the current context
func GetAPIKey() (string, error) {
	ctx, _, err := GetCurrentContext()
	if err != nil {
		return "", err
	}

	user, err := GetUser(ctx.User)
	if err != nil {
		return "", err
	}

	return user.Token, nil
}

// ShouldUseV1API returns whether to use V1 API based on context and environment
func ShouldUseV1API() bool {
	// Check global environment variable first
	if os.Getenv("KUBIYA_CLI_USE_V1_API") == "true" {
		return true
	}

	// Check context-specific setting
	ctx, _, err := GetCurrentContext()
	if err != nil {
		return false // Default to V2
	}

	return ctx.UseV1API
}
