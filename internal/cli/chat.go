package cli

import (
	"bufio"
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/kubiyabot/cli/internal/config"
	"github.com/kubiyabot/cli/internal/kubiya"
	"github.com/kubiyabot/cli/internal/style"
	"github.com/kubiyabot/cli/internal/tui"
	"github.com/mattn/go-isatty"
	"github.com/spf13/cobra"
)

// Add this type definition at the package level
type toolExecution struct {
	name       string
	args       string
	output     strings.Builder
	hasOutput  bool
	isComplete bool
	msgID      string
	failed     bool
	status     string // "waiting", "running", "done", "failed"
	startTime  time.Time
}

// Add a buffer for chat messages
type chatBuffer struct {
	content     string
	sentence    strings.Builder
	inCodeBlock bool
	codeBlock   strings.Builder
}

// Add status emojis
const (
	statusWaiting = "⏳" // Tool is queued
	statusRunning = "🔄" // Tool is running
	statusDone    = "✅" // Tool completed successfully
	statusFailed  = "❌" // Tool failed
)

func newChatCommand(cfg *config.Config) *cobra.Command {
	var (
		agentID   string
		agentName string
		message      string
		noClassify   bool

		interactive  bool
		debug        bool
		stream       bool
		clearSession bool
		sessionID    string
		contextFiles []string
		stdinInput   bool
		sourceTest   bool
		sourceUUID   string
		sourceName   string
		suggestTool  string
	)

	// Helper function to fetch content from URL
	fetchURL := func(url string) (string, error) {
		resp, err := http.Get(url)
		if err != nil {
			return "", err
		}
		defer resp.Body.Close()

		if resp.StatusCode != http.StatusOK {
			return "", fmt.Errorf("failed to fetch URL %s: status %d", url, resp.StatusCode)
		}

		content, err := io.ReadAll(resp.Body)
		if err != nil {
			return "", err
		}
		return string(content), nil
	}

	// Helper function to expand wildcards and read files
	expandAndReadFiles := func(patterns []string) (map[string]string, error) {
		context := make(map[string]string)
		for _, pattern := range patterns {
			// Handle URLs
			if strings.HasPrefix(pattern, "http://") || strings.HasPrefix(pattern, "https://") {
				content, err := fetchURL(pattern)
				if err != nil {
					return nil, fmt.Errorf("failed to fetch URL %s: %w", pattern, err)
				}
				context[pattern] = content
				continue
			}

			// Handle file patterns
			matches, err := filepath.Glob(pattern)
			if err != nil {
				return nil, fmt.Errorf("invalid pattern %s: %w", pattern, err)
			}

			if len(matches) == 0 {
				return nil, fmt.Errorf("no files match pattern: %s", pattern)
			}

			for _, match := range matches {
				info, err := os.Stat(match)
				if err != nil {
					return nil, fmt.Errorf("failed to stat file %s: %w", match, err)
				}

				// Skip directories
				if info.IsDir() {
					continue
				}

				content, err := os.ReadFile(match)
				if err != nil {
					return nil, fmt.Errorf("failed to read file %s: %w", match, err)
				}
				context[match] = string(content)
			}
		}
		return context, nil
	}

	cmd := &cobra.Command{
		Use:   "chat",
		Short: "💬 Chat with a agent",
		Long: `Start a chat session with a Kubiya agent.
You can either use interactive mode, specify a message directly, or pipe input from stdin.
Use --context to include additional files for context (supports wildcards and URLs).
The command will automatically select the most appropriate agent unless one is specified.`,
		Example: `  # Interactive chat mode
  kubiya chat --interactive

  # Using context files with wildcards
  kubiya chat -n "security" -m "Review this code" --context "src/*.go" --context "tests/**/*_test.go"

  # Using URLs as context
  kubiya chat -n "security" -m "Check this" --context https://raw.githubusercontent.com/org/repo/main/config.yaml

  # Multiple context sources
  kubiya chat -n "devops" \
    --context "k8s/*.yaml" \
    --context "https://example.com/deployment.yaml" \
    --context "Dockerfile" \
    -m "Review deployment"

  # Pipe from stdin with context
  cat error.log | kubiya chat -n "debug" --stdin --context "config/*.yaml"

  # Auto-classify the most appropriate agent
  kubiya chat -m "Help me with Kubernetes deployment issues"`,
		RunE: func(cmd *cobra.Command, args []string) error {
			cfg.Debug = cfg.Debug || debug

			if interactive {
				chatUI := tui.NewChatUI(cfg)
				return chatUI.Run()
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
					fmt.Printf("Resuming session ID: %s\n", sessionID)
				}
			}

			// Handle stdin input
			if stdinInput {
				stat, _ := os.Stdin.Stat()
				if (stat.Mode() & os.ModeCharDevice) != 0 {
					return fmt.Errorf("no stdin data provided")
				}

				reader := bufio.NewReader(os.Stdin)
				var sb strings.Builder
				for {
					line, err := reader.ReadString('\n')
					if err != nil && err != io.EOF {
						return fmt.Errorf("error reading stdin: %w", err)
					}
					sb.WriteString(line)
					if err == io.EOF {
						break
					}
				}
				message = sb.String()
			}

			// Validate input
			if message == "" && !stdinInput {
				return fmt.Errorf("message is required (use -m, --stdin, or pipe input)")
			}

			// Load context from all sources
			context, err := expandAndReadFiles(contextFiles)
			if err != nil {
				return fmt.Errorf("failed to load context: %w", err)
			}

			// Setup client
			client := kubiya.NewClient(cfg)

			// Auto-classify by default unless agent is explicitly specified or --no-classify is set
			shouldClassify := agentID == "" && agentName == "" && !noClassify

			// If auto-classify is enabled (default), use the classification endpoint
			if shouldClassify {
				if debug {
					fmt.Printf("🔍 Classification prompt: %s\n", message)
				}

				// Create classification request
				reqBody := map[string]string{
					"message": message,
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

			// Add these variables
			var (
				toolExecutions map[string]*toolExecution = make(map[string]*toolExecution)
				messageBuffer  map[string]*chatBuffer    = make(map[string]*chatBuffer)
				noColor        bool                      = !isatty.IsTerminal(os.Stdout.Fd())
			)

			// Before the message handling loop, add style configuration for non-TTY:
			if noColor {
				// Disable all styling for non-TTY environments
				style.DisableColors()
			}

			// Send message with context
			msgChan, err := client.SendMessageWithContext(cmd.Context(), agentID, message, sessionID, context)
			if err != nil {
				return err
			}

			// Read messages and handle session ID
			var finalResponse strings.Builder

			// Add these message type constants
			const (
				systemMsg  = "system"
				chatMsg    = "chat"
				toolMsg    = "tool"
				toolOutput = "tool_output"
			)

			// Update the message handling loop:
			for msg := range msgChan {
				if msg.Error != "" {
					fmt.Print(style.ErrorStyle.Render("\n❌ Error: " + msg.Error + "\n"))
					return fmt.Errorf("error from server: %s", msg.Error)
				}

				// Handle system messages
				if msg.Type == systemMsg {
					fmt.Print(style.SystemStyle.Render("\n🔄 " + msg.Content + "\n"))
					continue
				}

				switch msg.Type {
				case toolMsg:
					toolInfo := strings.TrimSpace(msg.Content)
					if strings.HasPrefix(toolInfo, "Tool:") {
						parts := strings.SplitN(toolInfo, "Arguments:", 2)
						toolName := strings.TrimSpace(strings.TrimPrefix(parts[0], "Tool:"))
						toolArgs := ""
						if len(parts) > 1 {
							toolArgs = strings.TrimSpace(parts[1])
						}

						// Only process if we have complete arguments and haven't seen this exact tool+args combination
						if strings.HasSuffix(toolArgs, "}") {
							// Check for duplicate tool execution
							isDuplicate := false
							for _, te := range toolExecutions {
								if te.name == toolName && te.args == toolArgs && !te.isComplete {
									isDuplicate = true
									break
								}
							}

							if !isDuplicate {
								// Create new tool execution
								te := &toolExecution{
									name:      toolName,
									args:      toolArgs,
									msgID:     msg.MessageID,
									status:    "waiting",
									startTime: time.Now(),
								}
								toolExecutions[msg.MessageID] = te

								// Print tool header with enhanced formatting
								fmt.Printf("\n%s\n", style.ToolHeaderStyle.Render(fmt.Sprintf("┌─ 🔧 Running %s ", toolName)))
								if toolArgs != "" {
									prettyArgs := toolArgs
									if json.Valid([]byte(toolArgs)) {
										var prettyJSON bytes.Buffer
										json.Indent(&prettyJSON, []byte(toolArgs), "    ", "  ")
										prettyArgs = prettyJSON.String()
									}
									fmt.Printf("%s\n", style.ToolArgsStyle.Render(fmt.Sprintf("└─ Args: %s", prettyArgs)))
								}
								fmt.Println()
							}
						}
					}

				case toolOutput:
					te := toolExecutions[msg.MessageID]
					if te != nil && !te.isComplete {
						storedContent := messageBuffer[msg.MessageID]
						if storedContent == nil {
							storedContent = &chatBuffer{}
							messageBuffer[msg.MessageID] = storedContent
						}

						newContent := msg.Content
						if len(storedContent.content) > 0 {
							newContent = msg.Content[len(storedContent.content):]
						}

						trimmedContent := strings.TrimSpace(newContent)
						if trimmedContent != "" {
							te.hasOutput = true
							te.output.WriteString(newContent)

							// Try to parse as JSON first for structured output
							var outputData struct {
								State   string `json:"state,omitempty"`
								Status  string `json:"status,omitempty"`
								Output  string `json:"output,omitempty"`
								Error   string `json:"error,omitempty"`
								Message string `json:"message,omitempty"`
							}

							if err := json.Unmarshal([]byte(trimmedContent), &outputData); err == nil {
								// Handle structured output
								if outputData.State != "" {
									te.status = outputData.State
								}
								if outputData.Error != "" {
									te.failed = true
									fmt.Printf("%s %s │ %s\n",
										style.ToolOutputPrefixStyle.Render("❌"),
										style.ToolNameStyle.Render(te.name),
										style.ErrorStyle.Render(outputData.Error))
								}
								if outputData.Output != "" || outputData.Message != "" {
									output := outputData.Output
									if output == "" {
										output = outputData.Message
									}
									fmt.Printf("%s %s │ %s\n",
										style.ToolOutputPrefixStyle.Render("│"),
										style.ToolNameStyle.Render(te.name),
										style.ToolOutputStyle.Render(output))
								}
							} else {
								// Handle plain text output
								lines := strings.Split(trimmedContent, "\n")
								for _, line := range lines {
									line = strings.TrimSpace(line)
									if line != "" {
										prefix := "│"
										outputStyle := style.ToolOutputStyle

										switch {
										case strings.Contains(strings.ToLower(line), "error"):
											prefix = "❌"
											outputStyle = style.ErrorStyle
											te.failed = true
										case strings.Contains(strings.ToLower(line), "warning"):
											prefix = "⚠️"
											outputStyle = style.WarningStyle
										case strings.Contains(strings.ToLower(line), "success"):
											prefix = "✅"
											outputStyle = style.SuccessStyle
										}

										fmt.Printf("%s %s │ %s\n",
											style.ToolOutputPrefixStyle.Render(prefix),
											style.ToolNameStyle.Render(te.name),
											outputStyle.Render(line))
									}
								}
							}
						}
						storedContent.content = msg.Content
					}

				default:
					// Handle tool completion
					for msgID, te := range toolExecutions {
						if te.hasOutput && !te.isComplete {
							te.isComplete = true
							duration := time.Since(te.startTime).Seconds()

							// Determine status emoji and completion message
							var statusEmoji string
							var completionStatus string
							if te.failed {
								statusEmoji = statusFailed
								completionStatus = "failed"
							} else {
								statusEmoji = statusDone
								completionStatus = "completed"
							}

							// Print completion status with summary
							fmt.Printf("\n%s %s (%0.1fs)\n",
								style.ToolStatusStyle.Render(statusEmoji),
								style.ToolCompleteStyle.Render(fmt.Sprintf("Tool %s %s",
									te.name,
									completionStatus)),
								duration)

							// Print error summary if failed
							if te.failed {
								fmt.Printf("%s %s\n",
									style.ToolOutputPrefixStyle.Render("!"),
									style.ErrorStyle.Render("Tool encountered errors during execution"))
							}

							// Print output summary
							if te.output.Len() > 0 {
								fmt.Printf("%s %s\n",
									style.ToolOutputPrefixStyle.Render("└"),
									style.ToolSummaryStyle.Render(fmt.Sprintf("Output: %d bytes", te.output.Len())))
							}
							fmt.Println()
							delete(toolExecutions, msgID)
						}
					}

					// Regular chat message
					if msg.SenderName != "You" {
						buf, exists := messageBuffer[msg.MessageID]
						if !exists {
							buf = &chatBuffer{}
							messageBuffer[msg.MessageID] = buf
						}

						if len(msg.Content) > len(buf.content) {
							newContent := msg.Content[len(buf.content):]

							// Accumulate content and handle code blocks
							for _, char := range newContent {
								if char == '`' {
									buf.inCodeBlock = !buf.inCodeBlock
									if buf.inCodeBlock {
										// Print accumulated sentence before code block
										if buf.sentence.Len() > 0 {
											sentence := strings.TrimSpace(buf.sentence.String())
											if sentence != "" {
												// Print without [Bot] prefix
												fmt.Printf("%s\n",
													style.AgentStyle.Render(sentence))
												buf.sentence.Reset()
											}
										}
									} else {
										// Print accumulated code block
										if buf.codeBlock.Len() > 0 {
											fmt.Printf("%s\n%s\n%s\n",
												style.CodeBlockStyle.Render("```"),
												style.CodeBlockStyle.Render(buf.codeBlock.String()),
												style.CodeBlockStyle.Render("```"))
											buf.codeBlock.Reset()
										}
									}
									continue
								}

								if buf.inCodeBlock {
									buf.codeBlock.WriteRune(char)
								} else {
									buf.sentence.WriteRune(char)
									if char == '.' || char == '!' || char == '?' || char == '\n' {
										sentence := strings.TrimSpace(buf.sentence.String())
										if sentence != "" {
											// Print without [Bot] prefix
											fmt.Printf("%s\n",
												style.AgentStyle.Render(sentence))
											buf.sentence.Reset()
										}
									}
								}
							}

							buf.content = msg.Content
						}
					}

					if msg.Final {
						// Print any remaining content in the sentence buffer
						if buf, exists := messageBuffer[msg.MessageID]; exists {
							remaining := strings.TrimSpace(buf.sentence.String())
							if remaining != "" {
								fmt.Printf("%s %s\n",
									style.AgentNameStyle.Render("["+msg.SenderName+"]"),
									style.AgentStyle.Render(remaining))
							}
						}
						fmt.Println()
					}
				}
			}

			if !stream {
				fmt.Println(finalResponse.String())
			}

			return nil
		},
	}

	cmd.Flags().StringVarP(&agentID, "agent", "t", "", "Agent ID (optional)")
	cmd.Flags().StringVarP(&agentName, "name", "n", "", "Agent name (optional)")
	cmd.Flags().StringVarP(&message, "message", "m", "", "Message to send")
	cmd.Flags().BoolVarP(&interactive, "interactive", "i", false, "Start interactive chat mode")
	cmd.Flags().BoolVar(&debug, "debug", false, "Enable debug logging")
	cmd.Flags().BoolVar(&stream, "stream", true, "Stream the response")
	cmd.Flags().BoolVar(&clearSession, "clear-session", false, "Clear the current session")
	cmd.Flags().StringVar(&sessionID, "session", "", "Session ID to resume")
	cmd.Flags().StringArrayVar(&contextFiles, "context", []string{}, "Files to include as context (supports wildcards and URLs)")
	cmd.Flags().BoolVar(&stdinInput, "stdin", false, "Read message from stdin")
	cmd.Flags().BoolVar(&sourceTest, "source-test", false, "Test source connection")
	cmd.Flags().StringVar(&sourceUUID, "source-uuid", "", "Source UUID")
	cmd.Flags().StringVar(&sourceName, "source-name", "", "Source name")
	cmd.Flags().StringVar(&suggestTool, "suggest-tool", "", "Suggest a tool to use")
	cmd.Flags().BoolVar(&noClassify, "no-classify", false, "Disable automatic agent classification")

	return cmd
}

// Add this helper function at package level:
func contains(slice []string, str string) bool {
	for _, s := range slice {
		if s == str {
			return true
		}
	}
	return false
}
