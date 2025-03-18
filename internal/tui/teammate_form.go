package tui

import (
	"context"
	"fmt"

	"github.com/charmbracelet/bubbles/list"
	"github.com/charmbracelet/bubbles/textinput"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
	"github.com/kubiyabot/cli/internal/config"
	"github.com/kubiyabot/cli/internal/kubiya"
)

// Item represents an item in a list
type Item struct {
	TitleText       string
	DescriptionText string
	Id              string
}

func (i Item) Title() string       { return i.TitleText }
func (i Item) Description() string { return i.DescriptionText }
func (i Item) FilterValue() string { return i.TitleText }

// FormPage represents different pages in the TUI
type FormPage int

const (
	BasicInfoPage FormPage = iota
	SourcesPage
	SecretsPage
	EnvVarsPage
	IntegrationsPage
	ConfirmPage
)

// TeammateForm represents the TUI form
type TeammateForm struct {
	cfg              *config.Config
	client           *kubiya.Client
	inputs           []textinput.Model
	sourcesInput     textinput.Model
	secretsInput     textinput.Model
	envKeyInput      textinput.Model
	envValueInput    textinput.Model
	integrationInput textinput.Model

	// Data
	sources          []kubiya.Source
	secrets          []kubiya.Secret
	availableSources []Item
	availableSecrets []Item

	// Lists
	sourcesList list.Model
	secretsList list.Model

	// State
	focus           int
	done            bool
	err             error
	page            FormPage
	selectedSources []string
	selectedSecrets []string
	envVars         map[string]string
	integrations    []string
	addingEnvVar    bool
	currentEnvKey   string

	// Display
	width, height int
}

func NewTeammateForm(cfg *config.Config) *TeammateForm {
	// Initialize basic inputs
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

	// Initialize sources input
	sourcesInput := textinput.New()
	sourcesInput.Placeholder = "Enter source ID to add"
	sourcesInput.CharLimit = 50
	sourcesInput.Width = 40
	sourcesInput.Prompt = "Source ID: "

	// Initialize secrets input
	secretsInput := textinput.New()
	secretsInput.Placeholder = "Enter secret name to add"
	secretsInput.CharLimit = 50
	secretsInput.Width = 40
	secretsInput.Prompt = "Secret name: "

	// Initialize env var inputs
	envKeyInput := textinput.New()
	envKeyInput.Placeholder = "ENV_VAR_NAME"
	envKeyInput.CharLimit = 50
	envKeyInput.Width = 30
	envKeyInput.Prompt = "Key: "

	envValueInput := textinput.New()
	envValueInput.Placeholder = "value"
	envValueInput.CharLimit = 100
	envValueInput.Width = 40
	envValueInput.Prompt = "Value: "

	// Initialize integration input
	integrationInput := textinput.New()
	integrationInput.Placeholder = "github, aws, slack, etc."
	integrationInput.CharLimit = 30
	integrationInput.Width = 30
	integrationInput.Prompt = "Integration: "

	// Initialize sources list
	sourcesList := list.New([]list.Item{}, list.NewDefaultDelegate(), 0, 0)
	sourcesList.Title = "Available Sources"
	sourcesList.SetShowStatusBar(false)
	sourcesList.SetFilteringEnabled(true)
	sourcesList.Styles.Title = lipgloss.NewStyle().MarginLeft(2).Bold(true)

	// Initialize secrets list
	secretsList := list.New([]list.Item{}, list.NewDefaultDelegate(), 0, 0)
	secretsList.Title = "Available Secrets"
	secretsList.SetShowStatusBar(false)
	secretsList.SetFilteringEnabled(true)
	secretsList.Styles.Title = lipgloss.NewStyle().MarginLeft(2).Bold(true)

	return &TeammateForm{
		cfg:              cfg,
		client:           kubiya.NewClient(cfg),
		inputs:           inputs,
		sourcesInput:     sourcesInput,
		secretsInput:     secretsInput,
		envKeyInput:      envKeyInput,
		envValueInput:    envValueInput,
		integrationInput: integrationInput,
		sourcesList:      sourcesList,
		secretsList:      secretsList,
		focus:            0,
		page:             BasicInfoPage,
		selectedSources:  []string{},
		selectedSecrets:  []string{},
		envVars:          make(map[string]string),
		integrations:     []string{},
		addingEnvVar:     false,
	}
}

