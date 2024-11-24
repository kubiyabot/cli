package tui

import (
	"context"
	"fmt"

	"github.com/charmbracelet/bubbles/help"
	"github.com/charmbracelet/bubbles/list"
	"github.com/charmbracelet/bubbles/paginator"
	"github.com/charmbracelet/bubbles/progress"
	"github.com/charmbracelet/bubbles/spinner"
	"github.com/charmbracelet/bubbles/textinput"
	"github.com/charmbracelet/bubbles/viewport"
	"github.com/kubiyabot/cli/internal/config"
	"github.com/kubiyabot/cli/internal/kubiya"
	"github.com/kubiyabot/cli/internal/portforward"
)

// BrowserState represents the state of the source browser
type BrowserState int

const (
	stateSourceList BrowserState = iota
	stateToolList
	stateToolDetail
	stateExecuteTool
	stateContextSelect
	stateEnvVarSelect
	stateEnvVarOptions
	stateEnvVarValueInput
	stateFileSelect
	stateExecuteConfirm
	stateExecuting
)

// Messages for the source browser
type (
	sourcesLoadedMsg []kubiya.Source
	toolsLoadedMsg   struct {
		tools []kubiya.Tool
		err   error
	}
	portForwardMsg struct {
		ready bool
		err   error
	}
	toolExecutedMsg struct {
		output string
		err    error
	}
	contextListMsg []list.Item
)

// List items for the source browser
type (
	sourceItem struct {
		source kubiya.Source
	}
	toolItem struct {
		tool kubiya.Tool
	}
	contextItem struct {
		name    string
		current bool
	}
)

// Implement list.Item interface
func (i sourceItem) Title() string       { return i.source.Name }
func (i sourceItem) Description() string { return i.source.Description }
func (i sourceItem) FilterValue() string { return i.source.Name }

func (i toolItem) Title() string       { return i.tool.Name }
func (i toolItem) Description() string { return i.tool.Description }
func (i toolItem) FilterValue() string { return i.tool.Name }

func (i contextItem) Title() string {
	if i.current {
		return fmt.Sprintf("✓ %s (current)", i.name)
	}
	return i.name
}
func (i contextItem) Description() string { return "" }
func (i contextItem) FilterValue() string { return i.name }

// SourceBrowser represents the main browser component
type SourceBrowser struct {
	cfg           *config.Config
	client        *kubiya.Client
	state         BrowserState
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
		forwarder *portforward.PortForwarder
		ready     bool
		cancel    context.CancelFunc
	}
	kubeContexts []string
	contextList  list.Model
	keys         keyMap
	help         help.Model
	progress     progress.Model
	paginator    paginator.Model
	showHelp     bool
	ready        bool
}

// executionState represents the state of tool execution
type executionState struct {
	args              map[string]string
	envVars           map[string]*kubiya.EnvVarStatus
	envVarNames       []string
	context           string
	teammate          string
	executing         bool
	output            string
	error             error
	files             map[string]string
	activeInput       int
	confirmed         bool
	prepared          bool
	currentEnvVarName string
	envVarOptions     []envVarOption
	textInput         textinput.Model
	activeOption      int
	secrets           map[string]string
}

// envVarOption represents an environment variable option
type envVarOption struct {
	icon   string
	label  string
	value  string
	action func() error
}
