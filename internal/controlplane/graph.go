package controlplane

import (
	"fmt"
)

// GraphNode represents a node in the context graph
type GraphNode struct {
	ID         string                 `json:"id"`
	Labels     []string               `json:"labels"`
	Properties map[string]interface{} `json:"properties"`
}

// GraphRelationship represents a relationship in the context graph
type GraphRelationship struct {
	ID         string                 `json:"id"`
	Type       string                 `json:"type"`
	StartNode  string                 `json:"start_node"`
	EndNode    string                 `json:"end_node"`
	Properties map[string]interface{} `json:"properties"`
}

// GraphStats represents statistics about the context graph
type GraphStats struct {
	TotalNodes         int      `json:"total_nodes"`
	TotalRelationships int      `json:"total_relationships"`
	Labels             []string `json:"labels"`
	RelationshipTypes  []string `json:"relationship_types"`
}

// NodeSearchRequest represents a request to search for nodes
type NodeSearchRequest struct {
	Label         string      `json:"label,omitempty"`
	PropertyName  string      `json:"property_name,omitempty"`
	PropertyValue interface{} `json:"property_value,omitempty"`
}

// TextSearchRequest represents a request to search by text
type TextSearchRequest struct {
	PropertyName string `json:"property_name"`
	SearchText   string `json:"search_text"`
	Label        string `json:"label,omitempty"`
}

// SubgraphRequest represents a request to get a subgraph
type SubgraphRequest struct {
	NodeID            string   `json:"node_id"`
	Depth             int      `json:"depth"`
	RelationshipTypes []string `json:"relationship_types,omitempty"`
}

// SubgraphResponse represents a subgraph
type SubgraphResponse struct {
	Nodes         []GraphNode         `json:"nodes"`
	Relationships []GraphRelationship `json:"relationships"`
}

// Integration represents an integration in the graph
type Integration struct {
	Name        string `json:"name"`
	Type        string `json:"type"`
	Description string `json:"description,omitempty"`
}

// CustomQueryRequest represents a custom Cypher query
type CustomQueryRequest struct {
	Query string `json:"query"`
}

// GraphHealth checks the health of the context graph API
func (c *Client) GraphHealth() error {
	var result map[string]interface{}
	return c.get("/api/v1/context-graph/health", &result)
}

// GetGraphStats retrieves statistics about the context graph
func (c *Client) GetGraphStats(integration string) (*GraphStats, error) {
	path := "/api/v1/context-graph/api/v1/graph/stats"
	if integration != "" {
		path += fmt.Sprintf("?integration=%s", integration)
	}

	var stats GraphStats
	err := c.get(path, &stats)
	return &stats, err
}

// ListNodes retrieves all nodes in the graph
func (c *Client) ListNodes(integration string, skip, limit int) ([]GraphNode, error) {
	path := fmt.Sprintf("/api/v1/context-graph/api/v1/graph/nodes?skip=%d&limit=%d", skip, limit)
	if integration != "" {
		path += fmt.Sprintf("&integration=%s", integration)
	}

	var nodes []GraphNode
	err := c.get(path, &nodes)
	return nodes, err
}

// SearchNodes searches for nodes in the graph
func (c *Client) SearchNodes(req *NodeSearchRequest, integration string, skip, limit int) ([]GraphNode, error) {
	path := fmt.Sprintf("/api/v1/context-graph/api/v1/graph/nodes/search?skip=%d&limit=%d", skip, limit)
	if integration != "" {
		path += fmt.Sprintf("&integration=%s", integration)
	}

	var nodes []GraphNode
	err := c.post(path, req, &nodes)
	return nodes, err
}

// GetNode retrieves a specific node by ID
func (c *Client) GetNode(nodeID string, integration string) (*GraphNode, error) {
	path := fmt.Sprintf("/api/v1/context-graph/api/v1/graph/nodes/%s", nodeID)
	if integration != "" {
		path += fmt.Sprintf("?integration=%s", integration)
	}

	var node GraphNode
	err := c.get(path, &node)
	return &node, err
}

