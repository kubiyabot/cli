package mcp

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"
	"time"

	"github.com/kubiyabot/cli/internal/kubiya"
	"github.com/mark3labs/mcp-go/mcp"
)

// Platform tool handlers using existing client methods

func (s *Server) createRunnerHandler(ctx context.Context, request mcp.CallToolRequest) (*mcp.CallToolResult, error) {
	args := request.Params.Arguments
	name, ok := args["name"].(string)
	if !ok || name == "" {
		return mcp.NewToolResultError("name parameter is required"), nil
	}

	description, _ := args["description"].(string)

	// Create runner manifest - this is the supported operation
	manifest, err := s.client.CreateRunnerManifest(ctx, name)
	if err != nil {
		return mcp.NewToolResultError(fmt.Sprintf("Failed to create runner manifest: %v", err)), nil
	}

	data, err := json.MarshalIndent(manifest, "", "  ")
	if err != nil {
		return mcp.NewToolResultError(fmt.Sprintf("Failed to marshal runner manifest: %v", err)), nil
	}

	output := fmt.Sprintf("✅ Runner manifest created successfully for '%s':\n%s", name, string(data))
	if description != "" {
		output = fmt.Sprintf("✅ Runner manifest created for '%s' (%s):\n%s", name, description, string(data))
	}

	return mcp.NewToolResultText(output), nil
}

func (s *Server) deleteRunnerHandler(ctx context.Context, request mcp.CallToolRequest) (*mcp.CallToolResult, error) {
	args := request.Params.Arguments
	name, ok := args["name"].(string)
	if !ok || name == "" {
		return mcp.NewToolResultError("name parameter is required"), nil
	}

	// TODO: Add DeleteRunner method to client when API supports it
	return mcp.NewToolResultError(fmt.Sprintf("DeleteRunner API not yet available for: %s", name)), nil
}

func (s *Server) listRunnersHandler(ctx context.Context, request mcp.CallToolRequest) (*mcp.CallToolResult, error) {
	runners, err := s.client.ListRunners(ctx)
	if err != nil {
		return mcp.NewToolResultError(fmt.Sprintf("Failed to list runners: %v", err)), nil
	}

	data, err := json.MarshalIndent(runners, "", "  ")
	if err != nil {
		return mcp.NewToolResultError(fmt.Sprintf("Failed to marshal runners: %v", err)), nil
	}

	return mcp.NewToolResultText(string(data)), nil
}

func (s *Server) listAgentsHandler(ctx context.Context, request mcp.CallToolRequest) (*mcp.CallToolResult, error) {
	agents, err := s.client.ListAgents(ctx)
	if err != nil {
		return mcp.NewToolResultError(fmt.Sprintf("Failed to list agents: %v", err)), nil
	}

	data, err := json.MarshalIndent(agents, "", "  ")
	if err != nil {
		return mcp.NewToolResultError(fmt.Sprintf("Failed to marshal agents: %v", err)), nil
	}

	return mcp.NewToolResultText(string(data)), nil
}

func (s *Server) chatWithAgentHandler(ctx context.Context, request mcp.CallToolRequest) (*mcp.CallToolResult, error) {
	args := request.Params.Arguments
	agentName, ok := args["agent_name"].(string)
	if !ok || agentName == "" {
		return mcp.NewToolResultError("agent_name parameter is required"), nil
	}

	message, ok := args["message"].(string)
	if !ok || message == "" {
		return mcp.NewToolResultError("message parameter is required"), nil
	}

	// Use the existing chat functionality via SendMessage
	eventChan, err := s.client.SendMessage(ctx, agentName, message, "")
	if err != nil {
		return mcp.NewToolResultError(fmt.Sprintf("Failed to send message to agent: %v", err)), nil
	}

	var response strings.Builder
	response.WriteString(fmt.Sprintf("🤖 Chatting with %s:\n", agentName))
	response.WriteString(fmt.Sprintf("👤 You: %s\n\n", message))

	// Collect the response
	for msg := range eventChan {
		if msg.Type == "message" {
			response.WriteString(fmt.Sprintf("🤖 %s: %s\n", agentName, msg.Content))
		}
		if msg.Final {
			break
		}
	}

	return mcp.NewToolResultText(response.String()), nil
}

