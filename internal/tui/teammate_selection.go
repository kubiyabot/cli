package tui

import (
	"context"
	"fmt"

	"github.com/charmbracelet/bubbles/key"
	"github.com/charmbracelet/bubbles/list"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
	"github.com/kubiyabot/cli/internal/config"
	"github.com/kubiyabot/cli/internal/kubiya"
)

// KeyMap defines the keybindings for the teammate selection
type KeyMap struct {
	Up     key.Binding
	Down   key.Binding
	Select key.Binding
	Quit   key.Binding
}

var DefaultKeyMap = KeyMap{
	Up: key.NewBinding(
		key.WithKeys("up", "k"),
		key.WithHelp("↑/k", "up"),
	),
	Down: key.NewBinding(
		key.WithKeys("down", "j"),
		key.WithHelp("↓/j", "down"),
	),
	Select: key.NewBinding(
		key.WithKeys("enter"),
		key.WithHelp("enter", "select"),
	),
	Quit: key.NewBinding(
		key.WithKeys("q", "ctrl+c", "esc"),
		key.WithHelp("q/ctrl+c/esc", "quit"),
	),
}

// teammateItem wraps kubiya.Teammate to implement list.Item
type teammateItem struct {
	teammate kubiya.Teammate
	warning  string
}

func (t teammateItem) Title() string {
	if t.warning != "" {
		return t.teammate.Name + " ⚠️"
	}
	return t.teammate.Name
}

func (t teammateItem) Description() string {
	if t.warning != "" {
		return t.warning + " - " + t.teammate.Desc
	}
	return t.teammate.Desc
}

func (t teammateItem) FilterValue() string { return t.teammate.Name }

type TeammateSelectionModel struct {
	list      list.Model
	cfg       *config.Config
	client    *kubiya.Client
	selected  kubiya.Teammate
	err       error
	keyMap    KeyMap
	loaded    bool
	teammates []kubiya.Teammate
	cursor    int
	width     int
	height    int
}

func NewTeammateSelectionModel(cfg *config.Config) (*TeammateSelectionModel, error) {
	client := kubiya.NewClient(cfg)

	delegate := list.NewDefaultDelegate()
	delegate.Styles.SelectedTitle = delegate.Styles.SelectedTitle.
		Foreground(lipgloss.Color("#00FF7F")).
		BorderForeground(lipgloss.Color("#00FF7F"))

	l := list.New([]list.Item{}, delegate, 0, 0)
	l.Title = "Select a Teammate"
	l.SetShowStatusBar(false)
	l.SetFilteringEnabled(false)
	l.Styles.Title = lipgloss.NewStyle().
		Bold(true).
		Foreground(lipgloss.Color("#00FF7F")).
		MarginLeft(2)

	return &TeammateSelectionModel{
		list:   l,
		cfg:    cfg,
		client: client,
		keyMap: DefaultKeyMap,
		loaded: false,
		cursor: 0,
	}, nil
}

func (m *TeammateSelectionModel) Init() tea.Cmd {
	return tea.Batch(
		m.fetchTeammates,
	)
}

func (m *TeammateSelectionModel) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	var cmds []tea.Cmd

	switch msg := msg.(type) {
	case tea.WindowSizeMsg:
		m.width = msg.Width
		m.height = msg.Height
		m.list.SetSize(msg.Width-4, msg.Height-4)

	case []kubiya.Teammate:
		m.teammates = msg
		items := make([]list.Item, len(msg))
		for i, t := range msg {
			warning := ""
			if t.Environment != nil {
				if runnerVersion, ok := t.Environment["RUNNER_VERSION"]; ok && runnerVersion == "v1" {
					warning = "Using deprecated v1 runner"
				}
			}
			items[i] = teammateItem{
				teammate: t,
				warning:  warning,
			}
		}
		m.list.SetItems(items)
		m.loaded = true
		if len(m.teammates) > 0 {
			m.selected = m.teammates[0]
		}

	case tea.KeyMsg:
		if !m.loaded {
			break
		}

		switch {
		case key.Matches(msg, m.keyMap.Quit):
			return m, tea.Quit

		case key.Matches(msg, m.keyMap.Select):
			if len(m.teammates) > 0 {
				currentIndex := m.list.Index()
				if currentIndex >= 0 && currentIndex < len(m.teammates) {
					selectedTeammate := m.teammates[currentIndex]
					if selectedTeammate.Environment != nil {
						if runnerVersion, ok := selectedTeammate.Environment["RUNNER_VERSION"]; ok && runnerVersion == "v1" {
							m.err = fmt.Errorf("cannot select teammate with v1 runner - please upgrade the runner first")
							return m, tea.Quit
						}
					}
					m.selected = selectedTeammate
					return m, tea.Quit
				}
			}

		case key.Matches(msg, m.keyMap.Up):
			currentIndex := m.list.Index()
			if currentIndex > 0 {
				m.list.CursorUp()
				newIndex := currentIndex - 1
				if newIndex >= 0 && newIndex < len(m.teammates) {
					m.selected = m.teammates[newIndex]
				}
			}

		case key.Matches(msg, m.keyMap.Down):
			currentIndex := m.list.Index()
			if currentIndex < len(m.teammates)-1 {
				m.list.CursorDown()
				newIndex := currentIndex + 1
				if newIndex >= 0 && newIndex < len(m.teammates) {
					m.selected = m.teammates[newIndex]
				}
			}
		}

	case error:
		m.err = msg
		return m, tea.Quit
	}

	newListModel, cmd := m.list.Update(msg)
	m.list = newListModel
	if cmd != nil {
		cmds = append(cmds, cmd)
	}

	return m, tea.Batch(cmds...)
}

func (m *TeammateSelectionModel) View() string {
	if m.err != nil {
		return lipgloss.NewStyle().
			Foreground(lipgloss.Color("#FF0000")).
			Render(fmt.Sprintf("\n  Error: %s\n  Press q to quit\n", m.err.Error()))
	}

	if !m.loaded {
		return "\n  Loading teammates...\n"
	}

	helpText := lipgloss.NewStyle().
		Foreground(lipgloss.Color("#666666")).
		Render("\n  ⚠️  Warning: Teammates with v1 runners need to be upgraded")

	return lipgloss.NewStyle().
		Margin(1).
		Render(m.list.View() + helpText)
}

func (m *TeammateSelectionModel) fetchTeammates() tea.Msg {
	teammates, err := m.client.ListTeammates(context.Background())
	if err != nil {
		return err
	}

	for i, teammate := range teammates {
		if teammate.Environment != nil {
			if runnerVersion, ok := teammate.Environment["RUNNER_VERSION"]; ok {
				if runnerVersion == "v1" {
					teammates[i].Desc = fmt.Sprintf("⚠️ Using deprecated v1 runner - %s", teammate.Desc)
				}
			}
		}
	}

	return teammates
}

func (m *TeammateSelectionModel) Selected() kubiya.Teammate {
	return m.selected
}
