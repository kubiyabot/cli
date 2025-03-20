package cli

import (
	"context"
	"encoding/csv"
	"encoding/json"
	"fmt"
	"os"
	"os/signal"
	"sort"
	"strings"
	"text/tabwriter"
	"time"

	"github.com/fatih/color"
	"github.com/kubiyabot/cli/internal/config"
	"github.com/kubiyabot/cli/internal/kubiya"
	"github.com/kubiyabot/cli/internal/style"
	"github.com/kubiyabot/cli/internal/tui"
	"github.com/mattn/go-isatty"
	"github.com/spf13/cobra"
)

func newAuditCommand(cfg *config.Config) *cobra.Command {
	cmd := &cobra.Command{
		Use:     "audit",
		Aliases: []string{"logs"},
		Short:   "📊 View and monitor audit logs",
		Long:    `View, filter, and stream audit logs in real-time or historically.`,
	}

	cmd.AddCommand(
		newAuditListCommand(cfg),
		newAuditStreamCommand(cfg),
		newAuditExportCommand(cfg),
		newAuditDescribeCommand(cfg),
		newAuditSearchCommand(cfg),
	)

	return cmd
}

func newAuditListCommand(cfg *config.Config) *cobra.Command {
	var (
		outputFormat  string
		limit         int
		categoryType  string
		categoryName  string
		resourceType  string
		actionType    string
		sessionID     string
		startTime     string
		endTime       string
		pageSize      int
		page          int
		sortDirection string
	)

	cmd := &cobra.Command{
		Use:   "list",
		Short: "📋 List audit logs",
		Example: `  # List all audit logs (last 24 hours by default)
  kubiya audit list
  
  # Filter by category type and name
  kubiya audit list --category-type agents --category-name WebhookSanity
  
  # Filter by time range
  kubiya audit list --start-time "2023-04-01T00:00:00Z" --end-time "2023-04-02T00:00:00Z"
  
  # Filter by session ID
  kubiya audit list --session-id "session123"
  
  # Output as JSON
  kubiya audit list --output json`,
		RunE: func(cmd *cobra.Command, args []string) error {
			client := kubiya.NewClient(cfg)

			// Set default time range if not provided
			if startTime == "" {
				// Default to 24 hours ago
				startTime = time.Now().UTC().Add(-24 * time.Hour).Format(time.RFC3339)
			}

			// Validate time formats
			if startTime != "" {
				if _, err := time.Parse(time.RFC3339, startTime); err != nil {
					return fmt.Errorf("invalid start time format, please use RFC3339 format (e.g., 2023-04-01T00:00:00Z): %w", err)
				}
			}

			if endTime != "" {
				if _, err := time.Parse(time.RFC3339, endTime); err != nil {
					return fmt.Errorf("invalid end time format, please use RFC3339 format (e.g., 2023-04-01T00:00:00Z): %w", err)
				}
			}

			// Build query
			query := kubiya.AuditQuery{
				Filter: kubiya.AuditFilter{
					CategoryType: categoryType,
					CategoryName: categoryName,
					ResourceType: resourceType,
					ActionType:   actionType,
					SessionID:    sessionID,
				},
				Page:     page,
				PageSize: pageSize,
				Sort: kubiya.AuditSort{
					Timestamp: -1, // Default to descending
				},
			}

			// Set sort direction
			if sortDirection == "asc" {
				query.Sort.Timestamp = 1
			} else if sortDirection == "desc" {
				query.Sort.Timestamp = -1
			}

			// Set timestamp filter if provided
			if startTime != "" || endTime != "" {
				if startTime != "" {
					query.Filter.Timestamp.GTE = startTime
				}
				if endTime != "" {
					query.Filter.Timestamp.LTE = endTime
				}
			}

			// Fetch audit items
			items, err := client.Audit().ListAuditItems(cmd.Context(), query)
			if err != nil {
				return fmt.Errorf("failed to fetch audit logs: %w", err)
			}

			// Apply limit if specified
			if limit > 0 && limit < len(items) {
				items = items[:limit]
			}

			// Display items based on output format
			switch outputFormat {
			case "json":
				return json.NewEncoder(os.Stdout).Encode(items)
			case "yaml":
				return fmt.Errorf("yaml output format not implemented yet")
			default:
				// Print header
				title := style.TitleStyle.Render("AUDIT LOGS")
				fmt.Printf("╭────────────────────────────────────────────────────────────────────────────────────────╮\n")
				fmt.Printf("│ 📊 %-85s │\n", title)
				fmt.Printf("╰────────────────────────────────────────────────────────────────────────────────────────╯\n\n")

				// Use tabwriter for aligned columns
				w := tabwriter.NewWriter(os.Stdout, 0, 0, 2, ' ', 0)

				// Write colored header
				fmt.Fprintln(w, style.HeaderStyle.Render("TIMESTAMP")+"\t"+
					style.HeaderStyle.Render("CATEGORY")+"\t"+
					style.HeaderStyle.Render("RESOURCE")+"\t"+
					style.HeaderStyle.Render("ACTION")+"\t"+
					style.HeaderStyle.Render("RESULT"))

				fmt.Fprintln(w, "───────────────────\t"+
					"───────────────────\t"+
					"───────────────────\t"+
					"───────────────────\t"+
					"─────────")

				if len(items) == 0 {
					fmt.Fprintln(w, style.DimStyle.Render("<no audit logs found>"))
				} else {
					for _, item := range items {
						// Parse timestamp
						ts, err := time.Parse(time.RFC3339, item.Timestamp)
						if err != nil {
							ts = time.Now()
						}
						timestamp := ts.Format("2006-01-02 15:04:05")

						// Format category
						category := item.CategoryType
						if item.CategoryName != "" {
							category += "/" + item.CategoryName
						}

						// Format resource
						resource := item.ResourceType
						if item.ResourceText != "" {
							resource += ": " + item.ResourceText
						}

						// Format result with color based on success
						var result string
						if item.ActionSuccessful {
							result = style.SuccessStyle.Render("Success")
						} else {
							result = style.ErrorStyle.Render("Failed")
						}

						fmt.Fprintf(w, "%s\t%s\t%s\t%s\t%s\n",
							style.DimStyle.Render(timestamp),
							style.SubtitleStyle.Render(truncateAuditString(category, 20)),
							truncateAuditString(resource, 25),
							style.HighlightStyle.Render(item.ActionType),
							result,
						)
					}
				}

				if err := w.Flush(); err != nil {
					return err
				}

				// Print summary and tips
				fmt.Printf("\n%s Found %d audit logs\n", style.SubtitleStyle.Render("ℹ️"), len(items))
				fmt.Printf("\n%s\n", style.SubtitleStyle.Render("💡 Tips:"))
				fmt.Printf("  • %s for real-time monitoring\n", style.CommandStyle.Render("kubiya audit stream"))
				fmt.Printf("  • %s for machine-readable output\n", style.CommandStyle.Render("--output json"))
				fmt.Printf("  • %s to apply more specific filters\n", style.CommandStyle.Render("--category-type, --action-type, etc."))

				return nil
			}
		},
	}

	// Add filters
	cmd.Flags().StringVarP(&outputFormat, "output", "o", "text", "Output format (text|json)")
	cmd.Flags().IntVarP(&limit, "limit", "l", 0, "Limit the number of audit logs to display")
	cmd.Flags().StringVar(&categoryType, "category-type", "", "Filter by category type (e.g., agents, webhook)")
	cmd.Flags().StringVar(&categoryName, "category-name", "", "Filter by category name")
	cmd.Flags().StringVar(&resourceType, "resource-type", "", "Filter by resource type")
	cmd.Flags().StringVar(&actionType, "action-type", "", "Filter by action type")
	cmd.Flags().StringVar(&sessionID, "session-id", "", "Filter by session ID")
	cmd.Flags().StringVar(&startTime, "start-time", "", "Filter by start time (RFC3339 format)")
	cmd.Flags().StringVar(&endTime, "end-time", "", "Filter by end time (RFC3339 format)")
	cmd.Flags().IntVar(&pageSize, "page-size", 50, "Number of items per page")
	cmd.Flags().IntVar(&page, "page", 1, "Page number")
	cmd.Flags().StringVar(&sortDirection, "sort", "desc", "Sort direction (asc|desc)")

	return cmd
}

