package mcp

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"
	"time"

	extism "github.com/extism/go-sdk"
	"github.com/mark3labs/mcp-go/mcp"
)

// addExecuteTool adds the core tool execution capability
func (s *Server) addExecuteTool() error {

	s.server.AddTool(mcp.NewTool("workflow_dsl_wasm",
		mcp.WithDescription("Execute a Kubiya workflow dsl with wasm"),
		mcp.WithString("content", mcp.Required(), mcp.Description("Content of workflow dsl script"))),
		s.workflowDslWasmHandler)

	s.server.AddTool(mcp.NewTool("execute_tool",
		mcp.WithDescription("Execute a Kubiya tool with live streaming output"),
		mcp.WithString("tool_name", mcp.Required(), mcp.Description("Name of the tool to execute")),
		mcp.WithString("runner", mcp.Description("Runner to use for execution (optional, will use default)")),
		mcp.WithObject("args", mcp.Description("Arguments to pass to the tool")),
		mcp.WithString("integration_template", mcp.Description("Integration template to apply (optional)")),
	), s.executeToolHandler)

	// Add create_on_demand_tool capability
	s.server.AddTool(mcp.NewTool("create_on_demand_tool",
		mcp.WithDescription("Create and execute a tool on-demand using tool definition schema"),
		mcp.WithObject("tool_def", mcp.Required(), mcp.Description("Tool definition object with schema: {name, description, type, image?, content?, args?, env?, with_files?}")),
		mcp.WithString("runner", mcp.Description("Runner to use for execution (optional, will use default)")),
		mcp.WithObject("args", mcp.Description("Arguments to pass to the tool")),
		mcp.WithString("integration_template", mcp.Description("Integration template to apply (optional)")),
	), s.createOnDemandToolHandler)

	// Add workflow execution capability
	s.server.AddTool(mcp.NewTool("execute_workflow",
		mcp.WithDescription("Execute a workflow with live streaming output and step-by-step progress. Supports secrets and environment variables."),
		mcp.WithObject("workflow_def", mcp.Required(), mcp.Description("Workflow definition object with schema: {name, description, steps[], type, runner?, params?, env?}")),
		mcp.WithString("runner", mcp.Description("Runner to use for execution (optional, will use default)")),
		mcp.WithObject("params", mcp.Description("Parameters to pass to the workflow")),
		mcp.WithObject("secrets", mcp.Description("Secrets to pass to the workflow execution (passed in request body, not workflow schema)")),
		mcp.WithObject("env", mcp.Description("Environment variables to add to the workflow (merged with workflow.env section)")),
	), s.executeWorkflowHandler)

	// Add whitelisted tool execution capability
	s.server.AddTool(mcp.NewTool("execute_whitelisted_tool",
		mcp.WithDescription("Execute a whitelisted tool from the MCP server configuration with live streaming"),
		mcp.WithString("tool_name", mcp.Required(), mcp.Description("Name of the whitelisted tool to execute")),
		mcp.WithString("runner", mcp.Description("Runner to use for execution (optional, will use default)")),
		mcp.WithObject("args", mcp.Description("Arguments to pass to the tool")),
		mcp.WithString("integration_template", mcp.Description("Integration template to apply (optional)")),
	), s.executeWhitelistedToolHandler)

	return nil
}

// addCorePlatformTools adds essential platform tools that are always available
func (s *Server) addCorePlatformTools() error {
	// Core read-only tools that should always be available
	coreTools := []struct {
		name        string
		description string
		handler     func(context.Context, mcp.CallToolRequest) (*mcp.CallToolResult, error)
		params      []func(string, ...mcp.PropertyOption) mcp.ToolOption
	}{
		{
			name:        "list_runners",
			description: "List all available runners",
			handler:     s.listRunnersHandler,
		},
		{
			name:        "list_agents",
			description: "List all available agents",
			handler:     s.listAgentsHandler,
		},
		{
			name:        "list_integrations",
			description: "List available integrations",
			handler:     s.listIntegrationsHandler,
		},
		{
			name:        "list_sources",
			description: "List all tool sources",
			handler:     s.listSourcesHandler,
		},
		{
			name:        "list_secrets",
			description: "List all available secrets (always available for workflow execution)",
			handler:     s.listSecretsHandler,
		},
		{
			name:        "check_runner_health",
			description: "Check health status of a specific runner or all runners",
			handler:     s.checkRunnerHealthHandler,
			params: []func(string, ...mcp.PropertyOption) mcp.ToolOption{
				func(name string, opts ...mcp.PropertyOption) mcp.ToolOption {
					return mcp.WithString("runner_name", append(opts, mcp.Description("Name of runner to check (optional, checks all if not provided"))...)
				},
			},
		},
		{
			name:        "find_available_runner",
			description: "Find the best available runner for execution based on health and load",
			handler:     s.findAvailableRunnerHandler,
			params: []func(string, ...mcp.PropertyOption) mcp.ToolOption{
				func(name string, opts ...mcp.PropertyOption) mcp.ToolOption {
					return mcp.WithString("runner_type", append(opts, mcp.Description("Type of runner needed (optional)"))...)
				},
			},
		},
	}

	for _, tool := range coreTools {
		opts := []mcp.ToolOption{mcp.WithDescription(tool.description)}
		if tool.params != nil {
			for _, param := range tool.params {
				opts = append(opts, param(""))
			}
		}
		s.server.AddTool(mcp.NewTool(tool.name, opts...), tool.handler)
	}

	return nil
}

