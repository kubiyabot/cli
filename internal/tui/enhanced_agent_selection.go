package tui

import (
	"context"
	"fmt"
	"sort"
	"strings"
	"time"

	"github.com/charmbracelet/bubbles/help"
	"github.com/charmbracelet/bubbles/key"
	"github.com/charmbracelet/bubbles/list"
	"github.com/charmbracelet/bubbles/spinner"
	"github.com/charmbracelet/bubbles/textinput"
	"github.com/charmbracelet/bubbles/viewport"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
	"github.com/kubiyabot/cli/internal/config"
	"github.com/kubiyabot/cli/internal/kubiya"
)

// Enhanced Agent Selection UI with detailed previews and validation
type EnhancedAgentSelection struct {
	cfg    *config.Config
	client *kubiya.Client

	// UI components
	list        list.Model
	viewport    viewport.Model
	textinput   textinput.Model
	spinner     spinner.Model
	help        help.Model
	keyMap      agentSelectionKeyMap

	// State management
	agents          []kubiya.Agent
	filteredAgents  []kubiya.Agent
	selectedAgent   *kubiya.Agent
	detailedAgent   *kubiya.Agent
	width           int
	height          int
	ready           bool
	err             error
	loading         bool
	showDetails     bool
	showSearch      bool
	searchMode      bool
	detailsLoading  bool

	// Agent validation and status
	agentStatus     map[string]AgentStatus
	lastRefresh     time.Time
	refreshInterval time.Duration

	// Context for cancellation
	ctx    context.Context
	cancel context.CancelFunc

	// Callbacks
	onAgentSelected func(agent kubiya.Agent)
	onBack          func()
	onError         func(error)
}

// Agent status information
type AgentStatus struct {
	Available    bool      `json:"available"`
	ResponseTime duration  `json:"response_time"`
	ToolsCount   int       `json:"tools_count"`
	LastChecked  time.Time `json:"last_checked"`
	Status       string    `json:"status"` // "online", "offline", "busy", "unknown"
	ErrorMessage string    `json:"error_message,omitempty"`
}

// Custom duration type for JSON marshaling
type duration time.Duration

func (d duration) MarshalJSON() ([]byte, error) {
	return []byte(fmt.Sprintf(`"%s"`, time.Duration(d).String())), nil
}

// Key map for agent selection
type agentSelectionKeyMap struct {
	Up           key.Binding
	Down         key.Binding
	Select       key.Binding
	Back         key.Binding
	Quit         key.Binding
	ShowDetails  key.Binding
	Search       key.Binding
	Refresh      key.Binding
	Help         key.Binding
	ClearSearch  key.Binding
	NextPage     key.Binding
	PrevPage     key.Binding
	FirstItem    key.Binding
	LastItem     key.Binding
}

// Enhanced agent list item with rich display
type enhancedAgentItem struct {
	agent  kubiya.Agent
	status AgentStatus
}

func (item enhancedAgentItem) Title() string {
	// Status indicator
	var statusEmoji string
	switch item.status.Status {
	case "online":
		statusEmoji = "🟢"
	case "busy":
		statusEmoji = "🟡"
	case "offline":
		statusEmoji = "🔴"
	default:
		statusEmoji = "⚪"
	}

	// Type indicator
	typeEmoji := getAgentTypeEmoji(item.agent.InstructionType)
	
	// Build title with status and type
	title := fmt.Sprintf("%s %s %s", statusEmoji, typeEmoji, item.agent.Name)
	
	// Add tool count if available
	if item.status.ToolsCount > 0 {
		title += fmt.Sprintf(" (%d tools)", item.status.ToolsCount)
	}
	
	return title
}

func (item enhancedAgentItem) Description() string {
	desc := item.agent.Description
	if desc == "" {
		desc = item.agent.Desc // fallback to legacy field
	}
	
	// Add additional info
	var info []string
	
	if item.agent.LLMModel != "" {
		info = append(info, fmt.Sprintf("Model: %s", item.agent.LLMModel))
	}
	
	if len(item.agent.Integrations) > 0 {
		info = append(info, fmt.Sprintf("Integrations: %d", len(item.agent.Integrations)))
	}
	
	if len(item.agent.Runners) > 0 {
		info = append(info, fmt.Sprintf("Runners: %s", strings.Join(item.agent.Runners, ", ")))
	}
	
	if item.status.ResponseTime > 0 {
		info = append(info, fmt.Sprintf("Response: %s", time.Duration(item.status.ResponseTime).String()))
	}
	
	if len(info) > 0 {
		desc += "\n" + strings.Join(info, " • ")
	}
	
	return desc
}

