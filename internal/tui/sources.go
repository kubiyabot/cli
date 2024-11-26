package tui

import (
	"context"
	"fmt"

	"github.com/charmbracelet/bubbles/list"
	tea "github.com/charmbracelet/bubbletea"
)

type errMsg error

// fetchSources retrieves the list of sources
func (s *SourceBrowser) fetchSources() tea.Cmd {
	return func() tea.Msg {
		sources, err := s.client.ListSources(context.Background())
		if err != nil {
			return errMsg(err)
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

		s.ready = true
		return sourcesLoadedMsg(sources)
	}
}

// fetchTools retrieves the list of tools for the current source
func (s *SourceBrowser) fetchTools() tea.Cmd {
	return func() tea.Msg {
		if s.currentSource == nil {
			return toolsLoadedMsg{err: fmt.Errorf("No source selected")}
		}

		if s.debug {
			fmt.Printf("Fetching tools for source: %s (%s)\n", s.currentSource.Name, s.currentSource.URL)
		}

		tools, err := s.client.ListTools(context.Background(), s.currentSource.URL)
		if err != nil {
			if s.debug {
				fmt.Printf("Error fetching tools: %v\n", err)
			}
			return toolsLoadedMsg{err: err}
		}

		if s.debug {
			fmt.Printf("Found %d tools\n", len(tools))
		}

		items := make([]list.Item, len(tools))
		for i, tool := range tools {
			items[i] = toolItem{tool: tool}
		}

		delegate := list.NewDefaultDelegate()
		delegate.Styles.SelectedTitle = selectedItemStyle
		delegate.Styles.SelectedDesc = selectedItemStyle

		s.toolList = list.New(items, delegate, s.width, s.height-4)
		s.toolList.Title = fmt.Sprintf("Tools - %s", s.currentSource.Name)
		s.toolList.SetShowStatusBar(false)
		s.toolList.SetFilteringEnabled(true)
		s.toolList.Styles.Title = titleStyle
		s.toolList.Styles.FilterPrompt = itemStyle
		s.toolList.Styles.FilterCursor = itemStyle

		return toolsLoadedMsg{tools: tools}
	}
}