func (s *Server) listIntegrationsHandler(ctx context.Context, request mcp.CallToolRequest) (*mcp.CallToolResult, error) {
	integrations, err := s.client.ListIntegrations(ctx)
	if err != nil {
		return mcp.NewToolResultError(fmt.Sprintf("Failed to list integrations: %v", err)), nil
	}

	data, err := json.MarshalIndent(integrations, "", "  ")
	if err != nil {
		return mcp.NewToolResultError(fmt.Sprintf("Failed to marshal integrations: %v", err)), nil
	}

	return mcp.NewToolResultText(string(data)), nil
}

func (s *Server) createIntegrationHandler(ctx context.Context, request mcp.CallToolRequest) (*mcp.CallToolResult, error) {
	args := request.Params.Arguments
	integrationType, ok := args["type"].(string)
	if !ok || integrationType == "" {
		return mcp.NewToolResultError("type parameter is required"), nil
	}

	name, ok := args["name"].(string)
	if !ok || name == "" {
		return mcp.NewToolResultError("name parameter is required"), nil
	}

	config, _ := args["config"].(map[string]interface{})

	// Only support GitHub integration for now as that's what the client supports
	if integrationType == "github" {
		url, err := s.client.CreateGithubIntegration(ctx)
		if err != nil {
			return mcp.NewToolResultError(fmt.Sprintf("Failed to create GitHub integration: %v", err)), nil
		}

		result := map[string]interface{}{
			"type":             integrationType,
			"name":             name,
			"config":           config,
			"installation_url": url,
		}

		data, err := json.MarshalIndent(result, "", "  ")
		if err != nil {
			return mcp.NewToolResultError(fmt.Sprintf("Failed to marshal integration: %v", err)), nil
		}

		return mcp.NewToolResultText(fmt.Sprintf("✅ GitHub integration setup initiated:\n%s\n\nVisit the installation URL to complete the setup.", string(data))), nil
	}

	return mcp.NewToolResultError(fmt.Sprintf("Integration type '%s' not yet supported. Currently supported: github", integrationType)), nil
}

func (s *Server) listSourcesHandler(ctx context.Context, request mcp.CallToolRequest) (*mcp.CallToolResult, error) {
	sources, err := s.client.ListSources(ctx)
	if err != nil {
		return mcp.NewToolResultError(fmt.Sprintf("Failed to list sources: %v", err)), nil
	}

	data, err := json.MarshalIndent(sources, "", "  ")
	if err != nil {
		return mcp.NewToolResultError(fmt.Sprintf("Failed to marshal sources: %v", err)), nil
	}

	return mcp.NewToolResultText(string(data)), nil
}

func (s *Server) listSecretsHandler(ctx context.Context, request mcp.CallToolRequest) (*mcp.CallToolResult, error) {
	secrets, err := s.client.ListSecrets(ctx)
	if err != nil {
		return mcp.NewToolResultError(fmt.Sprintf("Failed to list secrets: %v", err)), nil
	}

	data, err := json.MarshalIndent(secrets, "", "  ")
	if err != nil {
		return mcp.NewToolResultError(fmt.Sprintf("Failed to marshal secrets: %v", err)), nil
	}

	return mcp.NewToolResultText(string(data)), nil
}