func (item enhancedAgentItem) FilterValue() string {
	searchable := fmt.Sprintf("%s %s %s %s", 
		item.agent.Name, 
		item.agent.Description, 
		item.agent.Desc,
		strings.Join(item.agent.Tools, " "))
	
	// Add integrations and runners to search
	searchable += " " + strings.Join(item.agent.Integrations, " ")
	searchable += " " + strings.Join(item.agent.Runners, " ")
	searchable += " " + strings.Join(item.agent.Tags, " ")
	
	return searchable
}

// Get emoji for agent type
func getAgentTypeEmoji(instructionType string) string {
	switch strings.ToLower(instructionType) {
	case "tools":
		return "🛠️"
	case "knowledge":
		return "📚"
	case "workflow":
		return "⚙️"
	case "assistant":
		return "🤖"
	case "chat":
		return "💬"
	default:
		return "🔧"
	}
}

// Initialize enhanced agent selection
func NewEnhancedAgentSelection(cfg *config.Config) *EnhancedAgentSelection {
	// Initialize components
	s := spinner.New()
	s.Spinner = spinner.Dot
	s.Style = lipgloss.NewStyle().Foreground(lipgloss.Color("69"))

	delegate := list.NewDefaultDelegate()
	delegate.SetHeight(3) // Increase height for richer display
	delegate.Styles.SelectedTitle = lipgloss.NewStyle().
		Foreground(lipgloss.Color("170")).
		Bold(true).
		Border(lipgloss.RoundedBorder()).
		BorderForeground(lipgloss.Color("170")).
		Padding(0, 1)
	delegate.Styles.SelectedDesc = lipgloss.NewStyle().
		Foreground(lipgloss.Color("245")).
		Padding(0, 1)
	delegate.Styles.NormalTitle = lipgloss.NewStyle().
		Foreground(lipgloss.Color("252"))
	delegate.Styles.NormalDesc = lipgloss.NewStyle().
		Foreground(lipgloss.Color("245"))

	agentList := list.New([]list.Item{}, delegate, 0, 0)
	agentList.Title = "🤖 Select Kubiya Agent"
	agentList.SetShowStatusBar(true)
	agentList.SetFilteringEnabled(true)
	agentList.SetShowPagination(true)
	agentList.Styles.Title = lipgloss.NewStyle().
		Bold(true).
		Foreground(lipgloss.Color("170")).
		Border(lipgloss.RoundedBorder()).
		BorderForeground(lipgloss.Color("170")).
		Padding(1, 2)
	agentList.Styles.StatusBar = lipgloss.NewStyle().
		Foreground(lipgloss.Color("245"))

	// Text input for search
	ti := textinput.New()
	ti.Placeholder = "Search agents..."
	ti.Focus()
	ti.CharLimit = 50

	// Viewport for detailed view
	vp := viewport.New(0, 0)

	// Key map
	keyMap := agentSelectionKeyMap{
		Up: key.NewBinding(
			key.WithKeys("k", "up"),
			key.WithHelp("↑/k", "up"),
		),
		Down: key.NewBinding(
			key.WithKeys("j", "down"),
			key.WithHelp("↓/j", "down"),
		),
		Select: key.NewBinding(
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
		ShowDetails: key.NewBinding(
			key.WithKeys("d", "tab"),
			key.WithHelp("d/tab", "details"),
		),
		Search: key.NewBinding(
			key.WithKeys("/"),
			key.WithHelp("/", "search"),
		),
		Refresh: key.NewBinding(
			key.WithKeys("r"),
			key.WithHelp("r", "refresh"),
		),
		Help: key.NewBinding(
			key.WithKeys("?"),
			key.WithHelp("?", "help"),
		),
		ClearSearch: key.NewBinding(
			key.WithKeys("ctrl+l"),
			key.WithHelp("ctrl+l", "clear search"),
		),
		NextPage: key.NewBinding(
			key.WithKeys("n"),
			key.WithHelp("n", "next page"),
		),
		PrevPage: key.NewBinding(
			key.WithKeys("p"),
			key.WithHelp("p", "prev page"),
		),
		FirstItem: key.NewBinding(
			key.WithKeys("g"),
			key.WithHelp("g", "first"),
		),
		LastItem: key.NewBinding(
			key.WithKeys("G"),
			key.WithHelp("G", "last"),
		),
	}

	ctx, cancel := context.WithCancel(context.Background())

	return &EnhancedAgentSelection{
		cfg:             cfg,
		client:          kubiya.NewClient(cfg),
		list:            agentList,
		viewport:        vp,
		textinput:       ti,
		spinner:         s,
		help:            help.New(),
		keyMap:          keyMap,
		agentStatus:     make(map[string]AgentStatus),
		refreshInterval: time.Minute * 5,
		ctx:             ctx,
		cancel:          cancel,
	}
}

// Initialize the UI
func (ui *EnhancedAgentSelection) Init() tea.Cmd {
	return tea.Batch(
		ui.spinner.Tick,
		ui.loadAgents(),
		textinput.Blink,
	)
}

// Update handles all UI updates
func (ui *EnhancedAgentSelection) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	var cmds []tea.Cmd
	var cmd tea.Cmd

	switch msg := msg.(type) {
	case tea.WindowSizeMsg:
		ui.width = msg.Width
		ui.height = msg.Height
		ui.updateLayout()
		return ui, nil

	case []kubiya.Agent:
		ui.agents = msg
		ui.filteredAgents = msg
		ui.updateAgentList()
		ui.ready = true
		ui.loading = false
		return ui, ui.validateAgents()

	case AgentStatus:
		// Update specific agent status
		return ui, nil

	case error:
		ui.err = msg
		ui.loading = false
		ui.detailsLoading = false
		if ui.onError != nil {
			ui.onError(msg)
		}
		return ui, nil

	case tea.KeyMsg:
		return ui.handleKeyPress(msg)
	}

	// Update components based on current mode
	if ui.searchMode {
		ui.textinput, cmd = ui.textinput.Update(msg)
		cmds = append(cmds, cmd)
		
		// Update search results
		if ui.textinput.Value() != "" {
			ui.filterAgents(ui.textinput.Value())
		} else {
			ui.filteredAgents = ui.agents
			ui.updateAgentList()
		}
	} else {
		ui.list, cmd = ui.list.Update(msg)
		cmds = append(cmds, cmd)
		
		// Update selected agent details
		if selectedItem, ok := ui.list.SelectedItem().(enhancedAgentItem); ok {
			if ui.selectedAgent == nil || ui.selectedAgent.UUID != selectedItem.agent.UUID {
				ui.selectedAgent = &selectedItem.agent
				if ui.showDetails {
					cmds = append(cmds, ui.loadAgentDetails(selectedItem.agent))
				}
			}
		}
	}

	// Update spinner if loading
	if ui.loading || ui.detailsLoading {
		ui.spinner, cmd = ui.spinner.Update(msg)
		cmds = append(cmds, cmd)
	}

	// Update viewport if showing details
	if ui.showDetails {
		ui.viewport, cmd = ui.viewport.Update(msg)
		cmds = append(cmds, cmd)
	}

	return ui, tea.Batch(cmds...)
}