// addAdvancedPlatformTools adds platform management tools that require --allow-platform-apis
func (s *Server) addAdvancedPlatformTools() error {
	advancedTools := []struct {
		name        string
		description string
		handler     func(context.Context, mcp.CallToolRequest) (*mcp.CallToolResult, error)
		params      []func(string, ...mcp.PropertyOption) mcp.ToolOption
	}{
		{
			name:        "create_runner",
			description: "Create a new Kubiya runner",
			handler:     s.createRunnerHandler,
			params: []func(string, ...mcp.PropertyOption) mcp.ToolOption{
				func(name string, opts ...mcp.PropertyOption) mcp.ToolOption {
					return mcp.WithString("name", append(opts, mcp.Required(), mcp.Description("Name of the runner"))...)
				},
				func(name string, opts ...mcp.PropertyOption) mcp.ToolOption {
					return mcp.WithString("description", append(opts, mcp.Description("Description of the runner"))...)
				},
			},
		},
		{
			name:        "delete_runner",
			description: "Delete a Kubiya runner",
			handler:     s.deleteRunnerHandler,
			params: []func(string, ...mcp.PropertyOption) mcp.ToolOption{
				func(name string, opts ...mcp.PropertyOption) mcp.ToolOption {
					return mcp.WithString("name", append(opts, mcp.Required(), mcp.Description("Name of the runner to delete"))...)
				},
			},
		},
		{
			name:        "chat_with_agent",
			description: "Chat with a specific agent",
			handler:     s.chatWithAgentHandler,
			params: []func(string, ...mcp.PropertyOption) mcp.ToolOption{
				func(name string, opts ...mcp.PropertyOption) mcp.ToolOption {
					return mcp.WithString("agent_name", append(opts, mcp.Required(), mcp.Description("Name of the agent to chat with"))...)
				},
				func(name string, opts ...mcp.PropertyOption) mcp.ToolOption {
					return mcp.WithString("message", append(opts, mcp.Required(), mcp.Description("Message to send to the agent"))...)
				},
			},
		},
		{
			name:        "create_integration",
			description: "Create a new integration",
			handler:     s.createIntegrationHandler,
			params: []func(string, ...mcp.PropertyOption) mcp.ToolOption{
				func(name string, opts ...mcp.PropertyOption) mcp.ToolOption {
					return mcp.WithString("type", append(opts, mcp.Required(), mcp.Description("Integration type"))...)
				},
				func(name string, opts ...mcp.PropertyOption) mcp.ToolOption {
					return mcp.WithString("name", append(opts, mcp.Required(), mcp.Description("Integration name"))...)
				},
				func(name string, opts ...mcp.PropertyOption) mcp.ToolOption {
					return mcp.WithObject("config", append(opts, mcp.Description("Integration configuration"))...)
				},
			},
		},
		{
			name:        "create_source",
			description: "Create a new tool source",
			handler:     s.createSourceHandler,
			params: []func(string, ...mcp.PropertyOption) mcp.ToolOption{
				func(name string, opts ...mcp.PropertyOption) mcp.ToolOption {
					return mcp.WithString("name", append(opts, mcp.Required(), mcp.Description("Source name"))...)
				},
				func(name string, opts ...mcp.PropertyOption) mcp.ToolOption {
					return mcp.WithString("url", append(opts, mcp.Required(), mcp.Description("Source URL"))...)
				},
				func(name string, opts ...mcp.PropertyOption) mcp.ToolOption {
					return mcp.WithString("description", append(opts, mcp.Description("Source description"))...)
				},
			},
		},
		{
			name:        "execute_tool_from_source",
			description: "Execute a tool from a specific source (by UUID or URL)",
			handler:     s.executeToolFromSourceHandler,
			params: []func(string, ...mcp.PropertyOption) mcp.ToolOption{
				func(name string, opts ...mcp.PropertyOption) mcp.ToolOption {
					return mcp.WithString("source", append(opts, mcp.Required(), mcp.Description("Source UUID or URL"))...)
				},
				func(name string, opts ...mcp.PropertyOption) mcp.ToolOption {
					return mcp.WithString("tool_name", append(opts, mcp.Required(), mcp.Description("Name of the tool to execute"))...)
				},
				func(name string, opts ...mcp.PropertyOption) mcp.ToolOption {
					return mcp.WithString("runner", append(opts, mcp.Description("Runner to use for execution"))...)
				},
				func(name string, opts ...mcp.PropertyOption) mcp.ToolOption {
					return mcp.WithObject("args", append(opts, mcp.Description("Arguments to pass to the tool"))...)
				},
			},
		},
		{
			name:        "discover_source",
			description: "Discover tools in a source URL without creating it permanently",
			handler:     s.discoverSourceHandler,
			params: []func(string, ...mcp.PropertyOption) mcp.ToolOption{
				func(name string, opts ...mcp.PropertyOption) mcp.ToolOption {
					return mcp.WithString("url", append(opts, mcp.Required(), mcp.Description("Source URL to discover"))...)
				},
				func(name string, opts ...mcp.PropertyOption) mcp.ToolOption {
					return mcp.WithString("runner", append(opts, mcp.Description("Runner to use for discovery"))...)
				},
				func(name string, opts ...mcp.PropertyOption) mcp.ToolOption {
					return mcp.WithObject("config", append(opts, mcp.Description("Dynamic configuration for source"))...)
				},
			},
		},
	}

	for _, tool := range advancedTools {
		opts := []mcp.ToolOption{mcp.WithDescription(tool.description)}
		if tool.params != nil {
			for _, param := range tool.params {
				opts = append(opts, param(""))
			}
		}
		s.server.AddTool(mcp.NewTool(tool.name, opts...), tool.handler)
	}

	return nil
}

