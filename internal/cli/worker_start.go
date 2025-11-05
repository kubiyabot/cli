package cli

import (
	"context"
	_ "embed"
	"fmt"
	"os"
	"os/exec"
	"os/signal"
	"runtime"
	"strconv"
	"strings"
	"syscall"

	"github.com/docker/docker/api/types/container"
	"github.com/docker/docker/api/types/image"
	"github.com/docker/docker/client"
	"github.com/docker/docker/pkg/stdcopy"
	"github.com/kubiyabot/cli/internal/config"
	"github.com/spf13/cobra"
)

const (
	defaultWorkerImage     = "ghcr.io/kubiyabot/agent-worker"
	defaultImageTag        = "latest"
	defaultControlPlaneURL = "https://control-plane.kubiya.ai"
	minPythonMajor         = 3
	minPythonMinor         = 8
)

//go:embed requirements.txt
var requirementsTxt string

type WorkerStartOptions struct {
	// Worker configuration
	QueueID        string
	DeploymentType string // "docker" or "local"
	DaemonMode     bool   // Run in background with supervision

	// Docker options
	Image     string
	ImageTag  string
	AutoPull  bool

	// Daemon options
	MaxLogSize    int64
	MaxLogBackups int

	// Python package options
	PackageVersion string // Version of kubiya-control-plane-api to install (default: ">=0.3.0")
	LocalWheel     string // Path to local wheel file (for development only)

	cfg *config.Config
}