// Render the UI
func (ui *EnhancedAgentSelection) View() string {
	if ui.err != nil {
		return ui.renderError()
	}

	if !ui.ready {
		return ui.renderLoading()
	}

	if ui.showDetails {
		return ui.renderWithDetails()
	}

	return ui.renderAgentList()
}

// Handle key presses
func (ui *EnhancedAgentSelection) handleKeyPress(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	switch {
	case key.Matches(msg, ui.keyMap.Quit):
		ui.cancel()
		return ui, tea.Quit

	case key.Matches(msg, ui.keyMap.Back):
		if ui.searchMode {
			ui.searchMode = false
			ui.textinput.SetValue("")
			ui.filteredAgents = ui.agents
			ui.updateAgentList()
			return ui, nil
		}
		if ui.showDetails {
			ui.showDetails = false
			return ui, nil
		}
		if ui.onBack != nil {
			ui.onBack()
		}
		return ui, nil

	case key.Matches(msg, ui.keyMap.Select):
		if ui.searchMode {
			ui.searchMode = false
			return ui, nil
		}
		if selectedItem, ok := ui.list.SelectedItem().(enhancedAgentItem); ok {
			return ui, ui.selectAgent(selectedItem.agent)
		}
		return ui, nil

	case key.Matches(msg, ui.keyMap.ShowDetails):
		if ui.selectedAgent != nil {
			ui.showDetails = !ui.showDetails
			if ui.showDetails {
				return ui, ui.loadAgentDetails(*ui.selectedAgent)
			}
		}
		return ui, nil

	case key.Matches(msg, ui.keyMap.Search):
		ui.searchMode = !ui.searchMode
		if ui.searchMode {
			ui.textinput.Focus()
		} else {
			ui.textinput.Blur()
		}
		return ui, nil

	case key.Matches(msg, ui.keyMap.Refresh):
		ui.loading = true
		return ui, ui.loadAgents()

	case key.Matches(msg, ui.keyMap.ClearSearch):
		ui.textinput.SetValue("")
		ui.filteredAgents = ui.agents
		ui.updateAgentList()
		return ui, nil

	case key.Matches(msg, ui.keyMap.FirstItem):
		if !ui.searchMode {
			ui.list.Select(0)
		}
		return ui, nil

	case key.Matches(msg, ui.keyMap.LastItem):
		if !ui.searchMode {
			ui.list.Select(len(ui.filteredAgents) - 1)
		}
		return ui, nil
	}

	return ui, nil
}

