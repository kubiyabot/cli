package tui

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"sync"
	"time"

	"github.com/charmbracelet/bubbles/help"
	"github.com/charmbracelet/bubbles/key"
	"github.com/charmbracelet/bubbles/list"
	"github.com/charmbracelet/bubbles/spinner"
	"github.com/charmbracelet/bubbles/textarea"
	"github.com/charmbracelet/bubbles/viewport"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
	"github.com/kubiyabot/cli/internal/config"
	"github.com/kubiyabot/cli/internal/kubiya"
)

// Enhanced chat states
type enhancedChatState int

const (
	agentSelectState enhancedChatState = iota
	chatState
	settingsState
	historyState
)

// Connection states
type connectionState int

const (
	connected connectionState = iota
	connecting
	disconnected
	reconnecting
)

// Enhanced chat message with additional metadata
type EnhancedChatMessage struct {
	kubiya.ChatMessage
	ID          string                 `json:"id"`
	LocalTime   time.Time              `json:"local_time"`
	Status      string                 `json:"status"` // "sent", "delivered", "failed", "retry"
	Metadata    map[string]interface{} `json:"metadata,omitempty"`
	RetryCount  int                    `json:"retry_count"`
	IsLocal     bool                   `json:"is_local"`
	FormattedAt string                 `json:"formatted_at"`
}

// Chat session with persistence
type EnhancedChatSession struct {
	ID               string                 `json:"id"`
	AgentID          string                 `json:"agent_id"`
	AgentName        string                 `json:"agent_name"`
	Messages         []EnhancedChatMessage  `json:"messages"`
	CreatedAt        time.Time              `json:"created_at"`
	LastActive       time.Time              `json:"last_active"`
	Context          map[string]string      `json:"context,omitempty"`
	Settings         map[string]interface{} `json:"settings,omitempty"`
	TotalMessages    int                    `json:"total_messages"`
	TotalToolsUsed   int                    `json:"total_tools_used"`
	AverageResponse  time.Duration          `json:"average_response"`
}

// Enhanced tool execution tracking
type EnhancedToolExecution struct {
	ID            string                 `json:"id"`
	Name          string                 `json:"name"`
	Args          string                 `json:"args"`
	StartTime     time.Time              `json:"start_time"`
	EndTime       time.Time              `json:"end_time"`
	Duration      time.Duration          `json:"duration"`
	Status        string                 `json:"status"` // "waiting", "running", "completed", "failed"
	Output        string                 `json:"output"`
	Error         string                 `json:"error,omitempty"`
	MessageID     string                 `json:"message_id"`
	RetryCount    int                    `json:"retry_count"`
	MaxRetries    int                    `json:"max_retries"`
	Metadata      map[string]interface{} `json:"metadata,omitempty"`
}

// Key bindings for enhanced chat
type enhancedChatKeyMap struct {
	Send           key.Binding
	Quit           key.Binding
	Back           key.Binding
	Clear          key.Binding
	History        key.Binding
	Settings       key.Binding
	Reconnect      key.Binding
	SaveSession    key.Binding
	LoadSession    key.Binding
	NextAgent      key.Binding
	PrevAgent      key.Binding
	ToggleDebug    key.Binding
	ExportChat     key.Binding
	Help           key.Binding
}

