package cli

import (
	"context"
	_ "embed"
	"fmt"
	"os"
	"os/exec"
	"os/signal"
	"strings"
	"syscall"
	"time"

	"github.com/docker/docker/api/types/container"
	"github.com/docker/docker/api/types/image"
	"github.com/docker/docker/client"
	"github.com/docker/docker/pkg/stdcopy"
	"github.com/kubiyabot/cli/internal/config"
	"github.com/spf13/cobra"
)

const (
	defaultWorkerImage = "ghcr.io/kubiyabot/agent-worker"
	defaultImageTag    = "latest"
	controlPlaneURL    = "https://agent-control-plane.vercel.app"
)

type WorkerStartOptions struct {
	// Worker configuration
	QueueID          string
	DeploymentType   string // "docker" or "local"
	DaemonMode       bool   // Run in background with supervision

	// Docker options
	Image            string
	ImageTag         string
	AutoPull         bool

	// Daemon options
	MaxLogSize       int64
	MaxLogBackups    int

	cfg              *config.Config
}

func newWorkerStartCommand(cfg *config.Config) *cobra.Command {
	opts := &WorkerStartOptions{
		cfg: cfg,
	}

	cmd := &cobra.Command{
		Use:   "start",
		Short: "🚀 Start an agent worker",
		Long: `Start an agent worker to execute tasks from a queue.

The worker will run in the foreground by default. Press Ctrl+C to stop.
Use -d flag to run in daemon mode with automatic crash recovery.

Deployment Types:
  • local  - Run worker locally with embedded Python (no Docker required)
  • docker - Run worker in Docker container (default)

Examples:
  # Start worker locally (no Docker)
  kubiya worker start --queue-id=<queue-id> --type=local

  # Start worker in daemon mode with supervision
  kubiya worker start --queue-id=<queue-id> --type=local -d

  # Start worker in Docker
  kubiya worker start --queue-id=<queue-id> --type=docker

  # With custom image tag
  kubiya worker start --queue-id=<queue-id> --image-tag=v1.2.3`,
		RunE: func(cmd *cobra.Command, args []string) error {
			return opts.Run(cmd.Context())
		},
	}

	cmd.Flags().StringVar(&opts.QueueID, "queue-id", "", "Worker queue ID (required)")
	cmd.Flags().StringVar(&opts.DeploymentType, "type", "local", "Deployment type: local or docker")
	cmd.Flags().BoolVarP(&opts.DaemonMode, "daemon", "d", false, "Run in daemon mode with supervision and crash recovery")
	cmd.Flags().Int64Var(&opts.MaxLogSize, "max-log-size", defaultMaxLogSize, "Maximum log file size in bytes (daemon mode)")
	cmd.Flags().IntVar(&opts.MaxLogBackups, "max-log-backups", defaultMaxBackups, "Maximum number of log backup files (daemon mode)")
	cmd.Flags().StringVar(&opts.Image, "image", defaultWorkerImage, "Worker Docker image (docker mode)")
	cmd.Flags().StringVar(&opts.ImageTag, "image-tag", defaultImageTag, "Worker image tag (docker mode)")
	cmd.Flags().BoolVar(&opts.AutoPull, "pull", true, "Automatically pull latest image (docker mode)")

	cmd.MarkFlagRequired("queue-id")

	return cmd
}

func (opts *WorkerStartOptions) Run(ctx context.Context) error {
	// Validate deployment type
	opts.DeploymentType = strings.ToLower(opts.DeploymentType)
	if opts.DeploymentType != "docker" && opts.DeploymentType != "local" {
		return fmt.Errorf("invalid deployment type: %s (must be 'docker' or 'local')", opts.DeploymentType)
	}

	// Route to appropriate implementation
	if opts.DeploymentType == "local" {
		return opts.RunLocal(ctx)
	}
	return opts.RunDocker(ctx)
}

func (opts *WorkerStartOptions) RunLocal(ctx context.Context) error {
	// Handle daemon mode
	if opts.DaemonMode {
		// Check if we're the child process
		if !IsDaemonChild() {
			// We're the parent - fork and exit
			if err := Daemonize(); err != nil {
				return err
			}
			// Parent exits here (won't reach this)
		}
		// Continue as daemon child
		return opts.runLocalDaemon(ctx)
	}

	// Run in foreground mode
	return opts.runLocalForeground(ctx)
}

