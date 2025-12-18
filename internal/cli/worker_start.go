package cli

import (
	"context"
	_ "embed"
	"fmt"
	"os"
	"os/exec"
	"os/signal"
	"path/filepath"
	"runtime"
	"strconv"
	"strings"
	"time"

	"github.com/docker/docker/api/types/container"
	"github.com/docker/docker/api/types/image"
	"github.com/docker/docker/client"
	"github.com/docker/docker/pkg/stdcopy"
	"github.com/kubiyabot/cli/internal/config"
	kubiyacontext "github.com/kubiyabot/cli/internal/context"
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
	PackageSource  string // Flexible package source: version, file path, git URL, or GitHub ref
	NoCache        bool   // Clear pip cache before installation
	ForceUpdate    bool   // Force update to latest version from PyPI, bypassing cache

	// Auto-update options
	AutoUpdate           bool   // Enable automatic updates
	UpdateCheckInterval  string // Interval for checking updates (e.g., "5m", "10m")

	// Local LiteLLM Proxy options
	EnableLocalProxy     bool   // Enable local LiteLLM proxy
	ProxyConfigFile      string // Path to LiteLLM proxy config file (JSON or YAML)
	ProxyConfigJSON      string // Inline LiteLLM proxy config (JSON string)

	cfg             *config.Config
	progressManager *output.ProgressManager
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
  KUBIYA_WORKER_PACKAGE_SOURCE   - Flexible package source specification
  KUBIYA_WORKER_LOCAL_WHEEL      - Path to local wheel file for development
  KUBIYA_ENABLE_LOCAL_PROXY      - Enable local LiteLLM proxy ("true" or "1")
  KUBIYA_PROXY_CONFIG_FILE       - Path to LiteLLM proxy config file (JSON or YAML)
  KUBIYA_PROXY_CONFIG_JSON       - Inline LiteLLM proxy config (JSON string)

Local LiteLLM Proxy:
  Override the control plane's LLM gateway with a local LiteLLM proxy for custom model routing,
  observability (Langfuse), and cost optimization. The proxy runs alongside the worker process.

  Priority (highest to lowest):
    1. CLI flags: --enable-local-proxy with --proxy-config-file or --proxy-config-json
    2. Environment variables: KUBIYA_ENABLE_LOCAL_PROXY with KUBIYA_PROXY_CONFIG_FILE or KUBIYA_PROXY_CONFIG_JSON
    3. Context configuration: ~/.kubiya/config (litellm-proxy section)
    4. Control plane queue settings (configured via UI or API)
    5. Control plane LLM gateway (default)

Examples:
  # Start worker locally (no Docker)
  kubiya worker start --queue-id=<queue-id> --type=local

  # Enable local LiteLLM proxy with config file (CLI flag)
  kubiya worker start --queue-id=<queue-id> --enable-local-proxy --proxy-config-file=./litellm_config.json

  # Enable local LiteLLM proxy with inline JSON (CLI flag)
  kubiya worker start --queue-id=<queue-id> --enable-local-proxy --proxy-config-json='{"model_list":[{"model_name":"gpt-4","litellm_params":{"model":"azure/gpt-4","api_key":"env:AZURE_API_KEY"}}]}'

  # Enable local LiteLLM proxy with environment variables
  export KUBIYA_ENABLE_LOCAL_PROXY=true
  export KUBIYA_PROXY_CONFIG_FILE=./litellm_config.json
  kubiya worker start --queue-id=<queue-id>

  # Or with inline JSON via env var
  export KUBIYA_ENABLE_LOCAL_PROXY=1
  export KUBIYA_PROXY_CONFIG_JSON='{"model_list":[{"model_name":"gpt-4","litellm_params":{"model":"azure/gpt-4"}}]}'
  kubiya worker start --queue-id=<queue-id>

  # Configure via context (persisted in ~/.kubiya/config)
  # Edit your context to include:
  #   litellm-proxy:
  #     enabled: true
  #     config-file: /path/to/litellm_config.json
  # Then all workers will automatically use this configuration
  kubiya worker start --queue-id=<queue-id>

  # Specific PyPI version
  kubiya worker start --queue-id=<queue-id> --package-source=0.5.0
  kubiya worker start --queue-id=<queue-id> --package-source=">=0.3.0"

  # Git commit SHA
  kubiya worker start --queue-id=<queue-id> --package-source=git+https://github.com/kubiyabot/control-plane-api.git@abc1234

  # Git branch
  kubiya worker start --queue-id=<queue-id> --package-source=git+https://github.com/kubiyabot/control-plane-api.git@main

  # Git tag
  kubiya worker start --queue-id=<queue-id> --package-source=git+https://github.com/kubiyabot/control-plane-api.git@v0.5.0

  # GitHub shorthand (owner/repo@ref)
  kubiya worker start --queue-id=<queue-id> --package-source=kubiyabot/control-plane-api@abc1234
  kubiya worker start --queue-id=<queue-id> --package-source=kubiyabot/control-plane-api@feature-branch

  # Local wheel file
  kubiya worker start --queue-id=<queue-id> --package-source=/path/to/wheel.whl

  # Start worker in daemon mode
  kubiya worker start --queue-id=<queue-id> --type=local -d

  # Start worker in Docker
  kubiya worker start --queue-id=<queue-id> --type=docker --image-tag=v1.2.3

  # Daemon mode with local LiteLLM proxy
  kubiya worker start --queue-id=<queue-id> --type=local -d --enable-local-proxy --proxy-config-file=./litellm_config.json`,
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
	cmd.Flags().StringVar(&opts.PackageSource, "package-source", "", "Flexible package source: PyPI version, local file, git URL (git+https://github.com/org/repo.git@ref), or GitHub shorthand (owner/repo@ref) (env: KUBIYA_WORKER_PACKAGE_SOURCE)")
	cmd.Flags().BoolVar(&opts.NoCache, "no-cache", false, "Clear pip cache before installation (useful for troubleshooting)")
	cmd.Flags().BoolVar(&opts.ForceUpdate, "force-update", false, "Force update to latest version from PyPI, bypassing cache")
	cmd.Flags().BoolVar(&opts.AutoUpdate, "auto-update", false, "Enable automatic worker updates (config + package)")
	cmd.Flags().StringVar(&opts.UpdateCheckInterval, "update-check-interval", "5m", "Interval for checking updates (e.g., 5m, 10m, 1h)")

	// Local LiteLLM Proxy flags
	cmd.Flags().BoolVar(&opts.EnableLocalProxy, "enable-local-proxy", false, "Enable local LiteLLM proxy alongside worker (overrides control plane config)")
	cmd.Flags().StringVar(&opts.ProxyConfigFile, "proxy-config-file", "", "Path to LiteLLM proxy config file (JSON or YAML)")
	cmd.Flags().StringVar(&opts.ProxyConfigJSON, "proxy-config-json", "", "Inline LiteLLM proxy config as JSON string")

	cmd.MarkFlagRequired("queue-id")

	return cmd
}

func (opts *WorkerStartOptions) Run(ctx context.Context) error {
	// Apply environment variables if flags not set
	// Priority: --package-source > env KUBIYA_WORKER_PACKAGE_SOURCE > --package-version/--local-wheel
	if opts.PackageSource == "" {
		if envSource := os.Getenv("KUBIYA_WORKER_PACKAGE_SOURCE"); envSource != "" {
			opts.PackageSource = envSource
		}
	}

	// Fallback to old flags for backward compatibility
	if opts.PackageSource == "" {
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
	}

	// Apply LiteLLM proxy configuration with priority:
	// 1. CLI flags (already set)
	// 2. Environment variables
	// 3. Context configuration

	// First, try to load from context if CLI flags not set
	if !opts.EnableLocalProxy && opts.ProxyConfigFile == "" && opts.ProxyConfigJSON == "" {
		if kubiyaCtx, _, err := kubiyacontext.GetCurrentContext(); err == nil && kubiyaCtx.LiteLLMProxy != nil {
			if kubiyaCtx.LiteLLMProxy.Enabled {
				opts.EnableLocalProxy = true
				if kubiyaCtx.LiteLLMProxy.ConfigFile != "" {
					opts.ProxyConfigFile = kubiyaCtx.LiteLLMProxy.ConfigFile
				}
				if kubiyaCtx.LiteLLMProxy.ConfigJSON != "" {
					opts.ProxyConfigJSON = kubiyaCtx.LiteLLMProxy.ConfigJSON
				}
			}
		}
	}

	// Then apply environment variables if still not set
	if !opts.EnableLocalProxy {
		if envEnable := os.Getenv("KUBIYA_ENABLE_LOCAL_PROXY"); envEnable == "true" || envEnable == "1" {
			opts.EnableLocalProxy = true
		}
	}
	if opts.ProxyConfigFile == "" {
		if envFile := os.Getenv("KUBIYA_PROXY_CONFIG_FILE"); envFile != "" {
			opts.ProxyConfigFile = envFile
		}
	}
	if opts.ProxyConfigJSON == "" {
		if envJSON := os.Getenv("KUBIYA_PROXY_CONFIG_JSON"); envJSON != "" {
			opts.ProxyConfigJSON = envJSON
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
	opts.progressManager = pm
	ptermManager := pm.PTermManager()
	logger := ptermManager.Logger()
	progress := NewWorkerStartProgress(pm)

	// Show styled header with PTerm box
	pm.Println("")

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

	ptermManager.Box().
		WithTitle("Worker Startup").
		WithTitleTopCenter().
		Println(headerContent)

	pm.Println("")

	// Check API key
	if opts.cfg.APIKey == "" {
		return fmt.Errorf("KUBIYA_API_KEY is required\nRun: kubiya login")
	}

	logger.Debug("Worker start initiated", "queue_id", opts.QueueID)

	// Clean package manager cache if requested
	if opts.NoCache {
		pkgMgr := NewPackageManager()
		logger.Debug("Cleaning package cache", "manager", pkgMgr.Name())
		if err := pkgMgr.ClearCache(); err != nil {
			pm.Warning(fmt.Sprintf("Cache cleanup failed: %v", err))
		} else {
			pm.Success(fmt.Sprintf("%s cache cleared", pkgMgr.Name()))
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
	logger.Debug("Environment validated", "python", versionOnly)

	// Create worker directory
	homeDir, err := os.UserHomeDir()
	if err != nil {
		return fmt.Errorf("failed to get home directory: %w", err)
	}

	workerDir := fmt.Sprintf("%s/.kubiya/workers/%s", homeDir, opts.QueueID)
	if err := os.MkdirAll(workerDir, 0755); err != nil {
		return fmt.Errorf("failed to create worker directory: %w", err)
	}
	logger.Debug("Worker directory created", "path", workerDir)

	// Create virtual environment if it doesn't exist
	venvDir := fmt.Sprintf("%s/venv", workerDir)
	if _, err := os.Stat(venvDir); os.IsNotExist(err) {
		logger.Info("Creating virtual environment", "path", venvDir)
		spinner := NewPTermSpinner(ptermManager, "Setting up Python environment")

		venvCmd := exec.Command(pythonCmd, "-m", "venv", venvDir)
		if err := venvCmd.Run(); err != nil {
			spinner.Fail("Failed to create virtual environment")
			return fmt.Errorf("failed to create virtual environment: %w", err)
		}
		spinner.Success("Python environment ready")
		logger.Debug("Virtual environment created successfully")
	} else {
		logger.Debug("Using existing virtual environment", "path", venvDir)
	}

	// Initialize package manager
	pkgMgr := NewPackageManager()
	logger.Info("Package manager detected", "manager", pkgMgr.Name())

	// Only upgrade pip if using pip (uv doesn't need pip upgrade)
	if _, ok := pkgMgr.(*PipPackageManager); ok {
		// Check if pip upgrade is needed
		needsUpgrade, _, err := checkPipVersion(venvDir, "23.0")
		if err != nil {
			// Continue with upgrade to be safe
			needsUpgrade = true
		}

		if needsUpgrade {
			spinner := NewPTermSpinner(ptermManager, "Upgrading pip")

			if err := UpgradePip(venvDir, true); err != nil {
				spinner.Fail("Failed to upgrade pip")
				return fmt.Errorf("failed to upgrade pip: %w", err)
			}

			spinner.Success("pip upgraded")
		}
	} else {
		logger.Debug("Using uv, skipping pip upgrade")
	}

	// Install worker package with extras
	var packageSpec string
	var installSpinner *PTermSpinner
	var sourceDisplay string
	skipInstall := false
	forceReinstall := false

	// Determine package source (priority: --package-source > --local-wheel > --package-version)
	var sourceInfo *PackageSourceInfo
	if opts.PackageSource != "" {
		// Use new flexible package source
		parsedSource, err := parsePackageSource(opts.PackageSource)
		if err != nil {
			return fmt.Errorf("failed to parse package source: %w", err)
		}
		sourceInfo = parsedSource
		packageSpec = parsedSource.Spec
		sourceDisplay = parsedSource.Display
		logger.Debug("Using package source", "source", sourceDisplay)

		// For git sources and local files, always force reinstall
		if parsedSource.Type == PackageSourceGitURL ||
			parsedSource.Type == PackageSourceGitHubShorthand ||
			parsedSource.Type == PackageSourceLocalFile {
			forceReinstall = true
		}
	} else if opts.LocalWheel != "" {
		// Backward compatibility: --local-wheel
		if _, err := os.Stat(opts.LocalWheel); os.IsNotExist(err) {
			return fmt.Errorf("local wheel file not found: %s", opts.LocalWheel)
		}
		packageSpec = opts.LocalWheel + "[worker]"
		sourceDisplay = fmt.Sprintf("local file: %s", opts.LocalWheel)
		forceReinstall = true
	} else if opts.PackageVersion != "" {
		// Backward compatibility: --package-version
		baseSpec := "kubiya-control-plane-api"
		if !strings.HasPrefix(opts.PackageVersion, ">=") &&
			!strings.HasPrefix(opts.PackageVersion, "==") &&
			!strings.HasPrefix(opts.PackageVersion, "~=") &&
			!strings.HasPrefix(opts.PackageVersion, "<") &&
			!strings.HasPrefix(opts.PackageVersion, ">") {
			baseSpec += "==" + opts.PackageVersion
		} else {
			baseSpec += opts.PackageVersion
		}
		packageSpec = baseSpec + "[worker]"
		sourceDisplay = fmt.Sprintf("PyPI: %s", opts.PackageVersion)
	} else {
		// Default: latest from PyPI
		packageSpec = "kubiya-control-plane-api[worker]"
		sourceDisplay = "PyPI: latest"
	}

	// Handle --force-update flag
	if opts.ForceUpdate {
		logger.Info("Force update requested, fetching latest version from PyPI...")
		latestVersion, err := GetLatestPackageVersion("kubiya-control-plane-api")
		if err != nil {
			return fmt.Errorf("failed to fetch latest version from PyPI: %w", err)
		}
		packageSpec = fmt.Sprintf("kubiya-control-plane-api[worker]==%s", latestVersion)
		sourceDisplay = fmt.Sprintf("PyPI: %s (latest)", latestVersion)
		forceReinstall = true
		logger.Info("Latest version found", "version", latestVersion)
	}

	// Check if we can skip installation (only for PyPI sources)
	workerBinary := fmt.Sprintf("%s/bin/kubiya-control-plane-worker", venvDir)
	if !forceReinstall && (sourceInfo == nil || sourceInfo.Type == PackageSourcePyPI) {
		// Check if package is already installed with correct version
		packageName, desiredVersion := parsePackageSpec(packageSpec)

		// Use enhanced version checking with optional PyPI check
		versionInfo, err := checkPackageVersionEx(venvDir, packageName, desiredVersion, true)

		// Also check if worker binary exists
		binaryExists := false
		if _, err := os.Stat(workerBinary); err == nil {
			binaryExists = true
		}

		// Skip install only if version is satisfied AND binary exists
		if err == nil && versionInfo.SatisfiesRequest && binaryExists {
			skipInstall = true

			// Check if cached version is outdated compared to PyPI
			if versionInfo.IsOutdated && versionInfo.Latest != "" {
				pterm.Warning.Printf("⚠️  Cached worker v%s is available, but v%s is available on PyPI\n",
					versionInfo.Installed, versionInfo.Latest)
				pterm.Info.Println("    Run with --force-update to install the latest version")
			} else if versionInfo.Latest != "" && !versionInfo.IsOutdated {
				pterm.Info.Printf("✓ Using cached worker (v%s, latest)\n", versionInfo.Installed)
			} else {
				// PyPI check failed or returned no data, use simple message
				pterm.Info.Printf("Using cached worker (v%s)\n", versionInfo.Installed)
			}
			logger.Debug("Skipping installation, using cached version", "version", versionInfo.Installed)
		} else if !binaryExists {
			pterm.Info.Println("Worker binary missing, reinstalling...")
		} else if !versionInfo.SatisfiesRequest {
			if versionInfo.Installed != "" && desiredVersion != "" {
				pterm.Info.Printf("Upgrading worker (v%s → %s)...\n", versionInfo.Installed, desiredVersion)
			}
		}
	}

	if !skipInstall {
		// Add timeout context for package install (max 300 seconds = 5 minutes)
		installCtx, installCancel := context.WithTimeout(context.Background(), 300*time.Second)
		defer installCancel()

		// Show spinner with package manager info
		installSpinner = NewPTermSpinner(ptermManager, fmt.Sprintf("Installing dependencies (%s)", pkgMgr.Name()))
		logger.Debug("Installing worker package", "package", packageSpec, "manager", pkgMgr.Name())

		err := pkgMgr.Install(installCtx, venvDir, packageSpec, true, forceReinstall)

		if installSpinner != nil {
			if err != nil {
				installSpinner.Fail("Failed to install dependencies")
			} else {
				installSpinner.Success("Dependencies installed")
			}
		}

		if err != nil {
			// Check if error is due to timeout
			if err == context.DeadlineExceeded || strings.Contains(err.Error(), "context deadline exceeded") {
				return fmt.Errorf("dependency installation timed out after 5 minutes - this may indicate network issues or slow PyPI mirrors")
			}

			if opts.LocalWheel != "" {
				return fmt.Errorf("failed to install worker package from local wheel: %w", err)
			}
			return fmt.Errorf("failed to install worker package: %w\nMake sure kubiya-control-plane-api is available", err)
		}

		// Step 2: Dependencies
		pterm.Print(pterm.LightCyan("[2/3] "))
		pterm.Success.Println("Dependencies installed")
		logger.Debug("Dependencies installed successfully", "package", packageSpec)
	}

	// Verify worker binary is available (workerBinary already declared above)
	if _, err := os.Stat(workerBinary); os.IsNotExist(err) {
		return fmt.Errorf("worker binary not found at %s\nPackage may not have installed correctly", workerBinary)
	}

	// Install litellm[proxy] if local proxy is enabled
	if opts.EnableLocalProxy || (os.Getenv("KUBIYA_ENABLE_LOCAL_PROXY") != "" && os.Getenv("KUBIYA_ENABLE_LOCAL_PROXY") != "false") {
		liteLLMSpinner := NewPTermSpinner(ptermManager, "Installing litellm proxy dependencies")
		logger.Info("Local proxy enabled, installing litellm[proxy]")

		liteLLMCtx, liteLLMCancel := context.WithTimeout(context.Background(), 120*time.Second)
		defer liteLLMCancel()

		err := pkgMgr.Install(liteLLMCtx, venvDir, "litellm[proxy]", false, false)
		if err != nil {
			if liteLLMSpinner != nil {
				liteLLMSpinner.Fail("Failed to install litellm proxy dependencies")
			}
			return fmt.Errorf("failed to install litellm[proxy]: %w\nLocal proxy requires litellm with proxy extras", err)
		}

		if liteLLMSpinner != nil {
			liteLLMSpinner.Success("LiteLLM proxy dependencies installed")
		}
		logger.Debug("LiteLLM proxy dependencies installed successfully")
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

	// Setup local LiteLLM proxy if enabled
	var liteLLMSupervisor *LiteLLMProxySupervisor
	var liteLLMProxyURL string

	// Priority 1: Check CLI flags first
	if opts.EnableLocalProxy {
		logger.Info("Local LiteLLM proxy enabled via CLI flag")
		pterm.Info.Println("Setting up local LLM proxy gateway (CLI config)...")

		// Load config from file or inline JSON
		var proxyConfig *LiteLLMProxyConfig
		var err error

		if opts.ProxyConfigFile != "" {
			// Load from file
			proxyConfig, err = LoadLiteLLMConfigFromFile(opts.ProxyConfigFile)
			if err != nil {
				pm.Warning(fmt.Sprintf("Failed to load proxy config from file: %v", err))
				pm.Warning("Worker will use control plane proxy as fallback")
			}
		} else if opts.ProxyConfigJSON != "" {
			// Parse inline JSON
			proxyConfig, err = ParseLiteLLMConfigFromJSON(opts.ProxyConfigJSON)
			if err != nil {
				pm.Warning(fmt.Sprintf("Failed to parse inline proxy config: %v", err))
				pm.Warning("Worker will use control plane proxy as fallback")
			}
		} else {
			pm.Warning("--enable-local-proxy requires either --proxy-config-file or --proxy-config-json")
			pm.Warning("Worker will use control plane proxy as fallback")
		}

		if proxyConfig != nil {
			// Create supervisor with CLI config
			liteLLMSupervisor, err = NewLiteLLMProxySupervisor(
				opts.QueueID,
				workerDir,
				proxyConfig,
				30, // timeout seconds (LiteLLM can take 15-20s to start)
				3,  // default retries
			)
			if err != nil {
				pm.Warning(fmt.Sprintf("Failed to create LiteLLM proxy supervisor: %v", err))
				pm.Warning("Worker will use control plane proxy as fallback")
			} else {
				// Start proxy
				proxyInfo, err := liteLLMSupervisor.Start(ctx)
				if err != nil {
					logPath := filepath.Join(workerDir, "litellm_proxy.log")
					pm.Error(fmt.Sprintf("Failed to start LiteLLM proxy: %v", err))
					pm.Error(fmt.Sprintf("Check logs at: %s", logPath))
					return fmt.Errorf("local LiteLLM proxy failed to start (required with --enable-local-proxy flag)")
				}

				logger.Info("LiteLLM proxy started from CLI config", "pid", proxyInfo.PID, "port", proxyInfo.Port)
				pterm.Info.Printfln("LiteLLM Proxy started (PID: %d, Port: %d)", proxyInfo.PID, proxyInfo.Port)
				pterm.Info.Printfln("Proxy logs: %s", filepath.Join(workerDir, "litellm_proxy.log"))

				// Wait for proxy health check
				if err := liteLLMSupervisor.WaitReady(ctx); err != nil {
					logPath := filepath.Join(workerDir, "litellm_proxy.log")
					pm.Error(fmt.Sprintf("LiteLLM proxy failed health check: %v", err))
					pm.Error(fmt.Sprintf("Check logs at: %s", logPath))
					liteLLMSupervisor.Stop()
					return fmt.Errorf("local LiteLLM proxy health check failed (PID: %d)", proxyInfo.PID)
				}

				liteLLMProxyURL = proxyInfo.BaseURL
				pterm.Success.Printfln("✓ Local LLM proxy ready at %s", liteLLMProxyURL)
			}
		}
	}

	// Priority 2: Check control plane config if CLI flags not provided
	if !opts.EnableLocalProxy && liteLLMSupervisor == nil {
		// Fetch worker queue config for local proxy settings
		controlPlaneClient, err := controlplane.New(opts.cfg.APIKey, false)
		if err != nil {
			logger.Debug("Failed to create control plane client for config fetch", "error", err)
		} else {
			queueConfig, err := controlPlaneClient.GetWorkerQueueConfig(opts.QueueID)
			if err != nil {
				logger.Debug("Failed to fetch worker queue config", "error", err)
			} else if queueConfig.Settings != nil && IsLocalProxyEnabled(queueConfig.Settings) {
				// Local LiteLLM proxy is enabled
				logger.Info("Local LiteLLM proxy enabled in queue config")
				pterm.Info.Println("Setting up local LLM proxy gateway (control plane config)...")

				// Parse config
				proxyConfig, err := ParseLiteLLMConfigFromSettings(queueConfig.Settings)
				if err != nil {
					pm.Warning(fmt.Sprintf("Failed to parse LiteLLM config: %v", err))
					pm.Warning("Worker will use control plane proxy as fallback")
				} else {
					// Get timeout settings
					timeoutSeconds, maxRetries := GetProxyTimeoutSettings(queueConfig.Settings)

					// Create supervisor
					liteLLMSupervisor, err = NewLiteLLMProxySupervisor(
						opts.QueueID,
						workerDir,
						proxyConfig,
						timeoutSeconds,
						maxRetries,
					)
					if err != nil {
						pm.Warning(fmt.Sprintf("Failed to create LiteLLM proxy supervisor: %v", err))
						pm.Warning("Worker will use control plane proxy as fallback")
					} else {
						// Start proxy
						proxyInfo, err := liteLLMSupervisor.Start(ctx)
						if err != nil {
							logPath := filepath.Join(workerDir, "litellm_proxy.log")
							pm.Warning(fmt.Sprintf("Failed to start LiteLLM proxy: %v", err))
							pm.Warning(fmt.Sprintf("Check logs at: %s", logPath))
							pm.Warning("Worker will use control plane proxy as fallback")
							liteLLMSupervisor = nil
						} else {
							logger.Info("LiteLLM proxy started", "pid", proxyInfo.PID, "port", proxyInfo.Port)
							pterm.Info.Printfln("LiteLLM Proxy started (PID: %d, Port: %d)", proxyInfo.PID, proxyInfo.Port)
							pterm.Info.Printfln("Proxy logs: %s", filepath.Join(workerDir, "litellm_proxy.log"))

							// Wait for proxy to be ready
							spinner := NewPTermSpinner(ptermManager, "Waiting for local LLM proxy")
							err := liteLLMSupervisor.WaitReady(ctx)
							if err != nil {
								spinner.Fail("Local LLM proxy failed to start")
								logPath := filepath.Join(workerDir, "litellm_proxy.log")
								pm.Warning(fmt.Sprintf("LiteLLM proxy health check failed: %v", err))
								pm.Warning(fmt.Sprintf("Check logs at: %s", logPath))
								pm.Warning("Worker will use control plane proxy as fallback")
								liteLLMSupervisor.Stop()
								liteLLMSupervisor = nil
							} else {
								spinner.Success("Local LLM proxy ready")
								liteLLMProxyURL = proxyInfo.BaseURL
								logger.Info("LiteLLM proxy ready", "url", liteLLMProxyURL)
								pterm.Success.Printfln("✓ Local LLM proxy ready at %s", liteLLMProxyURL)
							}
						}
					}
				}
			}
		}
	}

	// Setup signal handling
	sigChan := make(chan os.Signal, 1)
	signal.Notify(sigChan, getShutdownSignals()...)

	// Start worker process
	startSpinner := NewPTermSpinner(ptermManager, "Starting worker process")

	// Prepare worker command
	controlPlaneURL := opts.getControlPlaneURL()
	logger.Debug("Preparing worker command", "binary", workerBinary, "queue_id", opts.QueueID)

	workerCmd := exec.Command(
		workerBinary,
		"--queue-id", opts.QueueID,
		"--api-key", opts.cfg.APIKey,
		"--control-plane-url", controlPlaneURL,
	)

	// Set process group so we can kill all child processes
	// This is critical for Python processes that may spawn subprocesses
	setupProcessGroup(workerCmd)

	// Set environment variables - take precedence over CLI args
	workerEnv := []string{
		fmt.Sprintf("QUEUE_ID=%s", opts.QueueID),
		fmt.Sprintf("KUBIYA_API_KEY=%s", opts.cfg.APIKey),
		fmt.Sprintf("CONTROL_PLANE_URL=%s", controlPlaneURL),
	}

	// Enable single execution mode for ephemeral queues
	if opts.SingleExecutionMode {
		workerEnv = append(workerEnv, "SINGLE_EXECUTION=true")
		logger.Debug("Single execution mode enabled")
	}

	// If local LiteLLM proxy is running, inject proxy URL
	if liteLLMProxyURL != "" {
		workerEnv = append(workerEnv,
			fmt.Sprintf("LITELLM_API_BASE=%s", liteLLMProxyURL),
			"LITELLM_API_KEY=dummy-key",
		)
		logger.Debug("Injected local LiteLLM proxy env vars", "base_url", liteLLMProxyURL)
	}

	workerCmd.Env = append(os.Environ(), workerEnv...)

	// Connect stdout/stderr
	workerCmd.Stdout = os.Stdout
	workerCmd.Stderr = os.Stderr

	// Start the worker process
	if err := workerCmd.Start(); err != nil {
		startSpinner.Fail("Failed to start worker")
		return fmt.Errorf("failed to start worker: %w", err)
	}

	// Give worker a moment to initialize
	time.Sleep(500 * time.Millisecond)

	// Step 3: Worker launch
	pterm.Print(pterm.LightCyan("[3/3] "))
	startSpinner.Success("Worker started")
	logger.Info("Worker process started successfully", "pid", workerCmd.Process.Pid)

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

	// Wait for signal, completion, update trigger, or context cancellation
	for {
		select {
		case <-ctx.Done():
			// Context cancelled (e.g., by parent process)
			pm.Println("\nContext cancelled, shutting down...")
			progress.Info("Gracefully stopping worker")
			pm.Println("   (Press Ctrl+C to force shutdown)")

			// Stop LiteLLM proxy first
			if liteLLMSupervisor != nil {
				logger.Debug("Stopping local LiteLLM proxy")
				if err := liteLLMSupervisor.Stop(); err != nil {
					logger.Debug("Failed to stop LiteLLM proxy", "error", err)
				}
			}

			// Terminate worker process if running (same cleanup as SIGINT)
			if workerCmd != nil && workerCmd.Process != nil {
				// Send SIGTERM to entire process group
				pgid := workerCmd.Process.Pid
				if err := killProcessGroup(pgid, true); err != nil {
					// Fallback to single process if process group kill fails
					pm.Warning(fmt.Sprintf("Failed to terminate process group: %v", err))
					if err := workerCmd.Process.Kill(); err != nil {
						pm.Warning(fmt.Sprintf("Failed to terminate process: %v", err))
					}
				}

				// Wait for process to exit gracefully (with shorter timeout for context cancellation)
				shutdownTimer := time.NewTimer(10 * time.Second)

				// Create a goroutine to listen for interrupt during context cancellation
				forceShutdown := make(chan struct{}, 1)
				go func() {
					<-sigChan
					pm.Println("\n\n⚠️  Force shutdown requested...")
					close(forceShutdown)
				}()

				select {
				case err := <-done:
					shutdownTimer.Stop()
					if err != nil {
						progress.Info("Worker process stopped")
					} else {
						progress.Success("Worker stopped successfully")
					}
				case <-forceShutdown:
					// User pressed Ctrl+C - force kill immediately
					shutdownTimer.Stop()
					pm.Warning("Force killing worker process...")
					if err := killProcessGroup(pgid, false); err != nil {
						if err := workerCmd.Process.Kill(); err != nil {
							pm.Warning(fmt.Sprintf("Failed to kill process: %v", err))
						}
					}
					time.Sleep(200 * time.Millisecond)
					select {
					case <-done:
					default:
					}
					progress.Info("Worker process force terminated")
				case <-shutdownTimer.C:
					// Timeout - force kill
					pm.Warning("Worker did not stop gracefully, forcing shutdown...")
					if err := killProcessGroup(pgid, false); err != nil {
						if err := workerCmd.Process.Kill(); err != nil {
							pm.Warning(fmt.Sprintf("Failed to kill process: %v", err))
						}
					}
					time.Sleep(500 * time.Millisecond)
					select {
					case <-done:
					default:
					}
					progress.Info("Worker process terminated")
				}
			}
			return ctx.Err()

		case <-sigChan:
			pm.Println("\nShutting down...")
			progress.Info("Gracefully stopping worker")
			pm.Println("   (Press Ctrl+C again to force shutdown)")

			// Stop LiteLLM proxy first
			if liteLLMSupervisor != nil {
				logger.Debug("Stopping local LiteLLM proxy")
				if err := liteLLMSupervisor.Stop(); err != nil {
					logger.Debug("Failed to stop LiteLLM proxy", "error", err)
				}
			}

			// Terminate worker process if running
			if workerCmd != nil && workerCmd.Process != nil {
				// Send SIGTERM to entire process group (negative PID)
				// This ensures all child processes are also terminated
				pgid := workerCmd.Process.Pid
				if err := killProcessGroup(pgid, true); err != nil {
					// Fallback to single process if process group kill fails
					pm.Warning(fmt.Sprintf("Failed to terminate process group: %v", err))
					if err := workerCmd.Process.Kill(); err != nil {
						pm.Warning(fmt.Sprintf("Failed to terminate process: %v", err))
					}
				}

				// Wait for process to exit gracefully (with timeout)
				// The 'done' channel is already waiting on workerCmd.Wait()
				shutdownTimer := time.NewTimer(30 * time.Second)

				// Create a goroutine to listen for second interrupt
				forceShutdown := make(chan struct{}, 1)
				go func() {
					<-sigChan
					pm.Println("\n\n⚠️  Force shutdown requested...")
					close(forceShutdown)
				}()

				select {
				case err := <-done:
					// Process exited
					shutdownTimer.Stop()
					if err != nil {
						progress.Info("Worker process stopped")
					} else {
						progress.Success("Worker stopped successfully")
					}
				case <-forceShutdown:
					// User pressed Ctrl+C again - force kill immediately
					shutdownTimer.Stop()
					pm.Warning("Force killing worker process...")
					if err := killProcessGroup(pgid, false); err != nil {
						// Fallback to single process kill
						if err := workerCmd.Process.Kill(); err != nil {
							pm.Warning(fmt.Sprintf("Failed to kill process: %v", err))
						}
					}
					// Wait a brief moment and drain done channel
					time.Sleep(200 * time.Millisecond)
					select {
					case <-done:
					default:
					}
					progress.Info("Worker process force terminated")
				case <-shutdownTimer.C:
					// Timeout - force kill entire process group
					pm.Warning("Worker did not stop gracefully after 30s, forcing shutdown...")
					if err := killProcessGroup(pgid, false); err != nil {
						// Fallback to single process kill
						if err := workerCmd.Process.Kill(); err != nil {
							pm.Warning(fmt.Sprintf("Failed to kill process: %v", err))
						}
					}
					// Wait a bit for the kill to take effect and drain done channel
					time.Sleep(500 * time.Millisecond)
					select {
					case <-done:
						// Process finally exited
					default:
						// Process still not responding
					}
					progress.Info("Worker process terminated")
				}
			}

			return nil

		case err := <-done:
			// Stop LiteLLM proxy when worker exits
			if liteLLMSupervisor != nil {
				logger.Debug("Stopping local LiteLLM proxy")
				if stopErr := liteLLMSupervisor.Stop(); stopErr != nil {
					logger.Debug("Failed to stop LiteLLM proxy", "error", stopErr)
				}
			}

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

// PackageSourceType represents the type of package source
type PackageSourceType int

const (
	PackageSourcePyPI PackageSourceType = iota
	PackageSourceLocalFile
	PackageSourceGitURL
	PackageSourceGitHubShorthand
)

// PackageSourceInfo contains parsed package source information
type PackageSourceInfo struct {
	Type       PackageSourceType
	Spec       string // Full pip install spec
	Display    string // Human-readable display name
	Repository string // For git sources
	Ref        string // For git sources (commit, branch, tag)
}

// parsePackageSource parses a flexible package source specification
func parsePackageSource(source string) (*PackageSourceInfo, error) {
	if source == "" {
		return nil, fmt.Errorf("package source is empty")
	}

	// Check if it's a local file (absolute or relative path, or ends with .whl)
	if strings.HasSuffix(source, ".whl") || strings.Contains(source, "/") || strings.Contains(source, "\\") {
		if _, err := os.Stat(source); err == nil {
			return &PackageSourceInfo{
				Type:    PackageSourceLocalFile,
				Spec:    source + "[worker]",
				Display: fmt.Sprintf("local file: %s", source),
			}, nil
		}
		// If file doesn't exist but looks like a path, still treat as local file (will error later)
		if strings.HasPrefix(source, "/") || strings.HasPrefix(source, "./") || strings.HasPrefix(source, "../") {
			return &PackageSourceInfo{
				Type:    PackageSourceLocalFile,
				Spec:    source + "[worker]",
				Display: fmt.Sprintf("local file: %s", source),
			}, nil
		}
	}

	// Check if it's a git URL (git+https://... or git+ssh://...)
	if strings.HasPrefix(source, "git+") {
		return &PackageSourceInfo{
			Type:    PackageSourceGitURL,
			Spec:    source + "#egg=kubiya-control-plane-api[worker]",
			Display: fmt.Sprintf("git: %s", source),
		}, nil
	}

	// Check if it's GitHub shorthand (owner/repo@ref)
	if strings.Contains(source, "/") && strings.Contains(source, "@") {
		parts := strings.SplitN(source, "@", 2)
		if len(parts) == 2 {
			repo := parts[0]
			ref := parts[1]
			gitURL := fmt.Sprintf("git+https://github.com/%s.git@%s", repo, ref)
			return &PackageSourceInfo{
				Type:       PackageSourceGitHubShorthand,
				Spec:       gitURL + "#egg=kubiya-control-plane-api[worker]",
				Display:    fmt.Sprintf("GitHub: %s @ %s", repo, ref),
				Repository: repo,
				Ref:        ref,
			}, nil
		}
	}

	// Otherwise treat as PyPI version specifier
	packageSpec := "kubiya-control-plane-api"
	if !strings.HasPrefix(source, ">=") &&
		!strings.HasPrefix(source, "==") &&
		!strings.HasPrefix(source, "~=") &&
		!strings.HasPrefix(source, "<") &&
		!strings.HasPrefix(source, ">") {
		packageSpec += "==" + source
	} else {
		packageSpec += source
	}

	return &PackageSourceInfo{
		Type:    PackageSourcePyPI,
		Spec:    packageSpec + "[worker]",
		Display: fmt.Sprintf("PyPI: %s", source),
	}, nil
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

// PackageVersionInfo contains comprehensive version information for a package
type PackageVersionInfo struct {
	Installed        string
	Requested        string
	Latest           string
	IsInstalled      bool
	SatisfiesRequest bool
	IsOutdated       bool
}

// checkPackageVersionEx checks package version and optionally queries PyPI for latest version
func checkPackageVersionEx(venvPath, packageName, desiredVersion string, checkPyPI bool) (*PackageVersionInfo, error) {
	info := &PackageVersionInfo{
		Requested: desiredVersion,
	}

	pipPath, _ := getVenvPaths(venvPath)

	// Check installed version
	cmd := exec.Command(pipPath, "show", packageName)
	output, err := cmd.CombinedOutput()
	if err != nil {
		// Package not installed
		info.IsInstalled = false
		info.SatisfiesRequest = false
		return info, nil
	}

	// Parse output for Version: line
	lines := strings.Split(string(output), "\n")
	for _, line := range lines {
		if strings.HasPrefix(line, "Version:") {
			info.Installed = strings.TrimSpace(strings.TrimPrefix(line, "Version:"))
			info.IsInstalled = true
			break
		}
	}

	if !info.IsInstalled {
		return info, nil
	}

	// Check if installed version satisfies requested version
	if desiredVersion == "" {
		info.SatisfiesRequest = true
	} else {
		info.SatisfiesRequest = (info.Installed == desiredVersion)
	}

	// Query PyPI for latest version if requested
	if checkPyPI {
		latestVersion, err := GetLatestPackageVersion(packageName)
		if err != nil {
			// If PyPI check fails (offline, network error, etc.), don't fail the whole operation
			// Just continue without latest version info (silently handle error)
		} else {
			info.Latest = latestVersion
			// Compare installed vs latest
			if info.Installed != "" && info.Latest != "" {
				info.IsOutdated = compareVersions(info.Installed, info.Latest) < 0
			}
		}
	}

	return info, nil
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
	signal.Notify(sigChan, getShutdownSignals()...)

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
	signal.Notify(sigChan, getShutdownSignals()...)

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

