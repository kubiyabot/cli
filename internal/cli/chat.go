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
	"sync"
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
	runner     string
	toolCallId string
	outputTruncated bool
}

// Add connection status tracking
type connectionStatus struct {
	runner      string
	runnerType  string // "k8s", "docker", "local", etc.
	connected   bool
	connectTime time.Time
	lastPing    time.Time
	latency     time.Duration
}

// Add tool call statistics
type toolCallStats struct {
	totalCalls     int
	activeCalls    int
	completedCalls int
	failedCalls    int
	toolTypes      map[string]int
	mu             sync.RWMutex
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
		agentID    string
		agentName  string
		message    string
		noClassify bool

		interactive     bool
		debug           bool
		stream          bool
		clearSession    bool
		sessionID       string
		contextFiles    []string
		stdinInput      bool
		sourceTest      bool
		sourceUUID      string
		sourceName      string
		suggestTool     string
		permissionLevel string

		showToolCalls   bool
		retries         int
		
		// Inline agent flags
		inline         bool
		toolsFile      string
		toolsJSON      string
		aiInstructions string
		description    string
		runners        []string
		integrations   []string
		secrets        []string
		envVars        []string
		llmModel       string
		isDebugMode    bool
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

	// Helper function to parse tools from JSON
	parseTools := func(toolsJSON string) ([]kubiya.Tool, error) {
		var tools []kubiya.Tool
		if err := json.Unmarshal([]byte(toolsJSON), &tools); err != nil {
			return nil, fmt.Errorf("failed to parse tools JSON: %w", err)
		}
		return tools, nil
	}

	// Helper function to validate tool definition
	validateTool := func(tool kubiya.Tool) error {
		if tool.Name == "" {
			return fmt.Errorf("tool name is required")
		}
		if tool.Description == "" {
			return fmt.Errorf("tool description is required")
		}
		if tool.Content == "" {
			return fmt.Errorf("tool content is required")
		}
		// Validate required args
		for _, arg := range tool.Args {
			if arg.Name == "" {
				return fmt.Errorf("tool arg name is required")
			}
			if arg.Description == "" {
				return fmt.Errorf("tool arg description is required for arg: %s", arg.Name)
			}
		}
		return nil
	}

	// Helper function to parse environment variables
	parseEnvVars := func(envVars []string) (map[string]string, error) {
		envMap := make(map[string]string)
		for _, env := range envVars {
			parts := strings.SplitN(env, "=", 2)
			if len(parts) != 2 {
				return nil, fmt.Errorf("invalid environment variable format: %s (expected KEY=VALUE)", env)
			}
			envMap[parts[0]] = parts[1]
		}
		return envMap, nil
	}

	cmd := &cobra.Command{
		Use:   "chat",
		Short: "💬 Chat with a agent",
		Long: `Start a chat session with a Kubiya agent.
You can either use enhanced interactive mode, specify a message directly, or pipe input from stdin.
Use --context to include additional files for context (supports wildcards and URLs).
The command will automatically select the most appropriate agent unless one is specified.

Enhanced Interactive Mode Features:
• Beautiful terminal UI with colors and formatting
• Session persistence and history management
• Connection retry and error handling
• Tool execution tracking with real-time status
• Keyboard shortcuts for improved productivity
• Auto-save functionality
• Message history navigation

Permission Levels:
• read: Execute read-only operations (kubectl get, describe, logs, etc.)
• readwrite: Execute all operations including read-write (kubectl apply, delete, etc.)
• ask: Ask for confirmation before executing operations

For inline agents, use --inline with --tools-file or --tools-json to provide custom tools.`,
		Example: `  # Enhanced interactive chat mode
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
  kubiya chat -m "Help me with Kubernetes deployment issues"

  # Different permission levels
  kubiya chat -n "devops" -m "Show me the pods" --permission-level read
  kubiya chat -n "devops" -m "Deploy the application" --permission-level readwrite
  kubiya chat -n "devops" -m "Check system status" --permission-level ask

  # Continue a previous conversation
  kubiya chat --session abc123-def456-ghi789 -m "What about the logs?"

  # Inline agent with tools from file
  kubiya chat --inline --tools-file tools.json --ai-instructions "You are a helpful assistant" \
    --description "Custom inline agent" --runners "kubiyamanaged" -m "kubectl get pods"

  # Inline agent with tools from JSON string
  kubiya chat --inline --tools-json '[{"name":"echo","description":"Echo tool","content":"echo hello"}]' \
    --llm-model "azure/gpt-4-32k" --debug-mode -m "Run echo command"

  # Inline agent with environment variables and secrets
  kubiya chat --inline --tools-file tools.json --env-vars "ENV1=value1" --env-vars "ENV2=value2" \
    --secrets "SECRET1" --integrations "jira" -m "Use the tools"`,
		RunE: func(cmd *cobra.Command, args []string) error {
			cfg.Debug = cfg.Debug || debug

			if interactive {
				chatUI := tui.NewEnhancedChatUI(cfg)
				return chatUI.Run()
			}

			// Handle inline agent validation
			if inline {
				if agentID != "" || agentName != "" {
					return fmt.Errorf("cannot use --inline with --agent or --name")
				}
				if toolsFile == "" && toolsJSON == "" {
					return fmt.Errorf("--inline requires either --tools-file or --tools-json")
				}
				if toolsFile != "" && toolsJSON != "" {
					return fmt.Errorf("cannot use both --tools-file and --tools-json")
				}
				if description == "" {
					description = "Inline agent"
				}
				if len(runners) == 0 {
					runners = []string{"kubiyamanaged"}
				}
				if llmModel == "" {
					llmModel = "azure/gpt-4-32k"
				}
				if len(integrations) == 0 {
					integrations = []string{}
				}
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

			// Validate permission level
			if permissionLevel != "" && permissionLevel != "read" && permissionLevel != "readwrite" && permissionLevel != "ask" {
				return fmt.Errorf("invalid permission level: %s (must be 'read', 'readwrite', or 'ask')", permissionLevel)
			}
			if permissionLevel == "" {
				permissionLevel = "read" // Default to read-only
			}

			// Load context from all sources
			context, err := expandAndReadFiles(contextFiles)
			if err != nil {
				return fmt.Errorf("failed to load context: %w", err)
			}

			// Enhance message with permission level context
			enhancedMessage := message
			if !interactive {
				permissionMsg := ""
				switch permissionLevel {
				case "read":
					permissionMsg = "\n\n[SYSTEM] You have permission to execute READ-ONLY operations (kubectl get, describe, logs, etc.). You should execute these operations directly without asking for confirmation."
				case "readwrite":
					permissionMsg = "\n\n[SYSTEM] You have FULL PERMISSION to execute any operations including read-write operations (kubectl apply, delete, create, etc.). Execute operations directly without asking for confirmation."
				case "ask":
					permissionMsg = "\n\n[SYSTEM] You should ASK for confirmation before executing any operations. Present the commands you want to run and wait for user approval."
				}
				enhancedMessage = message + permissionMsg
			}

			// Setup client
			client := kubiya.NewClient(cfg)

			// Add these variables
			var (
				toolExecutions map[string]*toolExecution = make(map[string]*toolExecution)
				messageBuffer  map[string]*chatBuffer    = make(map[string]*chatBuffer)
				noColor        bool                      = !isatty.IsTerminal(os.Stdout.Fd())
				connStatus     *connectionStatus
				toolStats      = &toolCallStats{
					toolTypes: make(map[string]int),
				}
				rawEventLogging = os.Getenv("KUBIYA_RAW_EVENTS") == "1" || debug
				msgChan         <-chan kubiya.ChatMessage
			)

			// Handle inline agent
			if inline {
				var tools []kubiya.Tool

				// Parse tools from file or JSON
				if toolsFile != "" {
					if debug {
						fmt.Printf("🔍 Loading tools from file: %s\n", toolsFile)
					}

					toolsData, err := os.ReadFile(toolsFile)
					if err != nil {
						return fmt.Errorf("failed to read tools file: %w", err)
					}

					tools, err = parseTools(string(toolsData))
					if err != nil {
						return fmt.Errorf("failed to parse tools from file: %w", err)
					}
				} else if toolsJSON != "" {
					if debug {
						fmt.Printf("🔍 Parsing tools from JSON string\n")
					}

					tools, err = parseTools(toolsJSON)
					if err != nil {
						return fmt.Errorf("failed to parse tools from JSON: %w", err)
					}
				}

				// Validate tools
				for i, tool := range tools {
					if err := validateTool(tool); err != nil {
						return fmt.Errorf("tool validation failed for tool %d (%s): %w", i, tool.Name, err)
					}
				}

				// Parse environment variables
				envVarsMap, err := parseEnvVars(envVars)
				if err != nil {
					return fmt.Errorf("failed to parse environment variables: %w", err)
				}

				// Add KUBIYA_RUNNER if runners are specified
				if len(runners) > 0 {
					envVarsMap["KUBIYA_RUNNER"] = runners[0]
				}

				if debug {
					fmt.Printf("🤖 Creating inline agent with %d tools\n", len(tools))
					fmt.Printf("📋 Tools: %v\n", func() []string {
						names := make([]string, len(tools))
						for i, t := range tools {
							names[i] = t.Name
						}
						return names
					}())
				}

				// Convert tools to the correct format for inline agent
				inlineTools := make([]map[string]interface{}, len(tools))
				for i, tool := range tools {
					// Ensure args is an empty slice if nil
					args := tool.Args
					if args == nil {
						args = []kubiya.ToolArg{}
					}

					// Ensure env is an empty slice if nil
					env := tool.Env
					if env == nil {
						env = []string{}
					}

					// Ensure with_files is an empty slice if nil
					withFiles := tool.WithFiles
					if withFiles == nil {
						withFiles = []interface{}{}
					}

					// Ensure with_volumes is an empty slice if nil
					withVolumes := tool.WithVolumes
					if withVolumes == nil {
						withVolumes = []interface{}{}
					}

					inlineTools[i] = map[string]interface{}{
						"name":         tool.Name,
						"alias":        tool.Alias,
						"description":  tool.Description,
						"type":         tool.Type,
						"content":      tool.Content,
						"args":         args,
						"env":          env,
						"image":        tool.Image,
						"with_files":   withFiles,
						"with_volumes": withVolumes,
					}
				}

				// Create inline agent request - use the same message handling as regular agents
				inlineAgent := map[string]interface{}{
					"uuid":                  nil,
					"name":                  "inline",
					"description":           description,
					"ai_instructions":       aiInstructions,
					"tools":                 inlineTools,
					"runners":               runners,
					"integrations":          integrations,
					"secrets":               secrets,
					"environment_variables": envVarsMap,
					"llm_model":             llmModel,
					"is_debug_mode":         true, // Always true for inline agents to capture tool output
					"owners":                []string{},
					"allowed_users":         []string{},
					"allowed_groups":        []string{},
					"starters":              []interface{}{},
					"tasks":                 []string{},
					"sources":               []string{},
					"links":                 []string{},
					"image":                 "",
					"additional_data":       map[string]interface{}{},
				}

				if debug {
					fmt.Printf("🔍 CLI: Creating inline agent with payload:\n")
					if jsonPayload, err := json.MarshalIndent(inlineAgent, "", "  "); err == nil {
						fmt.Printf("Agent Definition: %s\n", string(jsonPayload))
					}
					fmt.Printf("Message: %s\n", message)
					fmt.Printf("SessionID: %s\n", sessionID)
					fmt.Printf("Context files: %d\n", len(context))
					fmt.Printf("User Email: %s\n", os.Getenv("KUBIYA_USER_EMAIL"))
					fmt.Printf("Organization: %s\n", os.Getenv("KUBIYA_ORG"))
					apiKey := os.Getenv("KUBIYA_API_KEY")
					if len(apiKey) > 20 {
						fmt.Printf("API Key: %s\n", apiKey[:20]+"...")
					} else {
						fmt.Printf("API Key: %s\n", apiKey)
					}
					fmt.Printf("Base URL: %s\n", cfg.BaseURL)
					fmt.Printf("Debug mode: %v\n", debug)
				}

				msgChan, err = client.SendInlineAgentMessage(cmd.Context(), message, sessionID, context, inlineAgent)
				if err != nil {
					return err
				}

				// Set agentID for inline agent to use the same processing logic
				agentID = "inline"
			}

			// Auto-classify by default unless agent is explicitly specified or --no-classify is set
			shouldClassify := agentID == "" && agentName == "" && !noClassify && !inline

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
				baseURL = strings.TrimSuffix(baseURL, "/api/v1")

				classifyURL := fmt.Sprintf("%s/http-bridge/v1/classify/agent", baseURL)
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

			// Before the message handling loop, add style configuration for non-TTY:
			if noColor {
				// Disable all styling for non-TTY environments
				style.DisableColors()
			}

			// Initialize connection status - fetch actual runner from agent
			var agentRunner string
			var agentInfo *kubiya.Agent
			
			// Fetch agent information to get the actual runner (skip for inline agents)
			if !inline {
				if agentInfo, err = client.GetAgent(cmd.Context(), agentID); err != nil {
					if debug {
						fmt.Printf("⚠️ Failed to get agent info: %v, using default runner\n", err)
					}
					agentRunner = "kubiyamanaged"
				} else if len(agentInfo.Runners) > 0 {
					agentRunner = agentInfo.Runners[0] // Use first runner
				} else {
					agentRunner = "kubiyamanaged" // fallback
				}
			} else {
				// For inline agents, use the provided runners or default
				if len(runners) > 0 {
					agentRunner = runners[0]
				} else {
					agentRunner = "kubiyamanaged"
				}
			}
			
			// Override with environment variable if set
			if os.Getenv("KUBIYA_RUNNER") != "" {
				agentRunner = os.Getenv("KUBIYA_RUNNER")
			}
			
			connStatus = &connectionStatus{
				runner:      agentRunner,
				runnerType:  "k8s",
				connected:   false,
				connectTime: time.Now(),
			}
			
			// Show enhanced connection status with progress indicators
			fmt.Printf("%s\n", style.InfoBoxStyle.Render(
				fmt.Sprintf("🔗 Connecting to agent runner service at: runner://%s (%s)",
					connStatus.runner, connStatus.runnerType)))
			
			// Show additional agent details if available
			if agentInfo != nil {
				fmt.Printf("%s\n", style.InfoBoxStyle.Render(
					fmt.Sprintf("🤖 Agent: %s (%s)", agentInfo.Name, agentInfo.Description)))
				if agentInfo.LLMModel != "" {
					fmt.Printf("%s\n", style.InfoBoxStyle.Render(
						fmt.Sprintf("🧠 Model: %s", agentInfo.LLMModel)))
				}
			}
			
			// Show connection steps
			fmt.Printf("%s\n", style.SpinnerStyle.Render("⏳ Establishing secure connection..."))

			fmt.Printf("%s\n", style.SpinnerStyle.Render("🔐 Authenticating with API key..."))
			fmt.Printf("%s\n", style.SpinnerStyle.Render("🤖 Initializing agent session..."))
			
			// Send message with context, with retry mechanism for robustness (skip for inline agents)
			if !inline {
				msgChan, err = client.SendMessageWithContext(cmd.Context(), agentID, enhancedMessage, sessionID, context)
				if err != nil {
					// If context method fails, try with retry mechanism
					if strings.Contains(err.Error(), "connection") || strings.Contains(err.Error(), "timeout") {
						fmt.Printf("\r%s\n", style.WarningStyle.Render("⚠️  Initial connection failed, retrying with enhanced resilience..."))
						msgChan, err = client.SendMessageWithRetry(cmd.Context(), agentID, enhancedMessage, sessionID, retries)
						if err != nil {
							return fmt.Errorf("failed to send message after %d retries: %w", retries, err)
						}
					} else {
						return err
					}
				}
			}

			// Connection established
			connStatus.connected = true
			connStatus.lastPing = time.Now()
			connStatus.latency = time.Since(connStatus.connectTime)

			
			// Clear previous status lines and show success
			fmt.Printf("\r\033[K")  // Clear current line
			fmt.Printf("\033[3A")   // Move up 3 lines
			fmt.Printf("\033[J")    // Clear from cursor to end of screen
			
			fmt.Printf("%s\n", style.SuccessStyle.Render(
				fmt.Sprintf("✅ Connected to runner://%s (%s) - latency: %v", 
					connStatus.runner, connStatus.runnerType, connStatus.latency)))
			fmt.Printf("%s\n", style.InfoBoxStyle.Render("🚀 Agent is ready and processing your request..."))

			// Read messages and handle session ID
			var finalResponse strings.Builder
			var completionReason string
			var hasError bool
			var actualSessionID string
			var toolsExecuted bool
			var streamRetryCount int
			var anyOutputTruncated bool

			// Add these message type constants
			const (
				systemMsg  = "system"
				chatMsg    = "chat"
				toolMsg    = "tool"
				toolOutput = "tool_output"
			)

			// Update the message handling loop with stream error handling:
			for msg := range msgChan {
				if msg.Error != "" {
					// Check if this is a stream error that we can retry
					if strings.Contains(strings.ToLower(msg.Error), "stream_error") || 
					   strings.Contains(strings.ToLower(msg.Error), "connection") ||
					   strings.Contains(strings.ToLower(msg.Error), "timeout") {
						
						if streamRetryCount < retries {
							streamRetryCount++
							fmt.Printf("\r%s\n", style.WarningStyle.Render(
								fmt.Sprintf("⚠️  Stream error detected (attempt %d/%d): %s", 
									streamRetryCount, retries, msg.Error)))
							fmt.Printf("%s\n", style.SpinnerStyle.Render("🔄 Attempting to reconnect..."))
							
							// Attempt to reconnect
							time.Sleep(time.Duration(streamRetryCount) * time.Second) // Exponential backoff
							if !inline {
								msgChan, err = client.SendMessageWithRetry(cmd.Context(), agentID, enhancedMessage, actualSessionID, 1)
								if err != nil {
									fmt.Printf("%s\n", style.ErrorStyle.Render(fmt.Sprintf("❌ Reconnection failed: %v", err)))
									continue
								}
								fmt.Printf("%s\n", style.SuccessStyle.Render("✅ Reconnected successfully, continuing..."))
								continue
							}
						} else {
							fmt.Fprintf(os.Stderr, "%s\n", style.ErrorStyle.Render(
								fmt.Sprintf("❌ Stream failed after %d retries: %s", retries, msg.Error)))
							hasError = true
							return fmt.Errorf("stream failed after %d retries: %s", retries, msg.Error)
						}
					} else {
						fmt.Fprintf(os.Stderr, "%s\n", style.ErrorStyle.Render("❌ Error: " + msg.Error))
						hasError = true
						return fmt.Errorf("error from server: %s", msg.Error)
					}
				}

				// Raw event logging for debugging
				if rawEventLogging {
					fmt.Printf("[RAW EVENT] Type: %s, Content: %s, MessageID: %s, Final: %v, SessionID: %s\n",
						msg.Type, msg.Content, msg.MessageID, msg.Final, msg.SessionID)
				}

				// Capture completion reason and session ID from final messages
				if msg.Final && msg.FinishReason != "" {
					completionReason = msg.FinishReason
				}
				if msg.SessionID != "" {
					actualSessionID = msg.SessionID
				}

				// Handle system messages
				if msg.Type == systemMsg {
					fmt.Fprintf(os.Stderr, "%s\n", style.SystemStyle.Render("🔄 "+msg.Content))
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
								// Update tool statistics
								toolStats.mu.Lock()
								toolStats.totalCalls++
								toolStats.activeCalls++
								toolStats.toolTypes[toolName]++
								toolStats.mu.Unlock()

								// Create new tool execution
								te := &toolExecution{
									name:       toolName,
									args:       toolArgs,
									msgID:      msg.MessageID,
									status:     "waiting",
									startTime:  time.Now(),
									runner:     connStatus.runner,
									toolCallId: msg.MessageID,
								}
								toolExecutions[msg.MessageID] = te
								toolsExecuted = true


								// Only show tool calls if flag is enabled
								if showToolCalls {
									// Enhanced tool execution header with better UX and stats
									fmt.Printf("\n")
									
									// Show tool statistics inline
									toolStats.mu.RLock()
									statsStr := fmt.Sprintf("[%d/%d tools]", toolStats.activeCalls, toolStats.totalCalls)
									if toolStats.activeCalls > 1 {
										statsStr = fmt.Sprintf("[+%d active tools]", toolStats.activeCalls)
									}
									toolStats.mu.RUnlock()
									
									fmt.Printf("%s\n", style.InfoBoxStyle.Render(
										fmt.Sprintf("🚀 %s %s %s", 
											style.ToolNameStyle.Render("EXECUTING"), 
											style.HighlightStyle.Render(toolName),
											style.ToolStatsStyle.Render(statsStr))))
									
									if toolArgs != "" {
										prettyArgs := toolArgs
										if json.Valid([]byte(toolArgs)) {
											var prettyJSON bytes.Buffer
											json.Indent(&prettyJSON, []byte(toolArgs), "  ", "  ")
											prettyArgs = prettyJSON.String()
										}
										fmt.Printf("%s\n", style.ToolArgsStyle.Render(fmt.Sprintf("📋 Parameters: %s", prettyArgs)))
									}
									
									// Show waiting indicator with animation and runner info
									fmt.Printf("%s\n", style.SpinnerStyle.Render(
										fmt.Sprintf("⏳ Initializing on runner://%s...", connStatus.runner)))
									fmt.Printf("%s\n", style.ToolDividerStyle.Render(strings.Repeat("─", 50)))
								}
							}
						}
					}

				case toolOutput:
					te := toolExecutions[msg.MessageID]
					if te != nil && !te.isComplete {
						// Mark that we received output
						te.hasOutput = true

						// Update status to running when we start receiving output
						if te.status == "waiting" {
							te.status = "running"
							// Update the status display only if showing tool calls
							if showToolCalls {
								fmt.Printf("\r%s\n", style.SpinnerStyle.Render("🔄 Processing tool execution..."))
							}
						}

						// Also check if we need to mark as complete based on content
						if strings.Contains(strings.ToLower(msg.Content), "completed") ||
							strings.Contains(strings.ToLower(msg.Content), "finished") ||
							strings.Contains(strings.ToLower(msg.Content), "done") {
							te.isComplete = true
						}

						// Store the full content
						te.output.WriteString(msg.Content)

						// Get the current content buffer for this message
						storedContent := messageBuffer[msg.MessageID]
						if storedContent == nil {
							storedContent = &chatBuffer{}
							messageBuffer[msg.MessageID] = storedContent
						}

						// Only process new content since last update
						newContent := msg.Content
						if len(storedContent.content) > 0 {
							newContent = msg.Content[len(storedContent.content):]
						}

						// Process new content if any
						trimmedContent := strings.TrimSpace(newContent)
						
						// Check for empty output scenarios that should show "output truncated"
						if trimmedContent == "" || trimmedContent == "\"\"" || trimmedContent == "{}" {
							te.outputTruncated = true
							anyOutputTruncated = true
							if showToolCalls {
								fmt.Printf("%s %s │ %s\n",
									style.ToolOutputPrefixStyle.Render("📋"),
									style.ToolNameStyle.Render(te.name),
									style.ToolOutputStyle.Render("output truncated"))
							}
						} else if trimmedContent != "" {
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
									hasError = true
									if showToolCalls {
										fmt.Printf("%s %s │ %s\n",
											style.ToolOutputPrefixStyle.Render("❌"),
											style.ToolNameStyle.Render(te.name),
											style.ErrorStyle.Render(outputData.Error))
									}
								}
								if outputData.Output != "" || outputData.Message != "" {
									output := outputData.Output
									if output == "" {
										output = outputData.Message
									}
									// Split multi-line outputs
									lines := strings.Split(output, "\n")
									for _, line := range lines {
										line = strings.TrimSpace(line)
										if line != "" && showToolCalls {
											fmt.Printf("%s %s │ %s\n",
												style.ToolOutputPrefixStyle.Render("│"),
												style.ToolNameStyle.Render(te.name),
												style.ToolOutputStyle.Render(line))
										}
									}
								}
							} else {
								// Handle plain text output - split by lines and process each
								lines := strings.Split(trimmedContent, "\n")
								for _, line := range lines {
									line = strings.TrimSpace(line)
									if line != "" {
										prefix := "│"
										outputStyle := style.ToolOutputStyle

										// Detect different types of messages
										lowerLine := strings.ToLower(line)
										switch {
										case strings.Contains(lowerLine, "error") || strings.Contains(lowerLine, "failed") || strings.Contains(lowerLine, "fail"):
											prefix = "❌"
											outputStyle = style.ErrorStyle
											te.failed = true
											hasError = true
										case strings.Contains(lowerLine, "warning") || strings.Contains(lowerLine, "warn"):
											prefix = "⚠️"
											outputStyle = style.WarningStyle
										case strings.Contains(lowerLine, "success") || strings.Contains(lowerLine, "completed") || strings.Contains(lowerLine, "done"):
											prefix = "✅"
											outputStyle = style.SuccessStyle
										case strings.Contains(lowerLine, "info") || strings.Contains(lowerLine, "information"):
											prefix = "ℹ️"
											outputStyle = style.ToolOutputStyle
										}

										if showToolCalls {
											fmt.Printf("%s %s │ %s\n",
												style.ToolOutputPrefixStyle.Render(prefix),
												style.ToolNameStyle.Render(te.name),
												outputStyle.Render(line))
										}
									}
								}
							}
						}

						// Update stored content
						storedContent.content = msg.Content
					}

				default:
					// Handle tool completion - check on any non-tool message type if we have pending tool executions
					for msgID, te := range toolExecutions {
						if te.hasOutput && !te.isComplete {
							// Mark as complete if we receive a final message or if enough time has passed
							if msg.Final || time.Since(te.startTime) > 2*time.Minute {
								te.isComplete = true
							} else if te.status == "running" && time.Since(te.startTime) > 30*time.Second {
								// If running for more than 30 seconds without completion, mark as complete
								te.isComplete = true
							}

							if te.isComplete {
								// Update tool statistics
								toolStats.mu.Lock()
								toolStats.activeCalls--
								if te.failed {
									toolStats.failedCalls++
								} else {
									toolStats.completedCalls++
								}
								toolStats.mu.Unlock()

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

								// Show updated statistics only if tool calls are enabled
								if showToolCalls {
									toolStats.mu.RLock()
									updatedStatsStr := fmt.Sprintf("[%d active, %d completed, %d failed]", 
										toolStats.activeCalls, toolStats.completedCalls, toolStats.failedCalls)
									toolStats.mu.RUnlock()

									// Print completion status with enhanced summary
									fmt.Printf("\n%s\n",
										style.InfoBoxStyle.Render(fmt.Sprintf("%s %s %s (%0.1fs) %s",
											statusEmoji,
											style.ToolNameStyle.Render(te.name),
											style.ToolCompleteStyle.Render(strings.ToUpper(completionStatus)),
											duration,
											style.ToolStatsStyle.Render(updatedStatsStr))))

									// Print error summary if failed
									if te.failed {
										fmt.Printf("%s %s\n",
											style.ToolOutputPrefixStyle.Render("⚠️"),
											style.ErrorStyle.Render("Tool encountered errors during execution"))
									}

									// Print output summary with better formatting
									if te.output.Len() > 0 {
										outputSize := te.output.Len()
										var sizeStr string
										if outputSize < 1024 {
											sizeStr = fmt.Sprintf("%d bytes", outputSize)
										} else if outputSize < 1024*1024 {
											sizeStr = fmt.Sprintf("%.1f KB", float64(outputSize)/1024)
										} else {
											sizeStr = fmt.Sprintf("%.1f MB", float64(outputSize)/(1024*1024))
										}
										
										outputSummary := fmt.Sprintf("Output: %s on runner://%s", sizeStr, te.runner)
										if te.outputTruncated {
											outputSummary += " (output was truncated)"
										}
										
										fmt.Printf("%s %s\n",
											style.ToolOutputPrefixStyle.Render("📊"),
											style.ToolSummaryStyle.Render(outputSummary))
									} else if te.outputTruncated {
										fmt.Printf("%s %s\n",
											style.ToolOutputPrefixStyle.Render("📋"),
											style.ToolSummaryStyle.Render(fmt.Sprintf("Output was truncated on runner://%s", te.runner)))
									}
									
									// Show KUBIYA_DEBUG recommendation if output was truncated
									if te.outputTruncated {
										fmt.Printf("%s %s\n",
											style.ToolOutputPrefixStyle.Render("💡"),
											style.WarningStyle.Render("Tip: Set KUBIYA_DEBUG=1 environment variable to see full tool outputs"))
									}
									fmt.Printf("%s\n", style.ToolDividerStyle.Render(strings.Repeat("─", 50)))
								}
								delete(toolExecutions, msgID)
							}
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
								fmt.Printf("%s\n",
									style.AgentStyle.Render(remaining))
							}
							// Also handle any remaining code block
							if buf.codeBlock.Len() > 0 {
								fmt.Printf("%s\n%s\n%s\n",
									style.CodeBlockStyle.Render("```"),
									style.CodeBlockStyle.Render(buf.codeBlock.String()),
									style.CodeBlockStyle.Render("```"))
							}
						}
						// Add final completion message to ensure stream end is visible
						if msg.Type == "completion" && msg.FinishReason != "" {
							if debug {
								fmt.Printf("\n[STREAM COMPLETE] Reason: %s\n", msg.FinishReason)
							}
						}
						fmt.Println()
					}
				}
			}

			if !stream {
				fmt.Println(finalResponse.String())
			}

			// Handle follow-up for non-interactive sessions without tool execution (skip for inline agents)
			if !interactive && !toolsExecuted && !hasError && completionReason != "error" && !inline {
				if debug {
					fmt.Printf("🔄 No tools executed, sending follow-up prompt\n")
				}

				followUpMsg := "You didn't seem to execute anything. You're running in a non-interactive session and the user confirms to execute read-only operations. EXECUTE RIGHT AWAY!"

				// Send follow-up message
				followUpChan, err := client.SendMessageWithContext(cmd.Context(), agentID, followUpMsg, actualSessionID, map[string]string{})
				if err != nil {
					if debug {
						fmt.Printf("⚠️ Failed to send follow-up message: %v\n", err)
					}
				} else {
					fmt.Printf("\n%s\n", style.InfoBoxStyle.Render("🔄 Following up to ensure execution..."))

					// Process follow-up response
					for msg := range followUpChan {
						if msg.Error != "" {
							fmt.Fprintf(os.Stderr, "%s\n", style.ErrorStyle.Render("❌ Error: "+msg.Error))
							hasError = true
							break
						}
						if msg.SessionID != "" {
							actualSessionID = msg.SessionID
						}
						// Handle follow-up messages similar to main processing
						if msg.Type == "tool" {
							toolsExecuted = true
						}
					}
				}
			}

			// Show session continuation message
			if !interactive && actualSessionID != "" {
				fmt.Printf("\n%s\n", style.InfoBoxStyle.Render("💬 To continue this conversation, run:"))

				// Include agent name in the continuation command if available
				var continuationCmd string
				if agentName != "" {
					continuationCmd = fmt.Sprintf("kubiya chat -n %s --session %s -m \"your message here\"", agentName, actualSessionID)
				} else if agentID != "" {
					continuationCmd = fmt.Sprintf("kubiya chat -t %s --session %s -m \"your message here\"", agentID, actualSessionID)
				} else {
					continuationCmd = fmt.Sprintf("kubiya chat --session %s -m \"your message here\"", actualSessionID)
				}

				fmt.Printf("%s\n", style.HighlightStyle.Render(continuationCmd))
				fmt.Println()
			}
			
			// Show KUBIYA_DEBUG recommendation if any output was truncated
			if anyOutputTruncated && !interactive {
				fmt.Printf("\n%s\n", style.InfoBoxStyle.Render("💡 Tool Output Visibility"))
				fmt.Printf("%s\n", style.WarningStyle.Render("Some tool outputs were truncated. To see full tool outputs, set:"))
				fmt.Printf("%s\n", style.HighlightStyle.Render("export KUBIYA_DEBUG=1"))
				fmt.Printf("%s\n", style.WarningStyle.Render("Then re-run your command to see detailed tool execution logs."))
				fmt.Println()
			}

			// Handle completion status and exit codes
			if debug {
				fmt.Printf("🔍 Completion reason: %s, hasError: %v, toolsExecuted: %v\n", completionReason, hasError, toolsExecuted)
			}

			// Return proper exit code based on completion status
			if hasError {
				if debug {
					fmt.Printf("🚨 Exiting with error code 1 due to tool failures\n")
				}
				return fmt.Errorf("agent execution completed with errors")
			}

			// Check for non-successful completion reasons
			switch completionReason {
			case "error":
				if debug {
					fmt.Printf("🚨 Exiting with error code 1 due to completion reason: %s\n", completionReason)
				}
				return fmt.Errorf("agent execution failed with reason: %s", completionReason)
			case "stop", "length", "":
				// Normal successful completion
				if debug {
					fmt.Printf("✅ Exiting with success code 0\n")
				}
				return nil
			default:
				// Unknown completion reason - log but don't fail
				if debug {
					fmt.Printf("⚠️ Unknown completion reason: %s, treating as success\n", completionReason)
				}
				return nil
			}
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
	cmd.Flags().StringVar(&permissionLevel, "permission-level", "read", "Permission level for tool execution (read, readwrite, ask)")
	cmd.Flags().BoolVar(&showToolCalls, "show-tool-calls", true, "Show tool call execution details")
	cmd.Flags().IntVar(&retries, "retries", 3, "Number of retries for stream errors")
	
	// Inline agent flags
	cmd.Flags().BoolVar(&inline, "inline", false, "Use inline agent mode")
	cmd.Flags().StringVar(&toolsFile, "tools-file", "", "JSON file containing tools definition")
	cmd.Flags().StringVar(&toolsJSON, "tools-json", "", "JSON string containing tools definition")
	cmd.Flags().StringVar(&aiInstructions, "ai-instructions", "", "AI instructions for the inline agent")
	cmd.Flags().StringVar(&description, "description", "", "Description for the inline agent")
	cmd.Flags().StringArrayVar(&runners, "runners", []string{}, "Runners for the inline agent")
	cmd.Flags().StringArrayVar(&integrations, "integrations", []string{}, "Integrations for the inline agent")
	cmd.Flags().StringArrayVar(&secrets, "secrets", []string{}, "Secrets for the inline agent")
	cmd.Flags().StringArrayVar(&envVars, "env-vars", []string{}, "Environment variables for the inline agent (KEY=VALUE format)")
	cmd.Flags().StringVar(&llmModel, "llm-model", "", "LLM model for the inline agent")
	cmd.Flags().BoolVar(&isDebugMode, "debug-mode", false, "Enable debug mode for the inline agent")

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