// Render different UI states
func (ui *EnhancedAgentSelection) renderAgentList() string {
	var b strings.Builder

	// Header
	header := ui.renderHeader()
	b.WriteString(header + "\n")

	// Search bar
	if ui.searchMode {
		searchBar := ui.renderSearchBar()
		b.WriteString(searchBar + "\n")
	}

	// Agent list
	b.WriteString(ui.list.View() + "\n")

	// Status bar
	statusBar := ui.renderStatusBar()
	b.WriteString(statusBar + "\n")

	// Help
	help := ui.renderHelp()
	b.WriteString(help)

	return b.String()
}

func (ui *EnhancedAgentSelection) renderWithDetails() string {
	var b strings.Builder

	// Split view: list on left, details on right
	listWidth := ui.width / 2 - 2
	detailsWidth := ui.width - listWidth - 4

	// Update list size for split view
	ui.list.SetSize(listWidth, ui.height-8)

	// Header
	header := ui.renderHeader()
	b.WriteString(header + "\n")

	// Main content area
	listView := ui.list.View()
	detailsView := ui.renderAgentDetails()

	// Side-by-side layout
	listStyle := lipgloss.NewStyle().
		Width(listWidth).
		Height(ui.height - 8).
		Border(lipgloss.RoundedBorder()).
		BorderForeground(lipgloss.Color("69"))

	detailsStyle := lipgloss.NewStyle().
		Width(detailsWidth).
		Height(ui.height - 8).
		Border(lipgloss.RoundedBorder()).
		BorderForeground(lipgloss.Color("170"))

	sideBySide := lipgloss.JoinHorizontal(
		lipgloss.Top,
		listStyle.Render(listView),
		detailsStyle.Render(detailsView),
	)

	b.WriteString(sideBySide + "\n")

	// Help
	help := ui.renderHelp()
	b.WriteString(help)

	return b.String()
}

func (ui *EnhancedAgentSelection) renderHeader() string {
	title := "🤖 Kubiya Enhanced Agent Selection"
	
	if ui.loading {
		title += " " + ui.spinner.View()
	}
	
	style := lipgloss.NewStyle().
		Bold(true).
		Foreground(lipgloss.Color("170")).
		Border(lipgloss.RoundedBorder()).
		BorderForeground(lipgloss.Color("170")).
		Padding(1, 2).
		Width(ui.width - 4).
		Align(lipgloss.Center)

	return style.Render(title)
}

