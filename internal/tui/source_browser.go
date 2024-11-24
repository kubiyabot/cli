package tui

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"os/exec"
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

var (
	titleStyle = lipgloss.NewStyle().
			Bold(true).
			Foreground(lipgloss.Color("#FAFAFA")).
			Background(lipgloss.Color("#7D56F4")).
			Padding(0, 1)

	itemStyle = lipgloss.NewStyle().
			Foreground(lipgloss.Color("#FAFAFA"))

	selectedItemStyle = lipgloss.NewStyle().
				Foreground(lipgloss.Color("#7D56F4")).
				Bold(true)

	toolStyle = lipgloss.NewStyle().
			Foreground(lipgloss.Color("#7D56F4")).
			Bold(true)
)

type state int

const (
	stateSourceList state = iota
	stateToolList
	stateToolDetail
	stateExecuteTool
)

type executionState struct {
	args      map[string]string
	envVars   map[string]string
	context   string
	teammate  string
	executing bool
	output    string
	error     error
}

type SourceBrowser struct {
	cfg           *config.Config
	client        *kubiya.Client
	state         state
	sources       []kubiya.Source
	currentSource *kubiya.Source
	currentTool   *kubiya.Tool
	sourceList    list.Model
	toolList      list.Model
	viewport      viewport.Model
	spinner       spinner.Model
	inputs        []textinput.Model
	err           error
	width         int
	height        int
	execution     executionState
	contexts      []string
	teammates     []kubiya.Teammate
	debug         bool
	portForward   struct {
		cmd    *exec.Cmd
		ready  bool
		cancel context.CancelFunc
	}
}

type sourceItem struct {
	source kubiya.Source
}

func (i sourceItem) Title() string       { return i.source.Name }
func (i sourceItem) Description() string { return i.source.Description }
func (i sourceItem) FilterValue() string { return i.source.Name }

type toolItem struct {
	tool kubiya.Tool
}

func (i toolItem) Title() string       { return i.tool.Name }
func (i toolItem) Description() string { return i.tool.Description }
func (i toolItem) FilterValue() string { return i.tool.Name }

type toolExecutedMsg struct {
	output string
	err    error
}

type portForwardMsg struct {
	ready bool
	err   error
}

type toolsLoadedMsg struct {
	tools []kubiya.Tool
	err   error
}

func NewSourceBrowser(cfg *config.Config) *SourceBrowser {
	s := &SourceBrowser{
		cfg:    cfg,
		client: kubiya.NewClient(cfg),
		state:  stateSourceList,
		debug:  cfg.Debug,
	}

	s.spinner = spinner.New()
	s.spinner.Spinner = spinner.Dot
	s.spinner.Style = lipgloss.NewStyle().Foreground(lipgloss.Color("205"))

	return s
}

func (s *SourceBrowser) Init() tea.Cmd {
	return tea.Batch(
		s.spinner.Tick,
		s.fetchSources(),
	)
}

