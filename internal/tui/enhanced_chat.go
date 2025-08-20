package tui

import (
	"context"
	"fmt"
	"strings"
	"sync"
	"time"

	"github.com/charmbracelet/bubbles/spinner"
	"github.com/charmbracelet/bubbles/textarea"
	"github.com/charmbracelet/bubbles/viewport"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
	"github.com/kubiyabot/cli/internal/config"
	"github.com/kubiyabot/cli/internal/kubiya"
)

// Enhanced Chat Styles
var (
	enhancedHeaderStyle = lipgloss.NewStyle().
		Bold(true).
		Foreground(lipgloss.Color("#7C3AED")).
		Background(lipgloss.Color("#1F1B24")).
		Padding(0, 1).
		MarginBottom(1)

	enhancedAgentHeaderStyle = lipgloss.NewStyle().
		Bold(true).
		Foreground(lipgloss.Color("#10B981")).
		Background(lipgloss.Color("#064E3B")).
		Padding(0, 2).
		MarginBottom(1)

	enhancedMessageStyle = lipgloss.NewStyle().
		Padding(0, 1).
		MarginBottom(1)

	enhancedUserMessageStyle = enhancedMessageStyle.Copy().
		Foreground(lipgloss.Color("#3B82F6")).
		Bold(true)

	enhancedAgentMessageStyle = enhancedMessageStyle.Copy().
		Foreground(lipgloss.Color("#10B981"))

	enhancedToolCallStyle = lipgloss.NewStyle().
		Foreground(lipgloss.Color("#F59E0B")).
		Background(lipgloss.Color("#78350F")).
		Padding(0, 1).
		MarginLeft(2).
		MarginBottom(1)

	enhancedToolSuccessStyle = lipgloss.NewStyle().
		Foreground(lipgloss.Color("#10B981")).
		Background(lipgloss.Color("#064E3B")).
		Padding(0, 1).
		MarginLeft(2)

	enhancedToolErrorStyle = lipgloss.NewStyle().
		Foreground(lipgloss.Color("#EF4444")).
		Background(lipgloss.Color("#7F1D1D")).
		Padding(0, 1).
		MarginLeft(2)

	enhancedInputStyle = lipgloss.NewStyle().
		Border(lipgloss.RoundedBorder()).
		BorderForeground(lipgloss.Color("#7C3AED")).
		Padding(1).
		MarginTop(1)

	enhancedStatusStyle = lipgloss.NewStyle().
		Foreground(lipgloss.Color("#6B7280")).
		Italic(true)

	enhancedErrorStyle = lipgloss.NewStyle().
		Foreground(lipgloss.Color("#EF4444")).
		Bold(true)

	enhancedSuccessStyle = lipgloss.NewStyle().
		Foreground(lipgloss.Color("#10B981")).
		Bold(true)
)

// Message buffer for differential updates (like CLI)
type chatBuffer struct {
	content     string
	sentence    strings.Builder
	codeBlock   strings.Builder
	inCodeBlock bool
}

// Enhanced Chat Model
type EnhancedChatModel struct {
	cfg    *config.Config
	client *kubiya.Client

	// UI State
	width, height int
	ready         bool
	err           error

	// Current state
	state enhancedChatState

	// Components
	agentList   []kubiya.Agent
	viewport    viewport.Model
	textarea    textarea.Model
	spinner     spinner.Model

	// Chat state
	selectedAgent      *kubiya.Agent
	messages           []ChatDisplayMessage
	sessionID          string
	isStreaming        bool
	streamingContent   string
	toolExecutions     map[string]*ToolExecutionState
	toolStats          *ToolStatistics
	showToolCalls      bool
	
	// Message buffering (like CLI)
	messageBuffer      map[string]*chatBuffer
	messageMutex       sync.RWMutex
	
	// Context
	ctx         context.Context
	cancel      context.CancelFunc
}

type enhancedChatState int

const (
	enhancedStateLoading enhancedChatState = iota
	enhancedStateAgentSelect
	enhancedStateChat
	enhancedStateError
)

type ChatDisplayMessage struct {
	Content   string
	IsUser    bool
	Timestamp time.Time
	Type      string
}

type ToolExecutionState struct {
	Name           string
	Status         string // "starting", "running", "completed", "failed"
	StartTime      time.Time
	EndTime        time.Time
	Output         string
	Error          string
	Args           string
	Duration       time.Duration
	OutputTruncated bool
}

