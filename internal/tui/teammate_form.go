package tui

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"

	"github.com/charmbracelet/bubbles/list"
	"github.com/charmbracelet/bubbles/textinput"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
	"github.com/kubiyabot/cli/internal/config"
	"github.com/kubiyabot/cli/internal/kubiya"
	"gopkg.in/yaml.v3"
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

// SourceInputType represents different ways to input a source
type SourceInputType int

const (
	ExistingSourceType SourceInputType = iota
	URLSourceType
	LocalSourceType
	InlineSourceType
)

// TeammateForm represents the TUI form
type TeammateForm struct {
	cfg                  *config.Config
	client               *kubiya.Client
	inputs               []textinput.Model
	sourcesInput         textinput.Model
	sourceURLInput       textinput.Model
	sourceLocalPathInput textinput.Model
	sourceInlineInput    textinput.Model
	secretsInput         textinput.Model
	envKeyInput          textinput.Model
	envValueInput        textinput.Model
	integrationInput     textinput.Model

	// Data
	sources          []kubiya.Source
	secrets          []kubiya.Secret
	availableSources []Item
	availableSecrets []Item

	// Lists
	sourcesList list.Model
	secretsList list.Model

	// State
	focus                 int
	done                  bool
	err                   error
	page                  FormPage
	selectedSources       []string
	selectedSecrets       []string
	envVars               map[string]string
	integrations          []string
	addingEnvVar          bool
	currentEnvKey         string
	sourceInputType       SourceInputType
	inlineSourceContent   string
	sourceURLValue        string
	sourceLocalPathValue  string
	inlineSourceName      string
	inlineSourceInputMode bool

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

	// Initialize source URL input
	sourceURLInput := textinput.New()
	sourceURLInput.Placeholder = "https://github.com/org/repo"
	sourceURLInput.CharLimit = 100
	sourceURLInput.Width = 50
	sourceURLInput.Prompt = "Source URL: "

	// Initialize source local path input
	sourceLocalPathInput := textinput.New()
	sourceLocalPathInput.Placeholder = "/path/to/local/directory or file"
	sourceLocalPathInput.CharLimit = 100
	sourceLocalPathInput.Width = 50
	sourceLocalPathInput.Prompt = "Local Path: "

	// Initialize source inline input (multi-line support)
	sourceInlineInput := textinput.New()
	sourceInlineInput.Placeholder = "Enter a name for this inline source"
	sourceInlineInput.CharLimit = 50
	sourceInlineInput.Width = 40
	sourceInlineInput.Prompt = "Inline Source Name: "

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
		cfg:                  cfg,
		client:               kubiya.NewClient(cfg),
		inputs:               inputs,
		sourcesInput:         sourcesInput,
		sourceURLInput:       sourceURLInput,
		sourceLocalPathInput: sourceLocalPathInput,
		sourceInlineInput:    sourceInlineInput,
		secretsInput:         secretsInput,
		envKeyInput:          envKeyInput,
		envValueInput:        envValueInput,
		integrationInput:     integrationInput,
		sourcesList:          sourcesList,
		secretsList:          secretsList,
		focus:                0,
		page:                 BasicInfoPage,
		selectedSources:      []string{},
		selectedSecrets:      []string{},
		envVars:              make(map[string]string),
		integrations:         []string{},
		addingEnvVar:         false,
		sourceInputType:      ExistingSourceType,
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

	// Reset source input states
	f.sourceInputType = ExistingSourceType
	f.inlineSourceInputMode = false
	f.inlineSourceContent = ""
	f.inlineSourceName = ""
	f.sourceURLValue = ""
	f.sourceLocalPathValue = ""
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

	// If there's any pending inline source, try to create it
	if form.inlineSourceInputMode && form.inlineSourceName != "" && form.inlineSourceContent != "" {
		ctx := context.Background()
		source, err := form.createInlineSource(ctx, form.inlineSourceName, form.inlineSourceContent)
		if err != nil {
			return nil, fmt.Errorf("failed to create inline source: %w", err)
		}
		if source != nil {
			form.selectedSources = append(form.selectedSources, source.UUID)
		}
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
		// Make sure we have a name, fallback to ID if empty
		srcName := source.Name
		if srcName == "" {
			srcName = "Unnamed source"
		}

		// Create a friendlier description
		srcType := "unknown"
		if source.Type != "" {
			srcType = source.Type
		}

		description := fmt.Sprintf("UUID: %s, Type: %s", source.UUID, srcType)
		if source.URL != "" {
			description += fmt.Sprintf(", URL: %s", source.URL)
		}

		items = append(items, Item{
			TitleText:       srcName,
			DescriptionText: description,
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
			// If in inline source input mode, exit that mode instead of quitting
			if f.page == SourcesPage && f.inlineSourceInputMode {
				f.inlineSourceInputMode = false
				return f, nil
			}
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
			} else if f.page == SourcesPage && !f.inlineSourceInputMode {
				// Cycle through source input types
				f.sourceInputType = (f.sourceInputType + 1) % 4
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
			} else if f.page == SourcesPage && !f.inlineSourceInputMode {
				// Cycle backward through source input types
				f.sourceInputType--
				if f.sourceInputType < 0 {
					f.sourceInputType = 3
				}
			}

		case "enter":
			// If we're in inline source input mode, treat Enter differently
			if f.page == SourcesPage && f.inlineSourceInputMode {
				// Add a newline to the content
				f.inlineSourceContent += "\n"
				return f, nil
			}

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
				switch f.sourceInputType {
				case ExistingSourceType:
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

				case URLSourceType:
					sourceURL := f.sourceURLInput.Value()
					if sourceURL != "" {
						// Create a source from URL
						ctx := context.Background()
						source, err := f.client.LoadSource(ctx, sourceURL)
						if err != nil {
							// Handle error - store it to display later
							f.err = fmt.Errorf("failed to load source from URL: %w", err)
						} else if source != nil {
							// Add the source UUID to selected sources
							f.selectedSources = append(f.selectedSources, source.UUID)
							f.sourceURLInput.SetValue("")
						}
					} else {
						// If input is empty, go to next page
						f.page = SecretsPage
						f.secretsInput.Focus()
					}

				case LocalSourceType:
					localPath := f.sourceLocalPathInput.Value()
					if localPath != "" {
						// Create a source from local path
						ctx := context.Background()
						source, err := f.client.LoadSource(ctx, localPath)
						if err != nil {
							// Handle error - store it to display later
							f.err = fmt.Errorf("failed to load source from local path: %w", err)
						} else if source != nil {
							// Add the source UUID to selected sources
							f.selectedSources = append(f.selectedSources, source.UUID)
							f.sourceLocalPathInput.SetValue("")
						}
					} else {
						// If input is empty, go to next page
						f.page = SecretsPage
						f.secretsInput.Focus()
					}

				case InlineSourceType:
					inlineName := f.sourceInlineInput.Value()
					if inlineName != "" {
						if f.inlineSourceInputMode {
							// We're already in inline input mode and have content, submit it
							ctx := context.Background()
							// Create an inline source with the content
							source, err := f.createInlineSource(ctx, inlineName, f.inlineSourceContent)
							if err != nil {
								f.err = fmt.Errorf("failed to create inline source: %w", err)
							} else if source != nil {
								// Add the source UUID to selected sources
								f.selectedSources = append(f.selectedSources, source.UUID)
							}
							// Reset the input mode and content
							f.inlineSourceInputMode = false
							f.inlineSourceContent = ""
							f.sourceInlineInput.SetValue("")
						} else {
							// Enter inline input mode
							f.inlineSourceName = inlineName
							f.inlineSourceInputMode = true
							// Initialize with a default example template
							f.inlineSourceContent = getDefaultInlineSourceContent(inlineName)
						}
					} else {
						// If input is empty, go to next page
						f.page = SecretsPage
						f.secretsInput.Focus()
					}
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

		case "ctrl+s":
			// In inline source mode, ctrl+s saves the content
			if f.page == SourcesPage && f.inlineSourceInputMode {
				ctx := context.Background()
				source, err := f.createInlineSource(ctx, f.inlineSourceName, f.inlineSourceContent)
				if err != nil {
					f.err = fmt.Errorf("failed to create inline source: %w", err)
				} else if source != nil {
					// Add the source UUID to selected sources
					f.selectedSources = append(f.selectedSources, source.UUID)
				}
				// Exit inline mode
				f.inlineSourceInputMode = false
				f.inlineSourceContent = ""
				f.sourceInlineInput.SetValue("")
			}

		case "backspace":
			// Handle backspace for deleting items
			if f.page == SourcesPage && !f.inlineSourceInputMode && len(f.selectedSources) > 0 {
				// Only delete if the respective input field is empty
				switch f.sourceInputType {
				case ExistingSourceType:
					if f.sourcesInput.Value() == "" {
						f.selectedSources = f.selectedSources[:len(f.selectedSources)-1]
					}
				case URLSourceType:
					if f.sourceURLInput.Value() == "" {
						f.selectedSources = f.selectedSources[:len(f.selectedSources)-1]
					}
				case LocalSourceType:
					if f.sourceLocalPathInput.Value() == "" {
						f.selectedSources = f.selectedSources[:len(f.selectedSources)-1]
					}
				case InlineSourceType:
					if f.sourceInlineInput.Value() == "" {
						f.selectedSources = f.selectedSources[:len(f.selectedSources)-1]
					}
				}
			} else if f.page == SourcesPage && f.inlineSourceInputMode && len(f.inlineSourceContent) > 0 {
				// Handle backspace in inline edit mode
				f.inlineSourceContent = f.inlineSourceContent[:len(f.inlineSourceContent)-1]
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
				f.sourceInputType = ExistingSourceType
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
				switch f.sourceInputType {
				case ExistingSourceType:
					f.sourcesInput.Focus()
				case URLSourceType:
					f.sourceURLInput.Focus()
				case LocalSourceType:
					f.sourceLocalPathInput.Focus()
				case InlineSourceType:
					f.sourceInlineInput.Focus()
				}
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
		if f.inlineSourceInputMode {
			// In inline source mode, we capture regular input to build the content
			if keyMsg, ok := msg.(tea.KeyMsg); ok {
				// Capture printable characters
				if keyMsg.Type == tea.KeyRunes {
					f.inlineSourceContent += string(keyMsg.Runes)
				}
			}
		} else {
			// Update appropriate source input based on type
			var cmd tea.Cmd
			switch f.sourceInputType {
			case ExistingSourceType:
				f.sourcesInput.Focus()
				f.sourceURLInput.Blur()
				f.sourceLocalPathInput.Blur()
				f.sourceInlineInput.Blur()
				f.sourcesInput, cmd = f.sourcesInput.Update(msg)
			case URLSourceType:
				f.sourcesInput.Blur()
				f.sourceURLInput.Focus()
				f.sourceLocalPathInput.Blur()
				f.sourceInlineInput.Blur()
				f.sourceURLInput, cmd = f.sourceURLInput.Update(msg)
			case LocalSourceType:
				f.sourcesInput.Blur()
				f.sourceURLInput.Blur()
				f.sourceLocalPathInput.Focus()
				f.sourceInlineInput.Blur()
				f.sourceLocalPathInput, cmd = f.sourceLocalPathInput.Update(msg)
			case InlineSourceType:
				f.sourcesInput.Blur()
				f.sourceURLInput.Blur()
				f.sourceLocalPathInput.Blur()
				f.sourceInlineInput.Focus()
				f.sourceInlineInput, cmd = f.sourceInlineInput.Update(msg)
			}
			cmds = append(cmds, cmd)
		}

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

// createInlineSource creates a source from inline content
func (f *TeammateForm) createInlineSource(ctx context.Context, name, content string) (*kubiya.Source, error) {
	if content == "" {
		return nil, fmt.Errorf("empty source content")
	}

	// Attempt to parse the content as YAML or JSON to extract tools
	tools, err := parseInlineSourceContent(content)
	if err != nil {
		return nil, fmt.Errorf("failed to parse content: %w", err)
	}

	if len(tools) == 0 {
		return nil, fmt.Errorf("no tools found in the content")
	}

	// Log the tools we're creating
	toolNames := make([]string, len(tools))
	for i, tool := range tools {
		toolNames[i] = tool.Name
	}

	// Create the source with the tools
	source, err := f.client.CreateSource(ctx, "",
		kubiya.WithName(name),
		kubiya.WithInlineTools(tools))

	if err != nil {
		return nil, fmt.Errorf("failed to create source: %w", err)
	}

	return source, nil
}

// parseInlineSourceContent attempts to parse the content as YAML or JSON
func parseInlineSourceContent(content string) ([]kubiya.Tool, error) {
	// Try to parse the content as YAML first
	var yamlData map[string]interface{}
	yamlErr := yaml.Unmarshal([]byte(content), &yamlData)
	if yamlErr == nil {
		// Successfully parsed as YAML map
		var tools []kubiya.Tool

		// Check if YAML has a tools array
		if toolsArray, ok := yamlData["tools"].([]interface{}); ok {
			// Parse tools array
			for _, item := range toolsArray {
				if toolMap, ok := item.(map[string]interface{}); ok {
					tool := mapToTool(toolMap)
					tools = append(tools, tool)
				}
			}
			return tools, nil
		}

		// Check if it's a single tool definition
		if _, ok := yamlData["name"].(string); ok {
			tool := mapToTool(yamlData)
			return []kubiya.Tool{tool}, nil
		}

		// Treat each top-level key as a tool name
		for name, value := range yamlData {
			if valueMap, ok := value.(map[string]interface{}); ok {
				// Add the name if not already in the map
				if _, hasName := valueMap["name"]; !hasName {
					valueMap["name"] = name
				}
				tool := mapToTool(valueMap)
				tools = append(tools, tool)
			} else {
				// Simple tools with just a command
				tool := kubiya.Tool{
					Name:        name,
					Description: fmt.Sprintf("Simple tool: %s", name),
					Content:     fmt.Sprintf("%v", value),
				}
				tools = append(tools, tool)
			}
		}

		if len(tools) > 0 {
			return tools, nil
		}
	}

	// Try to parse as YAML array
	var yamlArray []interface{}
	yamlArrayErr := yaml.Unmarshal([]byte(content), &yamlArray)
	if yamlArrayErr == nil && len(yamlArray) > 0 {
		var tools []kubiya.Tool
		for i, item := range yamlArray {
			if itemMap, ok := item.(map[string]interface{}); ok {
				// If no name, generate one
				if _, hasName := itemMap["name"]; !hasName {
					itemMap["name"] = fmt.Sprintf("tool-%d", i+1)
				}
				tool := mapToTool(itemMap)
				tools = append(tools, tool)
			}
		}
		if len(tools) > 0 {
			return tools, nil
		}
	}

	// If all parsing failed, create a single tool with the raw content
	tool := kubiya.Tool{
		Name:        "inline-tool",
		Description: "Auto-generated from inline content",
		Content:     content,
	}

	return []kubiya.Tool{tool}, nil
}

// mapToTool converts a map to a kubiya.Tool
func mapToTool(m map[string]interface{}) kubiya.Tool {
	tool := kubiya.Tool{
		Name:        getStringValue(m, "name", "unnamed-tool"),
		Description: getStringValue(m, "description", ""),
		Type:        getStringValue(m, "type", "function"),
		Content:     getStringValue(m, "content", ""),
	}

	// Handle arguments
	if args, ok := m["args"].([]interface{}); ok {
		for _, arg := range args {
			if argMap, ok := arg.(map[string]interface{}); ok {
				toolArg := kubiya.ToolArg{
					Name:        getStringValue(argMap, "name", ""),
					Description: getStringValue(argMap, "description", ""),
				}

				// Check if required
				if req, ok := argMap["required"].(bool); ok {
					toolArg.Required = req
				}

				tool.Args = append(tool.Args, toolArg)
			}
		}
	}

	// Handle environment variables
	if env, ok := m["env"].([]interface{}); ok {
		for _, e := range env {
			if envString, ok := e.(string); ok {
				tool.Env = append(tool.Env, envString)
			}
		}
	}

	// Handle secrets
	if secrets, ok := m["secrets"].([]interface{}); ok {
		for _, s := range secrets {
			if secretString, ok := s.(string); ok {
				tool.Secrets = append(tool.Secrets, secretString)
			}
		}
	}

	// Handle Docker image - check if Tool struct has this field
	if image, ok := m["image"].(string); ok && image != "" {
		// Store the image in a metadata map
		metadata := make(map[string]interface{})
		metadata["image"] = image
		tool.Metadata = metadata
	}

	// Handle long running flag
	if longRunning, ok := m["long_running"].(bool); ok {
		tool.LongRunning = longRunning
	}

	// Handle with_files
	if withFiles, ok := m["with_files"].(map[string]interface{}); ok {
		// Convert to JSON and back to handle complex structures
		bytes, _ := json.Marshal(withFiles)
		json.Unmarshal(bytes, &tool.WithFiles)
	}

	// Handle with_volumes
	if withVolumes, ok := m["with_volumes"].(map[string]interface{}); ok {
		// Convert to JSON and back to handle complex structures
		bytes, _ := json.Marshal(withVolumes)
		json.Unmarshal(bytes, &tool.WithVolumes)
	}

	return tool
}

// getStringValue gets a string value from a map with a fallback
func getStringValue(m map[string]interface{}, key, fallback string) string {
	if val, ok := m[key].(string); ok {
		return val
	}
	return fallback
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

	// Error display
	if f.err != nil {
		s += fmt.Sprintf("Error: %s\n\n", f.err.Error())
		// Reset error after displaying
		f.err = nil
	}

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

		if f.inlineSourceInputMode {
			s += fmt.Sprintf("Editing Inline Source: %s\n\n", f.inlineSourceName)
			s += "Enter your source code or YAML definition below:\n"
			s += "----------------------\n"
			s += f.inlineSourceContent
			s += "\n----------------------\n"
			s += "\nPress Ctrl+S to save or Esc to cancel\n"
			s += "Press Enter for new line\n"
			return s
		}

		// Source type selection
		s += "Source Type: "
		sourceTypes := []string{"Existing", "URL", "Local Path", "Inline"}
		for i, srcType := range sourceTypes {
			if int(f.sourceInputType) == i {
				s += fmt.Sprintf("[%s] ", srcType)
			} else {
				s += fmt.Sprintf("%s ", srcType)
			}
		}
		s += "\nPress Tab to change source type\n\n"

		// Display the appropriate input field based on source type
		switch f.sourceInputType {
		case ExistingSourceType:
			s += f.sourcesInput.View() + "\n\n"

			s += "Available Sources:\n"
			if len(f.availableSources) == 0 {
				s += "  Loading...\n"
			} else {
				for i, item := range f.availableSources {
					if i < 10 { // Only show first 10 for brevity
						s += fmt.Sprintf("  • %s - %s\n", item.Title(), item.Id)
					}
				}
				if len(f.availableSources) > 10 {
					s += fmt.Sprintf("  (and %d more...)\n", len(f.availableSources)-10)
				}
			}

		case URLSourceType:
			s += f.sourceURLInput.View() + "\n\n"
			s += "Enter a Git repository URL or GitHub URL.\n"
			s += "Examples:\n"
			s += "  • https://github.com/org/repo\n"
			s += "  • https://github.com/org/repo/tree/branch\n"
			s += "  • https://github.com/org/repo/tree/branch/path\n"

		case LocalSourceType:
			s += f.sourceLocalPathInput.View() + "\n\n"
			s += "Enter a local directory or file path.\n"
			s += "Examples:\n"
			s += "  • /path/to/directory\n"
			s += "  • ./relative/path\n"
			s += "  • /path/to/file.py\n"
			s += "  • /path/to/manifest.yaml\n"

		case InlineSourceType:
			s += f.sourceInlineInput.View() + "\n\n"
			s += "Enter a name for your inline source, then press Enter to input the code.\n"
			s += "You can define tools in YAML, JSON, or Python format.\n\n"
			s += "Example YAML format (enter after naming your source):\n"
			s += "```yaml\n"
			s += "# Single tool definition\n"
			s += "name: my-docker-tool\n"
			s += "description: Tool that runs in a Docker container\n"
			s += "image: alpine:latest\n"
			s += "type: function\n"
			s += "content: echo 'Hello from Docker'\n"
			s += "env:\n"
			s += "  - API_KEY=secret\n"
			s += "  - DEBUG=true\n"
			s += "args:\n"
			s += "  - name: input\n"
			s += "    description: Input parameter\n"
			s += "    required: true\n"
			s += "long_running: false\n"
			s += "```\n"
			s += "Or define multiple tools using a tools array:\n"
			s += "```yaml\n"
			s += "tools:\n"
			s += "  - name: tool1\n"
			s += "    description: First tool\n"
			s += "    content: echo 'tool1'\n"
			s += "  - name: tool2\n"
			s += "    description: Second tool\n"
			s += "    content: echo 'tool2'\n"
			s += "```\n"
		}

		s += "\nSelected Sources:\n"
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

		s += "\nEnter source details and press Enter to add, or press Enter with empty field to continue\n"
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
					s += fmt.Sprintf("  • %s - %s\n", item.Title(), item.Description())
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

		// Show more detailed source information
		s += fmt.Sprintf("Sources: %d\n", len(f.selectedSources))
		if len(f.selectedSources) > 0 {
			s += "  Selected Sources:\n"
			for _, sourceID := range f.selectedSources {
				// Try to get source information
				sourceName := sourceID
				sourceType := ""
				for _, source := range f.sources {
					if source.UUID == sourceID {
						if source.Name != "" {
							sourceName = source.Name
						}
						sourceType = source.Type
						break
					}
				}
				if sourceType != "" {
					s += fmt.Sprintf("  • %s (%s, Type: %s)\n", sourceName, sourceID, sourceType)
				} else {
					s += fmt.Sprintf("  • %s (%s)\n", sourceName, sourceID)
				}
			}
			s += "\n"
		}

		s += fmt.Sprintf("Secrets: %d\n", len(f.selectedSecrets))
		if len(f.selectedSecrets) > 0 {
			s += "  Selected Secrets:\n"
			for _, secret := range f.selectedSecrets {
				s += fmt.Sprintf("  • %s\n", secret)
			}
			s += "\n"
		}

		s += fmt.Sprintf("Environment Variables: %d\n", len(f.envVars))
		if len(f.envVars) > 0 {
			s += "  Environment Variables:\n"
			for k, v := range f.envVars {
				// Mask sensitive data in display
				if strings.Contains(strings.ToLower(k), "password") ||
					strings.Contains(strings.ToLower(k), "secret") ||
					strings.Contains(strings.ToLower(k), "token") ||
					strings.Contains(strings.ToLower(k), "key") {
					s += fmt.Sprintf("  • %s=********\n", k)
				} else {
					s += fmt.Sprintf("  • %s=%s\n", k, v)
				}
			}
			s += "\n"
		}

		s += fmt.Sprintf("Integrations: %d\n", len(f.integrations))
		if len(f.integrations) > 0 {
			s += "  Integrations:\n"
			for _, integration := range f.integrations {
				s += fmt.Sprintf("  • %s\n", integration)
			}
			s += "\n"
		}

		s += "Press Enter to confirm and create the teammate, or Left to go back\n"
	}

	// Footer
	s += "\nNavigate: ←/→ or h/l to change pages | Ctrl+C to quit\n"

	return s
}

// getDefaultInlineSourceContent returns a simple example YAML for the user to edit
func getDefaultInlineSourceContent(name string) string {
	template := `# %s
# Delete or edit this example to create your own tool

name: %s-tool
description: Auto-generated tool for %s
type: function
content: echo "Hello from %s"

# Uncomment if you need Docker support:
# image: alpine:latest

# Uncomment to add environment variables:
# env:
#   - API_KEY=secret
#   - DEBUG=true

# Uncomment to add arguments:
# args:
#   - name: input
#     description: Input parameter
#     required: true
`
	return fmt.Sprintf(template, name, name, name, name)
}
