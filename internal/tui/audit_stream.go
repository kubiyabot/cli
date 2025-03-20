package tui

import (
	"context"
	"fmt"
	"io"
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
	"github.com/kubiyabot/cli/internal/kubiya"
)

// Define UI colors
var (
	// Base colors
	primaryColor   = lipgloss.Color("#7D56F4") // Purple
	secondaryColor = lipgloss.Color("#0CA4A5") // Teal
	accentColor    = lipgloss.Color("#F25D94") // Pink
	textColor      = lipgloss.Color("#FFFFFF") // White
	dimTextColor   = lipgloss.Color("#CCCCCC") // Light gray
	bgColor        = lipgloss.Color("#1A1B26") // Dark blue-gray

	// Status colors
	successColor = lipgloss.Color("#73F59F") // Green
	warningColor = lipgloss.Color("#F2C14E") // Yellow
	errorColor   = lipgloss.Color("#F25757") // Red

	// Category colors
	categoryColors = map[string]lipgloss.Color{
		"webhook":        lipgloss.Color("#61AFEF"), // Blue
		"triggers":       lipgloss.Color("#E06C75"), // Red-pink
		"agents":         lipgloss.Color("#98C379"), // Green
		"ai":             lipgloss.Color("#C678DD"), // Purple
		"tool_execution": lipgloss.Color("#E5C07B"), // Yellow
	}

	// UI component styles
	appTitleStyle = lipgloss.NewStyle().
			Foreground(primaryColor).
			Bold(true).
			MarginBottom(1)

	subtitleStyle = lipgloss.NewStyle().
			Foreground(secondaryColor).
			Bold(true)

	headerStyle = lipgloss.NewStyle().
			Foreground(textColor).
			Background(primaryColor).
			Padding(0, 1).
			Bold(true)

	tabStyle = lipgloss.NewStyle().
			Padding(0, 3).
			Foreground(dimTextColor)

	activeTabStyle = tabStyle.Copy().
			Foreground(primaryColor).
			Background(lipgloss.Color("#252636")).
			Bold(true).
			Border(lipgloss.Border{Bottom: "─"}, false, false, true, false).
			BorderForeground(accentColor)

	tabGapStyle = lipgloss.NewStyle().
			BorderForeground(lipgloss.Color("#333333")).
			Border(lipgloss.Border{Bottom: "─"}, false, false, true, false)

	infoTextStyle = lipgloss.NewStyle().
			Foreground(dimTextColor)

	helpStyle = lipgloss.NewStyle().
			Foreground(dimTextColor)

	// Event card styles
	cardStyle = lipgloss.NewStyle().
			Border(lipgloss.RoundedBorder()).
			BorderForeground(lipgloss.Color("#333333")).
			Padding(1, 2).
			Margin(0, 0, 1, 0)

	detailBoxStyle = lipgloss.NewStyle().
			Border(lipgloss.Border{
			Top:         "─",
			Bottom:      "─",
			Left:        "│",
			Right:       "│",
			TopLeft:     "┌",
			TopRight:    "┐",
			BottomLeft:  "└",
			BottomRight: "┘",
		}).
		BorderForeground(lipgloss.Color("#555555")).
		Padding(1, 2).
		Margin(1, 0)

	timestampStyle = lipgloss.NewStyle().
			Foreground(dimTextColor).
			Align(lipgloss.Right)

	categoryStyle = lipgloss.NewStyle().
			Bold(true)

	keyValueStyle = lipgloss.NewStyle().
			Width(15).
			Foreground(secondaryColor)

	valueStyle = lipgloss.NewStyle().
			Foreground(textColor)

	filterInputStyle = lipgloss.NewStyle().
				Border(lipgloss.RoundedBorder()).
				BorderForeground(primaryColor).
				Padding(0, 1).
				Width(30)

	filterLabelStyle = lipgloss.NewStyle().
				Foreground(secondaryColor).
				PaddingRight(1)

	filterBoxStyle = lipgloss.NewStyle().
			BorderForeground(primaryColor).
			Border(lipgloss.RoundedBorder()).
			Padding(1, 2).
			Margin(1, 0)

	statusTextStyle = lipgloss.NewStyle().
			Padding(0, 1).
			Bold(true)

	spinnerStyle = lipgloss.NewStyle().
			Foreground(accentColor)

	// Define layout dimensions
	docStyle = lipgloss.NewStyle().
			Background(bgColor).
			Padding(1, 2)
)