func (s *SourceBrowser) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	var cmds []tea.Cmd

	// Handle window size first
	if msg, ok := msg.(tea.WindowSizeMsg); ok {
		s.width = msg.Width
		s.height = msg.Height

		// Don't try to set sizes until lists are initialized
		if s.sourceList.Items() != nil {
			s.sourceList.SetWidth(msg.Width)
			s.sourceList.SetHeight(msg.Height - 4)
		}
		if s.toolList.Items() != nil {
			s.toolList.SetWidth(msg.Width)
			s.toolList.SetHeight(msg.Height - 4)
		}
		if s.viewport.Width != msg.Width {
			s.viewport.Width = msg.Width
			s.viewport.Height = msg.Height - 6
		}
	}

	// Handle other messages
	switch msg := msg.(type) {
	case sourcesLoadedMsg:
		s.sources = []kubiya.Source(msg)
		return s, nil

	case errMsg:
		s.err = msg
		return s, nil

	case tea.KeyMsg:
		switch msg.String() {
		case "ctrl+c", "q":
			return s, tea.Quit
		case "esc":
			switch s.state {
			case stateToolList:
				s.state = stateSourceList
			case stateToolDetail:
				s.state = stateToolList
			case stateExecuteTool:
				s.state = stateToolDetail
			}
			return s, nil
		case "enter":
			switch s.state {
			case stateSourceList:
				if i, ok := s.sourceList.SelectedItem().(sourceItem); ok {
					s.currentSource = &i.source
					s.state = stateToolList
					delegate := list.NewDefaultDelegate()
					delegate.Styles.SelectedTitle = selectedItemStyle
					delegate.Styles.SelectedDesc = selectedItemStyle
					s.toolList = list.New([]list.Item{}, delegate, s.width, s.height-4)
					s.toolList.Title = fmt.Sprintf("Tools in %s", s.currentSource.Name)
					s.toolList.SetShowStatusBar(false)
					s.toolList.SetFilteringEnabled(true)
					s.toolList.Styles.Title = titleStyle
					s.toolList.Styles.FilterPrompt = itemStyle
					s.toolList.Styles.FilterCursor = itemStyle

					return s, s.loadTools(s.currentSource)
				}
			case stateToolList:
				if i, ok := s.toolList.SelectedItem().(toolItem); ok {
					s.currentTool = &i.tool
					s.state = stateToolDetail
					return s, nil
				}
			case stateToolDetail:
				s.state = stateExecuteTool
				return s, s.initToolExecution()
			case stateExecuteTool:
				// Validate required arguments
				var missingArgs []string
				for i, arg := range s.currentTool.Args {
					if arg.Required && strings.TrimSpace(s.inputs[i].Value()) == "" {
						missingArgs = append(missingArgs, arg.Name)
					}
				}

				if len(missingArgs) > 0 {
					s.execution.error = fmt.Errorf("missing required arguments: %s", strings.Join(missingArgs, ", "))
					return s, nil
				}

				// All required args are provided, execute the tool
				s.execution.executing = true
				s.execution.error = nil
				return s, s.executeTool()
			}
		case "tab":
			if s.state == stateExecuteTool {
				s.focusNextInput()
				return s, nil
			}
		case "shift+tab":
			if s.state == stateExecuteTool {
				s.focusPreviousInput()
				return s, nil
			}
		}
	case portForwardMsg:
		if msg.err != nil {
			s.execution.error = msg.err
			return s, nil
		}
		s.portForward.ready = true
		return s, nil
	case toolExecutedMsg:
		s.execution.executing = false
		if msg.err != nil {
			s.execution.error = msg.err
		} else {
			s.execution.output = msg.output
		}
		return s, nil
	case toolsLoadedMsg:
		if msg.err != nil {
			s.err = msg.err
			s.state = stateSourceList
			return s, nil
		}
		if s.currentSource != nil {
			s.currentSource.Tools = msg.tools
		}
		items := make([]list.Item, len(msg.tools))
		for i, tool := range msg.tools {
			items[i] = toolItem{tool: tool}
		}

		s.toolList.SetItems(items)
		return s, nil
	}

	// Update spinner
	var cmd tea.Cmd
	s.spinner, cmd = s.spinner.Update(msg)
	cmds = append(cmds, cmd)

	// Update active component
	switch s.state {
	case stateSourceList:
		if s.sourceList.Items() != nil {
			s.sourceList, cmd = s.sourceList.Update(msg)
			cmds = append(cmds, cmd)
		}
	case stateToolList:
		if s.toolList.Items() != nil {
			s.toolList, cmd = s.toolList.Update(msg)
			cmds = append(cmds, cmd)
		}
	case stateExecuteTool:
		for i := range s.inputs {
			s.inputs[i], cmd = s.inputs[i].Update(msg)
			cmds = append(cmds, cmd)
		}
	}

	return s, tea.Batch(cmds...)
}

func (s *SourceBrowser) View() string {
	if s.err != nil {
		return fmt.Sprintf("Error: %v\nPress q to quit", s.err)
	}

	if s.sourceList.Items() == nil {
		return fmt.Sprintf("\n\n   Loading sources... %s\n\n", s.spinner.View())
	}

	switch s.state {
	case stateSourceList:
		return fmt.Sprintf(
			"%s\n%s",
			titleStyle.Render(" Sources "),
			s.sourceList.View(),
		)
	case stateToolList:
		if s.toolList.Items() == nil {
			return fmt.Sprintf(
				"%s\n\n   Loading tools... %s\n",
				titleStyle.Render(fmt.Sprintf(" Tools in %s ", s.currentSource.Name)),
				s.spinner.View(),
			)
		}
		return fmt.Sprintf(
			"%s\n%s",
			titleStyle.Render(fmt.Sprintf(" Tools in %s ", s.currentSource.Name)),
			s.toolList.View(),
		)
	case stateToolDetail:
		return s.renderToolDetail()
	case stateExecuteTool:
		return s.renderToolExecution()
	default:
		return "Loading..."
	}
}

func (s *SourceBrowser) fetchSources() tea.Cmd {
	return func() tea.Msg {
		sources, err := s.client.ListSources(context.Background())
		if err != nil {
			return errMsg{err}
		}

		items := make([]list.Item, len(sources))
		for i, source := range sources {
			items[i] = sourceItem{source: source}
		}

		delegate := list.NewDefaultDelegate()
		delegate.Styles.SelectedTitle = selectedItemStyle
		delegate.Styles.SelectedDesc = selectedItemStyle

		s.sourceList = list.New(items, delegate, s.width, s.height-4)
		s.sourceList.Title = "Sources"
		s.sourceList.SetShowStatusBar(false)
		s.sourceList.SetFilteringEnabled(true)
		s.sourceList.Styles.Title = titleStyle
		s.sourceList.Styles.FilterPrompt = itemStyle
		s.sourceList.Styles.FilterCursor = itemStyle

		return sourcesLoadedMsg(sources)
	}
}

