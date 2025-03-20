package kubiya

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"regexp"
	"sort"
	"strings"
	"time"

	"github.com/fatih/color"
	"github.com/mattn/go-isatty"
)

// WebhookTest represents a webhook test session
type WebhookTest struct {
	client              *Client
	query               AuditQuery
	webhookName         string
	sessionID           string
	conversationStarted bool
	processedEvents     map[string]bool
	agentName           string
}

// ConversationStep represents a step in the webhook conversation
type ConversationStep struct {
	Type      string // "webhook", "ai_message", "tool_execution"
	Timestamp time.Time
	Content   map[string]interface{}
}

// NewWebhookTest creates a new webhook test session
func NewWebhookTest(client *Client, webhookName string) *WebhookTest {
	// Set initial timestamp to 10 minutes ago to ensure we catch recent events
	// Use time.Now().UTC() to avoid timezone issues
	tenMinutesAgo := time.Now().UTC().Add(-10 * time.Minute).Format(time.RFC3339)

	fmt.Printf("🕒 Setting initial timestamp filter to: %s\n", tenMinutesAgo)
	fmt.Printf("🔍 Testing webhook with name: '%s'\n", webhookName)

	// Start with filters targeting webhook events specifically
	var categoryType, categoryName string

	// Set initial filters to target webhook events
	if os.Getenv("KUBIYA_BROAD_SEARCH") != "1" {
		// Default to focused webhook search
		categoryType = "triggers"
		categoryName = "webhook"
		fmt.Printf("🎯 Using focused search mode for webhook events\n")
	} else {
		// Use broader search if requested
		fmt.Printf("🔍 Using broad search mode to find any related events\n")
	}

	return &WebhookTest{
		client:      client,
		webhookName: webhookName,
		query: AuditQuery{
			Filter: AuditFilter{
				CategoryType: categoryType,
				CategoryName: categoryName,
				Timestamp: struct {
					GTE string `json:"gte,omitempty"`
					LTE string `json:"lte,omitempty"`
				}{
					GTE: tenMinutesAgo,
				},
			},
			Page:     1,
			PageSize: 50, // Increase page size for better chance of finding events
			Sort: AuditSort{
				Timestamp: -1, // Sort by timestamp descending
			},
		},
		processedEvents: make(map[string]bool),
	}
}

