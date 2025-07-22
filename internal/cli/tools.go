package cli

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"os/signal"
	"sort"
	"strconv"
	"strings"
	"text/tabwriter"
	"time"

	"github.com/kubiyabot/cli/internal/config"
	"github.com/kubiyabot/cli/internal/kubiya"
	"github.com/kubiyabot/cli/internal/sentry"
	"github.com/kubiyabot/cli/internal/style"
	"github.com/spf13/afero"
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
		newGenerateToolCommand(cfg),
		newExecToolCommand(cfg),
		newToolIntegrationsCommand(cfg),
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
				// Also include inline tools if available
				tools = append(tools, source.InlineTools...)
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
					// Also include inline tools if available
					tools = append(tools, metadata.InlineTools...)
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
				// Also check inline tools if not found
				if tool == nil {
					for _, t := range source.InlineTools {
						if t.Name == toolName {
							tool = &t
							sourceName = source.Name
							break
						}
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
					// Check regular tools
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
					// Check inline tools if not found
					for _, t := range metadata.InlineTools {
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
	usage := tool.Name
	for _, arg := range tool.Args {
		if arg.Required {
			usage += fmt.Sprintf(" <%s>", arg.Name)
		} else {
			usage += fmt.Sprintf(" [%s]", arg.Name)
		}
	}
	return usage
}

func newExecToolCommand(cfg *config.Config) *cobra.Command {
	var (
		runner          string
		toolName        string
		toolDesc        string
		toolType        string
		image           string
		content         string
		watch           bool
		jsonFile        string
		jsonInput       string
		outputFormat    string
		skipHealthCheck bool
		skipPolicyCheck bool
		timeout         int
		integrations    []string
		withFiles       []string
		withVolumes     []string
		withServices    []string
		envVars         []string
		args            []string
		argsJSON        string
		iconURL         string
		toolURL         string
		sourceUUID      string
	)

	cmd := &cobra.Command{
		Use:   "exec",
		Short: "🚀 Execute a tool with streaming output",
		Long: `Execute a tool directly by providing its definition.
The tool execution will stream output in real-time.

By default, the runner is set to "auto" which will:
1. Try to use 'kubiya-hosted' if it's healthy
2. Fall back to the first available healthy runner if kubiya-hosted is not available
3. Show which runner was selected and why

Environment Variables:
  KUBIYA_TOOL_TIMEOUT       - Default timeout in seconds (default: 300)
  KUBIYA_TOOL_RUNNER        - Default runner (default: auto) 
  KUBIYA_DEFAULT_RUNNER     - Default runner when "default" is specified or runner is empty
  KUBIYA_TOOL_OUTPUT_FORMAT - Default output format: text or stream-json (default: text)
  KUBIYA_TOOL_TYPE          - Default tool type (default: docker)
  KUBIYA_SKIP_HEALTH_CHECK  - Skip runner health check if set to "true" or "1"`,
		Example: `  # Execute a simple bash tool (auto runner selection)
  kubiya tool exec --name "hello" --content "echo Hello World"

  # Execute a Python tool with custom image
  kubiya tool exec --name "python-script" --type docker --image python:3.11 \
    --content "print('Hello from Python')"

  # Execute with a specific runner
  kubiya tool exec --name "test" --content "date" --runner core-testing-1

  # Execute a tool from JSON file
  kubiya tool exec --json-file tool.json

  # Execute with direct JSON input
  kubiya tool exec --json '{"name":"test","type":"docker","image":"alpine","content":"echo hello"}' 

  # Execute with raw JSON stream output
  kubiya tool exec --name "test" --content "date" --output stream-json

  # Execute with custom timeout (in seconds)
  kubiya tool exec --name "long-job" --content "sleep 60" --timeout 120

  # Execute with Kubernetes integration (in-cluster auth)
  kubiya tool exec --name "k8s-pods" --content "kubectl get pods -A" \
    --integration k8s/incluster

  # Execute with AWS credentials
  kubiya tool exec --name "aws-s3" --content "aws s3 ls" \
    --integration aws/creds

  # Execute with multiple integrations
  kubiya tool exec --name "deploy" --content "./deploy.sh" \
    --integration k8s/incluster --integration aws/creds

  # Execute with file mappings
  kubiya tool exec --name "config-tool" --content "cat /app/config.yaml" \
    --with-file ~/.config/myapp.yaml:/app/config.yaml

  # Execute with volumes (read-only)
  kubiya tool exec --name "docker-inspect" --content "docker ps -a" \
    --with-volume /var/run/docker.sock:/var/run/docker.sock:ro

  # Execute with environment variables
  kubiya tool exec --name "env-test" --content "echo \$MY_VAR" \
    --env MY_VAR=hello --env ANOTHER_VAR=world

  # Complex AWS tool with arguments
  kubiya tool exec --name "ec2-describe" --type docker \
    --image amazon/aws-cli:latest \
    --content 'aws ec2 describe-instances $([[ -n "$instance_ids" ]] && echo "--instance-ids $instance_ids")' \
    --arg instance_ids:string:"Comma-separated instance IDs":false \
    --with-file ~/.aws/credentials:/root/.aws/credentials \
    --env AWS_PROFILE=production

  # Execute a tool from a URL
  kubiya tool exec --tool-url https://raw.githubusercontent.com/kubiyabot/community-tools/main/aws/tools/ec2_describe_instances.yaml

  # Execute a tool from a source UUID
  kubiya tool exec --source-uuid 64b0cb09-d6b5-4ff7-9d4b-9e05c6c3ae56 --name ec2_describe_instances

  # Execute a tool with arguments (JSON format)
  kubiya tool exec --name "my-tool" --content "echo Hello \$name" \
    --args '{"name":"World","debug":true}'

  # Execute a tool from source with arguments
  kubiya tool exec --source-uuid abc123 --name "parameterized-tool" \
    --args '{"region":"us-east-1","instance_count":3}'`,
		RunE: func(cmd *cobra.Command, args []string) error {
			return sentry.WithTransaction(cmd.Context(), "tool_execute", func(ctx context.Context) error {
				client := kubiya.NewClient(cfg)

				// Add breadcrumb for tool execution
				sentry.AddBreadcrumb("tool", "Tool execution started", map[string]interface{}{
					"tool_name":     toolName,
					"source_uuid":   sourceUUID,
					"runner":        runner,
					"timeout":       timeout,
					"output_format": outputFormat,
				})

				// Apply environment variable defaults if flags were not set
				if cmd.Flags().Changed("timeout") == false {
					if envTimeout := os.Getenv("KUBIYA_TOOL_TIMEOUT"); envTimeout != "" {
						if t, err := strconv.Atoi(envTimeout); err == nil {
							timeout = t
							fmt.Printf("%s Using timeout from KUBIYA_TOOL_TIMEOUT: %d seconds\n",
								style.DimStyle.Render("🔧"), timeout)
						}
					}
				}

				if cmd.Flags().Changed("runner") == false && runner == "" {
					if envRunner := os.Getenv("KUBIYA_TOOL_RUNNER"); envRunner != "" {
						runner = envRunner
						fmt.Printf("%s Using runner from KUBIYA_TOOL_RUNNER: %s\n",
							style.DimStyle.Render("🔧"), runner)
					}
				}

				if cmd.Flags().Changed("output") == false {
					if envOutput := os.Getenv("KUBIYA_TOOL_OUTPUT_FORMAT"); envOutput != "" {
						outputFormat = envOutput
						fmt.Printf("%s Using output format from KUBIYA_TOOL_OUTPUT_FORMAT: %s\n",
							style.DimStyle.Render("🔧"), outputFormat)
					}
				}

				if cmd.Flags().Changed("type") == false && toolType == "" {
					if envType := os.Getenv("KUBIYA_TOOL_TYPE"); envType != "" {
						toolType = envType
						fmt.Printf("%s Using tool type from KUBIYA_TOOL_TYPE: %s\n",
							style.DimStyle.Render("🔧"), toolType)
					}
				}

				if cmd.Flags().Changed("skip-health-check") == false {
					if envSkip := os.Getenv("KUBIYA_SKIP_HEALTH_CHECK"); envSkip != "" {
						skipHealthCheck = strings.ToLower(envSkip) == "true" || envSkip == "1"
						if skipHealthCheck {
							fmt.Printf("%s Health check disabled by KUBIYA_SKIP_HEALTH_CHECK\n",
								style.DimStyle.Render("🔧"))
						}
					}
				}

				// Build tool definition
				var toolDef map[string]interface{}

				// Load from URL if specified
				if toolURL != "" {
					fmt.Printf("%s Loading tool from URL: %s\n", style.InfoStyle.Render("🌐"), toolURL)

					// Fetch the tool definition from URL
					resp, err := http.Get(toolURL)
					if err != nil {
						return fmt.Errorf("failed to fetch tool from URL: %w", err)
					}
					defer resp.Body.Close()

					if resp.StatusCode != http.StatusOK {
						return fmt.Errorf("failed to fetch tool from URL: status %d", resp.StatusCode)
					}

					// Parse the response based on content type
					contentType := resp.Header.Get("Content-Type")
					data, err := io.ReadAll(resp.Body)
					if err != nil {
						return fmt.Errorf("failed to read tool definition: %w", err)
					}

					// Try to parse as JSON or YAML
					if strings.Contains(contentType, "json") || strings.HasSuffix(toolURL, ".json") {
						if err := json.Unmarshal(data, &toolDef); err != nil {
							return fmt.Errorf("failed to parse JSON tool definition: %w", err)
						}
					} else {
						// For YAML, we'd need to parse it - for now assume JSON
						if err := json.Unmarshal(data, &toolDef); err != nil {
							return fmt.Errorf("failed to parse tool definition: %w", err)
						}
					}

					// Extract tool name
					if name, ok := toolDef["name"].(string); ok && name != "" {
						toolName = name
					}
					fmt.Printf("%s Loaded tool: %s\n", style.SuccessStyle.Render("✓"), toolName)
				} else if sourceUUID != "" {
					// Load from source UUID
					if toolName == "" {
						return fmt.Errorf("tool name is required when using --source-uuid")
					}

					fmt.Printf("%s Loading tool '%s' from source: %s\n",
						style.InfoStyle.Render("📦"), toolName, sourceUUID)

					// Get source metadata
					source, err := client.GetSourceMetadata(ctx, sourceUUID)
					if err != nil {
						return fmt.Errorf("failed to get source metadata: %w", err)
					}

					// Find the tool
					var found bool
					for _, tool := range source.Tools {
						if tool.Name == toolName {
							// Convert tool to map
							toolJSON, err := json.Marshal(tool)
							if err != nil {
								return fmt.Errorf("failed to marshal tool: %w", err)
							}
							if err := json.Unmarshal(toolJSON, &toolDef); err != nil {
								return fmt.Errorf("failed to unmarshal tool: %w", err)
							}
							found = true
							break
						}
					}

					// Also check inline tools
					if !found {
						for _, tool := range source.InlineTools {
							if tool.Name == toolName {
								// Convert tool to map
								toolJSON, err := json.Marshal(tool)
								if err != nil {
									return fmt.Errorf("failed to marshal tool: %w", err)
								}
								if err := json.Unmarshal(toolJSON, &toolDef); err != nil {
									return fmt.Errorf("failed to unmarshal tool: %w", err)
								}
								found = true
								break
							}
						}
					}

					if !found {
						return fmt.Errorf("tool '%s' not found in source %s", toolName, sourceUUID)
					}

					fmt.Printf("%s Loaded tool: %s\n", style.SuccessStyle.Render("✓"), toolName)
				} else if jsonInput != "" {
					// Parse direct JSON input
					if err := json.Unmarshal([]byte(jsonInput), &toolDef); err != nil {
						return fmt.Errorf("failed to parse JSON input: %w", err)
					}
					// Extract tool name if not provided
					if toolName == "" {
						if name, ok := toolDef["name"].(string); ok {
							toolName = name
						}
					}
				} else if jsonFile != "" {
					// Read tool definition from file
					data, err := os.ReadFile(jsonFile)
					if err != nil {
						return fmt.Errorf("failed to read JSON file: %w", err)
					}
					if err := json.Unmarshal(data, &toolDef); err != nil {
						return fmt.Errorf("failed to parse JSON file: %w", err)
					}
					// Extract tool name if not provided
					if toolName == "" {
						if name, ok := toolDef["name"].(string); ok {
							toolName = name
						}
					}
				} else {
					// Build tool definition from flags
					if toolName == "" {
						return fmt.Errorf("tool name is required")
					}
					if content == "" {
						return fmt.Errorf("tool content is required")
					}

					toolDef = map[string]interface{}{
						"name":        toolName,
						"description": toolDesc,
						"content":     content,
					}

					if toolType != "" {
						toolDef["type"] = toolType
					} else {
						toolDef["type"] = "docker" // Default type
					}
					if image != "" {
						toolDef["image"] = image
					}
				}

				// Apply integration templates
				if len(integrations) > 0 {
					fmt.Printf("%s Applying integration templates...\n", style.InfoStyle.Render("🔌"))
					fs := afero.NewOsFs()
					allIntegrations, err := GetAllToolIntegrations(fs)
					if err != nil {
						return fmt.Errorf("failed to load integrations: %w", err)
					}

					for _, integrationName := range integrations {
						integration, ok := allIntegrations[integrationName]
						if !ok {
							return fmt.Errorf("integration '%s' not found", integrationName)
						}

						fmt.Printf("%s Applying %s integration (%s)\n",
							style.SuccessStyle.Render("✓"),
							style.HighlightStyle.Render(integrationName),
							integration.Description)

						if err := ApplyIntegrationToTool(toolDef, integration); err != nil {
							return fmt.Errorf("failed to apply integration '%s': %w", integrationName, err)
						}
					}
				}

				// Apply additional tool properties from flags
				if err := applyToolPropertiesFromFlags(toolDef, withFiles, withVolumes, withServices, envVars, args, iconURL); err != nil {
					return fmt.Errorf("failed to apply tool properties: %w", err)
				}

				// Default runner if not specified
				if runner == "" {
					// Check for default runner env var
					if defaultRunner := os.Getenv("KUBIYA_DEFAULT_RUNNER"); defaultRunner != "" {
						runner = defaultRunner
						fmt.Printf("%s Using default runner from KUBIYA_DEFAULT_RUNNER: %s\n",
							style.DimStyle.Render("🔧"), runner)
					} else {
						runner = "auto"
					}
				}

				// Handle "default" runner value
				if runner == "default" {
					defaultRunner := os.Getenv("KUBIYA_DEFAULT_RUNNER")
					if defaultRunner != "" {
						runner = defaultRunner
						fmt.Printf("%s Using default runner from KUBIYA_DEFAULT_RUNNER: %s\n",
							style.DimStyle.Render("🔧"), runner)
					} else {
						runner = "auto"
						fmt.Printf("%s No KUBIYA_DEFAULT_RUNNER set, using auto selection\n",
							style.DimStyle.Render("🔧"))
					}
				}

				// Handle auto runner selection
				selectedRunner := runner
				if runner == "auto" {
					fmt.Printf("%s Auto-selecting runner...\n", style.InfoStyle.Render("🔍"))

					// Try kubiya-hosted first
					runnerInfo, err := client.GetRunner(ctx, "kubiya-hosted")
					if err == nil && isRunnerHealthy(runnerInfo) {
						selectedRunner = "kubiya-hosted"
						fmt.Printf("%s Selected primary runner: %s (healthy)\n",
							style.SuccessStyle.Render("✓"), style.HighlightStyle.Render(selectedRunner))
					} else {
						// kubiya-hosted is not healthy, find an alternative
						if err != nil {
							fmt.Printf("%s Primary runner 'kubiya-hosted' not accessible: %v\n",
								style.WarningStyle.Render("⚠️"), err)
						} else {
							fmt.Printf("%s Primary runner 'kubiya-hosted' is not healthy (status: %s)\n",
								style.WarningStyle.Render("⚠️"), runnerInfo.RunnerHealth.Status)
						}

						// List all runners and find a healthy one
						fmt.Printf("%s Searching for alternative runners...\n", style.DimStyle.Render("›"))
						runners, err := client.ListRunners(ctx)
						if err != nil {
							return fmt.Errorf("failed to list runners: %w", err)
						}

						// Find the first healthy runner
						for _, r := range runners {
							if r.Name != "kubiya-hosted" && isRunnerHealthy(r) {
								selectedRunner = r.Name
								fmt.Printf("%s Selected fallback runner: %s (healthy, %s)\n",
									style.SuccessStyle.Render("✓"),
									style.HighlightStyle.Render(selectedRunner),
									r.RunnerType)
								break
							}
						}

						// If no healthy runner found
						if selectedRunner == "auto" {
							return fmt.Errorf("no healthy runners available")
						}
					}
				} else if !skipHealthCheck {
					// For explicitly specified runners, check health
					runnerInfo, err := client.GetRunner(ctx, selectedRunner)
					if err != nil {
						fmt.Printf("%s Warning: Could not check runner health: %v\n",
							style.WarningStyle.Render("⚠️"), err)
					} else {
						// Check runner health status
						if !isRunnerHealthy(runnerInfo) {
							if runnerInfo.RunnerHealth.Error != "" {
								return fmt.Errorf("runner '%s' is not healthy: %s (error: %s)",
									selectedRunner, runnerInfo.RunnerHealth.Status, runnerInfo.RunnerHealth.Error)
							}
							return fmt.Errorf("runner '%s' is not healthy: %s",
								selectedRunner, runnerInfo.RunnerHealth.Status)
						}
						fmt.Printf("%s Runner '%s' is healthy (v%s)\n",
							style.SuccessStyle.Render("✓"), selectedRunner, runnerInfo.Version)
					}
				}

				// Policy validation (if enabled)
				if !skipPolicyCheck {
					// Check if OPA enforcement is enabled
					opaEnforce := os.Getenv("KUBIYA_OPA_ENFORCE")
					if opaEnforce == "true" || opaEnforce == "1" {
						fmt.Printf("%s Validating tool execution permissions...\n", style.InfoStyle.Render("🛡️"))

						// Extract args from toolDef for validation
						toolArgs := make(map[string]interface{})
						if args, ok := toolDef["args"].(map[string]interface{}); ok {
							toolArgs = args
						}

						allowed, message, err := client.ValidateToolExecution(ctx, toolName, toolArgs, selectedRunner)
						if err != nil {
							fmt.Printf("%s Policy validation failed: %v\n", style.WarningStyle.Render("⚠️"), err)
							fmt.Printf("%s Proceeding without policy validation...\n", style.DimStyle.Render("›"))
						} else if !allowed {
							fmt.Printf("%s Tool execution denied by policy\n", style.ErrorStyle.Render("❌"))
							if message != "" {
								fmt.Printf("%s Reason: %s\n", style.DimStyle.Render("›"), message)
							}
							return fmt.Errorf("tool execution denied by policy")
						} else {
							fmt.Printf("%s Tool execution authorized\n", style.SuccessStyle.Render("✅"))
							if message != "" {
								fmt.Printf("%s %s\n", style.DimStyle.Render("›"), message)
							}
						}
					} else {
						fmt.Printf("%s Policy enforcement disabled (KUBIYA_OPA_ENFORCE not set)\n", style.DimStyle.Render("🔧"))
					}
				}

				// Show execution info
				fmt.Printf("\n%s Executing tool: %s\n", style.StatusStyle.Render("🚀"), style.HighlightStyle.Render(toolName))
				fmt.Printf("%s Runner: %s", style.DimStyle.Render("📍"), style.HighlightStyle.Render(selectedRunner))
				if runner == "auto" && selectedRunner != "kubiya-hosted" {
					fmt.Printf(" %s", style.DimStyle.Render("(auto-selected fallback)"))
				}
				fmt.Printf("\n%s Timeout: ", style.DimStyle.Render("⏱️"))
				if timeout == 0 {
					fmt.Printf("%s\n", style.DimStyle.Render("none"))
				} else {
					fmt.Printf("%s\n", style.DimStyle.Render(fmt.Sprintf("%d seconds", timeout)))
				}
				fmt.Println()

				argVals := make(map[string]any)

				// Parse arguments from --args JSON or --arg flags
				if err := parseToolArguments(toolDef, args, argsJSON, argVals); err != nil {
					return fmt.Errorf("failed to parse tool arguments: %w", err)
				}
				// Execute tool with streaming
				events, err := client.ExecuteToolWithTimeout(ctx, toolName, toolDef, selectedRunner, time.Duration(timeout)*time.Second, argVals)
				if err != nil {
					return fmt.Errorf("failed to execute tool: %w", err)
				}

				// Process streaming events based on output format
				if outputFormat == "stream-json" {
					// Raw JSON stream output
					for event := range events {
						if event.Type == "data" {
							fmt.Println(event.Data)
						} else if event.Type == "error" {
							fmt.Fprintf(os.Stderr, `{"type":"error","message":"%s"}`+"\n", event.Data)
							return fmt.Errorf("tool execution failed")
						}
					}
					return nil
				}

				// Text output format (default)
				var hasError bool
				startTime := time.Now()
				var outputLines []string

				for event := range events {
					switch event.Type {
					case "data":
						if watch {
							// Try to parse as JSON for structured output
							var jsonData map[string]interface{}
							if err := json.Unmarshal([]byte(event.Data), &jsonData); err == nil {
								// Handle different event types from the API
								if eventType, ok := jsonData["type"].(string); ok {
									switch eventType {
									case "log":
										// Display log messages
										if content, ok := jsonData["content"].(string); ok {
											// Skip connection logs for cleaner output
											if strings.Contains(content, "Connected to cloud") ||
												strings.Contains(content, "starting session") ||
												strings.Contains(content, "connect") {
												fmt.Printf("%s %s\n", style.DimStyle.Render("›"), style.DimStyle.Render(content))
											} else if strings.Contains(content, "Creating new Engine session") {
												fmt.Printf("%s %s\n", style.InfoStyle.Render("🔧"), content)
											} else {
												fmt.Printf("%s %s\n", style.DimStyle.Render("📝"), content)
											}
										}
									case "tool-output":
										// Display tool output
										if content, ok := jsonData["content"].(string); ok {
											// Remove trailing newline for cleaner output
											content = strings.TrimRight(content, "\n")
											lines := strings.Split(content, "\n")
											for _, line := range lines {
												if line != "" {
													fmt.Printf("%s %s\n", style.OutputStyle.Render("│"), line)
													outputLines = append(outputLines, line)
												}
											}
										}
									case "status":
										// Handle completion status
										if status, ok := jsonData["status"].(string); ok {
											if endVal, ok := jsonData["end"].(bool); ok && endVal {
												duration := time.Since(startTime)
												fmt.Println() // Add spacing
												if status == "success" {
													fmt.Printf("%s Tool executed successfully in %0.2fs\n",
														style.SuccessStyle.Render("✅"), duration.Seconds())
													if len(outputLines) > 0 {
														fmt.Printf("%s %d lines of output generated\n",
															style.DimStyle.Render("📊"), len(outputLines))
													}
												} else {
													hasError = true
													fmt.Printf("%s Tool execution failed after %0.2fs\n",
														style.ErrorStyle.Render("❌"), duration.Seconds())
												}
												return nil
											}
										}
									default:
										// For other event types, show content if available
										if content, ok := jsonData["content"].(string); ok && content != "" {
											fmt.Printf("%s [%s] %s\n", style.DimStyle.Render("›"), eventType, content)
										}
									}
								}
							} else {
								// If not JSON, display as plain text
								fmt.Printf("%s %s\n", style.OutputStyle.Render("│"), event.Data)
							}
						}
					case "error":
						hasError = true
						fmt.Printf("\n%s Error: %s\n", style.ErrorStyle.Render("✗"), event.Data)
					case "complete", "done":
						duration := time.Since(startTime)
						fmt.Println() // Add spacing
						if !hasError {
							fmt.Printf("%s Tool executed successfully in %0.2fs\n",
								style.SuccessStyle.Render("✅"), duration.Seconds())
						} else {
							fmt.Printf("%s Tool execution failed after %0.2fs\n",
								style.ErrorStyle.Render("❌"), duration.Seconds())
						}
						return nil
					default:
						// Handle other event types if needed
						if watch && event.Data != "" {
							fmt.Printf("%s [%s] %s\n", style.DimStyle.Render("›"), event.Type, event.Data)
						}
					}
				}

				if hasError {
					return fmt.Errorf("tool execution failed")
				}

				return nil
			})
		},
	}

	cmd.Flags().StringVar(&toolName, "name", "", "Tool name")
	cmd.Flags().StringVar(&toolDesc, "description", "CLI executed tool", "Tool description")
	cmd.Flags().StringVar(&toolType, "type", "", "Tool type (docker, python, bash, etc.) - defaults to docker")
	cmd.Flags().StringVar(&image, "image", "", "Docker image for the tool")
	cmd.Flags().StringVar(&content, "content", "", "Tool content/script to execute")
	cmd.Flags().StringVar(&runner, "runner", "", "Runner to use for execution (default: auto)")
	cmd.Flags().BoolVar(&watch, "watch", true, "Watch and stream output in real-time")
	cmd.Flags().StringVar(&jsonFile, "json-file", "", "Path to JSON file with tool definition")
	cmd.Flags().StringVar(&jsonInput, "json", "", "Tool definition as JSON string")
	cmd.Flags().StringVar(&outputFormat, "output", "text", "Output format: text (default) or stream-json")
	cmd.Flags().BoolVar(&skipHealthCheck, "skip-health-check", false, "Skip runner health check")
	cmd.Flags().BoolVar(&skipPolicyCheck, "skip-policy-check", false, "Skip policy validation check")
	cmd.Flags().IntVar(&timeout, "timeout", 300, "Timeout in seconds for tool execution (0 for no timeout)")
	cmd.Flags().StringSliceVar(&integrations, "integration", []string{}, "Integration templates to apply (can be specified multiple times)")
	cmd.Flags().StringSliceVar(&withFiles, "with-file", []string{}, "File mappings in format 'source:destination' (can be specified multiple times)")
	cmd.Flags().StringSliceVar(&withVolumes, "with-volume", []string{}, "Volume mappings in format 'source:destination[:ro]' (can be specified multiple times)")
	cmd.Flags().StringSliceVar(&withServices, "with-service", []string{}, "Service dependencies (can be specified multiple times)")
	cmd.Flags().StringSliceVar(&envVars, "env", []string{}, "Environment variables in format 'KEY=VALUE' (can be specified multiple times)")
	cmd.Flags().StringSliceVar(&args, "arg", []string{}, "Tool arguments in format 'name:type:description:required' (can be specified multiple times)")
	cmd.Flags().StringVar(&argsJSON, "args", "", "Tool argument values as JSON object (e.g., '{\"param1\":\"value1\",\"param2\":\"value2\"}')")
	cmd.Flags().StringVar(&iconURL, "icon-url", "", "Icon URL for the tool")
	cmd.Flags().StringVar(&toolURL, "tool-url", "", "URL to load tool definition from")
	cmd.Flags().StringVar(&sourceUUID, "source-uuid", "", "Source UUID to load tool from")

	return cmd
}

func isRunnerHealthy(runner kubiya.Runner) bool {
	// Check various status fields that indicate health
	status := strings.ToLower(runner.RunnerHealth.Status)
	health := strings.ToLower(runner.RunnerHealth.Health)

	// Accept various indicators of health
	return status == "healthy" || status == "ok" || status == "ready" ||
		health == "healthy" || health == "true" || health == "ok" ||
		(status == "" && health == "") // Sometimes no status means it's running fine
}

// applyToolPropertiesFromFlags applies tool properties from command-line flags
func applyToolPropertiesFromFlags(toolDef map[string]interface{}, withFiles, withVolumes, withServices, envVars, args []string, iconURL string) error {
	// Apply file mappings
	if len(withFiles) > 0 {
		fileMappings := []interface{}{}
		for _, fileMapping := range withFiles {
			parts := strings.SplitN(fileMapping, ":", 2)
			if len(parts) != 2 {
				return fmt.Errorf("invalid file mapping format: %s (expected source:destination)", fileMapping)
			}

			source := expandPath(parts[0])
			dest := parts[1]

			fileMap := map[string]interface{}{
				"source":      source,
				"destination": dest,
			}

			// Check if source is a local file that should be read
			if IsLocalPath(source) {
				content, err := os.ReadFile(source)
				if err != nil {
					fmt.Printf("%s Warning: Could not read local file %s: %v\n", style.WarningStyle.Render("⚠"), source, err)
				} else {
					fileMap["content"] = string(content)
					fmt.Printf("%s Loaded content from local file: %s\n", style.SuccessStyle.Render("✓"), source)
				}
			}

			fileMappings = append(fileMappings, fileMap)
		}

		// Merge with existing file mappings
		if existing, ok := toolDef["with_files"].([]interface{}); ok {
			fileMappings = append(existing, fileMappings...)
		}
		toolDef["with_files"] = fileMappings
	}

	// Apply volume mappings
	if len(withVolumes) > 0 {
		volumeMappings := []interface{}{}
		for _, volumeMapping := range withVolumes {
			parts := strings.Split(volumeMapping, ":")
			if len(parts) < 2 || len(parts) > 3 {
				return fmt.Errorf("invalid volume mapping format: %s (expected source:destination[:ro])", volumeMapping)
			}

			volMap := map[string]interface{}{
				"source":      expandPath(parts[0]),
				"destination": parts[1],
			}

			if len(parts) == 3 && parts[2] == "ro" {
				volMap["read_only"] = true
			}

			volumeMappings = append(volumeMappings, volMap)
		}

		// Merge with existing volume mappings
		if existing, ok := toolDef["with_volumes"].([]interface{}); ok {
			volumeMappings = append(existing, volumeMappings...)
		}
		toolDef["with_volumes"] = volumeMappings
	}

	// Apply service dependencies
	if len(withServices) > 0 {
		services := []interface{}{}
		for _, service := range withServices {
			services = append(services, service)
		}

		// Merge with existing services
		if existing, ok := toolDef["with_services"].([]interface{}); ok {
			services = append(existing, services...)
		}
		toolDef["with_services"] = services
	}

	// Apply environment variables
	if len(envVars) > 0 {
		env := []interface{}{}
		for _, envVar := range envVars {
			env = append(env, envVar)
		}

		// Merge with existing env vars
		if existing, ok := toolDef["env"].([]interface{}); ok {
			env = append(existing, env...)
		}
		toolDef["env"] = env
	}

	// Apply tool arguments
	if len(args) > 0 {
		toolArgs := []interface{}{}
		for _, arg := range args {
			parts := strings.Split(arg, ":")
			if len(parts) < 3 {
				return fmt.Errorf("invalid argument format: %s (expected name:type:description[:required])", arg)
			}

			argDef := map[string]interface{}{
				"name":        parts[0],
				"type":        parts[1],
				"description": parts[2],
			}

			if len(parts) > 3 && parts[3] == "true" {
				argDef["required"] = true
			}

			toolArgs = append(toolArgs, argDef)
		}

		// Merge with existing args
		if existing, ok := toolDef["args"].([]interface{}); ok {
			toolArgs = append(existing, toolArgs...)
		}
		toolDef["args"] = toolArgs
	}

	// Apply icon URL
	if iconURL != "" {
		toolDef["icon_url"] = iconURL
	}

	return nil
}

// IsLocalPath checks if a path refers to a local file
func IsLocalPath(path string) bool {
	// Paths that start with these are typically remote/container paths
	remotePathPrefixes := []string{
		"/var/run/",
		"/tmp/kubernetes",
		"/etc/",
		"/opt/",
		"/usr/",
		"/sys/",
		"/proc/",
	}

	for _, prefix := range remotePathPrefixes {
		if strings.HasPrefix(path, prefix) {
			return false
		}
	}

	// Check if path starts with $HOME or ~ (local user paths)
	if strings.HasPrefix(path, "$HOME") || strings.HasPrefix(path, "~") {
		return true
	}

	// Check if file exists locally
	if _, err := os.Stat(path); err == nil {
		return true
	}

	return false
}

// parseToolArguments parses command line tool arguments and populates argVals
// Supports both --args JSON format and --arg flags format
// Arguments from --arg flags are in format "name:type:description[:required]"
// but the argVals should contain actual values for these arguments
func parseToolArguments(toolDef map[string]interface{}, argFlags []string, argsJSON string, argVals map[string]any) error {
	// If argsJSON is provided, parse it directly
	if argsJSON != "" {
		var jsonArgs map[string]interface{}
		if err := json.Unmarshal([]byte(argsJSON), &jsonArgs); err != nil {
			return fmt.Errorf("failed to parse --args JSON: %w", err)
		}

		// Copy parsed JSON args to argVals
		for key, value := range jsonArgs {
			argVals[key] = value
		}

		fmt.Printf("%s Parsed %d arguments from JSON\n",
			style.InfoStyle.Render("ℹ"), len(jsonArgs))
		return nil
	}
	// Get tool args definition from toolDef
	toolArgsInterface, exists := toolDef["args"]
	if !exists || toolArgsInterface == nil {
		// No args defined in tool, but user provided --arg flags
		if len(argFlags) > 0 {
			fmt.Printf("%s Tool has no argument definitions, but --arg flags provided. These will be ignored.\n",
				style.WarningStyle.Render("⚠"))
		}
		return nil
	}

	// Convert to proper format
	var toolArgs []interface{}
	switch v := toolArgsInterface.(type) {
	case []interface{}:
		toolArgs = v
	case []map[string]interface{}:
		for _, arg := range v {
			toolArgs = append(toolArgs, arg)
		}
	default:
		// If args is not in expected format, skip parsing
		return nil
	}

	// Create a map of argument definitions by name for quick lookup
	argDefs := make(map[string]map[string]interface{})
	for _, argInterface := range toolArgs {
		if argMap, ok := argInterface.(map[string]interface{}); ok {
			if name, ok := argMap["name"].(string); ok {
				argDefs[name] = argMap
			}
		}
	}

	// For CLI tool exec, we need to prompt for required arguments or set defaults
	// Since we don't have a way to pass actual values via flags, we'll use defaults
	// or prompt the user if arguments are required

	for name, argDef := range argDefs {
		// Check if required
		required := false
		if req, ok := argDef["required"].(bool); ok {
			required = req
		}

		// Get default value if available
		var defaultValue interface{}
		if def, ok := argDef["default"]; ok {
			defaultValue = def
		}

		// Get argument type
		argType := "string"
		if typ, ok := argDef["type"].(string); ok {
			argType = typ
		}

		// For now, use default values or empty values for required args
		// In a full implementation, we'd prompt the user or accept values via flags
		if defaultValue != nil {
			argVals[name] = defaultValue
		} else if required {
			// Set empty value based on type for required args
			switch argType {
			case "string":
				argVals[name] = ""
			case "number", "integer":
				argVals[name] = 0
			case "boolean":
				argVals[name] = false
			case "array":
				argVals[name] = []interface{}{}
			case "object":
				argVals[name] = map[string]interface{}{}
			default:
				argVals[name] = ""
			}

			fmt.Printf("%s Required argument '%s' set to default value for type '%s'\n",
				style.InfoStyle.Render("ℹ"), name, argType)
		}
	}

	return nil
}
