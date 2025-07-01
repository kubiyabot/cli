package mcp

import (
	"context"
	"fmt"
	"strings"
	"time"

	"github.com/mark3labs/mcp-go/mcp"
)

// executeWhitelistedToolHandler handles execution of whitelisted tools from configuration
func (s *Server) executeWhitelistedToolHandler(ctx context.Context, request mcp.CallToolRequest) (*mcp.CallToolResult, error) {
	args := request.Params.Arguments

	toolName, ok := args["tool_name"].(string)
	if !ok || toolName == "" {
		return mcp.NewToolResultError("tool_name parameter is required"), nil
	}

	// Check if tool is whitelisted
	var whitelistedTool *WhitelistedTool
	for _, tool := range s.serverConfig.WhitelistedTools {
		if tool.Name == toolName || tool.ToolName == toolName {
			whitelistedTool = &tool
			break
		}
	}

	if whitelistedTool == nil {
		return mcp.NewToolResultError(fmt.Sprintf("Tool '%s' is not whitelisted in MCP server configuration", toolName)), nil
	}

	runner, _ := args["runner"].(string)
	toolArgs, _ := args["args"].(map[string]interface{})
	integrationTemplate, _ := args["integration_template"].(string)

	// Create tool definition based on whitelisted configuration
	toolDef := map[string]interface{}{
		"name":        whitelistedTool.ToolName,
		"description": whitelistedTool.Description,
		"args":        toolArgs,
	}
	
	if integrationTemplate != "" {
		toolDef["integration_template"] = integrationTemplate
	}

	// Apply any integrations from whitelist
	if whitelistedTool.Integrations != nil {
		toolDef["integrations"] = whitelistedTool.Integrations
	}

	if runner == "" {
		runner = "default"
	}

	// Policy validation (if enabled)
	if s.serverConfig.EnableOPAPolicies {
		allowed, message, err := s.client.ValidateToolExecution(ctx, whitelistedTool.ToolName, toolArgs, runner)
		if err != nil {
			return mcp.NewToolResultError(fmt.Sprintf("Policy validation failed: %v", err)), nil
		}
		if !allowed {
			errorMsg := fmt.Sprintf("Whitelisted tool execution denied by policy: %s", whitelistedTool.ToolName)
			if message != "" {
				errorMsg += fmt.Sprintf(" - %s", message)
			}
			return mcp.NewToolResultError(errorMsg), nil
		}
	}

	var output strings.Builder
	output.WriteString(fmt.Sprintf("✅ Executing whitelisted tool: %s\n", whitelistedTool.Name))
	output.WriteString(fmt.Sprintf("📝 Description: %s\n", whitelistedTool.Description))
	output.WriteString(fmt.Sprintf("🔧 Tool Name: %s\n", whitelistedTool.ToolName))
	output.WriteString(fmt.Sprintf("📍 Runner: %s\n", runner))
	output.WriteString("=" + strings.Repeat("=", 50) + "\n\n")

	// Execute with timeout (5 minutes)
	timeout := 300 * time.Second
	eventChan, err := s.client.ExecuteToolWithTimeout(ctx, whitelistedTool.ToolName, toolDef, runner, timeout)
	if err != nil {
		return mcp.NewToolResultError(fmt.Sprintf("Failed to execute whitelisted tool: %v", err)), nil
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
			output.WriteString("\n✅ Whitelisted tool execution completed\n")
		default:
			output.WriteString(fmt.Sprintf("📝 %s: %s\n", event.Type, event.Data))
		}
	}

	return mcp.NewToolResultText(output.String()), nil
}