// getControlPlaneURL returns the control plane URL, checking environment variable first
func getControlPlaneURL() string {
	if url := os.Getenv("CONTROL_PLANE_GATEWAY_URL"); url != "" {
		return url
	}
	return defaultControlPlaneURL
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
  • local  - Run worker locally with Python package (no Docker required)
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
	cmd.Flags().StringVar(&opts.PackageVersion, "package-version", "", "Version of kubiya-control-plane-api to install from PyPI (empty = latest, local mode)")
	cmd.Flags().StringVar(&opts.LocalWheel, "local-wheel", "", "Path to local wheel file for development (local mode)")

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
	fmt.Printf("   Deployment Type:  Local (Python Package)\n")
	fmt.Printf("   Control Plane:    %s\n", getControlPlaneURL())
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

	// Check Python version with comprehensive validation
	pythonCmd, pythonVersion, err := checkPythonVersion()
	if err != nil {
		return err
	}
	fmt.Printf("✓ Python found: %s\n", pythonVersion)

	// Check pip is available
	if err := checkPipAvailable(pythonCmd); err != nil {
		return err
	}
	fmt.Println("✓ pip package manager available")

	// Create worker directory
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

	// Determine pip path in venv (python path not needed for package binary)
	pipPath, _ := getVenvPaths(venvDir)

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

	// Install worker package with extras
	fmt.Print("   Installing kubiya-control-plane-api package")

	var installCmd *exec.Cmd
	if opts.LocalWheel != "" {
		// Install from local wheel (development mode) with [worker] extras
		if _, err := os.Stat(opts.LocalWheel); os.IsNotExist(err) {
			fmt.Println("... failed")
			return fmt.Errorf("❌ local wheel file not found: %s", opts.LocalWheel)
		}
		installCmd = exec.Command(pipPath, "install", "--quiet", "--force-reinstall", opts.LocalWheel+"[worker]")
		fmt.Printf(" (local: %s[worker])... ", opts.LocalWheel)
	} else {
		// Install from PyPI (production mode) with [worker] extras
		// Build package spec - handle both formats: ">=0.3.0" or "0.3.0"
		packageSpec := "kubiya-control-plane-api"
		if opts.PackageVersion != "" {
			// If version doesn't start with operator, assume ==
			if !strings.HasPrefix(opts.PackageVersion, ">=") &&
				!strings.HasPrefix(opts.PackageVersion, "==") &&
				!strings.HasPrefix(opts.PackageVersion, "~=") &&
				!strings.HasPrefix(opts.PackageVersion, "<") &&
				!strings.HasPrefix(opts.PackageVersion, ">") {
				packageSpec += "==" + opts.PackageVersion
			} else {
				packageSpec += opts.PackageVersion
			}
		}
		// Add [worker] extras for all dependencies
		packageSpec += "[worker]"
		installCmd = exec.Command(pipPath, "install", "--quiet", packageSpec)
		if opts.PackageVersion != "" {
			fmt.Printf(" (%s)... ", packageSpec)
		} else {
			fmt.Print(" (latest with [worker] extras)... ")
		}
	}

	if err := installCmd.Run(); err != nil {
		fmt.Println("failed")
		if opts.LocalWheel != "" {
			return fmt.Errorf("❌ failed to install worker package from local wheel: %w", err)
		}
		return fmt.Errorf("❌ failed to install worker package from PyPI: %w\nMake sure kubiya-control-plane-api is published", err)
	}
	fmt.Println("done")
	fmt.Println("✓ Worker package with all dependencies installed")

	// Verify worker binary is available
	workerBinary := fmt.Sprintf("%s/bin/kubiya-control-plane-worker", venvDir)
	if _, err := os.Stat(workerBinary); os.IsNotExist(err) {
		return fmt.Errorf("❌ worker binary not found at %s\nPackage may not have installed correctly", workerBinary)
	}
	fmt.Println("✓ Worker binary verified")

	// Setup signal handling
	sigChan := make(chan os.Signal, 1)
	signal.Notify(sigChan, os.Interrupt, syscall.SIGTERM)

	// Run worker in a goroutine
	done := make(chan error, 1)
	go func() {
		// Run the worker binary from the package with both env vars and CLI args
		controlPlaneURL := getControlPlaneURL()

		// Pass both env vars (safer) and CLI args (fallback)
		workerCmd := exec.Command(
			workerBinary,
			"--queue-id", opts.QueueID,
			"--api-key", opts.cfg.APIKey,
			"--control-plane-url", controlPlaneURL,
		)

		// Set environment variables - take precedence over CLI args
		workerCmd.Env = append(os.Environ(),
			fmt.Sprintf("QUEUE_ID=%s", opts.QueueID),
			fmt.Sprintf("KUBIYA_API_KEY=%s", opts.cfg.APIKey),
			fmt.Sprintf("CONTROL_PLANE_URL=%s", controlPlaneURL),
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
	fmt.Printf("   🔗 Control Plane:    %s\n", getControlPlaneURL())
	fmt.Printf("   📦 Package:          kubiya-control-plane-api[worker]\n")
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

// checkPythonVersion checks for Python 3.8+ and returns the command and version string
func checkPythonVersion() (string, string, error) {
	// Try python3 first, then python
	pythonCommands := []string{"python3", "python"}

	for _, cmd := range pythonCommands {
		// Check if command exists
		checkCmd := exec.Command(cmd, "--version")
		output, err := checkCmd.CombinedOutput()
		if err != nil {
			continue
		}

		version := strings.TrimSpace(string(output))

		// Parse version (format: "Python 3.11.9")
		parts := strings.Fields(version)
		if len(parts) < 2 {
			continue
		}

		versionParts := strings.Split(parts[1], ".")
		if len(versionParts) < 2 {
			continue
		}

		major, err := strconv.Atoi(versionParts[0])
		if err != nil {
			continue
		}

		minor, err := strconv.Atoi(versionParts[1])
		if err != nil {
			continue
		}

		// Check minimum version
		if major < minPythonMajor || (major == minPythonMajor && minor < minPythonMinor) {
			return "", "", fmt.Errorf("❌ Python %d.%d+ is required, but found %s\nPlease upgrade Python", minPythonMajor, minPythonMinor, version)
		}

		return cmd, version, nil
	}

	// No Python found
	return "", "", fmt.Errorf("❌ Python %d.%d+ is not installed\n\nInstallation instructions:\n%s", minPythonMajor, minPythonMinor, getPythonInstallInstructions())
}

// checkPipAvailable checks if pip is available
func checkPipAvailable(pythonCmd string) error {
	checkCmd := exec.Command(pythonCmd, "-m", "pip", "--version")
	if err := checkCmd.Run(); err != nil {
		return fmt.Errorf("❌ pip is not available\n\nTo install pip:\n%s -m ensurepip --upgrade", pythonCmd)
	}
	return nil
}

// getPythonInstallInstructions returns OS-specific Python installation instructions
func getPythonInstallInstructions() string {
	switch runtime.GOOS {
	case "darwin":
		return `  macOS:
    brew install python@3.11
    # or download from https://www.python.org/downloads/`
	case "linux":
		return `  Linux (Ubuntu/Debian):
    sudo apt update && sudo apt install python3.11 python3.11-venv

  Linux (CentOS/RHEL):
    sudo yum install python311

  Linux (Fedora):
    sudo dnf install python3.11`
	case "windows":
		return `  Windows:
    Download from https://www.python.org/downloads/
    Or use: winget install Python.Python.3.11`
	default:
		return "  Please visit https://www.python.org/downloads/ to download Python"
	}
}

// getVenvPaths returns the pip and python paths for the given venv directory
func getVenvPaths(venvDir string) (pipPath, pythonPath string) {
	if runtime.GOOS == "windows" {
		pipPath = fmt.Sprintf("%s\\Scripts\\pip.exe", venvDir)
		pythonPath = fmt.Sprintf("%s\\Scripts\\python.exe", venvDir)
	} else {
		pipPath = fmt.Sprintf("%s/bin/pip", venvDir)
		pythonPath = fmt.Sprintf("%s/bin/python", venvDir)
	}
	return
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
		fmt.Sprintf("CONTROL_PLANE_URL=%s", getControlPlaneURL()),
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

	// Print startup info for parent
	if !IsDaemonChild() {
		fmt.Println()
		fmt.Println(strings.Repeat("═", 80))
		fmt.Println("🚀  KUBIYA AGENT WORKER (DAEMON MODE)")
		fmt.Println(strings.Repeat("═", 80))
		fmt.Println()
		fmt.Println("📋 CONFIGURATION")
		fmt.Println(strings.Repeat("─", 80))
		fmt.Printf("   Queue ID:         %s\n", opts.QueueID)
		fmt.Printf("   Deployment Type:  Local (Python Package)\n")
		fmt.Printf("   Mode:             Daemon with supervision\n")
		fmt.Printf("   Control Plane:    %s\n", getControlPlaneURL())
		fmt.Printf("   Worker Directory: %s\n", workerDir)
		fmt.Println()

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

	// Check Python version
	pythonCmd, _, err := checkPythonVersion()
	if err != nil {
		return err
	}

	// Setup virtual environment
	venvDir := fmt.Sprintf("%s/venv", workerDir)
	pipPath, _ := getVenvPaths(venvDir)

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

		// Install kubiya-control-plane-api package with worker extras
		packageSpec := "kubiya-control-plane-api"
		if opts.PackageVersion != "" {
			if !strings.HasPrefix(opts.PackageVersion, ">=") &&
				!strings.HasPrefix(opts.PackageVersion, "==") &&
				!strings.HasPrefix(opts.PackageVersion, "~=") &&
				!strings.HasPrefix(opts.PackageVersion, "<") &&
				!strings.HasPrefix(opts.PackageVersion, ">") {
				packageSpec += "==" + opts.PackageVersion
			} else {
				packageSpec += opts.PackageVersion
			}
		}
		packageSpec += "[worker]"

		installCmd := exec.Command(pipPath, "install", "--quiet", packageSpec)
		if err := installCmd.Run(); err != nil {
			return fmt.Errorf("failed to install %s: %w", packageSpec, err)
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

	// Start supervised worker with kubiya-control-plane-worker command
	workerBinPath := fmt.Sprintf("%s/bin/kubiya-control-plane-worker", venvDir)
	daemonInfo, err := supervisor.Start(workerBinPath, "", opts.cfg.APIKey)
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

// setupWorkerFiles is no longer needed - we now install from pip package
// This function is kept for backward compatibility but does nothing
func (opts *WorkerStartOptions) setupWorkerFiles(workerDir string) error {
	// Workers now use kubiya-control-plane-api pip package
	// No file embedding needed
	return nil
}

