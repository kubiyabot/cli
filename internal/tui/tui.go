package tui

import (
	"context"
	"fmt"
	"strings"
	"time"

	"github.com/charmbracelet/bubbles/list"
	"github.com/charmbracelet/bubbles/spinner"
	"github.com/charmbracelet/bubbles/textinput"
	"github.com/charmbracelet/bubbles/viewport"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
	"github.com/kubiyabot/cli/internal/config"
	"github.com/kubiyabot/cli/internal/kubiya"
)

type App struct {
	client *kubiya.Client
	model  Model
}

// New creates a new TUI application
func New(cfg *config.Config) *App {
	client := kubiya.NewClient(cfg)

	// Initialize list
	delegate := NewItemDelegate()
	agents := list.New([]list.Item{}, delegate, 0, 0)
	agents.SetShowTitle(false)
	agents.SetShowStatusBar(false)
	agents.SetFilteringEnabled(true)

	// Initialize spinner
	s := spinner.New()
	s.Spinner = spinner.Dot
	s.Style = lipgloss.NewStyle().Foreground(lipgloss.Color("205"))

	// Initialize input
	i := textinput.New()
	i.Placeholder = "Type your message..."
	i.CharLimit = 156
	i.Width = 50

	// Initialize viewport
	v := viewport.New(60, 20)
	v.Style = lipgloss.NewStyle().Border(lipgloss.RoundedBorder())

	model := Model{
		agents:   agents,
		spinner:  s,
		input:    i,
		viewport: v,
		loading:  true,
		client:   client,
	}

	return &App{
		client: client,
		model:  model,
	}
}

// Run starts the TUI application
func (a *App) Run() error {
	p := tea.NewProgram(a.model, tea.WithAltScreen())
	_, err := p.Run()
	return err
}

// Model represents the TUI state
type Model struct {
	agents          list.Model
	spinner         spinner.Model
	input           textinput.Model
	viewport        viewport.Model
	sessions        []ChatSession
	activeSession   *ChatSession
	err             error
	loading         bool
	chatting        bool
	width, height   int
	showingSessions bool
	statusMsg       string
	client          *kubiya.Client
}

// ChatSession represents an active chat session
type ChatSession struct {
	Agent     Agent
	Messages  []string
	SessionID string
}

// Agent represents a Kubiya teammate
type Agent struct {
	UUID           string
	Name           string
	Desc           string
	AIInstructions string
}

// Implement list.Item interface for Agent
func (a Agent) Title() string {
	status := "🟢"
	if a.AIInstructions != "" {
		status = "🌟"
	}
	return fmt.Sprintf("%s %s", status, a.Name)
}

func (a Agent) Description() string {
	return a.Desc
}

func (a Agent) FilterValue() string {
	return a.Name
}

// Initialize the model
func (m Model) Init() tea.Cmd {
	return tea.Batch(
		m.spinner.Tick,
		m.fetchTeammates,
	)
}

// fetchTeammates retrieves the list of teammates
func (m Model) fetchTeammates() tea.Msg {
	teammates, err := m.client.ListTeammates(context.Background())
	if err != nil {
		return errMsg{err}
	}

	var items []list.Item
	for _, t := range teammates {
		items = append(items, Agent{
			UUID:           t.UUID,
			Name:           t.Name,
			Desc:           t.Desc,
			AIInstructions: t.AIInstructions,
		})
	}
	return teammatesMsg(items)
}

type teammatesMsg []list.Item
type errMsg struct{ error }

// NewItemDelegate creates a new delegate for list items
func NewItemDelegate() list.DefaultDelegate {
	d := list.NewDefaultDelegate()

	d.Styles.SelectedTitle = lipgloss.NewStyle().
		BorderLeft(true).
		BorderStyle(lipgloss.NormalBorder()).
		BorderForeground(lipgloss.Color("#FF75B7")).
		Foreground(lipgloss.Color("#FF75B7")).
		Bold(true).
		Padding(0, 0, 0, 1)

	d.Styles.SelectedDesc = d.Styles.SelectedTitle.Copy().
		Foreground(lipgloss.Color("#969696"))

	return d
}