// EventItem represents a list item for the audit events
type EventItem struct {
	auditItem kubiya.AuditItem
	timestamp time.Time
	title     string
	desc      string
}

// UIMode represents the different UI modes
type UIMode int

const (
	NormalMode UIMode = iota
	SearchMode
	FilterUIMode
	ViewMode
)

// Custom tea commands
type tickCmd struct{}
type fetchedAuditItemsCmd struct{ items []kubiya.AuditItem }
type errorCmd struct{ err error }

func tick() tea.Cmd {
	return func() tea.Msg {
		return tickCmd{}
	}
}

func fetchAuditItems(client *kubiya.Client, query kubiya.AuditQuery) tea.Cmd {
	return func() tea.Msg {
		ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()

		items, err := client.Audit().ListAuditItems(ctx, query)
		if err != nil {
			return errorCmd{err}
		}

		return fetchedAuditItemsCmd{items}
	}
}

// Implement list.Item interface
func (i EventItem) Title() string       { return i.title }
func (i EventItem) Description() string { return i.desc }
func (i EventItem) FilterValue() string {
	// Create a rich filter value combining multiple fields
	return fmt.Sprintf("%s %s %s %s %s",
		i.auditItem.CategoryType,
		i.auditItem.CategoryName,
		i.auditItem.ResourceType,
		i.auditItem.ResourceText,
		i.auditItem.ActionType,
	)
}

// KeyMap defines key bindings
type KeyMap struct {
	Up        key.Binding
	Down      key.Binding
	Left      key.Binding
	Right     key.Binding
	Help      key.Binding
	Quit      key.Binding
	Filter    key.Binding
	Search    key.Binding
	ClearAll  key.Binding
	ViewEvent key.Binding
	Back      key.Binding
	Refresh   key.Binding
}

// FullHelp implements help.KeyMap interface
func (k KeyMap) FullHelp() [][]key.Binding {
	return [][]key.Binding{
		{k.Up, k.Down, k.Left, k.Right},
		{k.Help, k.Quit, k.Filter, k.Search},
		{k.ClearAll, k.ViewEvent, k.Back, k.Refresh},
	}
}

// ShortHelp implements help.KeyMap interface
func (k KeyMap) ShortHelp() []key.Binding {
	return []key.Binding{k.Help, k.Quit}
}

// DefaultKeyMap returns the default keybindings
func DefaultKeyMap() KeyMap {
	return KeyMap{
		Up: key.NewBinding(
			key.WithKeys("up", "k"),
			key.WithHelp("↑/k", "up"),
		),
		Down: key.NewBinding(
			key.WithKeys("down", "j"),
			key.WithHelp("↓/j", "down"),
		),
		Left: key.NewBinding(
			key.WithKeys("left", "h"),
			key.WithHelp("←/h", "prev tab"),
		),
		Right: key.NewBinding(
			key.WithKeys("right", "l"),
			key.WithHelp("→/l", "next tab"),
		),
		Help: key.NewBinding(
			key.WithKeys("?"),
			key.WithHelp("?", "help"),
		),
		Quit: key.NewBinding(
			key.WithKeys("q", "ctrl+c"),
			key.WithHelp("q", "quit"),
		),
		Filter: key.NewBinding(
			key.WithKeys("f"),
			key.WithHelp("f", "filter"),
		),
		Search: key.NewBinding(
			key.WithKeys("/"),
			key.WithHelp("/", "search"),
		),
		ClearAll: key.NewBinding(
			key.WithKeys("c"),
			key.WithHelp("c", "clear"),
		),
		ViewEvent: key.NewBinding(
			key.WithKeys("enter"),
			key.WithHelp("enter", "view details"),
		),
		Back: key.NewBinding(
			key.WithKeys("esc"),
			key.WithHelp("esc", "back"),
		),
		Refresh: key.NewBinding(
			key.WithKeys("r"),
			key.WithHelp("r", "refresh"),
		),
	}
}

// Tab represents a category of events to display
type Tab struct {
	Name   string
	Filter string // Category type filter
}

