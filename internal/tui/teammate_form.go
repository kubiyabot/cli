package tui

import (
	"github.com/charmbracelet/bubbles/textinput"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/kubiyabot/cli/internal/config"
	"github.com/kubiyabot/cli/internal/kubiya"
)

type TeammateForm struct {
	cfg    *config.Config
	inputs []textinput.Model
	focus  int
	done   bool
	err    error
}

func NewTeammateForm(cfg *config.Config) *TeammateForm {
	inputs := make([]textinput.Model, 4)

	// Name input
	inputs[0] = textinput.New()
	inputs[0].Placeholder = "DevOps Bot"
	inputs[0].Focus()
	inputs[0].CharLimit = 50
	inputs[0].Width = 30
	inputs[0].Prompt = "Name: "

	// Description input
	inputs[1] = textinput.New()
	inputs[1].Placeholder = "Handles DevOps tasks"
	inputs[1].CharLimit = 100
	inputs[1].Width = 50
	inputs[1].Prompt = "Description: "

	// LLM Model input
	inputs[2] = textinput.New()
	inputs[2].Placeholder = "azure/gpt-4"
	inputs[2].CharLimit = 30
	inputs[2].Width = 30
	inputs[2].Prompt = "LLM Model: "

	// Instruction Type input
	inputs[3] = textinput.New()
	inputs[3].Placeholder = "tools"
	inputs[3].CharLimit = 20
	inputs[3].Width = 20
	inputs[3].Prompt = "Type: "

	return &TeammateForm{
		cfg:    cfg,
		inputs: inputs,
		focus:  0,
	}
}

func (f *TeammateForm) SetDefaults(teammate *kubiya.Teammate) {
	f.inputs[0].SetValue(teammate.Name)
	f.inputs[1].SetValue(teammate.Description)
	f.inputs[2].SetValue(teammate.LLMModel)
	f.inputs[3].SetValue(teammate.InstructionType)
}

func (f *TeammateForm) Run() (*kubiya.Teammate, error) {
	p := tea.NewProgram(f)
	m, err := p.Run()
	if err != nil {
		return nil, err
	}

	form := m.(*TeammateForm)
	if form.err != nil {
		return nil, form.err
	}

	if !form.done {
		return nil, nil
	}

	return &kubiya.Teammate{
		Name:            f.inputs[0].Value(),
		Description:     f.inputs[1].Value(),
		LLMModel:        f.inputs[2].Value(),
		InstructionType: f.inputs[3].Value(),
	}, nil
}

func (f *TeammateForm) Init() tea.Cmd {
	return textinput.Blink
}

func (f *TeammateForm) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	var cmds []tea.Cmd

	switch msg := msg.(type) {
	case tea.KeyMsg:
		switch msg.Type {
		case tea.KeyEnter:
			if f.focus == len(f.inputs)-1 {
				f.done = true
				return f, tea.Quit
			}
			f.focus++
			cmds = append(cmds, f.inputs[f.focus].Focus())
			return f, tea.Batch(cmds...)

		case tea.KeyCtrlC, tea.KeyEsc:
			return f, tea.Quit

		case tea.KeyShiftTab, tea.KeyUp:
			f.focus--
			if f.focus < 0 {
				f.focus = len(f.inputs) - 1
			}

		case tea.KeyTab, tea.KeyDown:
			f.focus++
			if f.focus > len(f.inputs)-1 {
				f.focus = 0
			}
		}
	}

	for i := 0; i < len(f.inputs); i++ {
		if i == f.focus {
			f.inputs[i].Focus()
		} else {
			f.inputs[i].Blur()
		}
		var cmd tea.Cmd
		f.inputs[i], cmd = f.inputs[i].Update(msg)
		cmds = append(cmds, cmd)
	}

	return f, tea.Batch(cmds...)
}

func (f *TeammateForm) View() string {
	var s string
	s += "Create New Teammate\n\n"

	for i := 0; i < len(f.inputs); i++ {
		s += f.inputs[i].View() + "\n"
	}

	s += "\nPress tab to move between fields\n"
	s += "Press enter on the last field to submit\n"
	s += "Press ctrl+c to cancel\n"

	return s
}
