package mcp

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"os"
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
	// Implementation would go here
	return mcp.NewToolResultText("Sources listing not yet implemented"), nil
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
	// Add example prompt
	ps.mcpServer.AddPrompt(
		mcp.NewPrompt("debug_info",
			mcp.WithPromptDescription("Get debug information about the current session"),
			mcp.WithArgument("include_logs",
				mcp.ArgumentDescription("Include recent logs"),
			),
		),
		func(ctx context.Context, req mcp.GetPromptRequest) (*mcp.GetPromptResult, error) {
			// Add session to context
			sessionID := ps.getSessionIDFromContext(ctx)

			// Track timing
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

			// Call hooks
			duration := time.Since(start)
			ps.hooks.OnPromptCall(ctx, sessionID, "debug_info", duration, nil)

			messages := []mcp.PromptMessage{
				mcp.NewPromptMessage(mcp.RoleAssistant, mcp.NewTextContent(content.String())),
			}

			return mcp.NewGetPromptResult("Debug Information", messages), nil
		},
	)
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
			mcp.WithDescription("List all available sources"),
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
