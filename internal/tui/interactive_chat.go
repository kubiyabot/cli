package tui

import (
	"bufio"
	"context"
	"encoding/json"
	"fmt"
	"os"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/kubiyabot/cli/internal/config"
	"github.com/kubiyabot/cli/internal/kubiya"
)

// ToolExecution tracks tool call state
type ToolExecution struct {
	name              string
	status            string
	output            string
	startTime         time.Time
	endTime           time.Time
	failed            bool
	errorMsg          string
	outputTruncated   bool
	isComplete        bool
	hasShownOutput    bool
	animationStarted  bool
}

// ToolCallStats tracks tool execution statistics  
type ToolCallStats struct {
	mu             sync.RWMutex
	totalCalls     int
	activeCalls    int
	completedCalls int
	failedCalls    int
}

// InteractiveChatUI - A truly interactive chat interface that works everywhere
type InteractiveChatUI struct {
	cfg    *config.Config
	client *kubiya.Client
	
	// Context
	ctx    context.Context
	cancel context.CancelFunc
	
	// Tool tracking
	toolExecutions map[string]*ToolExecution
	toolStats      *ToolCallStats
	showToolCalls  bool
}

// NewInteractiveChatUI creates a new interactive chat UI
func NewInteractiveChatUI(cfg *config.Config) *InteractiveChatUI {
	ctx, cancel := context.WithCancel(context.Background())
	
	return &InteractiveChatUI{
		cfg:            cfg,
		client:         kubiya.NewClient(cfg),
		ctx:            ctx,
		cancel:         cancel,
		toolExecutions: make(map[string]*ToolExecution),
		toolStats:      &ToolCallStats{},
		showToolCalls:  true, // Default to showing tool calls
	}
}

// Run starts the interactive chat
func (ui *InteractiveChatUI) Run() error {
	defer ui.cancel()
	
	// Clear screen and show header
	fmt.Print("\033[2J\033[H") // Clear screen and move to top
	ui.printHeader()
	
	// Load and display agents
	fmt.Println("🔄 Loading agents...")
	agents, err := ui.loadAgents()
	if err != nil {
		fmt.Printf("❌ Error loading agents: %v\n", err)
		return err
	}
	
	if len(agents) == 0 {
		fmt.Println("❌ No agents available")
		return fmt.Errorf("no agents found")
	}
	
	// Show agent selection
	selectedAgent, err := ui.selectAgent(agents)
	if err != nil {
		return err
	}
	
	// Start chat with selected agent
	return ui.startChat(selectedAgent)
}

// printHeader displays the welcome header
func (ui *InteractiveChatUI) printHeader() {
	fmt.Println("🤖 ═══════════════════════════════════════════════════════════════")
	fmt.Println("🤖   KUBIYA INTERACTIVE CHAT")
	fmt.Println("🤖 ═══════════════════════════════════════════════════════════════")
	fmt.Println()
}

// loadAgents loads agents from the API
func (ui *InteractiveChatUI) loadAgents() ([]kubiya.Agent, error) {
	// Try main endpoint first
	agents, err := ui.client.ListAgents(ui.ctx)
	if err != nil {
		// Fallback to legacy endpoint
		if agents, err = ui.client.ListAgentsLegacy(ui.ctx); err != nil {
			return nil, fmt.Errorf("failed to load agents: %w", err)
		}
	}
	
	// Filter valid agents
	validAgents := make([]kubiya.Agent, 0)
	for _, agent := range agents {
		if agent.UUID != "" && agent.Name != "" {
			validAgents = append(validAgents, agent)
		}
	}
	
	return validAgents, nil
}

