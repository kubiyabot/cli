package kubiya

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
)

// ListWebhooks retrieves all webhooks
func (c *Client) ListWebhooks(ctx context.Context) ([]Webhook, error) {
	if c.debug {
		fmt.Printf("Listing webhooks using get helper\n")
	}

	resp, err := c.get(ctx, "/event")
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	var webhooks []Webhook
	if err := json.NewDecoder(resp.Body).Decode(&webhooks); err != nil {
		if c.debug {
			fmt.Printf("Error decoding webhook list response: %v\n", err)
		}
		return nil, err
	}

	if c.debug {
		fmt.Printf("Found %d webhooks\n", len(webhooks))
	}

	return webhooks, nil
}

// GetWebhook retrieves a specific webhook by ID
func (c *Client) GetWebhook(ctx context.Context, id string) (*Webhook, error) {
	if c.debug {
		fmt.Printf("Getting webhook %s using get helper\n", id)
	}

	resp, err := c.get(ctx, fmt.Sprintf("/event/%s", id))
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	var webhook Webhook
	if err := json.NewDecoder(resp.Body).Decode(&webhook); err != nil {
		return nil, err
	}

	return &webhook, nil
}

// UpdateWebhook updates an existing webhook
func (c *Client) UpdateWebhook(ctx context.Context, id string, webhook Webhook) (*Webhook, error) {
	if c.debug {
		fmt.Printf("Updating webhook %s using put helper\n", id)
	}

	resp, err := c.put(ctx, fmt.Sprintf("/event/%s", id), webhook)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	var updated Webhook
	if err := json.NewDecoder(resp.Body).Decode(&updated); err != nil {
		return nil, err
	}

	return &updated, nil
}

// CreateWebhook creates a new webhook
func (c *Client) CreateWebhook(ctx context.Context, webhook Webhook) (*Webhook, error) {
	// Use the client's post helper method which handles URL formatting consistently
	if c.debug {
		fmt.Printf("Creating webhook using post helper\n")
		data, _ := json.MarshalIndent(webhook, "", "  ")
		fmt.Printf("Webhook payload: %s\n", string(data))
	}

	resp, err := c.post(ctx, "/event", webhook)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		return nil, fmt.Errorf("unexpected status code: %d, response: %s", resp.StatusCode, string(body))
	}

	var created Webhook
	if err := json.NewDecoder(resp.Body).Decode(&created); err != nil {
		return nil, err
	}

	return &created, nil
}

// DeleteWebhook deletes a webhook by ID
func (c *Client) DeleteWebhook(ctx context.Context, id string) error {
	if c.debug {
		fmt.Printf("Deleting webhook %s using delete helper\n", id)
	}

	resp, err := c.delete(ctx, fmt.Sprintf("/event/%s", id))
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	// Parse response
	var result struct {
		Result string `json:"result"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return fmt.Errorf("failed to decode response: %w", err)
	}

	if result.Result != "ok" {
		return fmt.Errorf("unexpected response: %s", result.Result)
	}

	return nil
}

// TestWebhook sends test data to a webhook
func (c *Client) TestWebhook(ctx context.Context, webhookURL string, testData interface{}) error {
	data, err := json.Marshal(testData)
	if err != nil {
		return fmt.Errorf("failed to marshal test data: %w", err)
	}

	req, err := http.NewRequestWithContext(ctx, "POST", webhookURL, bytes.NewBuffer(data))
	if err != nil {
		return fmt.Errorf("failed to create request: %w", err)
	}

	req.Header.Set("Content-Type", "application/json")

	resp, err := c.client.Do(req)
	if err != nil {
		return fmt.Errorf("failed to execute request: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return fmt.Errorf("unexpected status code: %d", resp.StatusCode)
	}

	return nil
}

// TestWebhookWithResponse sends test data to a webhook and returns the full response
func (c *Client) TestWebhookWithResponse(ctx context.Context, webhookURL string, testData interface{}) (*http.Response, error) {
	data, err := json.Marshal(testData)
	if err != nil {
		return nil, fmt.Errorf("failed to marshal test data: %w", err)
	}

	req, err := http.NewRequestWithContext(ctx, "POST", webhookURL, bytes.NewBuffer(data))
	if err != nil {
		return nil, fmt.Errorf("failed to create request: %w", err)
	}

	req.Header.Set("Content-Type", "application/json")

	resp, err := c.client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("failed to execute request: %w", err)
	}

	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		defer resp.Body.Close()
		return nil, fmt.Errorf("unexpected status code: %d", resp.StatusCode)
	}

	return resp, nil
}

// ImportWebhookFromJSON imports a webhook from JSON
func (c *Client) ImportWebhookFromJSON(ctx context.Context, jsonData []byte) (*Webhook, error) {
	var webhook Webhook
	if err := json.Unmarshal(jsonData, &webhook); err != nil {
		return nil, fmt.Errorf("failed to parse JSON: %w", err)
	}

	// Clear ID and other server-assigned fields
	webhook.ID = ""
	webhook.CreatedAt = ""
	webhook.UpdatedAt = ""
	webhook.WebhookURL = ""

	return c.CreateWebhook(ctx, webhook)
}
