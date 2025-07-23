package cli

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"strings"

	"github.com/kubiyabot/cli/internal/config"
)

// GitHubProvider implements the TriggerProviderInterface for GitHub
type GitHubProvider struct {
	cfg        *config.Config
	token      string
	baseURL    string
	httpClient *http.Client
}

// NewGitHubProvider creates a new GitHub provider instance using environment variables
func NewGitHubProvider(cfg *config.Config) (*GitHubProvider, error) {
	return NewGitHubProviderWithCredentials(cfg, "")
}

// NewGitHubProviderWithCredentials creates a new GitHub provider instance with explicit credentials
func NewGitHubProviderWithCredentials(cfg *config.Config, token string) (*GitHubProvider, error) {
	// Use provided token or fall back to environment variable
	if token == "" {
		token = os.Getenv("GITHUB_TOKEN")
	}

	if token == "" {
		return nil, fmt.Errorf("GITHUB_TOKEN environment variable or --github-token flag is required")
	}

	// Use GitHub API endpoint
	baseURL := "https://api.github.com"

	return &GitHubProvider{
		cfg:        cfg,
		token:      token,
		baseURL:    baseURL,
		httpClient: &http.Client{},
	}, nil
}

// CreateTrigger creates a new GitHub webhook trigger
func (g *GitHubProvider) CreateTrigger(trigger *Trigger) error {
	config, err := g.parseConfig(trigger.Config)
	if err != nil {
		return fmt.Errorf("invalid GitHub configuration: %w", err)
	}

	// Read workflow content (for future use in payload)
	_, err = g.readWorkflowContent(trigger.WorkflowRef)
	if err != nil {
		return fmt.Errorf("failed to read workflow: %w", err)
	}

	// Generate the webhook URL for Kubiya with proper runner
	runner := g.getRunnerFromConfig(config)
	kubiyaWebhookURL := fmt.Sprintf("https://api.kubiya.ai/api/v1/workflow?runner=%s&operation=execute_workflow", runner)

	// Prepare webhook payload for GitHub
	webhookPayload := map[string]interface{}{
		"name":   "web",
		"active": true,
		"events": config.Events,
		"config": map[string]interface{}{
			"url":          kubiyaWebhookURL,
			"content_type": "json",
			"insecure_ssl": "0",
		},
	}

	// Add secret if provided
	if config.Secret != "" {
		webhookPayload["config"].(map[string]interface{})["secret"] = config.Secret
	}

	// Parse repository owner/name
	parts := strings.Split(config.Repository, "/")
	if len(parts) != 2 {
		return fmt.Errorf("repository must be in format 'owner/repo', got: %s", config.Repository)
	}
	owner, repo := parts[0], parts[1]

	// Create the webhook in GitHub
	url := fmt.Sprintf("%s/repos/%s/%s/hooks", g.baseURL, owner, repo)

	fmt.Printf("🔗 Creating webhook at: %s\n", url)
	fmt.Printf("📍 Webhook URL: %s\n", kubiyaWebhookURL)
	fmt.Printf("📦 Repository: %s\n", config.Repository)
	fmt.Printf("🎯 Events: %s\n", strings.Join(config.Events, ", "))

	var result map[string]interface{}
	return g.makeGitHubRequest("POST", url, webhookPayload, &result)
}

// UpdateTrigger updates an existing GitHub webhook trigger
func (g *GitHubProvider) UpdateTrigger(trigger *Trigger) error {
	// GitHub webhooks would need webhook ID to update
	// For now, we'll recreate (delete + create)
	return fmt.Errorf("GitHub webhook updates not yet implemented - please delete and recreate")
}

// DeleteTrigger removes a GitHub webhook trigger by webhook ID
func (g *GitHubProvider) DeleteTrigger(repository, webhookID string) error {
	// Parse repository owner/name
	parts := strings.Split(repository, "/")
	if len(parts) != 2 {
		return fmt.Errorf("repository must be in format 'owner/repo', got: %s", repository)
	}
	owner, repo := parts[0], parts[1]

	// Delete the webhook from GitHub
	url := fmt.Sprintf("%s/repos/%s/%s/hooks/%s", g.baseURL, owner, repo, webhookID)
	
	fmt.Printf("🗑️  Deleting webhook: %s\n", webhookID)
	fmt.Printf("📦 Repository: %s\n", repository)
	fmt.Printf("🔗 DELETE URL: %s\n", url)
	
	return g.makeGitHubRequest("DELETE", url, nil, nil)
}

