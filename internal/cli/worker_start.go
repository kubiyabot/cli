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
	"time"

	"github.com/docker/docker/api/types/container"
	"github.com/docker/docker/api/types/image"
	"github.com/docker/docker/client"
	"github.com/docker/docker/pkg/stdcopy"
	"github.com/kubiyabot/cli/internal/config"
	"github.com/kubiyabot/cli/internal/controlplane"
	"github.com/kubiyabot/cli/internal/output"
	"github.com/pterm/pterm"
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
	QueueID              string
	DeploymentType       string // "docker" or "local"
	DaemonMode           bool   // Run in background with supervision
	SingleExecutionMode  bool   // Exit after completing one task (for ephemeral local execution)
	ControlPlaneURL      string // Override control plane URL

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
	NoCache        bool   // Clear pip cache before installation

	// Auto-update options
	AutoUpdate           bool   // Enable automatic updates
	UpdateCheckInterval  string // Interval for checking updates (e.g., "5m", "10m")

	cfg *config.Config
}

// getControlPlaneURL returns the effective control plane URL with priority:
// 1. --control-plane-url flag
// 2. Config BaseURL (from context or environment variables)
// 3. Default
func (opts *WorkerStartOptions) getControlPlaneURL() string {
	if opts.ControlPlaneURL != "" {
		return opts.ControlPlaneURL
	}
	if opts.cfg.BaseURL != "" {
		return opts.cfg.BaseURL
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

Environment Variables:
  KUBIYA_WORKER_PACKAGE_VERSION  - Package version to install (e.g., "0.5.0", ">=0.3.0")
  KUBIYA_WORKER_LOCAL_WHEEL      - Path to local wheel file for development

Examples:
  # Start worker locally (no Docker)
  kubiya worker start --queue-id=<queue-id> --type=local

  # Start worker with specific package version
  kubiya worker start --queue-id=<queue-id> --package-version=0.5.0

  # Using environment variable
  export KUBIYA_WORKER_PACKAGE_VERSION=0.5.0
  kubiya worker start --queue-id=<queue-id> --type=local

  # Start worker in daemon mode with supervision
  kubiya worker start --queue-id=<queue-id> --type=local -d

  # Development: use local wheel file
  kubiya worker start --queue-id=<queue-id> --local-wheel=/path/to/wheel

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
	cmd.Flags().StringVar(&opts.ControlPlaneURL, "control-plane-url", "", "Control plane URL (default: from context or https://control-plane.kubiya.ai)")
	cmd.Flags().Int64Var(&opts.MaxLogSize, "max-log-size", defaultMaxLogSize, "Maximum log file size in bytes (daemon mode)")
	cmd.Flags().IntVar(&opts.MaxLogBackups, "max-log-backups", defaultMaxBackups, "Maximum number of log backup files (daemon mode)")
	cmd.Flags().StringVar(&opts.Image, "image", defaultWorkerImage, "Worker Docker image (docker mode)")
	cmd.Flags().StringVar(&opts.ImageTag, "image-tag", defaultImageTag, "Worker image tag (docker mode)")
	cmd.Flags().BoolVar(&opts.AutoPull, "pull", true, "Automatically pull latest image (docker mode)")
	cmd.Flags().StringVar(&opts.PackageVersion, "package-version", "", "Version of kubiya-control-plane-api to install from PyPI (env: KUBIYA_WORKER_PACKAGE_VERSION, empty = latest)")
	cmd.Flags().StringVar(&opts.LocalWheel, "local-wheel", "", "Path to local wheel file for development (env: KUBIYA_WORKER_LOCAL_WHEEL)")
	cmd.Flags().BoolVar(&opts.NoCache, "no-cache", false, "Clear pip cache before installation (useful for troubleshooting)")
	cmd.Flags().BoolVar(&opts.AutoUpdate, "auto-update", false, "Enable automatic worker updates (config + package)")
	cmd.Flags().StringVar(&opts.UpdateCheckInterval, "update-check-interval", "5m", "Interval for checking updates (e.g., 5m, 10m, 1h)")

	cmd.MarkFlagRequired("queue-id")

	return cmd
}

func (opts *WorkerStartOptions) Run(ctx context.Context) error {
	// Apply environment variables if flags not set
	if opts.PackageVersion == "" {
		if envVersion := os.Getenv("KUBIYA_WORKER_PACKAGE_VERSION"); envVersion != "" {
			opts.PackageVersion = envVersion
		}
	}
	if opts.LocalWheel == "" {
		if envWheel := os.Getenv("KUBIYA_WORKER_LOCAL_WHEEL"); envWheel != "" {
			opts.LocalWheel = envWheel
		}
	}

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
	// Create progress manager with auto-detected mode
	pm := output.NewProgressManager()
	progress := NewWorkerStartProgress(pm)

	// Show styled header with pterm
	pm.Println("")

	// Create a styled header box
	headerContent := pterm.Sprintf(
		"%s\n\n"+
		"  %s  %s\n"+
		"  %s  %s",
		pterm.Bold.Sprint("🚀 Starting Kubiya Agent Worker"),
		pterm.LightCyan("Queue:"),
		pterm.Bold.Sprint(opts.QueueID),
		pterm.LightCyan("Control Plane:"),
		pterm.Bold.Sprint(opts.getControlPlaneURL()),
	)

	pterm.DefaultBox.
		WithTitle("Worker Startup").
		WithTitleTopCenter().
		WithBoxStyle(pterm.NewStyle(pterm.FgCyan)).
		Println(headerContent)

	pm.Println("")

	// Check API key
	if opts.cfg.APIKey == "" {
		return fmt.Errorf("KUBIYA_API_KEY is required\nRun: kubiya login")
	}

	// Clean pip cache if requested
	if opts.NoCache {
		if err := cleanPipCache(pm); err != nil {
			pm.Warning(fmt.Sprintf("Cache cleanup failed: %v", err))
		}
	}

	// Check Python version silently
	pythonCmd, pythonVersion, err := checkPythonVersion()
	if err != nil {
		return err
	}

	// Check pip is available
	if err := checkPipAvailable(pythonCmd); err != nil {
		return err
	}

	// Step 1: Environment
	pterm.Print(pterm.LightCyan("[1/3] "))
	// Extract just the version number from "Python 3.11.9"
	versionOnly := strings.TrimPrefix(pythonVersion, "Python ")
	pterm.Success.Printf("Environment ready • Python %s\n", pterm.Bold.Sprint(versionOnly))

	// Create worker directory
	homeDir, err := os.UserHomeDir()
	if err != nil {
		return fmt.Errorf("failed to get home directory: %w", err)
	}

	workerDir := fmt.Sprintf("%s/.kubiya/workers/%s", homeDir, opts.QueueID)
	if err := os.MkdirAll(workerDir, 0755); err != nil {
		return fmt.Errorf("failed to create worker directory: %w", err)
	}

	// Create virtual environment if it doesn't exist
	venvDir := fmt.Sprintf("%s/venv", workerDir)
	if _, err := os.Stat(venvDir); os.IsNotExist(err) {
		spinner := pm.Spinner("Setting up Python environment")
		spinner.Start()

		venvCmd := exec.Command(pythonCmd, "-m", "venv", venvDir)
		if err := venvCmd.Run(); err != nil {
			spinner.Stop()
			return fmt.Errorf("failed to create virtual environment: %w", err)
		}
		spinner.Stop()
		pterm.Success.Println("Python environment ready")
	}

	// Determine pip path in venv (python path not needed for package binary)
	pipPath, _ := getVenvPaths(venvDir)

	// Check if pip upgrade is needed
	needsUpgrade, _, err := checkPipVersion(venvDir, "23.0")
	if err != nil {
		// Continue with upgrade to be safe
		needsUpgrade = true
	}

	if needsUpgrade {
		spinner := pm.Spinner("Upgrading pip")
		spinner.Start()

		pipUpgradeCmd := exec.Command(pipPath, "install", "--quiet", "--upgrade", "pip")
		if err := pipUpgradeCmd.Run(); err != nil {
			spinner.Stop()
			return fmt.Errorf("failed to upgrade pip: %w", err)
		}

		spinner.Stop()
		pterm.Success.Println("pip upgraded")
	}

	// Install worker package with extras
	var installCmd *exec.Cmd
	var packageSpec string
	var installSpinner *output.Spinner
	skipInstall := false

	if opts.LocalWheel != "" {
		// Install from local wheel (development mode) with [worker] extras
		if _, err := os.Stat(opts.LocalWheel); os.IsNotExist(err) {
			return fmt.Errorf("local wheel file not found: %s", opts.LocalWheel)
		}

		// For local wheels, always reinstall (development mode)
		packageSpec = opts.LocalWheel + "[worker]"

		// Show spinner during installation
		installSpinner = pm.Spinner("Installing dependencies from local wheel")
		installSpinner.Start()

		// Add timeout context for pip install (max 300 seconds = 5 minutes)
		installCtx, installCancel := context.WithTimeout(context.Background(), 300*time.Second)
		defer installCancel()
		installCmd = exec.CommandContext(installCtx, pipPath, "install", "--quiet", "--force-reinstall", packageSpec)
	} else {
		// Install from PyPI (production mode) with [worker] extras
		// Build package spec - handle both formats: ">=0.3.0" or "0.3.0"
		packageSpec = "kubiya-control-plane-api"
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

		// Check if package is already installed with correct version
		packageName, desiredVersion := parsePackageSpec(packageSpec)
		satisfied, installedVersion, err := checkPackageVersion(venvDir, packageName, desiredVersion)

		// Also check if worker binary exists and is executable
		workerBinary := fmt.Sprintf("%s/bin/kubiya-control-plane-worker", venvDir)
		binaryExists := false
		if _, err := os.Stat(workerBinary); err == nil {
			binaryExists = true
		}

		// Skip install only if version is satisfied AND binary exists
		if err == nil && satisfied && binaryExists {
			skipInstall = true
			pterm.Info.Printf("Using cached worker (v%s)\n", installedVersion)
		} else if !binaryExists {
			// Binary missing - force reinstall
			skipInstall = false
			pterm.Info.Println("Worker binary missing, reinstalling...")
		} else if !satisfied {
			// Version mismatch - reinstall
			skipInstall = false
			if installedVersion != "" {
				pterm.Info.Printf("Upgrading worker (v%s → %s)...\n", installedVersion, desiredVersion)
			}
		}

		if !skipInstall {
			// Add [worker] extras for all dependencies
			packageSpec += "[worker]"

			// Show spinner during installation
			installSpinner = pm.Spinner("Installing dependencies")
			installSpinner.Start()

			// Add timeout context for pip install (max 300 seconds = 5 minutes)
			installCtx, installCancel := context.WithTimeout(context.Background(), 300*time.Second)
			defer installCancel()
			installCmd = exec.CommandContext(installCtx, pipPath, "install", "--quiet", packageSpec)
		}
	}

	if !skipInstall {
		err := installCmd.Run()
		if installSpinner != nil {
			installSpinner.Stop()
		}

		if err != nil {
			// Check if error is due to timeout
			if err == context.DeadlineExceeded || strings.Contains(err.Error(), "context deadline exceeded") {
				return fmt.Errorf("dependency installation timed out after 5 minutes - this may indicate network issues or slow PyPI mirrors")
			}

			if opts.LocalWheel != "" {
				return fmt.Errorf("failed to install worker package from local wheel: %w", err)
			}
			return fmt.Errorf("failed to install worker package from PyPI: %w\nMake sure kubiya-control-plane-api is published", err)
		}

		// Step 2: Dependencies
		pterm.Print(pterm.LightCyan("[2/3] "))
		pterm.Success.Println("Dependencies installed")
	}

	// Verify worker binary is available
	workerBinary := fmt.Sprintf("%s/bin/kubiya-control-plane-worker", venvDir)
	if _, err := os.Stat(workerBinary); os.IsNotExist(err) {
		return fmt.Errorf("worker binary not found at %s\nPackage may not have installed correctly", workerBinary)
	}

	// Check if auto-update is enabled (flag or env var)
	autoUpdateEnabled := opts.AutoUpdate || IsAutoUpdateEnabled()

	// Parse update check interval
	updateCheckInterval, err := time.ParseDuration(opts.UpdateCheckInterval)
	if err != nil {
		updateCheckInterval = 5 * time.Minute
		progress.Warning("Invalid update-check-interval, using default: 5m")
	}

	// Initialize auto-update system if enabled
	var updateMonitor *UpdateMonitor
	var updateCoordinator *UpdateCoordinator
	if autoUpdateEnabled {
		// Create control plane client
		controlPlaneClient, err := controlplane.New(opts.cfg.APIKey, false)
		if err != nil {
			return fmt.Errorf("failed to create control plane client: %w", err)
		}

		// Generate worker ID (in production, this should be persistent)
		workerID := fmt.Sprintf("worker-%s", opts.QueueID)

		// Create update monitor
		updateMonitor = NewUpdateMonitor(
			opts.QueueID,
			workerID,
			controlPlaneClient,
			autoUpdateEnabled,
			updateCheckInterval,
			false, // debug
		)

		// Create update coordinator
		updateCoordinator = NewUpdateCoordinator(
			opts.QueueID,
			workerID,
			controlPlaneClient,
			workerDir,
			false, // debug
		)

		// Start update monitor
		updateMonitor.Start(ctx)
		defer updateMonitor.Stop()

		pterm.Info.Println("Auto-update enabled")
	}

	// Setup signal handling
	sigChan := make(chan os.Signal, 1)
	signal.Notify(sigChan, os.Interrupt, syscall.SIGTERM)

	// Start worker process
	startSpinner := pm.Spinner("Starting worker process")
	startSpinner.Start()

	// Prepare worker command
	controlPlaneURL := opts.getControlPlaneURL()
	workerCmd := exec.Command(
		workerBinary,
		"--queue-id", opts.QueueID,
		"--api-key", opts.cfg.APIKey,
		"--control-plane-url", controlPlaneURL,
	)

	// Set environment variables - take precedence over CLI args
	workerEnv := []string{
		fmt.Sprintf("QUEUE_ID=%s", opts.QueueID),
		fmt.Sprintf("KUBIYA_API_KEY=%s", opts.cfg.APIKey),
		fmt.Sprintf("CONTROL_PLANE_URL=%s", controlPlaneURL),
	}

	// Enable single execution mode for ephemeral queues
	if opts.SingleExecutionMode {
		workerEnv = append(workerEnv, "SINGLE_EXECUTION=true")
	}

	workerCmd.Env = append(os.Environ(), workerEnv...)

	// Connect stdout/stderr
	workerCmd.Stdout = os.Stdout
	workerCmd.Stderr = os.Stderr

	// Start the worker process
	if err := workerCmd.Start(); err != nil {
		startSpinner.Stop()
		return fmt.Errorf("failed to start worker: %w", err)
	}

	// Give worker a moment to initialize
	time.Sleep(500 * time.Millisecond)

	startSpinner.Stop()

	// Step 3: Worker launch
	pterm.Print(pterm.LightCyan("[3/3] "))
	pterm.Success.Println("Worker started")

	pm.Println("")

	// Create a styled success panel with visual elements
	successPanel := pterm.DefaultBox.
		WithTitle("✓ Ready").
		WithTitleTopCenter().
		WithBoxStyle(pterm.NewStyle(pterm.FgGreen)).
		Sprint(
			pterm.Sprintf(
				"%s\n\n"+
				"  %s  %s\n"+
				"  %s  %s\n\n"+
				"  %s",
				pterm.Bold.Sprintf("🎯 Worker is polling for tasks"),
				pterm.LightGreen("Status:"),
				pterm.Bold.Sprint("Active ●"),
				pterm.LightGreen("Queue: "),
				pterm.Bold.Sprint(opts.QueueID),
				pterm.FgGray.Sprint("Press Ctrl+C to stop gracefully"),
			),
		)

	pterm.Println(successPanel)
	pm.Println("")

	// Wait for worker completion in goroutine
	done := make(chan error, 1)
	go func() {
		if err := workerCmd.Wait(); err != nil {
			// Worker subprocess already printed its error to stderr
			// Just signal that it failed without wrapping
			done <- err
			return
		}
		done <- nil
	}()

	// Wait for signal, completion, or update trigger
	for {
		select {
		case <-sigChan:
			pm.Println("\nShutting down...")
			progress.Info("Gracefully stopping worker")

			// Terminate worker process if running
			if workerCmd != nil && workerCmd.Process != nil {
				workerCmd.Process.Signal(syscall.SIGTERM)
			}

			progress.Success("Worker stopped successfully")
			return nil

		case err := <-done:
			if err != nil {
				// Worker already printed its error to stderr
				// Show a clean context message and exit with proper code
				pm.Println("")
				pterm.Error.Println("Worker process stopped unexpectedly")
				pm.Println("")

				// Exit with worker's exit code (or 1 if unknown)
				if exitErr, ok := err.(*exec.ExitError); ok {
					os.Exit(exitErr.ExitCode())
				}
				os.Exit(1)
			}
			progress.Info("Worker process exited")
			return nil

		case trigger := <-func() <-chan UpdateTrigger {
			if updateMonitor != nil {
				return updateMonitor.UpdateChan()
			}
			// Return a channel that never sends if monitor is nil
			return make(chan UpdateTrigger)
		}():
			// Update triggered!
			fmt.Println()
			fmt.Println(strings.Repeat("═", 80))
			fmt.Println("🔄  UPDATE AVAILABLE")
			fmt.Println(strings.Repeat("═", 80))

			updateTypeStr := "Configuration"
			if trigger.Type == UpdateTypePackage {
				updateTypeStr = "Package Version"
				fmt.Printf("   Type:                %s\n", updateTypeStr)
				fmt.Printf("   New Version:         %s\n", trigger.NewPackageVersion)
			} else if trigger.Type == UpdateTypeBoth {
				updateTypeStr = "Configuration + Package"
				fmt.Printf("   Type:                %s\n", updateTypeStr)
				fmt.Printf("   New Config Version:  %s\n", trigger.NewConfigVersion[:8])
				fmt.Printf("   New Package Version: %s\n", trigger.NewPackageVersion)
			} else {
				fmt.Printf("   Type:                %s\n", updateTypeStr)
				fmt.Printf("   New Config Version:  %s\n", trigger.NewConfigVersion[:8])
			}

			fmt.Println()
			fmt.Println("   Coordinating rolling update with other workers...")
			fmt.Println(strings.Repeat("═", 80))

			// Coordinate the update
			if err := updateCoordinator.CoordinateUpdate(ctx, trigger); err != nil {
				fmt.Printf("⚠️  Update failed: %v\n", err)
				fmt.Println("   Worker will continue running with current version")
				fmt.Println()
				continue
			}

			// If we get here, the update was successful and the process will restart
			// The loop will exit when the process terminates
			fmt.Println("✓  Update completed successfully")
			fmt.Println("   Worker will restart with new version...")
			fmt.Println()
		}
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

// parsePackageSpec extracts package name and version from specs like:
// "package==1.2.3" -> ("package", "1.2.3")
// "package>=1.0.0" -> ("package", "1.0.0")
// "package" -> ("package", "")
// "./path/to/wheel.whl" -> ("kubiya-control-plane-api", "")
func parsePackageSpec(spec string) (string, string) {
	// Handle local wheel files
	if strings.HasSuffix(spec, ".whl") {
		return "kubiya-control-plane-api", ""
	}

	// Strip [worker] extras if present
	spec = strings.Split(spec, "[")[0]

	// Handle version specifiers (==, >=, ~=, >, <)
	for _, operator := range []string{"==", ">=", "~=", ">", "<"} {
		if strings.Contains(spec, operator) {
			parts := strings.Split(spec, operator)
			if len(parts) == 2 {
				return strings.TrimSpace(parts[0]), strings.TrimSpace(parts[1])
			}
		}
	}

	// No version specified
	return strings.TrimSpace(spec), ""
}

// checkPackageVersion checks if the correct version of the package is already installed
func checkPackageVersion(venvPath, packageName, desiredVersion string) (bool, string, error) {
	pipPath, _ := getVenvPaths(venvPath)

	cmd := exec.Command(pipPath, "show", packageName)
	output, err := cmd.CombinedOutput()
	if err != nil {
		// Package not installed
		return false, "", nil
	}

	// Parse output for Version: line
	lines := strings.Split(string(output), "\n")
	for _, line := range lines {
		if strings.HasPrefix(line, "Version:") {
			installedVersion := strings.TrimSpace(strings.TrimPrefix(line, "Version:"))

			// If desiredVersion is empty, any version is OK
			if desiredVersion == "" {
				return true, installedVersion, nil
			}

			// Compare versions
			if installedVersion == desiredVersion {
				return true, installedVersion, nil
			}

			return false, installedVersion, nil
		}
	}

	return false, "", nil
}

// compareVersions compares semantic versions (returns -1, 0, 1)
func compareVersions(v1, v2 string) int {
	parts1 := strings.Split(v1, ".")
	parts2 := strings.Split(v2, ".")

	for i := 0; i < len(parts1) && i < len(parts2); i++ {
		n1, _ := strconv.Atoi(parts1[i])
		n2, _ := strconv.Atoi(parts2[i])

		if n1 < n2 {
			return -1
		} else if n1 > n2 {
			return 1
		}
	}

	if len(parts1) < len(parts2) {
		return -1
	} else if len(parts1) > len(parts2) {
		return 1
	}

	return 0
}

// checkPipVersion checks if pip needs upgrading
func checkPipVersion(venvPath string, minVersion string) (bool, string, error) {
	pipPath, _ := getVenvPaths(venvPath)

	cmd := exec.Command(pipPath, "--version")
	output, err := cmd.CombinedOutput()
	if err != nil {
		return false, "", err
	}

	// Parse "pip X.Y.Z from ..."
	parts := strings.Fields(string(output))
	if len(parts) < 2 {
		return false, "", fmt.Errorf("unexpected pip --version output")
	}

	currentVersion := parts[1]

	// Compare versions
	if compareVersions(currentVersion, minVersion) >= 0 {
		return false, currentVersion, nil // No upgrade needed
	}

	return true, currentVersion, nil // Upgrade needed
}

// cleanPipCache removes pip cache directory
func cleanPipCache(pm *output.ProgressManager) error {
	spinner := pm.Spinner("Cleaning pip cache")
	spinner.Start()
	defer spinner.Stop()

	// Get cache directory using pip cache dir
	cmd := exec.Command("pip", "cache", "dir")
	output, err := cmd.CombinedOutput()
	if err != nil {
		// Try pip3 as fallback
		cmd = exec.Command("pip3", "cache", "dir")
		output, err = cmd.CombinedOutput()
		if err != nil {
			return fmt.Errorf("failed to get cache directory: %w", err)
		}
	}

	cacheDir := strings.TrimSpace(string(output))
	if cacheDir == "" {
		return fmt.Errorf("cache directory path is empty")
	}

	// Remove cache directory
	if err := os.RemoveAll(cacheDir); err != nil {
		return fmt.Errorf("failed to remove cache: %w", err)
	}

	pm.Success(fmt.Sprintf("Cleared pip cache at %s", cacheDir))
	return nil
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
		fmt.Sprintf("CONTROL_PLANE_URL=%s", opts.getControlPlaneURL()),
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
		fmt.Printf("   Control Plane:    %s\n", opts.getControlPlaneURL())
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

	venvExists := true
	if _, err := os.Stat(venvDir); os.IsNotExist(err) {
		venvExists = false
		venvCmd := exec.Command(pythonCmd, "-m", "venv", venvDir)
		if err := venvCmd.Run(); err != nil {
			return fmt.Errorf("failed to create virtual environment: %w", err)
		}
	}

	// Check if pip upgrade is needed (whether venv is new or existing)
	needsPipUpgrade := false
	if !venvExists {
		// New venv always needs pip upgrade
		needsPipUpgrade = true
	} else {
		// Check pip version for existing venv
		needs, _, err := checkPipVersion(venvDir, "23.0")
		if err != nil {
			// If we can't check, upgrade to be safe
			needsPipUpgrade = true
		} else {
			needsPipUpgrade = needs
		}
	}

	if needsPipUpgrade {
		pipUpgradeCmd := exec.Command(pipPath, "install", "--quiet", "--upgrade", "pip")
		if err := pipUpgradeCmd.Run(); err != nil {
			return fmt.Errorf("failed to upgrade pip: %w", err)
		}
	}

	// Check if package installation is needed
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

	needsPackageInstall := false
	if !venvExists {
		// New venv always needs package install
		needsPackageInstall = true
	} else {
		// Check if package version is satisfied for existing venv
		packageName, desiredVersion := parsePackageSpec(packageSpec)
		satisfied, _, err := checkPackageVersion(venvDir, packageName, desiredVersion)
		if err != nil || !satisfied {
			needsPackageInstall = true
		}
	}

	if needsPackageInstall {
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

	// Signal readiness to parent process
	socketPath := os.Getenv("KUBIYA_READINESS_SOCKET")
	if socketPath != "" {
		readinessServer := NewReadinessServer(socketPath)
		readinessInfo := DaemonReadinessInfo{
			PID:             daemonInfo.PID,
			QueueID:         daemonInfo.QueueID,
			ControlPlaneURL: opts.getControlPlaneURL(),
			WorkerDir:       daemonInfo.WorkerDir,
			StartTime:       daemonInfo.StartedAt,
		}
		if err := readinessServer.SignalReady(readinessInfo); err != nil {
			// Log but don't fail - parent may have timed out
			fmt.Fprintf(os.Stderr, "Warning: failed to signal readiness to parent: %v\n", err)
		}
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

