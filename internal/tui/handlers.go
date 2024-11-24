package tui

import (
	"fmt"
	"os"

	"github.com/charmbracelet/bubbles/textinput"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/kubiyabot/cli/internal/kubiya"
)

func (s *SourceBrowser) handleBack() tea.Cmd {
	switch s.state {
	case stateToolList:
		s.state = stateSourceList
	case stateToolDetail:
		s.state = stateToolList
	case stateExecuteTool:
		s.state = stateToolDetail
	case stateEnvVarSelect:
		s.state = stateExecuteTool
	case stateEnvVarOptions:
		s.state = stateEnvVarSelect
	case stateEnvVarValueInput:
		s.state = stateEnvVarSelect
	case stateExecuteConfirm:
		s.state = stateExecuteTool
	}
	return nil
}

func (s *SourceBrowser) handleEnter() tea.Cmd {
	switch s.state {
	case stateSourceList:
		if item, ok := s.sourceList.SelectedItem().(sourceItem); ok {
			s.currentSource = &item.source
			s.state = stateToolList
			return s.fetchTools()
		}
	case stateToolList:
		if item, ok := s.toolList.SelectedItem().(toolItem); ok {
			s.currentTool = &item.tool
			s.state = stateToolDetail
		}
	}
	return nil
}

func (s *SourceBrowser) handleWindowResize(msg tea.WindowSizeMsg) {
	s.width = msg.Width
	s.height = msg.Height

	if s.sourceList.Items() != nil {
		s.sourceList.SetWidth(msg.Width)
		s.sourceList.SetHeight(msg.Height - 4)
	}

	if s.toolList.Items() != nil {
		s.toolList.SetWidth(msg.Width)
		s.toolList.SetHeight(msg.Height - 4)
	}
}

func (s *SourceBrowser) handleExecuteToolInput(msg tea.KeyMsg) tea.Cmd {
	// First update the active input
	var cmd tea.Cmd
	if s.execution.activeInput < len(s.inputs) {
		s.inputs[s.execution.activeInput], cmd = s.inputs[s.execution.activeInput].Update(msg)
	}

	// Then handle navigation and submission
	switch msg.String() {
	case "tab":
		s.execution.activeInput = (s.execution.activeInput + 1) % len(s.inputs)
		for i := range s.inputs {
			if i == s.execution.activeInput {
				s.inputs[i].Focus()
			} else {
				s.inputs[i].Blur()
			}
		}
	case "shift+tab":
		s.execution.activeInput--
		if s.execution.activeInput < 0 {
			s.execution.activeInput = len(s.inputs) - 1
		}
		for i := range s.inputs {
			if i == s.execution.activeInput {
				s.inputs[i].Focus()
			} else {
				s.inputs[i].Blur()
			}
		}
	case "enter":
		// Store all inputs
		allValid := true
		for i, arg := range s.currentTool.Args {
			value := s.inputs[i].Value()
			if arg.Required && value == "" {
				allValid = false
				s.err = fmt.Errorf("Required argument %s cannot be empty", arg.Name)
				break
			}
			s.execution.args[arg.Name] = value
		}

		if allValid {
			if len(s.currentTool.Env) > 0 {
				// Initialize environment variables
				s.execution.envVarNames = s.currentTool.Env
				s.execution.activeInput = 0
				s.state = stateEnvVarSelect
				return nil // Don't execute yet, wait for env vars
			} else {
				// No environment variables needed, go straight to confirmation
				s.execution.prepared = true
				s.execution.confirmed = true // Mark as confirmed since no env vars needed
				s.state = stateExecuteConfirm
				return s.executeTool() // Now we can execute
			}
		}
	case "esc":
		s.state = stateToolDetail
	}

	return cmd
}

func (s *SourceBrowser) handleEnvVarSelectInput(msg tea.KeyMsg) tea.Cmd {
	switch msg.String() {
	case "up", "k":
		s.execution.activeInput = max(0, s.execution.activeInput-1)
	case "down", "j":
		s.execution.activeInput = min(len(s.execution.envVarNames)-1, s.execution.activeInput+1)
	case "e":
		// Use environment variable
		envName := s.execution.envVarNames[s.execution.activeInput]
		if val, exists := os.LookupEnv(envName); exists {
			s.execution.envVars[envName] = &kubiya.EnvVarStatus{Value: val}
			s.execution.activeInput = (s.execution.activeInput + 1) % len(s.execution.envVarNames)
		}
	case "s":
		// Use secret
		s.execution.currentEnvVarName = s.execution.envVarNames[s.execution.activeInput]
		s.state = stateEnvVarOptions
		s.execution.envVarOptions = []envVarOption{
			{icon: "🔑", label: "Use existing secret", value: "existing"},
			{icon: "➕", label: "Create new secret", value: "new"},
		}
	case "m":
		// Enter value manually
		s.execution.currentEnvVarName = s.execution.envVarNames[s.execution.activeInput]
		s.state = stateEnvVarValueInput
		s.execution.textInput = textinput.New()
		s.execution.textInput.Placeholder = "Enter value"
		s.execution.textInput.Focus()
	case " ":
		// Skip variable
		envName := s.execution.envVarNames[s.execution.activeInput]
		s.execution.envVars[envName] = nil
		s.execution.activeInput = (s.execution.activeInput + 1) % len(s.execution.envVarNames)
	case "enter":
		// Check if all required variables are set
		allSet := true
		for _, env := range s.execution.envVarNames {
			if s.execution.envVars[env] == nil {
				allSet = false
				break
			}
		}
		if allSet {
			s.execution.prepared = true
			s.execution.confirmed = true
			s.state = stateExecuteConfirm
			return s.executeTool()
		}
	case "esc":
		s.state = stateExecuteTool
	}
	return nil
}

func (s *SourceBrowser) handleEnvVarValueInput(msg tea.KeyMsg) tea.Cmd {
	var cmd tea.Cmd
	switch msg.String() {
	case "enter":
		envName := s.execution.currentEnvVarName
		s.execution.envVars[envName] = &kubiya.EnvVarStatus{
			Value: s.execution.textInput.Value(),
		}
		s.state = stateEnvVarSelect
	case "esc":
		s.state = stateEnvVarSelect
	default:
		s.execution.textInput, cmd = s.execution.textInput.Update(msg)
	}
	return cmd
}