func (ui *EnhancedAgentSelection) renderSearchBar() string {
	style := lipgloss.NewStyle().
		Border(lipgloss.RoundedBorder()).
		BorderForeground(lipgloss.Color("69")).
		Padding(1, 2).
		Width(ui.width - 4)

	content := fmt.Sprintf("🔍 Search: %s", ui.textinput.View())
	return style.Render(content)
}

func (ui *EnhancedAgentSelection) renderStatusBar() string {
	var status []string
	
	// Agent count
	total := len(ui.agents)
	shown := len(ui.filteredAgents)
	if total != shown {
		status = append(status, fmt.Sprintf("Showing %d of %d agents", shown, total))
	} else {
		status = append(status, fmt.Sprintf("%d agents available", total))
	}
	
	// Last refresh
	if !ui.lastRefresh.IsZero() {
		status = append(status, fmt.Sprintf("Last refresh: %s", ui.lastRefresh.Format("15:04:05")))
	}
	
	// Connection status
	onlineCount := 0
	for _, agentStatus := range ui.agentStatus {
		if agentStatus.Status == "online" {
			onlineCount++
		}
	}
	if onlineCount > 0 {
		status = append(status, fmt.Sprintf("%d online", onlineCount))
	}

	style := lipgloss.NewStyle().
		Foreground(lipgloss.Color("245")).
		Italic(true)

	return style.Render(strings.Join(status, " • "))
}

func (ui *EnhancedAgentSelection) renderAgentDetails() string {
	if ui.selectedAgent == nil {
		return "No agent selected"
	}

	agent := ui.selectedAgent
	var b strings.Builder

	// Agent header
	headerStyle := lipgloss.NewStyle().
		Bold(true).
		Foreground(lipgloss.Color("170")).
		Border(lipgloss.RoundedBorder()).
		BorderForeground(lipgloss.Color("170")).
		Padding(1).
		Width(ui.width/2 - 6)

	header := fmt.Sprintf("%s %s", getAgentTypeEmoji(agent.InstructionType), agent.Name)
	b.WriteString(headerStyle.Render(header) + "\n\n")

	// Description
	if agent.Description != "" {
		descStyle := lipgloss.NewStyle().
			Foreground(lipgloss.Color("252")).
			Italic(true).
			Margin(0, 0, 1, 0)
		b.WriteString(descStyle.Render(agent.Description) + "\n\n")
	}

	// Status
	if status, exists := ui.agentStatus[agent.UUID]; exists {
		statusStyle := lipgloss.NewStyle().
			Foreground(getStatusColor(status.Status)).
			Bold(true)
		b.WriteString(statusStyle.Render(fmt.Sprintf("Status: %s", status.Status)) + "\n")
		
		if status.ResponseTime > 0 {
			b.WriteString(fmt.Sprintf("Response Time: %s\n", time.Duration(status.ResponseTime).String()))
		}
		b.WriteString("\n")
	}

	// Technical details
	b.WriteString(ui.renderTechnicalDetails(agent))

	// Tools
	if len(agent.Tools) > 0 {
		b.WriteString(ui.renderToolsList(agent.Tools))
	}

	// Integrations
	if len(agent.Integrations) > 0 {
		b.WriteString(ui.renderIntegrationsList(agent.Integrations))
	}

	return b.String()
}