// ListWebhooks lists all webhooks for a repository and identifies Kubiya ones
func (g *GitHubProvider) ListWebhooks(repository string) ([]WebhookInfo, error) {
	// Parse repository owner/name
	parts := strings.Split(repository, "/")
	if len(parts) != 2 {
		return nil, fmt.Errorf("repository must be in format 'owner/repo', got: %s", repository)
	}
	owner, repo := parts[0], parts[1]

	url := fmt.Sprintf("%s/repos/%s/%s/hooks", g.baseURL, owner, repo)
	
	var response []map[string]interface{}
	if err := g.makeGitHubRequest("GET", url, nil, &response); err != nil {
		return nil, fmt.Errorf("failed to list webhooks: %w", err)
	}

	var webhooks []WebhookInfo
	for _, webhook := range response {
		id := fmt.Sprintf("%.0f", webhook["id"].(float64))
		name, _ := webhook["name"].(string)
		
		var webhookURL string
		var events []string
		
		if config, ok := webhook["config"].(map[string]interface{}); ok {
			webhookURL, _ = config["url"].(string)
		}
		
		if eventsList, ok := webhook["events"].([]interface{}); ok {
			for _, event := range eventsList {
				if eventStr, ok := event.(string); ok {
					events = append(events, eventStr)
				}
			}
		}
		
		// Check if this webhook points to Kubiya API
		isKubiya := strings.Contains(webhookURL, "api.kubiya.ai") || strings.Contains(webhookURL, "kubiya")
		
		webhooks = append(webhooks, WebhookInfo{
			ID:       id,
			Name:     name,
			URL:      webhookURL,
			Provider: "github",
			IsKubiya: isKubiya,
			Events:   events,
		})
	}
	
	return webhooks, nil
}

// TestTrigger sends a test event to verify the trigger works
func (g *GitHubProvider) TestTrigger(triggerID string) error {
	// GitHub has a test webhook functionality we could use
	return fmt.Errorf("test GitHub trigger not yet implemented")
}

// ValidateConfig validates the GitHub-specific configuration
func (g *GitHubProvider) ValidateConfig(config map[string]interface{}) error {
	repository, ok := config["repository"].(string)
	if !ok || repository == "" {
		return fmt.Errorf("repository is required")
	}

	// Validate repository format (owner/repo)
	parts := strings.Split(repository, "/")
	if len(parts) != 2 {
		return fmt.Errorf("repository must be in format 'owner/repo', got: %s", repository)
	}

	// Validate events
	events, ok := config["events"].([]interface{})
	if !ok || len(events) == 0 {
		return fmt.Errorf("at least one event is required")
	}

	// Convert and validate event names
	validEvents := map[string]bool{
		"push": true, "pull_request": true, "issues": true, "issue_comment": true,
		"pull_request_review": true, "pull_request_review_comment": true,
		"commit_comment": true, "create": true, "delete": true, "deployment": true,
		"deployment_status": true, "fork": true, "gollum": true, "label": true,
		"member": true, "membership": true, "milestone": true, "organization": true,
		"org_block": true, "page_build": true, "project": true, "project_card": true,
		"project_column": true, "public": true, "release": true, "repository": true,
		"status": true, "team": true, "team_add": true, "watch": true,
	}

	for _, event := range events {
		eventStr, ok := event.(string)
		if !ok {
			return fmt.Errorf("event must be a string, got: %T", event)
		}
		if !validEvents[eventStr] {
			return fmt.Errorf("invalid event: %s", eventStr)
		}
	}

	return nil
}

// GetRequiredEnvVars returns the required environment variables for GitHub
func (g *GitHubProvider) GetRequiredEnvVars() []string {
	return []string{
		"GITHUB_TOKEN",
	}
}

// Helper methods

func (g *GitHubProvider) parseConfig(config map[string]interface{}) (*GitHubTriggerConfig, error) {
	data, err := json.Marshal(config)
	if err != nil {
		return nil, err
	}

	var ghConfig GitHubTriggerConfig
	if err := json.Unmarshal(data, &ghConfig); err != nil {
		return nil, err
	}

	return &ghConfig, nil
}

func (g *GitHubProvider) makeGitHubRequest(method, url string, payload interface{}, result interface{}) error {
	var body io.Reader
	if payload != nil {
		jsonData, err := json.Marshal(payload)
		if err != nil {
			return fmt.Errorf("failed to marshal request: %w", err)
		}
		body = bytes.NewReader(jsonData)
	}

	req, err := http.NewRequest(method, url, body)
	if err != nil {
		return fmt.Errorf("failed to create request: %w", err)
	}

	req.Header.Set("Accept", "application/vnd.github.v3+json")
	req.Header.Set("Authorization", "token "+g.token)
	if payload != nil {
		req.Header.Set("Content-Type", "application/json")
	}

	resp, err := g.httpClient.Do(req)
	if err != nil {
		return fmt.Errorf("request failed: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		bodyBytes, _ := io.ReadAll(resp.Body)
		return fmt.Errorf("GitHub API error (status %d): %s", resp.StatusCode, string(bodyBytes))
	}

	if result != nil {
		return json.NewDecoder(resp.Body).Decode(result)
	}

	return nil
}

func (g *GitHubProvider) getRunnerFromConfig(config *GitHubTriggerConfig) string {
	// Get runner from environment variable or use default
	if runner := os.Getenv("KUBIYA_RUNNER"); runner != "" {
		return runner
	}
	return "gke-integration" // Default runner
}

func (g *GitHubProvider) readWorkflowContent(workflowPath string) (string, error) {
	// Read workflow file content
	content, err := os.ReadFile(workflowPath)
	if err != nil {
		return "", fmt.Errorf("failed to read workflow file %s: %w", workflowPath, err)
	}
	return string(content), nil
}