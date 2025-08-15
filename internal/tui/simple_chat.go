package tui

import (
	"context"
	"fmt"
	"os"
	"strings"
	"time"

	"github.com/charmbracelet/bubbles/list"
	"github.com/charmbracelet/bubbles/spinner"
	"github.com/charmbracelet/bubbles/textarea"
	"github.com/charmbracelet/bubbles/viewport"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
	"github.com/kubiyabot/cli/internal/config"
	"github.com/kubiyabot/cli/internal/kubiya"
)

// SimpleChatUI - A simple, robust interactive chat UI
type SimpleChatUI struct {
	cfg    *config.Config
	client *kubiya.Client
	
	// State
	state      chatUIState
	width      int
	height     int
	ready      bool
	err        error
	
	// UI Components
	agentList list.Model
	textarea  textarea.Model
	viewport  viewport.Model
	spinner   spinner.Model
	
	// Data
	agents        []kubiya.Agent
	selectedAgent *kubiya.Agent
	messages      []ChatMessage
	
	// Context
	ctx    context.Context
	cancel context.CancelFunc
}

type chatUIState int

const (
	stateAgentSelect chatUIState = iota
	stateChat
	stateError
)

type ChatMessage struct {
	Content string
	IsUser  bool
	Time    time.Time
}

// Agent list item implementation
type agentListItem struct {
	agent kubiya.Agent
}

func (i agentListItem) Title() string       { return i.agent.Name }
func (i agentListItem) Description() string { 
	if i.agent.Description != "" {
		return i.agent.Description
	}
	return i.agent.Desc
}
func (i agentListItem) FilterValue() string { return i.agent.Name + " " + i.agent.Description }

// Message types
type agentsLoadedMsg []kubiya.Agent
type chatResponseMsg kubiya.ChatMessage
type errorMsg error

// NewSimpleChatUI creates a new simple chat UI
func NewSimpleChatUI(cfg *config.Config) *SimpleChatUI {
	ctx, cancel := context.WithCancel(context.Background())
	
	// Create spinner
	s := spinner.New()
	s.Spinner = spinner.Dot
	s.Style = lipgloss.NewStyle().Foreground(lipgloss.Color("69"))
	
	// Create textarea
	ta := textarea.New()
	ta.Placeholder = "Type your message..."
	ta.SetWidth(80)
	ta.SetHeight(3)
	ta.ShowLineNumbers = false
	ta.KeyMap.InsertNewline.SetEnabled(false)
	
	// Create viewport
	vp := viewport.New(80, 20)
	vp.SetContent("Welcome to Kubiya Chat!\nSelect an agent to start chatting.")
	
	// Create agent list
	delegate := list.NewDefaultDelegate()
	agentList := list.New([]list.Item{}, delegate, 0, 0)
	agentList.Title = "Select Agent"
	agentList.SetShowStatusBar(false)
	agentList.SetFilteringEnabled(true)
	agentList.Styles.Title = lipgloss.NewStyle().Bold(true).Foreground(lipgloss.Color("170"))
	
	return &SimpleChatUI{
		cfg:       cfg,
		client:    kubiya.NewClient(cfg),
		state:     stateAgentSelect,
		agentList: agentList,
		textarea:  ta,
		viewport:  vp,
		spinner:   s,
		ctx:       ctx,
		cancel:    cancel,
		messages:  make([]ChatMessage, 0),
	}
}

// Init initializes the UI
func (ui *SimpleChatUI) Init() tea.Cmd {
	return tea.Batch(
		ui.spinner.Tick,
		ui.loadAgents(),
	)
}

// Update handles all state updates
func (ui *SimpleChatUI) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	var cmd tea.Cmd
	var cmds []tea.Cmd
	
	switch msg := msg.(type) {
	case tea.WindowSizeMsg:
		ui.width = msg.Width
		ui.height = msg.Height
		ui.updateSizes()
		
	case agentsLoadedMsg:
		ui.agents = []kubiya.Agent(msg)
		ui.updateAgentList()
		ui.ready = true
		ui.err = nil
		
	case chatResponseMsg:
		ui.addMessage(string(msg.Content), false)
		
	case errorMsg:
		ui.err = error(msg)
		ui.state = stateError
		
	case tea.KeyMsg:
		return ui.handleKeyMsg(msg)
		
	case spinner.TickMsg:
		ui.spinner, cmd = ui.spinner.Update(msg)
		cmds = append(cmds, cmd)
	}
	
	// Update components based on state
	switch ui.state {
	case stateAgentSelect:
		ui.agentList, cmd = ui.agentList.Update(msg)
		cmds = append(cmds, cmd)
		
	case stateChat:
		ui.textarea, cmd = ui.textarea.Update(msg)
		cmds = append(cmds, cmd)
		ui.viewport, cmd = ui.viewport.Update(msg)
		cmds = append(cmds, cmd)
	}
	
	return ui, tea.Batch(cmds...)
}

