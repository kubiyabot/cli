package cli

import (
	"bufio"
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"strings"

	"github.com/kubiyabot/cli/internal/config"
	"github.com/kubiyabot/cli/internal/kubiya"
	"github.com/kubiyabot/cli/internal/tui"
	"github.com/spf13/cobra"
)

func newChatCommand(cfg *config.Config) *cobra.Command {
	var (
		teammateID   string
		teammateName string
		message      string
		interactive  bool
		debug        bool
		stream       bool
		clearSession bool
		sessionID    string
		contextFiles []string
		stdinInput   bool
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
		Short: "💬 Chat with a teammate",
		Long: `Start a chat session with a Kubiya teammate.
You can either use interactive mode, specify a message directly, or pipe input from stdin.
Use --context to include additional files for context (supports wildcards and URLs).`,
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
  cat error.log | kubiya chat -n "debug" --stdin --context "config/*.yaml"`,
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

			// If teammate name is provided, look up the ID
			if teammateName != "" && teammateID == "" {
				client := kubiya.NewClient(cfg)
				teammates, err := client.ListTeammates(cmd.Context())
				if err != nil {
					return fmt.Errorf("failed to list teammates: %w", err)
				}

				for _, t := range teammates {
					if t.Name == teammateName {
						teammateID = t.UUID
						break
					}
				}

				if teammateID == "" {
					return fmt.Errorf("teammate not found: %s", teammateName)
				}
			}

			// Ensure we have a teammate ID
			if teammateID == "" {
				return fmt.Errorf("teammate ID or name is required")
			}

			// Send message with context
			client := kubiya.NewClient(cfg)
			msgChan, err := client.SendMessageWithContext(cmd.Context(), teammateID, message, sessionID, context)
			if err != nil {
				return err
			}

			// Read messages and handle session ID
			var finalResponse strings.Builder
			var lastContent string

			for msg := range msgChan {
				if msg.Error != "" {
					return fmt.Errorf("error from server: %s", msg.Error)
				}

				if stream {
					fmt.Printf("%s", msg.Content[len(lastContent):])
				} else {
					newContent := msg.Content[len(lastContent):]
					finalResponse.WriteString(newContent)
				}

				lastContent = msg.Content

				// Save session ID if autoSession is enabled
				if cfg.AutoSession && sessionID == "" && msg.SessionID != "" {
					sessionID = msg.SessionID
					if err := os.WriteFile(sessionFile, []byte(sessionID), 0644); err != nil {
						return fmt.Errorf("failed to save session ID: %w", err)
					}
				}
			}

			if !stream {
				fmt.Println(finalResponse.String())
			}

			return nil
		},
	}

	cmd.Flags().StringVarP(&teammateID, "id", "", "", "Teammate ID")
	cmd.Flags().StringVarP(&teammateName, "name", "n", "", "Teammate name")
	cmd.Flags().StringVarP(&message, "message", "m", "", "Message to send")
	cmd.Flags().BoolVarP(&interactive, "interactive", "i", false, "Start interactive chat mode")
	cmd.Flags().BoolVar(&debug, "debug", false, "Enable debug mode")
	cmd.Flags().BoolVar(&stream, "stream", false, "Stream assistant's response as it is received")
	cmd.Flags().BoolVar(&clearSession, "clear-session", false, "Clear stored session ID")
	cmd.Flags().StringVarP(&sessionID, "session-id", "s", "", "Session ID to continue conversation")
	cmd.Flags().StringArrayVar(&contextFiles, "context", []string{}, "Files, wildcards, or URLs to include as context (can be specified multiple times)")
	cmd.Flags().BoolVar(&stdinInput, "stdin", false, "Read message from stdin")

	return cmd
}