// Enhanced chat UI with better stability and features
type EnhancedChatUI struct {
	cfg    *config.Config
	client *kubiya.Client

	// State management
	state           enhancedChatState
	connectionState connectionState
	width           int
	height          int
	ready           bool
	err             error

	// UI components
	viewport        viewport.Model
	textarea        textarea.Model
	spinner         spinner.Model
	list            list.Model
	help            help.Model
	keyMap          enhancedChatKeyMap
	agentSelection  *EnhancedAgentSelection

	// Agent management
	agents        []kubiya.Agent
	selectedAgent kubiya.Agent
	agentIndex    int

	// Session management
	currentSession    *EnhancedChatSession
	sessionHistory    []EnhancedChatSession
	sessionFile       string
	autoSave          bool
	sessionMutex      sync.RWMutex

	// Tool execution tracking
	toolExecutions map[string]*EnhancedToolExecution
	toolMutex      sync.RWMutex

	// Connection management
	connectionRetries int
	maxRetries        int
	retryDelay        time.Duration
	isReconnecting    bool
	lastPing          time.Time
	pingInterval      time.Duration

	// Enhanced features
	debugMode       bool
	messageHistory  []string
	historyIndex    int
	isTyping        bool
	typingTimeout   time.Duration
	lastActivity    time.Time
	messageBuffer   strings.Builder
	pendingMessages []EnhancedChatMessage

	// Context and cancel functions
	ctx         context.Context
	cancel      context.CancelFunc
	cancelFuncs []context.CancelFunc

	// Program reference for sending messages
	program *tea.Program
}

// Initialize enhanced chat UI
func NewEnhancedChatUI(cfg *config.Config) *EnhancedChatUI {
	s := spinner.New()
	s.Spinner = spinner.Points
	s.Style = lipgloss.NewStyle().Foreground(lipgloss.Color("69"))

	ta := textarea.New()
	ta.Placeholder = "Type your message here..."
	ta.Focus()
	ta.SetWidth(60)
	ta.SetHeight(3)
	ta.ShowLineNumbers = false
	ta.KeyMap.InsertNewline.SetEnabled(false)

	vp := viewport.New(0, 0)
	vp.SetContent("Welcome to Enhanced Kubiya Chat!\n\nSelect an agent to start chatting.")

	// Create enhanced key map
	keyMap := enhancedChatKeyMap{
		Send: key.NewBinding(
			key.WithKeys("enter"),
			key.WithHelp("enter", "send message"),
		),
		Quit: key.NewBinding(
			key.WithKeys("ctrl+c", "q"),
			key.WithHelp("ctrl+c/q", "quit"),
		),
		Back: key.NewBinding(
			key.WithKeys("esc"),
			key.WithHelp("esc", "back"),
		),
		Clear: key.NewBinding(
			key.WithKeys("ctrl+l"),
			key.WithHelp("ctrl+l", "clear chat"),
		),
		History: key.NewBinding(
			key.WithKeys("ctrl+h"),
			key.WithHelp("ctrl+h", "history"),
		),
		Settings: key.NewBinding(
			key.WithKeys("ctrl+s"),
			key.WithHelp("ctrl+s", "settings"),
		),
		Reconnect: key.NewBinding(
			key.WithKeys("ctrl+r"),
			key.WithHelp("ctrl+r", "reconnect"),
		),
		SaveSession: key.NewBinding(
			key.WithKeys("ctrl+w"),
			key.WithHelp("ctrl+w", "save session"),
		),
		LoadSession: key.NewBinding(
			key.WithKeys("ctrl+o"),
			key.WithHelp("ctrl+o", "load session"),
		),
		NextAgent: key.NewBinding(
			key.WithKeys("ctrl+n"),
			key.WithHelp("ctrl+n", "next agent"),
		),
		PrevAgent: key.NewBinding(
			key.WithKeys("ctrl+p"),
			key.WithHelp("ctrl+p", "prev agent"),
		),
		ToggleDebug: key.NewBinding(
			key.WithKeys("ctrl+d"),
			key.WithHelp("ctrl+d", "toggle debug"),
		),
		ExportChat: key.NewBinding(
			key.WithKeys("ctrl+e"),
			key.WithHelp("ctrl+e", "export chat"),
		),
		Help: key.NewBinding(
			key.WithKeys("?"),
			key.WithHelp("?", "help"),
		),
	}

	delegate := list.NewDefaultDelegate()
	delegate.Styles.SelectedTitle = lipgloss.NewStyle().Foreground(lipgloss.Color("170")).Bold(true)
	delegate.Styles.SelectedDesc = lipgloss.NewStyle().Foreground(lipgloss.Color("245"))

	agentList := list.New([]list.Item{}, delegate, 0, 0)
	agentList.Title = "📱 Select Agent"
	agentList.SetShowStatusBar(false)
	agentList.SetFilteringEnabled(true)
	agentList.Styles.Title = lipgloss.NewStyle().Bold(true).Foreground(lipgloss.Color("170"))

	ctx, cancel := context.WithCancel(context.Background())

	// Create session file path
	homeDir, _ := os.UserHomeDir()
	sessionFile := filepath.Join(homeDir, ".kubiya", "chat_sessions.json")

	// Initialize enhanced agent selection
	agentSelection := NewEnhancedAgentSelection(cfg)

	ui := &EnhancedChatUI{
		cfg:             cfg,
		client:          kubiya.NewClient(cfg),
		state:           agentSelectState,
		connectionState: disconnected,
		viewport:        vp,
		textarea:        ta,
		spinner:         s,
		list:            agentList,
		help:            help.New(),
		keyMap:          keyMap,
		agentSelection:  agentSelection,
		toolExecutions:  make(map[string]*EnhancedToolExecution),
		maxRetries:      3,
		retryDelay:      time.Second * 2,
		pingInterval:    time.Second * 30,
		typingTimeout:   time.Second * 2,
		autoSave:        true,
		sessionFile:     sessionFile,
		ctx:             ctx,
		cancel:          cancel,
		messageHistory:  make([]string, 0, 100),
		historyIndex:    -1,
		pendingMessages: make([]EnhancedChatMessage, 0),
	}

	// Set up agent selection callbacks
	agentSelection.OnAgentSelected(func(agent kubiya.Agent) {
		ui.selectedAgent = agent
		ui.state = chatState
		ui.initializeSession(agent)
	})

	agentSelection.OnBack(func() {
		ui.cleanup()
	})

	agentSelection.OnError(func(err error) {
		ui.err = err
	})

	// Load session history
	ui.loadSessionHistory()

	return ui
}

