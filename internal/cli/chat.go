package cli

import (
	"fmt"

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
	)

	cmd := &cobra.Command{
		Use:   "chat",
		Short: "💬 Chat with a teammate",
		Long: `Start a chat session with a Kubiya teammate.
You can either use interactive mode or specify a teammate and message directly.`,
		Example: `  # Interactive chat mode (recommended)
  kubiya chat --interactive
  kubiya chat -i

  # Debug mode
  kubiya chat -i --debug

  # Non-interactive mode (for scripts)
  kubiya chat -n "security" -m "Review permissions"`,
		RunE: func(cmd *cobra.Command, args []string) error {
			cfg.Debug = debug

			if interactive {
				chatUI := tui.NewChatUI(cfg)
				return chatUI.Run()
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
			msgChan, err := client.SendMessage(cmd.Context(), teammateID, message, "")
			if err != nil {
				return err
			}

			// Print messages as they arrive
			for msg := range msgChan {
				if msg.Error != "" {
					return fmt.Errorf("error from server: %s", msg.Error)
				}
				fmt.Printf("%s\n", msg.Content)
			}
			return nil
		},
	}

	cmd.Flags().StringVarP(&teammateID, "id", "", "", "Teammate ID")
	cmd.Flags().StringVarP(&teammateName, "name", "n", "", "Teammate name")
	cmd.Flags().StringVarP(&message, "message", "m", "", "Message to send")
	cmd.Flags().BoolVarP(&interactive, "interactive", "i", false, "Start interactive chat mode")
	cmd.Flags().BoolVar(&debug, "debug", false, "Enable debug mode")

	return cmd
}
