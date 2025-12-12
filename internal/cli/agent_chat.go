package cli

import (
	"bufio"
	"context"
	"fmt"
	"os"
	"strings"

	"github.com/kubiyabot/cli/internal/config"
	"github.com/kubiyabot/cli/internal/controlplane"
	"github.com/kubiyabot/cli/internal/controlplane/entities"
	"github.com/kubiyabot/cli/internal/style"
	"github.com/spf13/cobra"
)

func newAgentChatCommand(cfg *config.Config) *cobra.Command {
	cmd := &cobra.Command{
		Use:     "chat <agent-id|team-id>",
		Aliases: []string{"c", "talk", "converse"},
		Short:   "💬 Start interactive chat with an agent or team",
		Long: `Start an interactive chat session with an agent or team.

The chat command provides a conversational interface to interact with your AI agents.
Each message you send creates a new execution, and you can continue the conversation
with follow-up questions and commands.`,
		Example: `  # Start chat with an agent
  kubiya agent chat 8064f4c8-fb5c-4a52-99f8-9075521500a3

  # Start chat with a team
  kubiya team chat 4274b510-ed05-42cb-9237-aa05e7319f8b

  # Execute a single task without interactive mode
  kubiya agent exec 8064f4c8 "Deploy to production"`,
	}

	cmd.AddCommand(
		newAgentInteractiveChatCommand(cfg),
		newTeamInteractiveChatCommand(cfg),
		newAgentExecCommand(cfg),
		newTeamExecCommand(cfg),
	)

	return cmd
}

func newAgentInteractiveChatCommand(cfg *config.Config) *cobra.Command {
	var workerQueue string
	var systemPrompt string

	cmd := &cobra.Command{
		Use:     "agent <agent-id>",
		Aliases: []string{"a"},
		Short:   "Chat with an agent",
		Args:    cobra.ExactArgs(1),
		Example: `  # Start interactive chat
  kubiya chat agent 8064f4c8-fb5c-4a52-99f8-9075521500a3

  # With custom worker queue
  kubiya chat agent 8064f4c8 --queue my-queue-id`,
		RunE: func(cmd *cobra.Command, args []string) error {
			agentID := args[0]

			client, err := controlplane.New(cfg.APIKey, cfg.Debug)
			if err != nil {
				return fmt.Errorf("failed to create client: %w", err)
			}

			// Get agent details
			agent, err := client.GetAgent(agentID)
			if err != nil {
				return fmt.Errorf("failed to get agent: %w", err)
			}

			// Beautiful banner
			fmt.Println()
			fmt.Println(style.CreateBanner(fmt.Sprintf("Chat with %s", agent.Name), "🤖"))
			fmt.Println()

			// Beautiful agent info box
			fmt.Println(style.CreateAgentInfoBox(agent.Name, agent.ID, string(agent.Runtime), string(agent.Status)))
			fmt.Println()

			// Get default worker queue if not specified
			if workerQueue == "" {
				queues, err := client.ListWorkerQueues()
				if err != nil || len(queues) == 0 {
					return fmt.Errorf("no worker queues available, please create one first")
				}
				workerQueue = queues[0].ID
				fmt.Println(style.CreateHelpBox(fmt.Sprintf("🎯 Using worker queue: %s", queues[0].Name)))
			}

			return startInteractiveChatSession(cmd.Context(), client, "agent", agentID, agent.Name, workerQueue, systemPrompt)
		},
	}

	cmd.Flags().StringVarP(&workerQueue, "queue", "q", "", "Worker queue ID to use for execution")
	cmd.Flags().StringVar(&systemPrompt, "system-prompt", "", "Custom system prompt")

	return cmd
}

