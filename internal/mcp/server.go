package mcp

import (
	"fmt"
	"log"

	"github.com/mark3labs/mcp-go/server"

	"github.com/kubiyabot/cli/internal/composer"
	"github.com/kubiyabot/cli/internal/config"
	"github.com/kubiyabot/cli/internal/kubiya"
)

// Server wraps the Kubiya client and provides MCP tools
type Server struct {
	client         *kubiya.Client
	composerClient *composer.Client
	config         *config.Config
	server         *server.MCPServer
	serverConfig   *Configuration
}

// NewServer creates a new MCP server instance
func NewServer(cfg *config.Config, serverConfig *Configuration) *Server {

	_ = newWFDslWasmPlugin()

	return &Server{
		client:         kubiya.NewClient(cfg),
		composerClient: composer.NewClient(cfg),
		config:         cfg,
		serverConfig:   serverConfig,
	}
}

// Start initializes and starts the MCP server
func (s *Server) Start() error {
	// Create MCP server with capabilities
	mcpServer := server.NewMCPServer(
		"Kubiya MCP Server",
		"1.0.0",
		server.WithToolCapabilities(true),
		server.WithPromptCapabilities(true),
		server.WithResourceCapabilities(true, true),
		server.WithLogging(),
	)

	s.server = mcpServer

	// Add tools
	if err := s.addTools(); err != nil {
		return fmt.Errorf("failed to add tools: %w", err)
	}

	// Add prompts
	if err := s.addPrompts(); err != nil {
		return fmt.Errorf("failed to add prompts: %w", err)
	}

	// Add resources
	if err := s.addResources(); err != nil {
		return fmt.Errorf("failed to add resources: %w", err)
	}

	// Start server
	log.Println("Starting Kubiya MCP Server...")
	return server.ServeStdio(mcpServer)
}

// addTools registers all available tools
func (s *Server) addTools() error {
	// If whitelist is configured, only add whitelisted tools
	if len(s.serverConfig.WhitelistedTools) > 0 {
		log.Printf("Using whitelist mode - only adding %d whitelisted tools", len(s.serverConfig.WhitelistedTools))
		return s.addWhitelistedToolsOnly()
	}

	// Default behavior: add all tools based on configuration
	log.Println("Using default mode - adding all configured tools")

	// Core tool execution
	if err := s.addExecuteTool(); err != nil {
		return err
	}

	// Core platform tools (always available)
	if err := s.addCorePlatformTools(); err != nil {
		return err
	}

	// Advanced platform APIs (if enabled)
	if s.serverConfig.AllowPlatformAPIs {
		if err := s.addAdvancedPlatformTools(); err != nil {
			return err
		}
	}

	// Knowledge base tools
	if err := s.addKnowledgeBaseTools(); err != nil {
		return err
	}

	if s.serverConfig.EnableDocumentation {
		if err := s.addDocumentationTools(); err != nil {
			return fmt.Errorf("failed to add documentation tools: %w", err)
		}
	}

	return nil
}

// addPrompts registers MCP prompts
func (s *Server) addPrompts() error {
	prompts := NewPrompts()
	return prompts.Register(s.server)
}

// addResources registers MCP resources
func (s *Server) addResources() error {
	resources := NewResources(s.client, s.composerClient)
	return resources.Register(s.server)
}

// addWhitelistedToolsOnly registers only the whitelisted tools as individual MCP tools
func (s *Server) addWhitelistedToolsOnly() error {
	for _, tool := range s.serverConfig.WhitelistedTools {
		log.Printf("Adding whitelisted tool: %s (%s)", tool.Name, tool.Description)

		// Convert WhitelistedTool to Kubiya Tool format
		kubiyaTool := kubiya.Tool{
			Name:        tool.Name,
			Source:      kubiya.ToolSource{ID: tool.Source.ID, URL: tool.Source.URL},
			Description: tool.Description,
			Args:        convertToolArgs(tool.Args),
			Env:         tool.Env,
			Content:     tool.Content,
			FileName:    tool.FileName,
			Secrets:     tool.Secrets,
			IconURL:     tool.IconURL,
			Type:        tool.Type,
			Alias:       tool.Alias,
			WithFiles:   tool.WithFiles,
			WithVolumes: tool.WithVolumes,
			LongRunning: tool.LongRunning,
			Metadata:    tool.Metadata,
			Mermaid:     tool.Mermaid,
		}

		// Create the tool handler that executes the whitelisted tool
		handler := NewWhitelistedToolHandler(s.client, tool, kubiyaTool)

		// Register the tool as an individual MCP tool
		if err := handler.Register(s.server); err != nil {
			return fmt.Errorf("failed to register whitelisted tool %s: %w", tool.Name, err)
		}
	}

	return nil
}

// convertToolArgs converts MCP ToolArg to Kubiya ToolArg
func convertToolArgs(mcpArgs []ToolArg) []kubiya.ToolArg {
	args := make([]kubiya.ToolArg, len(mcpArgs))
	for i, arg := range mcpArgs {
		args[i] = kubiya.ToolArg{
			Name:        arg.Name,
			Type:        arg.Type,
			Description: arg.Description,
			Required:    arg.Required,
			Default:     arg.Default,
			Options:     arg.Options,
			// OptionsFrom conversion would need proper mapping if needed
		}
	}
	return args
}