type ToolStatistics struct {
	mu             sync.RWMutex
	TotalCalls     int
	ActiveCalls    int
	CompletedCalls int
	FailedCalls    int
}

// Messages for Bubble Tea - Enhanced Chat
type enhancedAgentsLoadedMsg []kubiya.Agent
type enhancedStreamingMsg kubiya.ChatMessage
type enhancedStreamCompleteMsg struct{}
type enhancedErrorMsg error

// NewEnhancedChatModel creates a new enhanced chat model
func NewEnhancedChatModel(cfg *config.Config) *EnhancedChatModel {
	ctx, cancel := context.WithCancel(context.Background())

	// Create spinner
	s := spinner.New()
	s.Spinner = spinner.Dot
	s.Style = lipgloss.NewStyle().Foreground(lipgloss.Color("#7C3AED"))

	// Create textarea
	ta := textarea.New()
	ta.Placeholder = "💭 Type your message here... (Press Enter to send, Esc to go back)"
	ta.ShowLineNumbers = false
	ta.CharLimit = 2000
	ta.SetWidth(50)
	ta.SetHeight(3)
	ta.KeyMap.InsertNewline.SetEnabled(false)  // Disable multi-line, Enter sends message
	ta.Focus()

	// Create viewport
	vp := viewport.New(0, 0)

	return &EnhancedChatModel{
		cfg:            cfg,
		client:         kubiya.NewClient(cfg),
		state:          enhancedStateLoading,
		spinner:        s,
		textarea:       ta,
		viewport:       vp,
		ctx:            ctx,
		cancel:         cancel,
		messages:       make([]ChatDisplayMessage, 0),
		toolExecutions: make(map[string]*ToolExecutionState),
		toolStats:      &ToolStatistics{},
		showToolCalls:  true,
		messageBuffer:  make(map[string]*chatBuffer),
	}
}

// Init initializes the model
func (m *EnhancedChatModel) Init() tea.Cmd {
	return tea.Batch(
		m.spinner.Tick,
		m.loadAgents(),
	)
}

// Update handles all state updates
func (m *EnhancedChatModel) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	var cmd tea.Cmd
	var cmds []tea.Cmd

	switch msg := msg.(type) {
	case tea.WindowSizeMsg:
		m.width = msg.Width
		m.height = msg.Height
		m.updateComponentSizes()

	case enhancedAgentsLoadedMsg:
		m.agentList = []kubiya.Agent(msg)
		m.ready = true
		m.state = enhancedStateAgentSelect

	// Handle UI refresh during streaming
	case struct{ refreshUI bool }:
		// Just refresh the UI - streaming updates happen in the goroutine
		if m.isStreaming {
			return m, tea.Tick(100*time.Millisecond, func(t time.Time) tea.Msg {
				if m.isStreaming {
					return struct{ refreshUI bool }{true}
				}
				return nil
			})
		}
		
	case enhancedErrorMsg:
		m.err = error(msg)
		m.state = enhancedStateError

	case tea.KeyMsg:
		return m.handleKeyMsg(msg)

	case spinner.TickMsg:
		m.spinner, cmd = m.spinner.Update(msg)
		cmds = append(cmds, cmd)
		
	default:
		// No additional default handling needed
	}

	// Update viewport component (always safe)
	m.viewport, cmd = m.viewport.Update(msg)
	cmds = append(cmds, cmd)

	return m, tea.Batch(cmds...)
}

// View renders the model
func (m *EnhancedChatModel) View() string {
	if m.width == 0 || m.height == 0 {
		return "Loading..."
	}

	switch m.state {
	case enhancedStateLoading:
		return m.renderLoading()
	case enhancedStateError:
		return m.renderError()
	case enhancedStateAgentSelect:
		return m.renderAgentSelection()
	case enhancedStateChat:
		return m.renderChat()
	}

	return "Unknown state"
}

// Render functions
func (m *EnhancedChatModel) renderLoading() string {
	content := fmt.Sprintf("Loading agents... %s", m.spinner.View())
	return lipgloss.Place(m.width, m.height, lipgloss.Center, lipgloss.Center, content)
}

func (m *EnhancedChatModel) renderError() string {
	content := enhancedErrorStyle.Render(fmt.Sprintf("Error: %v\n\nPress 'r' to retry or 'q' to quit", m.err))
	return lipgloss.Place(m.width, m.height, lipgloss.Center, lipgloss.Center, content)
}

