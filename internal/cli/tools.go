package cli

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"os/signal"
	"sort"
	"strings"
	"text/tabwriter"
	"time"

	"github.com/kubiyabot/cli/internal/config"
	"github.com/kubiyabot/cli/internal/kubiya"
	"github.com/kubiyabot/cli/internal/style"
	"github.com/spf13/cobra"
)

var envIntegrationIcons = map[string]struct {
	icon  string
	label string
}{
	"AWS":        {"☁️", "AWS integration"},
	"SLACK":      {"💬", "Slack integration"},
	"KUBERNETES": {"⎈", "Kubernetes integration"},
	"GITHUB":     {"🐙", "GitHub integration"},
	"GITLAB":     {"🦊", "GitLab integration"},
	"JIRA":       {"📋", "Jira integration"},
	"DATADOG":    {"🐕", "Datadog integration"},
	"PROMETHEUS": {"📊", "Prometheus integration"},
	"VAULT":      {"🔒", "Vault integration"},
	"JENKINS":    {"🔧", "Jenkins integration"},
	"TERRAFORM":  {"🏗️", "Terraform integration"},
	"AZURE":      {"☁️", "Azure integration"},
	"GCP":        {"☁️", "GCP integration"},
	"BITBUCKET":  {"🪣", "Bitbucket integration"},
	"SERVICENOW": {"🔧", "ServiceNow integration"},
	"PAGERDUTY":  {"🚨", "PagerDuty integration"},
}