func newAuditStreamCommand(cfg *config.Config) *cobra.Command {
	var (
		categoryType   string
		categoryName   string
		resourceType   string
		actionType     string
		sessionID      string
		startTime      string
		filterExpr     string
		followAll      bool
		timeoutMinutes int
		verbose        bool
		useTui         bool // Add option to use TUI
	)

	cmd := &cobra.Command{
		Use:   "stream",
		Short: "📡 Stream audit logs in real-time",
		Example: `  # Stream all audit logs in real-time
  kubiya audit stream
  
  # Stream logs for a specific category and action
  kubiya audit stream --category-type agents --action-type sent
  
  # Stream logs for a specific session
  kubiya audit stream --session-id "session123"
  
  # Stream all logs in verbose mode
  kubiya audit stream --verbose
  
  # Stream with a timeout (stop after n minutes)
  kubiya audit stream --timeout 5
  
  # Stream with rich TUI display
  kubiya audit stream --tui`,
		RunE: func(cmd *cobra.Command, args []string) error {
			client := kubiya.NewClient(cfg)

			// Set default start time if not provided
			if startTime == "" {
				// Default to 5 minutes ago
				startTime = time.Now().UTC().Add(-5 * time.Minute).Format(time.RFC3339)
			}

			// Validate time format
			if startTime != "" {
				if _, err := time.Parse(time.RFC3339, startTime); err != nil {
					return fmt.Errorf("invalid start time format, please use RFC3339 format (e.g., 2023-04-01T00:00:00Z): %w", err)
				}
			}

			// Build initial query
			query := kubiya.AuditQuery{
				Filter: kubiya.AuditFilter{
					Timestamp: struct {
						GTE string `json:"gte,omitempty"`
						LTE string `json:"lte,omitempty"`
					}{
						GTE: startTime,
					},
					CategoryType: categoryType,
					CategoryName: categoryName,
					ResourceType: resourceType,
					ActionType:   actionType,
					SessionID:    sessionID,
				},
				Page:     1,
				PageSize: 50,
				Sort: kubiya.AuditSort{
					Timestamp: -1, // Sort by timestamp descending
				},
			}

			// Use TUI if specified
			if useTui {
				return tui.StartAuditStream(client, query, verbose)
			}

			// Create cancellable context
			ctx, cancel := context.WithCancel(cmd.Context())
			defer cancel()

			// Set up timeout if requested
			if timeoutMinutes > 0 {
				ctx, cancel = context.WithTimeout(ctx, time.Duration(timeoutMinutes)*time.Minute)
				defer cancel()
				fmt.Printf("⏱️  Streaming will automatically stop after %d minutes\n", timeoutMinutes)
			}

			// Set up signal handling for graceful cancellation
			sigChan := make(chan os.Signal, 1)
			signal.Notify(sigChan, os.Interrupt)

			go func() {
				<-sigChan
				fmt.Println("\n⚠️  Interrupt received, stopping stream...")
				cancel()
			}()

			// Display initial info
			fmt.Println("📡 Starting audit log stream...")
			fmt.Printf("🕒 Current time (local): %s\n", time.Now().Format(time.RFC3339))
			fmt.Printf("🕒 Current time (UTC):   %s\n", time.Now().UTC().Format(time.RFC3339))
			fmt.Printf("🔍 Filter timestamp from: %s\n", startTime)

			// Display filters being applied
			if categoryType != "" || categoryName != "" || resourceType != "" ||
				actionType != "" || sessionID != "" || filterExpr != "" {
				fmt.Println("🎯 Streaming with filters:")
				if categoryType != "" {
					fmt.Printf("   - Category Type: %s\n", categoryType)
				}
				if categoryName != "" {
					fmt.Printf("   - Category Name: %s\n", categoryName)
				}
				if resourceType != "" {
					fmt.Printf("   - Resource Type: %s\n", resourceType)
				}
				if actionType != "" {
					fmt.Printf("   - Action Type: %s\n", actionType)
				}
				if sessionID != "" {
					fmt.Printf("   - Session ID: %s\n", sessionID)
				}
				if filterExpr != "" {
					fmt.Printf("   - Custom Filter: %s\n", filterExpr)
				}
			}

			// Check if we're in a terminal
			isTerminal := isatty.IsTerminal(os.Stdout.Fd())

			// Create color instances for different states
			var successColor, errorColor, infoColor, headerColor, stepColor, detailColor func(a ...interface{}) string
			if isTerminal {
				successColor = color.New(color.FgGreen).SprintFunc()
				errorColor = color.New(color.FgRed).SprintFunc()
				infoColor = color.New(color.FgCyan).SprintFunc()
				headerColor = color.New(color.FgYellow, color.Bold).SprintFunc()
				stepColor = color.New(color.FgMagenta).SprintFunc()
				detailColor = color.New(color.FgWhite, color.Faint).SprintFunc()
			} else {
				successColor = fmt.Sprint
				errorColor = fmt.Sprint
				infoColor = fmt.Sprint
				headerColor = fmt.Sprint
				stepColor = fmt.Sprint
				detailColor = fmt.Sprint
			}

			// Print header
			fmt.Println(headerColor("\n=== Audit Stream Started ==="))
			fmt.Printf("%s Streaming audit events in real-time. Press Ctrl+C to stop...\n\n",
				infoColor("→"))

			// Start with polling rather than streaming for more reliable results
			ticker := time.NewTicker(3 * time.Second)
			defer ticker.Stop()

			// Use a map to store processed events to avoid duplicates
			processedEvents := make(map[string]bool)

			// Keep track of latest timestamp for polling
			var latestTimestamp string

			// Process events
			pollCount := 0
			startPollTime := time.Now()

			for {
				select {
				case <-ctx.Done():
					// Distinguish between timeout and cancellation
					if ctx.Err() == context.DeadlineExceeded {
						fmt.Printf("\n%s Streaming stopped after timeout (%d minutes)\n",
							infoColor("⏱️"),
							timeoutMinutes)
					} else {
						fmt.Println("\n" + headerColor("=== Audit Stream Ended ==="))
					}
					return nil

				case <-ticker.C:
					pollCount++

					// Show poll attempts in verbose mode or periodically
					if verbose || pollCount%10 == 0 {
						fmt.Printf("%s Poll attempt #%d - timestamp filter: %s\n",
							infoColor("🔄"),
							pollCount,
							query.Filter.Timestamp.GTE)
					}

					// Update query with latest timestamp if available
					if latestTimestamp != "" {
						prevTimestamp := query.Filter.Timestamp.GTE
						query.Filter.Timestamp.GTE = latestTimestamp

						// Only print timestamp update logs in verbose mode
						if verbose {
							fmt.Printf("%s Updated timestamp filter: %s → %s\n",
								infoColor("🕰️"),
								prevTimestamp,
								latestTimestamp)
						}
					}

					// Poll for new audit items
					auditItems, err := client.Audit().ListAuditItems(ctx, query)
					if err != nil {
						// Only show polling errors in verbose mode
						if verbose {
							fmt.Printf("%s Error polling for audit items: %v\n", errorColor("❌"), err)
						}
						continue
					}

					// Only show detailed poll results in verbose mode
					if verbose && len(auditItems) > 0 {
						fmt.Printf("%s Found %d events in poll #%d\n",
							infoColor("📥"),
							len(auditItems),
							pollCount)
					}

					// Process items if any found
					if len(auditItems) > 0 {
						for _, item := range auditItems {
							// Skip if we've already processed this event
							eventKey := fmt.Sprintf("%s-%s-%s-%s", item.Timestamp, item.CategoryType, item.CategoryName, item.ActionType)
							if _, seen := processedEvents[eventKey]; seen {
								continue
							}

							// Mark this event as processed
							processedEvents[eventKey] = true

							// Update latest timestamp if newer
							if item.Timestamp > latestTimestamp {
								latestTimestamp = item.Timestamp
							}

							// Format and print the event
							displayAuditEvent(item, verbose, successColor, errorColor, infoColor, headerColor, stepColor, detailColor)
						}
					} else if verbose && pollCount%5 == 0 {
						// Show periodic "no events" message in verbose mode
						fmt.Printf("%s No new events found after %s\n",
							infoColor("ℹ️"),
							time.Since(startPollTime).Round(time.Second).String())
					}
				}
			}
		},
	}

	// Add filters
	cmd.Flags().StringVar(&categoryType, "category-type", "", "Filter by category type (e.g., agents, webhook)")
	cmd.Flags().StringVar(&categoryName, "category-name", "", "Filter by category name")
	cmd.Flags().StringVar(&resourceType, "resource-type", "", "Filter by resource type")
	cmd.Flags().StringVar(&actionType, "action-type", "", "Filter by action type")
	cmd.Flags().StringVar(&sessionID, "session-id", "", "Filter by session ID")
	cmd.Flags().StringVar(&startTime, "start-time", "", "Start time for initial filter (RFC3339 format)")
	cmd.Flags().StringVar(&filterExpr, "filter", "", "Custom filter expression (advanced)")
	cmd.Flags().BoolVar(&followAll, "follow-all", false, "Follow all events without filtering")
	cmd.Flags().IntVar(&timeoutMinutes, "timeout", 0, "Stop streaming after specified minutes")
	cmd.Flags().BoolVar(&verbose, "verbose", false, "Show verbose output including polling details")
	cmd.Flags().BoolVar(&useTui, "tui", false, "Use rich terminal UI with BubbleTea")

	return cmd
}