func (opts *WorkerStartOptions) runLocalForeground(ctx context.Context) error {
	// Print beautiful startup banner
	fmt.Println()
	fmt.Println(strings.Repeat("═", 80))
	fmt.Println("🚀  KUBIYA AGENT WORKER")
	fmt.Println(strings.Repeat("═", 80))
	fmt.Println()

	// Configuration section
	fmt.Println("📋 CONFIGURATION")
	fmt.Println(strings.Repeat("─", 80))
	fmt.Printf("   Queue ID:         %s\n", opts.QueueID)
	fmt.Printf("   Deployment Type:  Local (Python)\n")
	fmt.Printf("   Control Plane:    %s\n", controlPlaneURL)
	fmt.Println()

	// Check API key
	if opts.cfg.APIKey == "" {
		return fmt.Errorf("❌ KUBIYA_API_KEY is required\nRun: kubiya login")
	}
	fmt.Println("✓ API Key authenticated")

	// Pre-flight checks section
	fmt.Println()
	fmt.Println("🔍 PRE-FLIGHT CHECKS")
	fmt.Println(strings.Repeat("─", 80))

	// Check if Python 3 is installed
	pythonCmd := "python3"
	checkCmd := exec.Command(pythonCmd, "--version")
	output, err := checkCmd.CombinedOutput()
	if err != nil {
		return fmt.Errorf("❌ Python 3 is not installed. Please install Python 3.8 or later")
	}
	pythonVersion := strings.TrimSpace(string(output))
	fmt.Printf("✓ Python found: %s\n", pythonVersion)

	// Create temporary directory for worker
	homeDir, err := os.UserHomeDir()
	if err != nil {
		return fmt.Errorf("failed to get home directory: %w", err)
	}

	workerDir := fmt.Sprintf("%s/.kubiya/workers/%s", homeDir, opts.QueueID)
	if err := os.MkdirAll(workerDir, 0755); err != nil {
		return fmt.Errorf("failed to create worker directory: %w", err)
	}

	fmt.Printf("✓ Worker directory: %s\n", workerDir)

	// Setup section
	fmt.Println()
	fmt.Println("⚙️  WORKER SETUP")
	fmt.Println(strings.Repeat("─", 80))

	// Write worker.py script
	workerPyPath := fmt.Sprintf("%s/worker.py", workerDir)
	if err := os.WriteFile(workerPyPath, []byte(workerPyScript), 0644); err != nil {
		return fmt.Errorf("❌ failed to write worker.py: %w", err)
	}

	// Write requirements.txt
	requirementsPath := fmt.Sprintf("%s/requirements.txt", workerDir)
	if err := os.WriteFile(requirementsPath, []byte(requirementsTxt), 0644); err != nil {
		return fmt.Errorf("❌ failed to write requirements.txt: %w", err)
	}

	// Write workflows directory
	workflowsDir := fmt.Sprintf("%s/workflows", workerDir)
	if err := os.MkdirAll(workflowsDir, 0755); err != nil {
		return fmt.Errorf("❌ failed to create workflows directory: %w", err)
	}

	if err := os.WriteFile(fmt.Sprintf("%s/__init__.py", workflowsDir), []byte(workflowsInit), 0644); err != nil {
		return fmt.Errorf("❌ failed to write workflows __init__.py: %w", err)
	}

	if err := os.WriteFile(fmt.Sprintf("%s/agent_execution.py", workflowsDir), []byte(agentExecutionWorkflow), 0644); err != nil {
		return fmt.Errorf("❌ failed to write agent_execution.py: %w", err)
	}

	if err := os.WriteFile(fmt.Sprintf("%s/team_execution.py", workflowsDir), []byte(teamExecutionWorkflow), 0644); err != nil {
		return fmt.Errorf("❌ failed to write team_execution.py: %w", err)
	}

	// Write activities directory
	activitiesDir := fmt.Sprintf("%s/activities", workerDir)
	if err := os.MkdirAll(activitiesDir, 0755); err != nil {
		return fmt.Errorf("❌ failed to create activities directory: %w", err)
	}

	if err := os.WriteFile(fmt.Sprintf("%s/__init__.py", activitiesDir), []byte(activitiesInit), 0644); err != nil {
		return fmt.Errorf("❌ failed to write activities __init__.py: %w", err)
	}

	if err := os.WriteFile(fmt.Sprintf("%s/agent_activities.py", activitiesDir), []byte(agentActivities), 0644); err != nil {
		return fmt.Errorf("❌ failed to write agent_activities.py: %w", err)
	}

	if err := os.WriteFile(fmt.Sprintf("%s/team_activities.py", activitiesDir), []byte(teamActivities), 0644); err != nil {
		return fmt.Errorf("❌ failed to write team_activities.py: %w", err)
	}

	if err := os.WriteFile(fmt.Sprintf("%s/toolset_activities.py", activitiesDir), []byte(toolsetActivities), 0644); err != nil {
		return fmt.Errorf("❌ failed to write toolset_activities.py: %w", err)
	}

	// Write control_plane_client.py (root level)
	if err := os.WriteFile(fmt.Sprintf("%s/control_plane_client.py", workerDir), []byte(controlPlaneClient), 0644); err != nil {
		return fmt.Errorf("❌ failed to write control_plane_client.py: %w", err)
	}

	// Write models directory
	modelsDir := fmt.Sprintf("%s/models", workerDir)
	if err := os.MkdirAll(modelsDir, 0755); err != nil {
		return fmt.Errorf("❌ failed to create models directory: %w", err)
	}

	if err := os.WriteFile(fmt.Sprintf("%s/__init__.py", modelsDir), []byte(modelsInit), 0644); err != nil {
		return fmt.Errorf("❌ failed to write models __init__.py: %w", err)
	}

	if err := os.WriteFile(fmt.Sprintf("%s/inputs.py", modelsDir), []byte(modelsInputs), 0644); err != nil {
		return fmt.Errorf("❌ failed to write models inputs.py: %w", err)
	}

	// Write services directory
	servicesDir := fmt.Sprintf("%s/services", workerDir)
	if err := os.MkdirAll(servicesDir, 0755); err != nil {
		return fmt.Errorf("❌ failed to create services directory: %w", err)
	}

	if err := os.WriteFile(fmt.Sprintf("%s/__init__.py", servicesDir), []byte(servicesInit), 0644); err != nil {
		return fmt.Errorf("❌ failed to write services __init__.py: %w", err)
	}

	if err := os.WriteFile(fmt.Sprintf("%s/agent_executor.py", servicesDir), []byte(agentExecutorService), 0644); err != nil {
		return fmt.Errorf("❌ failed to write agent_executor.py: %w", err)
	}

	if err := os.WriteFile(fmt.Sprintf("%s/agent_executor_v2.py", servicesDir), []byte(agentExecutorV2Service), 0644); err != nil {
		return fmt.Errorf("❌ failed to write agent_executor_v2.py: %w", err)
	}

	if err := os.WriteFile(fmt.Sprintf("%s/cancellation_manager.py", servicesDir), []byte(cancellationManagerService), 0644); err != nil {
		return fmt.Errorf("❌ failed to write cancellation_manager.py: %w", err)
	}

	if err := os.WriteFile(fmt.Sprintf("%s/session_service.py", servicesDir), []byte(sessionService), 0644); err != nil {
		return fmt.Errorf("❌ failed to write session_service.py: %w", err)
	}

	if err := os.WriteFile(fmt.Sprintf("%s/team_executor.py", servicesDir), []byte(teamExecutorService), 0644); err != nil {
		return fmt.Errorf("❌ failed to write team_executor.py: %w", err)
	}

	if err := os.WriteFile(fmt.Sprintf("%s/toolset_factory.py", servicesDir), []byte(toolsetFactoryService), 0644); err != nil {
		return fmt.Errorf("❌ failed to write toolset_factory.py: %w", err)
	}

	if err := os.WriteFile(fmt.Sprintf("%s/data_visualization.py", servicesDir), []byte(dataVisualizationService), 0644); err != nil {
		return fmt.Errorf("❌ failed to write data_visualization.py: %w", err)
	}

	// Write runtimes directory
	runtimesDir := fmt.Sprintf("%s/runtimes", workerDir)
	if err := os.MkdirAll(runtimesDir, 0755); err != nil {
		return fmt.Errorf("❌ failed to create runtimes directory: %w", err)
	}

	if err := os.WriteFile(fmt.Sprintf("%s/__init__.py", runtimesDir), []byte(runtimesInit), 0644); err != nil {
		return fmt.Errorf("❌ failed to write runtimes __init__.py: %w", err)
	}

	if err := os.WriteFile(fmt.Sprintf("%s/base.py", runtimesDir), []byte(runtimesBase), 0644); err != nil {
		return fmt.Errorf("❌ failed to write runtimes base.py: %w", err)
	}

	if err := os.WriteFile(fmt.Sprintf("%s/claude_code_runtime.py", runtimesDir), []byte(runtimesClaudeCode), 0644); err != nil {
		return fmt.Errorf("❌ failed to write claude_code_runtime.py: %w", err)
	}

	if err := os.WriteFile(fmt.Sprintf("%s/default_runtime.py", runtimesDir), []byte(runtimesDefault), 0644); err != nil {
		return fmt.Errorf("❌ failed to write default_runtime.py: %w", err)
	}

	if err := os.WriteFile(fmt.Sprintf("%s/factory.py", runtimesDir), []byte(runtimesFactory), 0644); err != nil {
		return fmt.Errorf("❌ failed to write runtimes factory.py: %w", err)
	}

	// Write utils directory
	utilsDir := fmt.Sprintf("%s/utils", workerDir)
	if err := os.MkdirAll(utilsDir, 0755); err != nil {
		return fmt.Errorf("❌ failed to create utils directory: %w", err)
	}

	if err := os.WriteFile(fmt.Sprintf("%s/__init__.py", utilsDir), []byte(utilsInit), 0644); err != nil {
		return fmt.Errorf("❌ failed to write utils __init__.py: %w", err)
	}

	if err := os.WriteFile(fmt.Sprintf("%s/retry_utils.py", utilsDir), []byte(utilsRetry), 0644); err != nil {
		return fmt.Errorf("❌ failed to write retry_utils.py: %w", err)
	}

	if err := os.WriteFile(fmt.Sprintf("%s/streaming_utils.py", utilsDir), []byte(utilsStreaming), 0644); err != nil {
		return fmt.Errorf("❌ failed to write streaming_utils.py: %w", err)
	}

	fmt.Println("✓ Worker code deployed from embedded binaries")
	fmt.Println("✓ Workflows and activities configured")

	// Create virtual environment if it doesn't exist
	venvDir := fmt.Sprintf("%s/venv", workerDir)
	if _, err := os.Stat(venvDir); os.IsNotExist(err) {
		fmt.Println()
		fmt.Println("📦 Creating Python virtual environment...")
		venvCmd := exec.Command(pythonCmd, "-m", "venv", venvDir)
		venvCmd.Stdout = os.Stdout
		venvCmd.Stderr = os.Stderr
		if err := venvCmd.Run(); err != nil {
			return fmt.Errorf("❌ failed to create virtual environment: %w", err)
		}
		fmt.Println("✓ Virtual environment created")
	} else {
		fmt.Println("✓ Virtual environment exists")
	}

	// Determine pip and python paths in venv
	pipPath := fmt.Sprintf("%s/bin/pip", venvDir)
	pythonPath := fmt.Sprintf("%s/bin/python", venvDir)

	// Install/upgrade pip
	fmt.Println()
	fmt.Println("📦 DEPENDENCIES")
	fmt.Println(strings.Repeat("─", 80))
	fmt.Print("   Upgrading pip... ")
	pipUpgradeCmd := exec.Command(pipPath, "install", "--quiet", "--upgrade", "pip")
	if err := pipUpgradeCmd.Run(); err != nil {
		return fmt.Errorf("❌ failed to upgrade pip: %w", err)
	}
	fmt.Println("done")

	// Install dependencies
	fmt.Print("   Installing Python dependencies (temporalio, etc.)... ")
	installCmd := exec.Command(pipPath, "install", "--quiet", "-r", requirementsPath)
	if err := installCmd.Run(); err != nil {
		fmt.Println("failed")
		return fmt.Errorf("❌ failed to install dependencies: %w", err)
	}
	fmt.Println("done")
	fmt.Println("✓ All dependencies installed")

	// Setup signal handling
	sigChan := make(chan os.Signal, 1)
	signal.Notify(sigChan, os.Interrupt, syscall.SIGTERM)

	// Run worker in a goroutine
	done := make(chan error, 1)
	go func() {
		// Create Python command to run the worker
		workerCmd := exec.Command(
			pythonPath,
			workerPyPath,
			"--queue-id", opts.QueueID,
			"--api-key", opts.cfg.APIKey,
			"--control-plane-url", controlPlaneURL,
		)

		// Set environment variables
		workerCmd.Env = append(os.Environ(),
			fmt.Sprintf("QUEUE_ID=%s", opts.QueueID),
			fmt.Sprintf("KUBIYA_API_KEY=%s", opts.cfg.APIKey),
			fmt.Sprintf("CONTROL_PLANE_URL=%s", controlPlaneURL),
			"LOG_LEVEL=INFO",
		)

		// Connect stdout/stderr
		workerCmd.Stdout = os.Stdout
		workerCmd.Stderr = os.Stderr

		// Run the worker
		if err := workerCmd.Run(); err != nil {
			done <- fmt.Errorf("❌ worker failed: %w", err)
			return
		}

		done <- nil
	}()

	// Print beautiful ready message
	fmt.Println()
	fmt.Println(strings.Repeat("═", 80))
	fmt.Println("✅  WORKER READY")
	fmt.Println(strings.Repeat("═", 80))
	fmt.Println()
	fmt.Printf("   🎯 Queue ID:         %s\n", opts.QueueID)
	fmt.Printf("   🔗 Control Plane:    %s\n", controlPlaneURL)
	fmt.Printf("   🐍 Runtime:          Python (venv)\n")
	fmt.Println()
	fmt.Println("   The worker is now polling for tasks...")
	fmt.Println("   Press Ctrl+C to stop gracefully")
	fmt.Println()
	fmt.Println(strings.Repeat("─", 80))
	fmt.Println()

	// Wait for signal or completion
	select {
	case <-sigChan:
		fmt.Println()
		fmt.Println(strings.Repeat("─", 80))
		fmt.Println("🛑  SHUTTING DOWN")
		fmt.Println(strings.Repeat("─", 80))
		fmt.Println("   Gracefully stopping worker...")
		fmt.Println("✓  Worker stopped successfully")
		fmt.Println()
		return nil
	case err := <-done:
		if err != nil {
			return err
		}
		fmt.Println()
		fmt.Println(strings.Repeat("─", 80))
		fmt.Println("✓  Worker process exited")
		fmt.Println(strings.Repeat("─", 80))
		fmt.Println()
		return nil
	}
}

