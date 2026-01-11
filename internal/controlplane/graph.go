package controlplane

import (
	"bufio"
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"
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

	var response struct {
		Nodes []GraphNode `json:"nodes"`
		Count int         `json:"count"`
	}
	err := c.get(path, &response)
	return response.Nodes, err
}

// SearchNodes searches for nodes in the graph
func (c *Client) SearchNodes(req *NodeSearchRequest, integration string, skip, limit int) ([]GraphNode, error) {
	path := fmt.Sprintf("/api/v1/context-graph/api/v1/graph/nodes/search?skip=%d&limit=%d", skip, limit)
	if integration != "" {
		path += fmt.Sprintf("&integration=%s", integration)
	}

	var response struct {
		Nodes []GraphNode `json:"nodes"`
		Count int         `json:"count"`
	}
	err := c.post(path, req, &response)
	return response.Nodes, err
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

	var response struct {
		Nodes []GraphNode `json:"nodes"`
		Count int         `json:"count"`
	}
	err := c.post(path, req, &response)
	return response.Nodes, err
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

// CustomQueryResponse represents the response from a custom Cypher query
type CustomQueryResponse struct {
	Results []map[string]interface{} `json:"results"`
	Count   int                      `json:"count"`
}

// ExecuteCustomQuery executes a custom Cypher query (read-only)
func (c *Client) ExecuteCustomQuery(req *CustomQueryRequest) ([]map[string]interface{}, error) {
	// Get context graph API base URL
	config, err := c.GetClientConfig()
	if err != nil {
		return nil, fmt.Errorf("failed to get client config: %w", err)
	}

	path := config.ContextGraphAPIBase + "/api/v1/graph/query"

	var response CustomQueryResponse
	err = c.post(path, req, &response)
	if err != nil {
		return nil, err
	}
	return response.Results, nil
}

// ============================================================================
// Intelligent Search Types and Methods
// ============================================================================

// IntelligentSearchRequest represents a request for AI-powered graph search
type IntelligentSearchRequest struct {
	Keywords              string   `json:"keywords"`
	MaxTurns              int      `json:"max_turns,omitempty"`
	SystemPrompt          string   `json:"system_prompt,omitempty"`
	AdditionalContext     string   `json:"additional_context,omitempty"`
	Model                 string   `json:"model,omitempty"`
	Temperature           float64  `json:"temperature,omitempty"`
	Integration           string   `json:"integration,omitempty"`
	LabelFilter           string   `json:"label_filter,omitempty"`
	EnableSemanticSearch  bool     `json:"enable_semantic_search,omitempty"`
	EnableCypherQueries   bool     `json:"enable_cypher_queries,omitempty"`
	Strategy              string   `json:"strategy,omitempty"`
	SessionID             string   `json:"session_id,omitempty"`
}

// ToolCall represents a tool invocation during search
type ToolCall struct {
	ToolName      string                 `json:"tool_name"`
	Arguments     map[string]interface{} `json:"arguments"`
	ResultSummary string                 `json:"result_summary,omitempty"`
}

// IntelligentSearchResponse represents the response from intelligent search
type IntelligentSearchResponse struct {
	Answer        string                `json:"answer"`
	Nodes         []GraphNode           `json:"nodes"`
	Relationships []GraphRelationship   `json:"relationships"`
	Subgraphs     []SubgraphResponse    `json:"subgraphs"`
	ToolCalls     []ToolCall            `json:"tool_calls"`
	TurnsUsed     int                   `json:"turns_used"`
	Confidence    string                `json:"confidence"`
	Suggestions   []string              `json:"suggestions"`
	SessionID     string                `json:"session_id,omitempty"`
}

// SessionInfo represents information about a search session
type SessionInfo struct {
	SessionID            string `json:"session_id"`
	Organization         string `json:"organization"`
	UserID               string `json:"user_id"`
	CreatedAt            string `json:"created_at"`
	LastAccessed         string `json:"last_accessed"`
	ExpiresAt            string `json:"expires_at"`
	ConversationLength   int    `json:"conversation_length"`
}

// SearchHealthResponse represents health status of intelligent search
type SearchHealthResponse struct {
	Status        string                       `json:"status"`
	Strategies    map[string]StrategyStatus    `json:"strategies"`
	Configuration SearchConfiguration          `json:"configuration"`
	Message       string                       `json:"message"`
}

// StrategyStatus represents availability of a search strategy
type StrategyStatus struct {
	Available bool `json:"available"`
	Default   bool `json:"default"`
}

// SearchConfiguration represents search service configuration
type SearchConfiguration struct {
	DefaultStrategy      string `json:"default_strategy"`
	RecommendedStrategy  string `json:"recommended_strategy"`
	LiteLLMConfigured    bool   `json:"litellm_configured"`
	SessionTTLMinutes    int    `json:"session_ttl_minutes"`
}

// StreamEvent represents a streaming event from intelligent search
type StreamEvent struct {
	Event string                 `json:"event"`
	Data  map[string]interface{} `json:"data"`
}

// IntelligentSearch performs AI-powered graph search
func (c *Client) IntelligentSearch(req *IntelligentSearchRequest) (*IntelligentSearchResponse, error) {
	path := "/api/v1/context-graph/intelligent-search"

	var result IntelligentSearchResponse
	err := c.post(path, req, &result)
	return &result, err
}

// GetSearchHealth checks the health of the intelligent search service
func (c *Client) GetSearchHealth() (*SearchHealthResponse, error) {
	path := "/api/v1/context-graph/intelligent-search/health"

	var health SearchHealthResponse
	err := c.get(path, &health)
	return &health, err
}

// IntelligentSearchStream performs streaming AI-powered graph search with SSE
// Returns a channel that emits StreamEvent objects
func (c *Client) IntelligentSearchStream(req *IntelligentSearchRequest) (<-chan StreamEvent, <-chan error, error) {
	path := "/api/v1/context-graph/intelligent-search/stream"

	// Create request
	body, err := json.Marshal(req)
	if err != nil {
		return nil, nil, fmt.Errorf("failed to marshal request: %w", err)
	}

	// Create HTTP request
	url := c.BaseURL + path
	httpReq, err := http.NewRequest(http.MethodPost, url, bytes.NewReader(body))
	if err != nil {
		return nil, nil, fmt.Errorf("failed to create request: %w", err)
	}

	// Set headers
	httpReq.Header.Set("Content-Type", "application/json")
	httpReq.Header.Set("Accept", "text/event-stream")
	httpReq.Header.Set("Authorization", fmt.Sprintf("Bearer %s", c.APIKey))
	httpReq.Header.Set("Cache-Control", "no-cache")
	httpReq.Header.Set("Connection", "keep-alive")

	// Make request
	resp, err := c.HTTPClient.Do(httpReq)
	if err != nil {
		return nil, nil, fmt.Errorf("failed to make request: %w", err)
	}

	if resp.StatusCode != http.StatusOK {
		resp.Body.Close()
		return nil, nil, fmt.Errorf("request failed with status: %d", resp.StatusCode)
	}

	// Create channels
	eventChan := make(chan StreamEvent, 10)
	errChan := make(chan error, 1)

	// Start goroutine to read SSE stream
	go func() {
		defer resp.Body.Close()
		defer close(eventChan)
		defer close(errChan)

		reader := bufio.NewReader(resp.Body)
		var currentEvent string

		for {
			line, err := reader.ReadString('\n')
			if err != nil {
				if err != io.EOF {
					errChan <- fmt.Errorf("error reading stream: %w", err)
				}
				return
			}

			line = strings.TrimSpace(line)

			// Skip empty lines
			if line == "" {
				continue
			}

			// Parse SSE format
			if strings.HasPrefix(line, "event: ") {
				currentEvent = strings.TrimPrefix(line, "event: ")
			} else if strings.HasPrefix(line, "data: ") {
				dataStr := strings.TrimPrefix(line, "data: ")

				// Parse JSON data
				var data map[string]interface{}
				if err := json.Unmarshal([]byte(dataStr), &data); err != nil {
					errChan <- fmt.Errorf("error parsing event data: %w", err)
					continue
				}

				// Send event
				eventChan <- StreamEvent{
					Event: currentEvent,
					Data:  data,
				}

				// Reset event type
				currentEvent = ""
			}
		}
	}()

	return eventChan, errChan, nil
}