// StartTest begins monitoring webhook events in real-time
func (wt *WebhookTest) StartTest(ctx context.Context) error {
	// Record the current time in both local and UTC
	now := time.Now()
	nowUTC := now.UTC()

	fmt.Printf("🕒 Current time (local): %s\n", now.Format(time.RFC3339))
	fmt.Printf("🕒 Current time (UTC):   %s\n", nowUTC.Format(time.RFC3339))

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

	// Create a timeout context to ensure we don't wait forever
	timeout := 60 * time.Second
	if timeoutStr := os.Getenv("KUBIYA_WEBHOOK_TIMEOUT"); timeoutStr != "" {
		if t, err := time.ParseDuration(timeoutStr); err == nil && t > 0 {
			timeout = t
		}
	}
	timeoutCtx, cancel := context.WithTimeout(ctx, timeout)
	defer cancel()

	fmt.Printf("%s Will wait up to %s for webhook events before timing out\n",
		infoColor("⏱️"),
		timeout.String())

	// Print header
	fmt.Println(headerColor("\n=== Webhook Test Session Started ==="))
	fmt.Printf("%s Monitoring webhook '%s' events in real-time...\n\n",
		infoColor("→"),
		stepColor(wt.webhookName),
	)

	// Create a channel for audit items
	items, err := wt.client.Audit().StreamAuditItems(ctx, wt.query)
	if err != nil {
		return fmt.Errorf("failed to start audit stream: %w", err)
	}

	// Use polling mode instead of streaming mode
	ticker := time.NewTicker(5 * time.Second)
	defer ticker.Stop()

	// Keep track of if we've seen any events
	seenEvents := false
	noEventsPrinted := false

	// Keep track of latest timestamp for polling
	var latestTimestamp string

	// Print initial query configuration
	fmt.Printf("%s Starting with broad query (no category filters) to find any related events\n",
		infoColor("📊"))
	fmt.Printf("%s Will narrow down to specific webhook events once detected\n",
		infoColor("🔍"))

	// Replace with more informative search filter display:
	// Print current query configuration
	if wt.query.Filter.CategoryType != "" || wt.query.Filter.CategoryName != "" {
		fmt.Printf("%s Current search filters: category_type='%s', category_name='%s'\n",
			infoColor("🎯"),
			wt.query.Filter.CategoryType,
			wt.query.Filter.CategoryName)
	} else {
		fmt.Printf("%s Using broad search (no category filters) to find all related events\n",
			infoColor("🔍"))
	}

	// Process incoming audit items or poll for new items
	pollCount := 0
	startTime := time.Now()
	for {
		select {
		case <-timeoutCtx.Done():
			// Check if this is a timeout vs a normal cancellation
			if ctx.Err() == nil {
				fmt.Printf("\n%s Timed out after %s - no webhook events found\n",
					errorColor("⏰"),
					time.Since(startTime).Round(time.Second).String())
				fmt.Println(errorColor("Consider checking that the webhook was triggered correctly or try again with KUBIYA_FORCE_WEBHOOK_MATCH=1"))
			}
			fmt.Println("\n" + headerColor("=== Webhook Test Session Ended ==="))
			return nil
		case <-ctx.Done():
			fmt.Println("\n" + headerColor("=== Webhook Test Session Ended ==="))
			return nil
		case <-ticker.C:
			// Use polling instead of streaming if we've seen reconnection issues
			if items == nil {
				pollCount++
				verbose := os.Getenv("KUBIYA_VERBOSE")

				// Only show poll attempts periodically to reduce noise
				if pollCount == 1 || pollCount%10 == 0 || verbose == "1" {
					fmt.Printf("%s Poll attempt #%d - timestamp filter: %s\n",
						infoColor("🔄"),
						pollCount,
						wt.query.Filter.Timestamp.GTE)
				}

				// Update query with latest timestamp if available
				if latestTimestamp != "" {
					prevTimestamp := wt.query.Filter.Timestamp.GTE
					wt.query.Filter.Timestamp.GTE = latestTimestamp

					// Only print timestamp update logs in verbose mode
					if verbose == "1" {
						fmt.Printf("%s Updated timestamp filter: %s → %s\n",
							infoColor("🕰️"),
							prevTimestamp,
							latestTimestamp)
					}
				}

				// Every 5 polls without results, move timestamp back to detect more events
				if pollCount > 1 && pollCount%5 == 0 && !wt.conversationStarted {
					// Parse the current timestamp
					currentTimestamp, err := time.Parse(time.RFC3339, wt.query.Filter.Timestamp.GTE)
					if err == nil {
						// Move timestamp back by 5 more minutes
						newTimestamp := currentTimestamp.Add(-5 * time.Minute).Format(time.RFC3339)
						fmt.Printf("%s No events found after %d polls, moving timestamp back: %s → %s\n",
							infoColor("⏪"),
							pollCount,
							wt.query.Filter.Timestamp.GTE,
							newTimestamp)
						wt.query.Filter.Timestamp.GTE = newTimestamp
					}
				}

				// Add a small delay to avoid processing the exact same timestamp
				time.Sleep(100 * time.Millisecond)

				// Poll for new audit items
				auditItems, err := wt.client.Audit().ListAuditItems(ctx, wt.query)
				if err != nil {
					fmt.Printf("%s Error polling for audit items: %v\n", errorColor("❌"), err)
					continue
				}

				// Only print detailed poll results in verbose mode or when there are actual items
				if (verbose == "1" || len(auditItems) > 0) && len(auditItems) > 0 {
					// Print poll information only when we find related events
					fmt.Printf("%s Found %d new events in poll #%d\n",
						infoColor("📥"),
						len(auditItems),
						pollCount)
				}

				if len(auditItems) == 0 {
					if wt.conversationStarted {
						// Only print this message every 10 polls to reduce noise
						if pollCount%10 == 0 {
							fmt.Printf("%s No new events found for webhook '%s' (session: %s)\n",
								infoColor("ℹ️"),
								wt.webhookName,
								wt.sessionID)
						}
					} else if pollCount%5 == 0 {
						// Show helpful message periodically with webhook name
						fmt.Printf("%s Still waiting for '%s' webhook events. If this persists, consider:\n",
							infoColor("⏳"),
							wt.webhookName)
						fmt.Printf("   1. Checking if the webhook was triggered successfully\n")
						fmt.Printf("   2. Verifying the webhook ID is correct\n")
						fmt.Printf("   3. Setting KUBIYA_FORCE_WEBHOOK_MATCH=1 to match any webhook\n")
						fmt.Printf("   4. Increasing the search time window with KUBIYA_BROAD_SEARCH=1\n")
					}
					continue
				}

				// Sort items chronologically to process oldest first
				// This helps maintain proper conversation order
				sort.Slice(auditItems, func(i, j int) bool {
					return auditItems[i].Timestamp < auditItems[j].Timestamp
				})

				// Process sorted items
				for _, item := range auditItems {
					// Only print detailed processing logs in verbose mode
					if verbose == "1" {
						fmt.Printf("%s Processing audit item: type=%s, name=%s, action=%s, timestamp=%s\n",
							infoColor("🔍"),
							item.CategoryType,
							item.CategoryName,
							item.ActionType,
							item.Timestamp)
					}

					// Process each item
					wt.processAuditItem(item, &seenEvents, &noEventsPrinted, &latestTimestamp,
						successColor, errorColor, infoColor, headerColor, stepColor, detailColor)
				}

				// Show waiting message if appropriate
				if !wt.conversationStarted && !seenEvents && !noEventsPrinted {
					fmt.Println(infoColor("⏳ Waiting for webhook events... This might take a few moments."))
					fmt.Printf("%s Conversation started: %t, Seen events: %t\n",
						infoColor("ℹ️"),
						wt.conversationStarted,
						seenEvents)
					noEventsPrinted = true
				}
			}
		case item, ok := <-items:
			if !ok {
				// Channel closed, switch to polling mode
				fmt.Println(infoColor("📊 Switching to polling mode for reliable monitoring..."))
				items = nil
				continue
			}

			// Process the streaming item
			fmt.Printf("%s Stream received item: type=%s, name=%s, action=%s\n",
				infoColor("📩"),
				item.CategoryType,
				item.CategoryName,
				item.ActionType)

			wt.processAuditItem(item, &seenEvents, &noEventsPrinted, &latestTimestamp,
				successColor, errorColor, infoColor, headerColor, stepColor, detailColor)
		}
	}
}

