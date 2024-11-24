package cli

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"sort"
	"strings"
	"text/tabwriter"
	"time"

	"github.com/charmbracelet/lipgloss"
	"github.com/kubiyabot/cli/internal/config"
	"github.com/kubiyabot/cli/internal/kubiya"
	"github.com/spf13/cobra"
)

var (
	titleStyle = lipgloss.NewStyle().
			Bold(true).
			Foreground(lipgloss.Color("#FAFAFA")).
			Background(lipgloss.Color("#7D56F4")).
			Padding(0, 1)

	subtitleStyle = lipgloss.NewStyle().
			Bold(true).
			Foreground(lipgloss.Color("#7D56F4"))

	highlightStyle = lipgloss.NewStyle().
			Foreground(lipgloss.Color("#7D56F4")).
			Bold(true)

	dimStyle = lipgloss.NewStyle().
			Foreground(lipgloss.Color("#666666"))
)

func newToolsCommand(cfg *config.Config) *cobra.Command {
	cmd := &cobra.Command{
		Use:     "tool",
		Aliases: []string{"tools"},
		Short:   "🛠️  Manage tools",
		Long:    `Search, list, and describe tools across all sources.`,
	}

	cmd.AddCommand(
		newListToolsCommand(cfg),
		newSearchToolsCommand(cfg),
		newDescribeToolCommand(cfg),
	)

	return cmd
}

func newListToolsCommand(cfg *config.Config) *cobra.Command {
	var (
		outputFormat string
		sourceUUID   string
	)

	cmd := &cobra.Command{
		Use:   "list",
		Short: "📋 List tools",
		Example: `  # List all tools
  kubiya tool list

  # List tools from a specific source
  kubiya tool list --source abc-123

  # Output in JSON format
  kubiya tool list --output json`,
		RunE: func(cmd *cobra.Command, args []string) error {
			client := kubiya.NewClient(cfg)

			var tools []kubiya.Tool
			if sourceUUID != "" {
				// Get tools from specific source
				source, err := client.GetSourceMetadata(cmd.Context(), sourceUUID)
				if err != nil {
					return err
				}
				tools = source.Tools
			} else {
				// Get tools from all sources
				sources, err := client.ListSources(cmd.Context())
				if err != nil {
					return err
				}

				for _, source := range sources {
					metadata, err := client.GetSourceMetadata(cmd.Context(), source.UUID)
					if err != nil {
						continue
					}
					tools = append(tools, metadata.Tools...)
				}
			}

			switch outputFormat {
			case "json":
				return json.NewEncoder(os.Stdout).Encode(tools)
			case "text":
				if len(tools) == 0 {
					fmt.Println("No tools found")
					return nil
				}

				fmt.Printf("\n%s\n\n", titleStyle.Render(" 🛠️  Tools "))

				w := tabwriter.NewWriter(os.Stdout, 0, 0, 2, ' ', 0)
				for _, tool := range tools {
					// Tool name and description
					fmt.Fprintf(w, "%s\n", highlightStyle.Render(tool.Name))
					if tool.Description != "" {
						fmt.Fprintf(w, "  %s\n", tool.Description)
					}

					// Arguments section
					if len(tool.Args) > 0 {
						fmt.Fprintf(w, "  %s:\n", subtitleStyle.Render("Arguments"))
						for _, arg := range tool.Args {
							required := dimStyle.Render("optional")
							if arg.Required {
								required = highlightStyle.Render("required")
							}
							fmt.Fprintf(w, "    • %s: %s (%s)\n",
								highlightStyle.Render(arg.Name),
								arg.Description,
								required,
							)
						}
					}

					// Environment variables section
					if len(tool.Env) > 0 {
						fmt.Fprintf(w, "  %s:\n", subtitleStyle.Render("Environment"))
						for _, env := range tool.Env {
							fmt.Fprintf(w, "    • %s\n", env)
						}
					}

					fmt.Fprintln(w, "")
				}
				return w.Flush()
			default:
				return fmt.Errorf("unknown output format: %s", outputFormat)
			}
		},
	}

	cmd.Flags().StringVarP(&outputFormat, "output", "o", "text", "Output format (text|json)")
	cmd.Flags().StringVarP(&sourceUUID, "source", "s", "", "Source UUID to list tools from")
	return cmd
}