// Initialize the enhanced chat UI
func (ui *EnhancedChatUI) Init() tea.Cmd {
	return tea.Batch(
		ui.spinner.Tick,
		ui.fetchAgents(),
		ui.startPingMonitor(),
		textarea.Blink,
	)
}

// Update handles all UI updates with enhanced error handling
func (ui *EnhancedChatUI) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	var cmds []tea.Cmd
	var cmd tea.Cmd

	// Handle window resize
	if msg, ok := msg.(tea.WindowSizeMsg); ok {
		ui.width = msg.Width
		ui.height = msg.Height
		ui.updateLayout()
	}

	// Handle different message types
	switch msg := msg.(type) {
	case []kubiya.Agent:
		ui.agents = msg
		ui.updateAgentList()
		ui.ready = true
		ui.connectionState = connected
		return ui, nil

	case kubiya.ChatMessage:
		return ui, ui.handleChatMessage(msg)

	case connectionMsg:
		ui.connectionState = msg.state
		return ui, nil

	case error:
		ui.err = msg
		ui.connectionState = disconnected
		if ui.connectionRetries < ui.maxRetries {
			return ui, ui.scheduleReconnect()
		}
		return ui, nil

	case reconnectMsg:
		return ui, ui.attemptReconnect()

	case pingMsg:
		ui.lastPing = time.Now()
		return ui, ui.scheduleNextPing()

	case toolExecutionMsg:
		ui.handleToolExecution(msg)
		return ui, nil

	case sessionSavedMsg:
		ui.updateStatusBar("Session saved successfully")
		return ui, nil

	case tea.KeyMsg:
		return ui.handleKeyPress(msg)
	}

	// Update UI components
	switch ui.state {
	case agentSelectState:
		if ui.agentSelection != nil {
			model, cmd := ui.agentSelection.Update(msg)
			if agentSel, ok := model.(*EnhancedAgentSelection); ok {
				ui.agentSelection = agentSel
			}
			cmds = append(cmds, cmd)
		} else {
			ui.list, cmd = ui.list.Update(msg)
			cmds = append(cmds, cmd)
		}

	case chatState:
		ui.textarea, cmd = ui.textarea.Update(msg)
		cmds = append(cmds, cmd)
		ui.viewport, cmd = ui.viewport.Update(msg)
		cmds = append(cmds, cmd)
	}

	// Update spinner if connecting
	if ui.connectionState == connecting || ui.connectionState == reconnecting {
		ui.spinner, cmd = ui.spinner.Update(msg)
		cmds = append(cmds, cmd)
	}

	return ui, tea.Batch(cmds...)
}

