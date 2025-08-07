package mcp

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"os"
	"strconv"
	"strings"
	"time"

	"github.com/getsentry/sentry-go"
	"github.com/kubiyabot/cli/internal/kubiya"
	"github.com/kubiyabot/cli/internal/mcp/filter"
	"github.com/kubiyabot/cli/internal/mcp/hooks"
	"github.com/kubiyabot/cli/internal/mcp/middleware"
	"github.com/kubiyabot/cli/internal/mcp/session"
	sentryutil "github.com/kubiyabot/cli/internal/sentry"
	"github.com/mark3labs/mcp-go/mcp"
	"github.com/mark3labs/mcp-go/server"
)

// ProductionServer is a production-ready MCP server with all features
type ProductionServer struct {
	kubiyaClient    *kubiya.Client
	sessionManager  *session.Manager
	logger          *log.Logger
	hooks           hooks.Hooks
	toolFilter      filter.ToolFilter
	middlewareChain middleware.Middleware
	mcpServer       *server.MCPServer
	config          *Config
}

// NewProductionServer creates a new production MCP server
func NewProductionServer(kubiyaClient *kubiya.Client, config *Config) (*ProductionServer, error) {
	// Initialize logger
	logger := log.New(os.Stdout, "[MCP] ", log.LstdFlags|log.Lshortfile)

	// Initialize session manager
	sessionTimeout := 30 * time.Minute
	if config.SessionTimeout > 0 {
		sessionTimeout = time.Duration(config.SessionTimeout) * time.Second
	}
	sessionManager := session.NewManager(sessionTimeout)

	// Initialize hooks
	compositeHooks := hooks.NewCompositeHooks(
		hooks.NewLoggingHooks(logger),
		hooks.NewSentryHooks(),
		hooks.NewMetricsHooks(),
	)

	// Initialize filters
	var toolFilters []filter.ToolFilter

	// Permission filter
	permissionFilter := filter.NewPermissionFilter(config.ToolPermissions)
	toolFilters = append(toolFilters, permissionFilter.Apply)

	// Environment filter
	envFilter := filter.NewEnvironmentFilter()
	toolFilters = append(toolFilters, envFilter.Apply)

	// Time-based filter (if enabled)
	if config.EnableTimeRestrictions {
		timeFilter := filter.NewTimeBasedFilter()
		toolFilters = append(toolFilters, timeFilter.Apply)
	}

	// Feature flag filter (if provided)
	if config.FeatureFlags != nil {
		featureFilter := filter.NewFeatureFlagFilter(config.FeatureFlags)
		toolFilters = append(toolFilters, featureFilter.Apply)
	}

	// Chain all filters
	chainedFilter := filter.Chain(toolFilters...)

	// Initialize middleware
	var middlewares []middleware.Middleware

	// Error recovery should be first (outermost)
	recoveryMW := middleware.NewErrorRecoveryMiddleware(logger)
	middlewares = append(middlewares, recoveryMW.Apply)

	// Timeout middleware with extended defaults for long-running tools
	defaultTimeout := 20 * time.Minute // Increased from 5 to 20 minutes
	timeoutMW := middleware.NewTimeoutMiddleware(defaultTimeout)
	
	// Set extended timeouts for known long-running tools
	longRunningTools := map[string]time.Duration{
		"execute_tool":            30 * time.Minute, // Tool execution can be very long
		"execute_workflow":        45 * time.Minute, // Workflows can be complex
		"create_on_demand_tool":   25 * time.Minute, // Dynamic tool creation + execution
		"execute_whitelisted_tool": 30 * time.Minute, // Whitelisted tools may be complex
		"workflow_dsl_wasm":       15 * time.Minute, // WASM execution can be slow
		"chat_with_agent":         10 * time.Minute, // Agent conversations can be lengthy
	}
	
	for tool, timeout := range longRunningTools {
		timeoutMW.SetToolTimeout(tool, timeout)
	}
	
	// Apply user-configured timeouts (override defaults)
	if config.ToolTimeouts != nil {
		for tool, timeout := range config.ToolTimeouts {
			timeoutMW.SetToolTimeout(tool, time.Duration(timeout)*time.Second)
		}
	}
	middlewares = append(middlewares, timeoutMW.Apply)

	// Logging middleware
	loggingMW := middleware.NewLoggingMiddleware(logger)
	middlewares = append(middlewares, loggingMW.Apply)

	// Rate limiting
	rateLimitMW := middleware.NewRateLimitMiddleware(config.RateLimit.RequestsPerSecond, config.RateLimit.Burst)
	middlewares = append(middlewares, rateLimitMW.Apply)

	// Authentication (if required)
	if config.RequireAuth {
		authMW := middleware.NewAuthMiddleware(sessionManager, true)
		middlewares = append(middlewares, authMW.Apply)
	}

	// Permission middleware
	permMW := NewPermissionMiddleware(config.ToolPermissions)
	middlewares = append(middlewares, permMW.Apply)

	// Chain all middleware
	chainedMiddleware := middleware.Chain(middlewares...)

	// Create server instance
	ps := &ProductionServer{
		kubiyaClient:    kubiyaClient,
		sessionManager:  sessionManager,
		logger:          logger,
		hooks:           compositeHooks,
		toolFilter:      chainedFilter,
		middlewareChain: chainedMiddleware,
		config:          config,
	}

	// Create MCP server
	serverOpts := []server.ServerOption{
		server.WithPromptCapabilities(true),
		server.WithResourceCapabilities(true, true),
		server.WithToolCapabilities(true),
	}

	ps.mcpServer = server.NewMCPServer(config.ServerName, config.ServerVersion, serverOpts...)

	// Register handlers
	ps.registerHandlers()

	return ps, nil
}

// registerHandlers registers all the MCP handlers with middleware and hooks
func (ps *ProductionServer) registerHandlers() {
	// Note: MCP-go doesn't have OnConnection/OnDisconnection methods
	// We'll need to handle session lifecycle differently
	// For now, we'll create sessions on first tool call

	// Register tool handlers
	ps.registerTools()

	// Register resource handlers
	ps.registerResources()

	// Register prompt handlers
	ps.registerPrompts()
}

// registerTools registers all available tools with middleware
func (ps *ProductionServer) registerTools() {
	// Get all available tools
	allTools := ps.getAllTools()

	// Register each tool with middleware
	for _, tool := range allTools {
		toolCopy := tool // Capture loop variable

		// Create handler
		handler := ps.createToolHandler(toolCopy.Name)

		// Wrap with middleware
		wrappedHandler := ps.middlewareChain(handler)

		// Register with MCP server
		ps.mcpServer.AddTool(
			toolCopy,
			func(ctx context.Context, req mcp.CallToolRequest) (*mcp.CallToolResult, error) {
				// Add session to context
				sessionID := ps.getSessionIDFromContext(ctx)
				if sess, exists := ps.sessionManager.GetSession(sessionID); exists {
					ctx = session.ContextWithSession(ctx, sess)
				}

				// Track timing
				start := time.Now()

				// Call wrapped handler
				result, err := wrappedHandler(ctx, req)

				// Call hooks
				duration := time.Since(start)
				ps.hooks.OnToolCall(ctx, sessionID, req.Params.Name, duration, err)

				return result, err
			},
		)
	}
}

// createToolHandler creates a handler for a specific tool
func (ps *ProductionServer) createToolHandler(toolName string) middleware.ToolHandler {
	switch toolName {
	case "execute_tool":
		return ps.handleExecuteTool
	case "list_runners":
		return ps.handleListRunners
	case "list_sources":
		return ps.handleListSources
	case "search_tools":
		return ps.handleSearchTools
	case "execute_tool_from_source":
		return ps.handleExecuteToolFromSource
	case "search_kb":
		return ps.handleSearchKB
	default:
		// Check whitelisted tools
		if ps.config.WhitelistedTools != nil {
			for _, wt := range ps.config.WhitelistedTools {
				if wt.Name == toolName {
					return ps.createWhitelistedToolHandler(wt)
				}
			}
		}

		// Default handler
		return func(ctx context.Context, req mcp.CallToolRequest) (*mcp.CallToolResult, error) {
			return mcp.NewToolResultError(fmt.Sprintf("Unknown tool: %s", toolName)), nil
		}
	}
}