// selectAgent shows agent list and handles selection
func (ui *InteractiveChatUI) selectAgent(agents []kubiya.Agent) (*kubiya.Agent, error) {
	reader := bufio.NewReader(os.Stdin)
	
	for {
		// Clear screen and show header
		fmt.Print("\033[2J\033[H")
		ui.printHeader()
		
		// Show agents
		fmt.Println("📋 Available Agents:")
		fmt.Println("━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━")
		
		for i, agent := range agents {
			desc := agent.Description
			if desc == "" {
				desc = agent.Desc
			}
			
			// Truncate long descriptions
			if len(desc) > 80 {
				desc = desc[:77] + "..."
			}
			
			fmt.Printf("%2d. %-20s - %s\n", i+1, agent.Name, desc)
		}
		
		fmt.Println("━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━")
		fmt.Printf("\n🎯 Select an agent (1-%d) or 'q' to quit: ", len(agents))
		
		// Read user input
		input, err := reader.ReadString('\n')
		if err != nil {
			return nil, fmt.Errorf("failed to read input: %w", err)
		}
		
		input = strings.TrimSpace(input)
		
		// Handle quit
		if strings.ToLower(input) == "q" || strings.ToLower(input) == "quit" {
			fmt.Println("👋 Goodbye!")
			return nil, fmt.Errorf("user quit")
		}
		
		// Parse selection
		selection, err := strconv.Atoi(input)
		if err != nil || selection < 1 || selection > len(agents) {
			fmt.Printf("❌ Invalid selection. Please enter a number between 1 and %d.\n", len(agents))
			fmt.Print("Press Enter to continue...")
			reader.ReadString('\n')
			continue
		}
		
		selectedAgent := &agents[selection-1]
		fmt.Printf("✅ Selected: %s\n", selectedAgent.Name)
		fmt.Print("Press Enter to start chatting...")
		reader.ReadString('\n')
		
		return selectedAgent, nil
	}
}

// startChat begins the conversation with the selected agent
func (ui *InteractiveChatUI) startChat(agent *kubiya.Agent) error {
	reader := bufio.NewReader(os.Stdin)
	
	// Generate a proper UUID for session ID (empty string will auto-generate one)
	var sessionID string
	
	// Clear screen and show chat header
	fmt.Print("\033[2J\033[H")
	ui.printChatHeader(agent)
	
	fmt.Printf("💬 Starting enhanced chat with %s...\n", agent.Name)
	fmt.Println("━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━")
	fmt.Println("📝 Type your messages below. Features available:")
	fmt.Println("   • 'quit/exit/q' - End the chat")
	fmt.Println("   • 'clear/cls' - Clear chat history") 
	fmt.Println("   • 'tools' - Toggle tool call visibility")
	fmt.Println("   • 'stats' - Show tool execution statistics")
	fmt.Println("   • 'help' - Show all commands")
	fmt.Printf("🔧 Tool calls: %s | Streaming: ✅ ENABLED\n", ui.getToolVisibilityStatus())
	fmt.Println("━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━")
	fmt.Println()
	
	for {
		// Show prompt
		fmt.Print("💭 You: ")
		
		// Read user input
		message, err := reader.ReadString('\n')
		if err != nil {
			fmt.Printf("❌ Error reading input: %v\n", err)
			continue
		}
		
		message = strings.TrimSpace(message)
		
		// Handle special commands
		switch strings.ToLower(message) {
		case "quit", "exit", "q":
			fmt.Println("👋 Chat ended. Thanks for using Kubiya!")
			return nil
			
		case "clear", "cls":
			fmt.Print("\033[2J\033[H")
			ui.printChatHeader(agent)
			fmt.Println("🗑️  Chat history cleared.")
			fmt.Println()
			// Reset session ID and tool tracking to start fresh
			sessionID = ""
			ui.toolExecutions = make(map[string]*ToolExecution)
			ui.toolStats = &ToolCallStats{}
			continue
			
		case "tools":
			ui.showToolCalls = !ui.showToolCalls
			status := ui.getToolVisibilityStatus()
			fmt.Printf("🔧 Tool call visibility: %s\n", status)
			fmt.Println()
			continue
			
		case "stats":
			ui.showToolStats()
			continue
			
		case "help":
			ui.showChatHelp()
			continue
			
		case "":
			continue // Skip empty messages
		}
		
		// Show that we're processing
		fmt.Printf("🤖 %s is thinking...\n", agent.Name)
		
		// Send message to agent and update sessionID
		newSessionID, err := ui.sendAndReceiveMessage(agent, message, sessionID)
		if err != nil {
			fmt.Printf("❌ Error communicating with agent: %v\n", err)
			fmt.Println("🔄 You can try again or type 'quit' to exit.")
		} else {
			// Update session ID for continuity
			if newSessionID != "" {
				sessionID = newSessionID
			}
		}
		
		fmt.Println()
	}
}

