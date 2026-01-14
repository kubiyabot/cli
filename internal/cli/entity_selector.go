package cli

import (
	"fmt"
	"io"
	"strings"

	"github.com/charmbracelet/bubbles/list"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
)

var (
	entityTitleStyle        = lipgloss.NewStyle().MarginLeft(2).Bold(true)
	entityItemStyle         = lipgloss.NewStyle().PaddingLeft(4)
	entitySelectedItemStyle = lipgloss.NewStyle().PaddingLeft(2).Foreground(lipgloss.Color("170"))
	entityDescStyle         = lipgloss.NewStyle().PaddingLeft(6).Foreground(lipgloss.Color("241"))
	entityPaginationStyle   = list.DefaultStyles().PaginationStyle.PaddingLeft(4)
	entityHelpStyle         = list.DefaultStyles().HelpStyle.PaddingLeft(4).PaddingBottom(1)
)

// entitySelectItem implements list.Item for matched entities
type entitySelectItem struct {
	match EntityMatch
}

func (i entitySelectItem) Title() string {
	icon := "🤖"
	if i.match.EntityType == "team" {
		icon = "👥"
	}
	return fmt.Sprintf("%s %s (Score: %.0f%%)", icon, i.match.EntityName, i.match.Score*100)
}

func (i entitySelectItem) Description() string {
	matchedOn := strings.Join(i.match.MatchedOn, ", ")
	return fmt.Sprintf("Matched: %s", matchedOn)
}

func (i entitySelectItem) FilterValue() string {
	return i.match.EntityName
}

// entitySelectDelegate handles rendering of entity items
type entitySelectDelegate struct{}

func (d entitySelectDelegate) Height() int  { return 2 }
func (d entitySelectDelegate) Spacing() int { return 1 }
func (d entitySelectDelegate) Update(_ tea.Msg, _ *list.Model) tea.Cmd {
	return nil
}

func (d entitySelectDelegate) Render(w io.Writer, m list.Model, index int, listItem list.Item) {
	i, ok := listItem.(entitySelectItem)
	if !ok {
		return
	}

	title := i.Title()
	desc := i.Description()

	if index == m.Index() {
		// Selected item
		fmt.Fprint(w, entitySelectedItemStyle.Render("> "+title)+"\n")
		fmt.Fprint(w, entityDescStyle.Render("  "+desc))
	} else {
		// Non-selected item
		fmt.Fprint(w, entityItemStyle.Render(title)+"\n")
		fmt.Fprint(w, entityDescStyle.Render(desc))
	}
}

// entitySelectModel is the bubbletea model for entity selection
type entitySelectModel struct {
	list     list.Model
	matches  []EntityMatch
	selected *EntityMatch
	quitting bool
}

func (m entitySelectModel) Init() tea.Cmd {
	return nil
}

func (m entitySelectModel) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.WindowSizeMsg:
		m.list.SetWidth(msg.Width)
		return m, nil

	case tea.KeyMsg:
		switch keypress := msg.String(); keypress {
		case "ctrl+c", "q", "esc":
			m.quitting = true
			m.selected = nil
			return m, tea.Quit

		case "enter":
			if i, ok := m.list.SelectedItem().(entitySelectItem); ok {
				m.selected = &i.match
				m.quitting = true
			}
			return m, tea.Quit
		}
	}

	var cmd tea.Cmd
	m.list, cmd = m.list.Update(msg)
	return m, cmd
}

func (m entitySelectModel) View() string {
	if m.quitting {
		return ""
	}
	return "\n" + m.list.View()
}

// ShowEntitySelector displays interactive selection UI
// Returns selected match or nil if cancelled
func ShowEntitySelector(matches []EntityMatch) (*EntityMatch, error) {
	if len(matches) == 0 {
		return nil, fmt.Errorf("no matches to select from")
	}

	// Convert matches to list items
	items := make([]list.Item, len(matches))
	for i, match := range matches {
		items[i] = entitySelectItem{match: match}
	}

	// Calculate list dimensions
	const defaultWidth = 60
	listHeight := len(matches)*3 + 6 // 3 lines per item + header/footer
	if listHeight > 20 {
		listHeight = 20
	}

	l := list.New(items, entitySelectDelegate{}, defaultWidth, listHeight)
	l.Title = fmt.Sprintf("Select Agent or Team (%d matches)", len(matches))
	l.SetShowStatusBar(false)
	l.SetFilteringEnabled(true)
	l.Styles.Title = entityTitleStyle
	l.Styles.PaginationStyle = entityPaginationStyle
	l.Styles.HelpStyle = entityHelpStyle

	m := entitySelectModel{
		list:    l,
		matches: matches,
	}

	p := tea.NewProgram(m)
	finalModel, err := p.Run()
	if err != nil {
		return nil, fmt.Errorf("error running selector: %w", err)
	}

	if fm, ok := finalModel.(entitySelectModel); ok {
		return fm.selected, nil
	}

	return nil, fmt.Errorf("unexpected model type")
}