// Enhanced view rendering with better styling
func (ui *EnhancedChatUI) View() string {
	if ui.err != nil {
		return ui.renderError()
	}

	if !ui.ready {
		return ui.renderLoading()
	}

	switch ui.state {
	case agentSelectState:
		if ui.agentSelection != nil {
			return ui.agentSelection.View()
		}
		return ui.renderAgentSelect()
	case chatState:
		return ui.renderChat()
	case settingsState:
		return ui.renderSettings()
	case historyState:
		return ui.renderHistory()
	}

	return ""
}

// Render different UI states
func (ui *EnhancedChatUI) renderAgentSelect() string {
	var b strings.Builder

	// Header
	header := lipgloss.NewStyle().
		Bold(true).
		Foreground(lipgloss.Color("170")).
		Border(lipgloss.RoundedBorder()).
		Padding(1).
		Width(ui.width - 4).
		Align(lipgloss.Center).
		Render("🤖 Kubiya Enhanced Chat")

	b.WriteString(header + "\n\n")

	// Connection status
	b.WriteString(ui.renderConnectionStatus() + "\n\n")

	// Agent list
	b.WriteString(ui.list.View() + "\n\n")

	// Help
	b.WriteString(ui.renderHelp())

	return b.String()
}

func (ui *EnhancedChatUI) renderChat() string {
	var b strings.Builder

	// Header with agent info
	header := ui.renderChatHeader()
	b.WriteString(header + "\n")

	// Connection status
	b.WriteString(ui.renderConnectionStatus() + "\n")

	// Chat messages
	b.WriteString(ui.viewport.View() + "\n")

	// Tool execution status
	if len(ui.toolExecutions) > 0 {
		b.WriteString(ui.renderToolExecutions() + "\n")
	}

	// Input area
	b.WriteString(ui.renderInput())

	// Help
	b.WriteString(ui.renderHelp())

	return b.String()
}

func (ui *EnhancedChatUI) renderConnectionStatus() string {
	var status, color string
	var emoji string

	switch ui.connectionState {
	case connected:
		status = "Connected"
		color = "46"
		emoji = "🟢"
	case connecting:
		status = "Connecting..."
		color = "226"
		emoji = "🟡"
	case disconnected:
		status = "Disconnected"
		color = "196"
		emoji = "🔴"
	case reconnecting:
		status = fmt.Sprintf("Reconnecting... (attempt %d/%d)", ui.connectionRetries+1, ui.maxRetries)
		color = "208"
		emoji = "🟠"
	}

	style := lipgloss.NewStyle().
		Foreground(lipgloss.Color(color)).
		Bold(true)

	return style.Render(fmt.Sprintf("%s %s", emoji, status))
}

func (ui *EnhancedChatUI) renderChatHeader() string {
	if ui.selectedAgent.UUID == "" {
		return ""
	}

	style := lipgloss.NewStyle().
		Bold(true).
		Foreground(lipgloss.Color("170")).
		Border(lipgloss.RoundedBorder()).
		Padding(0, 1).
		Width(ui.width - 4)

	info := fmt.Sprintf("💬 %s", ui.selectedAgent.Name)
	if ui.currentSession != nil {
		info += fmt.Sprintf(" | Messages: %d | Tools: %d", 
			ui.currentSession.TotalMessages, 
			ui.currentSession.TotalToolsUsed)
	}

	return style.Render(info)
}

