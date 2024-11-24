package tui

import (
	"fmt"
	"os"
	"os/signal"
	"strings"

	"github.com/charmbracelet/bubbles/help"
	"github.com/charmbracelet/bubbles/key"
	"github.com/charmbracelet/bubbles/paginator"
	"github.com/charmbracelet/bubbles/progress"
	"github.com/charmbracelet/bubbles/spinner"
	"github.com/charmbracelet/bubbles/viewport"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
	"github.com/kubiyabot/cli/internal/config"
	"github.com/kubiyabot/cli/internal/kubiya"
	"github.com/kubiyabot/cli/internal/style"
)

var (
	selectedItemStyle = lipgloss.NewStyle().
				Foreground(lipgloss.Color("205")).
				Bold(true)

	titleStyle = lipgloss.NewStyle().
			Foreground(lipgloss.Color("205")).
			Bold(true).
			Padding(0, 1)

	itemStyle = lipgloss.NewStyle().
			Foreground(lipgloss.Color("205"))
)

// Define the keyMap type and implement help.KeyMap interface
type keyMap struct {
	Up          key.Binding
	Down        key.Binding
	Enter       key.Binding
	Back        key.Binding
	Quit        key.Binding
	Help        key.Binding
	EnvVars     key.Binding
	Context     key.Binding
	Files       key.Binding
	Execute     key.Binding
	Filter      key.Binding
	ClearFilter key.Binding
	NextArg     key.Binding
	PrevArg     key.Binding
	CopyTeamEnv key.Binding
}

func (k keyMap) ShortHelp() []key.Binding {
	return []key.Binding{k.Help, k.Quit}
}

func (k keyMap) FullHelp() [][]key.Binding {
	return [][]key.Binding{
		{k.Enter, k.Back},
		{k.Help, k.Quit},
	}
}

// Define default keymap
var defaultKeyMap = keyMap{
	Up: key.NewBinding(
		key.WithKeys("up", "k"),
		key.WithHelp("↑/k", "up"),
	),
	Down: key.NewBinding(
		key.WithKeys("down", "j"),
		key.WithHelp("↓/j", "down"),
	),
	Enter: key.NewBinding(
		key.WithKeys("enter"),
		key.WithHelp("enter", "select"),
	),
	Back: key.NewBinding(
		key.WithKeys("esc"),
		key.WithHelp("esc", "back"),
	),
	Quit: key.NewBinding(
		key.WithKeys("q", "ctrl+c"),
		key.WithHelp("q", "quit"),
	),
	Help: key.NewBinding(
		key.WithKeys("?"),
		key.WithHelp("?", "help"),
	),
	EnvVars: key.NewBinding(
		key.WithKeys("e"),
		key.WithHelp("e", "env vars"),
	),
	Context: key.NewBinding(
		key.WithKeys("c"),
		key.WithHelp("c", "context"),
	),
	Files: key.NewBinding(
		key.WithKeys("f"),
		key.WithHelp("f", "files"),
	),
	Execute: key.NewBinding(
		key.WithKeys("x"),
		key.WithHelp("x", "execute"),
	),
	Filter: key.NewBinding(
		key.WithKeys("/"),
		key.WithHelp("/", "filter"),
	),
	ClearFilter: key.NewBinding(
		key.WithKeys("esc"),
		key.WithHelp("esc", "clear filter"),
	),
	NextArg: key.NewBinding(
		key.WithKeys("tab"),
		key.WithHelp("tab", "next arg"),
	),
	PrevArg: key.NewBinding(
		key.WithKeys("shift+tab"),
		key.WithHelp("shift+tab", "prev arg"),
	),
	CopyTeamEnv: key.NewBinding(
		key.WithKeys("t"),
		key.WithHelp("t", "copy team env"),
	),
}

// NewSourceBrowser creates a new source browser instance
func NewSourceBrowser(cfg *config.Config) *SourceBrowser {
	s := &SourceBrowser{
		cfg:    cfg,
		client: kubiya.NewClient(cfg),
		state:  stateSourceList,
		debug:  cfg.Debug,
		execution: executionState{
			args:        make(map[string]string),
			envVars:     make(map[string]*kubiya.EnvVarStatus),
			envVarNames: make([]string, 0),
			files:       make(map[string]string),
			activeInput: 0,
		},
		keys:      defaultKeyMap,
		help:      help.New(),
		progress:  progress.New(progress.WithDefaultGradient()),
		paginator: paginator.New(),
		showHelp:  false,
	}

	s.spinner = spinner.New()
	s.spinner.Spinner = spinner.Dot
	s.spinner.Style = lipgloss.NewStyle().Foreground(lipgloss.Color("205"))

	s.viewport = viewport.New(80, 20)
	s.viewport.Style = lipgloss.NewStyle().Border(lipgloss.RoundedBorder())

	s.help.ShowAll = true

	s.paginator.Type = paginator.Dots
	s.paginator.ActiveDot = style.HighlightStyle.Render("•")
	s.paginator.InactiveDot = style.DimStyle.Render("•")

	return s
}

// Run starts the source browser
func (s *SourceBrowser) Run() error {
	p := tea.NewProgram(s, tea.WithAltScreen())

	// Setup cleanup
	cleanup := func() {
		s.cleanup()
		fmt.Print("\033[H\033[2J")
	}
	defer cleanup()

	// Handle interrupt
	c := make(chan os.Signal, 1)
	signal.Notify(c, os.Interrupt)
	go func() {
		<-c
		cleanup()
		os.Exit(0)
	}()

	_, err := p.Run()
	return err
}

// Init initializes the source browser
func (s *SourceBrowser) Init() tea.Cmd {
	return tea.Batch(
		s.spinner.Tick,
		s.fetchSources(),
	)
}