// addKnowledgeBaseTools adds knowledge base management tools
func (s *Server) addKnowledgeBaseTools() error {
	// List knowledge base entries
	s.server.AddTool(mcp.NewTool("list_kb",
		mcp.WithDescription("List all knowledge base entries"),
		mcp.WithString("query", mcp.Description("Optional search query to filter entries")),
		mcp.WithNumber("limit", mcp.Description("Maximum number of entries to return (default: 50)")),
	), s.listKnowledgeHandler)

	// Search knowledge base
	s.server.AddTool(mcp.NewTool("search_kb",
		mcp.WithDescription("Search knowledge base entries"),
		mcp.WithString("query", mcp.Required(), mcp.Description("Search query")),
		mcp.WithNumber("limit", mcp.Description("Maximum number of results to return (default: 10)")),
	), s.searchKnowledgeHandler)

	// Get specific knowledge entry
	s.server.AddTool(mcp.NewTool("get_kb",
		mcp.WithDescription("Get a specific knowledge base entry"),
		mcp.WithString("uuid", mcp.Required(), mcp.Description("UUID of the knowledge entry")),
	), s.getKnowledgeHandler)

	return nil
}

// executeToolHandler handles tool execution
func (s *Server) executeToolHandler(ctx context.Context, request mcp.CallToolRequest) (*mcp.CallToolResult, error) {
	args := request.Params.Arguments

	toolName, ok := args["tool_name"].(string)
	if !ok || toolName == "" {
		return mcp.NewToolResultError("tool_name parameter is required"), nil
	}

	runner, _ := args["runner"].(string)
	toolArgs, _ := args["args"].(map[string]interface{})
	integrationTemplate, _ := args["integration_template"].(string)

	// Create tool definition
	toolDef := map[string]interface{}{
		"name": toolName,
		"args": toolArgs,
	}

	if integrationTemplate != "" {
		toolDef["integration_template"] = integrationTemplate
	}

	// Enhanced runner selection logic using production methods
	if runner == "" || runner == "default" {
		runner = "default"
	}
	// Note: "auto" runner selection is already handled by ExecuteToolWithTimeout method

	// Policy validation (if enabled)
	if s.serverConfig.EnableOPAPolicies {
		allowed, message, err := s.client.ValidateToolExecution(ctx, toolName, toolArgs, runner)
		if err != nil {
			return mcp.NewToolResultError(fmt.Sprintf("Policy validation failed: %v", err)), nil
		}
		if !allowed {
			errorMsg := fmt.Sprintf("Tool execution denied by policy: %s", toolName)
			if message != "" {
				errorMsg += fmt.Sprintf(" - %s", message)
			}
			return mcp.NewToolResultError(errorMsg), nil
		}
	}
	argVals := make(map[string]any) // to get argument values

	// Execute with timeout (5 minutes)
	timeout := 300 * time.Second
	eventChan, err := s.client.ExecuteToolWithTimeout(ctx, toolName, toolDef, runner, timeout, argVals)
	if err != nil {
		return mcp.NewToolResultError(fmt.Sprintf("Failed to execute tool: %v", err)), nil
	}

	var output strings.Builder
	output.WriteString(fmt.Sprintf("🚀 Executing tool: %s\n", toolName))
	if runner != "" {
		output.WriteString(fmt.Sprintf("📍 Runner: %s\n", runner))
	}
	output.WriteString("=" + strings.Repeat("=", 50) + "\n\n")

	for event := range eventChan {
		switch event.Type {
		case "error":
			output.WriteString(fmt.Sprintf("❌ Error: %s\n", event.Data))
			return mcp.NewToolResultText(output.String()), nil
		case "stdout":
			output.WriteString(event.Data)
		case "stderr":
			output.WriteString(fmt.Sprintf("⚠️ %s", event.Data))
		case "done":
			output.WriteString("\n✅ Tool execution completed\n")
		default:
			output.WriteString(fmt.Sprintf("📝 %s: %s\n", event.Type, event.Data))
		}
	}

	return mcp.NewToolResultText(output.String()), nil
}

