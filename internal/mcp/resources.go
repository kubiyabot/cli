package mcp

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"

	"github.com/kubiyabot/cli/internal/composer"
	"github.com/kubiyabot/cli/internal/kubiya"
	"github.com/mark3labs/mcp-go/mcp"
	"github.com/mark3labs/mcp-go/server"
)

// Resources manages MCP resources
type Resources struct {
	client         *kubiya.Client
	composerClient *composer.Client
}

// NewResources creates a new resources manager
func NewResources(client *kubiya.Client, composerClient *composer.Client) *Resources {
	return &Resources{
		client:         client,
		composerClient: composerClient,
	}
}

// Register adds all resources to the MCP server
func (r *Resources) Register(s *server.MCPServer) error {
	// Runners resource
	s.AddResource(mcp.NewResource("kubiya://runners", "Kubiya Runners",
		mcp.WithResourceDescription("List of available Kubiya runners"),
		mcp.WithMIMEType("application/json"),
	), r.runnersHandler)

	// Tool sources resource
	s.AddResource(mcp.NewResource("kubiya://sources", "Kubiya Sources",
		mcp.WithResourceDescription("List of available tool sources"),
		mcp.WithMIMEType("application/json"),
	), r.sourcesHandler)

	// Agents resource
	s.AddResource(mcp.NewResource("kubiya://agents", "Kubiya Agents",
		mcp.WithResourceDescription("List of available agents"),
		mcp.WithMIMEType("application/json"),
	), r.agentsHandler)

	// Integration templates resource
	s.AddResource(mcp.NewResource("kubiya://integrations", "Kubiya Integrations",
		mcp.WithResourceDescription("Available integration templates"),
		mcp.WithMIMEType("application/json"),
	), r.integrationsHandler)

	// Knowledge base resource
	s.AddResource(mcp.NewResource("kubiya://knowledge", "Kubiya Knowledge Base",
		mcp.WithResourceDescription("Knowledge base entries"),
		mcp.WithMIMEType("application/json"),
	), r.knowledgeHandler)

	// Python DSL examples resource
	s.AddResourceTemplate(mcp.NewResourceTemplate("composer://examples/{query}/{limit}", "Python DSL examples",
		mcp.WithTemplateDescription("Python DSL examples"),
		mcp.WithTemplateMIMEType("application/json"),
	), r.examplesHandler)

	return nil
}

func (r *Resources) runnersHandler(ctx context.Context, request mcp.ReadResourceRequest) ([]mcp.ResourceContents, error) {
	runners, err := r.client.ListRunners(ctx)
	if err != nil {
		return nil, fmt.Errorf("failed to list runners: %w", err)
	}

	data, err := json.MarshalIndent(runners, "", "  ")
	if err != nil {
		return nil, fmt.Errorf("failed to marshal runners: %w", err)
	}

	return []mcp.ResourceContents{
		mcp.TextResourceContents{
			URI:      request.Params.URI,
			MIMEType: "application/json",
			Text:     string(data),
		},
	}, nil
}

func (r *Resources) sourcesHandler(ctx context.Context, request mcp.ReadResourceRequest) ([]mcp.ResourceContents, error) {
	sources, err := r.client.ListSources(ctx)
	if err != nil {
		return nil, fmt.Errorf("failed to list sources: %w", err)
	}

	data, err := json.MarshalIndent(sources, "", "  ")
	if err != nil {
		return nil, fmt.Errorf("failed to marshal sources: %w", err)
	}

	return []mcp.ResourceContents{
		mcp.TextResourceContents{
			URI:      request.Params.URI,
			MIMEType: "application/json",
			Text:     string(data),
		},
	}, nil
}