func (opts *WorkerStartOptions) RunDocker(ctx context.Context) error {
	// Initialize Docker client
	cli, err := client.NewClientWithOpts(client.FromEnv, client.WithAPIVersionNegotiation())
	if err != nil {
		return fmt.Errorf("failed to create Docker client: %w\nIs Docker running?", err)
	}
	defer cli.Close()

	// Verify Docker is running
	if _, err := cli.Ping(ctx); err != nil {
		return fmt.Errorf("Docker daemon is not running: %w\nPlease start Docker and try again", err)
	}

	fmt.Println("✓ Docker is running")

	// Build full image name
	fullImage := fmt.Sprintf("%s:%s", opts.Image, opts.ImageTag)

	// Pull image if requested
	if opts.AutoPull {
		fmt.Printf("📥 Pulling image %s...\n", fullImage)
		reader, err := cli.ImagePull(ctx, fullImage, image.PullOptions{})
		if err != nil {
			return fmt.Errorf("failed to pull image: %w", err)
		}
		defer reader.Close()
		if _, err := stdcopy.StdCopy(os.Stdout, os.Stderr, reader); err != nil {
			// Ignore copy errors
		}
		fmt.Println("✓ Image pulled successfully")
	}

	// Prepare environment variables
	env := []string{
		fmt.Sprintf("QUEUE_ID=%s", opts.QueueID),
		fmt.Sprintf("CONTROL_PLANE_URL=%s", controlPlaneURL),
		"LOG_LEVEL=INFO",
	}

	if opts.cfg.APIKey != "" {
		env = append(env, fmt.Sprintf("KUBIYA_API_KEY=%s", opts.cfg.APIKey))
	}

	// Create container config
	containerConfig := &container.Config{
		Image: fullImage,
		Env:   env,
		Tty:   false,
		Labels: map[string]string{
			"kubiya.worker":     "true",
			"kubiya.queue-id":   opts.QueueID,
			"kubiya.created-by": "kubiya-cli",
		},
	}

	hostConfig := &container.HostConfig{
		AutoRemove: true,
	}

	fmt.Printf("🚀 Starting worker for queue %s...\n", opts.QueueID)
	fmt.Println("   Press Ctrl+C to stop")
	fmt.Println(strings.Repeat("─", 80))

	// Create container
	resp, err := cli.ContainerCreate(ctx, containerConfig, hostConfig, nil, nil, "")
	if err != nil {
		return fmt.Errorf("failed to create container: %w", err)
	}

	containerID := resp.ID

	// Setup signal handling
	sigChan := make(chan os.Signal, 1)
	signal.Notify(sigChan, os.Interrupt, syscall.SIGTERM)

	// Start container
	if err := cli.ContainerStart(ctx, containerID, container.StartOptions{}); err != nil {
		return fmt.Errorf("failed to start container: %w", err)
	}

	// Stream logs
	logsDone := make(chan error, 1)
	go func() {
		time.Sleep(500 * time.Millisecond)

		logOptions := container.LogsOptions{
			ShowStdout: true,
			ShowStderr: true,
			Follow:     true,
			Timestamps: false,
		}

		reader, err := cli.ContainerLogs(ctx, containerID, logOptions)
		if err != nil {
			logsDone <- fmt.Errorf("failed to get container logs: %w", err)
			return
		}
		defer reader.Close()

		_, err = stdcopy.StdCopy(os.Stdout, os.Stderr, reader)
		logsDone <- err
	}()

	// Wait for signal or completion
	select {
	case <-sigChan:
		fmt.Println("\n\n🛑 Stopping worker...")
		timeout := 10
		if err := cli.ContainerStop(ctx, containerID, container.StopOptions{Timeout: &timeout}); err != nil {
			fmt.Fprintf(os.Stderr, "Warning: failed to stop container gracefully: %v\n", err)
		}
		fmt.Println("✓ Worker stopped")
		return nil
	case err := <-logsDone:
		if err != nil {
			return fmt.Errorf("error streaming logs: %w", err)
		}
		fmt.Println("\n✓ Worker finished")
		return nil
	}
}