// Custom item delegate for event rendering
type EventDelegate struct{}

func (d EventDelegate) Height() int                             { return 3 }
func (d EventDelegate) Spacing() int                            { return 1 }
func (d EventDelegate) Update(_ tea.Msg, _ *list.Model) tea.Cmd { return nil }
func (d EventDelegate) Render(w io.Writer, m list.Model, index int, item list.Item) {
	eventItem, ok := item.(EventItem)
	if !ok {
		fmt.Fprint(w, "Error: item is not an EventItem")
		return
	}

	// Get category color
	catColor, exists := categoryColors[eventItem.auditItem.CategoryType]
	if !exists {
		catColor = secondaryColor
	}

	categoryColorStyle := categoryStyle.Copy().Foreground(catColor)

	// Icon based on category
	icon := "•"
	switch eventItem.auditItem.CategoryType {
	case "agents":
		icon = "👤"
	case "ai":
		icon = "🤖"
	case "webhook":
		icon = "📡"
	case "tool_execution":
		icon = "🔧"
	case "triggers":
		icon = "🔔"
	}

	// Style based on whether item is selected
	var itemStyle lipgloss.Style
	if index == m.Index() {
		itemStyle = cardStyle.Copy().
			BorderForeground(accentColor).
			Bold(true)
	} else {
		itemStyle = cardStyle
	}

	// Format status
	statusText := "Success"
	statusStyle := statusTextStyle.Copy().
		Background(successColor).
		Foreground(lipgloss.Color("#000000"))

	if !eventItem.auditItem.ActionSuccessful {
		statusText = "Failed"
		statusStyle = statusTextStyle.Copy().
			Background(errorColor).
			Foreground(lipgloss.Color("#FFFFFF"))
	}

	// Create header row with timestamp right-aligned
	header := lipgloss.JoinHorizontal(
		lipgloss.Left,
		fmt.Sprintf("%s %s",
			icon,
			categoryColorStyle.Render(eventItem.auditItem.CategoryType+"/"+eventItem.auditItem.CategoryName),
		),
	)

	// Add status and timestamp
	headerWidth := lipgloss.Width(header)
	statusWidth := lipgloss.Width(statusStyle.Render(statusText))
	timestampStr := eventItem.timestamp.Format("15:04:05")
	timeWidth := lipgloss.Width(timestampStyle.Render(timestampStr))

	width := m.Width() - 2
	padding := width - headerWidth - statusWidth - timeWidth - 6
	if padding < 1 {
		padding = 1
	}

	header = lipgloss.JoinHorizontal(
		lipgloss.Center,
		header,
		strings.Repeat(" ", padding),
		statusStyle.Render(statusText)+" ",
		timestampStyle.Render(timestampStr),
	)

	// Create content with resource info and action
	resource := eventItem.auditItem.ResourceType
	if eventItem.auditItem.ResourceText != "" {
		if resource != "" {
			resource += ": "
		}
		resource += eventItem.auditItem.ResourceText
	}

	content := lipgloss.JoinVertical(
		lipgloss.Left,
		header,
		lipgloss.JoinHorizontal(
			lipgloss.Left,
			keyValueStyle.Render("Action:"),
			valueStyle.Render(eventItem.auditItem.ActionType),
		),
		lipgloss.JoinHorizontal(
			lipgloss.Left,
			keyValueStyle.Render("Resource:"),
			valueStyle.Render(resource),
		),
	)

	fmt.Fprint(w, itemStyle.Width(width).Render(content))
}

// AuditStreamModel is the BubbleTea model for our improved UI
type AuditStreamModel struct {
	client          *kubiya.Client
	query           kubiya.AuditQuery
	keymap          KeyMap
	help            help.Model
	spinner         spinner.Model
	viewport        viewport.Model
	eventLists      map[string]list.Model
	tabs            []Tab
	activeTab       int
	events          []kubiya.AuditItem
	processedItems  map[string]bool
	latestTimestamp string
	filterInput     textinput.Model
	searchInput     textinput.Model
	mode            UIMode
	selectedEvent   *kubiya.AuditItem
	width           int
	height          int
	ready           bool
	err             error
	pollCount       int
	loading         bool
	verbose         bool
	filterInfo      string
	showHelp        bool
	filterText      string
}

