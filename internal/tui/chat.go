package tui

import (
	"context"
	"fmt"
	"strings"
	"time"

	"github.com/charmbracelet/bubbles/list"
	"github.com/charmbracelet/bubbles/spinner"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
	"github.com/kubiyabot/cli/internal/config"
	"github.com/kubiyabot/cli/internal/kubiya"
)

type screenState int

const (
	teammateSelectScreen screenState = iota
	chatScreen
)

type toolExecution struct {
	startTime        time.Time
	name             string
	output           string
	messageID        string
	isFinal          bool
	hasInitMessage   bool
	hasOutputMessage bool
	isComplete       bool
}

type ChatUI struct {
	cfg                  *config.Config
	client               *kubiya.Client
	messages             []kubiya.ChatMessage
	inputBuffer          string
	spinner              spinner.Model
	list                 list.Model
	err                  error
	teammates            []kubiya.Teammate
	selected             kubiya.Teammate
	width                int
	height               int
	state                screenState
	cursor               int
	ready                bool
	cancelFuncs          []context.CancelFunc
	P                    *tea.Program
	isBotTyping          bool
	spinnerStartTime     time.Time
	toolName             string
	toolRunning          bool
	toolExecutions       map[string]*toolExecution // Map to track ongoing tool executions
	currentToolMessageID string                    // Track current tool's MessageID
}

func NewChatUI(cfg *config.Config) *ChatUI {
	s := spinner.New()
	s.Spinner = spinner.Line
	s.Style = lipgloss.NewStyle().Foreground(lipgloss.Color("#3B82F6"))

	delegate := list.NewDefaultDelegate()
	delegate.Styles.SelectedTitle = lipgloss.NewStyle().Foreground(lipgloss.Color("205")).Bold(true)

	uiList := list.New([]list.Item{}, delegate, 0, 0)
	uiList.Title = "Select a Teammate"
	uiList.SetShowStatusBar(false)
	uiList.SetFilteringEnabled(true)
	uiList.Styles.Title = lipgloss.NewStyle().Bold(true).Foreground(lipgloss.Color("205"))

	return &ChatUI{
		cfg:                  cfg,
		client:               kubiya.NewClient(cfg),
		spinner:              s,
		state:                teammateSelectScreen,
		list:                 uiList,
		toolExecutions:       make(map[string]*toolExecution),
		currentToolMessageID: "",
	}
}

func (ui *ChatUI) Init() tea.Cmd {
	return tea.Batch(ui.spinner.Tick, ui.fetchTeammates)
}

func (ui *ChatUI) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	var cmds []tea.Cmd
	var cmd tea.Cmd

	switch msg := msg.(type) {

	case tea.WindowSizeMsg:
		ui.width = msg.Width
		ui.height = msg.Height
		ui.list.SetSize(ui.width, ui.height-4)

	case []kubiya.Teammate:
		ui.teammates = msg
		items := make([]list.Item, len(msg))
		for i, t := range msg {
			items[i] = teammateItem{teammate: t}
		}

		ui.list.SetItems(items)
		ui.ready = true
		return ui, nil

	case kubiya.ChatMessage:
		ui.handleChatMessage(msg)
		return ui, nil

	case finalizeMessage:
		ui.finalizeBotMessage()
		ui.isBotTyping = false // Bot has finished typing
		return ui, nil

	case error:
		ui.err = msg
		return ui, nil

	case tea.KeyMsg:
		switch ui.state {

		case teammateSelectScreen:
			switch msg.String() {
			case "ctrl+c", "q":
				ui.cancelContexts()
				return ui, tea.Quit
			case "enter":
				if item, ok := ui.list.SelectedItem().(teammateItem); ok {
					ui.selected = item.teammate
					ui.state = chatScreen
					ui.messages = nil
					ui.inputBuffer = ""
				}
			default:
				ui.list, cmd = ui.list.Update(msg)
				return ui, cmd
			}

		case chatScreen:
			// Prevent user input when the bot is typing
			if ui.isBotTyping {
				return ui, nil
			}
			switch msg.String() {
			case "ctrl+c", "q":
				ui.cancelContexts()
				return ui, tea.Quit
			case "esc":
				ui.cancelContexts()
				ui.state = teammateSelectScreen
			case "enter":
				if strings.TrimSpace(ui.inputBuffer) != "" {
					message := ui.inputBuffer
					ui.inputBuffer = ""
					ui.messages = append(ui.messages, kubiya.ChatMessage{
						Content:    message,
						SenderName: "You",
						Timestamp:  time.Now().Format(time.RFC3339),
						Final:      true,
					})
					return ui, ui.sendMessage(message)
				}
			case "backspace":
				if len(ui.inputBuffer) > 0 {
					ui.inputBuffer = ui.inputBuffer[:len(ui.inputBuffer)-1]
				}
			default:
				ui.inputBuffer += msg.String()
			}
		}
	}

	// Update spinner for typing indicator
	if ui.isBotTyping {
		ui.spinner, cmd = ui.spinner.Update(msg)
		cmds = append(cmds, cmd)
	}

	return ui, tea.Batch(cmds...)
}

