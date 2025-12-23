package cli

import (
	"context"
	"fmt"
	"os"
	"strings"
	"time"

	"github.com/google/uuid"
	"github.com/kubiyabot/cli/internal/config"
	"github.com/kubiyabot/cli/internal/controlplane"
	"github.com/kubiyabot/cli/internal/controlplane/entities"
	"github.com/kubiyabot/cli/internal/style"
)

// OnDemandExecutor orchestrates the full lifecycle of an on-demand execution:
// 1. Creates ephemeral worker queue
// 2. Starts worker in foreground mode
// 3. Waits for worker to be ready
// 4. Submits execution
// 5. Streams output in real-time
// 6. Cleans up all resources (queue + worker)
type OnDemandExecutor struct {
	client        *controlplane.Client
	cfg           *config.Config
	agentID       string
	prompt        string
	systemPrompt  string
	environmentID string

	// Environment configuration
	workingDir string
	envVars    []string // Format: KEY=VALUE
	envFile    string   // Path to .env file
	secrets    []string // Secret names to fetch from Kubiya API
	skillDirs  []string // Additional skill directories
	timeout    int      // Execution timeout in seconds
}

// Execute runs the full on-demand execution flow
func (e *OnDemandExecutor) Execute(ctx context.Context) error {
	// 1. Create ephemeral queue
	fmt.Println(style.CreateHelpBox("Creating ephemeral worker queue..."))
	queue, err := e.createEphemeralQueue(ctx)
	if err != nil {
		return fmt.Errorf("failed to create ephemeral queue: %w", err)
	}
	fmt.Printf("✓ Created ephemeral queue: %s\n", queue.ID)
	fmt.Println()

	// 2. Setup cleanup (ALWAYS runs via defer, even on errors/Ctrl+C)
	defer func() {
		fmt.Println()
		fmt.Println(style.CreateHelpBox("Cleaning up resources..."))
		e.cleanup(queue.ID)
		fmt.Println("✓ Cleanup complete")
	}()

	// 3. Start worker in foreground
	fmt.Println(style.CreateHelpBox("Starting worker..."))
	workerCtx, cancelWorker := context.WithCancel(ctx)
	defer cancelWorker()

	if err := e.startWorker(workerCtx, queue.ID); err != nil {
		return fmt.Errorf("failed to start worker: %w", err)
	}

	// 4. Wait for worker heartbeat
	fmt.Println("⏳ Waiting for worker to be ready...")
	if err := e.waitForWorkerReady(ctx, queue.ID, 30*time.Second); err != nil {
		return fmt.Errorf("worker not ready: %w", err)
	}
	fmt.Println("✓ Worker is ready")
	fmt.Println()

	// 5. Submit execution
	fmt.Println(style.CreateHelpBox("Submitting execution..."))
	execution, err := e.submitExecution(ctx, queue.ID)
	if err != nil {
		return fmt.Errorf("failed to submit execution: %w", err)
	}

	executionID := execution.GetID()

	// Show execution metadata
	metadata := map[string]string{
		"Execution ID": executionID,
		"Status":       style.CreateStatusBadge(string(execution.Status)),
	}
	fmt.Println(style.CreateMetadataBox(metadata))
	fmt.Println()

	// Show assistant prompt
	fmt.Print(style.AssistantPromptStyle.Render(" Assistant "))
	fmt.Print(" ")

	// 6. Stream output in real-time
	return e.streamExecutionOutput(ctx, executionID)
}

// createEphemeralQueue creates a new ephemeral worker queue with auto-cleanup
func (e *OnDemandExecutor) createEphemeralQueue(ctx context.Context) (*entities.WorkerQueue, error) {
	// Generate unique queue name
	name := fmt.Sprintf("ephemeral-%s", generateShortID())

	// Set ephemeral flags
	trueVal := true
	ttl := 3600 // 1 hour - queue auto-cleanup after execution completes

	req := &entities.WorkerQueueCreateRequest{
		Name:                    name,
		EnvironmentID:           e.environmentID,
		Ephemeral:               &trueVal,
		SingleExecutionMode:     &trueVal,
		AutoCleanupAfterSeconds: &ttl,
	}

	return e.client.CreateWorkerQueue(e.environmentID, req)
}

// startWorker starts a worker process in foreground mode
func (e *OnDemandExecutor) startWorker(ctx context.Context, queueID string) error {
	// Create worker options
	opts := &WorkerStartOptions{
		QueueID:        queueID,
		DeploymentType: "local",  // Always local for on-demand
		DaemonMode:     false,    // Always foreground
		cfg:            e.cfg,
	}

	// Start worker in goroutine (non-blocking)
	go func() {
		// Suppress worker output to keep terminal clean
		// Worker logs will still be available in worker.log if needed
		if err := opts.runLocalForeground(ctx); err != nil {
			// Only log critical errors
			if ctx.Err() == nil {
				fmt.Fprintf(os.Stderr, "\nWorker error: %v\n", err)
			}
		}
	}()

	return nil
}

// waitForWorkerReady polls until worker is registered and ready
func (e *OnDemandExecutor) waitForWorkerReady(ctx context.Context, queueID string, timeout time.Duration) error {
	ctx, cancel := context.WithTimeout(ctx, timeout)
	defer cancel()

	ticker := time.NewTicker(1 * time.Second)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			if ctx.Err() == context.DeadlineExceeded {
				return fmt.Errorf("timeout waiting for worker (ensure Python 3.8+ is installed)")
			}
			return ctx.Err()
		case <-ticker.C:
			workers, err := e.client.ListQueueWorkers(queueID)
			if err == nil && len(workers) > 0 {
				// Worker registered successfully
				return nil
			}
		}
	}
}

