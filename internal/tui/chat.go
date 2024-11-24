package tui

import (
	"bufio"
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"

	"github.com/gdamore/tcell/v2"
	"github.com/google/uuid"
	"github.com/kubiyabot/cli/internal/config"
	"github.com/kubiyabot/cli/internal/kubiya"
	"github.com/rivo/tview"
)

type ChatTUI struct {
	app          *tview.Application
	cfg          *config.Config
	client       *kubiya.Client
	teammate     *kubiya.Teammate
	teammates    []kubiya.Teammate
	messages     []chatMessage
	mainFlex     *tview.Flex
	chatFlex     *tview.Flex
	teammateList *tview.List
	chatView     *tview.TextView
	inputField   *tview.InputField
	statusBar    *tview.TextView
	loading      bool
	sessionID    string
	sseClient    *http.Client
	sseCancel    context.CancelFunc
	errChan      chan error
	debug        bool
}

type chatMessage struct {
	content   string
	fromUser  bool
	timestamp time.Time
}

// NewChatTUI creates a new chat TUI instance
func NewChatTUI(cfg *config.Config) *ChatTUI {
	c := &ChatTUI{
		app:       tview.NewApplication(),
		cfg:       cfg,
		client:    kubiya.NewClient(cfg),
		messages:  make([]chatMessage, 0),
		sessionID: uuid.New().String(),
		errChan:   make(chan error, 1),
		sseClient: &http.Client{},
	}

	// Create teammate list with keyboard navigation
	c.teammateList = tview.NewList()
	c.teammateList.ShowSecondaryText(false)
	c.teammateList.SetBorder(true)
	c.teammateList.SetTitle(" 👥 Teammates ")
	c.teammateList.SetTitleAlign(tview.AlignLeft)
	c.teammateList.SetSelectedBackgroundColor(tcell.ColorDarkBlue)
	c.teammateList.SetSelectedTextColor(tcell.ColorWhite)

	// Add keyboard shortcuts
	c.teammateList.SetInputCapture(func(event *tcell.EventKey) *tcell.EventKey {
		switch event.Key() {
		case tcell.KeyUp:
			current := c.teammateList.GetCurrentItem()
			if current > 0 {
				c.teammateList.SetCurrentItem(current - 1)
			}
			return nil
		case tcell.KeyDown:
			current := c.teammateList.GetCurrentItem()
			if current < c.teammateList.GetItemCount()-1 {
				c.teammateList.SetCurrentItem(current + 1)
			}
			return nil
		case tcell.KeyEnter:
			c.selectTeammate(c.teammateList.GetCurrentItem())
			return nil
		}
		return event
	})

	// Create chat view with scrolling
	c.chatView = tview.NewTextView()
	c.chatView.SetDynamicColors(true)
	c.chatView.SetScrollable(true)
	c.chatView.SetWordWrap(true)
	c.chatView.SetBorder(true)
	c.chatView.SetTitle(" 💬 Chat ")
	c.chatView.SetTitleAlign(tview.AlignLeft)

	// Add keyboard shortcuts for scrolling
	c.chatView.SetInputCapture(func(event *tcell.EventKey) *tcell.EventKey {
		switch event.Key() {
		case tcell.KeyPgUp:
			row, _ := c.chatView.GetScrollOffset()
			c.chatView.ScrollTo(row-1, 0)
		case tcell.KeyPgDn:
			row, _ := c.chatView.GetScrollOffset()
			c.chatView.ScrollTo(row+1, 0)
		}
		return event
	})

	// Create input field with better handling
	input := tview.NewInputField()
	input.SetLabel(" Message: ")
	input.SetFieldWidth(0)
	input.SetBorder(true)

	// Add keyboard shortcuts with better focus handling
	input.SetInputCapture(func(event *tcell.EventKey) *tcell.EventKey {
		switch event.Key() {
		case tcell.KeyEsc:
			c.app.SetFocus(c.teammateList)
			return nil
		case tcell.KeyEnter:
			message := input.GetText()
			if message != "" {
				c.sendMessage(message)
				input.SetText("")
			}
			return nil
		}
		return event
	})

	c.inputField = input

	// Create status bar
	c.statusBar = tview.NewTextView()
	c.statusBar.SetDynamicColors(true)
	c.statusBar.SetTextAlign(tview.AlignCenter)

	// Create layouts
	c.chatFlex = tview.NewFlex()
	c.chatFlex.SetDirection(tview.FlexRow)
	c.chatFlex.AddItem(c.chatView, 0, 1, false)
	c.chatFlex.AddItem(c.inputField, 3, 0, true)
	c.chatFlex.AddItem(c.statusBar, 1, 0, false)

	c.mainFlex = tview.NewFlex()
	c.mainFlex.AddItem(c.teammateList, 30, 0, true)
	c.mainFlex.AddItem(c.chatFlex, 0, 1, false)

	// Start error handler
	go c.handleErrors()

	return c
}