func (ui *ChatUI) View() string {
	if ui.err != nil {
		return fmt.Sprintf("Error: %v\n", ui.err)
	}

	if !ui.ready {
		return fmt.Sprintf("Loading... %s", ui.spinner.View())
	}

	switch ui.state {
	case teammateSelectScreen:
		return ui.list.View()

	case chatScreen:
		return ui.renderChatScreen()
	}

	return ""
}

func (ui *ChatUI) sendMessage(message string) tea.Cmd {
	return func() tea.Msg {
		ctx, cancel := context.WithCancel(context.Background())
		ui.cancelFuncs = append(ui.cancelFuncs, cancel)
		msgChan, err := ui.client.SendMessage(ctx, ui.selected.UUID, message, "")
		if err != nil {
			return err
		}

		// Indicate that the bot is typing and start the spinner timer
		ui.isBotTyping = true
		ui.spinnerStartTime = time.Now()

		go func() {
			for {
				select {
				case <-ctx.Done():
					return
				case msg, ok := <-msgChan:
					if !ok {
						// Mark the last message as final when the channel is closed
						ui.P.Send(finalizeMessage{})
						return
					}
					// Set the sender name to the teammate's name
					msg.SenderName = ui.selected.Name
					ui.P.Send(msg)
				}
			}
		}()

		return nil
	}
}