// NewAuditStreamModel creates a new audit stream model with the improved UI
func NewAuditStreamModel(client *kubiya.Client, query kubiya.AuditQuery, verbose bool) AuditStreamModel {
	keymap := DefaultKeyMap()

	// Set up spinner
	sp := spinner.New()
	sp.Spinner = spinner.Dot
	sp.Style = spinnerStyle

	// Create help model
	help := help.New()
	help.ShowAll = false
	help.Styles.ShortKey = helpStyle.Copy()
	help.Styles.FullKey = helpStyle.Copy()
	help.Styles.ShortDesc = helpStyle.Copy()
	help.Styles.FullDesc = helpStyle.Copy()

	// Define tabs for different event categories
	tabs := []Tab{
		{Name: "All", Filter: ""},
		{Name: "Webhooks", Filter: "webhook"},
		{Name: "Agents", Filter: "agents"},
		{Name: "Triggers", Filter: "triggers"},
		{Name: "Tools", Filter: "tool_execution"},
		{Name: "AI", Filter: "ai"},
	}

	// Create filter input
	filterInput := textinput.New()
	filterInput.Placeholder = "Type to filter..."
	filterInput.CharLimit = 50
	filterInput.Width = 30

	// Create search input
	searchInput := textinput.New()
	searchInput.Placeholder = "Search..."
	searchInput.CharLimit = 50
	searchInput.Width = 30

	// Create list models for each tab
	eventLists := make(map[string]list.Model)
	for _, tab := range tabs {
		// Use DefaultDelegate which expects list.Item interface
		delegate := list.NewDefaultDelegate()
		listModel := list.New([]list.Item{}, delegate, 0, 0)
		listModel.SetShowStatusBar(false)
		listModel.SetFilteringEnabled(false)
		listModel.SetShowHelp(false)
		listModel.SetShowTitle(false)
		listModel.DisableQuitKeybindings()
		eventLists[tab.Name] = listModel
	}

	// Create filter info string
	var filterInfo strings.Builder
	if query.Filter.CategoryType != "" {
		filterInfo.WriteString(fmt.Sprintf("Category: %s ",
			lipgloss.NewStyle().Foreground(accentColor).Render(query.Filter.CategoryType)))
	}
	if query.Filter.CategoryName != "" {
		filterInfo.WriteString(fmt.Sprintf("Name: %s ",
			lipgloss.NewStyle().Foreground(accentColor).Render(query.Filter.CategoryName)))
	}
	if query.Filter.ResourceType != "" {
		filterInfo.WriteString(fmt.Sprintf("Resource: %s ",
			lipgloss.NewStyle().Foreground(accentColor).Render(query.Filter.ResourceType)))
	}
	if query.Filter.ActionType != "" {
		filterInfo.WriteString(fmt.Sprintf("Action: %s ",
			lipgloss.NewStyle().Foreground(accentColor).Render(query.Filter.ActionType)))
	}
	if query.Filter.SessionID != "" {
		filterInfo.WriteString(fmt.Sprintf("Session: %s ",
			lipgloss.NewStyle().Foreground(accentColor).Render(query.Filter.SessionID)))
	}
	if query.Filter.Timestamp.GTE != "" {
		filterInfo.WriteString(fmt.Sprintf("From: %s ",
			lipgloss.NewStyle().Foreground(accentColor).Render(query.Filter.Timestamp.GTE)))
	}

	return AuditStreamModel{
		client:          client,
		query:           query,
		keymap:          keymap,
		help:            help,
		spinner:         sp,
		viewport:        viewport.New(0, 0),
		tabs:            tabs,
		eventLists:      eventLists,
		events:          []kubiya.AuditItem{},
		processedItems:  make(map[string]bool),
		latestTimestamp: query.Filter.Timestamp.GTE,
		filterInput:     filterInput,
		searchInput:     searchInput,
		width:           0,
		height:          0,
		loading:         true,
		verbose:         verbose,
		filterInfo:      filterInfo.String(),
		mode:            NormalMode,
	}
}

// Init initializes the model
func (m *AuditStreamModel) Init() tea.Cmd {
	return tea.Batch(
		m.spinner.Tick,
		fetchAuditItems(m.client, m.query),
	)
}