func (s *Server) checkRunnerHealthHandler(ctx context.Context, request mcp.CallToolRequest) (*mcp.CallToolResult, error) {
	args := request.Params.Arguments
	runnerName, _ := args["runner_name"].(string)

	if runnerName != "" {
		// Check specific runner health using production API
		runner, err := s.client.GetRunner(ctx, runnerName)
		if err != nil {
			return mcp.NewToolResultError(fmt.Sprintf("Failed to get runner '%s': %v", runnerName, err)), nil
		}

		health := map[string]interface{}{
			"name":        runner.Name,
			"type":        runner.RunnerType,
			"description": runner.Description,
			"version":     runner.Version,
			"namespace":   runner.Namespace,
			"checked_at":  time.Now().UTC().Format(time.RFC3339),
			"runner_health": map[string]interface{}{
				"status":  runner.RunnerHealth.Status,
				"health":  runner.RunnerHealth.Health,
				"version": runner.RunnerHealth.Version,
			},
			"tool_manager_health": map[string]interface{}{
				"status":  runner.ToolManagerHealth.Status,
				"health":  runner.ToolManagerHealth.Health,
				"version": runner.ToolManagerHealth.Version,
				"error":   runner.ToolManagerHealth.Error,
			},
			"agent_manager_health": map[string]interface{}{
				"status":  runner.AgentManagerHealth.Status,
				"health":  runner.AgentManagerHealth.Health,
				"version": runner.AgentManagerHealth.Version,
				"error":   runner.AgentManagerHealth.Error,
			},
		}

		data, err := json.MarshalIndent(health, "", "  ")
		if err != nil {
			return mcp.NewToolResultError(fmt.Sprintf("Failed to marshal health data: %v", err)), nil
		}

		return mcp.NewToolResultText(string(data)), nil
	} else {
		// Check all runners health using production API with concurrent health checks
		runners, err := s.client.ListRunners(ctx)
		if err != nil {
			return mcp.NewToolResultError(fmt.Sprintf("Failed to list runners: %v", err)), nil
		}

		healthResults := make(map[string]interface{})
		for _, runner := range runners {
			healthResults[runner.Name] = map[string]interface{}{
				"name":        runner.Name,
				"type":        runner.RunnerType,
				"description": runner.Description,
				"version":     runner.Version,
				"namespace":   runner.Namespace,
				"checked_at":  time.Now().UTC().Format(time.RFC3339),
				"runner_health": map[string]interface{}{
					"status":  runner.RunnerHealth.Status,
					"health":  runner.RunnerHealth.Health,
					"version": runner.RunnerHealth.Version,
				},
				"tool_manager_health": map[string]interface{}{
					"status":  runner.ToolManagerHealth.Status,
					"health":  runner.ToolManagerHealth.Health,
					"version": runner.ToolManagerHealth.Version,
					"error":   runner.ToolManagerHealth.Error,
				},
				"agent_manager_health": map[string]interface{}{
					"status":  runner.AgentManagerHealth.Status,
					"health":  runner.AgentManagerHealth.Health,
					"version": runner.AgentManagerHealth.Version,
					"error":   runner.AgentManagerHealth.Error,
				},
			}
		}

		data, err := json.MarshalIndent(healthResults, "", "  ")
		if err != nil {
			return mcp.NewToolResultError(fmt.Sprintf("Failed to marshal health data: %v", err)), nil
		}

		return mcp.NewToolResultText(string(data)), nil
	}
}