// Update handles all UI updates
func (s *SourceBrowser) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	var cmds []tea.Cmd

	switch msg := msg.(type) {
	case tea.KeyMsg:
		if s.err != nil {
			s.clearError()
		}

		// Handle common key bindings
		switch {
		case key.Matches(msg, s.keys.Quit):
			return s, tea.Quit
		case key.Matches(msg, s.keys.Back):
			return s, s.handleBack()
		case key.Matches(msg, s.keys.Enter):
			// Add handling for execution confirmation
			if s.state == stateExecuteConfirm {
				s.execution.confirmed = true
				return s, s.executeTool()
			}
			return s, s.handleEnter()
		case key.Matches(msg, s.keys.Help):
			s.showHelp = !s.showHelp
		}

		// Handle state-specific input
		switch s.state {
		case stateToolDetail:
			if key.Matches(msg, s.keys.Execute) {
				s.state = stateExecuteTool
				return s, s.initializeExecution()
			}
		case stateExecuteTool:
			return s, s.handleExecuteToolInput(msg)
		case stateEnvVarSelect:
			return s, s.handleEnvVarSelectInput(msg)
		case stateEnvVarValueInput:
			return s, s.handleEnvVarValueInput(msg)
		case stateExecuteConfirm:
			switch msg.String() {
			case "enter":
				s.execution.confirmed = true
				return s, s.executeTool()
			case "esc":
				s.state = stateExecuteTool
			}
		}
	}

	// Handle window size
	if msg, ok := msg.(tea.WindowSizeMsg); ok {
		s.handleWindowResize(msg)
	}

	// Update components based on state
	switch s.state {
	case stateSourceList:
		if s.sourceList.Items() != nil {
			var cmd tea.Cmd
			s.sourceList, cmd = s.sourceList.Update(msg)
			cmds = append(cmds, cmd)
		}
	case stateToolList:
		if s.toolList.Items() != nil {
			var cmd tea.Cmd
			s.toolList, cmd = s.toolList.Update(msg)
			cmds = append(cmds, cmd)
		}
	case stateExecuteTool:
		if len(s.inputs) > 0 {
			for i := range s.inputs {
				var cmd tea.Cmd
				s.inputs[i], cmd = s.inputs[i].Update(msg)
				cmds = append(cmds, cmd)
			}
		}
	}

	// Update spinner
	var cmd tea.Cmd
	s.spinner, cmd = s.spinner.Update(msg)
	cmds = append(cmds, cmd)

	return s, tea.Batch(cmds...)
}

// View renders the UI
func (s *SourceBrowser) View() string {
	if s.err != nil {
		return fmt.Sprintf("\n  Error: %v\n", s.err)
	}

	if !s.ready {
		return fmt.Sprintf("\n  Initializing... %s", s.spinner.View())
	}

	var b strings.Builder

	// Main content based on state
	switch s.state {
	case stateSourceList:
		if s.sourceList.Items() == nil {
			return fmt.Sprintf("\n\n   Loading sources... %s\n\n", s.spinner.View())
		}
		b.WriteString(s.sourceList.View())
	case stateToolList:
		if s.toolList.Items() == nil {
			return fmt.Sprintf("\n\n   Loading tools... %s\n\n", s.spinner.View())
		}
		b.WriteString(s.toolList.View())
	case stateToolDetail:
		b.WriteString(s.renderToolDetail())
	case stateExecuteTool:
		b.WriteString(s.renderToolExecution())
	case stateEnvVarSelect:
		b.WriteString(s.renderEnvVarSelection())
	case stateEnvVarOptions:
		b.WriteString(s.renderEnvVarOptions())
	case stateEnvVarValueInput:
		b.WriteString(s.renderEnvVarValueInput())
	case stateExecuteConfirm:
		b.WriteString(s.renderExecutionConfirmation())
	case stateExecuting:
		b.WriteString(s.renderExecutionProgress())
	}

	// Add progress bar for execution
	if s.state == stateExecuting {
		b.WriteString("\n\n")
		b.WriteString(s.progress.View())
	}

	// Add paginator for lists
	if s.state == stateSourceList || s.state == stateToolList {
		b.WriteString("\n\n")
		b.WriteString(s.paginator.View())
	}

	// Add help
	if s.showHelp {
		b.WriteString("\n\n")
		b.WriteString(s.help.View(s.keys))
	}

	return b.String()
}

// Basic utility methods
func (s *SourceBrowser) cleanup() {
	if s.portForward.forwarder != nil {
		s.portForward.forwarder.Stop()
	}
	if s.portForward.cancel != nil {
		s.portForward.cancel()
	}
}

func (s *SourceBrowser) clearError() {
	s.err = nil
}

// Update the handleEnter() method to properly handle execution confirmation
func (s *SourceBrowser) handleExecutionConfirmation() tea.Cmd {
	switch s.state {
	case stateExecuteConfirm:
		if s.execution.prepared {
			s.execution.confirmed = true
			s.state = stateExecuting
			return s.executeTool()
		}
	}
	return nil
}

// Add validation before execution
func (s *SourceBrowser) validateExecution() error {
	if s.currentTool == nil {
		return fmt.Errorf("no tool selected")
	}

	// Validate required arguments
	for _, arg := range s.currentTool.Args {
		if arg.Required {
			if value, exists := s.execution.args[arg.Name]; !exists || value == "" {
				return fmt.Errorf("missing required argument: %s", arg.Name)
			}
		}
	}

	// Validate required env vars
	for _, env := range s.currentTool.Env {
		if value, exists := s.execution.envVars[env]; !exists || value == nil || value.Value == "" {
			return fmt.Errorf("missing required environment variable: %s", env)
		}
	}

	return nil
}
