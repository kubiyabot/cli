package tui

import (
	"fmt"

	"github.com/charmbracelet/bubbles/list"
	"github.com/charmbracelet/bubbles/spinner"
	"github.com/charmbracelet/bubbles/textinput"
	"github.com/charmbracelet/bubbles/viewport"
	"github.com/kubiyabot/cli/internal/config"
	"github.com/kubiyabot/cli/internal/kubiya"
)

// New creates a new TUI application
func New(cfg *config.Config) *SourceBrowser {
	return NewSourceBrowser(cfg)
}

// ChatApp represents the chat TUI application
type ChatApp struct {
	client *kubiya.Client
	model  ChatModel
}

// ChatModel represents the chat TUI state
type ChatModel struct {
	agents          list.Model
	spinner         spinner.Model
	input           textinput.Model
	viewport        viewport.Model
	sessions        []ChatSession
	activeSession   *ChatSession
	err             error
	loading         bool
	chatting        bool
	width, height   int
	showingSessions bool
	statusMsg       string
	client          *kubiya.Client
}

// ChatSession represents an active chat session
type ChatSession struct {
	Agent     ChatAgent
	Messages  []string
	SessionID string
}

// ChatAgent represents a Kubiya teammate in chat
type ChatAgent struct {
	UUID           string
	Name           string
	Desc           string
	AIInstructions string
}

// Implement list.Item interface for ChatAgent
func (a ChatAgent) Title() string {
	status := "🟢"
	if a.AIInstructions != "" {
		status = "🌟"
	}
	return fmt.Sprintf("%s %s", status, a.Name)
}

func (a ChatAgent) Description() string { return a.Desc }
func (a ChatAgent) FilterValue() string { return a.Name }

type chatTeammatesMsg []list.Item
type chatErrMsg struct{ error }

// Rest of the chat-related code...
