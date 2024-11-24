package tui

import (
	"fmt"

	"github.com/charmbracelet/bubbles/textinput"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/kubiyabot/cli/internal/kubiya"
)

func (s *SourceBrowser) initializeExecution() tea.Cmd {
	return func() tea.Msg {
		// Reset execution state
		s.execution = executionState{
			args:        make(map[string]string),
			envVars:     make(map[string]*kubiya.EnvVarStatus),
			envVarNames: make([]string, 0),
			files:       make(map[string]string),
			activeInput: 0,
		}

		// Create inputs for required arguments
		s.inputs = make([]textinput.Model, len(s.currentTool.Args))
		for i, arg := range s.currentTool.Args {
			input := textinput.New()
			input.Placeholder = arg.Description
			input.Prompt = fmt.Sprintf("%s: ", arg.Name)
			input.Width = 40
			if i == 0 {
				input.Focus()
			}
			s.inputs[i] = input
		}

		return nil
	}
}
