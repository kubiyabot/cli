package cli

import (
	"context"
	"fmt"
	"os"

	"github.com/kubiyabot/cli/internal/config"
	"github.com/kubiyabot/cli/internal/kubiya"
)

// createClient creates a new Kubiya client
func createClient() (*kubiya.Client, error) {
	// Check for API token
	apiKey := os.Getenv("KUBIYA_API_KEY")
	if apiKey == "" {
		return nil, fmt.Errorf("KUBIYA_API_KEY environment variable is not set")
	}

	// Create config from API key
	cfg := &config.Config{
		APIKey:  apiKey,
		BaseURL: os.Getenv("KUBIYA_BASE_URL"),
		Debug:   os.Getenv("KUBIYA_DEBUG") == "true",
	}

	// Create new client
	client := kubiya.NewClient(cfg)
	return client, nil
}

// formatSourceType returns a formatted source type with an emoji
func formatSourceType(sourceType string) string {
	switch sourceType {
	case "git":
		return "🔄 git"
	case "local":
		return "📁 local"
	default:
		return "❓ " + sourceType
	}
}

// primaryText formats text for emphasis
func primaryText(text string) string {
	return fmt.Sprintf("\033[1;36m%s\033[0m", text) // Cyan bold
}

// Helper for loading tools from a source
func loadTools(ctx context.Context, client *kubiya.Client, sourceName string) ([]kubiya.Tool, error) {
	// In a real implementation, this would call the API
	// For now, we'll return mock data
	return []kubiya.Tool{
		{
			Name:        "sample-tool",
			Description: "A sample tool for demonstration purposes",
		},
	}, nil
}