// printChatHeader shows the chat header
func (ui *InteractiveChatUI) printChatHeader(agent *kubiya.Agent) {
	fmt.Println("💬 ═══════════════════════════════════════════════════════════════")
	fmt.Printf("💬   CHATTING WITH %s\n", strings.ToUpper(agent.Name))
	fmt.Println("💬 ═══════════════════════════════════════════════════════════════")
	fmt.Println()
}

// showChatHelp displays available commands with enhanced features
func (ui *InteractiveChatUI) showChatHelp() {
	fmt.Println()
	fmt.Println("🆘 ═══════════════ HELP ═══════════════")
	fmt.Println("📝 Type your message and press Enter")
	fmt.Println("🚪 quit/exit/q     - End the chat")
	fmt.Println("🗑️  clear/cls       - Clear chat history")
	fmt.Println("🔧 tools           - Toggle tool call visibility")
	fmt.Println("📊 stats           - Show tool execution statistics")
	fmt.Println("🆘 help            - Show this help")
	fmt.Println("═════════════════════════════════════════")
	fmt.Printf("💡 Tool calls visibility: %s\n", ui.getToolVisibilityStatus())
	fmt.Println("═════════════════════════════════════════")
	fmt.Println()
}

// getToolVisibilityStatus returns the current tool visibility status
func (ui *InteractiveChatUI) getToolVisibilityStatus() string {
	if ui.showToolCalls {
		return "ON (detailed)"
	}
	return "OFF (silent)"
}

// showToolStats displays current tool execution statistics
func (ui *InteractiveChatUI) showToolStats() {
	fmt.Println()
	fmt.Println("📊 ═══════════════ TOOL STATISTICS ═══════════════")
	
	ui.toolStats.mu.RLock()
	totalCalls := ui.toolStats.totalCalls
	activeCalls := ui.toolStats.activeCalls
	completedCalls := ui.toolStats.completedCalls
	failedCalls := ui.toolStats.failedCalls
	ui.toolStats.mu.RUnlock()
	
	fmt.Printf("📈 Total Tools Called: %d\n", totalCalls)
	fmt.Printf("🔄 Currently Active: %d\n", activeCalls)
	fmt.Printf("✅ Completed: %d\n", completedCalls)
	fmt.Printf("❌ Failed: %d\n", failedCalls)
	
	if totalCalls > 0 {
		successRate := float64(completedCalls) / float64(totalCalls) * 100
		fmt.Printf("📊 Success Rate: %.1f%%\n", successRate)
	}
	
	// Show active tool executions
	if len(ui.toolExecutions) > 0 {
		fmt.Println("\n🔧 Active Tool Executions:")
		for messageID, te := range ui.toolExecutions {
			if !te.isComplete {
				duration := time.Since(te.startTime).Seconds()
				fmt.Printf("   %s (%s) - running for %.1fs\n", te.name, messageID[:8], duration)
			}
		}
	}
	
	fmt.Println("═══════════════════════════════════════════════════")
	fmt.Println()
}