// Tool handler implementations
func (ps *ProductionServer) handleExecuteTool(ctx context.Context, req mcp.CallToolRequest) (*mcp.CallToolResult, error) {
	// Extract parameters
	var toolDef map[string]interface{}
	var toolName string
	var err error

	// Check if we have a tool URL
	if toolURL, ok := req.Params.Arguments["tool_url"].(string); ok && toolURL != "" {
		ps.logger.Printf("Loading tool from URL: %s", toolURL)

		// Fetch the tool definition from URL
		resp, err := http.Get(toolURL)
		if err != nil {
			return mcp.NewToolResultError(fmt.Sprintf("Failed to fetch tool from URL: %v", err)), nil
		}
		defer resp.Body.Close()

		if resp.StatusCode != http.StatusOK {
			return mcp.NewToolResultError(fmt.Sprintf("Failed to fetch tool: HTTP %d", resp.StatusCode)), nil
		}

		// Parse the tool definition
		if err := json.NewDecoder(resp.Body).Decode(&toolDef); err != nil {
			return mcp.NewToolResultError(fmt.Sprintf("Failed to parse tool definition: %v", err)), nil
		}

		// Extract tool name
		if name, ok := toolDef["name"].(string); ok {
			toolName = name
		} else {
			return mcp.NewToolResultError("Tool definition must include a name"), nil
		}
	} else if sourceUUID, ok := req.Params.Arguments["source_uuid"].(string); ok && sourceUUID != "" {
		// Load from source
		toolName, ok = req.Params.Arguments["tool_name"].(string)
		if !ok || toolName == "" {
			return mcp.NewToolResultError("tool_name is required when using source_uuid"), nil
		}

		ps.logger.Printf("Loading tool %s from source %s", toolName, sourceUUID)

		// Fetch the source
		source, err := ps.kubiyaClient.GetSource(ctx, sourceUUID)
		if err != nil {
			return mcp.NewToolResultError(fmt.Sprintf("Failed to fetch source: %v", err)), nil
		}

		// Find the tool
		var found bool
		for _, tool := range source.Tools {
			if tool.Name == toolName {
				// Convert tool to map
				toolJSON, err := json.Marshal(tool)
				if err != nil {
					return mcp.NewToolResultError(fmt.Sprintf("Failed to marshal tool: %v", err)), nil
				}
				if err := json.Unmarshal(toolJSON, &toolDef); err != nil {
					return mcp.NewToolResultError(fmt.Sprintf("Failed to unmarshal tool: %v", err)), nil
				}
				found = true
				break
			}
		}

		// Also check inline tools
		if !found {
			for _, tool := range source.InlineTools {
				if tool.Name == toolName {
					// Convert tool to map
					toolJSON, err := json.Marshal(tool)
					if err != nil {
						return mcp.NewToolResultError(fmt.Sprintf("Failed to marshal tool: %v", err)), nil
					}
					if err := json.Unmarshal(toolJSON, &toolDef); err != nil {
						return mcp.NewToolResultError(fmt.Sprintf("Failed to unmarshal tool: %v", err)), nil
					}
					found = true
					break
				}
			}
		}

		if !found {
			return mcp.NewToolResultError(fmt.Sprintf("Tool '%s' not found in source %s", toolName, sourceUUID)), nil
		}

		ps.logger.Printf("Loaded tool: %s", toolName)
	} else {
		// Use provided tool definition
		toolDef, ok := req.Params.Arguments["tool_def"].(map[string]interface{})
		if !ok {
			return mcp.NewToolResultError("tool_def, tool_url, or source_uuid is required"), nil
		}

		toolName, _ = req.Params.Arguments["tool_name"].(string)
		if toolName == "" {
			if name, ok := toolDef["name"].(string); ok {
				toolName = name
			} else {
				return mcp.NewToolResultError("tool name is required"), nil
			}
		}
	}

	// Apply integrations if specified
	if integrations, ok := req.Params.Arguments["integrations"].([]interface{}); ok {
		ps.logger.Printf("Applying %d integrations to tool", len(integrations))

		// Import the cli package for integration support
		// We need to create a lightweight integration manager for the MCP server
		for _, integration := range integrations {
			integrationName, ok := integration.(string)
			if !ok {
				continue
			}

			ps.logger.Printf("Applying integration: %s", integrationName)

			// Apply integration based on name
			// For now, we'll apply the most common integrations inline
			// In a production setup, this would use the same integration system
			switch integrationName {
			case "kubernetes/incluster":
				// Apply Kubernetes in-cluster integration
				if toolDef["image"] == nil || toolDef["image"] == "" {
					toolDef["image"] = "bitnami/kubectl:latest"
				}

				// Add file mappings
				withFiles, _ := toolDef["with_files"].([]interface{})
				withFiles = append(withFiles,
					map[string]interface{}{
						"source":      "/var/run/secrets/kubernetes.io/serviceaccount/token",
						"destination": "/var/run/secrets/kubernetes.io/serviceaccount/token",
					},
					map[string]interface{}{
						"source":      "/var/run/secrets/kubernetes.io/serviceaccount/ca.crt",
						"destination": "/var/run/secrets/kubernetes.io/serviceaccount/ca.crt",
					},
				)
				toolDef["with_files"] = withFiles

				// Add environment variables
				env, _ := toolDef["env"].([]interface{})
				env = append(env,
					"KUBERNETES_SERVICE_HOST=kubernetes.default.svc",
					"KUBERNETES_SERVICE_PORT=443",
				)
				toolDef["env"] = env

				// Wrap content with setup script
				content, _ := toolDef["content"].(string)
				toolDef["content"] = `#!/bin/bash
set -e

# Setup Kubernetes in-cluster authentication
if [ -f /var/run/secrets/kubernetes.io/serviceaccount/token ]; then
    export KUBE_TOKEN=$(cat /var/run/secrets/kubernetes.io/serviceaccount/token)
    kubectl config set-cluster in-cluster \
        --server=https://kubernetes.default.svc \
        --certificate-authority=/var/run/secrets/kubernetes.io/serviceaccount/ca.crt > /dev/null 2>&1
    kubectl config set-credentials in-cluster --token=$KUBE_TOKEN > /dev/null 2>&1
    kubectl config set-context in-cluster --cluster=in-cluster --user=in-cluster > /dev/null 2>&1
    kubectl config use-context in-cluster > /dev/null 2>&1
    echo "✓ Kubernetes in-cluster authentication configured"
fi

# User script
` + content

			case "aws/cli":
				// Apply AWS CLI integration
				if toolDef["image"] == nil || toolDef["image"] == "" {
					toolDef["image"] = "amazon/aws-cli:latest"
				}

				// Add file mappings
				withFiles, _ := toolDef["with_files"].([]interface{})
				withFiles = append(withFiles,
					map[string]interface{}{
						"source":      "~/.aws/credentials",
						"destination": "/root/.aws/credentials",
					},
					map[string]interface{}{
						"source":      "~/.aws/config",
						"destination": "/root/.aws/config",
					},
				)
				toolDef["with_files"] = withFiles

				// Add environment variables
				env, _ := toolDef["env"].([]interface{})
				env = append(env,
					"AWS_PROFILE=${AWS_PROFILE:-default}",
					"AWS_DEFAULT_REGION=${AWS_DEFAULT_REGION:-us-east-1}",
				)
				toolDef["env"] = env

			case "database/postgres":
				// Apply PostgreSQL integration
				if toolDef["image"] == nil || toolDef["image"] == "" {
					toolDef["image"] = "postgres:15-alpine"
				}

				// Add services
				withServices, _ := toolDef["with_services"].([]interface{})
				withServices = append(withServices, "postgres:15")
				toolDef["with_services"] = withServices

				// Add environment variables
				env, _ := toolDef["env"].([]interface{})
				env = append(env,
					"PGHOST=${PGHOST:-postgres}",
					"PGPORT=${PGPORT:-5432}",
					"PGDATABASE=${PGDATABASE:-postgres}",
					"PGUSER=${PGUSER:-postgres}",
					"PGPASSWORD=${PGPASSWORD:-postgres}",
				)
				toolDef["env"] = env

				// Wrap content with wait script
				content, _ := toolDef["content"].(string)
				toolDef["content"] = `#!/bin/bash
set -e

# Wait for PostgreSQL to be ready
for i in {1..30}; do
    if pg_isready -h $PGHOST -p $PGPORT -U $PGUSER; then
        echo "✓ PostgreSQL is ready"
        break
    fi
    echo "Waiting for PostgreSQL... ($i/30)"
    sleep 1
done

# User script
` + content
			}
		}
	}
	var argVals map[string]any

	// Apply any additional parameters to the tool definition
	// This allows overriding specific properties
	if args, ok := req.Params.Arguments["args"].(map[string]interface{}); ok {
		toolDef["args"] = args
		argVals = args // Store for later use
	}
	if env, ok := req.Params.Arguments["env"].([]interface{}); ok {
		toolDef["env"] = env
	}
	if withFiles, ok := req.Params.Arguments["with_files"].([]interface{}); ok {
		toolDef["with_files"] = withFiles
	}
	if withVolumes, ok := req.Params.Arguments["with_volumes"].([]interface{}); ok {
		toolDef["with_volumes"] = withVolumes
	}
	if withServices, ok := req.Params.Arguments["with_services"].([]interface{}); ok {
		toolDef["with_services"] = withServices
	}

	runner, _ := req.Params.Arguments["runner"].(string)
	if runner == "" {
		runner = "auto"
	}

	// Log the tool definition for debugging
	if ps.logger != nil {
		toolDefJSON, _ := json.MarshalIndent(toolDef, "", "  ")
		ps.logger.Printf("Executing tool %s with definition:\n%s", toolName, string(toolDefJSON))
	}

	// Execute with timeout
	timeout := 5 * time.Minute
	if timeoutSecs, ok := req.Params.Arguments["timeout"].(float64); ok && timeoutSecs > 0 {
		timeout = time.Duration(timeoutSecs) * time.Second
	}

	events, err := ps.kubiyaClient.ExecuteToolWithTimeout(ctx, toolName, toolDef, runner, timeout, argVals)
	if err != nil {
		return mcp.NewToolResultError(fmt.Sprintf("Failed to execute tool: %v", err)), nil
	}

	// Collect output
	var output strings.Builder
	var hasError bool
	for event := range events {
		switch event.Type {
		case "data":
			// Try to parse as JSON
			var data map[string]interface{}
			if err := json.Unmarshal([]byte(event.Data), &data); err == nil {
				// Handle different event types
				if eventType, ok := data["type"].(string); ok {
					switch eventType {
					case "tool-output":
						if content, ok := data["content"].(string); ok {
							output.WriteString(content)
						}
					case "log":
						// Optional: Include logs in output
						if content, ok := data["content"].(string); ok {
							ps.logger.Printf("[Tool Log] %s", content)
						}
					case "status":
						if status, ok := data["status"].(string); ok {
							if status != "success" {
								hasError = true
							}
						}
					}
				} else if outputStr, ok := data["output"].(string); ok {
					// Backward compatibility
					output.WriteString(outputStr)
				}
			} else {
				// If not JSON, treat as plain text output
				output.WriteString(event.Data)
			}
		case "error":
			hasError = true
			return mcp.NewToolResultError(fmt.Sprintf("Tool execution error: %s", event.Data)), nil
		}
	}

	if hasError {
		return mcp.NewToolResultError("Tool execution failed"), nil
	}

	return mcp.NewToolResultText(output.String()), nil
}

func (ps *ProductionServer) handleListRunners(ctx context.Context, req mcp.CallToolRequest) (*mcp.CallToolResult, error) {
	runners, err := ps.kubiyaClient.ListRunners(ctx)
	if err != nil {
		return mcp.NewToolResultError(fmt.Sprintf("Failed to list runners: %v", err)), nil
	}

	// Format output
	var output strings.Builder
	output.WriteString("Available Runners:\n\n")

	for _, runner := range runners {
		status := "unknown"
		if runner.RunnerHealth.Status != "" {
			status = runner.RunnerHealth.Status
		} else if runner.RunnerHealth.Health != "" {
			status = runner.RunnerHealth.Health
		}

		output.WriteString(fmt.Sprintf("- %s\n", runner.Name))
		output.WriteString(fmt.Sprintf("  Status: %s\n", status))
		if runner.Description != "" {
			output.WriteString(fmt.Sprintf("  Description: %s\n", runner.Description))
		}
		output.WriteString("\n")
	}

	return mcp.NewToolResultText(output.String()), nil
}

func (ps *ProductionServer) handleListSources(ctx context.Context, req mcp.CallToolRequest) (*mcp.CallToolResult, error) {
	args := req.Params.Arguments
	
	// Parse pagination parameters
	page := 1
	if pageStr, ok := args["page"].(string); ok {
		if p, err := strconv.Atoi(pageStr); err == nil && p > 0 {
			page = p
		}
	} else if pageFloat, ok := args["page"].(float64); ok && pageFloat > 0 {
		page = int(pageFloat)
	}
	
	// Set default page size with fallback
	pageSize := ps.config.DefaultPageSize
	if pageSize <= 0 {
		pageSize = 20 // Default fallback
	}
	maxResponseSize := ps.config.MaxResponseSize
	if maxResponseSize <= 0 {
		maxResponseSize = 51200 // 50KB default
	}
	maxToolsInResponse := ps.config.MaxToolsInResponse
	if maxToolsInResponse <= 0 {
		maxToolsInResponse = 50 // Default max tools
	}
	
	if pageSizeStr, ok := args["page_size"].(string); ok {
		if ps, err := strconv.Atoi(pageSizeStr); err == nil && ps > 0 {
			pageSize = ps
		}
	} else if pageSizeFloat, ok := args["page_size"].(float64); ok && pageSizeFloat > 0 {
		pageSize = int(pageSizeFloat)
	}
	
	// Limit page size to maximum configured
	if pageSize > maxToolsInResponse {
		pageSize = maxToolsInResponse
	}

	// Get sources from API
	sources, err := ps.kubiyaClient.ListSources(ctx)
	if err != nil {
		return mcp.NewToolResultError(fmt.Sprintf("Failed to list sources: %v", err)), nil
	}

	// Convert to metadata-only format
	var sourceMetadata []interface{}
	for _, source := range sources {
		// Get basic metadata for each source
		metadata := map[string]interface{}{
			"uuid":        source.UUID,
			"name":        source.Name,
			"description": source.Description,
			"url":         source.URL,
			"tool_count":  len(source.Tools) + len(source.InlineTools),
			"created_at":  source.CreatedAt,
			"updated_at":  source.UpdatedAt,
			"status":      "active", // Default status
		}
		
		// Note: Git metadata would need to be fetched separately if needed
		
		sourceMetadata = append(sourceMetadata, metadata)
	}

	// Apply pagination
	paginatedItems, currentPage, totalPages, hasMore := paginateItems(sourceMetadata, page, pageSize)

	// Create response with pagination info
	response := map[string]interface{}{
		"sources": paginatedItems,
		"pagination": map[string]interface{}{
			"page":        currentPage,
			"page_size":   pageSize,
			"total_pages": totalPages,
			"total_items": len(sourceMetadata),
			"has_more":    hasMore,
		},
	}

	// Marshal and apply size limits
	data, err := json.MarshalIndent(response, "", "  ")
	if err != nil {
		return mcp.NewToolResultError(fmt.Sprintf("Failed to marshal sources: %v", err)), nil
	}

	// Apply response size limit
	if len(data) > maxResponseSize {
		truncated := data[:maxResponseSize-100] // Reserve space for truncation message
		truncated = append(truncated, []byte("\n\n... Response truncated due to size limit ...")...)
		data = truncated
	}
	
	return mcp.NewToolResultText(string(data)), nil
}

