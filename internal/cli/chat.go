package cli

import (
	"fmt"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/kubiyabot/cli/internal/config"
	"github.com/kubiyabot/cli/internal/tui"
	"github.com/spf13/cobra"
)

func newChatCommand(cfg *config.Config) *cobra.Command {
	var (
		teammateID string

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
				// Start with teammate selection model
				selectionModel, err := tui.NewTeammateSelectionModel(cfg)
				if err != nil {
					return err
				}

				p := tea.NewProgram(selectionModel, tea.WithAltScreen())
				model, err := p.Run()
				if err != nil {
					return err
				}

				// Get the selected teammate from the model
				if selectionModel, ok := model.(*tui.TeammateSelectionModel); ok {
					selected := selectionModel.Selected()
					if selected.UUID == "" {
						return fmt.Errorf("no teammate selected")
					}

					// Start chat with selected teammate
					chatModel, err := tui.NewChatModel(cfg, selected)
					if err != nil {
						return err
					}

					// Start new program with alt screen for chat
					p = tea.NewProgram(chatModel, tea.WithAltScreen())
					return p.Start()
				}
				return fmt.Errorf("unexpected model type")
			}

			// Non-interactive mode
			chatModel, err := tui.NewChatModel(cfg)
			if err != nil {
				return err
			}

			p := tea.NewProgram(chatModel, tea.WithAltScreen())
			return p.Start()
		},
	}

	cmd.Flags().StringVarP(&teammateID, "id", "", "", "Teammate ID")
	cmd.Flags().StringVarP(&teammateName, "name", "n", "", "Teammate name")
	cmd.Flags().StringVarP(&message, "message", "m", "", "Message to send")
	cmd.Flags().BoolVarP(&interactive, "interactive", "i", false, "Start interactive chat mode")
	cmd.Flags().BoolVar(&debug, "debug", false, "Enable debug mode")

	return cmd
}
