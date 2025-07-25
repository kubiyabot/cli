package cli

import (
	"fmt"
	"strings"

	"github.com/charmbracelet/glamour"
	"github.com/kubiyabot/cli/internal/config"
	"github.com/kubiyabot/cli/internal/kubiya"
	"github.com/spf13/cobra"
)

func newQueryDocumentationCommand(cfg *config.Config) *cobra.Command {
	cmd := &cobra.Command{
		Use:     "query [prompt]",
		Aliases: []string{"q", "search"},
		Short:   "🔍 Query the knowledge base",
		Long:    `Query the Kubiya documentation for information on commands, features, and usage.`,
		Args:    cobra.MinimumNArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			// Join all arguments as the prompt
			prompt := strings.Join(args, " ")

			trieve, err := kubiya.GetTrieveConfig()
			cobra.CheckErr(err)
			resp, err := trieve.SearchDocumentationByGroup(prompt)
			cobra.CheckErr(err)

			for _, group := range resp.Results {
				for _, chunk := range group.Chuncks {
					// print the chunk title
					title := fmt.Sprintf("# %s", chunk.Chunk.StringTitle())
					out, _ := glamour.Render(title, "dracula")
					fmt.Print(out)

					// print the chunk body
					style := "dracula"
					out, err = glamour.Render(chunk.Chunk.String(), style)
					cobra.CheckErr(err)
					fmt.Print(out)
				}
			}
			return nil
		},
	}

	return cmd
}

func newDocumentationCommand(cfg *config.Config) *cobra.Command {
	cmd := &cobra.Command{
		Use:     "documentation",
		Aliases: []string{"doc", "docs"},
		Short:   "🔍 Query the central knowledge base",
		Long:    `Query the central knowledge base for contextual information with intelligent search capabilities.`,
	}

	cmd.AddCommand(
		newQueryDocumentationCommand(cfg),
	)

	return cmd
}