// Display a single audit event with formatting
func displayAuditEvent(item kubiya.AuditItem, verbose bool,
	successColor, errorColor, infoColor, headerColor, stepColor, detailColor func(a ...interface{}) string) {

	// Parse timestamp
	timestamp, err := time.Parse(time.RFC3339, item.Timestamp)
	if err != nil {
		timestamp = time.Now()
	}

	// Determine icon based on category type
	icon := "📄"
	switch item.CategoryType {
	case "agents":
		icon = "👤"
	case "ai":
		icon = "🤖"
	case "webhook":
		icon = "📡"
	case "tool_execution":
		icon = "🔧"
	case "triggers":
		icon = "🔔"
	}

	// Print category-specific header
	fmt.Printf("%s %s %s\n",
		headerColor(icon+" "+item.CategoryType+"/"+item.CategoryName+":"),
		infoColor(timestamp.Format("15:04:05")),
		stepColor(item.ActionType))

	// Print resource info
	if item.ResourceType != "" || item.ResourceText != "" {
		fmt.Printf("   %s %s",
			stepColor("Resource:"),
			item.ResourceType)

		if item.ResourceText != "" {
			fmt.Printf(": %s", item.ResourceText)
		}
		fmt.Println()
	}

	// Print result status with appropriate color
	status := "Success"
	if !item.ActionSuccessful {
		status = "Failed"
	}
	statusColor := successColor
	if !item.ActionSuccessful {
		statusColor = errorColor
	}
	fmt.Printf("   %s %s\n",
		stepColor("Status:"),
		statusColor(status))

	// Try to extract message content from various possible locations
	contentDisplayed := extractAndDisplayContent(item, verbose, infoColor, stepColor, detailColor)

	// Print extra details if available and in verbose mode
	if verbose && len(item.Extra) > 0 && !contentDisplayed {
		fmt.Printf("   %s\n", stepColor("Details:"))
		for key, value := range item.Extra {
			if key == "session_id" {
				continue // Skip session_id in output, it's noisy
			}

			// Format the value nicely
			var valueStr string
			switch v := value.(type) {
			case string:
				valueStr = v
			case map[string]interface{}:
				jsonBytes, _ := json.MarshalIndent(v, "      ", "  ")
				valueStr = string(jsonBytes)
			default:
				valueStr = fmt.Sprintf("%v", v)
			}

			fmt.Printf("      %s: %s\n",
				stepColor(key),
				detailColor(valueStr))
		}
	}

	// Print separator between events
	fmt.Println(strings.Repeat("─", 80))
}