func (s *SourceBrowser) loadTools(source *kubiya.Source) tea.Cmd {
	return func() tea.Msg {
		metadata, err := s.client.GetSourceMetadata(context.Background(), source.UUID)
		if err != nil {
			if s.debug {
				fmt.Printf("Error loading tools: %v\n", err)
			}
			return toolsLoadedMsg{err: err}
		}

		if s.debug {
			fmt.Printf("Loaded %d tools for source %s\n", len(metadata.Tools), source.Name)
		}

		return toolsLoadedMsg{tools: metadata.Tools}
	}
}

func (s *SourceBrowser) renderToolDetail() string {
	if s.currentTool == nil {
		return "No tool selected"
	}

	var b strings.Builder
	b.WriteString(titleStyle.Render(fmt.Sprintf(" Tool: %s ", s.currentTool.Name)))
	b.WriteString("\n\n")

	if s.currentTool.Description != "" {
		b.WriteString(fmt.Sprintf("Description: %s\n", s.currentTool.Description))
	}
	b.WriteString(fmt.Sprintf("Type: %s\n", s.currentTool.Type))

	if len(s.currentTool.Args) > 0 {
		b.WriteString("\nArguments:\n")
		for _, arg := range s.currentTool.Args {
			required := "optional"
			if arg.Required {
				required = "required"
			}
			b.WriteString(fmt.Sprintf("  • %s: %s (%s)\n", arg.Name, arg.Description, required))
		}
	}

	if len(s.currentTool.Env) > 0 {
		b.WriteString("\nEnvironment Variables:\n")
		for _, env := range s.currentTool.Env {
			b.WriteString(fmt.Sprintf("  • %s\n", env))
		}
	}

	b.WriteString("\nPress Enter to execute this tool, Esc to go back")
	return b.String()
}

func (s *SourceBrowser) initToolExecution() tea.Cmd {
	if s.currentTool == nil {
		return nil
	}

	s.execution = executionState{
		args:    make(map[string]string),
		envVars: make(map[string]string),
	}

	// Create input fields only for required arguments
	var inputs []textinput.Model
	for _, arg := range s.currentTool.Args {
		if arg.Required {
			input := textinput.New()
			input.Placeholder = arg.Description
			input.Prompt = "" // We'll show the prompt in the UI
			input.Width = 40
			inputs = append(inputs, input)
		}
	}
	s.inputs = inputs

	// Focus the first input if available
	if len(s.inputs) > 0 {
		s.inputs[0].Focus()
	}

	return nil
}

func (s *SourceBrowser) renderToolExecution() string {
	if s.currentTool == nil {
		return "No tool selected"
	}

	var b strings.Builder
	b.WriteString(titleStyle.Render(fmt.Sprintf(" Execute: %s ", s.currentTool.Name)))
	b.WriteString("\n\n")

	if s.currentTool.Description != "" {
		b.WriteString(fmt.Sprintf("Description: %s\n\n", s.currentTool.Description))
	}

	// Show arguments section
	if len(s.currentTool.Args) > 0 {
		b.WriteString(titleStyle.Render(" Required Arguments "))
		b.WriteString("\n\n")

		// Show focused state more clearly
		for i, input := range s.inputs {
			arg := s.currentTool.Args[i]
			if arg.Required {
				prefix := "  "
				if input.Focused() {
					prefix = "→ "
				}
				b.WriteString(fmt.Sprintf("%s%s: %s\n", prefix, arg.Name, input.View()))
				if arg.Description != "" {
					b.WriteString(fmt.Sprintf("    ℹ️  %s\n", arg.Description))
				}
				b.WriteString("\n")
			}
		}
	}

	// Show execution status
	if s.execution.executing {
		b.WriteString("\n⏳ Executing...\n")
	} else if s.execution.error != nil {
		b.WriteString(fmt.Sprintf("\n❌ Error: %v\n", s.execution.error))
	} else if s.execution.output != "" {
		b.WriteString("\n✅ Output:\n")
		b.WriteString(s.execution.output)
	}

	// Show simplified help
	b.WriteString("\n\nControls:\n")
	b.WriteString("• Tab/Shift+Tab: Navigate fields\n")
	b.WriteString("• Enter: Execute tool\n")
	b.WriteString("• Esc: Go back\n")

	return b.String()
}

