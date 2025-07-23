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

// DatadogProvider implements the TriggerProviderInterface for Datadog
type DatadogProvider struct {
	cfg        *config.Config
	apiKey     string
	appKey     string
	baseURL    string
	httpClient *http.Client
}

// NewDatadogProvider creates a new Datadog provider instance using environment variables
func NewDatadogProvider(cfg *config.Config) (*DatadogProvider, error) {
	return NewDatadogProviderWithCredentials(cfg, "", "", "")
}

// NewDatadogProviderWithCredentials creates a new Datadog provider instance with explicit credentials
func NewDatadogProviderWithCredentials(cfg *config.Config, apiKey, appKey, site string) (*DatadogProvider, error) {
	// Use provided credentials or fall back to environment variables
	if apiKey == "" {
		apiKey = os.Getenv("DD_API_KEY")
	}
	if appKey == "" {
		appKey = os.Getenv("DD_APPLICATION_KEY")
	}
	if site == "" {
		site = os.Getenv("DD_SITE")
	}

	if apiKey == "" {
		return nil, fmt.Errorf("DD_API_KEY environment variable or --dd-api-key flag is required")
	}

	if appKey == "" {
		return nil, fmt.Errorf("DD_APPLICATION_KEY environment variable or --dd-app-key flag is required")
	}

	// Use custom endpoint if provided, otherwise default to US
	baseURL := site
	if baseURL == "" {
		baseURL = "https://api.datadoghq.com"
	} else {
		// For custom sites, use the site directly (e.g., custom.datadoghq.com)
		baseURL = fmt.Sprintf("https://%s", baseURL)
	}

	return &DatadogProvider{
		cfg:        cfg,
		apiKey:     apiKey,
		appKey:     appKey,
		baseURL:    baseURL,
		httpClient: &http.Client{},
	}, nil
}

// CreateTrigger creates a new Datadog webhook trigger
func (d *DatadogProvider) CreateTrigger(trigger *Trigger) error {
	config, err := d.parseConfig(trigger.Config)
	if err != nil {
		return fmt.Errorf("invalid Datadog configuration: %w", err)
	}

	// Read workflow content
	workflowContent, err := d.readWorkflowContent(trigger.WorkflowRef)
	if err != nil {
		return fmt.Errorf("failed to read workflow: %w", err)
	}

	// Generate the webhook URL for Kubiya with proper runner
	runner := d.getRunnerFromConfig(config)
	kubiyaWebhookURL := fmt.Sprintf("https://api.kubiya.ai/api/v1/workflow?runner=%s&operation=execute_workflow", runner)

	// Prepare webhook payload with workflow content
	webhookPayload := map[string]interface{}{
		"name":           fmt.Sprintf("webhooks/%s", config.WebhookName),
		"url":            kubiyaWebhookURL,
		"encode_as":      d.getEncodeAs(config),
		"custom_headers": d.getCustomHeaders(config, trigger, workflowContent),
		"payload":        d.getPayload(config, workflowContent),
	}

	// Create the webhook in Datadog using the correct webhook integration API
	url := fmt.Sprintf("%s/api/v1/integration/webhooks", d.baseURL)

	fmt.Printf("🔗 Creating webhook at: %s\n", url)
	fmt.Printf("📍 Webhook URL: %s\n", kubiyaWebhookURL)

	return d.makeDatadogRequest("POST", url, webhookPayload, nil)
}

// UpdateTrigger updates an existing Datadog webhook trigger
func (d *DatadogProvider) UpdateTrigger(trigger *Trigger) error {
	// For updates, we can use the same logic as create since PUT is idempotent
	return d.CreateTrigger(trigger)
}

// DeleteTrigger removes a Datadog webhook trigger by webhook name
func (d *DatadogProvider) DeleteTrigger(webhookName string) error {
	// Delete the webhook from Datadog using the webhook integration API
	url := fmt.Sprintf("%s/api/v1/integration/webhooks/%s", 
		d.baseURL, webhookName)
	
	fmt.Printf("🗑️  Deleting webhook: %s\n", webhookName)
	fmt.Printf("🔗 DELETE URL: %s\n", url)
	
	return d.makeDatadogRequest("DELETE", url, nil, nil)
}