func (r *Resources) agentsHandler(ctx context.Context, request mcp.ReadResourceRequest) ([]mcp.ResourceContents, error) {
	agents, err := r.client.ListAgents(ctx)
	if err != nil {
		return nil, fmt.Errorf("failed to list agents: %w", err)
	}

	data, err := json.MarshalIndent(agents, "", "  ")
	if err != nil {
		return nil, fmt.Errorf("failed to marshal agents: %w", err)
	}

	return []mcp.ResourceContents{
		mcp.TextResourceContents{
			URI:      request.Params.URI,
			MIMEType: "application/json",
			Text:     string(data),
		},
	}, nil
}

func (r *Resources) integrationsHandler(ctx context.Context, request mcp.ReadResourceRequest) ([]mcp.ResourceContents, error) {
	integrations, err := r.client.ListIntegrations(ctx)
	if err != nil {
		return nil, fmt.Errorf("failed to list integrations: %w", err)
	}

	data, err := json.MarshalIndent(integrations, "", "  ")
	if err != nil {
		return nil, fmt.Errorf("failed to marshal integrations: %w", err)
	}

	return []mcp.ResourceContents{
		mcp.TextResourceContents{
			URI:      request.Params.URI,
			MIMEType: "application/json",
			Text:     string(data),
		},
	}, nil
}

func (r *Resources) knowledgeHandler(ctx context.Context, request mcp.ReadResourceRequest) ([]mcp.ResourceContents, error) {
	knowledge, err := r.client.ListKnowledge(ctx, "", 100) // List all knowledge, limit 100
	if err != nil {
		return nil, fmt.Errorf("failed to list knowledge: %w", err)
	}

	data, err := json.MarshalIndent(knowledge, "", "  ")
	if err != nil {
		return nil, fmt.Errorf("failed to marshal knowledge: %w", err)
	}

	return []mcp.ResourceContents{
		mcp.TextResourceContents{
			URI:      request.Params.URI,
			MIMEType: "application/json",
			Text:     string(data),
		},
	}, nil
}

// examplesHandler handles requests for Python DSL examples from the vector database
func (r *Resources) examplesHandler(ctx context.Context, request mcp.ReadResourceRequest) ([]mcp.ResourceContents, error) {
	// For path-based resource templates, the MCP library might send arrays
	// Let's use a simpler approach: just get string representations without brackets

	query := ""
	if q := request.Params.Arguments["query"]; q != nil {
		str := fmt.Sprintf("%v", q)
		// If it looks like an array (starts with [ and ends with ]), extract the content
		if strings.HasPrefix(str, "[") && strings.HasSuffix(str, "]") {
			// Remove brackets and get first item if multiple (space-separated)
			content := strings.TrimPrefix(strings.TrimSuffix(str, "]"), "[")
			// If there are multiple space-separated items, take the first
			parts := strings.Fields(content)
			if len(parts) > 0 {
				query = parts[0]
			} else {
				query = content
			}
		} else {
			query = str
		}
	}

	limit := ""
	if l := request.Params.Arguments["limit"]; l != nil {
		str := fmt.Sprintf("%v", l)
		// If it looks like an array (starts with [ and ends with ]), extract the content
		if strings.HasPrefix(str, "[") && strings.HasSuffix(str, "]") {
			// Remove brackets and get first item if multiple (space-separated)
			content := strings.TrimPrefix(strings.TrimSuffix(str, "]"), "[")
			// If there are multiple space-separated items, take the first
			parts := strings.Fields(content)
			if len(parts) > 0 {
				limit = parts[0]
			} else {
				limit = content
			}
		} else {
			limit = str
		}
	}

	examples, err := r.composerClient.GetExamples(ctx, query, limit)
	if err != nil {
		return nil, fmt.Errorf("failed to get examples: %w", err)
	}

	data, err := json.MarshalIndent(examples, "", "  ")
	if err != nil {
		return nil, fmt.Errorf("failed to marshal knowledge: %w", err)
	}

	return []mcp.ResourceContents{
		mcp.TextResourceContents{
			URI:      request.Params.URI,
			MIMEType: "application/json",
			Text:     string(data),
		},
	}, nil
}