func (s *Server) findAvailableRunnerHandler(ctx context.Context, request mcp.CallToolRequest) (*mcp.CallToolResult, error) {
	args := request.Params.Arguments
	runnerType, _ := args["runner_type"].(string)

	// Get all runners with health information using production API
	runners, err := s.client.ListRunners(ctx)
	if err != nil {
		return mcp.NewToolResultError(fmt.Sprintf("Failed to list runners: %v", err)), nil
	}

	// Find best available runner using production health scoring
	var bestRunner *kubiya.Runner
	var bestScore float64 = -1

	result := map[string]interface{}{
		"requested_type": runnerType,
		"candidates":     []map[string]interface{}{},
		"selected":       nil,
		"reason":         "",
	}

	candidates := []map[string]interface{}{}

	for _, runner := range runners {
		// Filter by type if specified
		if runnerType != "" && runner.RunnerType != runnerType {
			continue
		}

		// Production health scoring based on actual health status
		score := 0.0
		overallStatus := "unhealthy"

		// Check runner health status
		runnerHealthy := (runner.RunnerHealth.Status == "healthy" || runner.RunnerHealth.Status == "ok") &&
			(runner.RunnerHealth.Health == "healthy" || runner.RunnerHealth.Health == "true")

		// Check tool manager health
		toolManagerHealthy := (runner.ToolManagerHealth.Status == "healthy" || runner.ToolManagerHealth.Status == "ok") &&
			runner.ToolManagerHealth.Error == ""

		// Check agent manager health
		agentManagerHealthy := (runner.AgentManagerHealth.Status == "healthy" || runner.AgentManagerHealth.Status == "ok") &&
			runner.AgentManagerHealth.Error == ""

		// Calculate composite health score
		if runnerHealthy && toolManagerHealthy && agentManagerHealthy {
			score = 100.0
			overallStatus = "healthy"
		} else if runnerHealthy && (toolManagerHealthy || agentManagerHealthy) {
			score = 70.0
			overallStatus = "degraded"
		} else if runnerHealthy {
			score = 50.0
			overallStatus = "partial"
		} else {
			score = 0.0
			overallStatus = "unhealthy"
		}

		// Priority bonus for known reliable runners
		priorityRunners := []string{"kubiya-hosted", "kubiya-hosted-1", "kubiya-cloud"}
		for _, priority := range priorityRunners {
			if runner.Name == priority && score > 0 {
				score += 10.0 // Bonus for priority runners
				break
			}
		}

		candidate := map[string]interface{}{
			"name":        runner.Name,
			"type":        runner.RunnerType,
			"status":      overallStatus,
			"score":       score,
			"description": runner.Description,
			"namespace":   runner.Namespace,
			"health_details": map[string]interface{}{
				"runner_health":        runner.RunnerHealth,
				"tool_manager_health":  runner.ToolManagerHealth,
				"agent_manager_health": runner.AgentManagerHealth,
			},
		}
		candidates = append(candidates, candidate)

		if score > bestScore {
			bestScore = score
			bestRunner = &runner
		}
	}

	result["candidates"] = candidates

	if bestRunner != nil && bestScore > 0 {
		result["selected"] = map[string]interface{}{
			"name":        bestRunner.Name,
			"type":        bestRunner.RunnerType,
			"score":       bestScore,
			"description": bestRunner.Description,
			"namespace":   bestRunner.Namespace,
		}
		result["reason"] = fmt.Sprintf("Selected runner '%s' with health score %.1f", bestRunner.Name, bestScore)
	} else {
		result["reason"] = "No healthy runners found"
		if runnerType != "" {
			result["reason"] = fmt.Sprintf("No healthy runners found for type '%s'", runnerType)
		}
	}

	data, err := json.MarshalIndent(result, "", "  ")
	if err != nil {
		return mcp.NewToolResultError(fmt.Sprintf("Failed to marshal result: %v", err)), nil
	}

	return mcp.NewToolResultText(string(data)), nil
}

func (s *Server) createSourceHandler(ctx context.Context, request mcp.CallToolRequest) (*mcp.CallToolResult, error) {
	args := request.Params.Arguments
	name, ok := args["name"].(string)
	if !ok || name == "" {
		return mcp.NewToolResultError("name parameter is required"), nil
	}

	url, ok := args["url"].(string)
	if !ok || url == "" {
		return mcp.NewToolResultError("url parameter is required"), nil
	}

	description, _ := args["description"].(string)

	// Use the existing CreateSource method with options
	source, err := s.client.CreateSource(ctx, url, kubiya.WithName(name))
	if err != nil {
		return mcp.NewToolResultError(fmt.Sprintf("Failed to create source: %v", err)), nil
	}

	data, err := json.MarshalIndent(source, "", "  ")
	if err != nil {
		return mcp.NewToolResultError(fmt.Sprintf("Failed to marshal source: %v", err)), nil
	}

	output := fmt.Sprintf("✅ Source created successfully:\n%s", string(data))
	if description != "" {
		output = fmt.Sprintf("✅ Source '%s' created successfully:\n%s", description, string(data))
	}

	return mcp.NewToolResultText(output), nil
}

// Additional handlers for enhanced functionality