func (ui *EnhancedAgentSelection) renderTechnicalDetails(agent *kubiya.Agent) string {
	var b strings.Builder

	sectionStyle := lipgloss.NewStyle().
		Foreground(lipgloss.Color("69")).
		Bold(true).
		Margin(1, 0, 0, 0)

	itemStyle := lipgloss.NewStyle().
		Foreground(lipgloss.Color("245")).
		Margin(0, 0, 0, 2)

	b.WriteString(sectionStyle.Render("📋 Technical Details") + "\n")
	
	if agent.LLMModel != "" {
		b.WriteString(itemStyle.Render(fmt.Sprintf("• Model: %s", agent.LLMModel)) + "\n")
	}
	
	if agent.InstructionType != "" {
		b.WriteString(itemStyle.Render(fmt.Sprintf("• Type: %s", agent.InstructionType)) + "\n")
	}
	
	if len(agent.Runners) > 0 {
		b.WriteString(itemStyle.Render(fmt.Sprintf("• Runners: %s", strings.Join(agent.Runners, ", "))) + "\n")
	}
	
	if len(agent.Sources) > 0 {
		b.WriteString(itemStyle.Render(fmt.Sprintf("• Sources: %d", len(agent.Sources))) + "\n")
	}
	
	if len(agent.Owners) > 0 {
		b.WriteString(itemStyle.Render(fmt.Sprintf("• Owners: %s", strings.Join(agent.Owners, ", "))) + "\n")
	}
	
	if agent.IsDebugMode {
		b.WriteString(itemStyle.Render("• Debug Mode: Enabled") + "\n")
	}

	b.WriteString("\n")
	return b.String()
}

func (ui *EnhancedAgentSelection) renderToolsList(tools []string) string {
	var b strings.Builder

	sectionStyle := lipgloss.NewStyle().
		Foreground(lipgloss.Color("226")).
		Bold(true).
		Margin(1, 0, 0, 0)

	b.WriteString(sectionStyle.Render(fmt.Sprintf("🛠️  Tools (%d)", len(tools))) + "\n")

	// Show tools in a grid format
	itemStyle := lipgloss.NewStyle().
		Foreground(lipgloss.Color("245")).
		Margin(0, 0, 0, 2)

	for i, tool := range tools {
		if i >= 10 { // Limit display to first 10 tools
			remaining := len(tools) - 10
			b.WriteString(itemStyle.Render(fmt.Sprintf("  ... and %d more", remaining)) + "\n")
			break
		}
		b.WriteString(itemStyle.Render(fmt.Sprintf("• %s", tool)) + "\n")
	}

	b.WriteString("\n")
	return b.String()
}

func (ui *EnhancedAgentSelection) renderIntegrationsList(integrations []string) string {
	var b strings.Builder

	sectionStyle := lipgloss.NewStyle().
		Foreground(lipgloss.Color("141")).
		Bold(true).
		Margin(1, 0, 0, 0)

	b.WriteString(sectionStyle.Render(fmt.Sprintf("🔗 Integrations (%d)", len(integrations))) + "\n")

	itemStyle := lipgloss.NewStyle().
		Foreground(lipgloss.Color("245")).
		Margin(0, 0, 0, 2)

	for _, integration := range integrations {
		b.WriteString(itemStyle.Render(fmt.Sprintf("• %s", integration)) + "\n")
	}

	b.WriteString("\n")
	return b.String()
}

func (ui *EnhancedAgentSelection) renderHelp() string {
	var keys []key.Binding
	
	if ui.searchMode {
		keys = []key.Binding{
			ui.keyMap.Back,
			ui.keyMap.Select,
			ui.keyMap.ClearSearch,
		}
	} else {
		keys = []key.Binding{
			ui.keyMap.Select,
			ui.keyMap.ShowDetails,
			ui.keyMap.Search,
			ui.keyMap.Refresh,
			ui.keyMap.Back,
			ui.keyMap.Quit,
		}
	}

	return ui.help.ShortHelpView(keys)
}

func (ui *EnhancedAgentSelection) renderError() string {
	style := lipgloss.NewStyle().
		Foreground(lipgloss.Color("196")).
		Bold(true).
		Border(lipgloss.RoundedBorder()).
		BorderForeground(lipgloss.Color("196")).
		Padding(1, 2)

	return style.Render(fmt.Sprintf("❌ Error: %v\n\nPress 'r' to retry or 'q' to quit", ui.err))
}

func (ui *EnhancedAgentSelection) renderLoading() string {
	style := lipgloss.NewStyle().
		Foreground(lipgloss.Color("69")).
		Bold(true).
		Border(lipgloss.RoundedBorder()).
		BorderForeground(lipgloss.Color("69")).
		Padding(1, 2)

	return style.Render(fmt.Sprintf("Loading agents... %s", ui.spinner.View()))
}