func newSearchToolsCommand(cfg *config.Config) *cobra.Command {
	var outputFormat string

	cmd := &cobra.Command{
		Use:   "search [query]",
		Short: "🔍 Search for tools",
		Example: `  # Search for tools by name or description
  kubiya tool search kubernetes

  # Search with JSON output
  kubiya tool search deploy --output json`,
		Args: cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			client := kubiya.NewClient(cfg)
			sources, err := client.ListSources(cmd.Context())
			if err != nil {
				return err
			}

			query := strings.ToLower(args[0])
			type searchResult struct {
				Tool   kubiya.Tool
				Source kubiya.Source
				Error  error
			}

			// Create channels
			resultChan := make(chan searchResult)
			done := make(chan struct{})
			sem := make(chan struct{}, 3) // Limit concurrent requests

			// Start search goroutines
			var activeSearches int
			for _, source := range sources {
				// Skip sources that don't match query in name/description
				if !strings.Contains(strings.ToLower(source.Name), query) &&
					!strings.Contains(strings.ToLower(source.Description), query) {
					continue
				}

				sem <- struct{}{} // Acquire semaphore
				activeSearches++

				go func(s kubiya.Source) {
					defer func() {
						<-sem // Release semaphore
						resultChan <- searchResult{Source: s, Error: fmt.Errorf("done")}
					}()

					// Set timeout for each source search
					ctx, cancel := context.WithTimeout(cmd.Context(), 10*time.Second)
					defer cancel()

					metadata, err := client.GetSourceMetadata(ctx, s.UUID)
					if err != nil {
						resultChan <- searchResult{Source: s, Error: err}
						return
					}

					// Send matching tools
					for _, tool := range metadata.Tools {
						if strings.Contains(strings.ToLower(tool.Name), query) ||
							strings.Contains(strings.ToLower(tool.Description), query) {
							resultChan <- searchResult{Tool: tool, Source: s}
						}
					}
				}(source)
			}

			if activeSearches == 0 {
				fmt.Printf("No sources found matching '%s'\n", args[0])
				return nil
			}

			// Collect results with timeout
			var matches []struct {
				Tool   kubiya.Tool
				Source kubiya.Source
			}
			var errors []string
			completed := 0

			// Show progress
			spinner := []string{"⠋", "⠙", "⠹", "⠸", "⠼", "⠴", "⠦", "⠧", "⠇", "⠏"}
			spinnerIdx := 0
			lastUpdate := time.Now()

			// Start result collection in goroutine
			go func() {
				for completed < activeSearches {
					select {
					case result := <-resultChan:
						if result.Error != nil {
							if result.Error.Error() != "done" {
								errors = append(errors, fmt.Sprintf("%s: %v", result.Source.Name, result.Error))
							}
							completed++
						} else {
							matches = append(matches, struct {
								Tool   kubiya.Tool
								Source kubiya.Source
							}{result.Tool, result.Source})
						}
					}
				}
				close(done)
			}()

			// Show progress with timeout
			fmt.Printf("🔍 Searching for '%s' ", args[0])
			timeout := time.After(30 * time.Second)
			for {
				select {
				case <-done:
					fmt.Printf("\r\033[K") // Clear line
					goto DONE
				case <-timeout:
					fmt.Printf("\r\033[K") // Clear line
					return fmt.Errorf("search timed out after 30 seconds")
				case <-time.After(100 * time.Millisecond):
					if time.Since(lastUpdate) >= 100*time.Millisecond {
						fmt.Printf("\r\033[K🔍 Searching for '%s' %s", args[0], spinner[spinnerIdx])
						spinnerIdx = (spinnerIdx + 1) % len(spinner)
						lastUpdate = time.Now()
					}
				}
			}

		DONE:
			// Show any errors that occurred during search
			if len(errors) > 0 {
				fmt.Printf("\n⚠️  Some sources had errors:\n")
				for _, err := range errors {
					fmt.Printf("  • %s\n", err)
				}
				fmt.Println()
			}

			// Output results
			switch outputFormat {
			case "json":
				return json.NewEncoder(os.Stdout).Encode(matches)
			case "text":
				if len(matches) == 0 {
					fmt.Printf("No tools found matching '%s'\n", args[0])
					return nil
				}

				// Sort results
				sort.Slice(matches, func(i, j int) bool {
					if matches[i].Source.Name == matches[j].Source.Name {
						return matches[i].Tool.Name < matches[j].Tool.Name
					}
					return matches[i].Source.Name < matches[j].Source.Name
				})

				// Display results
				fmt.Printf("\n%s\n\n", titleStyle.Render(fmt.Sprintf(" Found %d Tools ", len(matches))))

				w := tabwriter.NewWriter(os.Stdout, 0, 0, 2, ' ', 0)
				currentSource := ""
				for _, match := range matches {
					if currentSource != match.Source.Name {
						currentSource = match.Source.Name
						fmt.Fprintf(w, "\n%s\n", subtitleStyle.Render(fmt.Sprintf("📦 %s", match.Source.Name)))
					}

					fmt.Fprintf(w, "  %s\n", highlightStyle.Render(match.Tool.Name))
					if match.Tool.Description != "" {
						fmt.Fprintf(w, "    %s\n", match.Tool.Description)
					}

					if len(match.Tool.Args) > 0 {
						reqCount := countRequiredArgs(match.Tool.Args)
						fmt.Fprintf(w, "    %s: %d (%d required)\n",
							dimStyle.Render("Arguments"),
							len(match.Tool.Args),
							reqCount,
						)
					}

					fmt.Fprintln(w, "")
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

func newDescribeToolCommand(cfg *config.Config) *cobra.Command {
	var (
		outputFormat string
		sourceUUID   string
	)

	cmd := &cobra.Command{
		Use:   "describe [tool-name]",
		Short: "📖 Show detailed information about a tool",
		Example: `  # Describe a tool
  kubiya tool describe deploy-app

  # Describe a tool from a specific source
  kubiya tool describe deploy-app --source abc-123`,
		Args: cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			client := kubiya.NewClient(cfg)
			toolName := args[0]

			var tool *kubiya.Tool
			var sourceName string

			if sourceUUID != "" {
				// Get tool from specific source
				source, err := client.GetSourceMetadata(cmd.Context(), sourceUUID)
				if err != nil {
					return err
				}
				for _, t := range source.Tools {
					if t.Name == toolName {
						tool = &t
						sourceName = source.Name
						break
					}
				}
			} else {
				// Search all sources
				sources, err := client.ListSources(cmd.Context())
				if err != nil {
					return err
				}

				for _, source := range sources {
					metadata, err := client.GetSourceMetadata(cmd.Context(), source.UUID)
					if err != nil {
						continue
					}
					for _, t := range metadata.Tools {
						if t.Name == toolName {
							tool = &t
							sourceName = source.Name
							break
						}
					}
					if tool != nil {
						break
					}
				}
			}

			if tool == nil {
				return fmt.Errorf("tool '%s' not found", toolName)
			}

			switch outputFormat {
			case "json":
				return json.NewEncoder(os.Stdout).Encode(tool)
			case "text":
				fmt.Printf("\n%s\n\n", titleStyle.Render(fmt.Sprintf(" 🛠️  Tool: %s ", tool.Name)))
				fmt.Printf("%s %s\n\n", subtitleStyle.Render("Source:"), sourceName)

				if tool.Description != "" {
					fmt.Printf("%s\n%s\n\n", subtitleStyle.Render("Description:"), tool.Description)
				}

				if len(tool.Args) > 0 {
					fmt.Printf("%s\n", subtitleStyle.Render("Arguments:"))
					w := tabwriter.NewWriter(os.Stdout, 0, 0, 2, ' ', 0)
					for _, arg := range tool.Args {
						required := dimStyle.Render("optional")
						if arg.Required {
							required = highlightStyle.Render("required")
						}
						fmt.Fprintf(w, "  • %s\t%s\t(%s)\n",
							highlightStyle.Render(arg.Name),
							arg.Description,
							required,
						)
					}
					w.Flush()
					fmt.Println()
				}

				if len(tool.Env) > 0 {
					fmt.Printf("%s\n", subtitleStyle.Render("Environment Variables:"))
					for _, env := range tool.Env {
						fmt.Printf("  • %s\n", env)
					}
					fmt.Println()
				}

				if tool.LongRunning {
					fmt.Printf("%s\n%s\n\n",
						subtitleStyle.Render("Execution:"),
						"⏳ This is a long-running task",
					)
				}

				fmt.Printf("To execute this tool:\n")
				fmt.Printf("  %s\n", highlightStyle.Render(fmt.Sprintf("kubiya browse")))
				fmt.Printf("  %s\n", highlightStyle.Render(fmt.Sprintf("kubiya source interactive")))
				return nil
			default:
				return fmt.Errorf("unknown output format: %s", outputFormat)
			}
		},
	}

	cmd.Flags().StringVarP(&outputFormat, "output", "o", "text", "Output format (text|json)")
	cmd.Flags().StringVarP(&sourceUUID, "source", "s", "", "Source UUID to search in")
	return cmd
}

func countRequiredArgs(args []kubiya.ToolArg) int {
	count := 0
	for _, arg := range args {
		if arg.Required {
			count++
		}
	}
	return count
}
