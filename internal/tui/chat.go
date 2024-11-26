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

type ChatUI struct {
	cfg         *config.Config
	client      *kubiya.Client
	messages    []kubiya.ChatMessage
	inputBuffer string
	spinner     spinner.Model
	list        list.Model
	err         error
	teammates   []kubiya.Teammate
	selected    kubiya.Teammate
	width       int
	height      int
	state       screenState
	cursor      int
	ready       bool
	cancelFuncs []context.CancelFunc
	P           *tea.Program
	isBotTyping bool   // Indicates if the bot is currently typing/responding
	toolOutput  string // Stores the output from the tool execution
	toolRunning bool   // Indicates if a tool execution is in progress
}

func NewChatUI(cfg *config.Config) *ChatUI {
	s := spinner.New()
	s.Spinner = spinner.Dot

	delegate := list.NewDefaultDelegate()
	delegate.Styles.SelectedTitle = lipgloss.NewStyle().Foreground(lipgloss.Color("205")).Bold(true)

	uiList := list.New([]list.Item{}, delegate, 0, 0)
	uiList.Title = "Select a Teammate"
	uiList.SetShowStatusBar(false)
	uiList.SetFilteringEnabled(true)
	uiList.Styles.Title = lipgloss.NewStyle().Bold(true).Foreground(lipgloss.Color("205"))

	return &ChatUI{
		cfg:     cfg,
		client:  kubiya.NewClient(cfg),
		spinner: s,
		state:   teammateSelectScreen,
		list:    uiList,
	}
}

func (ui *ChatUI) Init() tea.Cmd {
	return tea.Batch(ui.spinner.Tick, ui.fetchTeammates)
}

func (ui *ChatUI) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
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
		// If the bot has started responding, set isBotTyping to true
		if msg.SenderName == ui.selected.Name && !msg.Final {
			ui.isBotTyping = true
		}
		// When the bot message is finalized, set isBotTyping to false
		if msg.Final {
			ui.isBotTyping = false
		}
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

	ui.spinner, cmd = ui.spinner.Update(msg)
	return ui, cmd
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

		// Add a placeholder message for the bot's response
		placeholderMsg := kubiya.ChatMessage{
			Content:    "",
			SenderName: ui.selected.Name,
			Timestamp:  time.Now().Format(time.RFC3339),
			Final:      false,
		}
		ui.messages = append(ui.messages, placeholderMsg)

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
		ui.toolRunning = !msg.Final // Update toolRunning status based on message finality
	}

	if msg.SenderName == ui.selected.Name {
		// Update the last message from the teammate if it's not final
		if len(ui.messages) > 0 {
			lastMsg := &ui.messages[len(ui.messages)-1]
			if lastMsg.SenderName == ui.selected.Name && !lastMsg.Final {
				// Update the content, finality, and timestamp
				lastMsg.Content = msg.Content
				lastMsg.Final = msg.Final
				lastMsg.Timestamp = msg.Timestamp
			} else {
				// Append a new message if the last one is final
				ui.messages = append(ui.messages, msg)
			}
		} else {
			// No previous messages; append the new message
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

// Rendering the chat screen with styling
func (ui *ChatUI) renderChatScreen() string {
	var b strings.Builder

	// Header
	headerStyle := lipgloss.NewStyle().
		Bold(true).
		Foreground(lipgloss.Color("205")).
		Padding(0, 1)

	b.WriteString(headerStyle.Render(fmt.Sprintf("Chatting with %s", ui.selected.Name)))
	b.WriteString("\n\n")

	// Messages rendering
	for _, msg := range ui.messages {
		timestamp := formatTimestamp(msg.Timestamp)
		var senderStyle, messageStyle lipgloss.Style

		if msg.SenderName == "You" {
			senderStyle = lipgloss.NewStyle().Foreground(lipgloss.Color("32")).Bold(true)
			messageStyle = lipgloss.NewStyle().Foreground(lipgloss.Color("32"))
		} else {
			senderStyle = lipgloss.NewStyle().Foreground(lipgloss.Color("33")).Bold(true)
			messageStyle = lipgloss.NewStyle().Foreground(lipgloss.Color("33"))
		}

		sender := senderStyle.Render(msg.SenderName)
		content := messageStyle.Render(msg.Content)

		// Check if the message is tool output and format accordingly
		if msg.Type == "tool_output" {
			toolStyle := lipgloss.NewStyle().
				Foreground(lipgloss.Color("244")).
				Italic(true)
			content = toolStyle.Render(msg.Content)
		}

		b.WriteString(fmt.Sprintf("[%s] %s: %s\n", timestamp, sender, content))
	}

	// Show "Bot is typing..." when bot is responding
	if ui.isBotTyping {
		typingStyle := lipgloss.NewStyle().
			Foreground(lipgloss.Color("33")).
			Italic(true)
		// show team mate name and "is typing..."
		b.WriteString(typingStyle.Render(fmt.Sprintf("%s is typing...", ui.selected.Name)))
	}

	// Input prompt
	// Disable input when bot is typing
	if !ui.isBotTyping {
		inputStyle := lipgloss.NewStyle().
			Foreground(lipgloss.Color("244")).
			Bold(true)
		prompt := inputStyle.Render("\n> ") + ui.inputBuffer
		b.WriteString(prompt)
	}

	// Footer
	footerStyle := lipgloss.NewStyle().
		Foreground(lipgloss.Color("240"))

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