func (ui *ChatUI) handleChatMessage(msg kubiya.ChatMessage) {
	// Handle tool execution messages
	if msg.Type == "tool" || msg.Type == "tool_output" {
		te, exists := ui.toolExecutions[msg.MessageID]
		if !exists {
			// First time seeing this tool execution, create new entry
			te = &toolExecution{
				startTime:        time.Now(),
				name:             "",
				messageID:        msg.MessageID,
				output:           "",
				isFinal:          false,
				hasInitMessage:   false,
				hasOutputMessage: false,
				isComplete:       false,
			}
			ui.toolExecutions[msg.MessageID] = te
		}

		// Update tool name if available
		if msg.Type == "tool" && msg.Content != "" {
			te.name = msg.Content
		}

		// Display tool initiation message only once
		if !te.hasInitMessage {
			te.hasInitMessage = true
			initMsg := kubiya.ChatMessage{
				Content:    te.name,
				SenderName: msg.SenderName,
				Timestamp:  msg.Timestamp,
				Type:       "tool_init",
				Final:      false,
				MessageID:  msg.MessageID,
			}
			ui.messages = append(ui.messages, initMsg)
		}

		// Accumulate output and update the message in ui.messages
		if msg.Content != "" && msg.Type == "tool_output" {
			te.output += msg.Content + "\n"

			// Update or add a message in ui.messages for the tool output
			if !te.hasOutputMessage {
				// First time adding the tool output message
				toolOutputMsg := kubiya.ChatMessage{
					Content:    te.output,
					SenderName: msg.SenderName,
					Timestamp:  msg.Timestamp,
					Type:       "tool_output",
					MessageID:  msg.MessageID,
					Final:      false,
				}
				ui.messages = append(ui.messages, toolOutputMsg)
				te.hasOutputMessage = true
			} else {
				// Update existing tool output message
				for i := len(ui.messages) - 1; i >= 0; i-- {
					if ui.messages[i].Type == "tool_output" && ui.messages[i].MessageID == msg.MessageID {
						ui.messages[i].Content = te.output
						ui.messages[i].Timestamp = msg.Timestamp
						break
					}
				}
			}
		}

		if msg.Final && !te.isComplete {
			// Tool execution finished
			te.isFinal = true
			te.isComplete = true
			duration := time.Since(te.startTime)

			// Update the tool output message to be final
			if te.hasOutputMessage {
				for i := len(ui.messages) - 1; i >= 0; i-- {
					if ui.messages[i].Type == "tool_output" && ui.messages[i].MessageID == msg.MessageID {
						ui.messages[i].Final = true
						break
					}
				}
			}

			// Append a message indicating the tool execution completion
			executionMsg := kubiya.ChatMessage{
				Content:    fmt.Sprintf("🔧 Executed `%s`\n\n⏱ Duration: %dms", te.name, duration.Milliseconds()),
				SenderName: msg.SenderName,
				Timestamp:  msg.Timestamp,
				Type:       "tool_execution_complete",
				Final:      true,
			}
			ui.messages = append(ui.messages, executionMsg)

			// Remove from ongoing tool executions
			delete(ui.toolExecutions, msg.MessageID)
			ui.toolRunning = false
			ui.currentToolMessageID = ""
		} else {
			// Tool execution in progress
			ui.toolRunning = true
			ui.toolName = te.name
			ui.currentToolMessageID = msg.MessageID
		}
		return // We handled the tool message
	}

	// Handle regular chat messages
	if msg.SenderName == ui.selected.Name {
		// Update typing indicator
		ui.isBotTyping = !msg.Final
		if ui.isBotTyping {
			ui.spinnerStartTime = time.Now() // Reset spinner timer
		}

		// Find if a message with the same MessageID already exists
		var existingMsg *kubiya.ChatMessage
		for i := len(ui.messages) - 1; i >= 0; i-- {
			if ui.messages[i].MessageID == msg.MessageID {
				existingMsg = &ui.messages[i]
				break
			}
		}

		if existingMsg != nil {
			// Update the existing message content and finality
			existingMsg.Content = msg.Content
			existingMsg.Final = msg.Final
			existingMsg.Timestamp = msg.Timestamp
		} else {
			// Append new message if it doesn't exist
			ui.messages = append(ui.messages, msg)
		}
	} else {
		// Message from the user or other sources; append it
		ui.messages = append(ui.messages, msg)
	}
}

func (ui *ChatUI) finalizeBotMessage() {
	if len(ui.messages) > 0 {
		lastMsg := &ui.messages[len(ui.messages)-1]
		if lastMsg.SenderName == ui.selected.Name {
			lastMsg.Final = true
		}
	}
}

func (ui *ChatUI) Run() error {
	p := tea.NewProgram(ui)
	ui.P = p
	return p.Start()
}

func (ui *ChatUI) fetchTeammates() tea.Msg {
	teammates, err := ui.client.ListTeammates(context.Background())
	if err != nil {
		return err
	}
	return teammates
}

func (ui *ChatUI) cancelContexts() {
	for _, cancel := range ui.cancelFuncs {
		cancel()
	}
	ui.cancelFuncs = nil
}

// finalizeMessage is used to mark the last teammate message as final when msgChan closes
type finalizeMessage struct{}

type teammateItem struct {
	teammate kubiya.Teammate
}

func (t teammateItem) Title() string       { return t.teammate.Name }
func (t teammateItem) Description() string { return t.teammate.Desc }
func (t teammateItem) FilterValue() string { return t.teammate.Name }

