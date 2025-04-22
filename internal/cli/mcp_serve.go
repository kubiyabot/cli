package cli

import (
	"context"
	"errors"
	"fmt"
	"os"
	"strings"

	"github.com/google/uuid"
	"github.com/kubiyabot/cli/internal/config"
	"github.com/kubiyabot/cli/internal/kubiya"

	"github.com/mark3labs/mcp-go/mcp"
	"github.com/mark3labs/mcp-go/server"
	"github.com/spf13/afero"
	"github.com/spf13/cobra"
)

func getTeammateIdentifiers() ([]string, error) {
	envnvar := os.Getenv("TEAMMATE_UUIDS") // or names
	if envnvar == "" {
		return nil, errors.New("TEAMMATE_UUIDS environment variable is not set")
	}
	ret := strings.Split(envnvar, ",")
	for i, id := range ret {
		ret[i] = strings.TrimSpace(id)
	}
	return ret, nil
}

func getSpecificTeammates(ctx context.Context, cli *kubiya.Client) ([]kubiya.Teammate, error) {
	teammatesIdentifiers, err := getTeammateIdentifiers()
	if err != nil {
		return nil, err
	}
	allTeammates, err := cli.ListTeammates(ctx)
	if err != nil {
		return nil, fmt.Errorf("failed to list teammates: %w", err)
	}
	ret := make([]kubiya.Teammate, 0)
	for _, teammate := range allTeammates {
		if teammate.UUID == "" || teammate.Name == "" {
			continue
		}
		if contains(teammatesIdentifiers, teammate.UUID) || contains(teammatesIdentifiers, teammate.Name) {
			ret = append(ret, teammate)
		}
	}
	return ret, nil
}

func getToolsFromTeammate(ctx context.Context, cli *kubiya.Client, teammates []kubiya.Teammate) (map[string][]kubiya.Tool, map[string]kubiya.Teammate, error) {
	toolsMap := make(map[string][]kubiya.Tool, 0)
	teammateMap := make(map[string]kubiya.Teammate, 0)
	for _, teammate := range teammates {
		teammateMap[teammate.UUID] = teammate
		for _, sourceid := range teammate.Sources {
			// TODO: fetch these metadatas in parallel
			sourceMetadata, _ := cli.GetSourceMetadata(ctx, sourceid)
			toolsMap[teammate.UUID] = append(toolsMap[teammate.UUID], sourceMetadata.Tools...)
		}
	}
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
	return newToolName
}

func kubiyaToolMcpWrapper(ctx context.Context, cli *kubiya.Client, teammate kubiya.Teammate, tool kubiya.Tool) server.ServerTool {
	handler := func(ctx context.Context, request mcp.CallToolRequest) (*mcp.CallToolResult, error) {
		_, outp, err := executeKubiyaToolViaTeammate(ctx, cli, teammate, tool, nil) // TODO: pass args
		if err != nil {
			return nil, fmt.Errorf("failed to execute tool: %w", err)
		}
		return mcp.NewToolResultText(outp), nil
	}
	return server.ServerTool{
		Tool: mcp.NewTool(
			canonicalToolName(teammate.Name, tool.Name),
			mcp.WithDescription(tool.Description),
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
			cli := kubiya.NewClient(cfg)
			cmd.Context()
			teammates, err := getSpecificTeammates(cmd.Context(), cli)
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
	fmt.Println("Starting MCP server...")
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