func (m *EnhancedChatModel) renderAgentSelection() string {
	var content strings.Builder
	
	// Header
	header := enhancedHeaderStyle.Width(m.width).Render("🤖 Select a Kubiya Agent")
	content.WriteString(header + "\n\n")

	// Agent list with rich information
	for i, agent := range m.agentList {
		// Agent name and basic info
		agentHeader := fmt.Sprintf("%2d. %s", i+1, agent.Name)
		content.WriteString(enhancedAgentHeaderStyle.Render(agentHeader) + "\n")
		
		// Description
		desc := agent.Description
		if desc == "" {
			desc = agent.Desc
		}
		if desc != "" {
			if len(desc) > 100 {
				desc = desc[:97] + "..."
			}
			content.WriteString("   " + enhancedStatusStyle.Render("📝 "+desc) + "\n")
		}
		
		// Runner information with icons
		if len(agent.Runners) > 0 {
			runner := agent.Runners[0]
			runnerIcon := m.getRunnerIcon(runner)
			content.WriteString("   " + enhancedStatusStyle.Render(fmt.Sprintf("%s Runner: %s", runnerIcon, runner)) + "\n")
		}
		
		// Integrations with icons
		if len(agent.Integrations) > 0 {
			integrationsList := make([]string, 0)
			for _, integration := range agent.Integrations {
				if len(integrationsList) < 5 { // Show max 5 integrations
					icon := m.getIntegrationIcon(integration)
					integrationsList = append(integrationsList, icon+integration)
				}
			}
			if len(integrationsList) > 0 {
				content.WriteString("   " + enhancedStatusStyle.Render("🔌 Integrations: "+strings.Join(integrationsList, ", ")) + "\n")
			}
		}
		
		// Tools preview
		if len(agent.Tools) > 0 {
			toolCount := len(agent.Tools)
			toolPreview := fmt.Sprintf("🛠️  %d tools available", toolCount)
			if toolCount <= 3 {
				// Show tool names if few tools
				toolNames := make([]string, 0)
				for _, toolName := range agent.Tools {
					if toolName != "" {
						toolNames = append(toolNames, toolName)
					}
				}
				if len(toolNames) > 0 {
					toolPreview = "🛠️  Tools: " + strings.Join(toolNames, ", ")
				}
			}
			content.WriteString("   " + enhancedStatusStyle.Render(toolPreview) + "\n")
		}
		
		// Separator between agents
		content.WriteString("\n")
	}

	// Instructions
	instructions := enhancedStatusStyle.Render("Press 1-9 to select agent, Enter for first agent, or 'q' to quit")
	content.WriteString(instructions)

	return content.String()
}

// Helper functions for icons
func (m *EnhancedChatModel) getRunnerIcon(runner string) string {
	switch strings.ToLower(runner) {
	case "kubiyamanaged", "managed":
		return "☁️ "
	case "kubernetes", "k8s":
		return "🚢 "
	case "docker":
		return "🐳 "
	case "local":
		return "💻 "
	default:
		return "⚙️ "
	}
}

func (m *EnhancedChatModel) getIntegrationIcon(integration string) string {
	switch strings.ToLower(integration) {
	case "aws":
		return "🟠"
	case "azure":
		return "🔵"
	case "gcp", "google":
		return "🟡"
	case "kubernetes", "k8s":
		return "🚢"
	case "jira":
		return "🎯"
	case "slack":
		return "💬"
	case "github":
		return "🐙"
	case "gitlab":
		return "🦊"
	case "jenkins":
		return "🔨"
	case "terraform":
		return "🏗️"
	case "ansible":
		return "🎭"
	case "docker":
		return "🐳"
	case "prometheus":
		return "📊"
	case "grafana":
		return "📈"
	case "datadog":
		return "🐕"
	default:
		return "🔧"
	}
}

func (m *EnhancedChatModel) renderChat() string {
	var content strings.Builder

	// Agent header
	if m.selectedAgent != nil {
		header := enhancedAgentHeaderStyle.Width(m.width).Render(fmt.Sprintf("💬 Chatting with %s", m.selectedAgent.Name))
		content.WriteString(header + "\n")
	}

	// Messages area
	m.viewport.SetContent(m.renderMessages())
	content.WriteString(m.viewport.View())
	content.WriteString("\n")

	// Input area
	inputArea := enhancedInputStyle.Width(m.width - 4).Render(m.textarea.View())
	content.WriteString(inputArea)

	// Status line
	statusLine := m.renderStatusLine()
	content.WriteString("\n" + statusLine)

	return content.String()
}