func (ps *ProductionServer) handleSearchTools(ctx context.Context, req mcp.CallToolRequest) (*mcp.CallToolResult, error) {
	args := req.Params.Arguments
	
	// Get required query parameter
	query, ok := args["query"].(string)
	if !ok || query == "" {
		return mcp.NewToolResultError("query parameter is required"), nil
	}
	
	// Parse pagination parameters
	page := 1
	if pageStr, ok := args["page"].(string); ok {
		if p, err := strconv.Atoi(pageStr); err == nil && p > 0 {
			page = p
		}
	} else if pageFloat, ok := args["page"].(float64); ok && pageFloat > 0 {
		page = int(pageFloat)
	}
	
	// Set default page size with fallback
	pageSize := ps.config.DefaultPageSize
	if pageSize <= 0 {
		pageSize = 20 // Default fallback
	}
	maxResponseSize := ps.config.MaxResponseSize
	if maxResponseSize <= 0 {
		maxResponseSize = 51200 // 50KB default
	}
	maxToolsInResponse := ps.config.MaxToolsInResponse
	if maxToolsInResponse <= 0 {
		maxToolsInResponse = 50 // Default max tools
	}
	
	if pageSizeStr, ok := args["page_size"].(string); ok {
		if ps, err := strconv.Atoi(pageSizeStr); err == nil && ps > 0 {
			pageSize = ps
		}
	} else if pageSizeFloat, ok := args["page_size"].(float64); ok && pageSizeFloat > 0 {
		pageSize = int(pageSizeFloat)
	}
	
	// Limit page size to maximum configured
	if pageSize > maxToolsInResponse {
		pageSize = maxToolsInResponse
	}
	
	// Get optional filter parameters
	sourceUUID, _ := args["source_uuid"].(string)
	toolType, _ := args["tool_type"].(string)
	longRunningOnly, _ := args["long_running_only"].(bool)

	// Get all sources
	sources, err := ps.kubiyaClient.ListSources(ctx)
	if err != nil {
		return mcp.NewToolResultError(fmt.Sprintf("Failed to list sources: %v", err)), nil
	}

	// Filter sources if sourceUUID is provided
	if sourceUUID != "" {
		var filteredSources []kubiya.Source
		for _, source := range sources {
			if source.UUID == sourceUUID {
				filteredSources = append(filteredSources, source)
				break
			}
		}
		sources = filteredSources
	}

	// Search through all tools
	var allMatches []map[string]interface{}
	queryLower := strings.ToLower(query)

	for _, source := range sources {
		// Get source metadata to access tools
		metadata, err := ps.kubiyaClient.GetSourceMetadata(ctx, source.UUID)
		if err != nil {
			continue // Skip sources that can't be accessed
		}

		// Search through regular tools
		for _, tool := range metadata.Tools {
			if matchesToolCriteria(tool, queryLower, toolType, longRunningOnly) {
				toolMeta := map[string]interface{}{
					"name":         tool.Name,
					"description":  tool.Description,
					"source_uuid":  source.UUID,
					"source_name":  source.Name,
					"type":         tool.Type,
					"arg_count":    len(tool.Args),
					"long_running": tool.LongRunning,
				}
				allMatches = append(allMatches, toolMeta)
			}
		}

		// Search through inline tools
		for _, tool := range metadata.InlineTools {
			if matchesToolCriteria(tool, queryLower, toolType, longRunningOnly) {
				toolMeta := map[string]interface{}{
					"name":         tool.Name,
					"description":  tool.Description,
					"source_uuid":  source.UUID,
					"source_name":  source.Name,
					"type":         tool.Type,
					"arg_count":    len(tool.Args),
					"long_running": tool.LongRunning,
				}
				allMatches = append(allMatches, toolMeta)
			}
		}
	}

	// Convert to interface{} slice for pagination
	var items []interface{}
	for _, match := range allMatches {
		items = append(items, match)
	}

	// Apply pagination
	paginatedItems, currentPage, totalPages, hasMore := paginateItems(items, page, pageSize)

	// Create response
	response := map[string]interface{}{
		"tools": paginatedItems,
		"pagination": map[string]interface{}{
			"page":        currentPage,
			"page_size":   pageSize,
			"total_pages": totalPages,
			"total_items": len(allMatches),
			"has_more":    hasMore,
		},
		"query": query,
		"filters": map[string]interface{}{
			"source_uuid":       sourceUUID,
			"tool_type":         toolType,
			"long_running_only": longRunningOnly,
		},
	}

	// Marshal and apply size limits
	data, err := json.MarshalIndent(response, "", "  ")
	if err != nil {
		return mcp.NewToolResultError(fmt.Sprintf("Failed to marshal search results: %v", err)), nil
	}

	// Apply response size limit
	if len(data) > maxResponseSize {
		truncated := data[:maxResponseSize-100] // Reserve space for truncation message
		truncated = append(truncated, []byte("\n\n... Response truncated due to size limit ...")...)
		data = truncated
	}
	
	return mcp.NewToolResultText(string(data)), nil
}

// matchesToolCriteria checks if a tool matches the search criteria
func matchesToolCriteria(tool kubiya.Tool, queryLower, toolType string, longRunningOnly bool) bool {
	// Check query match (name or description)
	toolNameLower := strings.ToLower(tool.Name)
	toolDescLower := strings.ToLower(tool.Description)
	if !strings.Contains(toolNameLower, queryLower) && !strings.Contains(toolDescLower, queryLower) {
		return false
	}
	
	// Check tool type filter
	if toolType != "" && strings.ToLower(tool.Type) != strings.ToLower(toolType) {
		return false
	}
	
	// Check long running filter
	if longRunningOnly && !tool.LongRunning {
		return false
	}
	
	return true
}