// View renders the UI
func (ui *SimpleChatUI) View() string {
	if ui.width == 0 {
		return "Loading..."
	}
	
	switch ui.state {
	case stateError:
		return ui.renderError()
	case stateAgentSelect:
		if !ui.ready {
			return ui.renderLoading()
		}
		return ui.renderAgentSelect()
	case stateChat:
		return ui.renderChat()
	}
	
	return "Unknown state"
}

// Render functions
func (ui *SimpleChatUI) renderLoading() string {
	style := lipgloss.NewStyle().
		Width(ui.width).
		Height(ui.height).
		Align(lipgloss.Center, lipgloss.Center)
		
	return style.Render(fmt.Sprintf("Loading agents... %s", ui.spinner.View()))
}

func (ui *SimpleChatUI) renderError() string {
	style := lipgloss.NewStyle().
		Width(ui.width).
		Height(ui.height).
		Align(lipgloss.Center, lipgloss.Center).
		Foreground(lipgloss.Color("196")).
		Bold(true)
		
	errorText := fmt.Sprintf("Error: %v\n\nPress 'r' to retry or 'q' to quit", ui.err)
	return style.Render(errorText)
}

func (ui *SimpleChatUI) renderAgentSelect() string {
	headerStyle := lipgloss.NewStyle().
		Bold(true).
		Foreground(lipgloss.Color("170")).
		Align(lipgloss.Center).
		Width(ui.width).
		Margin(1, 0)
	
	header := headerStyle.Render("🤖 Select a Kubiya Agent")
	
	// Update list size
	ui.agentList.SetSize(ui.width-4, ui.height-6)
	listView := ui.agentList.View()
	
	helpStyle := lipgloss.NewStyle().
		Foreground(lipgloss.Color("245")).
		Align(lipgloss.Center).
		Width(ui.width)
	
	help := helpStyle.Render("↑/↓: navigate • enter: select • /: search • q: quit")
	
	return lipgloss.JoinVertical(
		lipgloss.Left,
		header,
		listView,
		help,
	)
}

func (ui *SimpleChatUI) renderChat() string {
	if ui.selectedAgent == nil {
		return "No agent selected"
	}
	
	// Header with agent info
	headerStyle := lipgloss.NewStyle().
		Bold(true).
		Foreground(lipgloss.Color("170")).
		Border(lipgloss.RoundedBorder()).
		Padding(0, 2).
		Width(ui.width - 4)
	
	header := headerStyle.Render(fmt.Sprintf("💬 %s", ui.selectedAgent.Name))
	
	// Messages viewport
	ui.viewport.Width = ui.width - 4
	ui.viewport.Height = ui.height - 10
	
	// Input area
	ui.textarea.SetWidth(ui.width - 10)
	
	inputStyle := lipgloss.NewStyle().
		Border(lipgloss.RoundedBorder()).
		Padding(0, 1).
		Width(ui.width - 4)
	
	input := inputStyle.Render(fmt.Sprintf("Message: %s", ui.textarea.View()))
	
	// Help
	helpStyle := lipgloss.NewStyle().
		Foreground(lipgloss.Color("245")).
		Align(lipgloss.Center).
		Width(ui.width)
	
	help := helpStyle.Render("enter: send • esc: back to agents • ctrl+c: quit")
	
	return lipgloss.JoinVertical(
		lipgloss.Left,
		header,
		ui.viewport.View(),
		input,
		help,
	)
}