func (s *Server) executeToolFromSourceHandler(ctx context.Context, request mcp.CallToolRequest) (*mcp.CallToolResult, error) {
	args := request.Params.Arguments

	sourceIdentifier, ok := args["source"].(string)
	if !ok || sourceIdentifier == "" {
		return mcp.NewToolResultError("source parameter is required (UUID or URL)"), nil
	}

	toolName, ok := args["tool_name"].(string)
	if !ok || toolName == "" {
		return mcp.NewToolResultError("tool_name parameter is required"), nil
	}

	runner, _ := args["runner"].(string)
	toolArgs, _ := args["args"].(map[string]interface{})

	// First, get the source (either by UUID or URL)
	var source *kubiya.Source
	var err error

	// Try to get by UUID first
	if len(sourceIdentifier) == 36 { // UUID length
		source, err = s.client.GetSource(ctx, sourceIdentifier)
		if err != nil {
			// If UUID failed, try by URL
			source, err = s.client.GetSourceByURL(ctx, sourceIdentifier)
		}
	} else {
		// Try by URL
		source, err = s.client.GetSourceByURL(ctx, sourceIdentifier)
	}

	if err != nil {
		return mcp.NewToolResultError(fmt.Sprintf("Failed to find source '%s': %v", sourceIdentifier, err)), nil
	}

	// Get source metadata to find tools
	sourceMetadata, err := s.client.GetSourceMetadata(ctx, source.UUID)
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

	if foundTool == nil {
		return mcp.NewToolResultError(fmt.Sprintf("Tool '%s' not found in source '%s'", toolName, sourceIdentifier)), nil
	}

	// Execute the tool using the existing execution logic
	return s.executeSpecificTool(ctx, foundTool, toolArgs, runner)
}

func (s *Server) discoverSourceHandler(ctx context.Context, request mcp.CallToolRequest) (*mcp.CallToolResult, error) {
	args := request.Params.Arguments

	sourceURL, ok := args["url"].(string)
	if !ok || sourceURL == "" {
		return mcp.NewToolResultError("url parameter is required"), nil
	}

	runner, _ := args["runner"].(string)
	config, _ := args["config"].(map[string]interface{})

	// Use the existing DiscoverSource method
	discovered, err := s.client.DiscoverSource(ctx, sourceURL, config, runner, nil)
	if err != nil {
		return mcp.NewToolResultError(fmt.Sprintf("Failed to discover source: %v", err)), nil
	}

	data, err := json.MarshalIndent(discovered, "", "  ")
	if err != nil {
		return mcp.NewToolResultError(fmt.Sprintf("Failed to marshal discovery result: %v", err)), nil
	}

	return mcp.NewToolResultText(fmt.Sprintf("🔍 Source discovery completed:\n%s", string(data))), nil
}

