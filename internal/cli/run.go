package cli

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"strings"

	"github.com/kubiyabot/cli/internal/config"
	"github.com/kubiyabot/cli/internal/kubiya"
	"github.com/kubiyabot/cli/internal/style"
	"github.com/spf13/cobra"
)

func newRunCommand(cfg *config.Config) *cobra.Command {
	var (
		agentID   string
		agentName string
		argValues    []string
		clearSession bool
		sessionID    string
		noClassify   bool
		debug        bool
	)

	cmd := &cobra.Command{
		Use:   "run [tool-name]",
		Short: "🚀 Run a tool using the converse API",
		RunE: func(cmd *cobra.Command, args []string) error {
			if len(args) == 0 {
				return fmt.Errorf("tool name is required")
			}
			toolName := args[0]

			// Setup client
			client := kubiya.NewClient(cfg)

			// Auto-classify by default unless agent is explicitly specified or --no-classify is set
			shouldClassify := agentID == "" && agentName == "" && !noClassify

			// If auto-classify is enabled (default), use the classification endpoint
			if shouldClassify {
				// Format the prompt for classification
				var prompt strings.Builder
				prompt.WriteString(fmt.Sprintf("Run the tool '%s'", toolName))
				if len(argValues) > 0 {
					prompt.WriteString(" with the following arguments:\n")
					for _, arg := range argValues {
						parts := strings.SplitN(arg, "=", 2)
						if len(parts) == 2 {
							prompt.WriteString(fmt.Sprintf("%s: %s\n", parts[0], parts[1]))
						}
					}
				}

				if debug {
					fmt.Printf("🔍 Classification prompt: %s\n", prompt.String())
				}

				// Create classification request
				reqBody := map[string]string{
					"message": prompt.String(),
				}
				reqJSON, err := json.Marshal(reqBody)
				if err != nil {
					return fmt.Errorf("failed to marshal classification request: %w", err)
				}

				// Create HTTP request
				baseURL := strings.TrimSuffix(cfg.BaseURL, "/")
				if strings.HasSuffix(baseURL, "/api/v1") {
					baseURL = strings.TrimSuffix(baseURL, "/api/v1")
				}
				classifyURL := fmt.Sprintf("%s/api/v1/http-bridge/v1/classify/agent", baseURL)
				req, err := http.NewRequestWithContext(cmd.Context(), http.MethodPost, classifyURL, bytes.NewBuffer(reqJSON))
				if err != nil {
					return fmt.Errorf("failed to create classification request: %w", err)
				}

				// Set headers
				req.Header.Set("Content-Type", "application/json")
				req.Header.Set("Authorization", "UserKey "+cfg.APIKey)

				if debug {
					fmt.Printf("🌐 Sending classification request to: %s\n", classifyURL)
				}

				// Send request
				resp, err := http.DefaultClient.Do(req)
				if err != nil {
					return fmt.Errorf("failed to send classification request: %w", err)
				}
				defer resp.Body.Close()

				// Read response body
				body, err := io.ReadAll(resp.Body)
				if err != nil {
					return fmt.Errorf("failed to read classification response: %w", err)
				}

				if debug {
					fmt.Printf("📥 Classification response status: %d\n", resp.StatusCode)
					fmt.Printf("📄 Classification response body: %s\n", string(body))
				}

				if resp.StatusCode != http.StatusOK {
					return fmt.Errorf("classification failed with status %d: %s", resp.StatusCode, string(body))
				}

				// Parse response
				var agents []struct {
					UUID        string `json:"uuid"`
					Name        string `json:"name"`
					Description string `json:"description"`
				}
				if err := json.Unmarshal(body, &agents); err != nil {
					return fmt.Errorf("failed to parse classification response: %w", err)
				}

				if len(agents) == 0 {
					if debug {
						fmt.Println("❌ No suitable agent found in the classification response")
					}
					return fmt.Errorf("no suitable agent found for the task")
				}

				// Use the first (best) agent
				agentID = agents[0].UUID
				fmt.Printf("🤖 Auto-selected agent: %s (%s)\n", agents[0].Name, agents[0].Description)
			}

			// If agent name is provided, look up the ID
			if agentName != "" && agentID == "" {
				if debug {
					fmt.Printf("🔍 Looking up agent by name: %s\n", agentName)
				}

				agents, err := client.GetAgents(cmd.Context())
				if err != nil {
					return fmt.Errorf("failed to list agents: %w", err)
				}

				if debug {
					fmt.Printf("📋 Found %d agents\n", len(agents))
				}

				found := false
				for _, t := range agents {
					if strings.EqualFold(t.Name, agentName) {
						agentID = t.UUID
						found = true
						if debug {
							fmt.Printf("✅ Found matching agent: %s (UUID: %s)\n", t.Name, t.UUID)
						}
						break
					}
				}

				if !found {
					if debug {
						fmt.Printf("❌ No agent found with name: %s\n", agentName)
					}
					return fmt.Errorf("agent with name '%s' not found", agentName)
				}
			}

			// Ensure we have a agent ID by this point
			if agentID == "" {
				return fmt.Errorf("no agent selected - please specify a agent or allow auto-classification")
			}

			// Session storage file path
			sessionFile := filepath.Join(os.TempDir(), "kubiya_last_session")

			// Handle clear session flag
			if clearSession {
				if err := os.Remove(sessionFile); err != nil && !os.IsNotExist(err) {
					return fmt.Errorf("failed to clear session: %w", err)
				}
				fmt.Println("Session cleared.")
				return nil
			}

			// Load last session ID if autoSession is enabled
			if sessionID == "" && cfg.AutoSession {
				if data, err := os.ReadFile(sessionFile); err == nil {
					sessionID = string(data)
					if debug {
						fmt.Printf("📥 Loaded session ID from file: %s\n", sessionID)
					}
					fmt.Printf("Resuming session ID: %s\n", sessionID)
				}
			}

			// Parse arguments
			argMap := make(map[string]string)
			for _, arg := range argValues {
				parts := strings.SplitN(arg, "=", 2)
				if len(parts) != 2 {
					return fmt.Errorf("invalid argument format: %s (expected key=value)", arg)
				}
				argMap[parts[0]] = parts[1]
			}

			// Format the prompt
			var prompt strings.Builder
			prompt.WriteString(fmt.Sprintf("Run the tool '%s' with the following arguments:\n", toolName))
			for key, value := range argMap {
				prompt.WriteString(fmt.Sprintf("%s: %s\n", key, value))
			}

			if debug {
				fmt.Printf("📤 Sending prompt: %s\n", prompt.String())
			}

			// Execute the tool
			fmt.Printf("🚀 Executing %s...\n", toolName)

			// Send message with context
			msgChan, err := client.SendMessageWithContext(cmd.Context(), agentID, prompt.String(), sessionID, nil)
			if err != nil {
				return fmt.Errorf("failed to execute tool: %w", err)
			}

			// Add these message type constants
			const (
				systemMsg  = "system"
				chatMsg    = "chat"
				toolMsg    = "tool"
				toolOutput = "tool_output"
			)

			// Progress spinner characters
			spinnerChars := []string{"⠋", "⠙", "⠹", "⠸", "⠼", "⠴", "⠦", "⠧", "⠇", "⠏"}
			spinnerIndex := 0

			// Create a map to track seen messages and prevent duplicates
			seenMessages := make(map[string]bool)
			var finalOutput strings.Builder
			var isProcessing bool
			var lastToolOutput string

			// Handle messages
			for msg := range msgChan {
				// Debug logging
				if debug {
					fmt.Printf("\n📥 Debug: Received message: Type=%s, Error=%q, SessionID=%s\n", msg.Type, msg.Error, msg.SessionID)
					if msg.Content != "" {
						fmt.Printf("📄 Debug: Content: %s\n", msg.Content)
					}
				}

				if msg.Error != "" {
					// Clear processing indicator if active
					if isProcessing {
						fmt.Print("\r" + strings.Repeat(" ", 50) + "\r")
					}
					if debug {
						fmt.Printf("\n❌ Debug: Error received: %s\n", msg.Error)
					}
					fmt.Print(style.ErrorStyle.Render("\n❌ Error: " + msg.Error + "\n"))
					return fmt.Errorf("error from server: %s", msg.Error)
				}

				// Save session ID if we get one
				if msg.SessionID != "" {
					sessionID = msg.SessionID
					if cfg.AutoSession {
						if err := os.WriteFile(sessionFile, []byte(sessionID), 0644); err != nil {
							if debug {
								fmt.Printf("\n⚠️ Debug: Failed to save session ID: %v\n", err)
							}
							fmt.Printf("Warning: failed to save session ID: %v\n", err)
						} else if debug {
							fmt.Printf("\n💾 Debug: Saved session ID to file: %s\n", sessionID)
						}
					}
				}

				// Skip empty messages
				if msg.Content == "" {
					continue
				}

				// Generate a key for deduplication
				msgKey := fmt.Sprintf("%s:%s", msg.Type, msg.Content)
				if seenMessages[msgKey] {
					if debug {
						fmt.Printf("\n🔄 Debug: Skipping duplicate message: %s\n", msgKey)
					}
					continue // Skip duplicate messages
				}
				seenMessages[msgKey] = true

				// Handle different message types
				switch msg.Type {
				case systemMsg:
					// Show processing indicator with spinner
					if !isProcessing {
						if debug {
							fmt.Printf("\n🔄 Debug: Starting processing indicator\n")
						}
						fmt.Print(style.SystemStyle.Render("\r🔄 Processing..."))
						isProcessing = true
					}
					// Update spinner
					spinnerIndex = (spinnerIndex + 1) % len(spinnerChars)
					fmt.Printf("\r%s %s", spinnerChars[spinnerIndex], style.SystemStyle.Render("Processing..."))

				case toolOutput:
					// Clear processing indicator
					if isProcessing {
						if debug {
							fmt.Printf("\n🛠️ Debug: Clearing processing indicator\n")
						}
						fmt.Print("\r" + strings.Repeat(" ", 50) + "\r")
						isProcessing = false
					}

					// Only store if it's different from the last output
					if msg.Content != lastToolOutput {
						if debug {
							fmt.Printf("\n📤 Debug: New tool output received\n")
						}
						lastToolOutput = msg.Content
						finalOutput.WriteString(msg.Content)
					} else if debug {
						fmt.Printf("\n🔄 Debug: Skipping duplicate tool output\n")
					}

				case chatMsg:
					// Clear processing indicator
					if isProcessing {
						if debug {
							fmt.Printf("\n💬 Debug: Clearing processing indicator\n")
						}
						fmt.Print("\r" + strings.Repeat(" ", 50) + "\r")
						isProcessing = false
					}

					// Store chat messages with emoji
					if debug {
						fmt.Printf("\n💬 Debug: New chat message received\n")
					}
					finalOutput.WriteString(style.ChatStyle.Render("\n💬 " + msg.Content + "\n"))

				case toolMsg:
					// Clear processing indicator
					if isProcessing {
						if debug {
							fmt.Printf("\n🔧 Debug: Clearing processing indicator\n")
						}
						fmt.Print("\r" + strings.Repeat(" ", 50) + "\r")
						isProcessing = false
					}

					// Store tool messages with emoji
					if debug {
						fmt.Printf("\n🔧 Debug: New tool message received\n")
					}
					finalOutput.WriteString(style.ToolStyle.Render("\n🔧 " + msg.Content + "\n"))
				}
			}

			// Clear any remaining processing indicator
			if isProcessing {
				if debug {
					fmt.Printf("\n🔄 Debug: Clearing final processing indicator\n")
				}
				fmt.Print("\r" + strings.Repeat(" ", 50) + "\r")
			}

			// Print the final output
			if finalOutput.Len() > 0 {
				if debug {
					fmt.Printf("\n📤 Debug: Printing final output\n")
				}
				fmt.Print(finalOutput.String())
			}

			return nil
		},
	}

	cmd.Flags().StringVarP(&agentID, "agent", "t", "", "Agent ID (optional)")
	cmd.Flags().StringVarP(&agentName, "name", "n", "", "Agent name (optional)")
	cmd.Flags().BoolVar(&noClassify, "no-classify", false, "Disable automatic agent classification")
	cmd.Flags().StringArrayVar(&argValues, "arg", []string{}, "Tool arguments (key=value)")
	cmd.Flags().BoolVar(&clearSession, "clear-session", false, "Clear the current session")
	cmd.Flags().StringVar(&sessionID, "session", "", "Session ID to resume")
	cmd.Flags().BoolVar(&debug, "debug", false, "Enable debug logging")

	return cmd
}