// Event handlers
func (ui *SimpleChatUI) handleKeyMsg(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	switch ui.state {
	case stateError:
		switch msg.String() {
		case "r":
			ui.err = nil
			ui.state = stateAgentSelect
			return ui, ui.loadAgents()
		case "q", "ctrl+c":
			return ui, tea.Quit
		}
		
	case stateAgentSelect:
		switch msg.String() {
		case "enter":
			if item, ok := ui.agentList.SelectedItem().(agentListItem); ok {
				ui.selectedAgent = &item.agent
				ui.state = stateChat
				ui.textarea.Focus()
				ui.addMessage(fmt.Sprintf("Connected to %s", item.agent.Name), false)
			}
			return ui, nil
		case "q", "ctrl+c":
			return ui, tea.Quit
		}
		
	case stateChat:
		switch msg.String() {
		case "enter":
			return ui, ui.sendMessage()
		case "esc":
			ui.state = stateAgentSelect
			ui.textarea.Blur()
			return ui, nil
		case "ctrl+c":
			return ui, tea.Quit
		}
	}
	
	return ui, nil
}

// Helper functions
func (ui *SimpleChatUI) updateSizes() {
	if ui.width > 0 && ui.height > 0 {
		ui.agentList.SetSize(ui.width-4, ui.height-6)
		ui.viewport.Width = ui.width - 4
		ui.viewport.Height = ui.height - 10
		ui.textarea.SetWidth(ui.width - 10)
	}
}

func (ui *SimpleChatUI) updateAgentList() {
	items := make([]list.Item, len(ui.agents))
	for i, agent := range ui.agents {
		items[i] = agentListItem{agent: agent}
	}
	ui.agentList.SetItems(items)
}

func (ui *SimpleChatUI) addMessage(content string, isUser bool) {
	msg := ChatMessage{
		Content: content,
		IsUser:  isUser,
		Time:    time.Now(),
	}
	ui.messages = append(ui.messages, msg)
	ui.updateChatView()
}

func (ui *SimpleChatUI) updateChatView() {
	var content strings.Builder
	
	for _, msg := range ui.messages {
		timeStr := msg.Time.Format("15:04")
		if msg.IsUser {
			content.WriteString(fmt.Sprintf("[%s] You: %s\n", timeStr, msg.Content))
		} else {
			agentName := "Agent"
			if ui.selectedAgent != nil {
				agentName = ui.selectedAgent.Name
			}
			content.WriteString(fmt.Sprintf("[%s] %s: %s\n", timeStr, agentName, msg.Content))
		}
	}
	
	ui.viewport.SetContent(content.String())
	ui.viewport.GotoBottom()
}

