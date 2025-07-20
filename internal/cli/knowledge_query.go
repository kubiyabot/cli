package cli

import (
	"encoding/json"
	"fmt"
	"strings"

	"github.com/kubiyabot/cli/internal/config"
	"github.com/kubiyabot/cli/internal/kubiya"
	"github.com/kubiyabot/cli/internal/style"
	"github.com/spf13/cobra"
)

func newKnowledgeCommand(cfg *config.Config) *cobra.Command {
	cmd := &cobra.Command{
		Use:     "knowledge",
		Aliases: []string{"kb"},
		Short:   "🔍 Query the central knowledge base",
		Long:    `Query the central knowledge base for contextual information with intelligent search capabilities.`,
	}

	cmd.AddCommand(
		newQueryKnowledgeCommand(cfg),
	)

	return cmd
}

func newQueryKnowledgeCommand(cfg *config.Config) *cobra.Command {
	var (
		stream         bool
		outputFormat   string
		responseFormat string
		userID         string
		orgID          string
	)

	cmd := &cobra.Command{
		Use:     "query [prompt]",
		Aliases: []string{"q", "search"},
		Short:   "🔍 Query the knowledge base",
		Long: `Query the central knowledge base for contextual information.

This command provides intelligent search capabilities across all available data sources
in the central knowledge base, with real-time streaming updates on search progress.

Features:
  • Semantic search across multiple data sources
  • Real-time progress updates during search (with --stream)
  • Intelligent result summarization
  • Related query suggestions
  • Source tracking and metadata`,
		Example: `  # Query with streaming progress
  kubiya knowledge query "What are the current Kubernetes pod metrics?" --stream

  # Query without streaming (wait for complete response)
  kubiya knowledge query "How to deploy to production?"

  # Query with JSON output
  kubiya knowledge query "Database backup procedures" --output json`,
		Args: cobra.MinimumNArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			// Join all arguments as the prompt
			prompt := strings.Join(args, " ")

			// Create client
			client := kubiya.NewClient(cfg)

			// Prepare request
			req := kubiya.KnowledgeQueryRequest{
				Query:          prompt,
				UserID:         userID,
				OrgID:          orgID,
				BearerToken:    cfg.APIKey,
				ResponseFormat: responseFormat,
			}

			// If not streaming, accumulate all responses
			if !stream {
				fmt.Printf("🔍 Querying knowledge base... ")

				// Query and collect all events
				events, err := client.Knowledge().Query(cmd.Context(), req)
				if err != nil {
					fmt.Println("❌")
					return fmt.Errorf("failed to query knowledge base: %w", err)
				}

				var fullResponse strings.Builder
				var errorOccurred bool

				for event := range events {
					switch event.Type {
					case "data":
						// For non-streaming, just accumulate the data
						fullResponse.WriteString(event.Data)
					case "error":
						errorOccurred = true
						fullResponse.WriteString(fmt.Sprintf("\n❌ Error: %s", event.Data))
					}
				}

				if errorOccurred {
					fmt.Println("❌")
				} else {
					fmt.Println("✅")
				}

				// Output based on format
				result := fullResponse.String()
				switch outputFormat {
				case "json":
					// Try to format as JSON if possible
					var jsonData interface{}
					if err := json.Unmarshal([]byte(result), &jsonData); err == nil {
						formatted, _ := json.MarshalIndent(jsonData, "", "  ")
						fmt.Println(string(formatted))
					} else {
						// If not valid JSON, output as-is
						fmt.Println(result)
					}
				default:
					fmt.Println("\n" + result)
				}

				return nil
			}

			// Streaming mode
			fmt.Println(style.TitleStyle.Render("🔍 Knowledge Base Query"))
			fmt.Println(style.SubtitleStyle.Render("Query: " + prompt))
			fmt.Println()

			// Query with streaming
			events, err := client.Knowledge().Query(cmd.Context(), req)
			if err != nil {
				return fmt.Errorf("failed to query knowledge base: %w", err)
			}

			// Process streaming events
			for event := range events {
				switch event.Type {
				case "data":
					// Print data as it comes
					fmt.Print(event.Data)
				case "done":
					fmt.Println("\n\n" + style.SuccessStyle.Render("✅ Query completed"))
					// Try to parse done event for metadata
					if event.Data != "" {
						var doneData map[string]interface{}
						if err := json.Unmarshal([]byte(event.Data), &doneData); err == nil {
							if usage, ok := doneData["usage"].(map[string]interface{}); ok {
								fmt.Printf("\n📊 Usage: %v prompt tokens, %v completion tokens\n",
									usage["promptTokens"], usage["completionTokens"])
							}
						}
					}
				case "error":
					fmt.Println("\n\n" + style.ErrorStyle.Render("❌ Error: "+event.Data))
				}
			}

			return nil
		},
	}

	// Add flags
	cmd.Flags().BoolVarP(&stream, "stream", "s", true, "Stream the response in real-time")
	cmd.Flags().StringVarP(&outputFormat, "output", "o", "text", "Output format (text|json)")
	cmd.Flags().StringVar(&responseFormat, "response-format", "vercel", "Response format for API (vercel|sse|json)")
	cmd.Flags().StringVar(&userID, "user-id", "", "User ID for the query")
	cmd.Flags().StringVar(&orgID, "org-id", "", "Organization ID for the query")

	return cmd
}