// Update handles model updates
func (m *AuditStreamModel) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	var cmds []tea.Cmd

	// Common update logic for all modes
	switch msg := msg.(type) {
	case tea.KeyMsg:
		// Global key handlers that work in any mode
		switch {
		case key.Matches(msg, m.keymap.Quit):
			return m, tea.Quit

		case key.Matches(msg, m.keymap.Help):
			m.showHelp = !m.showHelp
			return m, nil

		case key.Matches(msg, m.keymap.Back) && m.mode != NormalMode:
			if m.mode == ViewMode {
				m.selectedEvent = nil
			}
			m.mode = NormalMode
			return m, nil
		}

		// Mode-specific key handlers
		if m.mode == NormalMode {
			switch {
			case key.Matches(msg, m.keymap.Left):
				m.activeTab = max(0, m.activeTab-1)
				return m, nil

			case key.Matches(msg, m.keymap.Right):
				m.activeTab = min(len(m.tabs)-1, m.activeTab+1)
				return m, nil

			case key.Matches(msg, m.keymap.Filter):
				m.mode = FilterUIMode
				m.filterInput.Focus()
				return m, nil

			case key.Matches(msg, m.keymap.Search):
				m.mode = SearchMode
				m.searchInput.Focus()
				return m, nil

			case key.Matches(msg, m.keymap.ClearAll):
				m.events = []kubiya.AuditItem{}
				m.processedItems = make(map[string]bool)
				m.updateLists()
				return m, nil

			case key.Matches(msg, m.keymap.Refresh):
				return m, fetchAuditItems(m.client, m.query)

			case key.Matches(msg, m.keymap.ViewEvent):
				// Get the selected event from the active list
				activeList := m.eventLists[m.tabs[m.activeTab].Name]
				if activeList.SelectedItem() != nil {
					eventItem := activeList.SelectedItem().(EventItem)
					m.selectedEvent = &eventItem.auditItem
					m.mode = ViewMode
				}
				return m, nil
			}
		}

	case tea.WindowSizeMsg:
		m.width = msg.Width
		m.height = msg.Height

		// Adjust sub-components based on window size
		headerHeight := 7 // Title + tabs + spacing
		footerHeight := 2 // Help text
		contentHeight := m.height - headerHeight - footerHeight

		// Update viewport and lists for new size
		m.viewport.Width = msg.Width - 4
		m.viewport.Height = contentHeight

		for name, lst := range m.eventLists {
			lst.SetSize(msg.Width-4, contentHeight)
			m.eventLists[name] = lst
		}

		m.ready = true

	case tickCmd:
		// Update query with latest timestamp
		if m.latestTimestamp != "" {
			m.query.Filter.Timestamp.GTE = m.latestTimestamp
		}

		// Increment poll count
		m.pollCount++

		// Fetch new items
		m.loading = true
		return m, tea.Batch(
			m.spinner.Tick,
			fetchAuditItems(m.client, m.query),
		)

	case fetchedAuditItemsCmd:
		m.loading = false

		// Process new items
		if len(msg.items) > 0 {
			newItems := false
			for _, item := range msg.items {
				// Skip if already processed
				eventKey := fmt.Sprintf("%s-%s-%s-%s", item.Timestamp, item.CategoryType, item.CategoryName, item.ActionType)
				if _, seen := m.processedItems[eventKey]; seen {
					continue
				}

				// Mark as processed
				m.processedItems[eventKey] = true

				// Update latest timestamp if newer
				if item.Timestamp > m.latestTimestamp {
					m.latestTimestamp = item.Timestamp
				}

				// Add to events list
				m.events = append(m.events, item)
				newItems = true
			}

			// Update lists if we received new items
			if newItems {
				m.updateLists()
			}
		}

		// Schedule next poll in 3 seconds
		return m, tea.Batch(
			tea.Tick(3*time.Second, func(time.Time) tea.Msg {
				return tickCmd{}
			}),
		)

	case spinner.TickMsg:
		var cmd tea.Cmd
		m.spinner, cmd = m.spinner.Update(msg)
		cmds = append(cmds, cmd)

	case errorCmd:
		m.err = msg.err
		m.loading = false
		return m, tea.Tick(3*time.Second, func(time.Time) tea.Msg {
			return tickCmd{}
		})
	}

	// Mode-specific updates
	switch m.mode {
	case NormalMode:
		// Update active list
		activeList := m.eventLists[m.tabs[m.activeTab].Name]
		updatedList, cmd := activeList.Update(msg)
		m.eventLists[m.tabs[m.activeTab].Name] = updatedList
		cmds = append(cmds, cmd)

	case FilterUIMode:
		var cmd tea.Cmd
		m.filterInput, cmd = m.filterInput.Update(msg)
		cmds = append(cmds, cmd)

		// Apply filter when enter is pressed
		if msg, ok := msg.(tea.KeyMsg); ok && msg.Type == tea.KeyEnter {
			filterText := m.filterInput.Value()
			// Store the filter text for future use
			m.filterText = filterText
			// Update lists with filter applied
			m.updateListsWithFilter()
			m.mode = NormalMode
		}

	case SearchMode:
		var cmd tea.Cmd
		m.searchInput, cmd = m.searchInput.Update(msg)
		cmds = append(cmds, cmd)

		// Search when enter is pressed
		if msg, ok := msg.(tea.KeyMsg); ok && msg.Type == tea.KeyEnter {
			searchText := m.searchInput.Value()
			m.searchEvents(searchText)
			m.mode = NormalMode
		}

	case ViewMode:
		// Nothing special yet, just handle viewport scrolling
		if m.selectedEvent != nil {
			var cmd tea.Cmd
			m.viewport, cmd = m.viewport.Update(msg)
			cmds = append(cmds, cmd)
		}
	}

	return m, tea.Batch(cmds...)
}

