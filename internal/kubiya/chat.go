// kubiya/chat.go

package kubiya

import (
	"bufio"
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net/http"
	"os"
	"strconv"
	"strings"
	"sync"
	"time"
)

var logger *log.Logger

func init() {
	file, err := os.OpenFile("kubiya_chat.log", os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0666)
	if err != nil {
		// If we can't open the log file, fall back to stderr
		logger = log.New(os.Stderr, "KUBIYA: ", log.LstdFlags|log.Lshortfile)
		logger.Printf("Warning: Could not open log file: %v, logging to stderr", err)
		return
	}
	logger = log.New(file, "", log.LstdFlags|log.Lshortfile)
}

// ChatSession maintains chat session state
type ChatSession struct {
	ID       string
	Messages []ChatMessage
}

// Config holds the client configuration
type Config struct {
	APIKey string
	URL    string
}

var (
	sessions = make(map[string]*ChatSession)
	mu       sync.RWMutex
)

// SSEEvent represents a Server-Sent Event
type SSEEvent struct {
	Message   string `json:"message"`
	ID        string `json:"id"`
	Type      string `json:"type"`
	Done      bool   `json:"done,omitempty"`
	Status    string `json:"status,omitempty"`
	SessionID string `json:"session_id,omitempty"`
}

func getOrCreateSession(teammateID string) *ChatSession {
	mu.Lock()
	defer mu.Unlock()

	session, exists := sessions[teammateID]
	if !exists {
		sessionID := strconv.FormatInt(time.Now().UnixNano(), 10)
		session = &ChatSession{
			ID:       sessionID,
			Messages: make([]ChatMessage, 0),
		}
		sessions[teammateID] = session
	}
	return session
}

// SendMessage sends a message to a teammate and handles SSE responses
func (c *Client) SendMessage(ctx context.Context, teammateID, message string, sessionID string) (<-chan ChatMessage, error) {
	messagesChan := make(chan ChatMessage, 100)

	if message == "" {
		close(messagesChan)
		return messagesChan, nil
	}

	session := getOrCreateSession(teammateID)
	if sessionID == "" {
		sessionID = session.ID
	}

	payload := struct {
		Message   string `json:"message"`
		AgentUUID string `json:"agent_uuid"`
		SessionID string `json:"session_id"`
	}{
		Message:   message,
		AgentUUID: teammateID,
		SessionID: sessionID,
	}

	// Safety check for logger
	if logger != nil {
		logger.Printf("=== Starting SendMessage ===")
		logger.Printf("Payload: %+v", payload)
	}

	reqURL := fmt.Sprintf("%s/converse", c.baseURL)
	jsonData, err := json.Marshal(payload)
	if err != nil {
		if logger != nil {
			logger.Printf("Error marshalling payload: %v", err)
		}
		close(messagesChan)
		return nil, fmt.Errorf("failed to marshal payload: %w", err)
	}

	req, err := http.NewRequest("POST", reqURL, bytes.NewBuffer(jsonData))
	if err != nil {
		if logger != nil {
			logger.Printf("Error creating request: %v", err)
		}
		close(messagesChan)
		return nil, fmt.Errorf("failed to create request: %w", err)
	}

	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Accept", "text/event-stream")
	req.Header.Set("Authorization", "UserKey "+c.cfg.APIKey)
	req.Header.Set("Cache-Control", "no-cache")
	req.Header.Set("Connection", "keep-alive")

	client := &http.Client{Timeout: 0}

	resp, err := client.Do(req)
	if err != nil {
		if logger != nil {
			logger.Printf("Error executing request: %v", err)
		}
		close(messagesChan)
		return nil, fmt.Errorf("failed to execute request: %w", err)
	}

	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		resp.Body.Close()
		close(messagesChan)
		if logger != nil {
			logger.Printf("Unexpected status code: %d, body: %s", resp.StatusCode, string(body))
		}
		return nil, fmt.Errorf("unexpected status code: %d, body: %s", resp.StatusCode, string(body))
	}

	go func() {
		defer resp.Body.Close()
		defer close(messagesChan)

		reader := bufio.NewReader(resp.Body)
		var lastMessage string

		for {
			select {
			case <-ctx.Done():
				return
			default:
				line, err := reader.ReadString('\n')
				if err != nil {
					if err != io.EOF {
						if logger != nil {
							logger.Printf("Error reading line: %v", err)
						}
						messagesChan <- ChatMessage{
							Content:    fmt.Sprintf("Stream error: %v", err),
							Timestamp:  time.Now().Format(time.RFC3339),
							SenderName: "System",
							Type:       "error",
							Final:      true,
						}
					}
					return
				}

				line = strings.TrimSpace(line)
				if line == "" {
					continue
				}

				var event struct {
					Message   string `json:"message"`
					ID        string `json:"id"`
					Type      string `json:"type"`
					Done      bool   `json:"done,omitempty"`
					Status    string `json:"status,omitempty"`
					SessionID string `json:"session_id,omitempty"`
				}
				if err := json.Unmarshal([]byte(line), &event); err != nil {
					if logger != nil {
						logger.Printf("JSON unmarshal error: %v for line: %s", err, line)
					}
					continue
				}

				// Skip empty messages
				if event.Message == "" {
					continue
				}

				isFinal := event.Message == lastMessage ||
					strings.HasSuffix(event.Message, ".") ||
					strings.HasSuffix(event.Message, "?") ||
					strings.HasSuffix(event.Message, "!")

				msg := ChatMessage{
					Content:    event.Message,
					Type:       event.Type,
					MessageID:  event.ID,
					Timestamp:  time.Now().Format(time.RFC3339),
					SenderName: "Bot",
					Final:      isFinal,
					SessionID:  event.SessionID,
				}

				lastMessage = event.Message
				if logger != nil {
					logger.Printf("Sending message: %+v", msg)
				}
				messagesChan <- msg
			}
		}
	}()

	return messagesChan, nil
}