func newTeamInteractiveChatCommand(cfg *config.Config) *cobra.Command {
	var workerQueue string
	var systemPrompt string

	cmd := &cobra.Command{
		Use:     "team <team-id>",
		Aliases: []string{"t"},
		Short:   "Chat with a team",
		Args:    cobra.ExactArgs(1),
		Example: `  # Start interactive chat with a team
  kubiya chat team 4274b510-ed05-42cb-9237-aa05e7319f8b`,
		RunE: func(cmd *cobra.Command, args []string) error {
			teamID := args[0]

			client, err := controlplane.New(cfg.APIKey, cfg.Debug)
			if err != nil {
				return fmt.Errorf("failed to create client: %w", err)
			}

			// Get team details
			team, err := client.GetTeam(teamID)
			if err != nil {
				return fmt.Errorf("failed to get team: %w", err)
			}

			// Beautiful banner for team
			fmt.Println()
			fmt.Println(style.CreateBanner(fmt.Sprintf("Chat with Team: %s", team.Name), "👥"))
			fmt.Println()

			// Team info box
			teamInfo := map[string]string{
				"Team ID": team.ID,
			}
			if team.Description != nil {
				teamInfo["Description"] = *team.Description
			}
			fmt.Println(style.CreateMetadataBox(teamInfo))
			fmt.Println()

			// Get default worker queue if not specified
			if workerQueue == "" {
				queues, err := client.ListWorkerQueues()
				if err != nil || len(queues) == 0 {
					return fmt.Errorf("no worker queues available, please create one first")
				}
				workerQueue = queues[0].ID
				fmt.Println(style.CreateHelpBox(fmt.Sprintf("🎯 Using worker queue: %s", queues[0].Name)))
			}

			return startInteractiveChatSession(cmd.Context(), client, "team", teamID, team.Name, workerQueue, systemPrompt)
		},
	}

	cmd.Flags().StringVarP(&workerQueue, "queue", "q", "", "Worker queue ID to use for execution")
	cmd.Flags().StringVar(&systemPrompt, "system-prompt", "", "Custom system prompt")

	return cmd
}

func newAgentExecCommand(cfg *config.Config) *cobra.Command {
	var workerQueue string
	var systemPrompt string
	var onDemand bool

	// Environment configuration flags
	var workingDir string
	var envVars []string
	var envFile string
	var secrets []string
	var skillDirs []string
	var timeout int

	cmd := &cobra.Command{
		Use:     "exec <agent-id> <prompt>",
		Aliases: []string{"execute", "run", "task"},
		Short:   "Execute a single task with an agent",
		Args:    cobra.MinimumNArgs(2),
		Example: `  # Execute a task with an agent
  kubiya agent exec 8064f4c8 "Deploy to production"

  # With on-demand worker (auto-provisioned, ephemeral)
  kubiya agent exec 8064f4c8 "Deploy to production" --on-demand

  # With environment variables and working directory
  kubiya agent exec 8064f4c8 "Build project" --on-demand \
    --workdir /path/to/project \
    --env BUILD_ENV=production \
    --env DEBUG=false

  # With .env file and secrets
  kubiya agent exec 8064f4c8 "Deploy app" --on-demand \
    --env-file .env.production \
    --secret DATABASE_PASSWORD \
    --secret API_KEY

  # With custom timeout and skill directories
  kubiya agent exec 8064f4c8 "Run tests" --on-demand \
    --timeout 600 \
    --skill-dir ./custom-skills

  # With custom worker queue
  kubiya agent exec 8064f4c8 "Analyze logs" --queue my-queue`,
		RunE: func(cmd *cobra.Command, args []string) error {
			agentID := args[0]
			prompt := strings.Join(args[1:], " ")

			client, err := controlplane.New(cfg.APIKey, cfg.Debug)
			if err != nil {
				return fmt.Errorf("failed to create client: %w", err)
			}

			// Handle --on-demand flag
			if onDemand {
				// Get agent to determine environment
				agent, err := client.GetAgent(agentID)
				if err != nil {
					return fmt.Errorf("failed to get agent: %w", err)
				}

				// Use agent's environment or default
				var envID string
				if len(agent.EnvironmentIDs) > 0 {
					envID = agent.EnvironmentIDs[0]
				} else {
					envs, err := client.ListEnvironments()
					if err != nil || len(envs) == 0 {
						return fmt.Errorf("no environments available")
					}
					envID = envs[0].ID
				}

				// Execute with on-demand worker
				executor := &OnDemandExecutor{
					client:        client,
					cfg:           cfg,
					agentID:       agentID,
					prompt:        prompt,
					systemPrompt:  systemPrompt,
					environmentID: envID,
					// Environment configuration
					workingDir: workingDir,
					envVars:    envVars,
					envFile:    envFile,
					secrets:    secrets,
					skillDirs:  skillDirs,
					timeout:    timeout,
				}

				fmt.Println()
				fmt.Println(style.CreateBanner("On-Demand Task Execution", "⚡"))
				fmt.Println()

				return executor.Execute(cmd.Context())
			}

			// Regular execution with pre-existing queue
			if workerQueue == "" {
				queues, err := client.ListWorkerQueues()
				if err != nil || len(queues) == 0 {
					return fmt.Errorf("no worker queues available, please create one first or use --on-demand")
				}
				workerQueue = queues[0].ID
			}

			// Beautiful banner for single execution
			fmt.Println()
			fmt.Println(style.CreateBanner("Task Execution", "🚀"))
			fmt.Println()

			// Show task details
			taskInfo := map[string]string{
				"Task": prompt,
			}
			fmt.Println(style.CreateMetadataBox(taskInfo))
			fmt.Println()

			return executeSingleTask(cmd.Context(), client, "agent", agentID, prompt, workerQueue, systemPrompt)
		},
	}

	cmd.Flags().StringVarP(&workerQueue, "queue", "q", "", "Worker queue ID to use for execution")
	cmd.Flags().StringVar(&systemPrompt, "system-prompt", "", "Custom system prompt")
	cmd.Flags().BoolVar(&onDemand, "on-demand", false, "Provision ephemeral worker for this execution")

	// Environment configuration flags (only work with --on-demand)
	cmd.Flags().StringVar(&workingDir, "workdir", "", "Working directory for execution (on-demand only)")
	cmd.Flags().StringArrayVar(&envVars, "env", nil, "Environment variables in KEY=VALUE format (on-demand only, repeatable)")
	cmd.Flags().StringVar(&envFile, "env-file", "", "Load environment variables from .env file (on-demand only)")
	cmd.Flags().StringArrayVar(&secrets, "secret", nil, "Secret names to fetch from Kubiya API and inject as env vars (on-demand only, repeatable)")
	cmd.Flags().StringArrayVar(&skillDirs, "skill-dir", nil, "Additional skill directories to load (on-demand only, repeatable)")
	cmd.Flags().IntVar(&timeout, "timeout", 300, "Execution timeout in seconds (on-demand only, default: 300)")

	return cmd
}