// submitExecution submits the execution request to the agent
func (e *OnDemandExecutor) submitExecution(ctx context.Context, queueID string) (*entities.AgentExecution, error) {
	streamFlag := true

	// Convert queueID string to pointer (nil if empty, backend will auto-select)
	var queuePtr *string
	if queueID != "" {
		queuePtr = &queueID
	}

	req := &entities.ExecuteAgentRequest{
		Prompt:        e.prompt,
		WorkerQueueID: queuePtr,
		Stream:        &streamFlag,
	}
	if e.systemPrompt != "" {
		req.SystemPrompt = &e.systemPrompt
	}

	// Prepare and add execution environment configuration if needed
	envConfig, err := e.prepareExecutionEnvironment()
	if err != nil {
		return nil, fmt.Errorf("failed to prepare execution environment: %w", err)
	}
	if envConfig != nil {
		req.ExecutionEnvironment = envConfig
	}

	return e.client.ExecuteAgentV2(e.agentID, req)
}

// prepareExecutionEnvironment prepares the execution environment configuration
func (e *OnDemandExecutor) prepareExecutionEnvironment() (*entities.ExecutionEnvironmentOverride, error) {
	// Check if any environment configuration is provided
	hasConfig := e.workingDir != "" || len(e.envVars) > 0 || e.envFile != "" ||
		len(e.secrets) > 0 || len(e.skillDirs) > 0 || e.timeout > 0

	if !hasConfig {
		return nil, nil
	}

	config := &entities.ExecutionEnvironmentOverride{}

	// 1. Load .env file if specified
	var envFileMap map[string]string
	if e.envFile != "" {
		var err error
		envFileMap, err = loadEnvFile(e.envFile)
		if err != nil {
			return nil, fmt.Errorf("failed to load .env file: %w", err)
		}
	}

	// 2. Parse CLI env vars
	var cliEnvMap map[string]string
	if len(e.envVars) > 0 {
		cliEnvMap = parseEnvVars(e.envVars)
	}

	// 3. Merge environment variables (CLI overrides .env file)
	if len(envFileMap) > 0 || len(cliEnvMap) > 0 {
		config.EnvVars = mergeEnvVars(envFileMap, cliEnvMap)
	}

	// 4. Validate and set working directory
	if e.workingDir != "" {
		if err := validateWorkingDir(e.workingDir); err != nil {
			return nil, fmt.Errorf("invalid working directory: %w", err)
		}
		config.WorkingDir = e.workingDir
	}

	// 5. Validate and set skill directories
	if len(e.skillDirs) > 0 {
		validatedDirs := []string{}
		for _, dir := range e.skillDirs {
			if err := validateSkillDir(dir); err != nil {
				return nil, fmt.Errorf("invalid skill directory %s: %w", dir, err)
			}
			validatedDirs = append(validatedDirs, dir)
		}
		config.SkillDirs = validatedDirs
	}

	// 6. Set secrets (server will fetch these from Kubiya API)
	if len(e.secrets) > 0 {
		config.Secrets = e.secrets
	}

	// 7. Set timeout
	if e.timeout > 0 {
		config.Timeout = e.timeout
	}

	return config, nil
}

// streamExecutionOutput streams execution output to terminal in real-time
func (e *OnDemandExecutor) streamExecutionOutput(ctx context.Context, executionID string) error {
	eventChan, errChan := e.client.StreamExecutionOutput(ctx, executionID)

	var fullResponse strings.Builder
	streamStarted := false

	for {
		select {
		case event, ok := <-eventChan:
			if !ok {
				// Channel closed
				if !streamStarted {
					// No streaming data received, fetch final result
					finalExecution, err := e.client.GetExecution(executionID)
					if err == nil && finalExecution.Response != nil {
						fmt.Println(*finalExecution.Response)
					}
				}
				fmt.Println()
				fmt.Println()
				fmt.Println(style.CreateSuccessBox("Execution completed successfully"))
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
				return fmt.Errorf("execution error: %s", event.Content)
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
				// Try to fetch final result as fallback
				finalExecution, fetchErr := e.client.GetExecution(executionID)
				if fetchErr == nil && finalExecution.Response != nil {
					fmt.Println()
					fmt.Println("Partial output:")
					fmt.Println(*finalExecution.Response)
				}
				return fmt.Errorf("streaming error: %w", err)
			}

		case <-ctx.Done():
			fmt.Println()
			fmt.Println(style.CreateWarningBox("Execution interrupted by user"))
			return ctx.Err()
		}
	}
}

// cleanup removes all resources (queue + worker)
func (e *OnDemandExecutor) cleanup(queueID string) {
	// Always cleanup, even on errors
	// Delete queue (async, don't block user)
	go func() {
		if err := e.client.DeleteWorkerQueue(queueID); err != nil {
			// Warn but don't fail (TTL will clean up eventually)
			fmt.Fprintf(os.Stderr, "Warning: Queue cleanup failed (will auto-cleanup via TTL): %v\n", err)
		}
	}()

	// Give cleanup a moment to start
	time.Sleep(100 * time.Millisecond)
}

// generateShortID generates an 8-character random ID for queue names
func generateShortID() string {
	return uuid.New().String()[:8]
}