func (ui *EnhancedChatUI) renderInput() string {
	var b strings.Builder

	// Input prompt
	prompt := lipgloss.NewStyle().
		Foreground(lipgloss.Color("245")).
		Render("💭 Message:")

	b.WriteString(prompt + "\n")
	b.WriteString(ui.textarea.View() + "\n")

	// Show typing indicator
	if ui.isTyping {
		typing := lipgloss.NewStyle().
			Foreground(lipgloss.Color("245")).
			Italic(true).
			Render("💭 Typing...")
		b.WriteString(typing + "\n")
	}

	return b.String()
}

func (ui *EnhancedChatUI) renderHelp() string {
	keys := []key.Binding{
		ui.keyMap.Send,
		ui.keyMap.Back,
		ui.keyMap.Clear,
		ui.keyMap.Reconnect,
		ui.keyMap.Help,
		ui.keyMap.Quit,
	}

	return ui.help.ShortHelpView(keys)
}

func (ui *EnhancedChatUI) renderError() string {
	style := lipgloss.NewStyle().
		Foreground(lipgloss.Color("196")).
		Bold(true).
		Border(lipgloss.RoundedBorder()).
		Padding(1)

	return style.Render(fmt.Sprintf("❌ Error: %v\n\nPress 'r' to retry or 'q' to quit", ui.err))
}

func (ui *EnhancedChatUI) renderLoading() string {
	style := lipgloss.NewStyle().
		Foreground(lipgloss.Color("69")).
		Bold(true)

	return style.Render(fmt.Sprintf("Loading... %s", ui.spinner.View()))
}

func (ui *EnhancedChatUI) renderToolExecutions() string {
	var b strings.Builder

	ui.toolMutex.RLock()
	defer ui.toolMutex.RUnlock()

	if len(ui.toolExecutions) == 0 {
		return ""
	}

	style := lipgloss.NewStyle().
		Foreground(lipgloss.Color("226")).
		Bold(true).
		Border(lipgloss.RoundedBorder()).
		Padding(1)

	b.WriteString("🔧 Tool Executions:\n")

	for _, tool := range ui.toolExecutions {
		var statusEmoji string
		switch tool.Status {
		case "waiting":
			statusEmoji = "⏳"
		case "running":
			statusEmoji = "🔄"
		case "completed":
			statusEmoji = "✅"
		case "failed":
			statusEmoji = "❌"
		}

		duration := ""
		if tool.Status == "completed" || tool.Status == "failed" {
			duration = fmt.Sprintf(" (%.1fs)", tool.Duration.Seconds())
		}

		b.WriteString(fmt.Sprintf("%s %s%s\n", statusEmoji, tool.Name, duration))
	}

	return style.Render(b.String())
}

// Enhanced message types
type connectionMsg struct {
	state connectionState
}

type reconnectMsg struct{}

type pingMsg struct{}

type toolExecutionMsg struct {
	execution *EnhancedToolExecution
}

type sessionSavedMsg struct{}

// Helper functions for enhanced functionality
func (ui *EnhancedChatUI) updateLayout() {
	ui.list.SetSize(ui.width-4, ui.height-10)
	ui.textarea.SetWidth(ui.width - 20)
	ui.viewport.Width = ui.width - 4
	ui.viewport.Height = ui.height - 15
}

func (ui *EnhancedChatUI) updateAgentList() {
	items := make([]list.Item, len(ui.agents))
	for i, agent := range ui.agents {
		items[i] = agentItem{agent: agent}
	}
	ui.list.SetItems(items)
}

func (ui *EnhancedChatUI) updateStatusBar(message string) {
	// Implementation for status bar updates
	// This would be displayed in the UI
}