// ListWebhooks lists all webhooks and identifies Kubiya ones
func (d *DatadogProvider) ListWebhooks() ([]WebhookInfo, error) {
	// Try the correct webhook integration API endpoint
	url := fmt.Sprintf("%s/api/v1/integration/webhooks", d.baseURL)
	fmt.Printf("🔗 Listing webhooks at: %s\n", url)
	
	var response []map[string]interface{}
	if err := d.makeDatadogRequest("GET", url, nil, &response); err != nil {
		return nil, fmt.Errorf("failed to list webhooks: %w", err)
	}

	var webhooks []WebhookInfo
	for _, webhook := range response {
		name, _ := webhook["name"].(string)
		webhookURL, _ := webhook["url"].(string)
		
		// Check if this webhook points to Kubiya API
		isKubiya := strings.Contains(webhookURL, "api.kubiya.ai") || strings.Contains(webhookURL, "kubiya")
		
		webhooks = append(webhooks, WebhookInfo{
			Name:     name,
			URL:      webhookURL,
			Provider: "datadog",
			IsKubiya: isKubiya,
		})
	}
	
	return webhooks, nil
}

// TestTrigger sends a test event to verify the trigger works
func (d *DatadogProvider) TestTrigger(triggerID string) error {
	// This would simulate a webhook call to test the integration
	// Implementation depends on how we want to test (synthetic event, etc.)
	return fmt.Errorf("test trigger not yet implemented")
}

// ValidateConfig validates the Datadog-specific configuration
func (d *DatadogProvider) ValidateConfig(config map[string]interface{}) error {
	webhookName, ok := config["webhook_name"].(string)
	if !ok || webhookName == "" {
		return fmt.Errorf("webhook_name is required")
	}

	// Validate webhook name format (should be valid for Datadog)
	if strings.Contains(webhookName, " ") {
		return fmt.Errorf("webhook_name cannot contain spaces")
	}

	return nil
}

// GetRequiredEnvVars returns the required environment variables for Datadog
func (d *DatadogProvider) GetRequiredEnvVars() []string {
	return []string{
		"DD_API_KEY",
		"DD_APPLICATION_KEY",
	}
}

// Helper methods

func (d *DatadogProvider) parseConfig(config map[string]interface{}) (*DatadogTriggerConfig, error) {
	data, err := json.Marshal(config)
	if err != nil {
		return nil, err
	}

	var ddConfig DatadogTriggerConfig
	if err := json.Unmarshal(data, &ddConfig); err != nil {
		return nil, err
	}

	return &ddConfig, nil
}

func (d *DatadogProvider) makeDatadogRequest(method, url string, payload interface{}, result interface{}) error {
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

	req.Header.Set("Accept", "application/json")
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("DD-API-KEY", d.apiKey)
	req.Header.Set("DD-APPLICATION-KEY", d.appKey)

	resp, err := d.httpClient.Do(req)
	if err != nil {
		return fmt.Errorf("request failed: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		bodyBytes, _ := io.ReadAll(resp.Body)
		return fmt.Errorf("Datadog API error (status %d): %s", resp.StatusCode, string(bodyBytes))
	}

	if result != nil {
		return json.NewDecoder(resp.Body).Decode(result)
	}

	return nil
}

func (d *DatadogProvider) getRunnerName() string {
	// Get runner from config or use default
	if runner := os.Getenv("KUBIYA_RUNNER"); runner != "" {
		return runner
	}
	return "gke-integration" // Default runner
}

func (d *DatadogProvider) getRunnerFromConfig(config *DatadogTriggerConfig) string {
	// Check config first, then environment, then default
	if runner, exists := config.Environment["KUBIYA_RUNNER"]; exists && runner != "" {
		return runner
	}
	return d.getRunnerName()
}

func (d *DatadogProvider) readWorkflowContent(workflowPath string) (string, error) {
	// Read workflow file content
	content, err := os.ReadFile(workflowPath)
	if err != nil {
		return "", fmt.Errorf("failed to read workflow file %s: %w", workflowPath, err)
	}
	return string(content), nil
}

func (d *DatadogProvider) getEncodeAs(config *DatadogTriggerConfig) string {
	if config.EncodeAs != "" {
		return config.EncodeAs
	}
	return "json"
}

func (d *DatadogProvider) getCustomHeaders(config *DatadogTriggerConfig, trigger *Trigger, workflowContent string) string {
	if config.CustomHeaders != "" {
		return config.CustomHeaders
	}

	// Default headers for Kubiya workflow execution
	return "User-Agent: Datadog-Webhook-1.0\nContent-Type: application/yaml\nX-Trigger-ID: " + trigger.ID
}

func (d *DatadogProvider) getPayload(config *DatadogTriggerConfig, workflowContent string) string {
	if config.Payload != "" {
		return config.Payload
	}

	// Default payload includes the workflow content in the request body
	// This allows the workflow to be passed as the request body to Kubiya API
	return fmt.Sprintf(`{
		"workflow": %q,
		"event_data": {
			"body": "$EVENT_MSG",
			"title": "$EVENT_TITLE", 
			"date": "$DATE",
			"id": "$ID",
			"priority": "$PRIORITY",
			"tags": "$TAGS"
		}
	}`, workflowContent)
}