// Update handles all the application updates
func (m Model) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	var cmds []tea.Cmd

	switch msg := msg.(type) {
	case tea.KeyMsg:
		switch msg.String() {
		case "ctrl+c", "q":
			return m, tea.Quit
		case "esc":
			if m.chatting {
				m.chatting = false
				m.activeSession = nil
				return m, nil
			}
		case "enter":
			if m.chatting {
				if m.input.Value() != "" {
					message := m.input.Value()
					m.activeSession.Messages = append(m.activeSession.Messages,
						fmt.Sprintf("👤 You: %s", message))
					m.updateViewport()
					m.input.Reset()
					return m, m.sendMessage(message)
				}
			} else {
				if i, ok := m.agents.SelectedItem().(Agent); ok {
					session := ChatSession{
						Agent:     i,
						SessionID: fmt.Sprintf("%d", time.Now().UnixNano()),
						Messages:  make([]string, 0),
					}
					m.sessions = append(m.sessions, session)
					m.activeSession = &m.sessions[len(m.sessions)-1]
					m.chatting = true
					m.updateViewport()
					return m, nil
				}
			}
		}

	case tea.WindowSizeMsg:
		m.width = msg.Width
		m.height = msg.Height

		if !m.chatting {
			m.agents.SetWidth(msg.Width)
			m.agents.SetHeight(msg.Height - 4)
		} else {
			m.viewport.Width = msg.Width
			m.viewport.Height = msg.Height - 6
			m.updateViewport()
		}

	case teammatesMsg:
		m.loading = false
		m.agents.SetItems(msg)

	case errMsg:
		m.err = msg.error
		return m, nil

	case spinner.TickMsg:
		var cmd tea.Cmd
		m.spinner, cmd = m.spinner.Update(msg)
		cmds = append(cmds, cmd)
	}

	// Handle component updates
	if m.chatting {
		var cmd tea.Cmd
		m.input, cmd = m.input.Update(msg)
		cmds = append(cmds, cmd)
	} else {
		var cmd tea.Cmd
		m.agents, cmd = m.agents.Update(msg)
		cmds = append(cmds, cmd)
	}

	return m, tea.Batch(cmds...)
}

// View renders the application UI
func (m Model) View() string {
	if m.err != nil {
		return fmt.Sprintf("\nError: %v\nPress any key to exit...", m.err)
	}

	if m.loading {
		return fmt.Sprintf("\n  %s Loading teammates...\n\n", m.spinner.View())
	}

	if m.chatting {
		var s strings.Builder
		s.WriteString(fmt.Sprintf("💬 Chatting with %s\n\n", m.activeSession.Agent.Name))
		s.WriteString(m.viewport.View())
		s.WriteString("\n\n")
		s.WriteString(m.input.View())
		s.WriteString("\n\nPress ESC to go back • Ctrl+C to quit")
		return s.String()
	}

	return fmt.Sprintf(
		"%s\n\n%s\n\n%s",
		"🤖 Select a teammate to chat with:",
		m.agents.View(),
		"↑/↓: Navigate • Enter: Select • /: Filter • Ctrl+C: Quit",
	)
}

// Helper method to update the viewport content
func (m *Model) updateViewport() {
	if m.activeSession == nil {
		return
	}

	var content strings.Builder
	for _, msg := range m.activeSession.Messages {
		content.WriteString(msg + "\n")
	}
	m.viewport.SetContent(content.String())
	m.viewport.GotoBottom()
}

// Helper method to send a message
func (m Model) sendMessage(message string) tea.Cmd {
	return func() tea.Msg {
		resp, err := m.client.SendMessage(
			context.Background(),
			m.activeSession.Agent.UUID,
			message,
		)
		if err != nil {
			return errMsg{err}
		}

		m.activeSession.Messages = append(m.activeSession.Messages,
			fmt.Sprintf("🤖 %s: %s", m.activeSession.Agent.Name, resp.Content))
		m.updateViewport()
		return nil
	}
}