func (c *ChatTUI) selectTeammate(index int) {
	if c.cfg.Debug {
		fmt.Printf("Selecting teammate at index %d\n", index)
	}

	if index < 0 || index >= len(c.teammates) {
		if c.cfg.Debug {
			fmt.Printf("Invalid teammate index: %d (total teammates: %d)\n", index, len(c.teammates))
		}
		return
	}

	c.teammate = &c.teammates[index]
	if c.cfg.Debug {
		fmt.Printf("Selected teammate: %s (UUID: %s)\n", c.teammate.Name, c.teammate.UUID)
	}

	c.chatView.SetTitle(fmt.Sprintf(" 💬 Chat with %s ", c.teammate.Name))
	c.chatView.Clear()
	c.messages = nil
	c.sessionID = uuid.New().String() // New session for new teammate

	// Set focus to input field and clear any previous text
	c.inputField.SetText("")
	c.app.SetFocus(c.inputField)

	// Start SSE connection
	c.startSSE()
}

func (c *ChatTUI) startSSE() {
	// Cancel existing SSE connection if any
	if c.sseCancel != nil {
		c.sseCancel()
	}

	ctx, cancel := context.WithCancel(context.Background())
	c.sseCancel = cancel

	go func() {
		// Create initial connection message
		payload := struct {
			Message   string `json:"message"`
			AgentUUID string `json:"agent_uuid"`
			SessionID string `json:"session_id"`
		}{
			Message:   "Hello!",
			AgentUUID: c.teammate.UUID,
			SessionID: c.sessionID,
		}

		jsonData, err := json.Marshal(payload)
		if err != nil {
			c.errChan <- err
			return
		}

		req, err := http.NewRequestWithContext(ctx, "POST", fmt.Sprintf("%s/converse", c.cfg.BaseURL), bytes.NewBuffer(jsonData))
		if err != nil {
			c.errChan <- err
			return
		}

		req.Header.Set("Authorization", fmt.Sprintf("UserKey %s", c.cfg.APIKey))
		req.Header.Set("Content-Type", "application/json")
		req.Header.Set("Accept", "text/event-stream")

		resp, err := c.sseClient.Do(req)
		if err != nil {
			c.errChan <- err
			return
		}
		defer resp.Body.Close()

		reader := bufio.NewReader(resp.Body)
		for {
			select {
			case <-ctx.Done():
				return
			default:
				line, err := reader.ReadString('\n')
				if err != nil {
					if err != io.EOF {
						c.errChan <- err
					}
					return
				}

				if strings.HasPrefix(line, "data: ") {
					data := strings.TrimPrefix(line, "data: ")
					var response struct {
						Content string `json:"content"`
						Error   string `json:"error"`
					}
					if err := json.Unmarshal([]byte(data), &response); err != nil {
						c.errChan <- err
						continue
					}

					if response.Error != "" {
						c.errChan <- fmt.Errorf(response.Error)
						continue
					}

					if response.Content != "" {
						c.app.QueueUpdateDraw(func() {
							c.addMessage(response.Content, false)
						})
					}
				}
			}
		}
	}()
}

