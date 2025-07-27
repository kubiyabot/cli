package mcp

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"
	"time"

	"github.com/kubiyabot/cli/internal/kubiya"
	"github.com/mark3labs/mcp-go/mcp"
	"github.com/mark3labs/mcp-go/server"
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
		if tool.Name == toolName {
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
		"name":        whitelistedTool.Name,
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
		allowed, message, err := s.client.ValidateToolExecution(ctx, whitelistedTool.Name, toolArgs, runner)
		if err != nil {
			return mcp.NewToolResultError(fmt.Sprintf("Policy validation failed: %v", err)), nil
		}
		if !allowed {
			errorMsg := fmt.Sprintf("Whitelisted tool execution denied by policy: %s", whitelistedTool.Name)
			if message != "" {
				errorMsg += fmt.Sprintf(" - %s", message)
			}
			return mcp.NewToolResultError(errorMsg), nil
		}
	}

	var output strings.Builder
	output.WriteString(fmt.Sprintf("✅ Executing whitelisted tool: %s\n", whitelistedTool.Name))
	output.WriteString(fmt.Sprintf("📝 Description: %s\n", whitelistedTool.Description))
	output.WriteString(fmt.Sprintf("🔧 Tool Name: %s\n", whitelistedTool.Name))
	output.WriteString(fmt.Sprintf("📍 Runner: %s\n", runner))
	output.WriteString("=" + strings.Repeat("=", 50) + "\n\n")

	// Execute with timeout (30 minutes for whitelisted tools)
	timeout := 30 * time.Minute

	argVals := make(map[string]any) // to get argument values

	eventChan, err := s.client.ExecuteToolWithTimeout(ctx, whitelistedTool.Name, toolDef, runner, timeout, argVals)
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

// WhitelistedToolHandler handles individual whitelisted tools as MCP tools
type WhitelistedToolHandler struct {
	client     *kubiya.Client
	tool       WhitelistedTool
	kubiyaTool kubiya.Tool
}

// NewWhitelistedToolHandler creates a new handler for a whitelisted tool
func NewWhitelistedToolHandler(client *kubiya.Client, tool WhitelistedTool, kubiyaTool kubiya.Tool) *WhitelistedToolHandler {
	return &WhitelistedToolHandler{
		client:     client,
		tool:       tool,
		kubiyaTool: kubiyaTool,
	}
}

// Register registers the whitelisted tool as an individual MCP tool
func (h *WhitelistedToolHandler) Register(server *server.MCPServer) error {
	// Build MCP tool options
	var opts []mcp.ToolOption
	opts = append(opts, mcp.WithDescription(h.tool.Description))

	// Add arguments to the MCP tool
	for _, arg := range h.tool.Args {
		switch arg.Type {
		case "string":
			if arg.Required {
				opts = append(opts, mcp.WithString(arg.Name, mcp.Required(), mcp.Description(arg.Description)))
			} else {
				opts = append(opts, mcp.WithString(arg.Name, mcp.Description(arg.Description)))
			}
		case "number", "int", "integer":
			if arg.Required {
				opts = append(opts, mcp.WithNumber(arg.Name, mcp.Required(), mcp.Description(arg.Description)))
			} else {
				opts = append(opts, mcp.WithNumber(arg.Name, mcp.Description(arg.Description)))
			}
		case "boolean", "bool":
			if arg.Required {
				opts = append(opts, mcp.WithBoolean(arg.Name, mcp.Required(), mcp.Description(arg.Description)))
			} else {
				opts = append(opts, mcp.WithBoolean(arg.Name, mcp.Description(arg.Description)))
			}
		case "object":
			if arg.Required {
				opts = append(opts, mcp.WithObject(arg.Name, mcp.Required(), mcp.Description(arg.Description)))
			} else {
				opts = append(opts, mcp.WithObject(arg.Name, mcp.Description(arg.Description)))
			}
		case "array":
			if arg.Required {
				opts = append(opts, mcp.WithArray(arg.Name, mcp.Required(), mcp.Description(arg.Description)))
			} else {
				opts = append(opts, mcp.WithArray(arg.Name, mcp.Description(arg.Description)))
			}
		default:
			// Default to string type
			if arg.Required {
				opts = append(opts, mcp.WithString(arg.Name, mcp.Required(), mcp.Description(arg.Description)))
			} else {
				opts = append(opts, mcp.WithString(arg.Name, mcp.Description(arg.Description)))
			}
		}
	}

	// Register the tool with the handler
	server.AddTool(mcp.NewTool(h.tool.Name, opts...), h.handleToolCall)

	return nil
}