// View renders the UI
func (m *AuditStreamModel) View() string {
	if !m.ready {
		return "Initializing..."
	}

	// Common UI elements
	var view strings.Builder

	// Header section
	titleText := appTitleStyle.Render("Kubiya Audit Stream")
	view.WriteString(titleText + "\n")

	// Add tabs
	var tabsContent strings.Builder
	for i, tab := range m.tabs {
		tabText := tab.Name
		if i == m.activeTab {
			tabsContent.WriteString(activeTabStyle.Render(tabText))
		} else {
			tabsContent.WriteString(tabStyle.Render(tabText))
		}
	}

	// Fill remaining space with border
	tabsWidth := lipgloss.Width(tabsContent.String())
	tabsContent.WriteString(tabGapStyle.Width(m.width - tabsWidth - 4).Render(""))

	view.WriteString(tabsContent.String() + "\n\n")

	// Status line with spinner, poll count, and event count
	statusLine := fmt.Sprintf(
		"%s %s %s",
		m.spinner.View(),
		infoTextStyle.Render(fmt.Sprintf("Poll #%d", m.pollCount)),
		infoTextStyle.Render(fmt.Sprintf("Events: %d", len(m.events))),
	)

	// Add error if any
	if m.err != nil {
		statusLine += " " + lipgloss.NewStyle().Foreground(errorColor).Render(fmt.Sprintf("Error: %v", m.err))
	}

	view.WriteString(statusLine + "\n")

	// Add filter info if applicable
	if m.filterInfo != "" {
		view.WriteString(infoTextStyle.Render("Filters: "+m.filterInfo) + "\n")
	}

	// Main content changes based on mode
	switch m.mode {
	case NormalMode:
		activeList := m.eventLists[m.tabs[m.activeTab].Name]
		view.WriteString(activeList.View())

	case FilterUIMode:
		// Display filter input interface
		view.WriteString(filterBoxStyle.Render(
			lipgloss.JoinVertical(
				lipgloss.Left,
				subtitleStyle.Render("Filter Events"),
				"",
				lipgloss.JoinHorizontal(
					lipgloss.Left,
					filterLabelStyle.Render("Filter:"),
					filterInputStyle.Render(m.filterInput.View()),
				),
				"",
				infoTextStyle.Render("Enter to apply filter, Esc to cancel"),
			),
		))

	case SearchMode:
		// Display search input interface
		view.WriteString(filterBoxStyle.Render(
			lipgloss.JoinVertical(
				lipgloss.Left,
				subtitleStyle.Render("Search Events"),
				"",
				lipgloss.JoinHorizontal(
					lipgloss.Left,
					filterLabelStyle.Render("Search:"),
					filterInputStyle.Render(m.searchInput.View()),
				),
				"",
				infoTextStyle.Render("Enter to search, Esc to cancel"),
			),
		))

	case ViewMode:
		if m.selectedEvent != nil {
			view.WriteString(m.renderDetailView(*m.selectedEvent))
		}
	}

	// Help section at the bottom if enabled
	if m.showHelp {
		view.WriteString("\n" + m.help.View(m.keymap))
	} else {
		view.WriteString("\n" + helpStyle.Render("Press ? for help • Press q to quit"))
	}

	// Apply document style
	return docStyle.Render(view.String())
}