// GetNodeRelationships retrieves relationships for a node
func (c *Client) GetNodeRelationships(nodeID string, direction, relType, integration string, skip, limit int) ([]GraphRelationship, error) {
	path := fmt.Sprintf("/api/v1/context-graph/api/v1/graph/nodes/%s/relationships?direction=%s&skip=%d&limit=%d",
		nodeID, direction, skip, limit)

	if relType != "" {
		path += fmt.Sprintf("&relationship_type=%s", relType)
	}
	if integration != "" {
		path += fmt.Sprintf("&integration=%s", integration)
	}

	var relationships []GraphRelationship
	err := c.get(path, &relationships)
	return relationships, err
}

// SearchNodesByText searches for nodes using text search
func (c *Client) SearchNodesByText(req *TextSearchRequest, integration string, skip, limit int) ([]GraphNode, error) {
	path := fmt.Sprintf("/api/v1/context-graph/api/v1/graph/nodes/search/text?skip=%d&limit=%d", skip, limit)
	if integration != "" {
		path += fmt.Sprintf("&integration=%s", integration)
	}

	var nodes []GraphNode
	err := c.post(path, req, &nodes)
	return nodes, err
}

// GetSubgraph retrieves a subgraph starting from a node
func (c *Client) GetSubgraph(req *SubgraphRequest, integration string) (*SubgraphResponse, error) {
	path := "/api/v1/context-graph/api/v1/graph/subgraph"
	if integration != "" {
		path += fmt.Sprintf("?integration=%s", integration)
	}

	var subgraph SubgraphResponse
	err := c.post(path, req, &subgraph)
	return &subgraph, err
}

// ListLabels retrieves all node labels in the graph
func (c *Client) ListLabels(integration string, skip, limit int) ([]string, error) {
	path := fmt.Sprintf("/api/v1/context-graph/api/v1/graph/labels?skip=%d&limit=%d", skip, limit)
	if integration != "" {
		path += fmt.Sprintf("&integration=%s", integration)
	}

	var response struct {
		Labels []string `json:"labels"`
		Count  int      `json:"count"`
	}
	err := c.get(path, &response)
	return response.Labels, err
}

// ListRelationshipTypes retrieves all relationship types in the graph
func (c *Client) ListRelationshipTypes(integration string, skip, limit int) ([]string, error) {
	path := fmt.Sprintf("/api/v1/context-graph/api/v1/graph/relationship-types?skip=%d&limit=%d", skip, limit)
	if integration != "" {
		path += fmt.Sprintf("&integration=%s", integration)
	}

	var response struct {
		RelationshipTypes []string `json:"relationship_types"`
		Count             int      `json:"count"`
	}
	err := c.get(path, &response)
	return response.RelationshipTypes, err
}

// ListIntegrations retrieves all available integrations
func (c *Client) ListIntegrations(skip, limit int) ([]Integration, error) {
	path := fmt.Sprintf("/api/v1/context-graph/api/v1/graph/integrations?skip=%d&limit=%d", skip, limit)

	var response struct {
		Integrations []string `json:"integrations"`
		Count        int      `json:"count"`
	}
	err := c.get(path, &response)
	if err != nil {
		return nil, err
	}

	// Convert string names to Integration objects
	integrations := make([]Integration, len(response.Integrations))
	for i, name := range response.Integrations {
		integrations[i] = Integration{
			Name: name,
			Type: name, // Type is same as name for now
		}
	}
	return integrations, nil
}

// ExecuteCustomQuery executes a custom Cypher query (read-only)
func (c *Client) ExecuteCustomQuery(req *CustomQueryRequest) ([]map[string]interface{}, error) {
	path := "/api/v1/context-graph/api/v1/graph/query"

	var results []map[string]interface{}
	err := c.post(path, req, &results)
	return results, err
}