// Helper method to execute a specific tool
func (s *Server) executeSpecificTool(ctx context.Context, tool *kubiya.Tool, args map[string]interface{}, runner string) (*mcp.CallToolResult, error) {
	// Convert tool arguments to the format expected by ExecuteToolWithTimeout
	toolDef := map[string]interface{}{
		"name":        tool.Name,
		"description": tool.Description,
		"args":        args,
	}

	if runner == "" {
		runner = "default"
	}

	// Policy validation (if enabled)
	if s.serverConfig.EnableOPAPolicies {
		allowed, message, err := s.client.ValidateToolExecution(ctx, tool.Name, args, runner)
		if err != nil {
			return mcp.NewToolResultError(fmt.Sprintf("Policy validation failed: %v", err)), nil
		}
		if !allowed {
			errorMsg := fmt.Sprintf("Tool execution from source denied by policy: %s", tool.Name)
			if message != "" {
				errorMsg += fmt.Sprintf(" - %s", message)
			}
			return mcp.NewToolResultError(errorMsg), nil
		}
	}

	argVals := make(map[string]any) // to get argument values
	// Execute with timeout
	timeout := 30 * time.Minute // Extended timeout for platform tools
	eventChan, err := s.client.ExecuteToolWithTimeout(ctx, tool.Name, toolDef, runner, timeout, argVals)
	if err != nil {
		return mcp.NewToolResultError(fmt.Sprintf("Failed to execute tool: %v", err)), nil
	}

	var output strings.Builder
	output.WriteString(fmt.Sprintf("🚀 Executing tool: %s\n", tool.Name))
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

// createOnDemandToolHandler handles on-demand tool creation and execution
func (s *Server) createOnDemandToolHandler(ctx context.Context, request mcp.CallToolRequest) (*mcp.CallToolResult, error) {
	args := request.Params.Arguments

	toolDef, ok := args["tool_def"].(map[string]interface{})
	if !ok {
		return mcp.NewToolResultError("tool_def parameter is required and must be an object"), nil
	}

	// Validate required fields in tool definition
	name, ok := toolDef["name"].(string)
	if !ok || name == "" {
		return mcp.NewToolResultError("tool_def.name is required"), nil
	}

	description, _ := toolDef["description"].(string)
	toolType, _ := toolDef["type"].(string)
	if toolType == "" {
		toolType = "docker" // default to docker
	}

	// Get optional parameters
	runner, _ := args["runner"].(string)
	if runner == "" {
		runner = "default"
	}

	toolArgs, _ := args["args"].(map[string]interface{})
	if toolArgs == nil {
		toolArgs = make(map[string]interface{})
	}

	integrationTemplate, _ := args["integration_template"].(string)
	_ = integrationTemplate // Mark as used to avoid unused variable warning

	// Policy validation (if enabled)
	if s.serverConfig.EnableOPAPolicies {
		allowed, message, err := s.client.ValidateToolExecution(ctx, name, toolArgs, runner)
		if err != nil {
			return mcp.NewToolResultError(fmt.Sprintf("Policy validation failed: %v", err)), nil
		}
		if !allowed {
			errorMsg := fmt.Sprintf("On-demand tool execution denied by policy: %s", name)
			if message != "" {
				errorMsg += fmt.Sprintf(" - %s", message)
			}
			return mcp.NewToolResultError(errorMsg), nil
		}
	}

	var output strings.Builder
	output.WriteString(fmt.Sprintf("🔨 Creating on-demand tool: %s\n", name))
	output.WriteString(fmt.Sprintf("📝 Description: %s\n", description))
	output.WriteString(fmt.Sprintf("🔧 Type: %s\n", toolType))
	output.WriteString(fmt.Sprintf("📍 Runner: %s\n", runner))
	output.WriteString("=" + strings.Repeat("=", 50) + "\n\n")

	// Execute with timeout (30 minutes for discovery operations)
	timeout := 30 * time.Minute
	argVals := make(map[string]any) // to get argument values

	eventChan, err := s.client.ExecuteToolWithTimeout(ctx, name, toolDef, runner, timeout, argVals)
	if err != nil {
		return mcp.NewToolResultError(fmt.Sprintf("Failed to execute on-demand tool: %v", err)), nil
	}

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
			output.WriteString("\n✅ On-demand tool execution completed\n")
		default:
			output.WriteString(fmt.Sprintf("📝 %s: %s\n", event.Type, event.Data))
		}
	}

	return mcp.NewToolResultText(output.String()), nil
}