func getEnvIntegration(env string) (string, string, bool) {
	for prefix, integration := range envIntegrationIcons {
		if strings.HasPrefix(env, prefix+"_") {
			return integration.icon, integration.label, true
		}
	}
	return "", "", false
}

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
		newExecuteCommand(cfg),
		newGenerateToolCommand(cfg),
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

				fmt.Printf("\n%s\n\n", style.TitleStyle.Render(" 🛠️  Tools "))

				w := tabwriter.NewWriter(os.Stdout, 0, 0, 2, ' ', 0)
				for _, tool := range tools {
					// Tool name and description
					fmt.Fprintf(w, "%s\n", style.HighlightStyle.Render(tool.Name))
					if tool.Description != "" {
						fmt.Fprintf(w, "  %s\n", tool.Description)
					}

					// Arguments section
					if len(tool.Args) > 0 {
						fmt.Fprintf(w, "  %s:\n", style.SubtitleStyle.Render("Arguments"))
						for _, arg := range tool.Args {
							required := style.DimStyle.Render("optional")
							if arg.Required {
								required = style.HighlightStyle.Render("required")
							}
							fmt.Fprintf(w, "    • %s: %s (%s)\n",
								style.HighlightStyle.Render(arg.Name),
								arg.Description,
								required,
							)
						}
					}

					// Environment variables section
					if len(tool.Env) > 0 {
						fmt.Fprintf(w, "  %s:\n", style.SubtitleStyle.Render("Environment"))
						for _, env := range tool.Env {
							if icon, label, ok := getEnvIntegration(env); ok {
								fmt.Fprintf(w, "    • %s %s %s\n",
									env,
									icon,
									style.DimStyle.Render(fmt.Sprintf("(Inherited from %s)", label)))
							} else {
								fmt.Fprintf(w, "    • %s\n", env)
							}
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
	var (
		outputFormat   string
		nonInteractive bool
	)

	cmd := &cobra.Command{
		Use:   "search [query]",
		Short: "🔍 Search for tools",
		Args:  cobra.ExactArgs(1),
		Example: `  # Interactive search (default)
  kubiya tool search kubernetes

  # Non-interactive search
  kubiya tool search kubernetes --non-interactive

  # JSON output
  kubiya tool search deploy --output json`,
		RunE: func(cmd *cobra.Command, args []string) error {
			client := kubiya.NewClient(cfg)
			query := strings.ToLower(args[0])

			// Create context that can be cancelled
			ctx, cancel := context.WithCancel(cmd.Context())
			defer cancel()

			// Handle interrupt
			sigChan := make(chan os.Signal, 1)
			signal.Notify(sigChan, os.Interrupt)
			go func() {
				<-sigChan
				cancel()
			}()

			// Initialize all variables before any goto statements
			var matches []struct {
				Tool     kubiya.Tool
				Source   kubiya.Source
				Distance int
			}
			var completed int
			spinner := []string{"⠋", "⠙", "⠹", "⠸", "⠼", "⠴", "⠦", "⠧", "⠇", "⠏"}
			spinnerIdx := 0
			ticker := time.NewTicker(100 * time.Millisecond)
			defer ticker.Stop()

			sources, err := client.ListSources(ctx)
			if err != nil {
				return err
			}

			// Pre-filter sources based on name/description prefix match
			var relevantSources []kubiya.Source
			for _, source := range sources {
				if strings.Contains(strings.ToLower(source.Name), query) ||
					strings.Contains(strings.ToLower(source.Description), query) {
					relevantSources = append(relevantSources, source)
					continue
				}
				// If no direct match, check if source name starts with any word from query
				queryWords := strings.Fields(query)
				for _, word := range queryWords {
					if strings.HasPrefix(strings.ToLower(source.Name), word) ||
						strings.HasPrefix(strings.ToLower(source.Description), word) {
						relevantSources = append(relevantSources, source)
						break
					}
				}
			}

			// If no relevant sources found, search all sources
			if len(relevantSources) == 0 {
				relevantSources = sources
			}

			type searchResult struct {
				Tool     kubiya.Tool
				Source   kubiya.Source
				Distance int
				Error    error
			}

			resultChan := make(chan searchResult, len(relevantSources))
			sem := make(chan struct{}, 5)

			if !nonInteractive {
				fmt.Printf("🔍 Starting search for '%s' in %d sources...\n", args[0], len(relevantSources))
			}

			var activeSearches int
			for _, source := range relevantSources {
				select {
				case <-ctx.Done():
					goto SUMMARIZE
				case sem <- struct{}{}:
				}

				activeSearches++
				go func(s kubiya.Source) {
					defer func() {
						<-sem
						resultChan <- searchResult{Source: s, Error: fmt.Errorf("done")}
					}()

					metadata, err := client.GetSourceMetadataCached(ctx, s.UUID)
					if err != nil {
						resultChan <- searchResult{Source: s, Error: err}
						return
					}

					// First check for exact matches
					for _, tool := range metadata.Tools {
						select {
						case <-ctx.Done():
							return
						default:
							toolName := strings.ToLower(tool.Name)
							toolDesc := strings.ToLower(tool.Description)

							// Prioritize exact matches
							if strings.Contains(toolName, query) || strings.Contains(toolDesc, query) {
								resultChan <- searchResult{Tool: tool, Source: s, Distance: 0}
								continue
							}

							// Then check Levenshtein distance for close matches
							nameDistance := kubiya.LevenshteinDistance(toolName, query)
							descDistance := kubiya.LevenshteinDistance(toolDesc, query)
							distance := min(nameDistance, descDistance)

							if distance <= len(query)/2 {
								resultChan <- searchResult{Tool: tool, Source: s, Distance: distance}
							}
						}
					}
				}(source)
			}

			for completed < activeSearches {
				select {
				case <-ctx.Done():
					goto SUMMARIZE
				case result := <-resultChan:
					if result.Error != nil {
						if result.Error.Error() != "done" {
							continue
						}
						completed++
					} else {
						matches = append(matches, struct {
							Tool     kubiya.Tool
							Source   kubiya.Source
							Distance int
						}{result.Tool, result.Source, result.Distance})
					}
					if !nonInteractive {
						fmt.Printf("\r\033[K🔍 Progress: %d/%d sources (%d matches)",
							completed, activeSearches, len(matches))
					}
				case <-ticker.C:
					if !nonInteractive {
						fmt.Printf("\r\033[K🔍 Searching... %s", spinner[spinnerIdx])
						spinnerIdx = (spinnerIdx + 1) % len(spinner)
					}
				}

				// Early return if we have enough exact matches
				if len(matches) >= 10 && matches[0].Distance == 0 {
					goto SUMMARIZE
				}
			}

		SUMMARIZE:
			fmt.Printf("\r\033[K") // Clear line

			// Sort first by distance, then by name for ties
			sort.Slice(matches, func(i, j int) bool {
				if matches[i].Distance == matches[j].Distance {
					return matches[i].Tool.Name < matches[j].Tool.Name
				}
				return matches[i].Distance < matches[j].Distance
			})

			// Limit to top 10 matches
			if len(matches) > 10 {
				matches = matches[:10]
			}

			switch outputFormat {
			case "json":
				return json.NewEncoder(os.Stdout).Encode(matches)
			case "text":
				if len(matches) == 0 {
					fmt.Printf("No tools found matching '%s'\n", args[0])
					return nil
				}

				fmt.Printf("\n%s\n\n", style.TitleStyle.Render(fmt.Sprintf(" Top %d Tools ", len(matches))))

				w := tabwriter.NewWriter(os.Stdout, 0, 0, 2, ' ', 0)
				for i, match := range matches {
					fmt.Fprintf(w, "%d. %s\n", i+1, style.HighlightStyle.Render(match.Tool.Name))
					fmt.Fprintf(w, "   Source: %s\n", match.Source.Name)
					if match.Tool.Description != "" {
						fmt.Fprintf(w, "   %s\n", match.Tool.Description)
					}
					fmt.Fprintln(w)
				}
				return w.Flush()
			default:
				return fmt.Errorf("unknown output format: %s", outputFormat)
			}
		},
	}

	cmd.Flags().StringVarP(&outputFormat, "output", "o", "text", "Output format (text|json)")
	cmd.Flags().BoolVarP(&nonInteractive, "non-interactive", "n", false, "Non-interactive search mode")
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
				fmt.Printf("\n%s\n\n", style.TitleStyle.Render(fmt.Sprintf(" 🛠️  Tool: %s ", tool.Name)))
				fmt.Printf("%s %s\n\n", style.SubtitleStyle.Render("Source:"), sourceName)

				if tool.Description != "" {
					fmt.Printf("%s\n%s\n\n", style.SubtitleStyle.Render("Description:"), tool.Description)
				}

				if len(tool.Args) > 0 {
					fmt.Printf("%s\n", style.SubtitleStyle.Render("Arguments:"))
					w := tabwriter.NewWriter(os.Stdout, 0, 0, 2, ' ', 0)
					for _, arg := range tool.Args {
						required := style.DimStyle.Render("optional")
						if arg.Required {
							required = style.HighlightStyle.Render("required")
						}
						fmt.Fprintf(w, "  • %s\t%s\t(%s)\n",
							style.HighlightStyle.Render(arg.Name),
							arg.Description,
							required,
						)
					}
					w.Flush()
					fmt.Println()
				}

				if len(tool.Env) > 0 {
					fmt.Printf("%s\n", style.SubtitleStyle.Render("Environment Variables:"))
					for _, env := range tool.Env {
						if icon, label, ok := getEnvIntegration(env); ok {
							fmt.Printf("  • %s %s %s\n",
								env,
								icon,
								style.DimStyle.Render(fmt.Sprintf("(Inherited from %s)", label)))
						} else {
							fmt.Printf("  • %s\n", env)
						}
					}
					fmt.Println()
				}

				if tool.LongRunning {
					fmt.Printf("%s\n%s\n\n",
						style.SubtitleStyle.Render("Execution:"),
						"⏳ This is a long-running task",
					)
				}

				fmt.Printf("To execute this tool:\n")
				fmt.Printf("  %s\n", style.HighlightStyle.Render(fmt.Sprintf("kubiya browse")))
				fmt.Printf("  %s\n", style.HighlightStyle.Render(fmt.Sprintf("kubiya source interactive")))
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

func renderToolArgs(args []struct {
	Name        string `json:"name"`
	Type        string `json:"type"`
	Description string `json:"description"`
	Required    bool   `json:"required"`
}) string {
	var b strings.Builder
	b.WriteString("Arguments:\n")
	for _, arg := range args {
		required := ""
		if arg.Required {
			required = " (required)"
		}
		b.WriteString(fmt.Sprintf("  %s: %s%s\n", arg.Name, arg.Description, required))
		if arg.Type != "" {
			b.WriteString(fmt.Sprintf("    Type: %s\n", arg.Type))
		}
	}
	return b.String()
}

func formatToolUsage(tool *kubiya.Tool) string {
	var b strings.Builder
	b.WriteString(fmt.Sprintf("Usage: %s", tool.Name))

	// Add required args
	for _, arg := range tool.Args {
		if arg.Required {
			b.WriteString(fmt.Sprintf(" <%s>", arg.Name))
		}
	}

	// Add optional args
	for _, arg := range tool.Args {
		if !arg.Required {
			b.WriteString(fmt.Sprintf(" [%s]", arg.Name))
		}
	}

	return b.String()
}