func (ui *SimpleChatUI) loadAgents() tea.Cmd {
	return func() tea.Msg {
		// Try main endpoint first
		agents, err := ui.client.ListAgents(ui.ctx)
		if err != nil {
			// Fallback to legacy endpoint
			if legacyAgents, legacyErr := ui.client.ListAgentsLegacy(ui.ctx); legacyErr == nil {
				agents = legacyAgents
			} else {
				return errorMsg(fmt.Errorf("failed to load agents: %w", err))
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
			return errorMsg(fmt.Errorf("no valid agents found"))
		}
		
		return agentsLoadedMsg(validAgents)
	}
}

func (ui *SimpleChatUI) sendMessage() tea.Cmd {
	message := strings.TrimSpace(ui.textarea.Value())
	if message == "" {
		return nil
	}
	
	// Add user message
	ui.addMessage(message, true)
	
	// Clear textarea
	ui.textarea.SetValue("")
	
	// Send to agent
	if ui.selectedAgent == nil {
		return nil
	}
	
	return func() tea.Msg {
		ctx, cancel := context.WithTimeout(ui.ctx, 120*time.Second)
		defer cancel()
		
		// Use the enhanced SendMessageWithRetry for better reliability
		msgChan, err := ui.client.SendMessageWithRetry(ctx, ui.selectedAgent.UUID, message, "", 3)
		if err != nil {
			return errorMsg(fmt.Errorf("failed to send message: %w", err))
		}
		
		// Stream all responses properly
		var fullResponse strings.Builder
		hasResponse := false
		
		for {
			select {
			case response, ok := <-msgChan:
				if !ok {
					// Channel closed
					if hasResponse {
						return chatResponseMsg(kubiya.ChatMessage{
							Content: fullResponse.String(),
							Type:    "completion",
							Final:   true,
						})
					}
					return errorMsg(fmt.Errorf("no response from agent"))
				}
				
				hasResponse = true
				
				// Handle different message types
				switch response.Type {
				case "message", "completion":
					if response.Content != "" {
						// For streaming content, handle differential updates
						newContent := response.Content
						if len(newContent) > fullResponse.Len() {
							fullResponse.WriteString(newContent[fullResponse.Len():])
						}
					}
					
				case "tool_call", "tool_output":
					// For tool calls, show basic indication
					if response.Content != "" {
						fullResponse.WriteString(fmt.Sprintf("\n🔧 Tool: %s\n", response.Content))
					}
					
				case "error":
					return errorMsg(fmt.Errorf("agent error: %s", response.Error))
				}
				
				// If this is the final message, return accumulated response
				if response.Final {
					return chatResponseMsg(kubiya.ChatMessage{
						Content: fullResponse.String(),
						Type:    "completion",
						Final:   true,
					})
				}
				
			case <-ctx.Done():
				if hasResponse {
					// Return partial response on timeout
					return chatResponseMsg(kubiya.ChatMessage{
						Content: fullResponse.String(),
						Type:    "completion",
						Final:   true,
					})
				}
				return errorMsg(fmt.Errorf("timeout waiting for response"))
			}
		}
	}
}

// Cleanup releases resources
func (ui *SimpleChatUI) Cleanup() {
	ui.cancel()
}

// Run starts the chat UI
func (ui *SimpleChatUI) Run() error {
	// Check if we have a working terminal
	if !hasWorkingTerminal() {
		return ui.runFallbackMode()
	}
	
	// Use full TUI mode
	options := []tea.ProgramOption{tea.WithAltScreen()}
	p := tea.NewProgram(ui, options...)
	_, err := p.Run()
	
	ui.Cleanup()
	return err
}

// runFallbackMode runs a simple text-based interface when TUI is not available
func (ui *SimpleChatUI) runFallbackMode() error {
	fmt.Fprintf(os.Stderr, "🤖 Kubiya Interactive Chat (Text Mode)\n")
	fmt.Fprintf(os.Stderr, "Loading agents...\n")
	
	// Load agents
	agents, err := ui.client.ListAgents(ui.ctx)
	if err != nil {
		// Try legacy endpoint
		if agents, err = ui.client.ListAgentsLegacy(ui.ctx); err != nil {
			return fmt.Errorf("failed to load agents: %w", err)
		}
	}
	
	// Filter valid agents
	validAgents := make([]kubiya.Agent, 0)
	for _, agent := range agents {
		if agent.UUID != "" && agent.Name != "" {
			validAgents = append(validAgents, agent)
		}
	}
	
	if len(validAgents) == 0 {
		return fmt.Errorf("no valid agents found")
	}
	
	// Show agents and let user select
	fmt.Fprintf(os.Stderr, "\nAvailable agents:\n")
	for i, agent := range validAgents {
		desc := agent.Description
		if desc == "" {
			desc = agent.Desc
		}
		fmt.Fprintf(os.Stderr, "%d. %s - %s\n", i+1, agent.Name, desc)
	}
	
	fmt.Fprintf(os.Stderr, "\nNote: Interactive chat requires a proper terminal.\n")
	fmt.Fprintf(os.Stderr, "Please run this command in a terminal environment for the full interactive experience.\n")
	fmt.Fprintf(os.Stderr, "Use 'kubiya chat \"your message\"' for non-interactive messaging.\n")
	
	return nil
}

// hasWorkingTerminal checks if we have a working terminal
func hasWorkingTerminal() bool {
	// Allow forcing text mode
	if os.Getenv("KUBIYA_TEXT_MODE") != "" {
		return false
	}
	
	// Basic checks
	if os.Getenv("TERM") == "" {
		return false
	}
	
	if isNonInteractiveEnvironment() {
		return false
	}
	
	// Try to access /dev/tty if it exists (Unix-like systems)
	if _, err := os.Stat("/dev/tty"); err != nil {
		return false // /dev/tty not available
	}
	
	if tty, err := os.OpenFile("/dev/tty", os.O_RDWR, 0); err == nil {
		tty.Close()
		return true
	}
	
	return false
}

// isNonInteractiveEnvironment detects if we're in a CI/non-interactive environment
func isNonInteractiveEnvironment() bool {
	// Check for common CI environment variables
	ciEnvs := []string{
		"CI", "CONTINUOUS_INTEGRATION", "GITHUB_ACTIONS", 
		"JENKINS_URL", "BUILDKITE", "TRAVIS", "CIRCLECI",
	}
	
	for _, env := range ciEnvs {
		if os.Getenv(env) != "" {
			return true
		}
	}
	
	// Check if stdin is not a terminal
	if fileInfo, err := os.Stdin.Stat(); err == nil {
		return (fileInfo.Mode() & os.ModeCharDevice) == 0
	}
	
	return false
}