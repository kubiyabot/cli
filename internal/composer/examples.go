package composer

import (
	"context"
	"encoding/json"
)

// GetExamples retrieves examples from the knowledge base
func (c *Client) GetExamples(ctx context.Context, query string, limit string) (interface{}, error) {
	// Build path with query parameters
	params := map[string]string{
		"query": query,
		"limit": limit,
	}
	// Build the path with params
	pathWithParams, err := c.httpClient.BuildPathWithParams("/knowledge/search", params)
	if err != nil {
		return nil, err
	}

	// Pass the path to GET method
	resp, err := c.httpClient.GET(ctx, pathWithParams)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	// Decode the response
	var examples interface{}
	if err = json.NewDecoder(resp.Body).Decode(&examples); err != nil {
		return nil, err
	}

	return examples, nil
}
