package composer

import (
	"context"
	"encoding/json"
)

// GetExamples retrieves examples from the knowledge base
func (c *Client) GetExamples(ctx context.Context, query string, limit string) (any, error) {
	pathWithParams, err := c.httpClient.BuildPathWithParams("/knowledge/search", map[string]string{
		"query": query,
		"limit": limit,
	})
	if err != nil {
		return nil, err
	}

	resp, err := c.httpClient.GET(ctx, pathWithParams)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	var examples any
	if err = json.NewDecoder(resp.Body).Decode(&examples); err != nil {
		return nil, err
	}
	return examples, nil
}