func (c *ChatTUI) sendMessage(message string) {
	if message == "" || c.teammate == nil {
		return
	}

	// Add user message immediately
	c.addMessage(message, true)
	c.setTypingIndicator(true)

	// Clear input field immediately
	c.inputField.SetText("")
	c.app.SetFocus(c.inputField)

	// Send message in goroutine
	go func() {
		// Prepare request
		payload := struct {
			Message   string `json:"message"`
			AgentUUID string `json:"agent_uuid"`
			SessionID string `json:"session_id"`
		}{
			Message:   message,
			AgentUUID: c.teammate.UUID,
			SessionID: c.sessionID,
		}

		if c.debug {
			fmt.Printf("Sending message: %+v\n", payload)
		}

		jsonData, err := json.Marshal(payload)
		if err != nil {
			c.app.QueueUpdateDraw(func() {
				c.showError(fmt.Errorf("Failed to prepare message: %v", err))
				c.setTypingIndicator(false)
			})
			return
		}

		// Create request with proper headers for SSE
		req, err := http.NewRequest("POST", fmt.Sprintf("%s/converse", c.cfg.BaseURL), bytes.NewBuffer(jsonData))
		if err != nil {
			c.app.QueueUpdateDraw(func() {
				c.showError(fmt.Errorf("Failed to create request: %v", err))
				c.setTypingIndicator(false)
			})
			return
		}

		req.Header.Set("Authorization", fmt.Sprintf("UserKey %s", c.cfg.APIKey))
		req.Header.Set("Content-Type", "application/json")
		req.Header.Set("Accept", "text/event-stream")
		req.Header.Set("Cache-Control", "no-cache")
		req.Header.Set("Connection", "keep-alive")

		if c.debug {
			fmt.Printf("Request URL: %s\n", req.URL)
			fmt.Printf("Headers: %+v\n", req.Header)
		}

		// Use a client that doesn't timeout for SSE
		client := &http.Client{
			Transport: &http.Transport{
				MaxIdleConnsPerHost: 1,
			},
		}

		resp, err := client.Do(req)
		if err != nil {
			c.app.QueueUpdateDraw(func() {
				c.showError(fmt.Errorf("Network error: %v\nPlease check your internet connection", err))
				c.setTypingIndicator(false)
			})
			return
		}
		defer resp.Body.Close()

		if resp.StatusCode != http.StatusOK {
			body, _ := io.ReadAll(resp.Body)
			c.app.QueueUpdateDraw(func() {
				c.showError(fmt.Errorf("Server error (HTTP %d):\n%s", resp.StatusCode, string(body)))
				c.setTypingIndicator(false)
			})
			return
		}

		// Handle SSE stream
		reader := bufio.NewReader(resp.Body)
		var messageBuilder strings.Builder
		var lastUpdate time.Time

		for {
			line, err := reader.ReadString('\n')
			if err != nil {
				if err != io.EOF {
					c.app.QueueUpdateDraw(func() {
						c.showError(fmt.Errorf("Failed to read response: %v", err))
						c.setTypingIndicator(false)
					})
				}
				return
			}

			if c.debug {
				fmt.Printf("Received SSE line: %s", line)
			}

			// Handle SSE data
			if strings.HasPrefix(line, "data: ") {
				data := strings.TrimPrefix(line, "data: ")
				var response struct {
					Content string `json:"content"`
					Error   string `json:"error"`
				}

				if err := json.Unmarshal([]byte(data), &response); err != nil {
					if c.debug {
						fmt.Printf("Failed to parse SSE data: %v\n", err)
					}
					continue
				}

				if response.Error != "" {
					c.app.QueueUpdateDraw(func() {
						c.showError(fmt.Errorf("Server error: %s", response.Error))
						c.setTypingIndicator(false)
					})
					return
				}

				if response.Content != "" {
					messageBuilder.WriteString(response.Content)
					
					// Update UI at most every 100ms to prevent freezing
					if time.Since(lastUpdate) > 100*time.Millisecond {
						content := messageBuilder.String()
						c.app.QueueUpdateDraw(func() {
							c.setTypingIndicator(false)
							c.addMessage(content, false)
						})
						lastUpdate = time.Now()
					}
				}
			}
		}
	}()
}