// Enhanced error handling and reconnection
func (ui *EnhancedChatUI) scheduleReconnect() tea.Cmd {
	return tea.Tick(ui.retryDelay, func(time.Time) tea.Msg {
		return reconnectMsg{}
	})
}

func (ui *EnhancedChatUI) attemptReconnect() tea.Cmd {
	ui.connectionRetries++
	ui.connectionState = reconnecting
	ui.isReconnecting = true

	return func() tea.Msg {
		// Attempt to reconnect
		ui.client = kubiya.NewClient(ui.cfg)
		agents, err := ui.client.ListAgents(ui.ctx)
		if err != nil {
			return err
		}
		ui.isReconnecting = false
		ui.connectionRetries = 0
		return agents
	}
}

func (ui *EnhancedChatUI) startPingMonitor() tea.Cmd {
	return tea.Tick(ui.pingInterval, func(time.Time) tea.Msg {
		return pingMsg{}
	})
}

func (ui *EnhancedChatUI) scheduleNextPing() tea.Cmd {
	return tea.Tick(ui.pingInterval, func(time.Time) tea.Msg {
		return pingMsg{}
	})
}

// Enhanced session management
func (ui *EnhancedChatUI) loadSessionHistory() {
	ui.sessionMutex.Lock()
	defer ui.sessionMutex.Unlock()

	if _, err := os.Stat(ui.sessionFile); os.IsNotExist(err) {
		return
	}

	data, err := os.ReadFile(ui.sessionFile)
	if err != nil {
		return
	}

	var sessions []EnhancedChatSession
	if err := json.Unmarshal(data, &sessions); err != nil {
		return
	}

	ui.sessionHistory = sessions
}

func (ui *EnhancedChatUI) saveSessionHistory() error {
	ui.sessionMutex.Lock()
	defer ui.sessionMutex.Unlock()

	// Ensure directory exists
	if err := os.MkdirAll(filepath.Dir(ui.sessionFile), 0755); err != nil {
		return err
	}

	data, err := json.MarshalIndent(ui.sessionHistory, "", "  ")
	if err != nil {
		return err
	}

	return os.WriteFile(ui.sessionFile, data, 0644)
}

// Enhanced message handling
func (ui *EnhancedChatUI) handleChatMessage(msg kubiya.ChatMessage) tea.Cmd {
	// Convert to enhanced message
	enhancedMsg := EnhancedChatMessage{
		ChatMessage: msg,
		ID:          fmt.Sprintf("%d_%s", time.Now().UnixNano(), msg.MessageID),
		LocalTime:   time.Now(),
		Status:      "delivered",
		IsLocal:     false,
		FormattedAt: time.Now().Format("15:04:05"),
	}

	// Add to current session
	if ui.currentSession != nil {
		ui.currentSession.Messages = append(ui.currentSession.Messages, enhancedMsg)
		ui.currentSession.TotalMessages++
		ui.currentSession.LastActive = time.Now()
	}

	// Update viewport with new message
	ui.updateChatView()

	// Auto-save if enabled
	if ui.autoSave {
		return ui.saveSession()
	}

	return nil
}

func (ui *EnhancedChatUI) handleToolExecution(msg toolExecutionMsg) {
	ui.toolMutex.Lock()
	defer ui.toolMutex.Unlock()

	tool := msg.execution
	ui.toolExecutions[tool.ID] = tool

	if ui.currentSession != nil && tool.Status == "completed" {
		ui.currentSession.TotalToolsUsed++
	}

	// Clean up completed tools after some time
	if tool.Status == "completed" || tool.Status == "failed" {
		go func() {
			time.Sleep(time.Second * 30)
			ui.toolMutex.Lock()
			delete(ui.toolExecutions, tool.ID)
			ui.toolMutex.Unlock()
		}()
	}
}

func (ui *EnhancedChatUI) updateChatView() {
	if ui.currentSession == nil {
		return
	}

	var b strings.Builder

	// Render all messages
	for _, msg := range ui.currentSession.Messages {
		b.WriteString(ui.formatMessage(msg))
		b.WriteString("\n")
	}

	ui.viewport.SetContent(b.String())
	ui.viewport.GotoBottom()
}

