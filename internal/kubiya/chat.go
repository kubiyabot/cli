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
	"strings"
	"sync"
	"time"

	"github.com/google/uuid"
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

func getOrCreateSession(agentID string) *ChatSession {
	mu.Lock()
	defer mu.Unlock()

	session, exists := sessions[agentID]
	if !exists {
		sessionID := uuid.New().String()
		session = &ChatSession{
			ID:       sessionID,
			Messages: make([]ChatMessage, 0),
		}
		sessions[agentID] = session
	}
	return session
}

// SendMessage sends a message to an agent using the workflow-engine compatible streaming approach
func (c *Client) SendMessage(ctx context.Context, agentID, message string, sessionID string) (<-chan ChatMessage, error) {
	messagesChan := make(chan ChatMessage, 100)

	if message == "" {
		close(messagesChan)
		return messagesChan, nil
	}

	session := getOrCreateSession(agentID)
	if sessionID == "" {
		sessionID = session.ID
	}

	payload := struct {
		Message   string `json:"message"`
		AgentUUID string `json:"agent_uuid"`
		SessionID string `json:"session_id"`
		UserEmail string `json:"user_email,omitempty"`
		Org       string `json:"org,omitempty"`
	}{
		Message:   message,
		AgentUUID: agentID,
		SessionID: sessionID,
		UserEmail: os.Getenv("KUBIYA_USER_EMAIL"),
		Org:       os.Getenv("KUBIYA_ORG"),
	}

	// Safety check for logger
	if logger != nil {
		logger.Printf("=== Starting SendMessage ===")
		logger.Printf("Payload: %+v", payload)
	}

	reqURL := fmt.Sprintf("%s/hb/v4/stream", c.baseURL)
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
	req.Header.Set("x-vercel-ai-data-stream", "v1") // protocol flag
	req.Header.Set("Authorization", "UserKey "+c.cfg.APIKey)
	req.Header.Set("Cache-Control", "no-cache")
	req.Header.Set("Connection", "keep-alive")

	client := &http.Client{Timeout: 0} // No timeout for streaming

	resp, err := client.Do(req)
	if err != nil {
		if logger != nil {
			logger.Printf("Error executing request: %v", err)
		}
		close(messagesChan)
		// Provide more specific error messages
		if strings.Contains(err.Error(), "timeout") {
			return nil, fmt.Errorf("connection timeout - the server may be overloaded")
		} else if strings.Contains(err.Error(), "refused") {
			return nil, fmt.Errorf("connection refused - check if the API endpoint is accessible")
		} else if strings.Contains(err.Error(), "no such host") {
			return nil, fmt.Errorf("DNS resolution failed - check your network connection")
		}
		return nil, fmt.Errorf("failed to execute request: %w", err)
	}

	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		resp.Body.Close()
		close(messagesChan)
		if logger != nil {
			logger.Printf("Unexpected status code: %d, body: %s", resp.StatusCode, string(body))
		}
		
		// Provide user-friendly error messages based on status code
		switch resp.StatusCode {
		case 401:
			return nil, fmt.Errorf("authentication failed - check your API key")
		case 403:
			return nil, fmt.Errorf("access forbidden - insufficient permissions")
		case 404:
			return nil, fmt.Errorf("endpoint not found - check your configuration")
		case 429:
			return nil, fmt.Errorf("rate limit exceeded - please try again later")
		case 500:
			return nil, fmt.Errorf("server error - please try again later")
		case 502, 503, 504:
			return nil, fmt.Errorf("service temporarily unavailable - please try again later")
		default:
			return nil, fmt.Errorf("request failed with status %d: %s", resp.StatusCode, string(body))
		}
	}

	go func() {
		defer resp.Body.Close()
		defer close(messagesChan)

		scanner := bufio.NewScanner(resp.Body)
		var textBuilder strings.Builder
		lineCount := 0

		for scanner.Scan() {
			select {
			case <-ctx.Done():
				return
			default:
			}

			lineCount++
			line := scanner.Text()

			if logger != nil {
				logger.Printf("Processing line %d: %s", lineCount, line)
			}

			// Process Vercel AI data-stream protocol
			if len(line) < 3 || line[1] != ':' {
				if logger != nil {
					logger.Printf("Malformed line detected: %s", line)
				}
				continue
			}

			typ := line[0]
			payload := line[2:]

			switch typ {
			case '0': // partText
				var text string
				if err := json.Unmarshal([]byte(payload), &text); err != nil {
					if logger != nil {
						logger.Printf("Failed to unmarshal text part: %v", err)
					}
					continue
				}
				textBuilder.WriteString(text)
				
				messagesChan <- ChatMessage{
					Content:    textBuilder.String(),
					Type:       "text",
					Timestamp:  time.Now().Format(time.RFC3339),
					SenderName: "Bot",
					Final:      false,
					SessionID:  sessionID,
				}

			case '2': // partData
				var data []interface{}
				if err := json.Unmarshal([]byte(payload), &data); err != nil {
					if logger != nil {
						logger.Printf("Failed to unmarshal data part: %v", err)
					}
					continue
				}
				// Process data parts if needed

			case '3': // partError
				var errMsg string
				if err := json.Unmarshal([]byte(payload), &errMsg); err != nil {
					if logger != nil {
						logger.Printf("Failed to unmarshal error part: %v", err)
					}
					continue
				}
				messagesChan <- ChatMessage{
					Content:    errMsg,
					Type:       "error",
					Timestamp:  time.Now().Format(time.RFC3339),
					SenderName: "System",
					Final:      true,
					SessionID:  sessionID,
				}

			case 'd': // partFinishMessage
				messagesChan <- ChatMessage{
					Content:    textBuilder.String(),
					Type:       "completion",
					Timestamp:  time.Now().Format(time.RFC3339),
					SenderName: "Bot",
					Final:      true,
					SessionID:  sessionID,
				}
				return

			case 'g': // partReasoning
				var reasoning string
				if err := json.Unmarshal([]byte(payload), &reasoning); err != nil {
					if logger != nil {
						logger.Printf("Failed to unmarshal reasoning part: %v", err)
					}
					continue
				}
				// Process reasoning if needed

			case '9': // partToolCall
				var toolCall map[string]interface{}
				if err := json.Unmarshal([]byte(payload), &toolCall); err != nil {
					if logger != nil {
						logger.Printf("Failed to unmarshal tool call part: %v", err)
					}
					continue
				}
				// Display tool call
				if toolName, ok := toolCall["toolName"].(string); ok {
					var argsStr string
					if args, ok := toolCall["args"].(map[string]interface{}); ok {
						if argsBytes, err := json.Marshal(args); err == nil {
							argsStr = string(argsBytes)
						}
					}
					// Generate a unique MessageID for this tool call
					toolCallID := ""
					if id, ok := toolCall["toolCallId"].(string); ok {
						toolCallID = id
					}
					if toolCallID == "" {
						toolCallID = uuid.New().String()
					}
					messagesChan <- ChatMessage{
						Content:    fmt.Sprintf("Tool: %s\nArguments: %s", toolName, argsStr),
						Type:       "tool",
						Timestamp:  time.Now().Format(time.RFC3339),
						SenderName: "System",
						Final:      false,
						SessionID:  sessionID,
						MessageID:  toolCallID,
					}
				}

			case 'a': // partToolResult
				var toolResult map[string]interface{}
				if err := json.Unmarshal([]byte(payload), &toolResult); err != nil {
					if logger != nil {
						logger.Printf("Failed to unmarshal tool result part: %v", err)
					}
					continue
				}
				// Display tool result
				var resultStr string
				if result, ok := toolResult["result"]; ok {
					if resultBytes, err := json.Marshal(result); err == nil {
						resultStr = string(resultBytes)
					}
				}
				// Use the toolCallId to match with the corresponding tool call
				toolCallID := ""
				if id, ok := toolResult["toolCallId"].(string); ok {
					toolCallID = id
				}
				if toolCallID == "" {
					toolCallID = uuid.New().String()
				}
				messagesChan <- ChatMessage{
					Content:    resultStr,
					Type:       "tool_output",
					Timestamp:  time.Now().Format(time.RFC3339),
					SenderName: "System",
					Final:      false,
					SessionID:  sessionID,
					MessageID:  toolCallID,
				}

			default:
				if logger != nil {
					logger.Printf("Unknown stream part type: %c", typ)
				}
			}
		}

		if err := scanner.Err(); err != nil && err != io.EOF {
			if logger != nil {
				logger.Printf("Scanner error: %v", err)
			}
			messagesChan <- ChatMessage{
				Content:    fmt.Sprintf("Connection lost: %v", err),
				Timestamp:  time.Now().Format(time.RFC3339),
				SenderName: "System",
				Type:       "error",
				Final:      true,
				SessionID:  sessionID,
			}
		}
	}()

	return messagesChan, nil
}