// Rendering the chat screen with enhanced UI and typing indicator
func (ui *ChatUI) renderChatScreen() string {
	var b strings.Builder

	// Header
	headerStyle := lipgloss.NewStyle().
		Bold(true).
		Background(lipgloss.Color("#6366F1")).
		Foreground(lipgloss.Color("#FFFFFF")).
		Padding(0, 1)

	b.WriteString(headerStyle.Render(fmt.Sprintf(" Chatting with %s ", ui.selected.Name)))
	b.WriteString("\n\n")

	// Messages rendering
	for _, msg := range ui.messages {
		timestamp := formatTimestamp(msg.Timestamp)
		var senderStyle, messageStyle lipgloss.Style

		if msg.SenderName == "You" {
			senderStyle = lipgloss.NewStyle().Foreground(lipgloss.Color("#10B981")).Bold(true)
			messageStyle = lipgloss.NewStyle().Foreground(lipgloss.Color("#A7F3D0"))
		} else {
			senderStyle = lipgloss.NewStyle().Foreground(lipgloss.Color("#3B82F6")).Bold(true)
			messageStyle = lipgloss.NewStyle().Foreground(lipgloss.Color("#BFDBFE"))
		}

		sender := senderStyle.Render(msg.SenderName)
		content := messageStyle.Render(msg.Content)

		// Check the message type and format accordingly
		switch msg.Type {
		case "tool_init":
			// Format tool execution initiation message
			toolInfoStyle := lipgloss.NewStyle().
				Border(lipgloss.RoundedBorder()).
				Padding(1).
				BorderForeground(lipgloss.Color("#F59E0B"))
			// Extract tool name and arguments from message content
			parts := strings.Split(msg.Content, "Arguments:")
			toolName := strings.TrimSpace(strings.TrimPrefix(parts[0], "Tool:"))
			var arguments string
			if len(parts) > 1 {
				arguments = strings.TrimSpace(parts[1])
			}

			var toolInfo string
			if arguments != "" {
				toolInfo = fmt.Sprintf("🔧 Running %s\nParameters:\n%s", toolName, arguments)
			} else {
				toolInfo = fmt.Sprintf("🔧 Running %s", toolName)
			}
			content = toolInfoStyle.Render(toolInfo)

		case "tool_output":
			// Format tool output message in a box
			toolOutputStyle := lipgloss.NewStyle().
				Border(lipgloss.NormalBorder()).
				Padding(1).
				Margin(0, 2).
				BorderForeground(lipgloss.Color("#FBBF24"))

			content = toolOutputStyle.Render(msg.Content)

		case "tool_execution_complete":
			// Format tool execution completion message
			toolStyle := lipgloss.NewStyle().
				Foreground(lipgloss.Color("#FBBF24")).
				Bold(true)

			content = toolStyle.Render(msg.Content)
		case "error":
			// Format error messages
			errorStyle := lipgloss.NewStyle().
				Foreground(lipgloss.Color("#EF4444")).
				Bold(true)
			content = errorStyle.Render("Error: " + msg.Content)
		}

		b.WriteString(fmt.Sprintf("[%s] %s: %s\n", timestamp, sender, content))
	}

	// Show typing indicator when bot is typing
	if ui.isBotTyping {
		typingStyle := lipgloss.NewStyle().
			Foreground(lipgloss.Color("#3B82F6")).
			Italic(true)

		// Create animated dots
		elapsed := time.Since(ui.spinnerStartTime).Milliseconds()
		dotsCount := (elapsed / 300) % 4 // Change every 300ms
		dots := strings.Repeat(".", int(dotsCount))

		typingIndicator := typingStyle.Render(fmt.Sprintf("%s is typing%s", ui.selected.Name, dots))
		b.WriteString(typingIndicator + "\n")
	}

	// Show tool execution indicator when a tool is running
	if ui.toolRunning {
		te := ui.toolExecutions[ui.currentToolMessageID]
		if te != nil {
			toolStyle := lipgloss.NewStyle().
				Foreground(lipgloss.Color("#FBBF24")).
				Bold(true)

			// Create animated dots for the execution indicator
			elapsed := time.Since(te.startTime).Milliseconds()
			dotsCount := (elapsed / 300) % 4
			dots := strings.Repeat(".", int(dotsCount))

			toolIndicator := toolStyle.Render(fmt.Sprintf("Executing `%s`%s", ui.toolName, dots))
			b.WriteString(toolIndicator + "\n")
		}
	}

	// Input prompt
	inputStyle := lipgloss.NewStyle().
		Foreground(lipgloss.Color("#9CA3AF")).
		Bold(true)
	prompt := inputStyle.Render("\n> ") + ui.inputBuffer
	b.WriteString(prompt)

	// Footer
	footerStyle := lipgloss.NewStyle().
		Foreground(lipgloss.Color("#6B7280"))

	b.WriteString(footerStyle.Render("\n\nPress 'esc' to go back to teammate selection."))

	return b.String()
}

// Helper function to format timestamp to HH:MM:SS
func formatTimestamp(ts string) string {
	t, err := time.Parse(time.RFC3339, ts)
	if err != nil {
		return ts // Return the original timestamp if parsing fails
	}
	return t.Format("15:04:05")
}