// sendAndReceiveMessage handles the message exchange with advanced streaming and tool call support
func (ui *InteractiveChatUI) sendAndReceiveMessage(agent *kubiya.Agent, message, sessionID string) (string, error) {
	// Create timeout context  
	ctx, cancel := context.WithTimeout(ui.ctx, 120*time.Second)
	defer cancel()
	
	// Send message with retry support
	msgChan, err := ui.client.SendMessageWithRetry(ctx, agent.UUID, message, sessionID, 3)
	if err != nil {
		return "", fmt.Errorf("failed to send message: %w", err)
	}
	
	fmt.Printf("🤖 %s: ", agent.Name)
	
	// Track response state
	hasResponse := false
	actualSessionID := sessionID
	var responseBuilder strings.Builder
	toolsExecuted := false
	anyOutputTruncated := false
	hasError := false
	
	// Process streaming messages with advanced handling
	for {
		select {
		case msg, ok := <-msgChan:
			if !ok {
				// Channel closed - show final summary
				if !hasResponse {
					fmt.Println("(No response received)")
				} else {
					fmt.Println() // End the response
					ui.showFinalSummary(toolsExecuted, anyOutputTruncated, hasError)
				}
				return actualSessionID, nil
			}
			
			hasResponse = true
			
			// Update session ID if provided
			if msg.SessionID != "" {
				actualSessionID = msg.SessionID
			}
			
			// Handle different message types with enhanced logic
			switch msg.Type {
			case "message", "completion":
				ui.handleRegularMessage(msg, &responseBuilder)
				
			case "tool_call":
				toolsExecuted = true
				ui.handleToolCall(msg, &anyOutputTruncated, &hasError)
				
			case "tool_output":
				ui.handleToolOutput(msg, &anyOutputTruncated, &hasError)
				
			case "error":
				hasError = true
				fmt.Printf("\n❌ Error: %s\n", msg.Error)
				if msg.Final {
					return actualSessionID, fmt.Errorf("agent error: %s", msg.Error)
				}
			}
			
			// Check if this is the final message
			if msg.Final {
				fmt.Println() // Add final newline
				ui.showFinalSummary(toolsExecuted, anyOutputTruncated, hasError)
				return actualSessionID, nil
			}
			
		case <-ctx.Done():
			if !hasResponse {
				fmt.Println("⏱️ Request timed out")
			}
			return actualSessionID, fmt.Errorf("timeout waiting for response")
		}
	}
}

// handleRegularMessage processes regular chat messages with streaming
func (ui *InteractiveChatUI) handleRegularMessage(msg kubiya.ChatMessage, responseBuilder *strings.Builder) {
	if msg.Content != "" {
		// Handle streaming content with differential updates
		newContent := msg.Content
		if len(newContent) > responseBuilder.Len() {
			// Extract only the new part
			newPart := newContent[responseBuilder.Len():]
			fmt.Print(newPart)
			responseBuilder.WriteString(newPart)
		}
	}
}

// handleToolCall processes tool call initiation
func (ui *InteractiveChatUI) handleToolCall(msg kubiya.ChatMessage, anyOutputTruncated *bool, hasError *bool) {
	if !ui.showToolCalls {
		return
	}
	
	// Parse tool name from content
	toolName := "unknown"
	toolArgs := ""
	
	if msg.Content != "" {
		parts := strings.SplitN(msg.Content, ":", 2)
		if len(parts) > 0 {
			toolName = strings.TrimSpace(parts[0])
		}
		if len(parts) > 1 {
			toolArgs = strings.TrimSpace(parts[1])
		}
	}
	
	// Check if this is a new tool call
	te, exists := ui.toolExecutions[msg.MessageID]
	if !exists {
		// Create new tool execution
		te = &ToolExecution{
			name:      toolName,
			status:    "starting",
			startTime: time.Now(),
		}
		ui.toolExecutions[msg.MessageID] = te
		
		// Update stats
		ui.toolStats.mu.Lock()
		ui.toolStats.totalCalls++
		ui.toolStats.activeCalls++
		ui.toolStats.mu.Unlock()
		
		// Show tool call initiation
		fmt.Printf("\n🔧 %s", toolName)
		if toolArgs != "" {
			paramSummary := ui.formatToolParameters(toolArgs)
			if paramSummary != "" {
				fmt.Printf("\n   %s", paramSummary)
			}
		}
		fmt.Printf(" %s\n", "🔄 starting...")
	}
}