// ReceiveMessages implements SSE for receiving messages
func (c *Client) ReceiveMessages(ctx context.Context, teammateID string) (<-chan ChatMessage, error) {
	messagesChan := make(chan ChatMessage)

	go func() {
		defer close(messagesChan)

		session := getOrCreateSession(teammateID)
		mu.RLock()
		messages := make([]ChatMessage, len(session.Messages))
		copy(messages, session.Messages)
		mu.RUnlock()

		for _, msg := range messages {
			select {
			case <-ctx.Done():
				return
			case messagesChan <- msg:
			}
		}
	}()

	return messagesChan, nil
}

func (c *Client) SendMessageWithContext(ctx context.Context, teammateID, message, sessionID string, context map[string]string) (<-chan ChatMessage, error) {
	var contextMsg strings.Builder
	contextMsg.WriteString(message)
	contextMsg.WriteString("\n\nHere's some reference files for context:\n")

	for filename, content := range context {
		contextMsg.WriteString("\n")
		contextMsg.WriteString(filename)
		contextMsg.WriteString(":\n")
		contextMsg.WriteString(content)
		contextMsg.WriteString("\n")
	}

	return c.SendMessage(ctx, teammateID, contextMsg.String(), sessionID)
}

func (c *Client) GetConversationMessages(ctx context.Context, teammateID, message, sessionID string) (map[string]SSEEvent, error) {
	// synchronousely get the conversation messages
	payload := struct {
		Message   string `json:"message"`
		AgentUUID string `json:"agent_uuid"`
		SessionID string `json:"session_id"`
	}{
		Message:   message,
		AgentUUID: teammateID,
		SessionID: sessionID,
	}
	var body bytes.Buffer
	if err := json.NewEncoder(&body).Encode(payload); err != nil {
		return nil, fmt.Errorf("failed to encode request payload: %w", err)
	}
	req, err := http.NewRequestWithContext(ctx, "POST", fmt.Sprintf("%s/converse", c.baseURL), &body)
	if err != nil {
		return nil, err
	}
	req.Header.Set("Authorization", "UserKey "+c.cfg.APIKey)
	lines, err := c.doRaw(req)
	if err != nil {
		return nil, err
	}
	ret := make(map[string]SSEEvent, 0)

	for _, line := range strings.Split(string(lines), "\n") {
		// iterate over the lines and only keep the last event of each message id (compensate over streaming)
		if line == "" {
			continue
		}
		var ev SSEEvent
		err = json.Unmarshal([]byte(line), &ev)
		if err != nil {
			continue
		}
		ret[ev.ID] = ev
	}
	return ret, nil
}