func (opts *WorkerStartOptions) runLocalDaemon(ctx context.Context) error {
	homeDir, err := os.UserHomeDir()
	if err != nil {
		return fmt.Errorf("failed to get home directory: %w", err)
	}

	workerDir := fmt.Sprintf("%s/.kubiya/workers/%s", homeDir, opts.QueueID)

	// Setup worker directory and files (if parent process)
	if !IsDaemonChild() {
		// Print startup info for parent
		fmt.Println()
		fmt.Println(strings.Repeat("═", 80))
		fmt.Println("🚀  KUBIYA AGENT WORKER (DAEMON MODE)")
		fmt.Println(strings.Repeat("═", 80))
		fmt.Println()
		fmt.Println("📋 CONFIGURATION")
		fmt.Println(strings.Repeat("─", 80))
		fmt.Printf("   Queue ID:         %s\n", opts.QueueID)
		fmt.Printf("   Deployment Type:  Local (Python)\n")
		fmt.Printf("   Mode:             Daemon with supervision\n")
		fmt.Printf("   Control Plane:    %s\n", controlPlaneURL)
		fmt.Printf("   Worker Directory: %s\n", workerDir)
		fmt.Println()

		// Setup will happen in child process
		return nil
	}

	// We're in the daemon child process now
	if err := os.MkdirAll(workerDir, 0755); err != nil {
		return fmt.Errorf("failed to create worker directory: %w", err)
	}

	// Check API key
	if opts.cfg.APIKey == "" {
		return fmt.Errorf("KUBIYA_API_KEY is required")
	}

	// Check Python
	pythonCmd := "python3"
	checkCmd := exec.Command(pythonCmd, "--version")
	if _, err := checkCmd.CombinedOutput(); err != nil {
		return fmt.Errorf("Python 3 is not installed")
	}

	// Setup worker files (same as foreground mode)
	if err := opts.setupWorkerFiles(workerDir); err != nil {
		return err
	}

	// Setup virtual environment
	venvDir := fmt.Sprintf("%s/venv", workerDir)
	pythonPath, pipPath := fmt.Sprintf("%s/bin/python", venvDir), fmt.Sprintf("%s/bin/pip", venvDir)

	if _, err := os.Stat(venvDir); os.IsNotExist(err) {
		venvCmd := exec.Command(pythonCmd, "-m", "venv", venvDir)
		if err := venvCmd.Run(); err != nil {
			return fmt.Errorf("failed to create virtual environment: %w", err)
		}

		// Install dependencies
		pipUpgradeCmd := exec.Command(pipPath, "install", "--quiet", "--upgrade", "pip")
		if err := pipUpgradeCmd.Run(); err != nil {
			return fmt.Errorf("failed to upgrade pip: %w", err)
		}

		requirementsPath := fmt.Sprintf("%s/requirements.txt", workerDir)
		installCmd := exec.Command(pipPath, "install", "--quiet", "-r", requirementsPath)
		if err := installCmd.Run(); err != nil {
			return fmt.Errorf("failed to install dependencies: %w", err)
		}
	}

	// Get or use environment-configured log size
	maxLogSize := opts.MaxLogSize
	if envSize := GetMaxLogSizeFromEnv(); envSize != defaultMaxLogSize {
		maxLogSize = envSize
	}

	// Create process supervisor
	supervisor, err := NewProcessSupervisor(opts.QueueID, workerDir, maxLogSize, opts.MaxLogBackups)
	if err != nil {
		return fmt.Errorf("failed to create supervisor: %w", err)
	}

	// Start supervised worker
	workerPyPath := fmt.Sprintf("%s/worker.py", workerDir)
	daemonInfo, err := supervisor.Start(pythonPath, workerPyPath, opts.cfg.APIKey)
	if err != nil {
		return fmt.Errorf("failed to start daemon: %w", err)
	}

	// Write startup info file for parent to read
	infoFile := fmt.Sprintf("%s/daemon_info.txt", workerDir)
	startupInfo := fmt.Sprintf(`
╔═══════════════════════════════════════════════════════════════════════════════╗
║                    KUBIYA AGENT WORKER - DAEMON STARTED                       ║
╚═══════════════════════════════════════════════════════════════════════════════╝

📋 DAEMON INFORMATION
────────────────────────────────────────────────────────────────────────────────
   Process ID (PID):    %d
   Queue ID:            %s
   Worker Directory:    %s
   Log File:            %s
   PID File:            %s
   Started At:          %s

✅ SUPERVISION ENABLED
────────────────────────────────────────────────────────────────────────────────
   • Automatic crash recovery with exponential backoff
   • Maximum restart attempts: %d
   • Rotating logs (max size: %d MB, backups: %d)
   • Health monitoring enabled

📊 MANAGEMENT COMMANDS
────────────────────────────────────────────────────────────────────────────────
   View logs:     tail -f %s
   Stop worker:   kubiya worker stop --queue-id=%s
   Check status:  kubiya worker status --queue-id=%s

🎯 The worker is now running in the background and will automatically restart if it crashes.
`,
		daemonInfo.PID,
		daemonInfo.QueueID,
		daemonInfo.WorkerDir,
		daemonInfo.LogFile,
		daemonInfo.PIDFile,
		daemonInfo.StartedAt.Format("2006-01-02 15:04:05"),
		maxRestartAttempts,
		maxLogSize/(1024*1024),
		opts.MaxLogBackups,
		daemonInfo.LogFile,
		opts.QueueID,
		opts.QueueID,
	)
	os.WriteFile(infoFile, []byte(startupInfo), 0644)

	// Setup signal handling for graceful shutdown
	sigChan := make(chan os.Signal, 1)
	signal.Notify(sigChan, os.Interrupt, syscall.SIGTERM)

	// Wait for termination signal
	<-sigChan
	supervisor.Stop()

	return nil
}