// Helper method to update all lists with filtered events
func (m *AuditStreamModel) updateLists() {
	// First sort events by timestamp, newest first
	sort.Slice(m.events, func(i, j int) bool {
		return m.events[i].Timestamp > m.events[j].Timestamp
	})

	// Clear and rebuild all lists
	for _, tab := range m.tabs {
		var items []list.Item

		for _, event := range m.events {
			// Apply tab filtering
			if tab.Filter != "" && event.CategoryType != tab.Filter {
				continue
			}

			// Parse timestamp
			timestamp, err := time.Parse(time.RFC3339, event.Timestamp)
			if err != nil {
				timestamp = time.Now()
			}

			// Create list item
			resource := event.ResourceType
			if event.ResourceText != "" {
				if resource != "" {
					resource += ": "
				}
				resource += event.ResourceText
			}

			// Create formatted title and description
			title := fmt.Sprintf("%s/%s: %s",
				event.CategoryType,
				event.CategoryName,
				event.ActionType)

			desc := fmt.Sprintf("%s • %s",
				resource,
				timestamp.Format("15:04:05"))

			item := EventItem{
				auditItem: event,
				timestamp: timestamp,
				title:     title,
				desc:      desc,
			}

			items = append(items, item)
		}

		// Update the tab's list
		listModel := m.eventLists[tab.Name]
		listModel.SetItems(items)
		m.eventLists[tab.Name] = listModel
	}
}

// Helper method to search events matching the search text
func (m *AuditStreamModel) searchEvents(searchText string) {
	if searchText == "" {
		return
	}

	// Store the search text as filter text
	m.filterText = searchText
	// Update lists with filter applied
	m.updateListsWithFilter()
}

// Helper method that updates all lists with filtering
func (m *AuditStreamModel) updateListsWithFilter() {
	// If filter is empty, just update normally
	if m.filterText == "" {
		m.updateLists()
		return
	}

	// First sort events by timestamp, newest first
	sort.Slice(m.events, func(i, j int) bool {
		return m.events[i].Timestamp > m.events[j].Timestamp
	})

	// Clear and rebuild all lists with filtering
	for _, tab := range m.tabs {
		var items []list.Item

		for _, event := range m.events {
			// Apply tab filtering
			if tab.Filter != "" && event.CategoryType != tab.Filter {
				continue
			}

			// Create a filter string from the event for matching
			filterStr := strings.ToLower(fmt.Sprintf("%s %s %s %s %s",
				event.CategoryType,
				event.CategoryName,
				event.ResourceType,
				event.ResourceText,
				event.ActionType,
			))

			// Check if this event matches the filter
			if !strings.Contains(filterStr, strings.ToLower(m.filterText)) {
				continue
			}

			// Parse timestamp
			timestamp, err := time.Parse(time.RFC3339, event.Timestamp)
			if err != nil {
				timestamp = time.Now()
			}

			// Create list item
			resource := event.ResourceType
			if event.ResourceText != "" {
				if resource != "" {
					resource += ": "
				}
				resource += event.ResourceText
			}

			// Create formatted title and description
			title := fmt.Sprintf("%s/%s: %s",
				event.CategoryType,
				event.CategoryName,
				event.ActionType)

			desc := fmt.Sprintf("%s • %s",
				resource,
				timestamp.Format("15:04:05"))

			item := EventItem{
				auditItem: event,
				timestamp: timestamp,
				title:     title,
				desc:      desc,
			}

			items = append(items, item)
		}

		// Update the tab's list
		listModel := m.eventLists[tab.Name]
		listModel.SetItems(items)
		m.eventLists[tab.Name] = listModel
	}
}

