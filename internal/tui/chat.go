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

type tickMsg struct{}

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
	program          *tea.Program
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
		currentScreen: teammateSelectScreen,
		spinner:       s,
		thinking:      false,
	}

	if len(teammate) > 0 {
		model.selectedTeammate = teammate[0]
		model.currentScreen = chatScreen
	}

	p := tea.NewProgram(model, tea.WithAltScreen())
	model.program = p

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
		m.handleError(msg.err)
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
	case "enter", " ":
		if len(m.teammates) > 0 {
			m.selectedTeammate = m.teammates[m.cursor]
			m.currentScreen = chatScreen
			m.messages = []kubiya.ChatMessage{} // Clear messages when switching
			return m, tea.Batch(
				m.spinner.Tick,
				m.fetchMessages(),
			)
		}
	case "ctrl+c", "q", "esc":
		return m, tea.Quit
	}
	return m, nil
}

func (m *ChatModel) handleChatInput(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	switch msg.Type {
	case tea.KeyEnter:
		if m.inputBuffer == "" || m.thinking {
			return m, nil
		}

		message := m.inputBuffer
		m.inputBuffer = ""
		m.thinking = true

		go func() {
			if err := m.sendMessage(message); err != nil {
				m.program.Send(errMsg{err})
			}
		}()

		return m, tea.Batch(m.spinner.Tick)

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
	b.WriteString(helpStyle.Render("↑/↓: Navigate  •  Enter/Space: Select  •  ?: Toggle Help  •  Esc: Quit\n\n"))

	selectedStyle := lipgloss.NewStyle().
		Bold(true).
		Foreground(lipgloss.Color("#FFFFFF")).
		Background(lipgloss.Color("#333333")).
		Padding(0, 1)

	for i, t := range m.teammates {
		if i == m.cursor {
			b.WriteString(fmt.Sprintf("▶ %s", selectedStyle.Render(fmt.Sprintf("%s %s", "🟢", t.Name))))
			if t.Desc != "" {
				descStyle := lipgloss.NewStyle().Foreground(lipgloss.Color("#666666"))
				b.WriteString("\n    " + descStyle.Render(t.Desc))
			}
		} else {
			b.WriteString(fmt.Sprintf("  %s %s", "🟢", t.Name))
			if t.Desc != "" {
				descStyle := lipgloss.NewStyle().Foreground(lipgloss.Color("#666666"))
				b.WriteString("\n    " + descStyle.Render(t.Desc))
			}
		}
		b.WriteString("\n\n")
	}

	if m.showTeamHelp {
		b.WriteString("\nKeyboard Shortcuts:\n")
		b.WriteString(helpStyle.Render("• ↑/↓ or k/j: Navigate teammates\n"))
		b.WriteString(helpStyle.Render("• Enter/Space: Start chat with selected teammate\n"))
		b.WriteString(helpStyle.Render("• Tab: Switch teammate during chat\n"))
		b.WriteString(helpStyle.Render("• Esc: Quit\n"))
		b.WriteString(helpStyle.Render("• ?: Toggle help\n"))
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
		t, err := time.Parse(time.RFC3339, msg.Timestamp)
		timestamp := msg.Timestamp
		if err == nil {
			timestamp = t.Format("15:04:05")
		}
		timestamp = timestampStyle.Render(timestamp)
		if msg.SenderName == "You" {
			chatView += messageStyle.Render(fmt.Sprintf("[%s] You: %s\n", timestamp, msg.Content))
		} else {
			chatView += messageStyle.Render(fmt.Sprintf("[%s] %s: %s\n", timestamp, "Bot", msg.Content))
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
		if m.selectedTeammate.UUID == "" {
			return nil
		}

		msgChan, err := m.client.ReceiveMessages(context.Background(), m.selectedTeammate.UUID)
		if err != nil {
			return errMsg{err}
		}

		for msg := range msgChan {
			if msg.Error != "" {
				return errMsg{fmt.Errorf(msg.Error)}
			}
			return msg
		}

		return nil
	}
}

func (m *ChatModel) sendMessage(msg string) error {
	if m.selectedTeammate.UUID == "" {
		return fmt.Errorf("no teammate selected")
	}

	m.messages = append(m.messages, kubiya.ChatMessage{
		Content:    msg,
		SenderName: "You",
		Timestamp:  time.Now().Format(time.RFC3339),
	})

	msgChan, err := m.client.SendMessage(context.Background(), m.selectedTeammate.UUID, msg, "")
	if err != nil {
		return err
	}

	go func() {
		defer func() {
			m.thinking = false
			m.program.Send(tickMsg{}) // Force UI update
		}()

		for msg := range msgChan {
			if msg.Error != "" {
				m.program.Send(errMsg{fmt.Errorf(msg.Error)})
				return
			}
			m.program.Send(msg)
		}
	}()

	return nil
}

type errMsg struct {
	err error
}

func (m *ChatModel) handleError(err error) {
	m.thinking = false
	m.messages = append(m.messages, kubiya.ChatMessage{
		Content:    fmt.Sprintf("Error: %v", err),
		SenderName: "System",
		Timestamp:  time.Now().Format(time.RFC3339),
	})
}