// ReceiveMessages implements SSE for receiving messages
func (c *Client) ReceiveMessages(ctx context.Context, agentID string) (<-chan ChatMessage, error) {
	messagesChan := make(chan ChatMessage)

	go func() {
		defer close(messagesChan)

		session := getOrCreateSession(agentID)
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

func (c *Client) SendMessageWithContext(ctx context.Context, agentID, message, sessionID string, context map[string]string) (<-chan ChatMessage, error) {
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

	return c.SendMessage(ctx, agentID, contextMsg.String(), sessionID)
}

func (c *Client) GetConversationMessages(ctx context.Context, agentID, message, sessionID string) (map[string]SSEEvent, error) {
	// synchronousely get the conversation messages
	payload := struct {
		Message   string `json:"message"`
		AgentUUID string `json:"agent_uuid"`
		SessionID string `json:"session_id"`
	}{
		Message:   message,
		AgentUUID: agentID,
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

// SendInlineAgentMessage sends a message to an inline agent with custom tools
func (c *Client) SendInlineAgentMessage(ctx context.Context, message, sessionID string, context map[string]string, agentDef map[string]interface{}) error {
	messagesChan := make(chan ChatMessage, 100)
	
	// Add context to the message if provided
	var contextMsg strings.Builder
	contextMsg.WriteString(message)
	if len(context) > 0 {
		contextMsg.WriteString("\n\nHere's some reference files for context:\n")
		for filename, content := range context {
			contextMsg.WriteString("\n")
			contextMsg.WriteString(filename)
			contextMsg.WriteString(":\n")
			contextMsg.WriteString(content)
			contextMsg.WriteString("\n")
		}
	}

	// Generate unique user UUID and session ID if not provided
	if sessionID == "" {
		sessionID = uuid.New().String()
	}
	
	payload := struct {
		Message   string                 `json:"message"`
		SessionID string                 `json:"session_id"`
		UserUUID  string                 `json:"user_uuid"`
		UserEmail string                 `json:"user_email"`
		Org       string                 `json:"org"`
		Agent     map[string]interface{} `json:"agent"`
	}{
		Message:   contextMsg.String(),
		SessionID: sessionID,
		UserUUID:  uuid.New().String(),
		UserEmail: os.Getenv("KUBIYA_USER_EMAIL"),
		Org:       os.Getenv("KUBIYA_ORG"),
		Agent:     agentDef,
	}

	// Safety check for logger
	if logger != nil {
		logger.Printf("=== Starting SendInlineAgentMessage ===")
		logger.Printf("Payload: %+v", payload)
	}

	reqURL := fmt.Sprintf("%s/hb/v4/chat", c.baseURL)
	jsonData, err := json.Marshal(payload)
	if err != nil {
		if logger != nil {
			logger.Printf("Error marshalling payload: %v", err)
		}
		return fmt.Errorf("failed to marshal payload: %w", err)
	}

	req, err := http.NewRequest("POST", reqURL, bytes.NewBuffer(jsonData))
	if err != nil {
		if logger != nil {
			logger.Printf("Error creating request: %v", err)
		}
		return fmt.Errorf("failed to create request: %w", err)
	}

	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("x-vercel-ai-data-stream", "v1") // protocol flag
	req.Header.Set("Authorization", "UserKey "+c.cfg.APIKey)
	req.Header.Set("Cache-Control", "no-cache")
	req.Header.Set("Connection", "keep-alive")

	client := &http.Client{Timeout: 0}

	resp, err := client.Do(req)
	if err != nil {
		if logger != nil {
			logger.Printf("Error executing request: %v", err)
		}
		return fmt.Errorf("failed to execute request: %w", err)
	}

	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		resp.Body.Close()
		if logger != nil {
			logger.Printf("Unexpected status code: %d, body: %s", resp.StatusCode, string(body))
		}
		return fmt.Errorf("unexpected status code: %d, body: %s", resp.StatusCode, string(body))
	}

	// Handle the streaming response similar to SendMessage
	go func() {
		defer resp.Body.Close()
		defer close(messagesChan)

		scanner := bufio.NewScanner(resp.Body)
		var textBuilder strings.Builder
		lineCount := 0

		for scanner.Scan() {
			select {
			case <-ctx.Done():
				return
			default:
			}

			lineCount++
			line := scanner.Text()

			if logger != nil {
				logger.Printf("Processing line %d: %s", lineCount, line)
			}

			// Process Vercel AI data-stream protocol
			if len(line) < 3 || line[1] != ':' {
				if logger != nil {
					logger.Printf("Malformed line detected: %s", line)
				}
				continue
			}

			typ := line[0]
			payload := line[2:]

			switch typ {
			case '0': // partText
				var text string
				if err := json.Unmarshal([]byte(payload), &text); err != nil {
					if logger != nil {
						logger.Printf("Failed to unmarshal text part: %v", err)
					}
					continue
				}
				textBuilder.WriteString(text)
				
				// Print the text as it comes
				fmt.Print(text)

			case '2': // partData
				var data []interface{}
				if err := json.Unmarshal([]byte(payload), &data); err != nil {
					if logger != nil {
						logger.Printf("Failed to unmarshal data part: %v", err)
					}
					continue
				}
				// Process data parts if needed

			case '3': // partError
				var errMsg string
				if err := json.Unmarshal([]byte(payload), &errMsg); err != nil {
					if logger != nil {
						logger.Printf("Failed to unmarshal error part: %v", err)
					}
					continue
				}
				fmt.Printf("\n❌ Error: %s\n", errMsg)

			case 'd': // partFinishMessage
				fmt.Println("\n")
				return

			case 'g': // partReasoning
				var reasoning string
				if err := json.Unmarshal([]byte(payload), &reasoning); err != nil {
					if logger != nil {
						logger.Printf("Failed to unmarshal reasoning part: %v", err)
					}
					continue
				}
				// Process reasoning if needed

			case '9': // partToolCall
				var toolCall map[string]interface{}
				if err := json.Unmarshal([]byte(payload), &toolCall); err != nil {
					if logger != nil {
						logger.Printf("Failed to unmarshal tool call part: %v", err)
					}
					continue
				}
				// Display tool call
				if toolName, ok := toolCall["toolName"].(string); ok {
					var argsStr string
					if args, ok := toolCall["args"].(map[string]interface{}); ok {
						if argsBytes, err := json.Marshal(args); err == nil {
							argsStr = string(argsBytes)
						}
					}
					fmt.Printf("\n🚀 EXECUTING %s\n", toolName)
					if argsStr != "" {
						fmt.Printf("📋 Parameters: %s\n", argsStr)
					}
					fmt.Printf("⏳ Waiting for tool output...\n")
					fmt.Printf("%s\n", strings.Repeat("─", 50))
				}

			case 'a': // partToolResult
				var toolResult map[string]interface{}
				if err := json.Unmarshal([]byte(payload), &toolResult); err != nil {
					if logger != nil {
						logger.Printf("Failed to unmarshal tool result part: %v", err)
					}
					continue
				}
				// Display tool result
				var resultStr string
				if result, ok := toolResult["result"]; ok {
					if resultBytes, err := json.Marshal(result); err == nil {
						resultStr = string(resultBytes)
					}
				}
				fmt.Printf("│ %s\n", resultStr)

			default:
				if logger != nil {
					logger.Printf("Unknown stream part type: %c", typ)
				}
			}
		}

		if err := scanner.Err(); err != nil && err != io.EOF {
			if logger != nil {
				logger.Printf("Scanner error: %v", err)
			}
			fmt.Fprintf(os.Stderr, "\n❌ Connection lost: %v\n", err)
		}
	}()

	// Wait for the streaming to complete
	for range messagesChan {
		// Just consume the messages
	}

	return nil
}