func (f *TeammateForm) SetDefaults(teammate *kubiya.Teammate) {
	// Set basic info
	f.inputs[0].SetValue(teammate.Name)
	f.inputs[1].SetValue(teammate.Description)
	f.inputs[2].SetValue(teammate.LLMModel)
	f.inputs[3].SetValue(teammate.InstructionType)

	// Set sources, secrets, env vars, and integrations
	f.selectedSources = teammate.Sources
	f.selectedSecrets = teammate.Secrets
	f.envVars = teammate.Environment
	f.integrations = teammate.Integrations
}

func (f *TeammateForm) Run() (*kubiya.Teammate, error) {
	// Fetch available sources and secrets for selection
	go f.fetchSources()
	go f.fetchSecrets()

	p := tea.NewProgram(f, tea.WithAltScreen())
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
		Sources:         f.selectedSources,
		Secrets:         f.selectedSecrets,
		Environment:     f.envVars,
		Integrations:    f.integrations,
	}, nil
}

func (f *TeammateForm) fetchSources() {
	ctx := context.Background()
	sources, err := f.client.ListSources(ctx)
	if err != nil {
		return
	}

	f.sources = sources
	var items []list.Item
	for _, source := range sources {
		items = append(items, Item{
			TitleText:       source.Name,
			DescriptionText: fmt.Sprintf("UUID: %s, Type: %s", source.UUID, source.Type),
			Id:              source.UUID,
		})
	}

	f.availableSources = make([]Item, len(items))
	for i, item := range items {
		f.availableSources[i] = item.(Item)
	}

	// Convert to list.Item slice
	listItems := make([]list.Item, len(f.availableSources))
	for i, item := range f.availableSources {
		listItems[i] = item
	}

	// Update the list - this must be done in the update function
	f.sourcesList.SetItems(listItems)
}

func (f *TeammateForm) fetchSecrets() {
	ctx := context.Background()
	secrets, err := f.client.ListSecrets(ctx)
	if err != nil {
		return
	}

	f.secrets = secrets
	var items []list.Item
	for _, secret := range secrets {
		items = append(items, Item{
			TitleText:       secret.Name,
			DescriptionText: secret.Description,
			Id:              secret.Name,
		})
	}

	f.availableSecrets = make([]Item, len(items))
	for i, item := range items {
		f.availableSecrets[i] = item.(Item)
	}

	// Convert to list.Item slice
	listItems := make([]list.Item, len(f.availableSecrets))
	for i, item := range f.availableSecrets {
		listItems[i] = item
	}

	// Update the list
	f.secretsList.SetItems(listItems)
}

func (f *TeammateForm) Init() tea.Cmd {
	cmds := []tea.Cmd{textinput.Blink}
	return tea.Batch(cmds...)
}