// Helper method to render detailed view of a selected event
func (m *AuditStreamModel) renderDetailView(item kubiya.AuditItem) string {
	var content strings.Builder

	// Parse timestamp
	timestamp, err := time.Parse(time.RFC3339, item.Timestamp)
	if err != nil {
		timestamp = time.Now()
	}

	// Get category color
	catColor, exists := categoryColors[item.CategoryType]
	if !exists {
		catColor = secondaryColor
	}

	// Create header
	header := lipgloss.JoinHorizontal(
		lipgloss.Left,
		lipgloss.NewStyle().Foreground(catColor).Bold(true).Render(
			fmt.Sprintf("%s/%s", item.CategoryType, item.CategoryName),
		),
		"  ",
		lipgloss.NewStyle().Foreground(accentColor).Bold(true).Render(item.ActionType),
		"  ",
		timestampStyle.Render(timestamp.Format("2006-01-02 15:04:05")),
	)

	content.WriteString(subtitleStyle.Render("Event Details") + "\n\n")
	content.WriteString(header + "\n\n")

	// Basic event info
	basicInfo := []struct {
		label string
		value string
	}{
		{"Resource Type", item.ResourceType},
		{"Resource", item.ResourceText},
		{"Status", fmt.Sprintf("%t", item.ActionSuccessful)},
	}

	for _, info := range basicInfo {
		if info.value != "" {
			content.WriteString(fmt.Sprintf("%s %s\n",
				keyValueStyle.Render(info.label+":"),
				valueStyle.Render(info.value),
			))
		}
	}

	// Add extra fields
	if len(item.Extra) > 0 {
		content.WriteString("\n" + subtitleStyle.Render("Additional Data") + "\n\n")

		// Get sorted keys for consistent output
		keys := make([]string, 0, len(item.Extra))
		for k := range item.Extra {
			keys = append(keys, k)
		}
		sort.Strings(keys)

		for _, key := range keys {
			value := item.Extra[key]

			// Format different types of values
			var formattedValue string
			switch v := value.(type) {
			case string:
				formattedValue = v

			case map[string]interface{}:
				// For nested objects, format as indented key-value pairs
				var mapContent strings.Builder
				mapContent.WriteString("\n")

				// Get sorted map keys
				mapKeys := make([]string, 0, len(v))
				for k := range v {
					mapKeys = append(mapKeys, k)
				}
				sort.Strings(mapKeys)

				for _, mk := range mapKeys {
					mapVal := v[mk]
					mapContent.WriteString(fmt.Sprintf("    %s: %v\n",
						lipgloss.NewStyle().Foreground(secondaryColor).Render(mk),
						mapVal,
					))
				}
				formattedValue = mapContent.String()

			case []interface{}:
				// For arrays, format as list
				var arrContent strings.Builder
				arrContent.WriteString(fmt.Sprintf("\n    %s\n",
					lipgloss.NewStyle().Foreground(dimTextColor).Render(fmt.Sprintf("(%d items)", len(v))),
				))

				for i, item := range v {
					if i >= 10 {
						arrContent.WriteString(fmt.Sprintf("    %s\n",
							lipgloss.NewStyle().Foreground(dimTextColor).Render("... more items"),
						))
						break
					}
					arrContent.WriteString(fmt.Sprintf("    • %v\n", item))
				}
				formattedValue = arrContent.String()

			default:
				formattedValue = fmt.Sprintf("%v", v)
			}

			content.WriteString(fmt.Sprintf("%s %s\n",
				keyValueStyle.Render(key+":"),
				valueStyle.Render(formattedValue),
			))
		}
	}

	// Wrap content in a scrolling viewport
	m.viewport.SetContent(content.String())

	// Return viewport view inside detail box
	return detailBoxStyle.Render(m.viewport.View())
}

// StartAuditStream starts the audit stream TUI
func StartAuditStream(client *kubiya.Client, query kubiya.AuditQuery, verbose bool) error {
	model := NewAuditStreamModel(client, query, verbose)
	p := tea.NewProgram(
		&model,
		tea.WithAltScreen(),
	)

	_, err := p.Run()
	return err
}