// executeWorkflowHandler handles workflow execution with streaming
func (s *Server) executeWorkflowHandler(ctx context.Context, request mcp.CallToolRequest) (*mcp.CallToolResult, error) {
	args := request.Params.Arguments

	workflowDef, ok := args["workflow_def"].(map[string]interface{})
	if !ok {
		return mcp.NewToolResultError("workflow_def parameter is required and must be an object"), nil
	}

	// Validate required fields in workflow definition
	name, ok := workflowDef["name"].(string)
	if !ok || name == "" {
		return mcp.NewToolResultError("workflow_def.name is required"), nil
	}

	description, _ := workflowDef["description"].(string)
	workflowType, _ := workflowDef["type"].(string)
	if workflowType == "" {
		workflowType = "chain" // default to chain
	}

	// Get steps
	stepsInterface, ok := workflowDef["steps"]
	if !ok {
		return mcp.NewToolResultError("workflow_def.steps is required"), nil
	}

	steps, ok := stepsInterface.([]interface{})
	if !ok {
		return mcp.NewToolResultError("workflow_def.steps must be an array"), nil
	}

	// Get optional parameters with enhanced runner selection
	runner, _ := args["runner"].(string)
	if runner == "" {
		// Try to get runner from workflow definition
		if wfRunner, ok := workflowDef["runner"].(string); ok && wfRunner != "" {
			runner = wfRunner
		} else {
			runner = "default"
		}
	}

	workflowParams, _ := args["params"].(map[string]interface{})
	if workflowParams == nil {
		workflowParams = make(map[string]interface{})
	}

	// Get secrets and environment variables from MCP arguments
	secrets, _ := args["secrets"].(map[string]interface{})
	envVars, _ := args["env"].(map[string]interface{})

	var output strings.Builder

	// Enhanced runner selection logic
	if runner == "auto" {
		// Use find_available_runner logic to get best runner
		runners, err := s.client.ListRunners(ctx)
		if err == nil && len(runners) > 0 {
			// Simple auto-selection: pick first available runner
			runner = runners[0].Name
			output.WriteString(fmt.Sprintf("🤖 Auto-selected runner: %s\n", runner))
		} else {
			runner = "default"
			output.WriteString("⚠️ Auto-selection failed, using default runner\n")
		}
	} else if runner == "" || runner == "default" {
		runner = "default"
	}
	output.WriteString(fmt.Sprintf("🚀 Executing workflow: %s\n", name))
	output.WriteString(fmt.Sprintf("📝 Description: %s\n", description))
	output.WriteString(fmt.Sprintf("🔧 Type: %s\n", workflowType))
	output.WriteString(fmt.Sprintf("📍 Runner: %s\n", runner))
	output.WriteString(fmt.Sprintf("📊 Steps: %d\n", len(steps)))
	if secrets != nil && len(secrets) > 0 {
		output.WriteString(fmt.Sprintf("🔐 Secrets: %d provided\n", len(secrets)))
	}
	if envVars != nil && len(envVars) > 0 {
		output.WriteString(fmt.Sprintf("🌍 Environment Variables: %d provided\n", len(envVars)))
	}
	output.WriteString("=" + strings.Repeat("=", 50) + "\n\n")

	// Create the complete workflow definition to pass to execution
	completeWorkflow := map[string]interface{}{
		"name":        name,
		"description": description,
		"type":        workflowType,
		"steps":       steps,
		"runner":      runner,
		"params":      workflowParams,
	}

	// Handle environment variables - merge with workflow env section
	workflowEnv := make(map[string]interface{})

	// First, get env from workflow definition
	if existingEnv, ok := workflowDef["env"].(map[string]interface{}); ok {
		for k, v := range existingEnv {
			workflowEnv[k] = v
		}
	}

	// Then merge with env variables from MCP request
	if envVars != nil {
		for k, v := range envVars {
			workflowEnv[k] = v
		}
	}

	// Add any additional workflow properties
	for key, value := range workflowDef {
		if key != "name" && key != "description" && key != "type" && key != "steps" && key != "runner" && key != "params" && key != "env" {
			completeWorkflow[key] = value
		}
	}

	// Set the merged environment variables
	if len(workflowEnv) > 0 {
		completeWorkflow["env"] = workflowEnv
	}

	// Policy validation (if enabled)
	if s.serverConfig.EnableOPAPolicies {
		allowed, issues, err := s.client.ValidateWorkflowExecution(ctx, completeWorkflow, workflowParams, runner)
		if err != nil {
			return mcp.NewToolResultError(fmt.Sprintf("Workflow policy validation failed: %v", err)), nil
		}
		if !allowed {
			output.WriteString("❌ Workflow execution denied by policy:\n")
			for _, issue := range issues {
				output.WriteString(fmt.Sprintf("  • %s\n", issue))
			}
			return mcp.NewToolResultError(output.String()), nil
		}
		if len(issues) > 0 {
			output.WriteString("⚠️ Workflow execution permitted with warnings:\n")
			for _, issue := range issues {
				output.WriteString(fmt.Sprintf("  • %s\n", issue))
			}
			output.WriteString("\n")
		}
	}

	// Prepare workflow execution request - use 'params' instead of 'variables'
	workflowReq := kubiya.WorkflowExecutionRequest{
		Name:        name,
		Description: description,
		Steps:       steps,
		Params:      workflowParams,
		Secrets:     secrets,
		Env:         workflowEnv,
	}

	// Create robust workflow client for better connection handling
	robustClient, err := kubiya.NewRobustWorkflowClient(s.client.Workflow(), false)
	if err != nil {
		return mcp.NewToolResultError(fmt.Sprintf("Failed to create robust workflow client: %v", err)), nil
	}

	// Execute workflow using robust client with connection recovery
	eventChan, err := robustClient.ExecuteWorkflowRobust(ctx, workflowReq, runner)
	if err != nil {
		return mcp.NewToolResultError(fmt.Sprintf("Failed to execute workflow: %v", err)), nil
	}

	var executionID string
	for event := range eventChan {
		switch event.Type {
		case "state":
			// Initial state or state updates
			executionID = event.ExecutionID
			if event.State != nil {
				output.WriteString(fmt.Sprintf("📊 %s\n", event.Message))
				output.WriteString(fmt.Sprintf("🆔 Execution ID: %s\n", executionID))
				output.WriteString(fmt.Sprintf("📈 Progress: %d/%d steps completed\n\n",
					event.State.CompletedSteps, event.State.TotalSteps))
			}
		case "step":
			// Step status updates
			if event.StepStatus == "running" {
				if event.State != nil {
					progress := fmt.Sprintf("[%d/%d]", event.State.CompletedSteps+1, event.State.TotalSteps)
					output.WriteString(fmt.Sprintf("▶️ %s %s\n", progress, event.StepName))
				} else {
					output.WriteString(fmt.Sprintf("▶️ %s\n", event.StepName))
				}
				output.WriteString("⏳ Running...\n")
			} else if event.StepStatus == "completed" || event.StepStatus == "finished" {
				if event.Data != "" {
					// Show step output (truncated for MCP)
					displayOutput := event.Data
					if len(event.Data) > 500 {
						displayOutput = event.Data[:500] + "..."
					}
					output.WriteString(fmt.Sprintf("📤 Output: %s\n", displayOutput))
				}
				output.WriteString("✅ Step completed\n\n")
			} else if event.StepStatus == "failed" {
				output.WriteString("❌ Step failed\n\n")
			}
		case "data":
			// Raw data output
			if event.Data != "" {
				output.WriteString(fmt.Sprintf("📝 %s\n", event.Data))
			}
		case "reconnect":
			// Connection recovery
			if event.Reconnect {
				output.WriteString(fmt.Sprintf("🔄 %s\n", event.Message))
			} else {
				output.WriteString(fmt.Sprintf("✅ %s\n", event.Message))
			}
		case "complete":
			// Workflow completion
			if event.State != nil {
				if event.State.Status == "completed" {
					output.WriteString("\n🎉 Workflow completed successfully!\n")
				} else {
					output.WriteString("\n💥 Workflow execution failed\n")
				}
				if event.State.RetryCount > 0 {
					output.WriteString(fmt.Sprintf("🔄 Connection retries: %d\n", event.State.RetryCount))
				}
				output.WriteString(fmt.Sprintf("📊 Steps completed: %d/%d\n",
					event.State.CompletedSteps, event.State.TotalSteps))
			}
		case "error":
			// Error events
			if event.Error != "" {
				output.WriteString(fmt.Sprintf("💀 Error: %s\n", event.Error))
				return mcp.NewToolResultError(output.String()), nil
			}
		}
	}

	return mcp.NewToolResultText(output.String()), nil
}