// Helper function to extract and display message content
func extractAndDisplayContent(item kubiya.AuditItem, verbose bool,
	infoColor, stepColor, detailColor func(a ...interface{}) string) bool {

	// Flag to track if we've already displayed content
	contentDisplayed := false

	// Handle message content for agents
	if item.CategoryType == "agents" && item.ActionType == "sent" {
		// Try to extract message content from various possible locations
		var messageContent string
		var foundPaths []string

		// Check common content field names
		contentFields := []string{"content", "message", "text", "body", "response", "prompt", "query", "answer"}
		for _, field := range contentFields {
			if content, ok := item.Extra[field].(string); ok && content != "" {
				messageContent = content
				foundPaths = append(foundPaths, field)
				if verbose {
					fmt.Printf("   %s Found content in field: %s\n", infoColor("📝"), field)
				}
				break
			}
		}

		// If still empty, try to extract from resource_text
		if messageContent == "" && item.ResourceText != "" {
			// Try to extract content from resource_text for message events
			if strings.Contains(item.ResourceText, "type='msg'") && strings.Contains(item.ResourceText, "content=") {
				contentStartIndex := strings.Index(item.ResourceText, "content=")
				if contentStartIndex > 0 {
					contentStartIndex += 8 // length of "content="

					// Find the quote character used (either " or ')
					quoteChar := ""
					if contentStartIndex < len(item.ResourceText) {
						if item.ResourceText[contentStartIndex] == '"' {
							quoteChar = "\""
							contentStartIndex++ // skip the opening quote
						} else if item.ResourceText[contentStartIndex] == '\'' {
							quoteChar = "'"
							contentStartIndex++ // skip the opening quote
						}
					}

					if quoteChar != "" {
						// Find the closing quote, taking into account escaped quotes
						contentEndIndex := -1
						inEscape := false
						for i := 0; i < len(item.ResourceText[contentStartIndex:]); i++ {
							if inEscape {
								inEscape = false
								continue
							}

							if item.ResourceText[contentStartIndex+i] == '\\' {
								inEscape = true
								continue
							}

							if item.ResourceText[contentStartIndex+i] == quoteChar[0] {
								contentEndIndex = i
								break
							}
						}

						if contentEndIndex > 0 {
							messageContent = item.ResourceText[contentStartIndex : contentStartIndex+contentEndIndex]

							// Unescape any escaped quotes
							messageContent = strings.ReplaceAll(messageContent, "\\"+quoteChar, quoteChar)

							if verbose {
								fmt.Printf("   %s Extracted from resource_text\n", infoColor("📝"))
							}
						}
					}
				}
			}
		}

		// If we still don't have content and it's a user message, use resource_text directly
		if messageContent == "" && !strings.HasPrefix(item.ResourceText, "end=") && !strings.Contains(item.ResourceText, "type=") {
			messageContent = item.ResourceText
			if verbose {
				fmt.Printf("   %s Using resource_text directly as content\n", infoColor("📝"))
			}
		}

		// If we found content, display it
		if messageContent != "" {
			// For agent sent events, display the content prominently
			isUserMessage := false
			if val, ok := item.Extra["is_user_message"].(bool); ok {
				isUserMessage = val
			}

			// Determine message type/direction
			messageIcon := "🤖" // Default: bot message
			messageDirection := "Agent"
			if isUserMessage {
				messageIcon = "👤"
				messageDirection = "User"
			}

			// Format the output for better readability
			fmt.Printf("   %s %s %s\n",
				messageIcon,
				stepColor(messageDirection+":"),
				detailColor(messageContent))

			// Mark that we've displayed content
			contentDisplayed = true
		}
	}

	// Handle tool execution
	if item.CategoryType == "tool_execution" ||
		(item.CategoryType == "agents" && item.ResourceType == "Tool Execution") {
		fmt.Printf("   %s %s\n",
			stepColor("Tool:"),
			item.ResourceText)

		// Try to get the output or result
		var outputContent string
		outputFields := []string{"output", "result", "response", "data", "content"}

		// Try string fields first
		for _, field := range outputFields {
			if output, ok := item.Extra[field].(string); ok && output != "" {
				outputContent = output
				if verbose {
					fmt.Printf("   %s Found output in field: %s\n", infoColor("📝"), field)
				}
				break
			}
		}

		// Try map fields and convert to JSON
		if outputContent == "" {
			for _, field := range outputFields {
				if output, ok := item.Extra[field].(map[string]interface{}); ok && len(output) > 0 {
					outputJSON, _ := json.MarshalIndent(output, "      ", "  ")
					outputContent = string(outputJSON)
					if verbose {
						fmt.Printf("   %s Found object output in field: %s\n", infoColor("📝"), field)
					}
					break
				}
			}
		}

		if outputContent != "" {
			fmt.Printf("   %s\n   %s\n",
				stepColor("Result:"),
				detailColor(outputContent))
			contentDisplayed = true
		}
	}

	return contentDisplayed
}