func (ps *ProductionServer) handleExecuteToolFromSource(ctx context.Context, req mcp.CallToolRequest) (*mcp.CallToolResult, error) {
	args := req.Params.Arguments

	// Get required parameters
	sourceIdentifier, ok := args["source"].(string)
	if !ok || sourceIdentifier == "" {
		return mcp.NewToolResultError("source parameter is required (UUID or URL)"), nil
	}

	toolName, ok := args["tool_name"].(string)
	if !ok || toolName == "" {
		return mcp.NewToolResultError("tool_name parameter is required"), nil
	}

	// Get optional parameters
	runner, _ := args["runner"].(string)
	if runner == "" {
		runner = "auto"
	}

	toolArgs, _ := args["args"].(map[string]interface{})
	env, _ := args["env"].([]interface{})
	withFiles, _ := args["with_files"].([]interface{})
	withVolumes, _ := args["with_volumes"].([]interface{})
	withServices, _ := args["with_services"].([]interface{})

	timeout := 300 // 5 minutes default
	if timeoutFloat, ok := args["timeout"].(float64); ok && timeoutFloat > 0 {
		timeout = int(timeoutFloat)
	}

	// First, get the source (either by UUID or URL)
	var source *kubiya.Source
	var err error

	sources, err := ps.kubiyaClient.ListSources(ctx)
	if err != nil {
		return mcp.NewToolResultError(fmt.Sprintf("Failed to list sources: %v", err)), nil
	}

	// Try to find source by UUID or URL
	for _, s := range sources {
		if s.UUID == sourceIdentifier || s.URL == sourceIdentifier || s.Name == sourceIdentifier {
			source = &s
			break
		}
	}

	if source == nil {
		return mcp.NewToolResultError(fmt.Sprintf("Source not found: %s", sourceIdentifier)), nil
	}

	// Get source metadata to find tools
	sourceMetadata, err := ps.kubiyaClient.GetSourceMetadata(ctx, source.UUID)
	if err != nil {
		return mcp.NewToolResultError(fmt.Sprintf("Failed to get source metadata: %v", err)), nil
	}

	// Find the tool in the source
	var foundTool *kubiya.Tool
	for _, tool := range sourceMetadata.Tools {
		if tool.Name == toolName {
			foundTool = &tool
			break
		}
	}

	// Also check inline tools
	if foundTool == nil {
		for _, tool := range sourceMetadata.InlineTools {
			if tool.Name == toolName {
				foundTool = &tool
				break
			}
		}
	}

	if foundTool == nil {
		return mcp.NewToolResultError(fmt.Sprintf("Tool '%s' not found in source '%s'", toolName, sourceIdentifier)), nil
	}

	// Create tool definition for execution
	toolDef := map[string]interface{}{
		"name":        foundTool.Name,
		"description": foundTool.Description,
		"type":        foundTool.Type,
		"content":     foundTool.Content,
		"image":       foundTool.Image,
	}

	// Apply provided overrides
	if toolArgs != nil {
		toolDef["args"] = toolArgs
	}
	if env != nil {
		toolDef["env"] = env
	}
	if withFiles != nil {
		toolDef["with_files"] = withFiles
	}
	if withVolumes != nil {
		toolDef["with_volumes"] = withVolumes
	}
	if withServices != nil {
		toolDef["with_services"] = withServices
	}

	// Execute tool with timeout
	timeoutDuration := time.Duration(timeout) * time.Second
	argVals := make(map[string]any)
	if toolArgs != nil {
		argVals = toolArgs
	}

	events, err := ps.kubiyaClient.ExecuteToolWithTimeout(ctx, foundTool.Name, toolDef, runner, timeoutDuration, argVals)
	if err != nil {
		return mcp.NewToolResultError(fmt.Sprintf("Failed to execute tool: %v", err)), nil
	}

	// Collect output with size limits
	var output strings.Builder
	maxResponseSize := ps.config.MaxResponseSize
	if maxResponseSize <= 0 {
		maxResponseSize = 51200 // 50KB default
	}

	output.WriteString(fmt.Sprintf("🚀 Executing tool: %s (from source: %s)\n", foundTool.Name, source.Name))
	output.WriteString(fmt.Sprintf("📍 Runner: %s\n", runner))
	output.WriteString("=" + strings.Repeat("=", 50) + "\n\n")

	for event := range events {
		// Check size limit before adding more content
		if output.Len() > maxResponseSize-1000 {
			output.WriteString("\n... Output truncated due to size limit ...\n")
			break
		}

		switch event.Type {
		case "data":
			// Try to parse as JSON for structured events
			var data map[string]interface{}
			if err := json.Unmarshal([]byte(event.Data), &data); err == nil {
				if eventType, ok := data["type"].(string); ok {
					switch eventType {
					case "tool-output":
						if content, ok := data["content"].(string); ok {
							output.WriteString(content)
						}
					case "log":
						if content, ok := data["content"].(string); ok {
							output.WriteString(fmt.Sprintf("📝 %s\n", content))
						}
					case "status":
						if status, ok := data["status"].(string); ok && status != "success" {
							output.WriteString(fmt.Sprintf("⚠️ Status: %s\n", status))
						}
					}
				} else if outputStr, ok := data["output"].(string); ok {
					output.WriteString(outputStr)
				}
			} else {
				// Plain text output
				output.WriteString(event.Data)
			}
		case "error":
			output.WriteString(fmt.Sprintf("❌ Error: %s\n", event.Data))
			return mcp.NewToolResultError(output.String()), nil
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

func (ps *ProductionServer) handleSearchKB(ctx context.Context, req mcp.CallToolRequest) (*mcp.CallToolResult, error) {
	args := req.Params.Arguments
	query, ok := args["query"].(string)
	if !ok || query == "" {
		return mcp.NewToolResultError("query parameter is required"), nil
	}

	// Use the working Query method instead of SearchKnowledge
	knowledgeReq := kubiya.KnowledgeQueryRequest{
		Query: query,
		OrgID: ps.config.OrgID, // Automatically set org_id from user config
		// BearerToken will be set automatically by the Query method from the client config
		ResponseFormat: "vercel",
	}

	events, err := ps.kubiyaClient.Knowledge().Query(ctx, knowledgeReq)
	if err != nil {
		return mcp.NewToolResultError(fmt.Sprintf("Failed to search knowledge: %v", err)), nil
	}

	// Accumulate the streaming response
	var output strings.Builder
	var hasError bool

	for event := range events {
		switch event.Type {
		case "data":
			output.WriteString(event.Data)
		case "error":
			hasError = true
			output.WriteString(fmt.Sprintf("❌ Error: %s\n", event.Data))
		case "done":
			// Query completed successfully
			break
		}
	}

	if hasError {
		return mcp.NewToolResultError(output.String()), nil
	}

	return mcp.NewToolResultText(output.String()), nil
}

// createWhitelistedToolHandler creates a handler for whitelisted tools
func (ps *ProductionServer) createWhitelistedToolHandler(wt WhitelistedTool) middleware.ToolHandler {
	return func(ctx context.Context, req mcp.CallToolRequest) (*mcp.CallToolResult, error) {
		// Merge tool definition with defaults
		toolDef := make(map[string]interface{})
		for k, v := range wt.DefaultConfig {
			toolDef[k] = v
		}

		// Apply integrations from whitelisted tool config
		if integrations, ok := wt.DefaultConfig["integrations"].([]interface{}); ok {
			// Apply integrations using the same logic as handleExecuteTool
			ps.logger.Printf("Applying %d integrations from whitelisted tool config", len(integrations))

			for _, integration := range integrations {
				integrationName, ok := integration.(string)
				if !ok {
					continue
				}

				ps.logger.Printf("Applying integration: %s", integrationName)

				switch integrationName {
				case "kubernetes/incluster":
					if toolDef["image"] == nil || toolDef["image"] == "" {
						toolDef["image"] = "bitnami/kubectl:latest"
					}

					withFiles, _ := toolDef["with_files"].([]interface{})
					withFiles = append(withFiles,
						map[string]interface{}{
							"source":      "/var/run/secrets/kubernetes.io/serviceaccount/token",
							"destination": "/var/run/secrets/kubernetes.io/serviceaccount/token",
						},
						map[string]interface{}{
							"source":      "/var/run/secrets/kubernetes.io/serviceaccount/ca.crt",
							"destination": "/var/run/secrets/kubernetes.io/serviceaccount/ca.crt",
						},
					)
					toolDef["with_files"] = withFiles

					env, _ := toolDef["env"].([]interface{})
					env = append(env,
						"KUBERNETES_SERVICE_HOST=kubernetes.default.svc",
						"KUBERNETES_SERVICE_PORT=443",
					)
					toolDef["env"] = env

					content, _ := toolDef["content"].(string)
					toolDef["content"] = `#!/bin/bash
set -e

# Setup Kubernetes in-cluster authentication
if [ -f /var/run/secrets/kubernetes.io/serviceaccount/token ]; then
    export KUBE_TOKEN=$(cat /var/run/secrets/kubernetes.io/serviceaccount/token)
    kubectl config set-cluster in-cluster \
        --server=https://kubernetes.default.svc \
        --certificate-authority=/var/run/secrets/kubernetes.io/serviceaccount/ca.crt > /dev/null 2>&1
    kubectl config set-credentials in-cluster --token=$KUBE_TOKEN > /dev/null 2>&1
    kubectl config set-context in-cluster --cluster=in-cluster --user=in-cluster > /dev/null 2>&1
    kubectl config use-context in-cluster > /dev/null 2>&1
    echo "✓ Kubernetes in-cluster authentication configured"
fi

# User script
` + content

				case "aws/cli":
					if toolDef["image"] == nil || toolDef["image"] == "" {
						toolDef["image"] = "amazon/aws-cli:latest"
					}

					withFiles, _ := toolDef["with_files"].([]interface{})
					withFiles = append(withFiles,
						map[string]interface{}{
							"source":      "~/.aws/credentials",
							"destination": "/root/.aws/credentials",
						},
						map[string]interface{}{
							"source":      "~/.aws/config",
							"destination": "/root/.aws/config",
						},
					)
					toolDef["with_files"] = withFiles

					env, _ := toolDef["env"].([]interface{})
					env = append(env,
						"AWS_PROFILE=${AWS_PROFILE:-default}",
						"AWS_DEFAULT_REGION=${AWS_DEFAULT_REGION:-us-east-1}",
					)
					toolDef["env"] = env

				case "database/postgres":
					if toolDef["image"] == nil || toolDef["image"] == "" {
						toolDef["image"] = "postgres:15-alpine"
					}

					withServices, _ := toolDef["with_services"].([]interface{})
					withServices = append(withServices, "postgres:15")
					toolDef["with_services"] = withServices

					env, _ := toolDef["env"].([]interface{})
					env = append(env,
						"PGHOST=${PGHOST:-postgres}",
						"PGPORT=${PGPORT:-5432}",
						"PGDATABASE=${PGDATABASE:-postgres}",
						"PGUSER=${PGUSER:-postgres}",
						"PGPASSWORD=${PGPASSWORD:-postgres}",
					)
					toolDef["env"] = env

					content, _ := toolDef["content"].(string)
					toolDef["content"] = `#!/bin/bash
set -e

# Wait for PostgreSQL to be ready
for i in {1..30}; do
    if pg_isready -h $PGHOST -p $PGPORT -U $PGUSER; then
        echo "✓ PostgreSQL is ready"
        break
    fi
    echo "Waiting for PostgreSQL... ($i/30)"
    sleep 1
done

# User script
` + content
				}
			}
		}

		// Override with provided arguments
		if args, ok := req.Params.Arguments["args"].(map[string]interface{}); ok {
			for k, v := range args {
				toolDef[k] = v
			}
		}

		// Handle custom query for db_query tool
		if wt.Name == "db_query" {
			if query, ok := req.Params.Arguments["query"].(string); ok && query != "" {
				toolDef["content"] = fmt.Sprintf(`psql -c '%s'`, query)
			}
		}

		// Execute tool
		timeout := 5 * time.Minute
		if wt.Timeout > 0 {
			timeout = time.Duration(wt.Timeout) * time.Second
		}
		argVals := make(map[string]any) // to get argument values

		events, err := ps.kubiyaClient.ExecuteToolWithTimeout(ctx, wt.Name, toolDef, wt.Runner, timeout, argVals)
		if err != nil {
			return mcp.NewToolResultError(fmt.Sprintf("Failed to execute %s: %v", wt.Name, err)), nil
		}

		// Collect output
		var output strings.Builder
		for event := range events {
			if event.Type == "data" {
				var data map[string]interface{}
				if err := json.Unmarshal([]byte(event.Data), &data); err == nil {
					if outputStr, ok := data["output"].(string); ok {
						output.WriteString(outputStr)
					}
				}
			} else if event.Type == "error" {
				return mcp.NewToolResultError(fmt.Sprintf("Execution error: %s", event.Data)), nil
			}
		}

		return mcp.NewToolResultText(output.String()), nil
	}
}

// registerResources registers available resources
func (ps *ProductionServer) registerResources() {
	// Add configuration resource
	ps.mcpServer.AddResource(
		mcp.NewResource("config://current", "Current MCP server configuration",
			mcp.WithResourceDescription("Current MCP server configuration in JSON format"),
			mcp.WithMIMEType("application/json"),
		),
		func(ctx context.Context, req mcp.ReadResourceRequest) ([]mcp.ResourceContents, error) {
			// Add session to context
			sessionID := ps.getSessionIDFromContext(ctx)
			if sess, exists := ps.sessionManager.GetSession(sessionID); exists {
				ctx = session.ContextWithSession(ctx, sess)
			}

			// Track timing
			start := time.Now()

			// Serialize config (with sensitive data removed)
			safeConfig := ps.getSafeConfig()
			data, err := json.MarshalIndent(safeConfig, "", "  ")

			// Call hooks
			duration := time.Since(start)
			ps.hooks.OnResourceRead(ctx, sessionID, req.Params.URI, duration, err)

			if err != nil {
				return nil, err
			}

			return []mcp.ResourceContents{
				mcp.TextResourceContents{
					URI:      req.Params.URI,
					MIMEType: "application/json",
					Text:     string(data),
				},
			}, nil
		},
	)
}

// registerPrompts registers available prompts
func (ps *ProductionServer) registerPrompts() {
	// Debug information prompt
	ps.mcpServer.AddPrompt(
		mcp.NewPrompt("debug_info",
			mcp.WithPromptDescription("Get debug information about the current session"),
			mcp.WithArgument("include_logs",
				mcp.ArgumentDescription("Include recent logs"),
			),
		),
		ps.handleDebugInfoPrompt,
	)

	// Tool execution guidance prompt
	ps.mcpServer.AddPrompt(
		mcp.NewPrompt("tool_execution_guide",
			mcp.WithPromptDescription("Get guidance on executing tools with best practices"),
			mcp.WithArgument("tool_name",
				mcp.ArgumentDescription("Name of the tool to get guidance for"),
			),
			mcp.WithArgument("use_case",
				mcp.ArgumentDescription("Specific use case or scenario"),
			),
			mcp.WithArgument("environment",
				mcp.ArgumentDescription("Target environment (dev, staging, prod)"),
			),
		),
		ps.handleToolExecutionGuidePrompt,
	)

	// Workflow generation prompt
	ps.mcpServer.AddPrompt(
		mcp.NewPrompt("workflow_generation",
			mcp.WithPromptDescription("Generate workflows using Kubiya WASM DSL with best practices"),
			mcp.WithArgument("task_description",
				mcp.ArgumentDescription("Description of the task to automate"),
			),
			mcp.WithArgument("tools_needed",
				mcp.ArgumentDescription("List of tools or capabilities needed"),
			),
			mcp.WithArgument("complexity",
				mcp.ArgumentDescription("Workflow complexity level (default: medium)"),
			),
			mcp.WithArgument("environment",
				mcp.ArgumentDescription("Target environment (kubernetes, aws, etc.)"),
			),
		),
		ps.handleWorkflowGenerationPrompt,
	)

	// Source exploration prompt
	ps.mcpServer.AddPrompt(
		mcp.NewPrompt("source_exploration",
			mcp.WithPromptDescription("Get help exploring available sources and their tools"),
			mcp.WithArgument("source_uuid",
				mcp.ArgumentDescription("Specific source UUID to explore (optional)"),
			),
			mcp.WithArgument("tool_type",
				mcp.ArgumentDescription("Type of tools to focus on"),
			),
			mcp.WithArgument("use_case",
				mcp.ArgumentDescription("Your intended use case"),
			),
		),
		ps.handleSourceExplorationPrompt,
	)

	// Best practices prompt
	ps.mcpServer.AddPrompt(
		mcp.NewPrompt("kubiya_best_practices",
			mcp.WithPromptDescription("Get Kubiya platform best practices and recommendations"),
			mcp.WithArgument("topic",
				mcp.ArgumentDescription("Specific topic (tools, workflows, security, etc., default: general)"),
			),
			mcp.WithArgument("role",
				mcp.ArgumentDescription("Your role (developer, devops, admin, default: developer)"),
			),
		),
		ps.handleBestPracticesPrompt,
	)

	// Troubleshooting prompt
	ps.mcpServer.AddPrompt(
		mcp.NewPrompt("troubleshooting",
			mcp.WithPromptDescription("Get help troubleshooting tool execution and workflow issues"),
			mcp.WithArgument("issue_description",
				mcp.ArgumentDescription("Description of the issue you're experiencing"),
			),
			mcp.WithArgument("tool_name",
				mcp.ArgumentDescription("Name of the tool having issues"),
			),
			mcp.WithArgument("error_message",
				mcp.ArgumentDescription("Error message or output"),
			),
		),
		ps.handleTroubleshootingPrompt,
	)

	// Workflow examples prompt with WASM DSL patterns
	ps.mcpServer.AddPrompt(
		mcp.NewPrompt("workflow_examples",
			mcp.WithPromptDescription("Get comprehensive workflow examples using Kubiya WASM DSL with best practices"),
			mcp.WithArgument("pattern",
				mcp.ArgumentDescription("Workflow pattern type (simple, conditional, parallel, error-handling, ci-cd, infrastructure, default: simple)"),
			),
			mcp.WithArgument("use_case",
				mcp.ArgumentDescription("Specific use case to demonstrate"),
			),
		),
		ps.handleWorkflowExamplesPrompt,
	)
}

// Prompt handler implementations

func (ps *ProductionServer) handleDebugInfoPrompt(ctx context.Context, req mcp.GetPromptRequest) (*mcp.GetPromptResult, error) {
	sessionID := ps.getSessionIDFromContext(ctx)
	start := time.Now()

	var content strings.Builder
	content.WriteString("Debug Information\n")
	content.WriteString("=================\n\n")

	if sess, exists := ps.sessionManager.GetSession(sessionID); exists {
		content.WriteString(fmt.Sprintf("Session ID: %s\n", sess.ID))
		content.WriteString(fmt.Sprintf("User ID: %s\n", sess.UserID))
		content.WriteString(fmt.Sprintf("Permissions: %v\n", sess.Permissions))
		content.WriteString(fmt.Sprintf("Session Duration: %v\n", time.Since(sess.StartTime)))
	} else {
		content.WriteString("No active session found\n")
	}

	duration := time.Since(start)
	ps.hooks.OnPromptCall(ctx, sessionID, "debug_info", duration, nil)

	messages := []mcp.PromptMessage{
		mcp.NewPromptMessage(mcp.RoleAssistant, mcp.NewTextContent(content.String())),
	}

	return mcp.NewGetPromptResult("Debug Information", messages), nil
}

func (ps *ProductionServer) handleToolExecutionGuidePrompt(ctx context.Context, req mcp.GetPromptRequest) (*mcp.GetPromptResult, error) {
	args := req.Params.Arguments
	if args == nil {
		return nil, fmt.Errorf("missing arguments")
	}

	toolName := ""
	if val, exists := args["tool_name"]; exists {
		toolName = val
	}
	if toolName == "" {
		return nil, fmt.Errorf("tool_name argument is required")
	}

	useCase := "general"
	if val, exists := args["use_case"]; exists {
		useCase = val
	}
	environment := "development"
	if val, exists := args["environment"]; exists {
		environment = val
	}

	var prompt strings.Builder
	prompt.WriteString(fmt.Sprintf("You are an expert in the Kubiya platform helping with tool execution. "))
	prompt.WriteString(fmt.Sprintf("Provide comprehensive guidance for executing the '%s' tool.\n\n", toolName))

	prompt.WriteString("Please provide:\n")
	prompt.WriteString("1. **Tool Overview**: Brief description of what this tool does\n")
	prompt.WriteString("2. **Prerequisites**: Required setup, permissions, or dependencies\n")
	prompt.WriteString("3. **Best Practices**: Recommended approaches and configurations\n")
	prompt.WriteString("4. **Common Parameters**: Key arguments and their typical values\n")
	prompt.WriteString("5. **Error Prevention**: Common mistakes to avoid\n")
	prompt.WriteString("6. **Integration Tips**: How to use this tool with others\n")
	prompt.WriteString("7. **Environment Considerations**: Specific guidance for the target environment\n\n")

	if useCase != "general" {
		prompt.WriteString(fmt.Sprintf("**Specific Use Case**: %s\n", useCase))
	}
	if environment != "development" {
		prompt.WriteString(fmt.Sprintf("**Target Environment**: %s\n", environment))
	}

	prompt.WriteString("\n**CRITICAL: Pre-Execution Validation and Tool Discovery**\n")
	prompt.WriteString("Before providing guidance, ALWAYS include these validation steps:\n")
	prompt.WriteString("1. **Runner Validation**: Use `list_runners` and `check_runner_health` to find healthy runners\n")
	prompt.WriteString("2. **User Runner Choice**: If multiple healthy runners, ASK USER to choose\n")
	prompt.WriteString("3. **Secret Validation**: Use `list_secrets` to verify required secrets exist\n")
	prompt.WriteString("4. **Missing Dependencies**: If secrets/dependencies missing, ASK USER to create them\n")
	prompt.WriteString("5. **Tool Discovery**: Use `search_tools` to find related/alternative tools\n")
	prompt.WriteString("6. **Source Exploration**: Use `list_sources` to understand tool ecosystem\n")
	prompt.WriteString("7. **Reliable Execution**: Use `execute_tool_from_source` with source UUIDs\n")
	prompt.WriteString("8. **Smart Container Selection**: Recommend appropriate containers/executors\n")
	prompt.WriteString("9. **Business Logic Focus**: Prefer tools over raw shell commands for complex logic\n\n")
	prompt.WriteString("**Container Selection with Dependencies (CRITICAL):**\n")
	prompt.WriteString("- **AWS CLI tools**: amazon/aws-cli:latest + with_files for credentials:\n")
	prompt.WriteString("  ```json\n")
	prompt.WriteString("  \"with_files\": [{\n")
	prompt.WriteString("    \"source\": \"$HOME/.aws/credentials\",\n")
	prompt.WriteString("    \"destination\": \"/root/.aws/credentials\"\n")
	prompt.WriteString("  }]\n")
	prompt.WriteString("  ```\n")
	prompt.WriteString("- **Kubernetes tools**: kubiya/kubectl-light:latest + with_files for tokens:\n")
	prompt.WriteString("  ```json\n")
	prompt.WriteString("  \"with_files\": [{\n")
	prompt.WriteString("    \"source\": \"/var/run/secrets/kubernetes.io/serviceaccount/token\",\n")
	prompt.WriteString("    \"destination\": \"/tmp/kubernetes_context_token\"\n")
	prompt.WriteString("  }]\n")
	prompt.WriteString("  ```\n")
	prompt.WriteString("- **Python tools** → python_executor() with specific packages\n")
	prompt.WriteString("- **Security tools** → docker_executor() with security-focused containers\n")
	prompt.WriteString("- **Database tools** → containers with client libraries pre-installed\n\n")
	prompt.WriteString("**Tool Source Types and Management:**\n")
	prompt.WriteString("- **Git sources**: Tools from GitHub repositories\n")
	prompt.WriteString("  * Can have sync errors (check source.errors field)\n")
	prompt.WriteString("  * Require runner access to clone repositories\n")
	prompt.WriteString("  * Example: 'https://github.com/kubiya-solutions-engineering/cli-tools'\n")
	prompt.WriteString("- **Inline sources**: Custom tool groupings without git/URL\n")
	prompt.WriteString("  * type: 'inline', tools array with full tool specifications\n")
	prompt.WriteString("  * Perfect for forked tools, custom groupings, tool combinations\n")
	prompt.WriteString("  * No external dependencies, immediate availability\n")
	prompt.WriteString("- **Source discovery and creation**:\n")
	prompt.WriteString("  * Use POST /api/v1/sources/load?url=<github-url> to discover tools\n")
	prompt.WriteString("  * Use POST /api/v1/sources to create source packages\n")
	prompt.WriteString("  * Always check for discovery/sync errors in responses\n\n")
	prompt.WriteString("**Source Error Handling Examples:**\n")
	prompt.WriteString("```json\n")
	prompt.WriteString("\"errors\": [{\n")
	prompt.WriteString("  \"file\": \"aws_tools/tools/s3.py\",\n")
	prompt.WriteString("  \"type\": \"ImportError\", \n")
	prompt.WriteString("  \"error\": \"cannot import name 's3_list_buckets'\"\n")
	prompt.WriteString("}]\n")
	prompt.WriteString("```\n")
	prompt.WriteString("→ **Action**: Alert user, suggest source sync or tool fixing\n")

	messages := []mcp.PromptMessage{
		mcp.NewPromptMessage(mcp.RoleUser, mcp.NewTextContent(prompt.String())),
	}

	return mcp.NewGetPromptResult(fmt.Sprintf("Tool Execution Guide: %s", toolName), messages), nil
}

func (ps *ProductionServer) handleWorkflowGenerationPrompt(ctx context.Context, req mcp.GetPromptRequest) (*mcp.GetPromptResult, error) {
	args := req.Params.Arguments
	if args == nil {
		args = make(map[string]string)
	}

	taskDescription := ""
	if val, exists := args["task_description"]; exists {
		taskDescription = val
	}
	if taskDescription == "" {
		return nil, fmt.Errorf("task_description argument is required")
	}

	toolsNeeded := ""
	if val, exists := args["tools_needed"]; exists {
		toolsNeeded = val
	}
	complexity := "medium"
	if val, exists := args["complexity"]; exists {
		complexity = val
	}
	environment := "kubernetes"
	if val, exists := args["environment"]; exists {
		environment = val
	}

	var prompt strings.Builder
	prompt.WriteString("You are an expert in workflow automation using the Kubiya WASM DSL. ")
	prompt.WriteString("Generate a comprehensive workflow that follows best practices.\n\n")

	prompt.WriteString(fmt.Sprintf("**Task**: %s\n\n", taskDescription))

	if toolsNeeded != "" {
		prompt.WriteString(fmt.Sprintf("**Required Tools/Capabilities**: %s\n\n", toolsNeeded))
	}

	prompt.WriteString("Please create a workflow using the Kubiya SDK patterns that includes:\n\n")

	prompt.WriteString("1. **Workflow Structure** (based on SDK examples):\n")
	prompt.WriteString("   - Use `workflow(name).description(text)` pattern\n")
	prompt.WriteString("   - Chain methods for clean configuration\n")
	prompt.WriteString("   - Define steps with appropriate executors (shell_executor, python_executor)\n")
	prompt.WriteString("   - Example: `workflow('my-task').description('Task description').step('step1', 'command')`\n\n")

	prompt.WriteString("2. **Tool-Centric Step Implementation with Dependencies**:\n")
	prompt.WriteString("   - **Always prefer tool steps for complex business logic** instead of raw commands\n")
	prompt.WriteString("   - Use `search_tools` to find relevant tools and **check their dependencies**\n")
	prompt.WriteString("   - Use `execute_tool_from_source` for leveraging existing tool sources\n")
	prompt.WriteString("   - **CRITICAL: Check tool metadata for dependencies**:\n")
	prompt.WriteString("     * `with_files` - file mounting (AWS creds, k8s tokens, etc.)\n")
	prompt.WriteString("     * `with_volumes` - volume mounting requirements\n")
	prompt.WriteString("     * `env` - required environment variables\n")
	prompt.WriteString("     * `image` - specific container requirements\n")
	prompt.WriteString("     * `errors` - check for tool discovery/sync issues\n")
	prompt.WriteString("   - **Smart container/executor selection**:\n")
	prompt.WriteString("     * AWS tools → amazon/aws-cli:latest + AWS credential files\n")
	prompt.WriteString("     * Kubernetes tools → kubiya/kubectl-light:latest + k8s tokens\n")
	prompt.WriteString("     * Python tools → python_executor() with packages\n")
	prompt.WriteString("     * Custom tools → check tool.image specification\n")
	prompt.WriteString("   - Add environment variables with .env() method for tool configuration\n\n")

	prompt.WriteString("3. **Advanced Tool-Based Patterns**:\n")
	prompt.WriteString("   - **Tool Discovery Phase**: Always start with `search_tools` to find existing capabilities\n")
	prompt.WriteString("   - **Smart Container Selection**: Choose containers based on tool requirements (Python, Node, Docker, etc.)\n")
	prompt.WriteString("   - **AI-powered tool orchestration** with inline agents for decision-making\n")
	prompt.WriteString("   - **Streaming execution** with stream=True for real-time feedback\n")
	prompt.WriteString("   - **Multi-step tool pipelines** with parallel execution where tools allow\n")
	prompt.WriteString("   - **Dynamic tool selection** based on environment or conditions\n\n")

	prompt.WriteString("4. **Pre-Execution Validation and Runner Selection**:\n")
	prompt.WriteString("   - **Check runner health**: Use `check_runner_health` and `find_available_runner`\n")
	prompt.WriteString("   - **Validate secrets**: Use `list_secrets` to verify required secrets exist\n")
	prompt.WriteString("   - **Ask user for missing dependencies**: Present options for runners and missing secrets\n")
	prompt.WriteString("   - **Runner selection logic**: If multiple healthy runners, ask user to choose\n\n")
	prompt.WriteString("5. **Python-to-JSON Workflow Compilation**:\n")
	prompt.WriteString("   - **Always write Python code first** (easier to define than raw JSON)\n")
	prompt.WriteString("   - **Use WASM compilation tooling** to convert Python workflow to JSON\n")
	prompt.WriteString("   - **Execute the compiled JSON** with execute_workflow(compiled_json)\n")
	prompt.WriteString("   - **Pattern**: Validation → Python Definition → WASM Compile → JSON Execution\n\n")
	prompt.WriteString("6. **Execution Options**:\n")
	prompt.WriteString("   - Synchronous: execute_workflow(compiled_workflow_json, runner=selected_runner)\n")
	prompt.WriteString("   - Streaming: execute_workflow(compiled_workflow_json, stream=True, runner=selected_runner)\n")
	prompt.WriteString("   - Error handling and retry mechanisms\n\n")

	switch complexity {
	case "simple":
		prompt.WriteString("**Complexity Level**: Simple - Tool-focused linear workflow\n")
		prompt.WriteString("1. Start with `search_tools` to find relevant tools\n")
		prompt.WriteString("2. Use discovered tools instead of raw commands\n")
		prompt.WriteString("3. Choose appropriate container/executor for each tool\n")
	case "medium":
		prompt.WriteString("**Complexity Level**: Medium - Multi-tool orchestration\n")
		prompt.WriteString("1. Use `search_tools` to discover multiple relevant tools\n")
		prompt.WriteString("2. Smart executor selection based on tool requirements (Python, shell, docker)\n")
		prompt.WriteString("3. Tool chaining with proper data flow between tools\n")
		prompt.WriteString("4. Include parallel tool execution where appropriate\n")
	case "complex":
		prompt.WriteString("**Complexity Level**: Complex - Intelligent tool ecosystem\n")
		prompt.WriteString("1. **Discovery-first approach**: Use `search_tools` and `list_sources` for comprehensive tool mapping\n")
		prompt.WriteString("2. **AI-driven tool selection**: Use ai_agent() for dynamic tool choice based on conditions\n")
		prompt.WriteString("3. **Smart container orchestration**: Auto-select optimal containers per tool type\n")
		prompt.WriteString("4. **Real-time tool execution**: Streaming with execute_workflow(wf, stream=True)\n")
		prompt.WriteString("5. **Tool ecosystem integration**: Reference tool sources by UUID for reliability\n")
	}

	prompt.WriteString(fmt.Sprintf("**Target Environment**: %s\n\n", environment))

	prompt.WriteString("Provide the complete tool-centric workflow with:\n\n")
	prompt.WriteString("**CRITICAL: Tool-First Approach**\n")
	prompt.WriteString("1. **Discovery Phase**: Start with `search_tools` examples to find relevant tools\n")
	prompt.WriteString("2. **Tool Integration**: Show how to use discovered tools in workflow steps\n")
	prompt.WriteString("3. **Smart Container Selection**: Explain container/executor choice for each tool type\n")
	prompt.WriteString("4. **Reference Management**: Use tool source UUIDs for reliable tool access\n\n")
	prompt.WriteString("**Implementation Details**:\n")
	prompt.WriteString("- **Step 1**: Python SDK syntax with workflow() constructor and method chaining\n")
	prompt.WriteString("- **Step 2**: WASM compilation: `workflow_dsl_wasm(python_code)` → JSON output\n")
	prompt.WriteString("- **Step 3**: JSON execution: `execute_workflow(compiled_json, stream=True)`\n")
	prompt.WriteString("- Tool-aware executor selection in Python (python_executor, shell_executor)\n")
	prompt.WriteString("- MCP tool integration examples (search_tools, execute_tool_from_source)\n")
	prompt.WriteString("- Complete compilation and execution workflow examples\n\n")

	prompt.WriteString("**MANDATORY WORKFLOW APPROACH WITH DEPENDENCY MANAGEMENT:**\n")
	prompt.WriteString("1. **Tool discovery with dependency analysis** - Use search_tools and examine metadata:\n")
	prompt.WriteString("   - Check `with_files` for file mounting requirements (AWS creds, k8s tokens)\n")
	prompt.WriteString("   - Check `with_volumes` for volume mounting needs\n")
	prompt.WriteString("   - Check `env` for required environment variables\n")
	prompt.WriteString("   - Check `image` for specific container requirements\n")
	prompt.WriteString("   - Check `errors` field for tool discovery/sync issues\n")
	prompt.WriteString("2. **Source type awareness** - Understand tool source types:\n")
	prompt.WriteString("   - **Git sources**: Repository-backed tools (check sync status)\n")
	prompt.WriteString("   - **Inline sources**: Custom tool groupings (check tool definitions)\n")
	prompt.WriteString("   - **Error handling**: Alert user to source sync errors\n")
	prompt.WriteString("3. **Environment validation based on tool deps** - Check compatibility:\n")
	prompt.WriteString("   - Validate runners can handle tool requirements (k8s access, AWS perms)\n")
	prompt.WriteString("   - Check tool-required secrets exist in `list_secrets`\n")
	prompt.WriteString("   - ASK USER to choose runners and provide missing dependencies\n")
	prompt.WriteString("4. **Dependency-aware Python workflow** - Include tool configurations:\n")
	prompt.WriteString("   - Pass discovered `image`, `with_files`, `with_volumes`, `env` to tools\n")
	prompt.WriteString("   - Include all tool and workflow secrets\n")
	prompt.WriteString("5. **Compile and execute** - WASM compile → validated execution\n")
	prompt.WriteString("6. **Smart container orchestration** - Match containers to tool requirements\n")
	prompt.WriteString("7. **Reference by UUID** - Use source UUIDs for reliable tool access\n")
	prompt.WriteString("8. **Comprehensive validation** - Use all MCP validation and execution tools\n\n")
	prompt.WriteString("Reference the Kubiya SDK documentation at https://docs.kubiya.ai/sdk/examples for patterns and examples.")

	messages := []mcp.PromptMessage{
		mcp.NewPromptMessage(mcp.RoleUser, mcp.NewTextContent(prompt.String())),
	}

	return mcp.NewGetPromptResult("Workflow Generation with WASM DSL", messages), nil
}

func (ps *ProductionServer) handleSourceExplorationPrompt(ctx context.Context, req mcp.GetPromptRequest) (*mcp.GetPromptResult, error) {
	args := req.Params.Arguments
	if args == nil {
		args = make(map[string]string)
	}
	
	sourceUUID := ""
	if val, exists := args["source_uuid"]; exists {
		sourceUUID = val
	}
	toolType := ""
	if val, exists := args["tool_type"]; exists {
		toolType = val
	}
	useCase := ""
	if val, exists := args["use_case"]; exists {
		useCase = val
	}

	var prompt strings.Builder
	prompt.WriteString("You are guiding someone through exploring Kubiya sources and tools. ")
	prompt.WriteString("Help them discover and understand available capabilities.\n\n")

	if sourceUUID != "" {
		prompt.WriteString(fmt.Sprintf("**Focus Source**: %s\n", sourceUUID))
		prompt.WriteString("Start by using `list_sources` to get metadata about this specific source, ")
		prompt.WriteString("then use `search_tools` to explore its tools.\n\n")
	} else {
		prompt.WriteString("**General Exploration**:\n")
		prompt.WriteString("1. Use `list_sources` to see all available sources with pagination\n")
		prompt.WriteString("2. Use `search_tools` to find tools matching your criteria\n")
		prompt.WriteString("3. Examine tool metadata including arguments and requirements\n\n")
	}

	if toolType != "" {
		prompt.WriteString(fmt.Sprintf("**Tool Type Focus**: %s\n", toolType))
		prompt.WriteString("Filter your search to focus on tools of this type.\n\n")
	}

	if useCase != "" {
		prompt.WriteString(fmt.Sprintf("**Use Case**: %s\n\n", useCase))
	}

	prompt.WriteString("Provide guidance on:\n")
	prompt.WriteString("1. **Discovery Process**: How to systematically explore available tools\n")
	prompt.WriteString("2. **Tool Selection**: Criteria for choosing the right tools\n")
	prompt.WriteString("3. **Source Quality**: How to assess source reliability and maintenance\n")
	prompt.WriteString("4. **Integration Patterns**: How tools work together\n")
	prompt.WriteString("5. **Getting Started**: Next steps for hands-on experimentation\n\n")

	prompt.WriteString("Use the following MCP tools to demonstrate the exploration process:\n")
	prompt.WriteString("- `list_sources` - Get overview of available sources\n")
	prompt.WriteString("- `search_tools` - Find tools by name, description, or type\n")
	prompt.WriteString("- Show practical examples of command usage\n")

	messages := []mcp.PromptMessage{
		mcp.NewPromptMessage(mcp.RoleUser, mcp.NewTextContent(prompt.String())),
	}

	return mcp.NewGetPromptResult("Source and Tool Exploration Guide", messages), nil
}

func (ps *ProductionServer) handleBestPracticesPrompt(ctx context.Context, req mcp.GetPromptRequest) (*mcp.GetPromptResult, error) {
	args := req.Params.Arguments
	if args == nil {
		args = make(map[string]string)
	}
	
	topic := "general"
	if val, exists := args["topic"]; exists {
		topic = val
	}
	role := "developer"
	if val, exists := args["role"]; exists {
		role = val
	}

	var systemMessage string
	switch role {
	case "developer":
		systemMessage = "You are an expert developer advocate for the Kubiya platform with deep knowledge of development best practices, tool integration, and workflow automation."
	case "devops":
		systemMessage = "You are a DevOps expert specializing in the Kubiya platform with extensive experience in infrastructure automation, CI/CD, and operational best practices."
	case "admin":
		systemMessage = "You are a Kubiya platform administrator with expertise in security, compliance, governance, and enterprise-scale deployments."
	default:
		systemMessage = "You are a Kubiya platform expert with comprehensive knowledge across all aspects of the platform."
	}

	var prompt strings.Builder
	prompt.WriteString(fmt.Sprintf("Provide comprehensive best practices guidance for: **%s**\n\n", topic))

	switch topic {
	case "tools":
		prompt.WriteString("Cover these tool-related best practices:\n")
		prompt.WriteString("1. **Tool Selection**: Choosing the right tools for your use case\n")
		prompt.WriteString("2. **Tool Configuration**: Proper setup and parameterization\n")
		prompt.WriteString("3. **Source Management**: Organizing and maintaining tool sources\n")
		prompt.WriteString("4. **Testing**: Validating tool behavior in different environments\n")
		prompt.WriteString("5. **Integration**: Combining tools effectively\n")
		prompt.WriteString("6. **Performance**: Optimizing tool execution\n")

	case "workflows":
		prompt.WriteString("Cover these workflow best practices:\n")
		prompt.WriteString("1. **Design Patterns**: Common workflow architectures\n")
		prompt.WriteString("2. **Error Handling**: Robust error management strategies\n")
		prompt.WriteString("3. **State Management**: Handling workflow state and data\n")
		prompt.WriteString("4. **Parallelization**: When and how to run steps in parallel\n")
		prompt.WriteString("5. **Testing**: Workflow validation and testing approaches\n")
		prompt.WriteString("6. **Monitoring**: Observability and debugging\n")

	case "security":
		prompt.WriteString("Cover these security best practices:\n")
		prompt.WriteString("1. **Secret Management**: Secure handling of credentials\n")
		prompt.WriteString("2. **Access Control**: RBAC and permission management\n")
		prompt.WriteString("3. **Network Security**: Secure communications\n")
		prompt.WriteString("4. **Compliance**: Meeting regulatory requirements\n")
		prompt.WriteString("5. **Audit Logging**: Comprehensive activity tracking\n")
		prompt.WriteString("6. **Vulnerability Management**: Keeping tools and dependencies secure\n")

	default:
		prompt.WriteString("Cover these general best practices:\n")
		prompt.WriteString("1. **Platform Architecture**: Understanding Kubiya's core concepts\n")
		prompt.WriteString("2. **Development Workflow**: Efficient development practices\n")
		prompt.WriteString("3. **Testing Strategy**: Comprehensive testing approaches\n")
		prompt.WriteString("4. **Deployment**: Safe and reliable deployment practices\n")
		prompt.WriteString("5. **Monitoring**: Observability and alerting\n")
		prompt.WriteString("6. **Troubleshooting**: Common issues and solutions\n")
	}

	prompt.WriteString("\nFor each practice area, provide:\n")
	prompt.WriteString("- **Rationale**: Why this practice matters\n")
	prompt.WriteString("- **Implementation**: How to implement it\n")
	prompt.WriteString("- **Examples**: Concrete examples using Kubiya MCP tools\n")
	prompt.WriteString("- **Pitfalls**: Common mistakes to avoid\n")
	prompt.WriteString("- **Tools**: Specific MCP tools that help implement the practice\n")

	messages := []mcp.PromptMessage{
		mcp.NewPromptMessage(mcp.RoleAssistant, mcp.NewTextContent(systemMessage)),
		mcp.NewPromptMessage(mcp.RoleUser, mcp.NewTextContent(prompt.String())),
	}

	return mcp.NewGetPromptResult(fmt.Sprintf("Kubiya Best Practices: %s", strings.Title(topic)), messages), nil
}

func (ps *ProductionServer) handleTroubleshootingPrompt(ctx context.Context, req mcp.GetPromptRequest) (*mcp.GetPromptResult, error) {
	args := req.Params.Arguments
	if args == nil {
		args = make(map[string]string)
	}

	issueDescription := ""
	if val, exists := args["issue_description"]; exists {
		issueDescription = val
	}
	if issueDescription == "" {
		return nil, fmt.Errorf("issue_description argument is required")
	}

	toolName := ""
	if val, exists := args["tool_name"]; exists {
		toolName = val
	}
	errorMessage := ""
	if val, exists := args["error_message"]; exists {
		errorMessage = val
	}

	var prompt strings.Builder
	prompt.WriteString("You are a Kubiya platform expert specializing in troubleshooting tool execution and workflow issues. ")
	prompt.WriteString("Provide systematic diagnostic and resolution guidance.\n\n")

	prompt.WriteString(fmt.Sprintf("**Issue Description**: %s\n\n", issueDescription))

	if toolName != "" {
		prompt.WriteString(fmt.Sprintf("**Tool Involved**: %s\n", toolName))
		prompt.WriteString("Use `search_tools` to get current information about this tool.\n\n")
	}

	if errorMessage != "" {
		prompt.WriteString(fmt.Sprintf("**Error Message**: ```\n%s\n```\n\n", errorMessage))
	}

	prompt.WriteString("Provide a structured troubleshooting approach:\n\n")

	prompt.WriteString("1. **Initial Analysis**:\n")
	prompt.WriteString("   - Identify the most likely root causes\n")
	prompt.WriteString("   - Categorize the issue (configuration, permissions, connectivity, etc.)\n\n")

	prompt.WriteString("2. **Diagnostic Steps**:\n")
	prompt.WriteString("   - Use MCP tools to gather relevant information:\n")
	prompt.WriteString("     * `list_runners` - Check runner availability and health\n")
	prompt.WriteString("     * `list_sources` - Verify source accessibility\n")
	prompt.WriteString("     * `search_tools` - Confirm tool availability and parameters\n")
	prompt.WriteString("   - Validate prerequisites and dependencies\n\n")

	prompt.WriteString("3. **Common Solutions**:\n")
	prompt.WriteString("   - Step-by-step resolution procedures\n")
	prompt.WriteString("   - Configuration adjustments\n")
	prompt.WriteString("   - Alternative approaches or workarounds\n\n")

	prompt.WriteString("4. **Prevention**:\n")
	prompt.WriteString("   - How to avoid this issue in the future\n")
	prompt.WriteString("   - Monitoring and alerting recommendations\n")
	prompt.WriteString("   - Best practices to follow\n\n")

	prompt.WriteString("5. **Next Steps**:\n")
	prompt.WriteString("   - If the issue persists, what to try next\n")
	prompt.WriteString("   - How to gather additional diagnostic information\n")
	prompt.WriteString("   - When to escalate to support\n\n")

	prompt.WriteString("Format your response with clear action items and use Kubiya MCP tools to demonstrate the troubleshooting process.")

	messages := []mcp.PromptMessage{
		mcp.NewPromptMessage(mcp.RoleUser, mcp.NewTextContent(prompt.String())),
	}

	return mcp.NewGetPromptResult("Troubleshooting Guidance", messages), nil
}

func (ps *ProductionServer) handleWorkflowExamplesPrompt(ctx context.Context, req mcp.GetPromptRequest) (*mcp.GetPromptResult, error) {
	args := req.Params.Arguments
	if args == nil {
		args = make(map[string]string)
	}
	
	pattern := "simple"
	if val, exists := args["pattern"]; exists {
		pattern = val
	}
	useCase := ""
	if val, exists := args["use_case"]; exists {
		useCase = val
	}

	var prompt strings.Builder
	prompt.WriteString("You are an expert in Kubiya workflow automation providing comprehensive examples using WASM DSL. ")
	prompt.WriteString("Provide complete, production-ready workflow examples with best practices.\n\n")

	prompt.WriteString("Reference the Kubiya SDK documentation: https://docs.kubiya.ai/sdk/examples\n\n")

	switch pattern {
	case "simple":
		prompt.WriteString("**Pattern: Complete Validation and Execution Workflow**\n\n")
		prompt.WriteString("Complete workflow: Discovery → Validation → Python Definition → WASM Compilation → Execution\n\n")
		prompt.WriteString("**Step 1: Tool Discovery and Dependency Analysis**\n")
		prompt.WriteString("```python\n")
		prompt.WriteString("# Use MCP tools to discover available capabilities\n")
		prompt.WriteString("tools_result = search_tools(query='kubectl', page=1, page_size=10)\n")
		prompt.WriteString("# Result: Found kubectl tools in source uuid 'abc-123-def'\n")
		prompt.WriteString("\n")
		prompt.WriteString("# CRITICAL: Examine tool dependencies from search results\n")
		prompt.WriteString("kubectl_tool = tools_result[0]  # First kubectl tool found\n")
		prompt.WriteString("print(f'Tool image: {kubectl_tool[\"image\"]}')  # kubiya/kubectl-light:latest\n")
		prompt.WriteString("print(f'Required files: {kubectl_tool[\"with_files\"]}')  # k8s tokens/certs\n")
		prompt.WriteString("print(f'Environment vars: {kubectl_tool[\"env\"]}')  # Any required env vars\n")
		prompt.WriteString("print(f'Errors: {kubectl_tool.get(\"errors\", [])}')  # Check for discovery issues\n")
		prompt.WriteString("\n")
		prompt.WriteString("# Also check source metadata for additional context\n")
		prompt.WriteString("source_metadata = list_sources(source_uuid='abc-123-def')\n")
		prompt.WriteString("if source_metadata.get('errors'):\n")
		prompt.WriteString("    print('WARNING: Source has sync errors:', source_metadata['errors'])\n")
		prompt.WriteString("```\n\n")
		prompt.WriteString("**Step 2: Environment Validation Based on Tool Dependencies**\n")
		prompt.WriteString("```python\n")
		prompt.WriteString("# Check available runners\n")
		prompt.WriteString("runners = list_runners()\n")
		prompt.WriteString("healthy_runners = check_runner_health()  # Returns list of healthy runners\n")
		prompt.WriteString("\n")
		prompt.WriteString("# Check tool-specific requirements\n")
		prompt.WriteString("secrets = list_secrets()\n")
		prompt.WriteString("# Extract required secrets from tool environment variables\n")
		prompt.WriteString("tool_env_vars = kubectl_tool.get('env', [])\n")
		prompt.WriteString("required_secrets = tool_env_vars + ['KUBECONFIG']  # Add any workflow-specific secrets\n")
		prompt.WriteString("missing_secrets = [s for s in required_secrets if s not in secrets]\n")
		prompt.WriteString("\n")
		prompt.WriteString("# Validate runners can handle tool requirements\n")
		prompt.WriteString("# For k8s tools, prefer runners with cluster access\n")
		prompt.WriteString("k8s_capable_runners = [r for r in healthy_runners \n")
		prompt.WriteString("                      if 'kubernetes' in r.get('capabilities', [])]\n")
		prompt.WriteString("\n")
		prompt.WriteString("# ASK USER: Choose runner if multiple healthy options\n")
		prompt.WriteString("if len(healthy_runners) > 1:\n")
		prompt.WriteString("    print('Multiple healthy runners available:')\n")
		prompt.WriteString("    for i, runner in enumerate(healthy_runners):\n")
		prompt.WriteString("        print(f'{i+1}. {runner[\"name\"]} - {runner[\"status\"]}')\n")
		prompt.WriteString("    selected_runner = input('Choose runner (1-N): ')\n")
		prompt.WriteString("elif len(healthy_runners) == 0:\n")
		prompt.WriteString("    raise Exception('No healthy runners available!')\n")
		prompt.WriteString("else:\n")
		prompt.WriteString("    selected_runner = healthy_runners[0]['name']\n")
		prompt.WriteString("\n")
		prompt.WriteString("# ASK USER: Handle missing secrets\n")
		prompt.WriteString("if missing_secrets:\n")
		prompt.WriteString("    print(f'Missing required secrets: {missing_secrets}')\n")
		prompt.WriteString("    print('Please create these secrets before proceeding:')\n")
		prompt.WriteString("    for secret in missing_secrets:\n")
		prompt.WriteString("        print(f'  - {secret}')\n")
		prompt.WriteString("    proceed = input('Have you created the secrets? (y/n): ')\n")
		prompt.WriteString("    if proceed.lower() != 'y':\n")
		prompt.WriteString("        raise Exception('Required secrets not available')\n")
		prompt.WriteString("```\n\n")
		prompt.WriteString("**Step 3: Python Workflow Definition with Tool Dependencies**\n")
		prompt.WriteString("```python\n")
		prompt.WriteString("python_workflow = f'''\n")
		prompt.WriteString("wf = (\n")
		prompt.WriteString("    workflow('k8s-pod-check')\n")
		prompt.WriteString("    .description('Check pod status using discovered kubectl tool with dependencies')\n")
		prompt.WriteString("    .step('check_pods', \n")
		prompt.WriteString("          execute_tool_from_source(\n")
		prompt.WriteString("              source_uuid='abc-123-def', \n")
		prompt.WriteString("              tool_name='kubectl',\n")
		prompt.WriteString("              # Tool-specific configuration based on discovered metadata\n")
		prompt.WriteString("              image='{kubectl_tool[\"image\"]}',  # kubiya/kubectl-light:latest\n")
		prompt.WriteString("              with_files={kubectl_tool[\"with_files\"]},  # k8s token/cert files\n")
		prompt.WriteString("              env={kubectl_tool.get(\"env\", [])},  # Tool env requirements\n")
		prompt.WriteString("              args={{'command': 'get pods -n production'}}\n")
		prompt.WriteString("          ))\n")
		prompt.WriteString("    # Workflow-level environment variables\n")
		prompt.WriteString("    .env('NAMESPACE', 'production')\n")
		prompt.WriteString("    # Include all required secrets (tool + workflow)\n")
		prompt.WriteString("    .secrets({required_secrets})\n")
		prompt.WriteString(")\n")
		prompt.WriteString("'''\n")
		prompt.WriteString("```\n\n")
		prompt.WriteString("**Step 4: WASM Compilation**\n")
		prompt.WriteString("```python\n")
		prompt.WriteString("# Compile Python workflow to JSON using WASM tooling\n")
		prompt.WriteString("compiled_json = workflow_dsl_wasm(python_workflow)\n")
		prompt.WriteString("```\n\n")
		prompt.WriteString("**Step 5: Validated Execution**\n")
		prompt.WriteString("```python\n")
		prompt.WriteString("# Execute on the selected healthy runner with validated secrets\n")
		prompt.WriteString("result = execute_workflow(\n")
		prompt.WriteString("    compiled_json, \n")
		prompt.WriteString("    stream=True,\n")
		prompt.WriteString("    runner=selected_runner,\n")
		prompt.WriteString("    secrets=required_secrets\n")
		prompt.WriteString(")\n")
		prompt.WriteString("```\n\n")
		prompt.WriteString("**Why This Dependency-Aware Validation Approach:**\n")
		prompt.WriteString("1. **Prevents tool-specific failures** - Check file mounts, volumes, env vars before execution\n")
		prompt.WriteString("2. **Container compatibility** - Ensure runners can handle tool container requirements\n")
		prompt.WriteString("3. **Source error awareness** - Alert to sync/discovery issues before workflow creation\n")
		prompt.WriteString("4. **Environment-specific validation** - K8s tools need k8s-capable runners, AWS tools need AWS access\n")
		prompt.WriteString("5. **Complete dependency mapping** - Include all tool files, volumes, secrets in workflow\n")
		prompt.WriteString("6. **User control over complex setups** - Let users choose runners for tool-specific workloads\n")
		prompt.WriteString("7. **Python workflow with full context** - Include discovered tool configurations\n")
		prompt.WriteString("8. **WASM compilation with dependencies** - Compile workflows with complete tool specifications\n")
		prompt.WriteString("9. **Source type flexibility** - Support git, inline, and custom tool sources\n")
		prompt.WriteString("10. **Production-ready reliability** - Handle all edge cases and tool requirements\n\n")
		
		prompt.WriteString("Include a complete example for: ")
		if useCase != "" {
			prompt.WriteString(fmt.Sprintf("%s\n\n", useCase))
		} else {
			prompt.WriteString("Kubernetes pod status check and restart\n\n")
		}

	case "conditional":
		prompt.WriteString("**Pattern: Conditional Logic Workflow**\n\n")
		prompt.WriteString("Create a workflow with conditional branching showing:\n")
		prompt.WriteString("1. Conditional step execution based on previous results\n")
		prompt.WriteString("2. Multiple execution paths\n")
		prompt.WriteString("3. Result evaluation and decision making\n")
		prompt.WriteString("4. Default fallback behaviors\n")
		prompt.WriteString("5. State management across conditions\n\n")
		
		prompt.WriteString("Include examples of:\n")
		prompt.WriteString("- Environment-based execution paths\n")
		prompt.WriteString("- Health check results triggering different actions\n")
		prompt.WriteString("- Resource availability checks\n\n")

	case "parallel":
		prompt.WriteString("**Pattern: Multi-Tool Parallel with Complete Validation**\n\n")
		prompt.WriteString("Complete parallel workflow: Discovery → Runner/Secret Validation → Python → WASM → Execution\n\n")
		prompt.WriteString("**Step 1: Multi-Tool Discovery**\n")
		prompt.WriteString("```python\n")
		prompt.WriteString("# Discover tools for different parallel tasks\n")
		prompt.WriteString("search_tools(query='pytest', tool_type='python')  # → source: 'py-tools-uuid'\n")
		prompt.WriteString("search_tools(query='security', tool_type='docker')  # → source: 'sec-tools-uuid'\n")
		prompt.WriteString("search_tools(query='lint', tool_type='shell')  # → source: 'lint-tools-uuid'\n")
		prompt.WriteString("```\n\n")
		prompt.WriteString("**Step 2: Parallel Execution Environment Validation**\n")
		prompt.WriteString("```python\n")
		prompt.WriteString("# Check runners - parallel workflows need robust runners\n")
		prompt.WriteString("healthy_runners = check_runner_health()\n")
		prompt.WriteString("high_resource_runners = [r for r in healthy_runners if r['cpu'] > 4 and r['memory'] > 8]\n")
		prompt.WriteString("\n")
		prompt.WriteString("# ASK USER: Choose runner for parallel workload\n")
		prompt.WriteString("if len(high_resource_runners) > 1:\n")
		prompt.WriteString("    print('Multiple high-resource runners available for parallel execution:')\n")
		prompt.WriteString("    for i, runner in enumerate(high_resource_runners):\n")
		prompt.WriteString("        print(f'{i+1}. {runner[\"name\"]} - CPU: {runner[\"cpu\"]}, Memory: {runner[\"memory\"]}GB')\n")
		prompt.WriteString("    selected_runner = input('Choose runner for parallel workload (1-N): ')\n")
		prompt.WriteString("elif len(high_resource_runners) == 0:\n")
		prompt.WriteString("    print('WARNING: No high-resource runners found. Parallel execution may be slow.')\n")
		prompt.WriteString("    selected_runner = healthy_runners[0]['name'] if healthy_runners else None\n")
		prompt.WriteString("else:\n")
		prompt.WriteString("    selected_runner = high_resource_runners[0]['name']\n")
		prompt.WriteString("\n")
		prompt.WriteString("# Check secrets for all parallel tools\n")
		prompt.WriteString("required_secrets = ['GITHUB_TOKEN', 'SONAR_TOKEN', 'DOCKER_REGISTRY_TOKEN']\n")
		prompt.WriteString("secrets = list_secrets()\n")
		prompt.WriteString("missing = [s for s in required_secrets if s not in secrets]\n")
		prompt.WriteString("if missing:\n")
		prompt.WriteString("    print(f'Missing secrets for parallel tools: {missing}')\n")
		prompt.WriteString("    print('Please create these secrets for full parallel functionality')\n")
		prompt.WriteString("```\n\n")
		prompt.WriteString("**Step 3: Python Parallel Workflow Definition**\n")
		prompt.WriteString("```python\n")
		prompt.WriteString("parallel_workflow = '''\n")
		prompt.WriteString("wf = (\n")
		prompt.WriteString("    workflow('smart-parallel-pipeline')\n")
		prompt.WriteString("    .description('Parallel execution with discovered tools and validation')\n")
		prompt.WriteString("    .step('parallel_validation', parallel_executor([\n")
		prompt.WriteString("        # Python tool with smart container\n")
		prompt.WriteString("        execute_tool_from_source(source_uuid='py-tools-uuid', \n")
		prompt.WriteString("                                 tool_name='pytest-runner'),\n")
		prompt.WriteString("        # Security tool with docker container  \n")
		prompt.WriteString("        execute_tool_from_source(source_uuid='sec-tools-uuid',\n")
		prompt.WriteString("                                 tool_name='security-scanner'),\n")
		prompt.WriteString("        # Linting tool with shell executor\n")
		prompt.WriteString("        execute_tool_from_source(source_uuid='lint-tools-uuid',\n")
		prompt.WriteString("                                 tool_name='eslint-checker')\n")
		prompt.WriteString("    ]))\n")
		prompt.WriteString("    # Include all required secrets for parallel tools\n")
		prompt.WriteString("    .secrets(required_secrets)\n")
		prompt.WriteString(")\n")
		prompt.WriteString("'''\n")
		prompt.WriteString("```\n\n")
		prompt.WriteString("**Step 4: Compilation and Validated Execution**\n")
		prompt.WriteString("```python\n")
		prompt.WriteString("# Compile to JSON and execute on validated high-resource runner\n")
		prompt.WriteString("compiled_json = workflow_dsl_wasm(parallel_workflow)\n")
		prompt.WriteString("result = execute_workflow(\n")
		prompt.WriteString("    compiled_json, \n")
		prompt.WriteString("    stream=True,\n")
		prompt.WriteString("    runner=selected_runner,  # High-resource runner for parallel execution\n")
		prompt.WriteString("    secrets=required_secrets\n")
		prompt.WriteString(")\n")
		prompt.WriteString("```\n\n")
		prompt.WriteString("**Smart Container Selection by Tool Type:**\n")
		prompt.WriteString("1. Python tools → Containers with pytest, coverage pre-installed\n")
		prompt.WriteString("2. Security tools → Security-focused containers with scanning tools\n")
		prompt.WriteString("3. Linting tools → Node/shell containers with linting packages\n")
		prompt.WriteString("4. Database tools → Containers with client libraries\n\n")

	case "error-handling":
		prompt.WriteString("**Pattern: Comprehensive Error Handling**\n\n")
		prompt.WriteString("Create a robust workflow with error handling showing:\n")
		prompt.WriteString("1. Try-catch blocks for error containment\n")
		prompt.WriteString("2. Retry mechanisms with exponential backoff\n")
		prompt.WriteString("3. Rollback procedures for failed operations\n")
		prompt.WriteString("4. Error notification and alerting\n")
		prompt.WriteString("5. Graceful degradation strategies\n\n")
		
		prompt.WriteString("Include examples of:\n")
		prompt.WriteString("- Database migration with rollback\n")
		prompt.WriteString("- Service deployment with health checks\n")
		prompt.WriteString("- Data processing with validation\n\n")

	case "ci-cd":
		prompt.WriteString("**Pattern: CI/CD Pipeline Workflow**\n\n")
		prompt.WriteString("Create a comprehensive CI/CD workflow showing:\n")
		prompt.WriteString("1. Source code checkout and validation\n")
		prompt.WriteString("2. Build and test automation\n")
		prompt.WriteString("3. Security scanning and compliance checks\n")
		prompt.WriteString("4. Multi-environment deployment strategy\n")
		prompt.WriteString("5. Monitoring and rollback capabilities\n\n")
		
		prompt.WriteString("Include integration with:\n")
		prompt.WriteString("- Git repositories and webhooks\n")
		prompt.WriteString("- Container registries\n")
		prompt.WriteString("- Kubernetes clusters\n")
		prompt.WriteString("- Notification systems\n\n")

	case "infrastructure":
		prompt.WriteString("**Pattern: Infrastructure Management Workflow**\n\n")
		prompt.WriteString("Create an infrastructure automation workflow showing:\n")
		prompt.WriteString("1. Infrastructure provisioning and configuration\n")
		prompt.WriteString("2. Resource lifecycle management\n")
		prompt.WriteString("3. Compliance and security validation\n")
		prompt.WriteString("4. Cost optimization and resource cleanup\n")
		prompt.WriteString("5. Disaster recovery procedures\n\n")
		
		prompt.WriteString("Include examples for:\n")
		prompt.WriteString("- Cloud resource provisioning (AWS, Azure, GCP)\n")
		prompt.WriteString("- Kubernetes cluster management\n")
		prompt.WriteString("- Infrastructure compliance scanning\n")
		prompt.WriteString("- Resource tagging and cost tracking\n\n")

	default:
		prompt.WriteString(fmt.Sprintf("**Pattern: %s**\n\n", strings.Title(pattern)))
		prompt.WriteString("Create a workflow example for the specified pattern.\n\n")
	}

	prompt.WriteString("For each example, provide:\n\n")
	prompt.WriteString("1. **Complete YAML Workflow**:\n")
	prompt.WriteString("   ```yaml\n")
	prompt.WriteString("   # Include full workflow definition\n")
	prompt.WriteString("   ```\n\n")

	prompt.WriteString("2. **Step-by-Step Explanation**:\n")
	prompt.WriteString("   - Purpose and logic of each step\n")
	prompt.WriteString("   - Parameter explanations\n")
	prompt.WriteString("   - Integration points and dependencies\n\n")

	prompt.WriteString("3. **Best Practices Demonstrated**:\n")
	prompt.WriteString("   - Security considerations (secrets, permissions)\n")
	prompt.WriteString("   - Error handling and resilience\n")
	prompt.WriteString("   - Performance optimization\n")
	prompt.WriteString("   - Maintainability and readability\n\n")

	prompt.WriteString("4. **Usage Instructions**:\n")
	prompt.WriteString("   - How to deploy and execute the workflow\n")
	prompt.WriteString("   - Required environment setup\n")
	prompt.WriteString("   - Monitoring and debugging tips\n\n")

	prompt.WriteString("5. **Variations and Extensions**:\n")
	prompt.WriteString("   - How to adapt for different environments\n")
	prompt.WriteString("   - Additional features that could be added\n")
	prompt.WriteString("   - Integration with other workflows\n\n")

	if useCase != "" {
		prompt.WriteString(fmt.Sprintf("**Specific Use Case**: Tailor the examples for: %s\n\n", useCase))
	}

	prompt.WriteString("Ensure all examples follow Kubiya WASM DSL syntax and include proper source UUIDs, tool references, and environment-specific configurations.")

	messages := []mcp.PromptMessage{
		mcp.NewPromptMessage(mcp.RoleUser, mcp.NewTextContent(prompt.String())),
	}

	return mcp.NewGetPromptResult(fmt.Sprintf("Workflow Examples: %s Pattern", strings.Title(pattern)), messages), nil
}

// Helper function for safe string argument extraction
func getStringArg(args map[string]interface{}, key, defaultValue string) string {
	if val, exists := args[key]; exists {
		if str, ok := val.(string); ok {
			return str
		}
	}
	return defaultValue
}

// getAllTools returns all available tools (filtered by context)
func (ps *ProductionServer) getAllTools() []mcp.Tool {
	// Define all available tools
	allTools := []mcp.Tool{
		mcp.NewTool("execute_tool",
			mcp.WithDescription("Execute a Kubiya tool"),
			mcp.WithObject("tool_def", mcp.Description("Tool definition object (required if tool_url and source_uuid not provided)")),
			mcp.WithString("tool_url", mcp.Description("URL to load tool definition from")),
			mcp.WithString("source_uuid", mcp.Description("Source UUID to load tool from")),
			mcp.WithString("tool_name", mcp.Description("Tool name (required if using source_uuid)")),
			mcp.WithString("runner", mcp.Description("Runner to use (default: auto)")),
			mcp.WithNumber("timeout", mcp.Description("Timeout in seconds (default: 300)")),
			mcp.WithArray("integrations", mcp.Description("Integration templates to apply (e.g. kubernetes/incluster, aws/cli)")),
			mcp.WithObject("args", mcp.Description("Tool arguments override")),
			mcp.WithArray("env", mcp.Description("Environment variables override")),
			mcp.WithArray("with_files", mcp.Description("File mappings override")),
			mcp.WithArray("with_volumes", mcp.Description("Volume mappings override")),
			mcp.WithArray("with_services", mcp.Description("Service dependencies override")),
		),
		mcp.NewTool("list_runners",
			mcp.WithDescription("List all available runners and their status"),
		),
		mcp.NewTool("list_sources",
			mcp.WithDescription("List all available sources with metadata and pagination"),
			mcp.WithNumber("page", mcp.Description("Page number (default: 1)")),
			mcp.WithNumber("page_size", mcp.Description("Items per page (default: 20, max: 50)")),
		),
		mcp.NewTool("search_tools",
			mcp.WithDescription("Search for tools across all sources with pagination and filtering"),
			mcp.WithString("query", mcp.Required(), mcp.Description("Search query for tool names and descriptions")),
			mcp.WithNumber("page", mcp.Description("Page number (default: 1)")),
			mcp.WithNumber("page_size", mcp.Description("Items per page (default: 20, max: 50)")),
			mcp.WithString("source_uuid", mcp.Description("Filter by specific source UUID (optional)")),
			mcp.WithString("tool_type", mcp.Description("Filter by tool type (docker, python, bash, etc.)")),
			mcp.WithBoolean("long_running_only", mcp.Description("Show only long-running tools")),
		),
		mcp.NewTool("execute_tool_from_source",
			mcp.WithDescription("Execute a tool from a specific source (by UUID or URL)"),
			mcp.WithString("source", mcp.Required(), mcp.Description("Source UUID or URL")),
			mcp.WithString("tool_name", mcp.Required(), mcp.Description("Tool name to execute")),
			mcp.WithString("runner", mcp.Description("Runner to use (default: auto)")),
			mcp.WithObject("args", mcp.Description("Tool arguments")),
			mcp.WithArray("env", mcp.Description("Environment variables")),
			mcp.WithArray("with_files", mcp.Description("File mappings")),
			mcp.WithArray("with_volumes", mcp.Description("Volume mappings")),
			mcp.WithArray("with_services", mcp.Description("Service dependencies")),
			mcp.WithNumber("timeout", mcp.Description("Timeout in seconds (default: 300)")),
		),
		mcp.NewTool("search_kb",
			mcp.WithDescription("Search the knowledge base"),
			mcp.WithString("query", mcp.Required(), mcp.Description("Search query")),
			mcp.WithNumber("limit", mcp.Description("Maximum results to return")),
		),
	}

	// Add whitelisted tools
	if ps.config.WhitelistedTools != nil {
		for _, wt := range ps.config.WhitelistedTools {
			tool := mcp.NewTool(wt.Name, mcp.WithDescription(wt.Description))

			// Add any custom arguments
			if wt.Arguments != nil {
				// Create a new tool with all arguments
				toolOpts := []mcp.ToolOption{mcp.WithDescription(wt.Description)}

				for argName, argConfig := range wt.Arguments {
					argType, _ := argConfig["type"].(string)
					argDesc, _ := argConfig["description"].(string)
					argRequired, _ := argConfig["required"].(bool)

					switch argType {
					case "string":
						if argRequired {
							toolOpts = append(toolOpts, mcp.WithString(argName, mcp.Required(), mcp.Description(argDesc)))
						} else {
							toolOpts = append(toolOpts, mcp.WithString(argName, mcp.Description(argDesc)))
						}
					case "number":
						if argRequired {
							toolOpts = append(toolOpts, mcp.WithNumber(argName, mcp.Required(), mcp.Description(argDesc)))
						} else {
							toolOpts = append(toolOpts, mcp.WithNumber(argName, mcp.Description(argDesc)))
						}
					case "object":
						if argRequired {
							toolOpts = append(toolOpts, mcp.WithObject(argName, mcp.Required(), mcp.Description(argDesc)))
						} else {
							toolOpts = append(toolOpts, mcp.WithObject(argName, mcp.Description(argDesc)))
						}
					case "boolean":
						if argRequired {
							toolOpts = append(toolOpts, mcp.WithBoolean(argName, mcp.Required(), mcp.Description(argDesc)))
						} else {
							toolOpts = append(toolOpts, mcp.WithBoolean(argName, mcp.Description(argDesc)))
						}
					}
				}

				tool = mcp.NewTool(wt.Name, toolOpts...)
			}

			allTools = append(allTools, tool)
		}
	}

	// Apply filters based on context
	ctx := context.Background() // TODO: Get proper context
	return ps.toolFilter(ctx, allTools)
}

// Helper methods
func (ps *ProductionServer) getSessionIDFromContext(ctx context.Context) string {
	// This would need to be implemented based on how MCP-go provides session info
	// For now, return a placeholder
	return "default-session"
}

func (ps *ProductionServer) getSafeConfig() map[string]interface{} {
	// Return config with sensitive data removed
	return map[string]interface{}{
		"server_name":    ps.config.ServerName,
		"server_version": ps.config.ServerVersion,
		"environment":    os.Getenv("ENVIRONMENT"),
		"features":       ps.config.FeatureFlags,
		"rate_limit": map[string]interface{}{
			"requests_per_second": ps.config.RateLimit.RequestsPerSecond,
			"burst":               ps.config.RateLimit.Burst,
		},
	}
}

// Start starts the production server
func (ps *ProductionServer) Start(ctx context.Context) error {
	// Initialize Sentry transaction
	transaction := sentry.StartTransaction(ctx, "mcp_server_start")
	defer transaction.Finish()

	// Call start hook
	ps.hooks.OnServerStart(ctx)

	// Start the server
	ps.logger.Printf("Starting MCP server %s v%s", ps.config.ServerName, ps.config.ServerVersion)

	// The actual serving would be handled by the MCP framework
	return server.ServeStdio(ps.mcpServer)
}

// Shutdown gracefully shuts down the server
func (ps *ProductionServer) Shutdown(ctx context.Context) error {
	ps.logger.Println("Shutting down MCP server...")

	// Call stop hook
	ps.hooks.OnServerStop(ctx)

	// Clean up resources
	// The MCP server handles its own shutdown

	return nil
}

// NewPermissionMiddleware creates a new permission middleware
func NewPermissionMiddleware(toolPermissions map[string][]string) *PermissionMiddleware {
	// Convert map[string][]string to map[string]string for middleware compatibility
	// We'll use the first permission as the required one
	simplePermissions := make(map[string]string)
	for tool, perms := range toolPermissions {
		if len(perms) > 0 {
			simplePermissions[tool] = perms[0]
		}
	}
	return &PermissionMiddleware{
		toolPermissions: simplePermissions,
	}
}

// PermissionMiddleware checks tool permissions
type PermissionMiddleware struct {
	toolPermissions map[string]string // tool name -> required permission (simplified)
}

// Apply applies the permission middleware
func (m *PermissionMiddleware) Apply(next middleware.ToolHandler) middleware.ToolHandler {
	return func(ctx context.Context, req mcp.CallToolRequest) (*mcp.CallToolResult, error) {
		requiredPerm, needsCheck := m.toolPermissions[req.Params.Name]
		if !needsCheck {
			// No specific permission required
			return next(ctx, req)
		}

		sess, ok := session.SessionFromContext(ctx)
		if !ok {
			return mcp.NewToolResultError("Session required to check permissions"), nil
		}

		if !sess.HasPermission(requiredPerm) {
			sentryutil.CaptureMessage("Permission denied", sentry.LevelWarning, map[string]string{
				"user":                sess.UserID,
				"tool":                req.Params.Name,
				"required_permission": requiredPerm,
			})

			return mcp.NewToolResultError(fmt.Sprintf("Permission denied. Required permission: %s", requiredPerm)), nil
		}

		return next(ctx, req)
	}
}
