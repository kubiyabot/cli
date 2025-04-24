package cli

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"
	"sync"

	"github.com/google/uuid"
	"github.com/kubiyabot/cli/internal/config"
	"github.com/kubiyabot/cli/internal/kubiya"
	"github.com/kubiyabot/cli/internal/mcp_helpers"

	"github.com/mark3labs/mcp-go/mcp"
	"github.com/mark3labs/mcp-go/server"
	"github.com/spf13/afero"
	"github.com/spf13/cobra"
)

func loadTeammates(ctx context.Context, cli *kubiya.Client, teammatesIdentifiers []string) ([]kubiya.Teammate, error) {
	allTeammates, err := cli.ListTeammates(ctx)
	if err != nil {
		return nil, fmt.Errorf("failed to list teammates: %w", err)
	}
	ret := make([]kubiya.Teammate, 0)
	for _, teammate := range allTeammates {
		if teammate.UUID == "" || teammate.Name == "" {
			continue
		}
		if contains(teammatesIdentifiers, teammate.UUID) || contains(teammatesIdentifiers, teammate.Name) || teammatesIdentifiers[0] == "*" {
			ret = append(ret, teammate)
		}
	}
	return ret, nil
}

func getToolsFromTeammate(ctx context.Context, cli *kubiya.Client, teammates []kubiya.Teammate) (map[string][]kubiya.Tool, map[string]kubiya.Teammate, error) {
	toolsMap := make(map[string][]kubiya.Tool, 0)
	teammateMap := make(map[string]kubiya.Teammate, 0)
	var wg sync.WaitGroup
	var mu sync.Mutex
	for _, teammate := range teammates {
		teammateMap[teammate.UUID] = teammate
		for _, sourceid := range teammate.Sources {
			wg.Add(1)
			go func() {
				defer wg.Done()
				sourceMd, err := cli.GetSourceMetadata(ctx, sourceid)
				if err != nil {
					fmt.Printf("failed to get source metadata: %s\n", err)
					return
				}
				mu.Lock()
				defer mu.Unlock()
				toolsMap[teammate.UUID] = append(toolsMap[teammate.UUID], sourceMd.Tools...)
			}()
		}
	}
	wg.Wait()
	return toolsMap, teammateMap, nil
}

func executeKubiyaToolViaTeammate(ctx context.Context, cli *kubiya.Client, teammate kubiya.Teammate, tool kubiya.Tool, args map[string]any) (string, string, error) {
	msg := fmt.Sprintf("Execute %s", tool.Name)
	if len(args) > 0 {
		msg += "\nProvided argument values:\n"
		for k, v := range args {
			msg += fmt.Sprintf("%s: %v\n", k, v)
		}
	}
	sessionId := uuid.NewString()
	replies, err := cli.GetConversationMessages(ctx, teammate.UUID, msg, sessionId)
	if err != nil {
		return "", "", fmt.Errorf("failed to get reply messages: %w", err)
	}
	var inp, outp string
	for _, reply := range replies {
		if reply.Type == "tool_output" {
			outp = reply.Message
		}
		if reply.Type == "tool" {
			inp = reply.Message
		}
	}
	return inp, outp, nil
}

func canonicalToolName(teammatename, toolname string) string {
	newToolName := fmt.Sprintf("%s_%s", teammatename, toolname)
	newToolName = strings.ReplaceAll(newToolName, " ", "_")
	newToolName = strings.ReplaceAll(newToolName, "-", "_")
	if len(newToolName) > 64 {
		return newToolName[:64]
	}
	return newToolName
}