// processAuditItem handles processing a single audit item
func (wt *WebhookTest) processAuditItem(
	item AuditItem,
	seenEvents *bool,
	noEventsPrinted *bool,
	latestTimestamp *string,
	successColor, errorColor, infoColor, headerColor, stepColor, detailColor func(a ...interface{}) string,
) {
	// Update seen events flag
	*seenEvents = true

	verbose := os.Getenv("KUBIYA_VERBOSE")

	// Keep track of latest timestamp for polling mode
	if item.Timestamp > *latestTimestamp {
		*latestTimestamp = item.Timestamp
		if verbose == "1" {
			fmt.Printf("%s Updated latest timestamp: %s\n",
				infoColor("🕒"),
				*latestTimestamp)
		}
	}

	// Parse timestamp
	timestamp, err := time.Parse(time.RFC3339, item.Timestamp)
	if err != nil {
		timestamp = time.Now()
	}

	// If conversation hasn't started, look for the trigger event
	if !wt.conversationStarted {
		// Identify webhook trigger events - multiple detection strategies
		isWebhookTrigger := false
		isTargetWebhook := false

		// Strategy 1: Check for standard webhook trigger event format
		if item.CategoryType == "triggers" && item.CategoryName == "webhook" && item.ActionType == "received" {
			isWebhookTrigger = true

			// Check if this is the webhook we're looking for
			if strings.Contains(item.ResourceText, wt.webhookName) {
				isTargetWebhook = true
				fmt.Printf("%s Found target webhook '%s' using standard detection\n",
					infoColor("✅"),
					wt.webhookName)
			} else {
				fmt.Printf("%s Found webhook trigger but name '%s' doesn't match our target '%s'\n",
					infoColor("⚠️"),
					item.ResourceText,
					wt.webhookName)
			}
		}

		// Strategy 2: Check for webhook event with resource type
		if !isWebhookTrigger && item.CategoryType == "webhook" && (item.ResourceType == "webhook" || strings.Contains(strings.ToLower(item.ResourceText), "webhook")) {
			isWebhookTrigger = true

			// Check if this is the webhook we're looking for
			if strings.Contains(item.ResourceText, wt.webhookName) {
				isTargetWebhook = true
				fmt.Printf("%s Found target webhook '%s' using resource type detection\n",
					infoColor("✅"),
					wt.webhookName)
			}
		}

		// Strategy 3: Check for webhook-related action types
		if !isWebhookTrigger && (item.ActionType == "triggered" || item.ActionType == "executed" || item.ActionType == "invoked") {
			isWebhookTrigger = true

			// Check if this is the webhook we're looking for
			if strings.Contains(item.ResourceText, wt.webhookName) {
				isTargetWebhook = true
				fmt.Printf("%s Found target webhook '%s' using action type detection\n",
					infoColor("✅"),
					wt.webhookName)
			}
		}

		// Only consider the webhook if it's the one we're testing or we're in forced match mode
		if !isTargetWebhook && isWebhookTrigger {
			// If forced matching is enabled, or the webhook name is empty, still accept it
			if os.Getenv("KUBIYA_FORCE_WEBHOOK_MATCH") == "1" || item.ResourceText == "" {
				isTargetWebhook = true
				fmt.Printf("%s Force-accepting webhook trigger despite name mismatch ('%s' vs '%s')\n",
					infoColor("⚠️"),
					item.ResourceText,
					wt.webhookName)
			}
		}

		// Print extra info about the item for debugging
		if verbose == "1" {
			fmt.Printf("%s Full audit item details:\n", infoColor("📋"))
			fmt.Printf("   Resource Type: %s\n", item.ResourceType)
			fmt.Printf("   Resource Text: %s\n", item.ResourceText)
			fmt.Printf("   Category Type: %s\n", item.CategoryType)
			fmt.Printf("   Category Name: %s\n", item.CategoryName)
			fmt.Printf("   Action Type: %s\n", item.ActionType)
			fmt.Printf("   Action Successful: %t\n", item.ActionSuccessful)
			fmt.Printf("   Timestamp: %s\n", item.Timestamp)
			fmt.Printf("   Extra: %v\n", getMapKeys(item.Extra))
		}

		if isWebhookTrigger && isTargetWebhook {
			// This is a webhook trigger event for our target webhook
			wt.conversationStarted = true
			fmt.Printf("%s Found webhook trigger event! Conversation started.\n", infoColor("✨"))

			// Extract session ID from Extra if available
			if sessionID, ok := item.Extra["session_id"].(string); ok {
				wt.sessionID = sessionID
				fmt.Printf("%s Found session ID: %s\n",
					infoColor("🔍"),
					wt.sessionID,
				)

				// Also, try to check for related fields that might indicate what action will happen
				for _, potentialField := range []string{"action", "action_type", "event_type", "operation", "webhook_type"} {
					if val, ok := item.Extra[potentialField]; ok {
						fmt.Printf("%s Found potential trigger info: %s = %v\n",
							infoColor("🔍"),
							potentialField,
							val)
					}
				}
			} else {
				// Try to find a session ID or similar field in the Extra data
				sessionIDCandidates := []string{"session_id", "sessionId", "session", "id", "request_id", "requestId"}
				for _, candidate := range sessionIDCandidates {
					if val, ok := item.Extra[candidate].(string); ok {
						wt.sessionID = val
						fmt.Printf("%s Found alternative session ID from '%s': %s\n",
							infoColor("🔍"),
							candidate,
							wt.sessionID,
						)
						break
					}
				}

				if wt.sessionID == "" {
					fmt.Printf("%s No session_id found in Extra data. Extra keys: %v\n",
						infoColor("⚠️"),
						getMapKeys(item.Extra))
				}
			}

			// Get webhook name from resource if available or use the provided name
			webhookName := wt.webhookName
			if item.ResourceText != "" {
				webhookName = item.ResourceText
			}

			// Update query to look for all events with this session ID, not just triggers
			fmt.Printf("%s Updating query to be more inclusive for related events\n", infoColor("🔄"))

			// Clear previous category filters and search by a broader set of criteria
			wt.query.Filter.CategoryType = "" // Clear the category type filter
			wt.query.Filter.CategoryName = "" // Clear the category name filter

			// Add session ID to query but leave it empty to get ALL events
			wt.query.Filter.SessionID = "" // Don't limit by session ID

			// Store additional tracking information
			wt.agentName = "WebhookSanity" // This is the agent that typically handles webhooks
			if strings.Contains(item.ResourceText, "Webhook") || strings.Contains(webhookName, "Webhook") {
				wt.agentName = strings.TrimSpace(item.ResourceText)
			}
			fmt.Printf("%s Will track events for agent: %s\n", infoColor("🔍"), wt.agentName)

			// Print conversation start
			fmt.Printf("%s %s\n",
				headerColor("⚡ Webhook Triggered:"),
				infoColor(timestamp.Format("15:04:05")),
			)
			fmt.Printf("   %s %s\n",
				stepColor("Name:"),
				webhookName,
			)
			if wt.sessionID != "" {
				fmt.Printf("   %s %s\n",
					stepColor("Session:"),
					wt.sessionID,
				)
			}
			fmt.Println(strings.Repeat("─", 80))
			return
		} else if isWebhookTrigger && !isTargetWebhook {
			// This is a webhook trigger but not for our target
			if verbose == "1" {
				fmt.Printf("%s Skipping non-target webhook event: %s\n",
					infoColor("⏭️"),
					item.ResourceText)
			}
			return
		} else {
			// Only show non-trigger events in verbose mode
			if verbose == "1" {
				fmt.Printf("%s Not a trigger event: type=%s, name=%s, action=%s\n",
					infoColor("⏭️"),
					item.CategoryType,
					item.CategoryName,
					item.ActionType)
			}
		}
		// Skip events until conversation starts, but don't return
		return
	}

	// For started conversations, track events related to the webhook
	if wt.conversationStarted {
		// First, check for exact session ID match
		eventSessionID, _ := item.Extra["session_id"].(string)

		// Define a function to check if this event is related to our webhook
		isRelatedEvent := func() bool {
			// Exact session ID match is always related (high priority)
			if eventSessionID == wt.sessionID && eventSessionID != "" {
				if verbose == "1" {
					fmt.Printf("%s Event matched by exact session ID: %s\n", infoColor("✅"), wt.sessionID)
				}
				return true
			}

			// Direct webhook triggers for our specific webhook are always relevant
			if item.CategoryType == "triggers" && item.CategoryName == "webhook" {
				if item.ResourceText == wt.webhookName || strings.Contains(item.ResourceText, wt.webhookName) {
					if verbose == "1" {
						fmt.Printf("%s Event matched by webhook name: %s\n", infoColor("✅"), wt.webhookName)
					}
					return true
				}
			}

			// For agent events (WebhookSanity), be more strict by:
			// 1. Requiring the event to mention our webhook name somewhere
			// 2. Or requiring it to have our session ID
			// 3. Or requiring the event to be very close in time to our trigger (within 10 seconds)
			if item.CategoryType == "agents" {
				// Check if the webhookName is mentioned in any field
				webhookNameMentioned := false

				// Check ResourceText
				if strings.Contains(item.ResourceText, wt.webhookName) {
					webhookNameMentioned = true
				}

				// Check if any Extra field contains our webhook name
				for _, value := range item.Extra {
					if strValue, ok := value.(string); ok {
						if strings.Contains(strValue, wt.webhookName) {
							webhookNameMentioned = true
							break
						}
					}
				}

				// Check if this is very close in time to our trigger
				isTimeProximate := false
				if *latestTimestamp != "" {
					eventTime, err := time.Parse(time.RFC3339, item.Timestamp)
					if err == nil {
						triggerTime, triggerErr := time.Parse(time.RFC3339, *latestTimestamp)
						if triggerErr == nil {
							timeDiff := eventTime.Sub(triggerTime)
							// Only events within 10 seconds are considered related
							if timeDiff >= -10*time.Second && timeDiff <= 10*time.Second {
								isTimeProximate = true
							}
						}
					}
				}

				// Accept the event if it meets our stricter criteria
				if webhookNameMentioned || (isTimeProximate && wt.conversationStarted) {
					return true
				}
			}

			return false
		}

		// If not related, log and skip
		if !isRelatedEvent() {
			// Only show unrelated event skip messages in verbose mode
			if verbose == "1" {
				fmt.Printf("%s Skipping unrelated event: type=%s, name=%s, action=%s, session=%s\n",
					infoColor("⏭️"),
					item.CategoryType,
					item.CategoryName,
					item.ActionType,
					eventSessionID)
			}
			return
		}

		// For related events, still avoid duplicates
		eventKey := fmt.Sprintf("%s-%s-%s-%s", item.Timestamp, item.CategoryType, item.CategoryName, item.ActionType)

		// Skip if we've already processed this event
		if _, seen := wt.processedEvents[eventKey]; seen {
			// Only show skipped event messages in verbose mode
			if verbose == "1" {
				fmt.Printf("%s Skipping already processed event: %s\n",
					infoColor("⏭️"),
					eventKey)
			}
			return
		}

		// Mark this event as processed
		wt.processedEvents[eventKey] = true

		// Log relationship reason
		if verbose == "1" {
			if eventSessionID == wt.sessionID {
				fmt.Printf("%s Event matched by session ID: %s\n", infoColor("🔗"), wt.sessionID)
			} else if item.CategoryType == "agents" && item.CategoryName == wt.agentName {
				fmt.Printf("%s Event matched by agent name: %s\n", infoColor("🔗"), wt.agentName)
			} else {
				fmt.Printf("%s Event matched by other criteria\n", infoColor("🔗"))
			}
		}
	}

	// Print event based on category type
	switch item.CategoryType {
	case "ai":
		fmt.Printf("%s %s\n",
			headerColor("🤖 AI Message:"),
			infoColor(timestamp.Format("15:04:05")),
		)
		if content, ok := item.Extra["content"].(string); ok {
			fmt.Printf("   %s\n", detailColor(content))
		}

	case "tool_execution":
		fmt.Printf("%s %s\n",
			headerColor("🔧 Tool Execution:"),
			infoColor(timestamp.Format("15:04:05")),
		)
		fmt.Printf("   %s %s\n",
			stepColor("Tool:"),
			item.ResourceText,
		)
		if result, ok := item.Extra["result"].(map[string]interface{}); ok {
			resultJSON, _ := json.MarshalIndent(result, "   ", "  ")
			fmt.Printf("   %s\n   %s\n",
				stepColor("Result:"),
				detailColor(string(resultJSON)),
			)
		}

	case "webhook":
		fmt.Printf("%s %s\n",
			headerColor("📡 Webhook Event:"),
			infoColor(timestamp.Format("15:04:05")),
		)
		fmt.Printf("   %s %s\n",
			stepColor("Action:"),
			item.ActionType,
		)
		// Print status with appropriate color
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
			statusColor(status),
		)

		// Print extra details if available
		if len(item.Extra) > 0 {
			fmt.Printf("   %s\n", stepColor("Details:"))
			for key, value := range item.Extra {
				if key == "session_id" {
					continue // Skip session_id in output
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
					detailColor(valueStr),
				)
			}
		}

	case "triggers":
		fmt.Printf("%s %s\n",
			headerColor("🔔 Trigger Event:"),
			infoColor(timestamp.Format("15:04:05")),
		)
		fmt.Printf("   %s %s\n",
			stepColor("Trigger Type:"),
			item.CategoryName,
		)
		fmt.Printf("   %s %s\n",
			stepColor("Action:"),
			item.ActionType,
		)

		// Print extra details if available
		if len(item.Extra) > 0 {
			fmt.Printf("   %s\n", stepColor("Details:"))
			for key, value := range item.Extra {
				if key == "session_id" {
					continue // Skip session_id in output
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
					detailColor(valueStr),
				)
			}
		}

	case "agents":
		// Simplify the agent interaction header, keep just the timestamp
		headerPrefix := "👤"
		if item.ResourceType == "Tool Execution" {
			headerPrefix = "🔧"
		} else if item.ResourceType == "Session" {
			headerPrefix = "📝"
		}

		fmt.Printf("%s %s %s\n",
			headerColor(headerPrefix+" "+item.CategoryName+":"),
			infoColor(timestamp.Format("15:04:05")),
			stepColor(item.ActionType),
		)

		// Always show Extra data keys in verbose mode to help debug
		if verbose == "1" {
			fmt.Printf("   %s %v\n",
				stepColor("Available fields:"),
				getMapKeys(item.Extra))
		}

		// Flag to track if we've already displayed content
		contentDisplayed := false

		// Parse resource_text field which often contains the actual message content
		if item.ResourceType == "Message" && item.ResourceText != "" {
			// Try to extract content from resource_text
			resourceText := item.ResourceText

			// Try different parsing strategies for different resource_text formats
			messageContent := ""

			// Strategy 1: Pattern: end=False start=False type='msg' content="..."
			if strings.Contains(resourceText, "type='msg'") && strings.Contains(resourceText, "content=") {
				contentStartIndex := strings.Index(resourceText, "content=")
				if contentStartIndex > 0 {
					contentStartIndex += 8 // length of "content="

					// Find the quote character used (either " or ')
					quoteChar := ""
					if contentStartIndex < len(resourceText) {
						if resourceText[contentStartIndex] == '"' {
							quoteChar = "\""
							contentStartIndex++ // skip the opening quote
						} else if resourceText[contentStartIndex] == '\'' {
							quoteChar = "'"
							contentStartIndex++ // skip the opening quote
						}
					}

					if quoteChar != "" {
						// Find the closing quote, taking into account escaped quotes
						contentEndIndex := -1
						inEscape := false
						for i := 0; i < len(resourceText[contentStartIndex:]); i++ {
							if inEscape {
								inEscape = false
								continue
							}

							if resourceText[contentStartIndex+i] == '\\' {
								inEscape = true
								continue
							}

							if resourceText[contentStartIndex+i] == quoteChar[0] {
								contentEndIndex = i
								break
							}
						}

						if contentEndIndex > 0 {
							messageContent = resourceText[contentStartIndex : contentStartIndex+contentEndIndex]

							// Unescape any escaped quotes
							messageContent = strings.ReplaceAll(messageContent, "\\"+quoteChar, quoteChar)

							if verbose == "1" {
								fmt.Printf("   %s Extracted from resource_text (msg pattern)\n", infoColor("📝"))
							}
						}
					}
				}
			}
			// Strategy 2: Direct value if it looks like plain text (for user messages)
			if !strings.HasPrefix(resourceText, "end=") && !strings.Contains(resourceText, "type=") {
				messageContent = resourceText
				if verbose == "1" {
					fmt.Printf("   %s Using resource_text directly as content\n", infoColor("📝"))
				}
			}

			// If we found content, display it
			if messageContent != "" {
				// For agent sent events, display the content prominently
				if item.ActionType == "sent" {
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

					// Format the output for better readability by replacing the Type line with a clearer header
					fmt.Printf("   %s %s %s\n",
						messageIcon,
						headerColor(messageDirection+":"),
						detailColor(messageContent))

					// Mark that we've displayed content
					contentDisplayed = true
				} else {
					// For other events, just show the content
					fmt.Printf("   %s\n   %s\n",
						stepColor("Content:"),
						detailColor(messageContent))

					// Mark that we've displayed content
					contentDisplayed = true
				}
			}

			// For tool calls, still parse and show separately
			if item.ActionType == "sent" && strings.HasPrefix(resourceText, "end=") && strings.Contains(resourceText, "type='tool'") {
				// This is a tool call event
				fmt.Printf("   %s %s\n",
					stepColor("Type:"),
					"🔧 Tool Execution")

				// Extract tool name
				toolNameMatch := regexp.MustCompile(`name=['"]([^'"]+)['"]`).FindStringSubmatch(resourceText)
				if len(toolNameMatch) > 1 {
					fmt.Printf("   %s %s\n",
						stepColor("Tool:"),
						toolNameMatch[1])
				}

				// Extract arguments
				argsMatch := regexp.MustCompile(`arguments=['"]({[^'"]+})['"]`).FindStringSubmatch(resourceText)
				if len(argsMatch) > 1 {
					fmt.Printf("   %s %s\n",
						stepColor("Args:"),
						detailColor(argsMatch[1]))

					// Mark that we've displayed content
					contentDisplayed = true
				}
			}
		}

		// Check for tool output events
		if item.ResourceType == "Tool Execution" && item.ResourceText != "" {
			// Pattern for tool output events
			resourceText := item.ResourceText

			// Try to extract content from the tool execution output
			if strings.Contains(resourceText, "type='tool_output'") && strings.Contains(resourceText, "content='") {
				contentStartIndex := strings.Index(resourceText, "content='")
				if contentStartIndex > 0 {
					contentStartIndex += 9 // length of "content='"

					// Find the closing quote (considering possible escaping)
					contentEndIndex := -1
					for i := len(resourceText) - 1; i > contentStartIndex; i-- {
						if resourceText[i] == '\'' && (i == len(resourceText)-1 || resourceText[i+1] != ' ') {
							contentEndIndex = i
							break
						}
					}

					if contentEndIndex > contentStartIndex {
						outputContent := resourceText[contentStartIndex:contentEndIndex]

						// Clean up output by removing duplicate lines (common in tool output)
						outputLines := strings.Split(outputContent, "\n")
						uniqueLines := []string{}
						seen := make(map[string]bool)

						for _, line := range outputLines {
							trimmedLine := strings.TrimSpace(line)
							if trimmedLine != "" && !seen[trimmedLine] {
								seen[trimmedLine] = true
								uniqueLines = append(uniqueLines, line)
							}
						}

						cleanedOutput := strings.Join(uniqueLines, "\n")

						// Display the extracted tool output
						fmt.Printf("   %s %s\n",
							stepColor("Type:"),
							"🔧 Tool Output")

						// Extract tool name from resource text if available
						toolNameMatch := regexp.MustCompile(`tool_name=['"]([^'"]+)['"]`).FindStringSubmatch(resourceText)
						if len(toolNameMatch) > 1 {
							fmt.Printf("   %s %s\n",
								stepColor("Tool:"),
								toolNameMatch[1])
						}

						fmt.Printf("   %s\n   %s\n",
							stepColor("Output:"),
							detailColor(cleanedOutput))

						if verbose == "1" {
							fmt.Printf("   %s Extracted from resource_text\n", infoColor("📝"))
						}

						// Mark that we've displayed content
						contentDisplayed = true
					}
				}
			}
		}

		// Check for specific action types to display appropriate information
		if item.ActionType == "sent" {
			// Display message content for "sent" actions
			isUserMessage := false
			if val, ok := item.Extra["is_user_message"].(bool); ok {
				isUserMessage = val
			}

			// Determine message type/direction
			messageIcon := "🤖" // Default: bot message
			messageDirection := "Agent response"
			if isUserMessage {
				messageIcon = "👤"
				messageDirection = "User message"
			}

			fmt.Printf("   %s %s\n",
				stepColor("Type:"),
				fmt.Sprintf("%s %s", messageIcon, messageDirection))

			// Try to extract message content from various possible locations
			var messageContent string
			var foundPaths []string

			// Check common content field names
			contentFields := []string{"content", "message", "text", "body", "response", "prompt", "query", "answer"}
			for _, field := range contentFields {
				if content, ok := item.Extra[field].(string); ok && content != "" {
					messageContent = content
					foundPaths = append(foundPaths, field)
					if verbose == "1" {
						fmt.Printf("   %s Found content in field: %s\n", infoColor("📝"), field)
					}
					break
				}
			}

			// If still empty, try recursive search for content in nested maps
			if messageContent == "" {
				contentPaths := []string{}
				messageContent = findNestedContent(item.Extra, contentFields, "Extra", &contentPaths)
				if messageContent != "" && len(contentPaths) > 0 {
					foundPaths = contentPaths
					if verbose == "1" {
						fmt.Printf("   %s Found content in nested path: %v\n", infoColor("📝"), contentPaths)
					}
				}
			}

			// If we still didn't find content but have Extra data, dump it all as JSON in verbose mode
			if messageContent == "" && !contentDisplayed {
				verbose := os.Getenv("KUBIYA_VERBOSE")
				if verbose == "1" && len(item.Extra) > 0 {
					extraJSON, _ := json.MarshalIndent(item.Extra, "      ", "  ")
					fmt.Printf("   %s\n   %s\n",
						stepColor("Complete Extra data:"),
						detailColor(string(extraJSON)))
				}
			}

			if messageContent != "" {
				fmt.Printf("   %s\n   %s\n",
					stepColor("Content:"),
					detailColor(messageContent))
				contentDisplayed = true
			} else if !contentDisplayed {
				fmt.Printf("   %s %s\n",
					infoColor("⚠️"),
					"No content found in event data")
			}
		} else if item.ActionType == "output" || item.ActionType == "executed" {
			// Show command output for tool executions
			fmt.Printf("   %s %s\n",
				stepColor("Tool:"),
				item.ResourceText,
			)

			// Try to get the output or result from various possible fields
			var outputContent string
			outputFields := []string{"output", "result", "response", "data", "content"}

			// Try string fields first
			for _, field := range outputFields {
				if output, ok := item.Extra[field].(string); ok && output != "" {
					outputContent = output
					if verbose == "1" {
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
						if verbose == "1" {
							fmt.Printf("   %s Found object output in field: %s\n", infoColor("📝"), field)
						}
						break
					}
				}
			}

			// If we still didn't find output but have Extra data, dump it all as JSON in verbose mode
			if outputContent == "" && !contentDisplayed {
				verbose := os.Getenv("KUBIYA_VERBOSE")
				if verbose == "1" && len(item.Extra) > 0 {
					extraJSON, _ := json.MarshalIndent(item.Extra, "      ", "  ")
					fmt.Printf("   %s\n   %s\n",
						stepColor("Complete Extra data:"),
						detailColor(string(extraJSON)))
				}
			}

			if outputContent != "" {
				fmt.Printf("   %s\n   %s\n",
					stepColor("Result:"),
					detailColor(outputContent))
				contentDisplayed = true
			} else if !contentDisplayed {
				fmt.Printf("   %s %s\n",
					infoColor("⚠️"),
					"No output content found in event data")
			}
		} else {
			// For other action types, show all extra data
			if len(item.Extra) > 0 {
				fmt.Printf("   %s\n", stepColor("Details:"))
				for key, value := range item.Extra {
					if key == "session_id" {
						continue // Skip session_id in output
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
				contentDisplayed = true
			}
		}

	default:
		// Generic handler for any other event types
		fmt.Printf("%s %s\n",
			headerColor(fmt.Sprintf("📄 %s Event:", item.CategoryType)),
			infoColor(timestamp.Format("15:04:05")),
		)
		fmt.Printf("   %s %s\n",
			stepColor("Type:"),
			item.CategoryType,
		)
		if item.CategoryName != "" {
			fmt.Printf("   %s %s\n",
				stepColor("Name:"),
				item.CategoryName,
			)
		}
		fmt.Printf("   %s %s\n",
			stepColor("Action:"),
			item.ActionType,
		)

		// Print extra details if available
		if len(item.Extra) > 0 {
			fmt.Printf("   %s\n", stepColor("Details:"))
			for key, value := range item.Extra {
				if key == "session_id" {
					continue // Skip session_id in output
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
					detailColor(valueStr),
				)
			}
		}
	}

	// Print separator between events
	fmt.Println(strings.Repeat("─", 80))
}

// PrintWebhookEvent prints a single webhook event in a formatted way
func PrintWebhookEvent(w io.Writer, item AuditItem) {
	// Check if we're writing to a terminal
	isTerminal := false
	if f, ok := w.(*os.File); ok {
		isTerminal = isatty.IsTerminal(f.Fd())
	}

	// Create color instances for different states
	var successColor, errorColor, infoColor, headerColor func(a ...interface{}) string
	if isTerminal {
		successColor = color.New(color.FgGreen).SprintFunc()
		errorColor = color.New(color.FgRed).SprintFunc()
		infoColor = color.New(color.FgCyan).SprintFunc()
		headerColor = color.New(color.FgYellow, color.Bold).SprintFunc()
	} else {
		// If not in terminal, don't use colors
		successColor = fmt.Sprint
		errorColor = fmt.Sprint
		infoColor = fmt.Sprint
		headerColor = fmt.Sprint
	}

	// Format timestamp
	timestamp, err := time.Parse(time.RFC3339, item.Timestamp)
	if err != nil {
		timestamp = time.Now()
	}

	// Print event header
	fmt.Fprintf(w, "\n%s %s\n",
		infoColor(timestamp.Format("15:04:05")),
		headerColor("Webhook Event:"),
	)

	// Print event details
	fmt.Fprintf(w, "  %s: %s\n", infoColor("Resource"), item.ResourceText)
	fmt.Fprintf(w, "  %s: %s\n", infoColor("Action"), item.ActionType)

	// Print status with appropriate color
	status := "Success"
	if !item.ActionSuccessful {
		status = "Failed"
	}
	statusColor := successColor
	if !item.ActionSuccessful {
		statusColor = errorColor
	}
	fmt.Fprintf(w, "  %s: %s\n", infoColor("Status"), statusColor(status))

	// Print extra details if available
	if len(item.Extra) > 0 {
		fmt.Fprintf(w, "  %s:\n", infoColor("Details"))
		for key, value := range item.Extra {
			// Format the value nicely
			var valueStr string
			switch v := value.(type) {
			case string:
				valueStr = v
			case map[string]interface{}:
				// Convert map to JSON string
				if jsonBytes, err := json.MarshalIndent(v, "    ", "  "); err == nil {
					valueStr = string(jsonBytes)
				} else {
					valueStr = fmt.Sprintf("%v", v)
				}
			default:
				valueStr = fmt.Sprintf("%v", v)
			}

			// Print with proper indentation
			lines := strings.Split(valueStr, "\n")
			for i, line := range lines {
				if i == 0 {
					fmt.Fprintf(w, "    %s: %s\n", key, line)
				} else {
					fmt.Fprintf(w, "    %s\n", line)
				}
			}
		}
	}

	// Print separator
	fmt.Fprintln(w, strings.Repeat("-", 80))
}

// Helper function to get map keys as string slice
func getMapKeys(m map[string]interface{}) []string {
	keys := make([]string, 0, len(m))
	for k := range m {
		keys = append(keys, k)
	}
	return keys
}

// Helper function to recursively search for message content in nested maps
func findNestedContent(data map[string]interface{}, contentKeys []string, path string, foundPaths *[]string) string {
	// Try direct fields first
	for _, key := range contentKeys {
		if val, ok := data[key]; ok {
			if strVal, ok := val.(string); ok && strVal != "" {
				*foundPaths = append(*foundPaths, path+"."+key)
				return strVal
			}
		}
	}

	// Look for nested maps
	for k, v := range data {
		if nestedMap, ok := v.(map[string]interface{}); ok {
			if content := findNestedContent(nestedMap, contentKeys, path+"."+k, foundPaths); content != "" {
				return content
			}
		}
	}

	return ""
}