func (c *ChatTUI) addErrorMessage(errMsg string) {
	if c.cfg.Debug {
		fmt.Printf("Error occurred: %s\n", errMsg)
	}

	c.app.QueueUpdateDraw(func() {
		// Add error to chat with better formatting
		fmt.Fprintf(c.chatView, "\n[red]❌ Error[white]\n")
		fmt.Fprintf(c.chatView, "[red]%s[white]\n", errMsg)
		fmt.Fprintf(c.chatView, "[gray]%s[white]\n", time.Now().Format("15:04:05"))
		fmt.Fprintf(c.chatView, "[gray]%s[white]\n", strings.Repeat("─", 50))

		// Update status bar
		c.statusBar.SetText("[red]Error occurred - see above for details[white]")

		// Make sure error is visible
		c.chatView.ScrollToEnd()

		// Clear loading state
		c.setLoading(false)
	})
}

func (c *ChatTUI) addMessage(content string, fromUser bool) {
	msg := chatMessage{
		content:   content,
		fromUser:  fromUser,
		timestamp: time.Now(),
	}
	c.messages = append(c.messages, msg)

	// Clear view and rerender all messages
	c.chatView.Clear()
	for _, m := range c.messages {
		c.renderMessage(m)
	}
	c.chatView.ScrollToEnd()
}

func (c *ChatTUI) setLoading(loading bool) {
	c.loading = loading
	c.app.QueueUpdateDraw(func() {
		if loading {
			c.statusBar.SetText(fmt.Sprintf("[yellow]💭 %s is thinking...[white]", c.teammate.Name))
		} else {
			c.statusBar.SetText("")
		}
	})
}

func (c *ChatTUI) showError(err error) {
	if c.debug {
		fmt.Printf("Error occurred: %v\n", err)
	}

	c.app.QueueUpdateDraw(func() {
		// Show error in status bar
		c.statusBar.SetText(fmt.Sprintf("[red]Error: %v[white]", err))

		// Add error message to chat
		fmt.Fprintf(c.chatView, "\n[red]❌ Error occurred:[white]\n")
		fmt.Fprintf(c.chatView, "[red]%v[white]\n", err)
		fmt.Fprintf(c.chatView, "[gray]%s[white]\n\n", time.Now().Format("15:04:05"))

		// Clear typing indicator
		c.setTypingIndicator(false)
		c.chatView.ScrollToEnd()
	})
}

func (c *ChatTUI) clearError() {
	c.app.QueueUpdateDraw(func() {
		c.statusBar.SetText("")
	})
}

func (c *ChatTUI) loadTeammates() error {
	if c.cfg.Debug {
		fmt.Printf("Loading teammates...\n")
	}

	teammates, err := c.client.ListTeammates(context.Background())
	if err != nil {
		return fmt.Errorf("Failed to list teammates: %w", err)
	}

	if c.cfg.Debug {
		fmt.Printf("Found %d teammates\n", len(teammates))
	}

	c.teammates = teammates
	for _, teammate := range teammates {
		status := "🟢"
		if teammate.AIInstructions != "" {
			status = "🌟"
		}
		name := fmt.Sprintf("%s %s", status, teammate.Name)
		if c.cfg.Debug {
			fmt.Printf("Adding teammate: %s\n", name)
		}
		c.teammateList.AddItem(name, teammate.Desc, rune(0), nil)
	}

	return nil
}