// handleToolCall handles the actual execution of the whitelisted tool
func (h *WhitelistedToolHandler) handleToolCall(ctx context.Context, request mcp.CallToolRequest) (*mcp.CallToolResult, error) {
	// Convert MCP arguments to Kubiya tool execution format
	// Determine the runner to use (from tool config or default)
	runner := h.tool.Runner
	if runner == "" {
		runner = "default"
	}

	// Build tool definition for execution
	toolDef := map[string]interface{}{
		"name":         h.tool.Name,
		"description":  h.tool.Description,
		"type":         h.tool.Type,
		"content":      h.tool.Content,
		"args":         h.tool.Args,
		"env":          h.tool.Env,
		"secrets":      h.tool.Secrets,
		"with_files":   h.tool.WithFiles,
		"with_volumes": h.tool.WithVolumes,
		"long_running": h.tool.LongRunning,
		"metadata":     h.tool.Metadata,
	}

	// Add image if specified
	if h.tool.Image != "" {
		toolDef["image"] = h.tool.Image
	}

	// Add integration template if specified
	if len(h.tool.Integrations) > 0 {
		toolDef["integration_template"] = h.tool.Integrations[0] // Use first integration
	}

	// Determine timeout
	timeout := time.Duration(h.tool.Timeout) * time.Second
	if timeout == 0 {
		timeout = 10 * time.Minute // Default timeout
	}

	// Execute the tool using the client
	result, err := h.client.ExecuteToolWithTimeout(ctx, h.tool.Name, toolDef, runner, timeout, request.Params.Arguments)
	if err != nil {
		return mcp.NewToolResultError(fmt.Sprintf("Tool execution failed: %s", err.Error())), nil
	}

	// Format the result output
	output := strings.Builder{}
	output.WriteString(fmt.Sprintf("🔧 Executed tool: %s\n\n", h.tool.Name))

	// Process streaming events
	for event := range result {
		switch event.Type {
		case "data":
			// Parse JSON data from SSE event
			var data map[string]interface{}
			if err := json.Unmarshal([]byte(event.Data), &data); err == nil {
				if eventType, ok := data["type"].(string); ok {
					switch eventType {
					case "stdout":
						if stdout, ok := data["stdout"].(string); ok {
							output.WriteString(stdout)
						}
					case "stderr":
						if stderr, ok := data["stderr"].(string); ok {
							output.WriteString(fmt.Sprintf("❌ Error: %s\n", stderr))
						}
					case "exit":
						if exitCode, ok := data["exit_code"].(float64); ok {
							if exitCode != 0 {
								output.WriteString(fmt.Sprintf("❌ Tool exited with code: %.0f\n", exitCode))
							} else {
								output.WriteString("✅ Tool executed successfully\n")
							}
						}
					default:
						output.WriteString(fmt.Sprintf("📝 %s: %s\n", eventType, event.Data))
					}
				}
			} else {
				// If JSON parsing fails, just output the raw data
				output.WriteString(event.Data)
			}
		case "error":
			output.WriteString(fmt.Sprintf("❌ Error: %s\n", event.Data))
		case "done":
			output.WriteString("✅ Tool execution completed\n")
		default:
			output.WriteString(fmt.Sprintf("📝 %s: %s\n", event.Type, event.Data))
		}
	}

	return mcp.NewToolResultText(output.String()), nil
}