func (ui *EnhancedChatUI) formatMessage(msg EnhancedChatMessage) string {
	var style lipgloss.Style
	var prefix string

	if msg.SenderName == "You" {
		style = lipgloss.NewStyle().Foreground(lipgloss.Color("46"))
		prefix = "👤"
	} else {
		style = lipgloss.NewStyle().Foreground(lipgloss.Color("69"))
		prefix = "🤖"
	}

	timestamp := lipgloss.NewStyle().
		Foreground(lipgloss.Color("245")).
		Render(fmt.Sprintf("[%s]", msg.FormattedAt))

	sender := style.Bold(true).Render(msg.SenderName)
	content := style.Render(msg.Content)

	return fmt.Sprintf("%s %s %s: %s", timestamp, prefix, sender, content)
}

// Enhanced key handling
func (ui *EnhancedChatUI) handleKeyPress(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	switch {
	case key.Matches(msg, ui.keyMap.Quit):
		ui.cleanup()
		return ui, tea.Quit

	case key.Matches(msg, ui.keyMap.Back):
		if ui.state == chatState {
			ui.state = agentSelectState
			return ui, nil
		}

	case key.Matches(msg, ui.keyMap.Clear):
		if ui.state == chatState {
			ui.clearChat()
			return ui, nil
		}

	case key.Matches(msg, ui.keyMap.Reconnect):
		return ui, ui.attemptReconnect()

	case key.Matches(msg, ui.keyMap.SaveSession):
		if ui.state == chatState {
			return ui, ui.saveSession()
		}

	case key.Matches(msg, ui.keyMap.ToggleDebug):
		ui.debugMode = !ui.debugMode
		return ui, nil

	case key.Matches(msg, ui.keyMap.Send):
		if ui.state == chatState {
			return ui, ui.sendMessage()
		} else if ui.state == agentSelectState {
			return ui, ui.selectAgent()
		}
	}

	return ui, nil
}

// Enhanced utility functions
func (ui *EnhancedChatUI) clearChat() {
	if ui.currentSession != nil {
		ui.currentSession.Messages = []EnhancedChatMessage{}
		ui.currentSession.TotalMessages = 0
		ui.updateChatView()
	}
}

func (ui *EnhancedChatUI) saveSession() tea.Cmd {
	return func() tea.Msg {
		if ui.currentSession != nil {
			// Add or update session in history
			found := false
			for i, session := range ui.sessionHistory {
				if session.ID == ui.currentSession.ID {
					ui.sessionHistory[i] = *ui.currentSession
					found = true
					break
				}
			}
			if !found {
				ui.sessionHistory = append(ui.sessionHistory, *ui.currentSession)
			}

			// Keep only last 50 sessions
			if len(ui.sessionHistory) > 50 {
				ui.sessionHistory = ui.sessionHistory[len(ui.sessionHistory)-50:]
			}

			// Sort by last active
			sort.Slice(ui.sessionHistory, func(i, j int) bool {
				return ui.sessionHistory[i].LastActive.After(ui.sessionHistory[j].LastActive)
			})

			if err := ui.saveSessionHistory(); err != nil {
				return err
			}
		}
		return sessionSavedMsg{}
	}
}

func (ui *EnhancedChatUI) selectAgent() tea.Cmd {
	if item, ok := ui.list.SelectedItem().(agentItem); ok {
		ui.selectedAgent = item.agent
		ui.state = chatState

		// Create new session
		ui.currentSession = &EnhancedChatSession{
			ID:          fmt.Sprintf("%d_%s", time.Now().UnixNano(), ui.selectedAgent.UUID),
			AgentID:     ui.selectedAgent.UUID,
			AgentName:   ui.selectedAgent.Name,
			Messages:    []EnhancedChatMessage{},
			CreatedAt:   time.Now(),
			LastActive:  time.Now(),
			Settings:    make(map[string]interface{}),
		}

		ui.updateChatView()
		ui.textarea.Focus()
	}
	return nil
}