// Helper functions
func (ui *EnhancedAgentSelection) updateLayout() {
	if ui.showDetails {
		ui.list.SetSize(ui.width/2-2, ui.height-8)
		ui.viewport.Width = ui.width/2 - 2
		ui.viewport.Height = ui.height - 8
	} else {
		ui.list.SetSize(ui.width-4, ui.height-8)
	}
	ui.textinput.Width = ui.width - 20
}

func (ui *EnhancedAgentSelection) updateAgentList() {
	items := make([]list.Item, len(ui.filteredAgents))
	for i, agent := range ui.filteredAgents {
		status := ui.agentStatus[agent.UUID]
		status.ToolsCount = len(agent.Tools) // Update tool count
		items[i] = enhancedAgentItem{
			agent:  agent,
			status: status,
		}
	}
	ui.list.SetItems(items)
	ui.lastRefresh = time.Now()
}

func (ui *EnhancedAgentSelection) filterAgents(query string) {
	if query == "" {
		ui.filteredAgents = ui.agents
		ui.updateAgentList()
		return
	}

	query = strings.ToLower(query)
	var filtered []kubiya.Agent

	for _, agent := range ui.agents {
		searchText := strings.ToLower(fmt.Sprintf("%s %s %s %s %s",
			agent.Name,
			agent.Description,
			agent.Desc,
			strings.Join(agent.Tools, " "),
			strings.Join(agent.Integrations, " ")))

		if strings.Contains(searchText, query) {
			filtered = append(filtered, agent)
		}
	}

	ui.filteredAgents = filtered
	ui.updateAgentList()
}

func (ui *EnhancedAgentSelection) loadAgents() tea.Cmd {
	return func() tea.Msg {
		agents, err := ui.client.ListAgents(ui.ctx)
		if err != nil {
			return err
		}
		
		// Sort agents by name
		sort.Slice(agents, func(i, j int) bool {
			return agents[i].Name < agents[j].Name
		})
		
		return agents
	}
}

func (ui *EnhancedAgentSelection) loadAgentDetails(agent kubiya.Agent) tea.Cmd {
	ui.detailsLoading = true
	return func() tea.Msg {
		// Get detailed agent info
		detailedAgent, err := ui.client.GetAgent(ui.ctx, agent.UUID)
		if err != nil {
			return err
		}
		
		ui.detailedAgent = detailedAgent
		ui.detailsLoading = false
		
		return nil
	}
}

func (ui *EnhancedAgentSelection) validateAgents() tea.Cmd {
	return func() tea.Msg {
		// Validate each agent and update status
		for _, agent := range ui.agents {
			status := AgentStatus{
				Available:    true,
				Status:       "online",
				ToolsCount:   len(agent.Tools),
				LastChecked:  time.Now(),
				ResponseTime: duration(time.Millisecond * 100), // Mock response time
			}
			
			// Basic validation
			if agent.UUID == "" || agent.Name == "" {
				status.Available = false
				status.Status = "offline"
				status.ErrorMessage = "Invalid agent configuration"
			}
			
			ui.agentStatus[agent.UUID] = status
		}
		
		return nil
	}
}

func (ui *EnhancedAgentSelection) selectAgent(agent kubiya.Agent) tea.Cmd {
	return func() tea.Msg {
		if ui.onAgentSelected != nil {
			ui.onAgentSelected(agent)
		}
		return nil
	}
}

func getStatusColor(status string) lipgloss.Color {
	switch status {
	case "online":
		return lipgloss.Color("46")
	case "busy":
		return lipgloss.Color("226")
	case "offline":
		return lipgloss.Color("196")
	default:
		return lipgloss.Color("245")
	}
}

// Public methods for integration
func (ui *EnhancedAgentSelection) OnAgentSelected(callback func(agent kubiya.Agent)) {
	ui.onAgentSelected = callback
}

func (ui *EnhancedAgentSelection) OnBack(callback func()) {
	ui.onBack = callback
}

func (ui *EnhancedAgentSelection) OnError(callback func(error)) {
	ui.onError = callback
}

func (ui *EnhancedAgentSelection) Run() error {
	p := tea.NewProgram(ui, tea.WithAltScreen())
	_, err := p.Run()
	return err
}

func (ui *EnhancedAgentSelection) Cleanup() {
	ui.cancel()
}