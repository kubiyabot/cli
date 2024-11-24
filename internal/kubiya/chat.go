package kubiya

import (
	"context"
	"encoding/json"
	"net/http"

	"github.com/google/uuid"
)

// generateSessionID generates a random session ID
func generateSessionID() string {
	return uuid.New().String()
}

// SendMessage sends a message to a teammate
func (c *Client) SendMessage(ctx context.Context, teammateID, message string) error {
	payload := struct {
		Message   string `json:"message"`
		AgentUUID string `json:"agent_uuid"`
		SessionID string `json:"session_id"`
	}{
		Message:   message,
		AgentUUID: teammateID,
		SessionID: generateSessionID(),
	}

	resp, err := c.post(ctx, "/api/v1/converse", payload)
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	return nil
}

// ReceiveMessages implements SSE for receiving messages
func (c *Client) ReceiveMessages(ctx context.Context, teammateID string) (<-chan ChatMessage, error) {
	messagesChan := make(chan ChatMessage)

	req, err := http.NewRequestWithContext(
		ctx,
		"GET",
		c.baseURL+"/chat/stream/"+teammateID,
		nil,
	)
	if err != nil {
		return nil, err
	}

	resp, err := c.client.Do(req)
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
				if err := decoder.Decode(&msg); err != nil {
					return
				}
				messagesChan <- msg
			}
		}
	}()

	return messagesChan, nil
}