func (f *TeammateForm) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	var cmds []tea.Cmd

	switch msg := msg.(type) {
	case tea.KeyMsg:
		switch msg.String() {
		case "ctrl+c", "esc":
			return f, tea.Quit

		case "tab":
			if f.page == BasicInfoPage {
				f.focus = (f.focus + 1) % len(f.inputs)
				cmds = append(cmds, f.inputs[f.focus].Focus())
			} else if f.page == EnvVarsPage && f.addingEnvVar {
				if f.focus == 0 {
					f.focus = 1
					f.envKeyInput.Blur()
					f.envValueInput.Focus()
				} else {
					f.focus = 0
					f.envValueInput.Blur()
					f.envKeyInput.Focus()
				}
			}

		case "shift+tab":
			if f.page == BasicInfoPage {
				f.focus--
				if f.focus < 0 {
					f.focus = len(f.inputs) - 1
				}
				cmds = append(cmds, f.inputs[f.focus].Focus())
			} else if f.page == EnvVarsPage && f.addingEnvVar {
				if f.focus == 1 {
					f.focus = 0
					f.envValueInput.Blur()
					f.envKeyInput.Focus()
				} else {
					f.focus = 1
					f.envKeyInput.Blur()
					f.envValueInput.Focus()
				}
			}

		case "enter":
			switch f.page {
			case BasicInfoPage:
				if f.focus == len(f.inputs)-1 {
					f.page = SourcesPage
					f.sourcesInput.Focus()
				} else {
					f.focus++
					cmds = append(cmds, f.inputs[f.focus].Focus())
				}

			case SourcesPage:
				sourceId := f.sourcesInput.Value()
				if sourceId != "" {
					// Check if already selected
					for _, s := range f.selectedSources {
						if s == sourceId {
							return f, nil
						}
					}
					f.selectedSources = append(f.selectedSources, sourceId)
					f.sourcesInput.SetValue("")
				} else {
					// If input is empty, go to next page
					f.page = SecretsPage
					f.secretsInput.Focus()
				}

			case SecretsPage:
				secretName := f.secretsInput.Value()
				if secretName != "" {
					// Check if already selected
					for _, s := range f.selectedSecrets {
						if s == secretName {
							return f, nil
						}
					}
					f.selectedSecrets = append(f.selectedSecrets, secretName)
					f.secretsInput.SetValue("")
				} else {
					// If input is empty, go to next page
					f.page = EnvVarsPage
					f.addingEnvVar = true
					f.focus = 0
					f.envKeyInput.Focus()
				}

			case EnvVarsPage:
				if f.addingEnvVar {
					if f.focus == 1 {
						// Save environment variable and reset form
						key := f.envKeyInput.Value()
						value := f.envValueInput.Value()
						if key != "" {
							f.envVars[key] = value
							f.envKeyInput.SetValue("")
							f.envValueInput.SetValue("")
							f.focus = 0
							f.envKeyInput.Focus()
						}
					} else {
						// Move to value field
						f.focus = 1
						f.envKeyInput.Blur()
						f.envValueInput.Focus()
					}
				} else {
					// Start adding a new env var
					f.addingEnvVar = true
					f.focus = 0
					f.envKeyInput.Focus()
				}

			case IntegrationsPage:
				integration := f.integrationInput.Value()
				if integration != "" {
					// Check if already added
					for _, i := range f.integrations {
						if i == integration {
							return f, nil
						}
					}
					f.integrations = append(f.integrations, integration)
					f.integrationInput.SetValue("")
				} else {
					// If input is empty, go to confirmation page
					f.page = ConfirmPage
				}

			case ConfirmPage:
				f.done = true
				return f, tea.Quit
			}

		case "n":
			if f.page == EnvVarsPage && !f.addingEnvVar {
				f.addingEnvVar = true
				f.focus = 0
				f.envKeyInput.Focus()
			}

		case "d":
			if f.page == EnvVarsPage && !f.addingEnvVar && len(f.envVars) > 0 {
				// Simple implementation - remove last added env var
				for k := range f.envVars {
					delete(f.envVars, k)
					break
				}
			}

		case "backspace":
			// Handle backspace for deleting items
			if f.page == SourcesPage && msg.Type == tea.KeyBackspace && len(f.selectedSources) > 0 && f.sourcesInput.Value() == "" {
				f.selectedSources = f.selectedSources[:len(f.selectedSources)-1]
			} else if f.page == SecretsPage && msg.Type == tea.KeyBackspace && len(f.selectedSecrets) > 0 && f.secretsInput.Value() == "" {
				f.selectedSecrets = f.selectedSecrets[:len(f.selectedSecrets)-1]
			} else if f.page == IntegrationsPage && msg.Type == tea.KeyBackspace && len(f.integrations) > 0 && f.integrationInput.Value() == "" {
				f.integrations = f.integrations[:len(f.integrations)-1]
			}

		case "right", "l":
			// Next page
			switch f.page {
			case BasicInfoPage:
				f.page = SourcesPage
				f.sourcesInput.Focus()
			case SourcesPage:
				f.page = SecretsPage
				f.secretsInput.Focus()
			case SecretsPage:
				f.page = EnvVarsPage
				f.addingEnvVar = false
			case EnvVarsPage:
				f.page = IntegrationsPage
				f.integrationInput.Focus()
			case IntegrationsPage:
				f.page = ConfirmPage
			}

		case "left", "h":
			// Previous page
			switch f.page {
			case SourcesPage:
				f.page = BasicInfoPage
				f.focus = 0
				f.inputs[f.focus].Focus()
			case SecretsPage:
				f.page = SourcesPage
				f.sourcesInput.Focus()
			case EnvVarsPage:
				f.page = SecretsPage
				f.secretsInput.Focus()
			case IntegrationsPage:
				f.page = EnvVarsPage
				f.addingEnvVar = false
			case ConfirmPage:
				f.page = IntegrationsPage
				f.integrationInput.Focus()
			}
		}

	case tea.WindowSizeMsg:
		f.width = msg.Width
		f.height = msg.Height

		// Update list heights
		f.sourcesList.SetHeight(msg.Height - 10)
		f.sourcesList.SetWidth(msg.Width - 4)
		f.secretsList.SetHeight(msg.Height - 10)
		f.secretsList.SetWidth(msg.Width - 4)
	}

	// Handle input updates
	switch f.page {
	case BasicInfoPage:
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

	case SourcesPage:
		// Update sources input
		var cmd tea.Cmd
		f.sourcesInput, cmd = f.sourcesInput.Update(msg)
		cmds = append(cmds, cmd)

		// Update sources list
		var listCmd tea.Cmd
		f.sourcesList, listCmd = f.sourcesList.Update(msg)
		cmds = append(cmds, listCmd)

	case SecretsPage:
		// Update secrets input
		var cmd tea.Cmd
		f.secretsInput, cmd = f.secretsInput.Update(msg)
		cmds = append(cmds, cmd)

		// Update secrets list
		var listCmd tea.Cmd
		f.secretsList, listCmd = f.secretsList.Update(msg)
		cmds = append(cmds, listCmd)

	case EnvVarsPage:
		if f.addingEnvVar {
			// Update env key/value inputs
			var keyCmd, valueCmd tea.Cmd

			if f.focus == 0 {
				f.envKeyInput.Focus()
				f.envValueInput.Blur()
			} else {
				f.envKeyInput.Blur()
				f.envValueInput.Focus()
			}

			f.envKeyInput, keyCmd = f.envKeyInput.Update(msg)
			f.envValueInput, valueCmd = f.envValueInput.Update(msg)

			cmds = append(cmds, keyCmd, valueCmd)
		}

	case IntegrationsPage:
		var cmd tea.Cmd
		f.integrationInput, cmd = f.integrationInput.Update(msg)
		cmds = append(cmds, cmd)
	}

	return f, tea.Batch(cmds...)
}

