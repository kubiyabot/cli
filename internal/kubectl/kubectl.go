package kubectl

import (
	"bytes"
	"fmt"
	"os/exec"
	"strings"
)

// GetCurrentContext returns the current Kubernetes context
func GetCurrentContext() (string, error) {
	cmd := exec.Command("kubectl", "config", "current-context")
	var out bytes.Buffer
	cmd.Stdout = &out
	err := cmd.Run()
	if err != nil {
		return "", fmt.Errorf("failed to get current context: %w", err)
	}
	return strings.TrimSpace(out.String()), nil
}

// ListContexts returns all available Kubernetes contexts
func ListContexts() ([]string, error) {
	cmd := exec.Command("kubectl", "config", "get-contexts", "-o", "name")
	var out bytes.Buffer
	cmd.Stdout = &out
	err := cmd.Run()
	if err != nil {
		return nil, fmt.Errorf("failed to list contexts: %w", err)
	}

	contexts := strings.Split(strings.TrimSpace(out.String()), "\n")
	var result []string
	for _, ctx := range contexts {
		if ctx != "" {
			result = append(result, ctx)
		}
	}
	return result, nil
}

// SetContext sets the current Kubernetes context
func SetContext(context string) error {
	cmd := exec.Command("kubectl", "config", "use-context", context)
	var errOut bytes.Buffer
	cmd.Stderr = &errOut
	if err := cmd.Run(); err != nil {
		return fmt.Errorf("failed to set context: %s", errOut.String())
	}
	return nil
}

// ValidateContext checks if a context exists
func ValidateContext(context string) error {
	contexts, err := ListContexts()
	if err != nil {
		return err
	}
	for _, ctx := range contexts {
		if ctx == context {
			return nil
		}
	}
	return fmt.Errorf("context %q not found", context)
}
