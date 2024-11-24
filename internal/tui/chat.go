package tui

import (
	"context"
	"fmt"
	"strings"
	"time"

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

type ChatModel struct {
	client           *kubiya.Client
	messages         []kubiya.ChatMessage
	inputBuffer      string
	selectedTeammate kubiya.Teammate
	teammates        []kubiya.Teammate
	currentScreen    screenState
	spinner          spinner.Model
	thinking         bool
	width            int
	height           int
	cursor           int
	showTeamHelp     bool
}

func NewChatModel(cfg *config.Config, teammate ...kubiya.Teammate) (*ChatModel, error) {
	client := kubiya.NewClient(cfg)
	teammates, err := client.ListTeammates(context.Background())
	if err != nil {
		return nil, err
	}

	s := spinner.New()
	s.Spinner = spinner.Dot
	s.Style = lipgloss.NewStyle().Foreground(lipgloss.Color("205"))

	model := &ChatModel{
		client:        client,
		messages:      []kubiya.ChatMessage{},
		teammates:     teammates,
		currentScreen: chatScreen,
		spinner:       s,
		thinking:      false,
	}

	if len(teammate) > 0 {
		model.selectedTeammate = teammate[0]
		model.currentScreen = chatScreen
	}

	return model, nil
}

func (m *ChatModel) Init() tea.Cmd {
	return tea.Batch(
		m.spinner.Tick,
		m.fetchMessages(),
	)
}

func (m *ChatModel) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.WindowSizeMsg:
		m.width = msg.Width
		m.height = msg.Height

	case spinner.TickMsg:
		var cmd tea.Cmd
		m.spinner, cmd = m.spinner.Update(msg)
		return m, cmd

	case errMsg:
		// Handle error, maybe store it in the model to display
		return m, nil

	case tea.KeyMsg:
		switch m.currentScreen {
		case teammateSelectScreen:
			return m.handleTeammateSelectInput(msg)
		case chatScreen:
			return m.handleChatInput(msg)
		}

	case kubiya.ChatMessage:
		m.thinking = false
		m.messages = append(m.messages, msg)
		return m, m.fetchMessages()
	}

	return m, nil
}

func (m *ChatModel) handleTeammateSelectInput(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	switch msg.String() {
	case "up", "k":
		if m.cursor > 0 {
			m.cursor--
		}
	case "down", "j":
		if m.cursor < len(m.teammates)-1 {
			m.cursor++
		}
	case "?":
		m.showTeamHelp = !m.showTeamHelp
	case "enter":
		if len(m.teammates) > 0 {
			m.selectedTeammate = m.teammates[m.cursor]
			m.currentScreen = chatScreen
			return m, m.fetchMessages()
		}
	case "ctrl+c", "q":
		return m, tea.Quit
	}
	return m, nil
}

func (m *ChatModel) handleChatInput(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	switch msg.Type {
	case tea.KeyEnter:
		if m.inputBuffer == "" {
			return m, nil
		}
		m.thinking = true
		m.sendMessage(m.inputBuffer)
		m.inputBuffer = ""
		return m, tea.Batch(m.spinner.Tick, m.fetchMessages())

	case tea.KeyTab:
		m.currentScreen = teammateSelectScreen
		m.messages = []kubiya.ChatMessage{} // Clear messages when switching
		return m, nil

	case tea.KeyCtrlC, tea.KeyEsc:
		return m, tea.Quit

	case tea.KeyBackspace:
		if len(m.inputBuffer) > 0 {
			m.inputBuffer = m.inputBuffer[:len(m.inputBuffer)-1]
		}
		return m, nil

	default:
		if msg.Type == tea.KeyRunes {
			m.inputBuffer += string(msg.Runes)
		}
		return m, nil
	}
}

func (m *ChatModel) View() string {
	switch m.currentScreen {
	case teammateSelectScreen:
		return m.teammateSelectView()
	case chatScreen:
		return m.chatView()
	default:
		return "Unknown screen state"
	}
}

