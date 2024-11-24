package kubiya

import (
	"context"
	"encoding/json"
	"fmt"
	"strconv"
	"sync"
	"time"
)

// ChatSession maintains chat session state
type ChatSession struct {
	ID       string
	Messages []ChatMessage
}

// ChatMessage represents a message in the chat
type ChatMessage struct {
	Content    string `json:"content"`
	Error      string `json:"error,omitempty"`
	SessionID  string `json:"session_id,omitempty"`
	Timestamp  string `json:"timestamp"`
	SenderName string `json:"sender_name"`
	Type       string `json:"type,omitempty"`
	Message    string `json:"message,omitempty"`
}

var (
	sessions = make(map[string]*ChatSession)
	mu       sync.RWMutex
)

// getOrCreateSession retrieves existing session or creates new one
func getOrCreateSession(teammateID string) *ChatSession {
	mu.Lock()
	defer mu.Unlock()

	// Try to find existing session for this teammate
	for _, session := range sessions {
		if session.Messages != nil && len(session.Messages) > 0 {
			if msg := session.Messages[0]; msg.SessionID != "" {
				return session
			}
		}
	}

	// Create new session if none exists
	sessionID := strconv.FormatInt(time.Now().UnixNano(), 10)
	session := &ChatSession{
		ID:       sessionID,
		Messages: make([]ChatMessage, 0),
	}
	sessions[teammateID] = session
	return session
}

// SendMessage sends a message to a teammate
func (c *Client) SendMessage(ctx context.Context, teammateID, message string, sessionID string) (<-chan ChatMessage, error) {
	messagesChan := make(chan ChatMessage)

	// Don't send empty messages
	if message == "" {
		close(messagesChan)
		return messagesChan, nil
	}

	// Get or create session
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

	resp, err := c.post(ctx, "converse", payload)
	if err != nil {
		close(messagesChan)
		return nil, err
	}

	go func() {
		defer resp.Body.Close()
		defer close(messagesChan)

		decoder := json.NewDecoder(resp.Body)

		for {
			select {
			case <-ctx.Done():
				return
			default:
				var msg ChatMessage
				err := decoder.Decode(&msg)
				if err != nil {
					if err.Error() != "EOF" {
						messagesChan <- ChatMessage{Error: fmt.Sprintf("failed to decode message: %v", err)}
					}
					return
				}

				// Handle message format
				if msg.Content == "" && msg.Message != "" {
					msg.Content = msg.Message
				}
				if msg.SenderName == "" {
					msg.SenderName = "Bot"
				}
				if msg.Timestamp == "" {
					msg.Timestamp = time.Now().Format(time.RFC3339)
				}
				if msg.SessionID == "" {
					msg.SessionID = sessionID
				}

				// Add message to session history
				mu.Lock()
				session.Messages = append(session.Messages, msg)
				mu.Unlock()

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
		
		// Get existing session if any
		session := getOrCreateSession(teammateID)
		
		// Send existing messages
		mu.RLock()
		for _, msg := range session.Messages {
			select {
			case <-ctx.Done():
				return
			case messagesChan <- msg:
			}
		}
		mu.RUnlock()
	}()

	return messagesChan, nil
}
