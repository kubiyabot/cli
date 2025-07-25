package cli

import (
	"encoding/json"
	"fmt"
	"os"
	"text/tabwriter"

	"github.com/kubiyabot/cli/internal/config"
	"github.com/kubiyabot/cli/internal/kubiya"
	"github.com/spf13/cobra"
)

func newListCommand(cfg *config.Config) *cobra.Command {
	var outputFormat string

	cmd := &cobra.Command{
		Use:     "list",
		Short:   "👥 List available agents",
		Example: "  kubiya list\n  kubiya list --output json",
		RunE: func(cmd *cobra.Command, args []string) error {
			client := kubiya.NewClient(cfg)
			agents, err := client.ListAgents(cmd.Context())
			if err != nil {
				return err
			}

			switch outputFormat {
			case "json":
				return json.NewEncoder(os.Stdout).Encode(agents)
			case "text":
				w := tabwriter.NewWriter(os.Stdout, 0, 0, 2, ' ', 0)
				fmt.Fprintln(w, "👥 AGENTS")
				fmt.Fprintln(w, "UUID\tNAME\tSTATUS\tDESCRIPTION")
				for _, t := range agents {
					status := "🟢"
					if t.AIInstructions != "" {
						status = "🌟"
					}
					fmt.Fprintf(w, "%s\t%s\t%s\t%s\n",
						t.UUID,
						t.Name,
						status,
						t.Description,
					)
				}
				return w.Flush()
			default:
				return fmt.Errorf("unknown output format: %s", outputFormat)
			}
		},
	}

	cmd.Flags().StringVarP(&outputFormat, "output", "o", "text", "Output format (text|json)")
	return cmd
}