// Knowledge base handlers

func (s *Server) listKnowledgeHandler(ctx context.Context, request mcp.CallToolRequest) (*mcp.CallToolResult, error) {
	args := request.Params.Arguments
	query, _ := args["query"].(string)
	limit := 50
	if l, ok := args["limit"].(float64); ok {
		limit = int(l)
	}

	knowledge, err := s.client.ListKnowledge(ctx, query, limit)
	if err != nil {
		return mcp.NewToolResultError(fmt.Sprintf("Failed to list knowledge: %v", err)), nil
	}

	data, err := json.MarshalIndent(knowledge, "", "  ")
	if err != nil {
		return mcp.NewToolResultError(fmt.Sprintf("Failed to marshal knowledge: %v", err)), nil
	}

	return mcp.NewToolResultText(string(data)), nil
}

func (s *Server) searchKnowledgeHandler(ctx context.Context, request mcp.CallToolRequest) (*mcp.CallToolResult, error) {
	args := request.Params.Arguments
	query, ok := args["query"].(string)
	if !ok || query == "" {
		return mcp.NewToolResultError("query parameter is required"), nil
	}

	limit := 10
	if l, ok := args["limit"].(float64); ok {
		limit = int(l)
	}

	results, err := s.client.SearchKnowledge(ctx, query, limit)
	if err != nil {
		return mcp.NewToolResultError(fmt.Sprintf("Failed to search knowledge: %v", err)), nil
	}

	data, err := json.MarshalIndent(results, "", "  ")
	if err != nil {
		return mcp.NewToolResultError(fmt.Sprintf("Failed to marshal search results: %v", err)), nil
	}

	return mcp.NewToolResultText(string(data)), nil
}

func (s *Server) getKnowledgeHandler(ctx context.Context, request mcp.CallToolRequest) (*mcp.CallToolResult, error) {
	args := request.Params.Arguments
	uuid, ok := args["uuid"].(string)
	if !ok || uuid == "" {
		return mcp.NewToolResultError("uuid parameter is required"), nil
	}

	knowledge, err := s.client.GetKnowledge(ctx, uuid)
	if err != nil {
		return mcp.NewToolResultError(fmt.Sprintf("Failed to get knowledge: %v", err)), nil
	}

	data, err := json.MarshalIndent(knowledge, "", "  ")
	if err != nil {
		return mcp.NewToolResultError(fmt.Sprintf("Failed to marshal knowledge: %v", err)), nil
	}

	return mcp.NewToolResultText(string(data)), nil
}

// Workflow DSL WASM execution

func (s *Server) workflowDslWasmHandler(ctx context.Context, request mcp.CallToolRequest) (*mcp.CallToolResult, error) {
	args := request.Params.Arguments
	content, ok := args["content"].(string)
	if !ok || content == "" {
		return mcp.NewToolResultError("script parameter is required"), nil
	}

	if err := sem.Acquire(ctx, 1); err != nil {
		return mcp.NewToolResultError(fmt.Sprintf("Failed to acquire semaphore: %v", err)), nil
	}
	defer sem.Release(1)

	p := pluginPool.Get()
	if p == nil {
		return mcp.NewToolResultError("wasm plugin is unavailable"), nil
	}

	wrapped := p.(*struct {
		plugin *extism.Plugin
		ctx    context.Context
	})
	defer pluginPool.Put(p)

	// Execute
	_, output, err := wrapped.plugin.Call("execute_script", []byte(content))
	if err != nil {
		return mcp.NewToolResultError(fmt.Sprintf("Failed to execute script: %v", err)), nil
	}

	return mcp.NewToolResultText(string(output)), nil
}