// handleToolOutput processes tool execution output
func (ui *InteractiveChatUI) handleToolOutput(msg kubiya.ChatMessage, anyOutputTruncated *bool, hasError *bool) {
	if !ui.showToolCalls {
		return
	}
	
	te, exists := ui.toolExecutions[msg.MessageID]
	if !exists {
		return
	}
	
	// Update status to running
	if te.status == "starting" {
		te.status = "running"
		fmt.Printf("⚡ %s %s\n", te.name, "🔄 executing...")
	}
	
	// Process output content
	if msg.Content != "" {
		trimmedContent := strings.TrimSpace(msg.Content)
		
		// Check for errors in output
		if strings.Contains(strings.ToLower(trimmedContent), "error") || 
		   strings.Contains(strings.ToLower(trimmedContent), "failed") {
			te.failed = true
			te.errorMsg = trimmedContent
			*hasError = true
		}
		
		// Check for empty output
		if trimmedContent == "" || trimmedContent == "\"\"" || trimmedContent == "{}" {
			te.outputTruncated = true
			*anyOutputTruncated = true
		} else {
			// Show output preview (first 100 chars)
			if !te.hasShownOutput {
				preview := trimmedContent
				if len(preview) > 100 {
					preview = preview[:97] + "..."
				}
				fmt.Printf("   %s\n", preview)
				te.hasShownOutput = true
			}
		}
	}
	
	// Check if tool is complete
	if msg.Final || strings.Contains(strings.ToLower(msg.Content), "completed") {
		te.isComplete = true
		te.endTime = time.Now()
		
		// Update stats
		ui.toolStats.mu.Lock()
		ui.toolStats.activeCalls--
		if te.failed {
			ui.toolStats.failedCalls++
		} else {
			ui.toolStats.completedCalls++
		}
		ui.toolStats.mu.Unlock()
		
		// Show completion status
		duration := te.endTime.Sub(te.startTime).Seconds()
		status := "completed"
		emoji := "✅"
		if te.failed {
			status = "failed"
			emoji = "❌"
		}
		fmt.Printf("%s %s (%s %.1fs)\n", emoji, te.name, status, duration)
	}
}

// formatToolParameters creates a readable summary of tool parameters
func (ui *InteractiveChatUI) formatToolParameters(toolArgs string) string {
	if toolArgs == "" {
		return ""
	}
	
	// Try to parse as JSON first
	var params map[string]interface{}
	if err := json.Unmarshal([]byte(toolArgs), &params); err == nil {
		var parts []string
		for key, value := range params {
			valStr := fmt.Sprintf("%v", value)
			if len(valStr) > 50 {
				valStr = valStr[:47] + "..."
			}
			parts = append(parts, fmt.Sprintf("%s: %s", key, valStr))
		}
		if len(parts) > 0 {
			return strings.Join(parts, ", ")
		}
	}
	
	// Fallback to raw args with truncation
	if len(toolArgs) > 100 {
		return toolArgs[:97] + "..."
	}
	return toolArgs
}

// showFinalSummary displays a summary after the conversation
func (ui *InteractiveChatUI) showFinalSummary(toolsExecuted bool, anyOutputTruncated bool, hasError bool) {
	if !ui.showToolCalls || !toolsExecuted {
		return
	}
	
	ui.toolStats.mu.RLock()
	totalCalls := ui.toolStats.totalCalls
	completedCalls := ui.toolStats.completedCalls
	failedCalls := ui.toolStats.failedCalls
	ui.toolStats.mu.RUnlock()
	
	fmt.Printf("\n📊 Tool Summary: %d total, %d completed, %d failed", 
		totalCalls, completedCalls, failedCalls)
	
	if anyOutputTruncated {
		fmt.Printf(" (some output truncated)")
	}
	fmt.Println()
}

// wrapText wraps text to the specified width
func (ui *InteractiveChatUI) wrapText(text string, width int) []string {
	words := strings.Fields(text)
	if len(words) == 0 {
		return []string{}
	}
	
	var lines []string
	var currentLine strings.Builder
	
	for _, word := range words {
		// If adding this word would exceed the width, start a new line
		if currentLine.Len() > 0 && currentLine.Len()+1+len(word) > width {
			lines = append(lines, currentLine.String())
			currentLine.Reset()
		}
		
		// Add word to current line
		if currentLine.Len() > 0 {
			currentLine.WriteString(" ")
		}
		currentLine.WriteString(word)
	}
	
	// Add the last line
	if currentLine.Len() > 0 {
		lines = append(lines, currentLine.String())
	}
	
	return lines
}

// Cleanup releases resources
func (ui *InteractiveChatUI) Cleanup() {
	ui.cancel()
}