func (c *ChatTUI) handleErrors() {
	for err := range c.errChan {
		if c.cfg.Debug {
			fmt.Printf("Handling error: %v\n", err)
		}
		c.addErrorMessage(err.Error())
		c.setLoading(false) // Make sure to stop loading state
	}
}

// Run starts the chat TUI
func (c *ChatTUI) Run() error {
	// Load teammates first
	if err := c.loadTeammates(); err != nil {
		return fmt.Errorf("Failed to load teammates: %w", err)
	}

	// Set input field placeholder
	c.inputField.SetPlaceholder("Type your message and press Enter")

	// Debug logging
	if c.cfg.Debug {
		fmt.Printf("Loaded %d teammates\n", len(c.teammates))
		for _, t := range c.teammates {
			fmt.Printf("- %s (UUID: %s)\n", t.Name, t.UUID)
		}
	}

	// Run application with error handling
	if err := c.app.SetRoot(c.mainFlex, true).EnableMouse(true).Run(); err != nil {
		return fmt.Errorf("Application error: %w", err)
	}

	return nil
}

func (c *ChatTUI) renderMessage(msg chatMessage) {
	var prefix string
	if msg.fromUser {
		prefix = "[yellow]You[white]"
	} else {
		prefix = fmt.Sprintf("[green]%s[white]", c.teammate.Name)
	}

	fmt.Fprintf(c.chatView, "\n%s: %s\n[gray]%s[white]\n%s\n",
		prefix,
		msg.content,
		msg.timestamp.Format("15:04:05"),
		strings.Repeat("─", 50))
}

func (c *ChatTUI) setTypingIndicator(typing bool) {
	c.app.QueueUpdateDraw(func() {
		if typing {
			c.statusBar.SetText(fmt.Sprintf("[yellow]💭 %s is typing...[white]", c.teammate.Name))
			fmt.Fprintf(c.chatView, "\n[yellow]%s is typing...[white]", c.teammate.Name)
		} else {
			c.statusBar.SetText("")
			c.renderMessages() // Re-render to remove typing indicator
		}
		c.chatView.ScrollToEnd()
	})
}

func (c *ChatTUI) renderMessages() {
	c.chatView.Clear()
	fmt.Fprintf(c.chatView, "[::b]Chat with %s[white]\n\n", c.teammate.Name)

	for i, msg := range c.messages {
		// Add timestamp header for new days
		if i == 0 || !isSameDay(msg.timestamp, c.messages[i-1].timestamp) {
			fmt.Fprintf(c.chatView, "[::d]%s[white]\n", formatDateHeader(msg.timestamp))
		}

		// Format message with proper styling
		if msg.fromUser {
			fmt.Fprintf(c.chatView, "[yellow]You[white] [gray]%s[white]\n", msg.timestamp.Format("15:04"))
			fmt.Fprintf(c.chatView, "%s\n\n", msg.content)
		} else {
			fmt.Fprintf(c.chatView, "[green]%s[white] [gray]%s[white]\n", c.teammate.Name, msg.timestamp.Format("15:04"))
			fmt.Fprintf(c.chatView, "%s\n\n", msg.content)
		}
	}
}

func isSameDay(t1, t2 time.Time) bool {
	y1, m1, d1 := t1.Date()
	y2, m2, d2 := t2.Date()
	return y1 == y2 && m1 == m2 && d1 == d2
}

func formatDateHeader(t time.Time) string {
	if isToday(t) {
		return "Today"
	}
	if isYesterday(t) {
		return "Yesterday"
	}
	return t.Format("Monday, January 2")
}

func isToday(t time.Time) bool {
	now := time.Now()
	return isSameDay(now, t)
}

func isYesterday(t time.Time) bool {
	yesterday := time.Now().AddDate(0, 0, -1)
	return isSameDay(yesterday, t)
}
