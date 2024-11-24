package cli

import (
	"fmt"
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
	)

	cmd := &cobra.Command{
		Use:   "chat",
		Short: "💬 Chat with a teammate",
		Long: `Start a chat session with a Kubiya teammate.
You can either use interactive mode or specify a teammate and message directly.`,
		Example: `  # Interactive chat mode (recommended)
  kubiya chat --interactive
  kubiya chat -i

  # Non-interactive mode (for scripts)
  kubiya chat -n "security" -m "Review permissions"`,
		RunE: func(cmd *cobra.Command, args []string) error {
			// If interactive mode is requested, start the TUI
			if interactive {
				app := tui.New(cfg)
				return app.Run()
			}

			client := kubiya.NewClient(cfg)

			// Validate input
			if teammateID == "" && teammateName == "" {
				return fmt.Errorf("either --id or --name is required in non-interactive mode")
			}
			if message == "" {
				return fmt.Errorf("message is required in non-interactive mode")
			}

			// If teammate ID is not provided, search by name
			if teammateID == "" {
				teammates, err := client.ListTeammates(cmd.Context())
				if err != nil {
					return err
				}

				// Find matching teammates
				var matches []kubiya.Teammate
				searchName := strings.ToLower(teammateName)
				for _, teammate := range teammates {
					if strings.Contains(strings.ToLower(teammate.Name), searchName) {
						matches = append(matches, teammate)
					}
				}

				switch len(matches) {
				case 0:
					return fmt.Errorf("no teammates found matching name: %s", teammateName)
				case 1:
					teammateID = matches[0].UUID
					if !interactive {
						fmt.Printf("👋 Found teammate: %s (%s)\n", matches[0].Name, matches[0].UUID)
					}
				default:
					if interactive {
						return fmt.Errorf("multiple teammates found matching '%s'. Please use --id or provide a more specific name", teammateName)
					}

					fmt.Println("👥 Multiple teammates found:")
					for i, teammate := range matches {
						status := "🟢"
						if teammate.AIInstructions != "" {
							status = "🌟"
						}
						fmt.Printf("\n%d. %s %s\n", i+1, status, teammate.Name)
						if teammate.Desc != "" {
							fmt.Printf("   %s\n", teammate.Desc)
						}
						if teammate.AIInstructions != "" {
							fmt.Printf("   Special skills: %s\n", teammate.AIInstructions)
						}
					}

					var choice int
					fmt.Print("\nSelect a teammate (1-" + fmt.Sprint(len(matches)) + "): ")
					_, err := fmt.Scanf("%d", &choice)
					if err != nil || choice < 1 || choice > len(matches) {
						return fmt.Errorf("invalid selection")
					}

					teammateID = matches[choice-1].UUID
					fmt.Printf("\n👋 Selected: %s\n", matches[choice-1].Name)
				}
			}

			// Send the message
			fmt.Printf("\n💭 Sending message...\n")
			resp, err := client.SendMessage(cmd.Context(), teammateID, message)
			if err != nil {
				return err
			}

			if interactive {
				fmt.Println(resp.Content)
			} else {
				fmt.Printf("\n💬 Response:\n%s\n", resp.Content)
			}

			return nil
		},
	}

	cmd.Flags().StringVarP(&teammateID, "id", "i", "", "Teammate ID")
	cmd.Flags().StringVarP(&teammateName, "name", "n", "", "Teammate name")
	cmd.Flags().StringVarP(&message, "message", "m", "", "Message to send")
	cmd.Flags().BoolVarP(&interactive, "interactive", "t", false, "Start interactive chat mode")

	return cmd
}
