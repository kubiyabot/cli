package cli

import (
	"fmt"
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
	)

	cmd := &cobra.Command{
		Use:   "chat",
		Short: "💬 Chat with a teammate",
		Long: `Start a chat session with a Kubiya teammate.
You can either use interactive mode or specify a teammate and message directly.`,
		Example: `  # Interactive chat mode (recommended)
  kubiya chat --interactive
  kubiya chat -i

  # Non-interactive mode with session management
  kubiya chat -n "security" -m "Review permissions"

  # Continue a previous session
  kubiya chat -n "security" -m "Continue our discussion"

  # Stream assistant's response
  kubiya chat -n "security" -m "Review permissions" --stream

  # Clear stored session
  kubiya chat --clear-session`,
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

			// Load last session ID if autoSession is enabled and sessionID is not provided
			if sessionID == "" && cfg.AutoSession {
				if data, err := os.ReadFile(sessionFile); err == nil {
					sessionID = string(data)
					fmt.Printf("Resuming session ID: %s\n", sessionID)
				}
			}

			// Non-interactive mode
			if message == "" {
				return fmt.Errorf("message is required in non-interactive mode")
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

			// Send message
			client := kubiya.NewClient(cfg)
			msgChan, err := client.SendMessage(cmd.Context(), teammateID, message, sessionID)
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

	return cmd
}