func (m *EnhancedChatModel) renderMessages() string {
	var content strings.Builder

	// Show conversation history with better formatting
	for i, msg := range m.messages {
		timestamp := msg.Timestamp.Format("15:04")
		
		if msg.IsUser {
			userHeader := enhancedUserMessageStyle.Render(fmt.Sprintf("👤 [%s] You", timestamp))
			content.WriteString(userHeader + "\n")
			messageContent := enhancedMessageStyle.Render(fmt.Sprintf("   %s", msg.Content))
			content.WriteString(messageContent + "\n\n")
		} else {
			agentName := "Agent"
			if m.selectedAgent != nil {
				agentName = m.selectedAgent.Name
			}
			agentHeader := enhancedAgentMessageStyle.Render(fmt.Sprintf("🤖 [%s] %s", timestamp, agentName))
			content.WriteString(agentHeader + "\n")
			messageContent := enhancedMessageStyle.Render(fmt.Sprintf("   %s", msg.Content))
			content.WriteString(messageContent)
			
			// Add spacing between messages, but not after the last one
			if i < len(m.messages)-1 {
				content.WriteString("\n\n")
			}
		}
	}

	// Show streaming content if active
	if m.isStreaming {
		// Add spacing before streaming content if there are previous messages
		if len(m.messages) > 0 {
			content.WriteString("\n\n")
		}
		
		agentName := "Agent"
		if m.selectedAgent != nil {
			agentName = m.selectedAgent.Name
		}
		timestamp := time.Now().Format("15:04")
		
		if m.streamingContent != "" {
			agentHeader := enhancedAgentMessageStyle.Render(fmt.Sprintf("🤖 [%s] %s", timestamp, agentName))
			content.WriteString(agentHeader + "\n")
			streamContent := enhancedMessageStyle.Render(fmt.Sprintf("   %s", m.streamingContent))
			content.WriteString(streamContent)
			content.WriteString(enhancedStatusStyle.Render(" ●")) // Streaming indicator
		} else {
			// Show thinking indicator when no content yet
			agentHeader := enhancedAgentMessageStyle.Render(fmt.Sprintf("🤖 [%s] %s", timestamp, agentName))
			content.WriteString(agentHeader + "\n")
			thinkingContent := enhancedStatusStyle.Render("   💭 Thinking...")
			content.WriteString(thinkingContent)
		}
		content.WriteString("\n")
	}

	// Show tool executions
	if m.showToolCalls {
		content.WriteString(m.renderToolExecutions())
	}

	return content.String()
}

func (m *EnhancedChatModel) renderToolExecutions() string {
	var content strings.Builder

	for _, te := range m.toolExecutions {
		var style lipgloss.Style
		var statusEmoji string

		switch te.Status {
		case "starting":
			style = enhancedToolCallStyle
			statusEmoji = "🔄"
		case "running":
			style = enhancedToolCallStyle
			statusEmoji = "⚡"
		case "completed":
			style = enhancedToolSuccessStyle
			statusEmoji = "✅"
		case "failed":
			style = enhancedToolErrorStyle
			statusEmoji = "❌"
		default:
			style = enhancedToolCallStyle
			statusEmoji = "🔧"
		}

		line := style.Render(fmt.Sprintf("%s %s", statusEmoji, te.Name))
		if te.Status == "completed" || te.Status == "failed" {
			line += fmt.Sprintf(" (%.1fs)", te.Duration.Seconds())
		}
		content.WriteString(line + "\n")

		// Show args if available
		if te.Args != "" && len(te.Args) < 100 {
			argsLine := enhancedStatusStyle.Render(fmt.Sprintf("   Args: %s", te.Args))
			content.WriteString(argsLine + "\n")
		}

		// Show error if failed
		if te.Status == "failed" && te.Error != "" {
			errorLine := enhancedToolErrorStyle.Render(fmt.Sprintf("   Error: %s", te.Error))
			content.WriteString(errorLine + "\n")
		}
	}

	return content.String()
}