func (f *TeammateForm) View() string {
	var s string

	// Header
	s += "🤖 Kubiya Teammate Configuration\n\n"

	// Navigation
	pages := []string{"Basic Info", "Sources", "Secrets", "Env Vars", "Integrations", "Confirm"}
	nav := "Pages: "
	for i, page := range pages {
		if int(f.page) == i {
			nav += fmt.Sprintf("[%s] ", page)
		} else {
			nav += fmt.Sprintf("%s ", page)
		}
	}
	nav += "\n\n"
	s += nav

	// Page content
	switch f.page {
	case BasicInfoPage:
		s += "🧑‍💻 Basic Information\n\n"
		for i := 0; i < len(f.inputs); i++ {
			s += f.inputs[i].View() + "\n"
		}
		s += "\nPress Tab to move between fields\n"
		s += "Press Enter on the last field to continue to Sources\n"

	case SourcesPage:
		s += "📦 Source Selection\n\n"
		s += f.sourcesInput.View() + "\n\n"

		s += "Selected Sources:\n"
		if len(f.selectedSources) == 0 {
			s += "  (none)\n"
		} else {
			for _, src := range f.selectedSources {
				// Look up the name if available
				name := src
				for _, source := range f.sources {
					if source.UUID == src {
						name = fmt.Sprintf("%s (%s)", source.Name, src)
						break
					}
				}
				s += fmt.Sprintf("  • %s\n", name)
			}
		}

		s += "\nAvailable Sources:\n"
		if len(f.availableSources) == 0 {
			s += "  Loading...\n"
		} else {
			for i, item := range f.availableSources {
				if i < 10 { // Only show first 10 for brevity
					s += fmt.Sprintf("  • %s - %s\n", item.Title, item.Id)
				}
			}
			if len(f.availableSources) > 10 {
				s += fmt.Sprintf("  (and %d more...)\n", len(f.availableSources)-10)
			}
		}

		s += "\nEnter source ID and press Enter to add, or press Enter with empty field to continue\n"
		s += "Press Backspace on empty field to remove last source\n"

	case SecretsPage:
		s += "🔒 Secret Selection\n\n"
		s += f.secretsInput.View() + "\n\n"

		s += "Selected Secrets:\n"
		if len(f.selectedSecrets) == 0 {
			s += "  (none)\n"
		} else {
			for _, sec := range f.selectedSecrets {
				s += fmt.Sprintf("  • %s\n", sec)
			}
		}

		s += "\nAvailable Secrets:\n"
		if len(f.availableSecrets) == 0 {
			s += "  Loading...\n"
		} else {
			for i, item := range f.availableSecrets {
				if i < 10 { // Only show first 10 for brevity
					s += fmt.Sprintf("  • %s - %s\n", item.Title, item.Description)
				}
			}
			if len(f.availableSecrets) > 10 {
				s += fmt.Sprintf("  (and %d more...)\n", len(f.availableSecrets)-10)
			}
		}

		s += "\nEnter secret name and press Enter to add, or press Enter with empty field to continue\n"
		s += "Press Backspace on empty field to remove last secret\n"

	case EnvVarsPage:
		s += "⚙️ Environment Variables\n\n"

		if f.addingEnvVar {
			s += "Adding new environment variable:\n"
			s += f.envKeyInput.View() + "\n"
			s += f.envValueInput.View() + "\n\n"
			s += "Press Tab to move between key/value fields\n"
			s += "Press Enter to add this variable\n"
		} else {
			s += "Current Environment Variables:\n"
			if len(f.envVars) == 0 {
				s += "  (none)\n"
			} else {
				for k, v := range f.envVars {
					s += fmt.Sprintf("  • %s=%s\n", k, v)
				}
			}
			s += "\nPress 'n' to add a new variable, 'd' to delete, or Right/Left to navigate pages\n"
		}

	case IntegrationsPage:
		s += "🔌 Integrations\n\n"
		s += f.integrationInput.View() + "\n\n"

		s += "Current Integrations:\n"
		if len(f.integrations) == 0 {
			s += "  (none)\n"
		} else {
			for _, integration := range f.integrations {
				s += fmt.Sprintf("  • %s\n", integration)
			}
		}

		s += "\nEnter integration name and press Enter to add, or press Enter with empty field to continue\n"
		s += "Press Backspace on empty field to remove last integration\n"
		s += "Common integrations: github, aws, kubernetes, slack, jira, etc.\n"

	case ConfirmPage:
		s += "✅ Confirmation\n\n"

		s += "Teammate Configuration Summary:\n\n"
		s += fmt.Sprintf("Name: %s\n", f.inputs[0].Value())
		s += fmt.Sprintf("Description: %s\n", f.inputs[1].Value())
		s += fmt.Sprintf("LLM Model: %s\n", f.inputs[2].Value())
		s += fmt.Sprintf("Type: %s\n\n", f.inputs[3].Value())

		s += fmt.Sprintf("Sources: %d\n", len(f.selectedSources))
		s += fmt.Sprintf("Secrets: %d\n", len(f.selectedSecrets))
		s += fmt.Sprintf("Environment Variables: %d\n", len(f.envVars))
		s += fmt.Sprintf("Integrations: %d\n\n", len(f.integrations))

		s += "Press Enter to confirm and create the teammate, or Left to go back\n"
	}

	// Footer
	s += "\nNavigate: ←/→ or h/l to change pages | Ctrl+C to quit\n"

	return s
}
