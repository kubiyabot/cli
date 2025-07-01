package mcp

import (
	"fmt"
	"log"

	"github.com/kubiyabot/cli/internal/config"
	"github.com/kubiyabot/cli/internal/kubiya"
	"github.com/mark3labs/mcp-go/server"
)

// Server wraps the Kubiya client and provides MCP tools
type Server struct {
	client       *kubiya.Client
	config       *config.Config
	server       *server.MCPServer
	serverConfig *Configuration
}

// NewServer creates a new MCP server instance
func NewServer(cfg *config.Config, serverConfig *Configuration) *Server {
	return &Server{
		client:       kubiya.NewClient(cfg),
		config:       cfg,
		serverConfig: serverConfig,
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

	return nil
}

// addPrompts registers MCP prompts
func (s *Server) addPrompts() error {
	prompts := NewPrompts()
	return prompts.Register(s.server)
}

// addResources registers MCP resources
func (s *Server) addResources() error {
	resources := NewResources(s.client)
	return resources.Register(s.server)
}