func (m *EnhancedChatModel) renderStatusLine() string {
	var status strings.Builder

	// Streaming status
	if m.isStreaming {
		status.WriteString(enhancedStatusStyle.Render("🔄 Streaming... "))
	}

	// Tool stats
	if m.showToolCalls {
		m.toolStats.mu.RLock()
		stats := fmt.Sprintf("Tools: %d total, %d active, %d completed, %d failed", 
			m.toolStats.TotalCalls, m.toolStats.ActiveCalls, 
			m.toolStats.CompletedCalls, m.toolStats.FailedCalls)
		m.toolStats.mu.RUnlock()
		status.WriteString(enhancedStatusStyle.Render(stats))
	}

	// Commands
	status.WriteString(enhancedStatusStyle.Render(" | "))
	if m.isStreaming {
		status.WriteString(enhancedStatusStyle.Render("Streaming response... • Esc: agents • Ctrl+C: quit"))
	} else {
		status.WriteString(enhancedStatusStyle.Render("Enter: send • Esc: agents • Ctrl+C: quit"))
	}

	return status.String()
}

// Event handlers
func (m *EnhancedChatModel) handleKeyMsg(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	var cmd tea.Cmd
	
	switch m.state {
	case enhancedStateError:
		switch msg.String() {
		case "r":
			m.err = nil
			m.state = enhancedStateLoading
			return m, m.loadAgents()
		case "q", "ctrl+c":
			return m, tea.Quit
		}

	case enhancedStateAgentSelect:
		switch msg.String() {
		case "q", "ctrl+c":
			return m, tea.Quit
		case "1", "2", "3", "4", "5", "6", "7", "8", "9":
			idx := int(msg.String()[0] - '1')
			if idx < len(m.agentList) {
				m.selectedAgent = &m.agentList[idx]
				m.state = enhancedStateChat
				m.textarea.Focus()
				m.textarea.SetValue("") // Clear any previous content
				return m, nil
			}
		case "enter":
			// Handle enter on first agent
			if len(m.agentList) > 0 {
				m.selectedAgent = &m.agentList[0]
				m.state = enhancedStateChat
				m.textarea.Focus()
				m.textarea.SetValue("") // Clear any previous content
				return m, nil
			}
		}

	case enhancedStateChat:
		// Ensure textarea is focused when we're in chat state
		if !m.textarea.Focused() {
			m.textarea.Focus()
		}
		
		// Handle special keys first
		switch msg.String() {
		case "esc":
			m.state = enhancedStateAgentSelect
			m.textarea.Blur()
			return m, nil
		case "ctrl+c":
			return m, tea.Quit
		}
		
		// Handle textarea input - always process if we're in chat state
		m.textarea, cmd = m.textarea.Update(msg)
		
		// Check if user pressed enter to send message
		if msg.Type == tea.KeyEnter && !m.isStreaming {
			return m, tea.Batch(cmd, m.sendMessage())
		}
		
		return m, cmd
	}

	return m, nil
}

// Helper functions
func (m *EnhancedChatModel) updateComponentSizes() {
	if m.width > 0 && m.height > 0 {
		m.viewport.Width = m.width - 4
		m.viewport.Height = m.height - 12 // Leave space for input and status
		m.textarea.SetWidth(m.width - 8)
		m.textarea.SetHeight(3)
	}
}

func (m *EnhancedChatModel) loadAgents() tea.Cmd {
	return func() tea.Msg {
		// Try main endpoint first
		agents, err := m.client.ListAgents(m.ctx)
		if err != nil {
			// Fallback to legacy endpoint
			if legacyAgents, legacyErr := m.client.ListAgentsLegacy(m.ctx); legacyErr == nil {
				agents = legacyAgents
			} else {
				return enhancedErrorMsg(fmt.Errorf("failed to load agents: %w", err))
			}
		}

		// Filter valid agents
		validAgents := make([]kubiya.Agent, 0, len(agents))
		for _, agent := range agents {
			if agent.UUID != "" && agent.Name != "" {
				validAgents = append(validAgents, agent)
			}
		}

		if len(validAgents) == 0 {
			return enhancedErrorMsg(fmt.Errorf("no valid agents found"))
		}

		return enhancedAgentsLoadedMsg(validAgents)
	}
}

func (m *EnhancedChatModel) sendMessage() tea.Cmd {
	message := strings.TrimSpace(m.textarea.Value())
	if message == "" || m.selectedAgent == nil {
		return nil
	}

	// Add user message
	m.addMessage(message, true)

	// Clear textarea
	m.textarea.SetValue("")

	// Start streaming
	m.isStreaming = true
	m.streamingContent = ""

	// Ensure textarea stays focused during streaming
	
	return m.startStreaming(message)
}