// Helper function to truncate a string to a maximum length
func truncateAuditString(s string, maxLen int) string {
	if len(s) <= maxLen {
		return s
	}
	return s[:maxLen-3] + "..."
}

// Add a new export command
func newAuditExportCommand(cfg *config.Config) *cobra.Command {
	var (
		outputFormat  string
		outputFile    string
		categoryType  string
		categoryName  string
		resourceType  string
		actionType    string
		sessionID     string
		startTime     string
		endTime       string
		pageSize      int
		page          int
		sortDirection string
		maxItems      int
	)

	cmd := &cobra.Command{
		Use:   "export",
		Short: "📤 Export audit logs to a file",
		Example: `  # Export all audit logs from the last 24 hours to JSON file
  kubiya audit export --output json --file audit_logs.json
  
  # Export filtered logs to CSV
  kubiya audit export --category-type agents --start-time "2023-04-01T00:00:00Z" --output csv --file agent_logs.csv
  
  # Export with limit on number of items
  kubiya audit export --max-items 1000 --output json --file recent_logs.json`,
		RunE: func(cmd *cobra.Command, args []string) error {
			client := kubiya.NewClient(cfg)

			// Set default time range if not provided
			if startTime == "" {
				// Default to 24 hours ago
				startTime = time.Now().UTC().Add(-24 * time.Hour).Format(time.RFC3339)
			}

			// Validate time formats
			if startTime != "" {
				if _, err := time.Parse(time.RFC3339, startTime); err != nil {
					return fmt.Errorf("invalid start time format, please use RFC3339 format (e.g., 2023-04-01T00:00:00Z): %w", err)
				}
			}

			if endTime != "" {
				if _, err := time.Parse(time.RFC3339, endTime); err != nil {
					return fmt.Errorf("invalid end time format, please use RFC3339 format (e.g., 2023-04-01T00:00:00Z): %w", err)
				}
			}

			// Build query
			query := kubiya.AuditQuery{
				Filter: kubiya.AuditFilter{
					CategoryType: categoryType,
					CategoryName: categoryName,
					ResourceType: resourceType,
					ActionType:   actionType,
					SessionID:    sessionID,
				},
				Page:     page,
				PageSize: pageSize,
				Sort: kubiya.AuditSort{
					Timestamp: -1, // Default to descending
				},
			}

			// Set sort direction
			if sortDirection == "asc" {
				query.Sort.Timestamp = 1
			} else if sortDirection == "desc" {
				query.Sort.Timestamp = -1
			}

			// Set timestamp filter if provided
			if startTime != "" || endTime != "" {
				if startTime != "" {
					query.Filter.Timestamp.GTE = startTime
				}
				if endTime != "" {
					query.Filter.Timestamp.LTE = endTime
				}
			}

			// Validate required flags
			if outputFile == "" {
				return fmt.Errorf("--file flag is required for export")
			}

			fmt.Printf("Exporting audit logs to %s in %s format...\n", outputFile, outputFormat)

			// Fetch audit items (potentially multiple pages if maxItems > pageSize)
			var allItems []kubiya.AuditItem
			currentPage := page
			totalExported := 0

			for {
				// Update page in query
				query.Page = currentPage

				items, err := client.Audit().ListAuditItems(cmd.Context(), query)
				if err != nil {
					return fmt.Errorf("failed to fetch audit logs: %w", err)
				}

				if len(items) == 0 {
					break // No more items
				}

				allItems = append(allItems, items...)
				totalExported += len(items)

				// Check if we've hit max items or if there are no more pages
				if maxItems > 0 && totalExported >= maxItems {
					if totalExported > maxItems {
						// Trim to max items
						allItems = allItems[:maxItems]
						totalExported = maxItems
					}
					break
				}

				if len(items) < pageSize {
					break // Last page
				}

				currentPage++
			}

			// Create output file
			file, err := os.Create(outputFile)
			if err != nil {
				return fmt.Errorf("failed to create output file: %w", err)
			}
			defer file.Close()

			// Export based on output format
			switch outputFormat {
			case "json":
				encoder := json.NewEncoder(file)
				encoder.SetIndent("", "  ")
				if err := encoder.Encode(allItems); err != nil {
					return fmt.Errorf("failed to encode to JSON: %w", err)
				}

			case "csv":
				writer := csv.NewWriter(file)
				defer writer.Flush()

				// Write header
				header := []string{"Timestamp", "Category Type", "Category Name", "Resource Type",
					"Resource Text", "Action Type", "Action Successful", "Extra Data"}
				if err := writer.Write(header); err != nil {
					return fmt.Errorf("failed to write CSV header: %w", err)
				}

				// Write data rows
				for _, item := range allItems {
					// Convert Extra to string
					extraJSON, _ := json.Marshal(item.Extra)

					row := []string{
						item.Timestamp,
						item.CategoryType,
						item.CategoryName,
						item.ResourceType,
						item.ResourceText,
						item.ActionType,
						fmt.Sprintf("%t", item.ActionSuccessful),
						string(extraJSON),
					}

					if err := writer.Write(row); err != nil {
						return fmt.Errorf("failed to write CSV row: %w", err)
					}
				}

			case "txt":
				// Simple text format
				for _, item := range allItems {
					ts, _ := time.Parse(time.RFC3339, item.Timestamp)
					fmt.Fprintf(file, "[%s] %s/%s - %s: %s (%s)\n",
						ts.Format("2006-01-02 15:04:05"),
						item.CategoryType,
						item.CategoryName,
						item.ActionType,
						item.ResourceText,
						fmt.Sprintf("%t", item.ActionSuccessful))
				}

			default:
				return fmt.Errorf("unsupported output format: %s", outputFormat)
			}

			fmt.Printf("✅ Successfully exported %d audit logs to %s\n", totalExported, outputFile)
			return nil
		},
	}

	// Add filters and export options
	cmd.Flags().StringVarP(&outputFormat, "output", "o", "json", "Output format (json|csv|txt)")
	cmd.Flags().StringVarP(&outputFile, "file", "f", "", "Output file path (required)")
	cmd.Flags().StringVar(&categoryType, "category-type", "", "Filter by category type (e.g., agents, webhook)")
	cmd.Flags().StringVar(&categoryName, "category-name", "", "Filter by category name")
	cmd.Flags().StringVar(&resourceType, "resource-type", "", "Filter by resource type")
	cmd.Flags().StringVar(&actionType, "action-type", "", "Filter by action type")
	cmd.Flags().StringVar(&sessionID, "session-id", "", "Filter by session ID")
	cmd.Flags().StringVar(&startTime, "start-time", "", "Filter by start time (RFC3339 format)")
	cmd.Flags().StringVar(&endTime, "end-time", "", "Filter by end time (RFC3339 format)")
	cmd.Flags().IntVar(&pageSize, "page-size", 100, "Number of items per page when fetching")
	cmd.Flags().IntVar(&page, "page", 1, "Starting page number")
	cmd.Flags().StringVar(&sortDirection, "sort", "desc", "Sort direction (asc|desc)")
	cmd.Flags().IntVar(&maxItems, "max-items", 0, "Maximum number of items to export (0 for unlimited)")

	// Mark output file as required
	cmd.MarkFlagRequired("file")

	return cmd
}