func getToolInputSchema(tool kubiya.Tool) json.RawMessage {
	// converts the tool description as in the metadata to a valid mcp input schema

	// input schema types, as taken from the spec
	// https://github.com/modelcontextprotocol/modelcontextprotocol/blob/main/schema/2025-03-26/schema.ts#L800-L803

	type propertySchema struct {
		Type string `json:"type"`
		Desc string `json:"description,omitempty"`
	}
	type toolSchema struct {
		Type       string                    `json:"type"`
		Properties map[string]propertySchema `json:"properties"`
		Required   []string                  `json:"required"`
	}

	// create the actual schema
	ts := toolSchema{
		Type:       "object", // this is always an object
		Properties: make(map[string]propertySchema),
		Required:   make([]string, 0),
	}
	for _, arg := range tool.Args {
		ts.Properties[arg.Name] = propertySchema{
			Type: canonicalArgName(arg.Type),
			Desc: arg.Description,
		}

		if arg.Required {
			ts.Required = append(ts.Required, arg.Name)
		}
	}

	ret, _ := json.Marshal(ts)
	return ret
}

func canonicalArgName(argType string) string {
	switch argType {
	case "string", "text", "str":
		return "string"
	case "number", "integer", "float", "double", "int":
		return "number"
	case "boolean", "bool":
		return "boolean"
	case "array", "list":
		return "array"
	case "object", "dict", "map":
		return "object"
	default:
		return ""
	}
}

func kubiyaToolMcpWrapper(ctx context.Context, cli *kubiya.Client, teammate kubiya.Teammate, tool kubiya.Tool) server.ServerTool {
	handler := func(ctx context.Context, request mcp.CallToolRequest) (*mcp.CallToolResult, error) {
		_, outp, err := executeKubiyaToolViaTeammate(ctx, cli, teammate, tool, request.Params.Arguments)
		if err != nil {
			return nil, fmt.Errorf("failed to execute tool: %w", err)
		}
		return mcp.NewToolResultText(outp), nil
	}
	return server.ServerTool{
		Tool: mcp.NewToolWithRawSchema(
			canonicalToolName(teammate.Name, tool.Name),
			tool.Description,
			getToolInputSchema(tool),
		),
		Handler: handler,
	}
}

func transformKubiyaToolsToMcpTools(ctx context.Context, cli *kubiya.Client, teammate kubiya.Teammate, tools []kubiya.Tool) []server.ServerTool {
	ret := make([]server.ServerTool, len(tools))
	for i, tool := range tools {
		ret[i] = kubiyaToolMcpWrapper(ctx, cli, teammate, tool)
	}
	return ret
}

func newMcpServeCommand(cfg *config.Config, fs afero.Fs) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "serve",
		Short: "💻 Serve MCP server",
		Long:  "execute the local MCP server",
		RunE: func(cmd *cobra.Command, args []string) error {
			mcp_conf, err := mcp_helpers.LoadMcpConfig(fs)
			cobra.CheckErr(err)
			cfg, err := config.Load()
			cobra.CheckErr(err)

			cfg.APIKey = mcp_conf.ApiKey
			cli := kubiya.NewClient(cfg)

			teammates, err := loadTeammates(cmd.Context(), cli, mcp_conf.TeammateIds)
			if err != nil {
				return fmt.Errorf("failed to get teammate IDs: %w", err)
			}
			toolsmap, teammatesMap, err := getToolsFromTeammate(cmd.Context(), cli, teammates)
			if err != nil {
				return fmt.Errorf("failed to get tools from teammate sources: %w", err)
			}
			mcptools := make([]server.ServerTool, 0)
			for teammateId, tools := range toolsmap {
				for _, tool := range tools {
					teammate, ok := teammatesMap[teammateId]
					if !ok {
						return fmt.Errorf("failed to get teammate %s", teammateId)
					}
					mcptools = append(mcptools, kubiyaToolMcpWrapper(cmd.Context(), cli, teammate, tool))
				}
			}
			startMcpServer(mcptools)
			return nil
		},
	}
	return cmd
}

func startMcpServer(tools []server.ServerTool) error {
	s := server.NewMCPServer(
		"Kubiya",
		"0.0.1",
		server.WithLogging(),
	)

	// Add tools handler
	s.AddTools(tools...)

	// Start the stdio server
	if err := server.ServeStdio(s); err != nil {
		fmt.Printf("Server error: %v\n", err)
	}
	return nil
}