func (m *EnhancedChatModel) startStreaming(message string) tea.Cmd {
	return tea.Batch(
		func() tea.Msg {
			// Start streaming in goroutine and use tick to update UI
			go func() {
				ctx, cancel := context.WithTimeout(m.ctx, 120*time.Second)
				defer cancel()

				// Get the stream channel directly
				msgChan, err := m.client.SendMessageWithRetry(ctx, m.selectedAgent.UUID, message, m.sessionID, 3)
				if err != nil {
					return
				}

				// Process messages directly and update model state
				for msg := range msgChan {
					// Update session ID
					if msg.SessionID != "" {
						m.sessionID = msg.SessionID
					}

					// Handle errors
					if msg.Error != "" {
						m.isStreaming = false
						return
					}

					// Process regular messages (like CLI)
					if msg.SenderName != "You" {
						m.messageMutex.Lock()
						buf, exists := m.messageBuffer[msg.MessageID]
						if !exists {
							buf = &chatBuffer{}
							m.messageBuffer[msg.MessageID] = buf
						}

						// Differential content updates
						if len(msg.Content) > len(buf.content) {
							m.streamingContent = msg.Content
							buf.content = msg.Content
						}
						m.messageMutex.Unlock()

						// Check if this is the final message
						if msg.Final {
							// Add the final content and stop streaming
							if m.streamingContent != "" {
								m.addMessage(m.streamingContent, false)
							}
							m.isStreaming = false
							m.streamingContent = ""
							return
						}
					}
				}

				// Stream ended
				if m.streamingContent != "" {
					m.addMessage(m.streamingContent, false)
				}
				m.isStreaming = false
			}()
			
			return nil
		},
		// Start ticker to refresh UI during streaming
		tea.Tick(100*time.Millisecond, func(t time.Time) tea.Msg {
			if m.isStreaming {
				return struct{ refreshUI bool }{true}
			}
			return nil
		}),
	)
}




func (m *EnhancedChatModel) handleToolCall(msg kubiya.ChatMessage) {
	if !m.showToolCalls {
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

	// Create or update tool execution
	if _, exists := m.toolExecutions[msg.MessageID]; !exists {
		m.toolExecutions[msg.MessageID] = &ToolExecutionState{
			Name:      toolName,
			Status:    "starting",
			StartTime: time.Now(),
			Args:      toolArgs,
		}

		// Update stats
		m.toolStats.mu.Lock()
		m.toolStats.TotalCalls++
		m.toolStats.ActiveCalls++
		m.toolStats.mu.Unlock()
	}
}

func (m *EnhancedChatModel) handleToolOutput(msg kubiya.ChatMessage) {
	te, exists := m.toolExecutions[msg.MessageID]
	if !exists || !m.showToolCalls {
		return
	}

	// Update status
	if te.Status == "starting" {
		te.Status = "running"
	}

	// Process output content
	if msg.Content != "" {
		te.Output = msg.Content

		// Check for errors
		if strings.Contains(strings.ToLower(msg.Content), "error") ||
			strings.Contains(strings.ToLower(msg.Content), "failed") {
			te.Status = "failed"
			te.Error = msg.Content
		}
	}

	// Check if complete
	if msg.Final {
		te.EndTime = time.Now()
		te.Duration = te.EndTime.Sub(te.StartTime)

		if te.Status != "failed" {
			te.Status = "completed"
		}

		// Update stats
		m.toolStats.mu.Lock()
		m.toolStats.ActiveCalls--
		if te.Status == "failed" {
			m.toolStats.FailedCalls++
		} else {
			m.toolStats.CompletedCalls++
		}
		m.toolStats.mu.Unlock()
	}
}

func (m *EnhancedChatModel) addMessage(content string, isUser bool) {
	msg := ChatDisplayMessage{
		Content:   content,
		IsUser:    isUser,
		Timestamp: time.Now(),
	}
	m.messages = append(m.messages, msg)
	m.viewport.GotoBottom()
}

// Cleanup releases resources
func (m *EnhancedChatModel) Cleanup() {
	m.cancel()
}

// Run starts the enhanced chat UI
func RunEnhancedChat(cfg *config.Config) error {
	model := NewEnhancedChatModel(cfg)
	defer model.Cleanup()

	p := tea.NewProgram(model, tea.WithAltScreen())
	_, err := p.Run()
	return err
}