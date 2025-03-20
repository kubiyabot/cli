package kubiya

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"time"
)

// AuditItem represents a single audit log entry
type AuditItem struct {
	Org              string                 `json:"org"`
	Email            string                 `json:"email"`
	Version          int                    `json:"version"`
	CategoryType     string                 `json:"category_type"`
	CategoryName     string                 `json:"category_name"`
	ResourceType     string                 `json:"resource_type"`
	ResourceText     string                 `json:"resource_text"`
	ActionType       string                 `json:"action_type"`
	ActionSuccessful bool                   `json:"action_successful"`
	Timestamp        string                 `json:"timestamp"`
	Extra            map[string]interface{} `json:"extra"`
	Scope            string                 `json:"scope"`
}

// AuditFilter represents the filter parameters for audit queries
type AuditFilter struct {
	Timestamp struct {
		GTE string `json:"gte,omitempty"`
		LTE string `json:"lte,omitempty"`
	} `json:"timestamp,omitempty"`
	CategoryType string `json:"category_type,omitempty"`
	CategoryName string `json:"category_name,omitempty"`
	ResourceType string `json:"resource_type,omitempty"`
	ActionType   string `json:"action_type,omitempty"`
	SessionID    string `json:"session_id,omitempty"` // Used to filter by session ID
}

// AuditSort represents the sorting parameters for audit queries
type AuditSort struct {
	Timestamp int `json:"timestamp,omitempty"` // -1 for descending, 1 for ascending
}

// AuditQuery represents the complete query parameters for audit requests
type AuditQuery struct {
	Filter   AuditFilter `json:"filter"`
	Page     int         `json:"page"`
	PageSize int         `json:"page_size"`
	Sort     AuditSort   `json:"sort"`
}

// AuditClient handles all audit-related API interactions
type AuditClient struct {
	client  *Client
	baseURL string
}

// NewAuditClient creates a new audit client
func NewAuditClient(client *Client) *AuditClient {
	return &AuditClient{
		client:  client,
		baseURL: client.baseURL,
	}
}

// ListAuditItems retrieves audit items based on the provided query parameters
func (ac *AuditClient) ListAuditItems(ctx context.Context, query AuditQuery) ([]AuditItem, error) {
	// Convert query to URL parameters
	params := url.Values{}

	// Add filter
	if filterJSON, err := json.Marshal(query.Filter); err == nil {
		params.Add("filter", string(filterJSON))
	}

	// Add sort
	if sortJSON, err := json.Marshal(query.Sort); err == nil {
		params.Add("sort", string(sortJSON))
	}

	// Add pagination
	params.Add("page", fmt.Sprintf("%d", query.Page))
	params.Add("page_size", fmt.Sprintf("%d", query.PageSize))

	// Construct URL
	auditURL := fmt.Sprintf("%s/auditing/items?%s", ac.baseURL, params.Encode())

	// Create request
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, auditURL, nil)
	if err != nil {
		return nil, fmt.Errorf("failed to create request: %w", err)
	}

	// Set headers
	req.Header.Set("Authorization", "UserKey "+ac.client.cfg.APIKey)
	req.Header.Set("Content-Type", "application/json")

	// Execute request
	resp, err := ac.client.client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("failed to execute request: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		return nil, fmt.Errorf("unexpected status code: %d, body: %s", resp.StatusCode, string(body))
	}

	var items []AuditItem
	if err := json.NewDecoder(resp.Body).Decode(&items); err != nil {
		return nil, fmt.Errorf("failed to decode response: %w", err)
	}

	return items, nil
}

// StreamAuditItems streams audit items in real-time based on the provided query parameters
func (ac *AuditClient) StreamAuditItems(ctx context.Context, query AuditQuery) (<-chan AuditItem, error) {
	items := make(chan AuditItem)

	go func() {
		defer close(items)

		// Convert query to URL parameters
		params := url.Values{}

		// Add filter
		if filterJSON, err := json.Marshal(query.Filter); err == nil {
			params.Add("filter", string(filterJSON))
		}

		// Add sort
		if sortJSON, err := json.Marshal(query.Sort); err == nil {
			params.Add("sort", string(sortJSON))
		}

		// Add pagination
		params.Add("page", fmt.Sprintf("%d", query.Page))
		params.Add("page_size", fmt.Sprintf("%d", query.PageSize))

		// Construct URL
		auditURL := fmt.Sprintf("%s/auditing/items/stream?%s", ac.baseURL, params.Encode())

		// Create request
		req, err := http.NewRequestWithContext(ctx, http.MethodGet, auditURL, nil)
		if err != nil {
			return
		}

		// Set headers for SSE
		req.Header.Set("Authorization", "UserKey "+ac.client.cfg.APIKey)
		req.Header.Set("Accept", "text/event-stream")
		req.Header.Set("Cache-Control", "no-cache")
		req.Header.Set("Connection", "keep-alive")

		// Create client with no timeout for streaming
		client := &http.Client{Timeout: 0}

		// Execute request
		resp, err := client.Do(req)
		if err != nil {
			return
		}
		defer resp.Body.Close()

		if resp.StatusCode != http.StatusOK {
			return
		}

		// Read SSE messages
		decoder := json.NewDecoder(resp.Body)
		for {
			select {
			case <-ctx.Done():
				return
			default:
				var item AuditItem
				if err := decoder.Decode(&item); err != nil {
					if err != io.EOF {
						// Log error but continue processing
						continue
					}
					return
				}
				items <- item
			}
		}
	}()

	return items, nil
}

// GetAuditItemsByTimeRange retrieves audit items within a specific time range
func (ac *AuditClient) GetAuditItemsByTimeRange(ctx context.Context, startTime, endTime time.Time, page, pageSize int) ([]AuditItem, error) {
	query := AuditQuery{
		Filter: AuditFilter{
			Timestamp: struct {
				GTE string `json:"gte,omitempty"`
				LTE string `json:"lte,omitempty"`
			}{
				GTE: startTime.Format(time.RFC3339),
				LTE: endTime.Format(time.RFC3339),
			},
		},
		Page:     page,
		PageSize: pageSize,
		Sort: AuditSort{
			Timestamp: -1, // Sort by timestamp descending
		},
	}

	return ac.ListAuditItems(ctx, query)
}

// GetAuditItemsByCategory retrieves audit items for a specific category
func (ac *AuditClient) GetAuditItemsByCategory(ctx context.Context, categoryType, categoryName string, page, pageSize int) ([]AuditItem, error) {
	query := AuditQuery{
		Filter: AuditFilter{
			CategoryType: categoryType,
			CategoryName: categoryName,
		},
		Page:     page,
		PageSize: pageSize,
		Sort: AuditSort{
			Timestamp: -1, // Sort by timestamp descending
		},
	}

	return ac.ListAuditItems(ctx, query)
}

// GetAuditItemsByResourceType retrieves audit items for a specific resource type
func (ac *AuditClient) GetAuditItemsByResourceType(ctx context.Context, resourceType string, page, pageSize int) ([]AuditItem, error) {
	query := AuditQuery{
		Filter: AuditFilter{
			ResourceType: resourceType,
		},
		Page:     page,
		PageSize: pageSize,
		Sort: AuditSort{
			Timestamp: -1, // Sort by timestamp descending
		},
	}

	return ac.ListAuditItems(ctx, query)
}