func (m *ChatModel) teammateSelectView() string {
	var b strings.Builder

	headerStyle := lipgloss.NewStyle().Bold(true).Foreground(lipgloss.Color("#00FF7F"))
	b.WriteString(headerStyle.Render("🤖 Select a Teammate to Chat With\n\n"))

	helpStyle := lipgloss.NewStyle().Foreground(lipgloss.Color("#666666"))
	b.WriteString(helpStyle.Render("↑/↓: Navigate  •  Enter: Select  •  ?: Toggle Help\n\n"))

	for i, t := range m.teammates {
		teammateStyle := lipgloss.NewStyle()
		prefix := "  "

		if i == m.cursor {
			teammateStyle = teammateStyle.
				Background(lipgloss.Color("#333333")).
				Bold(true)
			prefix = "▶ "
		}

		status := "🟢"
		name := fmt.Sprintf("%s %s", status, t.Name)

		b.WriteString(teammateStyle.Render(fmt.Sprintf("%s%s\n", prefix, name)))
		if t.Desc != "" {
			descStyle := lipgloss.NewStyle().Foreground(lipgloss.Color("#666666"))
			b.WriteString(descStyle.Render(fmt.Sprintf("    %s\n", t.Desc)))
		}
		b.WriteString("\n")
	}

	if m.showTeamHelp {
		b.WriteString("\nKeyboard Shortcuts:\n")
		b.WriteString(helpStyle.Render("• ↑/↓ or k/j: Navigate teammates\n"))
		b.WriteString(helpStyle.Render("• Enter: Start chat with selected teammate\n"))
		b.WriteString(helpStyle.Render("• Tab: Switch to next teammate during chat\n"))
		b.WriteString(helpStyle.Render("• Shift+Tab: Switch to previous teammate\n"))
		b.WriteString(helpStyle.Render("• Esc: Return to teammate selection\n"))
		b.WriteString(helpStyle.Render("• Ctrl+C: Quit\n"))
	}

	return b.String()
}

func (m *ChatModel) chatView() string {
	headerStyle := lipgloss.NewStyle().Bold(true).Foreground(lipgloss.Color("#00FF7F"))
	inputStyle := lipgloss.NewStyle().Foreground(lipgloss.Color("#FFA500"))
	messageStyle := lipgloss.NewStyle().Foreground(lipgloss.Color("#FFFFFF"))
	timestampStyle := lipgloss.NewStyle().Foreground(lipgloss.Color("#666666"))

	header := headerStyle.Render(fmt.Sprintf("💬 Chatting with %s", m.selectedTeammate.Name))
	shortcuts := lipgloss.NewStyle().
		Foreground(lipgloss.Color("#666666")).
		Render("(Tab/Shift+Tab: Switch Teammate • Esc: Back • Ctrl+C: Quit)")

	var teamInfo string
	if m.selectedTeammate.Desc != "" {
		teamInfo = lipgloss.NewStyle().
			Foreground(lipgloss.Color("#666666")).
			Render(fmt.Sprintf("\n%s", m.selectedTeammate.Desc))
	}

	var chatView string
	for _, msg := range m.messages {
		timestamp := timestampStyle.Render(msg.Timestamp.Format("15:04:05"))
		if msg.SenderName == "You" {
			chatView += messageStyle.Render(fmt.Sprintf("[%s] You: %s\n", timestamp, msg.Content))
		} else {
			chatView += messageStyle.Render(fmt.Sprintf("[%s] %s: %s\n", timestamp, msg.SenderName, msg.Content))
		}
	}

	input := inputStyle.Render("> " + m.inputBuffer)
	thinking := ""
	if m.thinking {
		thinking = fmt.Sprintf("\n%s Thinking...", m.spinner.View())
	}

	return fmt.Sprintf(
		"%s\n%s%s\n\n%s\n\n%s%s",
		header,
		shortcuts,
		teamInfo,
		chatView,
		input,
		thinking,
	)
}

func (m *ChatModel) fetchMessages() tea.Cmd {
	return func() tea.Msg {
		messagesChan, err := m.client.ReceiveMessages(context.Background(), m.selectedTeammate.UUID)
		if err != nil {
			return errMsg{err}
		}

		select {
		case msg, ok := <-messagesChan:
			if !ok {
				return nil
			}
			return msg
		}
	}
}

func (m *ChatModel) sendMessage(content string) {
	msg := kubiya.ChatMessage{
		SenderName: "You",
		Content:    content,
		Timestamp:  time.Now(),
	}
	m.messages = append(m.messages, msg)

	go m.client.SendMessage(context.Background(), m.selectedTeammate.UUID, content)
}

type errMsg struct {
	err error
}