func newTeamExecCommand(cfg *config.Config) *cobra.Command {
	var workerQueue string
	var systemPrompt string

	cmd := &cobra.Command{
		Use:     "exec <team-id> <prompt>",
		Aliases: []string{"execute", "run", "task"},
		Short:   "Execute a single task with a team",
		Args:    cobra.MinimumNArgs(2),
		Example: `  # Execute a task with a team
  kubiya team exec 4274b510 "Analyze security logs"`,
		RunE: func(cmd *cobra.Command, args []string) error {
			teamID := args[0]
			prompt := strings.Join(args[1:], " ")

			client, err := controlplane.New(cfg.APIKey, cfg.Debug)
			if err != nil {
				return fmt.Errorf("failed to create client: %w", err)
			}

			// Get default worker queue if not specified
			if workerQueue == "" {
				queues, err := client.ListWorkerQueues()
				if err != nil || len(queues) == 0 {
					return fmt.Errorf("no worker queues available, please create one first")
				}
				workerQueue = queues[0].ID
			}

			// Beautiful banner for single execution
			fmt.Println()
			fmt.Println(style.CreateBanner("Task Execution", "🚀"))
			fmt.Println()

			// Show task details
			taskInfo := map[string]string{
				"Task": prompt,
			}
			fmt.Println(style.CreateMetadataBox(taskInfo))
			fmt.Println()

			return executeSingleTask(cmd.Context(), client, "team", teamID, prompt, workerQueue, systemPrompt)
		},
	}

	cmd.Flags().StringVarP(&workerQueue, "queue", "q", "", "Worker queue ID to use for execution")
	cmd.Flags().StringVar(&systemPrompt, "system-prompt", "", "Custom system prompt")

	return cmd
}