// Add a describe command for viewing detailed information about a specific audit event
func newAuditDescribeCommand(cfg *config.Config) *cobra.Command {
	var (
		itemID       string
		outputFormat string
	)

	cmd := &cobra.Command{
		Use:   "describe",
		Short: "🔍 Show detailed information about a specific audit event",
		Example: `  # Describe an audit event by ID
  kubiya audit describe --id event123
  
  # Output in JSON format
  kubiya audit describe --id event123 --output json`,
		RunE: func(cmd *cobra.Command, args []string) error {
			if itemID == "" {
				return fmt.Errorf("--id flag is required")
			}

			client := kubiya.NewClient(cfg)

			// Search for the specific audit item
			// This is a simplified approach - in a real implementation,
			// you would have a direct GetAuditItem method if available

			// In a real implementation, we'd have a direct method to get by ID
			// For now, simulate this by searching through recent items
			fmt.Printf("Searching for audit event with ID: %s\n", itemID)

			// Get recent items and search for the one with matching ID
			recentQuery := kubiya.AuditQuery{
				Filter: kubiya.AuditFilter{
					Timestamp: struct {
						GTE string `json:"gte,omitempty"`
						LTE string `json:"lte,omitempty"`
					}{
						GTE: time.Now().UTC().Add(-7 * 24 * time.Hour).Format(time.RFC3339),
					},
				},
				Page:     1,
				PageSize: 1000, // Use a large page size to search more items
				Sort: kubiya.AuditSort{
					Timestamp: -1,
				},
			}

			items, err := client.Audit().ListAuditItems(cmd.Context(), recentQuery)
			if err != nil {
				return fmt.Errorf("failed to search for audit event: %w", err)
			}

			// Find the matching item (this is illustrative; in a real implementation
			// we'd have a direct lookup by ID)
			var targetItem *kubiya.AuditItem
			for i, item := range items {
				// Check if this item matches our target ID
				// Here we're checking the Extra field - adjust based on where IDs are stored
				if idValue, ok := item.Extra["id"].(string); ok && idValue == itemID {
					targetItem = &items[i]
					break
				}

				// For now, also check if the item timestamp contains the ID (as a fallback)
				if strings.Contains(item.Timestamp, itemID) {
					targetItem = &items[i]
					break
				}
			}

			if targetItem == nil {
				return fmt.Errorf("audit event with ID %s not found", itemID)
			}

			// Display the item based on output format
			switch outputFormat {
			case "json":
				return json.NewEncoder(os.Stdout).Encode(targetItem)

			default: // text
				// Format the audit item details for display
				fmt.Println(style.TitleStyle.Render("AUDIT EVENT DETAILS"))
				fmt.Println(strings.Repeat("═", 80))

				ts, _ := time.Parse(time.RFC3339, targetItem.Timestamp)
				fmt.Printf("%s %s\n",
					style.HeaderStyle.Render("Timestamp:"),
					ts.Format("2006-01-02 15:04:05 -0700 MST"))

				fmt.Printf("%s %s/%s\n",
					style.HeaderStyle.Render("Category:"),
					targetItem.CategoryType,
					targetItem.CategoryName)

				fmt.Printf("%s %s\n",
					style.HeaderStyle.Render("Action:"),
					targetItem.ActionType)

				fmt.Printf("%s %s: %s\n",
					style.HeaderStyle.Render("Resource:"),
					targetItem.ResourceType,
					targetItem.ResourceText)

				status := "Success"
				statusStyle := style.SuccessStyle
				if !targetItem.ActionSuccessful {
					status = "Failed"
					statusStyle = style.ErrorStyle
				}
				fmt.Printf("%s %s\n",
					style.HeaderStyle.Render("Status:"),
					statusStyle.Render(status))

				// Print Extra data with nice formatting
				if len(targetItem.Extra) > 0 {
					fmt.Printf("\n%s\n", style.HeaderStyle.Render("Additional Data:"))
					fmt.Println(strings.Repeat("─", 80))

					// Get sorted keys for consistent output
					keys := make([]string, 0, len(targetItem.Extra))
					for k := range targetItem.Extra {
						keys = append(keys, k)
					}
					sort.Strings(keys)

					for _, key := range keys {
						value := targetItem.Extra[key]

						// Format the value based on its type
						var formattedValue string
						switch v := value.(type) {
						case string:
							formattedValue = v

						case map[string]interface{}:
							// For nested objects, format as JSON
							jsonBytes, _ := json.MarshalIndent(v, "    ", "  ")
							formattedValue = "\n    " + string(jsonBytes)

						case []interface{}:
							// For arrays, format as JSON
							jsonBytes, _ := json.MarshalIndent(v, "    ", "  ")
							formattedValue = "\n    " + string(jsonBytes)

						default:
							formattedValue = fmt.Sprintf("%v", v)
						}

						fmt.Printf("  %s %s\n",
							style.SubtitleStyle.Render(key+":"),
							formattedValue)
					}
				}
			}

			return nil
		},
	}

	cmd.Flags().StringVar(&itemID, "id", "", "ID of the audit event to describe (required)")
	cmd.Flags().StringVarP(&outputFormat, "output", "o", "text", "Output format (text|json)")

	// Mark ID as required
	cmd.MarkFlagRequired("id")

	return cmd
}

