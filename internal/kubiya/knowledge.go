package kubiya

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"net/http"
)

// Knowledge-related client methods
func (c *Client) ListKnowledge(ctx context.Context, query string, limit int) ([]Knowledge, error) {
	url := fmt.Sprintf("%s/knowledge", c.cfg.BaseURL)
	
	// Add query parameters
	if query != "" || limit > 0 {
		params := make([]string, 0)
		if query != "" {
			params = append(params, fmt.Sprintf("query=%s", query))
		}
		if limit > 0 {
			params = append(params, fmt.Sprintf("limit=%d", limit))
		}
		if len(params) > 0 {
			url += "?" + fmt.Sprintf("%s", params[0])
			for i := 1; i < len(params); i++ {
				url += "&" + params[i]
			}
		}
	}
	
	req, err := http.NewRequestWithContext(ctx, "GET", url, nil)
	if err != nil {
		return nil, err
	}

	req.Header.Set("Authorization", "UserKey "+c.cfg.APIKey)

	resp, err := c.client.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("unexpected status code: %d", resp.StatusCode)
	}

	var items []Knowledge
	if err := json.NewDecoder(resp.Body).Decode(&items); err != nil {
		return nil, err
	}

	return items, nil
}

func (c *Client) SearchKnowledge(ctx context.Context, query string, limit int) ([]Knowledge, error) {
	url := fmt.Sprintf("%s/knowledge/search?query=%s", c.cfg.BaseURL, query)
	if limit > 0 {
		url += fmt.Sprintf("&limit=%d", limit)
	}
	
	req, err := http.NewRequestWithContext(ctx, "GET", url, nil)
	if err != nil {
		return nil, err
	}

	req.Header.Set("Authorization", "UserKey "+c.cfg.APIKey)

	resp, err := c.client.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("unexpected status code: %d", resp.StatusCode)
	}

	var items []Knowledge
	if err := json.NewDecoder(resp.Body).Decode(&items); err != nil {
		return nil, err
	}

	return items, nil
}

func (c *Client) GetKnowledge(ctx context.Context, uuid string) (*Knowledge, error) {
	req, err := http.NewRequestWithContext(ctx, "GET",
		fmt.Sprintf("%s/knowledge/%s", c.cfg.BaseURL, uuid), nil)
	if err != nil {
		return nil, err
	}

	req.Header.Set("Authorization", "UserKey "+c.cfg.APIKey)

	resp, err := c.client.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("unexpected status code: %d", resp.StatusCode)
	}

	var item Knowledge
	if err := json.NewDecoder(resp.Body).Decode(&item); err != nil {
		return nil, err
	}

	return &item, nil
}

func (c *Client) CreateKnowledge(ctx context.Context, item Knowledge) (*Knowledge, error) {
	data, err := json.Marshal(item)
	if err != nil {
		return nil, err
	}

	req, err := http.NewRequestWithContext(ctx, "POST",
		fmt.Sprintf("%s/knowledge", c.cfg.BaseURL), bytes.NewBuffer(data))
	if err != nil {
		return nil, err
	}

	req.Header.Set("Authorization", "UserKey "+c.cfg.APIKey)
	req.Header.Set("Content-Type", "application/json")

	resp, err := c.client.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("unexpected status code: %d", resp.StatusCode)
	}

	var created Knowledge
	if err := json.NewDecoder(resp.Body).Decode(&created); err != nil {
		return nil, err
	}

	return &created, nil
}

func (c *Client) UpdateKnowledge(ctx context.Context, uuid string, item Knowledge) (*Knowledge, error) {
	data, err := json.Marshal(item)
	if err != nil {
		return nil, err
	}

	req, err := http.NewRequestWithContext(ctx, "PUT",
		fmt.Sprintf("%s/knowledge/%s", c.cfg.BaseURL, uuid), bytes.NewBuffer(data))
	if err != nil {
		return nil, err
	}

	req.Header.Set("Authorization", "UserKey "+c.cfg.APIKey)
	req.Header.Set("Content-Type", "application/json")

	resp, err := c.client.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("unexpected status code: %d", resp.StatusCode)
	}

	var updated Knowledge
	if err := json.NewDecoder(resp.Body).Decode(&updated); err != nil {
		return nil, err
	}

	return &updated, nil
}

func (c *Client) DeleteKnowledge(ctx context.Context, uuid string) error {
	req, err := http.NewRequestWithContext(ctx, "DELETE",
		fmt.Sprintf("%s/knowledge/%s", c.cfg.BaseURL, uuid), nil)
	if err != nil {
		return err
	}

	req.Header.Set("Authorization", "UserKey "+c.cfg.APIKey)

	resp, err := c.client.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("unexpected status code: %d", resp.StatusCode)
	}

	return nil
}