func (ui *EnhancedChatUI) sendMessage() tea.Cmd {
	message := strings.TrimSpace(ui.textarea.Value())
	if message == "" {
		return nil
	}

	// Add to message history
	ui.messageHistory = append(ui.messageHistory, message)
	if len(ui.messageHistory) > 100 {
		ui.messageHistory = ui.messageHistory[1:]
	}
	ui.historyIndex = len(ui.messageHistory)

	// Clear textarea
	ui.textarea.SetValue("")

	// Add user message to session
	userMsg := EnhancedChatMessage{
		ChatMessage: kubiya.ChatMessage{
			Content:    message,
			SenderName: "You",
			Timestamp:  time.Now().Format(time.RFC3339),
			Final:      true,
		},
		ID:          fmt.Sprintf("%d_user", time.Now().UnixNano()),
		LocalTime:   time.Now(),
		Status:      "sent",
		IsLocal:     true,
		FormattedAt: time.Now().Format("15:04:05"),
	}

	if ui.currentSession != nil {
		ui.currentSession.Messages = append(ui.currentSession.Messages, userMsg)
		ui.currentSession.TotalMessages++
		ui.currentSession.LastActive = time.Now()
	}

	ui.updateChatView()

	// Send message to agent
	return ui.sendToAgent(message)
}

func (ui *EnhancedChatUI) sendToAgent(message string) tea.Cmd {
	return func() tea.Msg {
		ctx, cancel := context.WithTimeout(ui.ctx, time.Minute*5)
		ui.cancelFuncs = append(ui.cancelFuncs, cancel)

		sessionID := ""
		if ui.currentSession != nil {
			sessionID = ui.currentSession.ID
		}

		msgChan, err := ui.client.SendMessage(ctx, ui.selectedAgent.UUID, message, sessionID)
		if err != nil {
			return err
		}

		// Handle incoming messages
		go func() {
			for {
				select {
				case <-ctx.Done():
					return
				case msg, ok := <-msgChan:
					if !ok {
						return
					}
					msg.SenderName = ui.selectedAgent.Name
					if ui.program != nil {
						ui.program.Send(msg)
					}
				}
			}
		}()

		return nil
	}
}

func (ui *EnhancedChatUI) fetchAgents() tea.Cmd {
	return func() tea.Msg {
		agents, err := ui.client.ListAgents(ui.ctx)
		if err != nil {
			return err
		}
		return agents
	}
}

func (ui *EnhancedChatUI) initializeSession(agent kubiya.Agent) {
	// Create new session
	ui.currentSession = &EnhancedChatSession{
		ID:          fmt.Sprintf("%d_%s", time.Now().UnixNano(), agent.UUID),
		AgentID:     agent.UUID,
		AgentName:   agent.Name,
		Messages:    []EnhancedChatMessage{},
		CreatedAt:   time.Now(),
		LastActive:  time.Now(),
		Settings:    make(map[string]interface{}),
	}

	ui.updateChatView()
	ui.textarea.Focus()
}

func (ui *EnhancedChatUI) cleanup() {
	ui.cancel()
	for _, cancel := range ui.cancelFuncs {
		cancel()
	}
	if ui.autoSave && ui.currentSession != nil {
		ui.saveSession()
	}
	if ui.agentSelection != nil {
		ui.agentSelection.Cleanup()
	}
}

func (ui *EnhancedChatUI) renderSettings() string {
	return "Settings view - coming soon!"
}

func (ui *EnhancedChatUI) renderHistory() string {
	return "History view - coming soon!"
}

// Run the enhanced chat UI
func (ui *EnhancedChatUI) Run() error {
	p := tea.NewProgram(ui, tea.WithAltScreen())
	ui.program = p
	_, err := p.Run()
	return err
}