// Add a search command for more advanced search capabilities
func newAuditSearchCommand(cfg *config.Config) *cobra.Command {
	var (
		outputFormat  string
		limit         int
		categoryType  string
		categoryName  string
		resourceType  string
		actionType    string
		sessionID     string
		startTime     string
		endTime       string
		pageSize      int
		page          int
		sortDirection string
		textQuery     string
		statusFilter  string
	)

	cmd := &cobra.Command{
		Use:   "search",
		Short: "🔎 Search audit logs with advanced filtering",
		Example: `  # Search for logs containing specific text
  kubiya audit search --text "error"
  
  # Search with multiple filters
  kubiya audit search --category-type agents --status failed --text "timeout"
  
  # Search within a time range
  kubiya audit search --start-time "2023-04-01T00:00:00Z" --end-time "2023-04-02T00:00:00Z" --text "webhook"`,
		RunE: func(cmd *cobra.Command, args []string) error {
			client := kubiya.NewClient(cfg)

			// Set default time range if not provided
			if startTime == "" {
				// Default to 24 hours ago
				startTime = time.Now().UTC().Add(-24 * time.Hour).Format(time.RFC3339)
			}

			// Validate time formats
			if startTime != "" {
				if _, err := time.Parse(time.RFC3339, startTime); err != nil {
					return fmt.Errorf("invalid start time format, please use RFC3339 format (e.g., 2023-04-01T00:00:00Z): %w", err)
				}
			}

			if endTime != "" {
				if _, err := time.Parse(time.RFC3339, endTime); err != nil {
					return fmt.Errorf("invalid end time format, please use RFC3339 format (e.g., 2023-04-01T00:00:00Z): %w", err)
				}
			}

			// Build query
			query := kubiya.AuditQuery{
				Filter: kubiya.AuditFilter{
					CategoryType: categoryType,
					CategoryName: categoryName,
					ResourceType: resourceType,
					ActionType:   actionType,
					SessionID:    sessionID,
				},
				Page:     page,
				PageSize: pageSize,
				Sort: kubiya.AuditSort{
					Timestamp: -1, // Default to descending
				},
			}

			// Set sort direction
			if sortDirection == "asc" {
				query.Sort.Timestamp = 1
			} else if sortDirection == "desc" {
				query.Sort.Timestamp = -1
			}

			// Set timestamp filter if provided
			if startTime != "" || endTime != "" {
				if startTime != "" {
					query.Filter.Timestamp.GTE = startTime
				}
				if endTime != "" {
					query.Filter.Timestamp.LTE = endTime
				}
			}

			// Fetch audit items
			items, err := client.Audit().ListAuditItems(cmd.Context(), query)
			if err != nil {
				return fmt.Errorf("failed to fetch audit logs: %w", err)
			}

			// Apply additional filtering not supported directly by the API
			filteredItems := []kubiya.AuditItem{}
			for _, item := range items {
				// Apply success/failure filter if provided
				if statusFilter != "" {
					switch strings.ToLower(statusFilter) {
					case "success", "successful":
						if !item.ActionSuccessful {
							continue
						}
					case "fail", "failed", "failure":
						if item.ActionSuccessful {
							continue
						}
					}
				}

				// Apply text search if provided
				if textQuery != "" {
					// Search in text fields
					textMatchFound := false

					// Convert textQuery to lowercase for case-insensitive search
					lowercaseQuery := strings.ToLower(textQuery)

					// Check various fields
					if strings.Contains(strings.ToLower(item.CategoryType), lowercaseQuery) ||
						strings.Contains(strings.ToLower(item.CategoryName), lowercaseQuery) ||
						strings.Contains(strings.ToLower(item.ResourceType), lowercaseQuery) ||
						strings.Contains(strings.ToLower(item.ResourceText), lowercaseQuery) ||
						strings.Contains(strings.ToLower(item.ActionType), lowercaseQuery) {
						textMatchFound = true
					}

					// If not found in standard fields, check Extra data
					if !textMatchFound {
						// Convert Extra to JSON for text searching
						extraJSON, _ := json.Marshal(item.Extra)
						if strings.Contains(strings.ToLower(string(extraJSON)), lowercaseQuery) {
							textMatchFound = true
						}
					}

					if !textMatchFound {
						continue
					}
				}

				// If we got here, the item passed all filters
				filteredItems = append(filteredItems, item)
			}

			// Apply limit if specified
			if limit > 0 && limit < len(filteredItems) {
				filteredItems = filteredItems[:limit]
			}

			// Display items based on output format
			switch outputFormat {
			case "json":
				return json.NewEncoder(os.Stdout).Encode(filteredItems)

			default: // text
				// Print header
				title := style.TitleStyle.Render("SEARCH RESULTS")
				fmt.Printf("╭────────────────────────────────────────────────────────────────────────────────────────╮\n")
				fmt.Printf("│ 🔎 %-85s │\n", title)
				fmt.Printf("╰────────────────────────────────────────────────────────────────────────────────────────╯\n\n")

				// Display search criteria
				fmt.Println(style.SubtitleStyle.Render("Search Criteria:"))
				if textQuery != "" {
					fmt.Printf("  • Text: %s\n", style.HighlightStyle.Render(textQuery))
				}
				if categoryType != "" {
					fmt.Printf("  • Category Type: %s\n", categoryType)
				}
				if categoryName != "" {
					fmt.Printf("  • Category Name: %s\n", categoryName)
				}
				if resourceType != "" {
					fmt.Printf("  • Resource Type: %s\n", resourceType)
				}
				if actionType != "" {
					fmt.Printf("  • Action Type: %s\n", actionType)
				}
				if statusFilter != "" {
					fmt.Printf("  • Status: %s\n", statusFilter)
				}
				if startTime != "" || endTime != "" {
					timeRange := "Time Range: "
					if startTime != "" {
						ts, _ := time.Parse(time.RFC3339, startTime)
						timeRange += fmt.Sprintf("From %s ", ts.Format("2006-01-02 15:04:05"))
					}
					if endTime != "" {
						ts, _ := time.Parse(time.RFC3339, endTime)
						timeRange += fmt.Sprintf("To %s", ts.Format("2006-01-02 15:04:05"))
					}
					fmt.Printf("  • %s\n", timeRange)
				}
				fmt.Println()

				// Use tabwriter for aligned columns
				w := tabwriter.NewWriter(os.Stdout, 0, 0, 2, ' ', 0)

				// Write colored header
				fmt.Fprintln(w, style.HeaderStyle.Render("TIMESTAMP")+"\t"+
					style.HeaderStyle.Render("CATEGORY")+"\t"+
					style.HeaderStyle.Render("RESOURCE")+"\t"+
					style.HeaderStyle.Render("ACTION")+"\t"+
					style.HeaderStyle.Render("RESULT"))

				fmt.Fprintln(w, "───────────────────\t"+
					"───────────────────\t"+
					"───────────────────\t"+
					"───────────────────\t"+
					"─────────")

				if len(filteredItems) == 0 {
					fmt.Fprintln(w, style.DimStyle.Render("<no matching audit logs found>"))
				} else {
					for _, item := range filteredItems {
						// Parse timestamp
						ts, err := time.Parse(time.RFC3339, item.Timestamp)
						if err != nil {
							ts = time.Now()
						}
						timestamp := ts.Format("2006-01-02 15:04:05")

						// Format category
						category := item.CategoryType
						if item.CategoryName != "" {
							category += "/" + item.CategoryName
						}

						// Format resource
						resource := item.ResourceType
						if item.ResourceText != "" {
							resource += ": " + item.ResourceText
						}

						// Format result with color based on success
						var result string
						if item.ActionSuccessful {
							result = style.SuccessStyle.Render("Success")
						} else {
							result = style.ErrorStyle.Render("Failed")
						}

						fmt.Fprintf(w, "%s\t%s\t%s\t%s\t%s\n",
							style.DimStyle.Render(timestamp),
							style.SubtitleStyle.Render(truncateAuditString(category, 20)),
							truncateAuditString(resource, 25),
							style.HighlightStyle.Render(item.ActionType),
							result,
						)
					}
				}

				if err := w.Flush(); err != nil {
					return err
				}

				// Print summary and tips
				fmt.Printf("\n%s Found %d matching audit logs\n", style.SubtitleStyle.Render("ℹ️"), len(filteredItems))
				fmt.Printf("\n%s\n", style.SubtitleStyle.Render("💡 Tips:"))
				fmt.Printf("  • Use %s to view full details of an event\n", style.CommandStyle.Render("kubiya audit describe --id <id>"))
				fmt.Printf("  • Use %s for machine-readable output\n", style.CommandStyle.Render("--output json"))
				fmt.Printf("  • Try %s for more specific text filtering\n", style.CommandStyle.Render("--text \"exact phrase\""))

				return nil
			}
		},
	}

	// Add search options
	cmd.Flags().StringVarP(&outputFormat, "output", "o", "text", "Output format (text|json)")
	cmd.Flags().IntVarP(&limit, "limit", "l", 0, "Limit the number of audit logs to display")
	cmd.Flags().StringVar(&categoryType, "category-type", "", "Filter by category type (e.g., agents, webhook)")
	cmd.Flags().StringVar(&categoryName, "category-name", "", "Filter by category name")
	cmd.Flags().StringVar(&resourceType, "resource-type", "", "Filter by resource type")
	cmd.Flags().StringVar(&actionType, "action-type", "", "Filter by action type")
	cmd.Flags().StringVar(&sessionID, "session-id", "", "Filter by session ID")
	cmd.Flags().StringVar(&startTime, "start-time", "", "Filter by start time (RFC3339 format)")
	cmd.Flags().StringVar(&endTime, "end-time", "", "Filter by end time (RFC3339 format)")
	cmd.Flags().IntVar(&pageSize, "page-size", 50, "Number of items per page")
	cmd.Flags().IntVar(&page, "page", 1, "Page number")
	cmd.Flags().StringVar(&sortDirection, "sort", "desc", "Sort direction (asc|desc)")
	cmd.Flags().StringVar(&textQuery, "text", "", "Search for text in audit logs")
	cmd.Flags().StringVar(&statusFilter, "status", "", "Filter by status (success|failed)")

	return cmd
}