func (s *SourceBrowser) setupPortForward() tea.Cmd {
	return func() tea.Msg {
		// Cancel existing port-forward if any
		if s.portForward.cancel != nil {
			s.portForward.cancel()
		}

		// Create cancelable context
		ctx, cancel := context.WithCancel(context.Background())
		s.portForward.cancel = cancel

		// Start port-forward
		cmd := exec.CommandContext(ctx, "kubectl", "port-forward", "-n", "kubiya", "svc/tool-manager", "5001:5001")
		if s.execution.context != "" {
			cmd.Args = append([]string{"--context", s.execution.context}, cmd.Args...)
		}

		// Capture output for debugging
		var stderr bytes.Buffer
		cmd.Stderr = &stderr

		if err := cmd.Start(); err != nil {
			return portForwardMsg{err: fmt.Errorf("failed to start port-forward: %w", err)}
		}

		s.portForward.cmd = cmd

		// Wait for port-forward to be ready
		time.Sleep(2 * time.Second)
		if stderr.Len() > 0 {
			return portForwardMsg{err: fmt.Errorf("port-forward error: %s", stderr.String())}
		}

		return portForwardMsg{ready: true}
	}
}

func (s *SourceBrowser) executeTool() tea.Cmd {
	return func() tea.Msg {
		// Setup port-forward first
		if !s.portForward.ready {
			msg := s.setupPortForward()()
			if pfMsg, ok := msg.(portForwardMsg); ok {
				if pfMsg.err != nil {
					return toolExecutedMsg{err: fmt.Errorf("port-forward setup failed: %w", pfMsg.err)}
				}
			}
		}

		// Collect arguments
		args := make(map[string]string)
		for i, input := range s.inputs {
			args[s.currentTool.Args[i].Name] = input.Value()
		}

		// Prepare execution request
		execReq := struct {
			ToolName  string            `json:"tool_name"`
			SourceURL string            `json:"source_url"`
			ArgMap    map[string]string `json:"arg_map"`
			EnvVars   map[string]string `json:"env_vars"`
			Async     bool              `json:"async"`
		}{
			ToolName:  s.currentTool.Name,
			SourceURL: s.currentSource.URL,
			ArgMap:    args,
			EnvVars:   s.execution.envVars,
			Async:     false,
		}

		// Send execution request
		jsonData, err := json.Marshal(execReq)
		if err != nil {
			return toolExecutedMsg{err: fmt.Errorf("failed to marshal request: %w", err)}
		}

		resp, err := http.Post("http://localhost:5001/tool/execute", "application/json", bytes.NewBuffer(jsonData))
		if err != nil {
			return toolExecutedMsg{err: fmt.Errorf("failed to execute tool: %w", err)}
		}
		defer resp.Body.Close()

		var execResp struct {
			Status      string `json:"status"`
			ExecutionID string `json:"execution_id,omitempty"`
			Output      string `json:"output,omitempty"`
			Error       string `json:"error,omitempty"`
		}

		if err := json.NewDecoder(resp.Body).Decode(&execResp); err != nil {
			return toolExecutedMsg{err: fmt.Errorf("failed to decode response: %w", err)}
		}

		if execResp.Error != "" {
			return toolExecutedMsg{err: fmt.Errorf("execution failed: %s", execResp.Error)}
		}

		return toolExecutedMsg{output: execResp.Output}
	}
}

type sourcesLoadedMsg []kubiya.Source

func (e errMsg) Error() string { return e.error.Error() }

func (s *SourceBrowser) Run() error {
	p := tea.NewProgram(s, tea.WithAltScreen())

	// Cleanup function
	defer func() {
		if s.portForward.cancel != nil {
			s.portForward.cancel()
		}
		if s.portForward.cmd != nil && s.portForward.cmd.Process != nil {
			s.portForward.cmd.Process.Kill()
		}
	}()

	_, err := p.Run()
	return err
}

func (s *SourceBrowser) focusNextInput() {
	for i := 0; i < len(s.inputs); i++ {
		if s.inputs[i].Focused() {
			s.inputs[i].Blur()
			next := (i + 1) % len(s.inputs)
			s.inputs[next].Focus()
			return
		}
	}
	// If no input is focused, focus the first one
	if len(s.inputs) > 0 {
		s.inputs[0].Focus()
	}
}

func (s *SourceBrowser) focusPreviousInput() {
	for i := 0; i < len(s.inputs); i++ {
		if s.inputs[i].Focused() {
			s.inputs[i].Blur()
			prev := (i - 1 + len(s.inputs)) % len(s.inputs)
			s.inputs[prev].Focus()
			return
		}
	}
	// If no input is focused, focus the last one
	if len(s.inputs) > 0 {
		s.inputs[len(s.inputs)-1].Focus()
	}
}