func startInteractiveChatSession(ctx context.Context, client *controlplane.Client, entityType, entityID, entityName, workerQueue, systemPrompt string) error {
	scanner := bufio.NewScanner(os.Stdin)

	// Beautiful instructions
	instructions := fmt.Sprintf("💬 Type your message and press %s\n"+
		"   Type %s or %s to end the session\n"+
		"   Press %s to abort an ongoing execution",
		style.KeyboardShortcutStyle.Render(" Enter "),
		style.KeyboardShortcutStyle.Render(" exit "),
		style.KeyboardShortcutStyle.Render(" quit "),
		style.KeyboardShortcutStyle.Render(" Ctrl+C "))
	fmt.Println(style.CreateHelpBox(instructions))
	fmt.Println()
	fmt.Println(style.CreateDivider(80))
	fmt.Println()

	for {
		// Show beautiful user prompt
		fmt.Print(style.UserPromptStyle.Render(" You "))
		fmt.Print(" ")

		// Read user input
		if !scanner.Scan() {
			break
		}

		input := strings.TrimSpace(scanner.Text())
		if input == "" {
			continue
		}

		// Check for exit commands
		if input == "exit" || input == "quit" {
			fmt.Println()
			goodbye := "Thank you for chatting! Have a great day! 👋"
			fmt.Println(style.GoodbyeBoxStyle.Render(goodbye))
			fmt.Println()
			break
		}

		// Execute the task
		fmt.Println()
		if err := executeSingleTask(ctx, client, entityType, entityID, input, workerQueue, systemPrompt); err != nil {
			fmt.Println()
			fmt.Println(style.CreateErrorBox(fmt.Sprintf("Execution failed: %v", err)))
			fmt.Println()
			continue
		}
		fmt.Println()
		fmt.Println(style.CreateDivider(80))
		fmt.Println()
	}

	if err := scanner.Err(); err != nil {
		return fmt.Errorf("error reading input: %w", err)
	}

	return nil
}

func executeSingleTask(ctx context.Context, client *controlplane.Client, entityType, entityID, prompt, workerQueue, systemPrompt string) error {
	// Create execution request
	var execution *entities.AgentExecution
	var err error

	streamFlag := true

	// Convert workerQueue string to pointer (nil if empty, backend will auto-select)
	var queuePtr *string
	if workerQueue != "" {
		queuePtr = &workerQueue
	}

	if entityType == "agent" {
		req := &entities.ExecuteAgentRequest{
			Prompt:        prompt,
			WorkerQueueID: queuePtr,
			Stream:        &streamFlag,
		}
		if systemPrompt != "" {
			req.SystemPrompt = &systemPrompt
		}
		execution, err = client.ExecuteAgentV2(entityID, req)
	} else {
		req := &entities.ExecuteTeamRequest{
			Prompt:        prompt,
			WorkerQueueID: queuePtr,
			Stream:        &streamFlag,
		}
		if systemPrompt != "" {
			req.SystemPrompt = &systemPrompt
		}
		execution, err = client.ExecuteTeamV2(entityID, req)
	}

	if err != nil {
		return fmt.Errorf("failed to start execution: %w", err)
	}

	executionID := execution.GetID()

	// Show execution metadata in a nice box
	metadata := map[string]string{
		"Execution ID": executionID,
		"Status":       style.CreateStatusBadge(string(execution.Status)),
	}
	fmt.Println(style.CreateMetadataBox(metadata))
	fmt.Println()

	// Show beautiful assistant prompt
	fmt.Print(style.AssistantPromptStyle.Render(" Assistant "))
	fmt.Print(" ")

	eventChan, errChan := client.StreamExecutionOutput(ctx, executionID)

	var fullResponse strings.Builder
	streamStarted := false

	for {
		select {
		case event, ok := <-eventChan:
			if !ok {
				// Channel closed
				if !streamStarted {
					// No streaming data received, fetch final result
					finalExecution, err := client.GetExecution(executionID)
					if err == nil && finalExecution.Response != nil {
						fmt.Println(*finalExecution.Response)
					}
				}
				return nil
			}

			streamStarted = true

			switch event.Type {
			case "chunk":
				// Stream content in real-time
				fmt.Print(style.OutputStyle.Render(event.Content))
				fullResponse.WriteString(event.Content)
			case "error":
				// Show error in a beautiful box
				fmt.Println()
				fmt.Println()
				fmt.Println(style.CreateErrorBox(event.Content))
				return nil
			case "complete":
				// Completion
				fmt.Println()
				fmt.Println()
				fmt.Println(style.CreateSuccessBox("Execution completed successfully"))
				return nil
			case "status":
				// Status update - shown in debug mode only
				if event.Status != nil {
					// Optionally show status updates
					fmt.Printf(" %s ", style.CreateStatusBadge(string(*event.Status)))
				}
			}

		case err := <-errChan:
			if err != nil {
				fmt.Println()
				fmt.Println(style.CreateErrorBox(fmt.Sprintf("Streaming error: %v", err)))
				return fmt.Errorf("streaming error: %w", err)
			}

		case <-ctx.Done():
			fmt.Println()
			fmt.Println(style.CreateWarningBox("Execution interrupted by user"))
			return ctx.Err()
		}
	}
}