func (opts *WorkerStartOptions) setupWorkerFiles(workerDir string) error {
	// Write worker.py script
	workerPyPath := fmt.Sprintf("%s/worker.py", workerDir)
	if err := os.WriteFile(workerPyPath, []byte(workerPyScript), 0644); err != nil {
		return fmt.Errorf("failed to write worker.py: %w", err)
	}

	// Write requirements.txt
	requirementsPath := fmt.Sprintf("%s/requirements.txt", workerDir)
	if err := os.WriteFile(requirementsPath, []byte(requirementsTxt), 0644); err != nil {
		return fmt.Errorf("failed to write requirements.txt: %w", err)
	}

	// Write workflows directory
	workflowsDir := fmt.Sprintf("%s/workflows", workerDir)
	if err := os.MkdirAll(workflowsDir, 0755); err != nil {
		return fmt.Errorf("failed to create workflows directory: %w", err)
	}

	if err := os.WriteFile(fmt.Sprintf("%s/__init__.py", workflowsDir), []byte(workflowsInit), 0644); err != nil {
		return fmt.Errorf("failed to write workflows __init__.py: %w", err)
	}

	if err := os.WriteFile(fmt.Sprintf("%s/agent_execution.py", workflowsDir), []byte(agentExecutionWorkflow), 0644); err != nil {
		return fmt.Errorf("failed to write agent_execution.py: %w", err)
	}

	if err := os.WriteFile(fmt.Sprintf("%s/team_execution.py", workflowsDir), []byte(teamExecutionWorkflow), 0644); err != nil {
		return fmt.Errorf("failed to write team_execution.py: %w", err)
	}

	// Write activities directory
	activitiesDir := fmt.Sprintf("%s/activities", workerDir)
	if err := os.MkdirAll(activitiesDir, 0755); err != nil {
		return fmt.Errorf("failed to create activities directory: %w", err)
	}

	if err := os.WriteFile(fmt.Sprintf("%s/__init__.py", activitiesDir), []byte(activitiesInit), 0644); err != nil {
		return fmt.Errorf("failed to write activities __init__.py: %w", err)
	}

	if err := os.WriteFile(fmt.Sprintf("%s/agent_activities.py", activitiesDir), []byte(agentActivities), 0644); err != nil {
		return fmt.Errorf("failed to write agent_activities.py: %w", err)
	}

	if err := os.WriteFile(fmt.Sprintf("%s/team_activities.py", activitiesDir), []byte(teamActivities), 0644); err != nil {
		return fmt.Errorf("failed to write team_activities.py: %w", err)
	}

	if err := os.WriteFile(fmt.Sprintf("%s/toolset_activities.py", activitiesDir), []byte(toolsetActivities), 0644); err != nil {
		return fmt.Errorf("failed to write toolset_activities.py: %w", err)
	}

	// Write control_plane_client.py (root level)
	if err := os.WriteFile(fmt.Sprintf("%s/control_plane_client.py", workerDir), []byte(controlPlaneClient), 0644); err != nil {
		return fmt.Errorf("failed to write control_plane_client.py: %w", err)
	}

	// Write models directory
	modelsDir := fmt.Sprintf("%s/models", workerDir)
	if err := os.MkdirAll(modelsDir, 0755); err != nil {
		return fmt.Errorf("failed to create models directory: %w", err)
	}

	if err := os.WriteFile(fmt.Sprintf("%s/__init__.py", modelsDir), []byte(modelsInit), 0644); err != nil {
		return fmt.Errorf("failed to write models __init__.py: %w", err)
	}

	if err := os.WriteFile(fmt.Sprintf("%s/inputs.py", modelsDir), []byte(modelsInputs), 0644); err != nil {
		return fmt.Errorf("failed to write models inputs.py: %w", err)
	}

	// Write services directory
	servicesDir := fmt.Sprintf("%s/services", workerDir)
	if err := os.MkdirAll(servicesDir, 0755); err != nil {
		return fmt.Errorf("failed to create services directory: %w", err)
	}

	if err := os.WriteFile(fmt.Sprintf("%s/__init__.py", servicesDir), []byte(servicesInit), 0644); err != nil {
		return fmt.Errorf("failed to write services __init__.py: %w", err)
	}

	if err := os.WriteFile(fmt.Sprintf("%s/agent_executor.py", servicesDir), []byte(agentExecutorService), 0644); err != nil {
		return fmt.Errorf("failed to write agent_executor.py: %w", err)
	}

	if err := os.WriteFile(fmt.Sprintf("%s/agent_executor_v2.py", servicesDir), []byte(agentExecutorV2Service), 0644); err != nil {
		return fmt.Errorf("failed to write agent_executor_v2.py: %w", err)
	}

	if err := os.WriteFile(fmt.Sprintf("%s/cancellation_manager.py", servicesDir), []byte(cancellationManagerService), 0644); err != nil {
		return fmt.Errorf("failed to write cancellation_manager.py: %w", err)
	}

	if err := os.WriteFile(fmt.Sprintf("%s/session_service.py", servicesDir), []byte(sessionService), 0644); err != nil {
		return fmt.Errorf("failed to write session_service.py: %w", err)
	}

	if err := os.WriteFile(fmt.Sprintf("%s/team_executor.py", servicesDir), []byte(teamExecutorService), 0644); err != nil {
		return fmt.Errorf("failed to write team_executor.py: %w", err)
	}

	if err := os.WriteFile(fmt.Sprintf("%s/toolset_factory.py", servicesDir), []byte(toolsetFactoryService), 0644); err != nil {
		return fmt.Errorf("failed to write toolset_factory.py: %w", err)
	}

	if err := os.WriteFile(fmt.Sprintf("%s/data_visualization.py", servicesDir), []byte(dataVisualizationService), 0644); err != nil {
		return fmt.Errorf("failed to write data_visualization.py: %w", err)
	}

	// Write runtimes directory
	runtimesDir := fmt.Sprintf("%s/runtimes", workerDir)
	if err := os.MkdirAll(runtimesDir, 0755); err != nil {
		return fmt.Errorf("failed to create runtimes directory: %w", err)
	}

	if err := os.WriteFile(fmt.Sprintf("%s/__init__.py", runtimesDir), []byte(runtimesInit), 0644); err != nil {
		return fmt.Errorf("failed to write runtimes __init__.py: %w", err)
	}

	if err := os.WriteFile(fmt.Sprintf("%s/base.py", runtimesDir), []byte(runtimesBase), 0644); err != nil {
		return fmt.Errorf("failed to write runtimes base.py: %w", err)
	}

	if err := os.WriteFile(fmt.Sprintf("%s/claude_code_runtime.py", runtimesDir), []byte(runtimesClaudeCode), 0644); err != nil {
		return fmt.Errorf("failed to write claude_code_runtime.py: %w", err)
	}

	if err := os.WriteFile(fmt.Sprintf("%s/default_runtime.py", runtimesDir), []byte(runtimesDefault), 0644); err != nil {
		return fmt.Errorf("failed to write default_runtime.py: %w", err)
	}

	if err := os.WriteFile(fmt.Sprintf("%s/factory.py", runtimesDir), []byte(runtimesFactory), 0644); err != nil {
		return fmt.Errorf("failed to write runtimes factory.py: %w", err)
	}

	// Write utils directory
	utilsDir := fmt.Sprintf("%s/utils", workerDir)
	if err := os.MkdirAll(utilsDir, 0755); err != nil {
		return fmt.Errorf("failed to create utils directory: %w", err)
	}

	if err := os.WriteFile(fmt.Sprintf("%s/__init__.py", utilsDir), []byte(utilsInit), 0644); err != nil {
		return fmt.Errorf("failed to write utils __init__.py: %w", err)
	}

	if err := os.WriteFile(fmt.Sprintf("%s/retry_utils.py", utilsDir), []byte(utilsRetry), 0644); err != nil {
		return fmt.Errorf("failed to write retry_utils.py: %w", err)
	}

	if err := os.WriteFile(fmt.Sprintf("%s/streaming_utils.py", utilsDir), []byte(utilsStreaming), 0644); err != nil {
		return fmt.Errorf("failed to write streaming_utils.py: %w", err)
	}

	return nil
}
