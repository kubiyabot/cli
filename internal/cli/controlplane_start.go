package cli

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"fmt"
	"net/http"
	"os"
	"os/exec"
	"os/signal"
	"path/filepath"
	"runtime"
	"strconv"
	"strings"
	"time"

	"github.com/kubiyabot/cli/internal/config"
	"github.com/kubiyabot/cli/internal/output"
	"github.com/kubiyabot/cli/internal/pterm"
	"github.com/kubiyabot/cli/internal/version"
	"github.com/kubiyabot/cli/internal/webui"
	ptermpkg "github.com/pterm/pterm"
	"github.com/spf13/cobra"
)

// ControlPlaneStartOptions holds all configuration for starting the control plane
type ControlPlaneStartOptions struct {
	// Package source (flexible like worker start)
	PackageSource  string
	LocalWheel     string
	PackageVersion string
	NoUpdate       bool
	NoCache        bool

	// Server configuration
	Host string
	Port int

	// Database (REQUIRED - at least one)
	DatabaseURL string
	SupabaseURL string
	SupabaseKey string

	// Optional services
	RedisURL          string
	TemporalHost      string
	SecretKey         string
	LiteLLMAPIKey     string
	LiteLLMGatewayURL string

	// Server options
	Workers     int
	Development bool

	// WebUI
	WebUIPort    int
	DisableWebUI bool

	cfg             *config.Config
	progressManager *output.ProgressManager
}

// newControlPlaneStartCommand creates the control-plane start command
func newControlPlaneStartCommand(cfg *config.Config) *cobra.Command {
	opts := &ControlPlaneStartOptions{
		cfg: cfg,
	}

	cmd := &cobra.Command{
		Use:   "start",
		Short: "🚀 Start the Control Plane API server",
		Long: `Start the Kubiya Control Plane API server locally with automatic
database migrations and flexible package installation.

The server provides:
  • REST API for agent orchestration
  • Swagger UI documentation at /api/docs
  • Real-time execution streaming
  • Multi-tenant agent management
  • Temporal workflow integration
  • Optional monitoring WebUI

Database Configuration:
  The control plane requires a PostgreSQL database. You can provide:
  1. DATABASE_URL directly
  2. Supabase credentials (SUPABASE_URL + SUPABASE_SERVICE_KEY)

The command will automatically run database migrations before starting.`,
		Example: `  # Start with default settings (uses env vars for database)
  export DATABASE_URL=postgresql://localhost/control_plane
  kubiya control-plane start

  # Start with explicit database URL
  kubiya control-plane start --database-url=postgresql://user:pass@localhost/control_plane

  # Start with Supabase
  kubiya control-plane start \
    --supabase-url=https://xxx.supabase.co \
    --supabase-key=eyJhbGc...

  # Start on custom port
  kubiya control-plane start --port=8888

  # Install from local source
  kubiya control-plane start \
    --package-source=/path/to/agent-control-plane

  # Install specific version
  kubiya control-plane start --package-version=0.5.0`,
		RunE: func(cmd *cobra.Command, args []string) error {
			return opts.Run(cmd.Context())
		},
	}

	// Package source flags
	cmd.Flags().StringVar(&opts.PackageSource, "package-source", "",
		"Package source: PyPI version, git URL, local path, or GitHub shorthand")
	cmd.Flags().StringVar(&opts.PackageVersion, "package-version", "",
		"PyPI version specifier (e.g., '0.5.0' or '>=0.3.0')")
	cmd.Flags().StringVar(&opts.LocalWheel, "local-wheel", "",
		"Path to local wheel file")
	cmd.Flags().BoolVar(&opts.NoUpdate, "no-update", false,
		"Skip checking for package updates")
	cmd.Flags().BoolVar(&opts.NoCache, "no-cache", false,
		"Force reinstall packages")

	// Server config flags
	cmd.Flags().StringVar(&opts.Host, "host", "0.0.0.0",
		"Server bind host")
	cmd.Flags().IntVar(&opts.Port, "port", 7777,
		"Server port")

	// Database flags
	cmd.Flags().StringVar(&opts.DatabaseURL, "database-url", "",
		"PostgreSQL connection URL")
	cmd.Flags().StringVar(&opts.SupabaseURL, "supabase-url", "",
		"Supabase project URL")
	cmd.Flags().StringVar(&opts.SupabaseKey, "supabase-key", "",
		"Supabase service role key")

	// Optional service flags
	cmd.Flags().StringVar(&opts.RedisURL, "redis-url", "",
		"Redis connection URL (default: redis://localhost:6379/0)")
	cmd.Flags().StringVar(&opts.TemporalHost, "temporal-host", "",
		"Temporal server host:port (default: localhost:7233)")
	cmd.Flags().StringVar(&opts.SecretKey, "secret-key", "",
		"JWT secret key (auto-generated if not provided)")
	cmd.Flags().StringVar(&opts.LiteLLMAPIKey, "litellm-api-key", "",
		"LiteLLM API key for LLM features")
	cmd.Flags().StringVar(&opts.LiteLLMGatewayURL, "litellm-gateway-url", "",
		"LiteLLM gateway URL for model routing (e.g., http://localhost:4000)")

	// Server options
	cmd.Flags().IntVar(&opts.Workers, "workers", 1,
		"Number of Gunicorn worker processes (default: 1)")
	cmd.Flags().BoolVar(&opts.Development, "development", false,
		"Enable development mode with hot reloading")

	// WebUI flags
	cmd.Flags().IntVar(&opts.WebUIPort, "webui-port", 0,
		"WebUI port (0 = auto-assign, disabled with --no-webui)")
	cmd.Flags().BoolVar(&opts.DisableWebUI, "no-webui", false,
		"Disable monitoring WebUI")

	return cmd
}

// Run executes the control-plane start command
func (opts *ControlPlaneStartOptions) Run(ctx context.Context) error {
	// Initialize progress manager
	opts.progressManager = output.NewProgressManager()
	pm := opts.progressManager
	ptermManager := pm.PTermManager()

	// Apply environment variables if flags not set
	opts.applyEnvironmentVariables()

	// Validate database configuration
	if err := opts.validateDatabase(); err != nil {
		return err
	}

	// Display startup header
	opts.displayStartupHeader()

	// Stage 1: Environment setup
	pm.Println(ptermpkg.LightCyan("[1/4] "))
	pythonCmd, pythonVersion, err := opts.checkPythonVersion()
	if err != nil {
		return err
	}
	pm.Success(fmt.Sprintf("Environment ready • Python %s", pythonVersion))

	// Check pip availability
	if err := checkPipAvailable(pythonCmd); err != nil {
		return fmt.Errorf("pip is required but not available: %w", err)
	}

	// Detect package manager (uv or pip)
	pkgMgr := NewPackageManager()

	// Setup virtual environment
	cpDir := filepath.Join(os.Getenv("HOME"), ".kubiya", "control-plane")
	venvDir := filepath.Join(cpDir, "venv")

	if err := os.MkdirAll(cpDir, 0755); err != nil {
		return fmt.Errorf("failed to create control-plane directory: %w", err)
	}

	if !pathExists(venvDir) {
		spinner := NewPTermSpinner(ptermManager, "Creating virtual environment")
		createCmd := exec.Command(pythonCmd, "-m", "venv", venvDir)
		if err := createCmd.Run(); err != nil {
			spinner.Fail("Failed to create virtual environment")
			return fmt.Errorf("failed to create virtual environment: %w", err)
		}
		spinner.Success("Virtual environment created")
	}

	// Upgrade pip if needed (only for pip, not uv)
	if pkgMgr.Name() == "pip" {
		// Check if pip needs upgrade
		pipNeedsUpgrade := false
		pipPath := filepath.Join(venvDir, "bin", "pip")
		if runtime.GOOS == "windows" {
			pipPath = filepath.Join(venvDir, "Scripts", "pip.exe")
		}
		versionCmd := exec.Command(pipPath, "--version")
		if output, err := versionCmd.CombinedOutput(); err == nil {
			// Simple heuristic: if pip version is less than 23.0, upgrade
			versionStr := string(output)
			if strings.Contains(versionStr, "pip ") {
				// Parse version
				parts := strings.Fields(versionStr)
				if len(parts) >= 2 {
					version := parts[1]
					versionParts := strings.Split(version, ".")
					if len(versionParts) > 0 {
						majorVersion, _ := strconv.Atoi(versionParts[0])
						if majorVersion < 23 {
							pipNeedsUpgrade = true
						}
					}
				}
			}
		}

		if pipNeedsUpgrade {
			spinner := NewPTermSpinner(ptermManager, "Upgrading pip")
			if err := UpgradePip(venvDir, true); err != nil {
				spinner.Warning("Failed to upgrade pip (continuing anyway)")
			} else {
				spinner.Success("Pip upgraded")
			}
		}
	}

	// Stage 2: Package installation
	pm.Println(ptermpkg.LightCyan("\n[2/4] "))
	packageSpec, err := opts.resolvePackageSpec()
	if err != nil {
		return err
	}

	spinner := NewPTermSpinner(ptermManager, "Installing Control Plane API")
	pm.Printf("      → %s\n", packageSpec)

	installCtx, installCancel := context.WithTimeout(context.Background(), 300*time.Second)
	defer installCancel()

	forceReinstall := opts.NoCache || !opts.NoUpdate
	if err := pkgMgr.Install(installCtx, venvDir, packageSpec, true, forceReinstall); err != nil {
		spinner.Fail("Failed to install Control Plane API")
		return fmt.Errorf("package installation failed: %w", err)
	}
	spinner.Success("Control Plane API installed")

	// Stage 3: Database migrations
	pm.Println(ptermpkg.LightCyan("\n[3/4] "))
	if err := opts.runMigrations(venvDir, ptermManager); err != nil {
		return err
	}

	// Stage 4: Start server
	pm.Println(ptermpkg.LightCyan("\n[4/4] "))

	// Start WebUI if enabled
	var webuiServer *webui.Server
	var webuiURL string
	if !opts.DisableWebUI {
		webuiConfig := webui.ServerConfig{
			QueueID:         fmt.Sprintf("control-plane-%d", opts.Port),
			WorkerDir:       cpDir,
			ControlPlaneURL: fmt.Sprintf("http://%s:%d", opts.Host, opts.Port),
			Port:            opts.WebUIPort,
			DeploymentType:  "control-plane",
			Version:         version.Version,
			BuildCommit:     version.GetCommit(),
			BuildDate:       version.GetBuildDate(),
			GoVersion:       runtime.Version(),
			OS:              runtime.GOOS,
			Arch:            runtime.GOARCH,
		}

		var err error
		webuiServer, err = webui.NewServer(webuiConfig)
		if err == nil {
			if startErr := webuiServer.Start(ctx); startErr == nil {
				webuiURL = webuiServer.URL()
			}
		}
	}

	serverCmd, err := opts.startServer(ctx, venvDir, webuiServer, ptermManager)
	if err != nil {
		if webuiServer != nil {
			webuiServer.Stop()
		}
		return err
	}

	// Update WebUI with server PID
	if webuiServer != nil {
		webuiServer.SetWorkerPID(serverCmd.Process.Pid)
	}

	pm.Success(fmt.Sprintf("Server started (PID: %d)", serverCmd.Process.Pid))

	// Display success banner
	opts.displaySuccessBanner(webuiURL)

	// Handle signals and wait for completion
	return opts.handleSignals(ctx, serverCmd, webuiServer)
}

// applyEnvironmentVariables applies environment variables to options if flags not set
func (opts *ControlPlaneStartOptions) applyEnvironmentVariables() {
	// Package source
	if opts.PackageSource == "" {
		if envSource := os.Getenv("KUBIYA_CONTROL_PLANE_PACKAGE_SOURCE"); envSource != "" {
			opts.PackageSource = envSource
		}
	}

	// Database
	if opts.DatabaseURL == "" {
		opts.DatabaseURL = os.Getenv("DATABASE_URL")
	}
	if opts.SupabaseURL == "" {
		opts.SupabaseURL = os.Getenv("SUPABASE_URL")
	}
	if opts.SupabaseKey == "" {
		opts.SupabaseKey = os.Getenv("SUPABASE_SERVICE_KEY")
	}

	// Optional services
	if opts.RedisURL == "" {
		opts.RedisURL = os.Getenv("REDIS_URL")
	}
	if opts.TemporalHost == "" {
		opts.TemporalHost = os.Getenv("TEMPORAL_HOST")
	}
	if opts.SecretKey == "" {
		opts.SecretKey = os.Getenv("SECRET_KEY")
	}
	if opts.LiteLLMAPIKey == "" {
		opts.LiteLLMAPIKey = os.Getenv("LITELLM_API_KEY")
	}
	if opts.LiteLLMGatewayURL == "" {
		opts.LiteLLMGatewayURL = os.Getenv("LITELLM_GATEWAY_URL")
	}

	// Server options
	if opts.Workers == 1 { // default value
		if workers := os.Getenv("GUNICORN_WORKERS"); workers != "" {
			if w, err := strconv.Atoi(workers); err == nil && w > 0 {
				opts.Workers = w
			}
		}
	}
	if !opts.Development {
		if dev := os.Getenv("DEVELOPMENT"); dev == "true" || dev == "1" {
			opts.Development = true
		}
	}
}

// validateDatabase checks that database configuration is provided
func (opts *ControlPlaneStartOptions) validateDatabase() error {
	hasDatabaseURL := opts.DatabaseURL != ""
	hasSupabase := opts.SupabaseURL != "" && opts.SupabaseKey != ""

	if !hasDatabaseURL && !hasSupabase {
		return fmt.Errorf(`❌ Database configuration required

Please provide database connection:

Option 1: Direct PostgreSQL URL
  kubiya control-plane start --database-url=postgresql://user:pass@host:5432/dbname

Option 2: Supabase credentials
  kubiya control-plane start --supabase-url=https://xxx.supabase.co \
    --supabase-key=eyJhbGc...

Or set environment variables:
  export DATABASE_URL=postgresql://...
  # OR
  export SUPABASE_URL=https://...
  export SUPABASE_SERVICE_KEY=eyJhbGc...`)
	}

	return nil
}

// checkPythonVersion checks for Python 3.10+ (required by control-plane)
func (opts *ControlPlaneStartOptions) checkPythonVersion() (string, string, error) {
	pythonCmds := []string{"python3", "python"}

	for _, cmd := range pythonCmds {
		versionCmd := exec.Command(cmd, "--version")
		output, err := versionCmd.CombinedOutput()
		if err != nil {
			continue
		}

		versionStr := strings.TrimSpace(string(output))
		// Parse "Python X.Y.Z"
		parts := strings.Fields(versionStr)
		if len(parts) >= 2 {
			versionParts := strings.Split(parts[1], ".")
			if len(versionParts) >= 2 {
				major := versionParts[0]
				minor := versionParts[1]

				// Check for Python 3.10+
				if major == "3" {
					minorInt, _ := strconv.Atoi(minor)
					if minorInt >= 10 {
						return cmd, parts[1], nil
					}
				}
			}
		}
	}

	return "", "", fmt.Errorf(`❌ Python 3.10+ is required for Control Plane

Please install Python 3.10 or later:
  macOS:   brew install python@3.11
  Ubuntu:  sudo apt install python3.11
  Windows: Download from python.org`)
}

// resolvePackageSpec determines the package specification to install
func (opts *ControlPlaneStartOptions) resolvePackageSpec() (string, error) {
	packageName := "kubiya-control-plane-api"
	extras := "[api]"

	// Priority order: PackageSource > LocalWheel > PackageVersion > latest
	if opts.PackageSource != "" {
		sourceInfo := parseControlPlanePackageSource(opts.PackageSource)
		return sourceInfo.Spec, nil
	}

	if opts.LocalWheel != "" {
		if !pathExists(opts.LocalWheel) {
			return "", fmt.Errorf("local wheel file not found: %s", opts.LocalWheel)
		}
		return opts.LocalWheel, nil
	}

	if opts.PackageVersion != "" {
		return fmt.Sprintf("%s%s==%s", packageName, extras, opts.PackageVersion), nil
	}

	// Default: fetch latest from PyPI
	if !opts.NoUpdate {
		latestVersion, err := GetLatestPackageVersion(packageName)
		if err == nil && latestVersion != "" {
			return fmt.Sprintf("%s%s==%s", packageName, extras, latestVersion), nil
		}
	}

	// Fallback: install without version constraint
	return fmt.Sprintf("%s%s", packageName, extras), nil
}

// parseControlPlanePackageSource parses flexible package source formats
func parseControlPlanePackageSource(source string) *PackageSourceInfo {
	packageName := "kubiya-control-plane-api"
	extras := "[api]"

	info := &PackageSourceInfo{
		Type:    PackageSourcePyPI,
		Display: source,
	}

	// Check if it's a file path
	if strings.HasPrefix(source, "/") || strings.HasPrefix(source, "./") || strings.HasPrefix(source, "../") {
		info.Type = PackageSourceLocalFile
		info.Spec = source
		return info
	}

	// Check if it's a git URL
	if strings.HasPrefix(source, "git+") || strings.Contains(source, ".git") {
		info.Type = PackageSourceGitURL
		info.Spec = source
		return info
	}

	// Check if it's GitHub shorthand (owner/repo@ref)
	if strings.Contains(source, "/") && strings.Contains(source, "@") {
		parts := strings.Split(source, "@")
		if len(parts) == 2 {
			info.Type = PackageSourceGitHubShorthand
			gitURL := fmt.Sprintf("git+https://github.com/%s.git@%s", parts[0], parts[1])
			info.Spec = gitURL
			info.Repository = parts[0]
			info.Ref = parts[1]
			return info
		}
	}

	// Otherwise, treat as PyPI version specifier
	info.Type = PackageSourcePyPI
	// Add package name and extras if just a version number
	if !strings.Contains(source, packageName) {
		info.Spec = fmt.Sprintf("%s%s==%s", packageName, extras, source)
	} else {
		info.Spec = source
	}

	return info
}

// runMigrations runs alembic database migrations
func (opts *ControlPlaneStartOptions) runMigrations(venvDir string, ptermManager *pterm.PTermManager) error {
	spinner := NewPTermSpinner(ptermManager, "Running database migrations")

	pythonPath := filepath.Join(venvDir, "bin", "python")
	if runtime.GOOS == "windows" {
		pythonPath = filepath.Join(venvDir, "Scripts", "python.exe")
	}

	// Build environment with database config
	env := opts.buildEnvVars()

	// Find the control_plane_api package location in site-packages
	findPkgCmd := exec.Command(pythonPath, "-c", "import control_plane_api; import os; print(os.path.dirname(control_plane_api.__file__))")
	findPkgCmd.Env = env
	pkgPathOutput, err := findPkgCmd.CombinedOutput()
	if err != nil {
		spinner.Fail("Failed to locate control_plane_api package")
		return fmt.Errorf("failed to locate control_plane_api package: %w\nOutput: %s", err, string(pkgPathOutput))
	}

	pkgPath := strings.TrimSpace(string(pkgPathOutput))

	// Run: alembic upgrade head from the package directory
	// Use the alembic command from venv
	alembicPath := filepath.Join(venvDir, "bin", "alembic")
	if runtime.GOOS == "windows" {
		alembicPath = filepath.Join(venvDir, "Scripts", "alembic.exe")
	}

	cmd := exec.Command(alembicPath, "upgrade", "head")
	cmd.Dir = pkgPath // Run from package directory where alembic.ini is located
	cmd.Env = env

	output, err := cmd.CombinedOutput()
	if err != nil {
		spinner.Fail("Migration failed")

		// Parse error and provide helpful message
		outputStr := string(output)
		if strings.Contains(outputStr, "connection refused") || strings.Contains(outputStr, "could not connect") {
			return fmt.Errorf(`❌ Database migration failed: connection refused

PostgreSQL is not running. Start it:
  macOS:   brew services start postgresql
  Linux:   sudo systemctl start postgresql

Test connection:
  psql %s -c "SELECT 1"`, opts.getDatabaseURLForDisplay())
		} else if strings.Contains(outputStr, "authentication failed") || strings.Contains(outputStr, "password authentication failed") {
			return fmt.Errorf(`❌ Database migration failed: authentication failed

Check your credentials:
  Database URL: %s

Verify username and password are correct.`, opts.getDatabaseURLForDisplay())
		} else if strings.Contains(outputStr, "database") && strings.Contains(outputStr, "does not exist") {
			return fmt.Errorf(`❌ Database migration failed: database does not exist

Create the database first:
  createdb <database_name>

Or check your DATABASE_URL is correct: %s`, opts.getDatabaseURLForDisplay())
		}

		return fmt.Errorf("migration failed: %w\n%s", err, outputStr)
	}

	spinner.Success("Database migrations complete")
	return nil
}

// startServer starts the uvicorn server
func (opts *ControlPlaneStartOptions) startServer(ctx context.Context, venvDir string, webuiServer *webui.Server, ptermManager *pterm.PTermManager) (*exec.Cmd, error) {
	spinner := NewPTermSpinner(ptermManager, "Starting API server")

	pythonPath := filepath.Join(venvDir, "bin", "python")
	if runtime.GOOS == "windows" {
		pythonPath = filepath.Join(venvDir, "Scripts", "python.exe")
	}

	var serverCmd *exec.Cmd

	// Use Gunicorn for multiple workers (production)
	if opts.Workers > 1 {
		// Command: gunicorn control_plane_api.app.main:app -w <workers> -k uvicorn.workers.UvicornWorker -b <host>:<port>
		serverCmd = exec.Command(
			pythonPath, "-m", "gunicorn",
			"control_plane_api.app.main:app",
			"-w", strconv.Itoa(opts.Workers),
			"-k", "uvicorn.workers.UvicornWorker",
			"-b", fmt.Sprintf("%s:%d", opts.Host, opts.Port),
		)
	} else {
		// Command: python -m uvicorn control_plane_api.app.main:app --host 0.0.0.0 --port 7777
		args := []string{
			"-m", "uvicorn",
			"control_plane_api.app.main:app",
			"--host", opts.Host,
			"--port", strconv.Itoa(opts.Port),
		}

		// Add --reload for development mode
		if opts.Development {
			args = append(args, "--reload")
		}

		serverCmd = exec.Command(pythonPath, args...)
	}

	// Set process group for signal handling
	setupProcessGroup(serverCmd)

	// Environment variables
	serverCmd.Env = opts.buildEnvVars()

	// Connect stdout/stderr
	if webuiServer != nil {
		stdoutCapture := webuiServer.GetLogWriter("control-plane-stdout", webui.LogLevelInfo)
		stderrCapture := webuiServer.GetLogWriter("control-plane-stderr", webui.LogLevelError)
		serverCmd.Stdout = webui.CreateMultiWriter(os.Stdout, stdoutCapture)
		serverCmd.Stderr = webui.CreateMultiWriter(os.Stderr, stderrCapture)
	} else {
		serverCmd.Stdout = os.Stdout
		serverCmd.Stderr = os.Stderr
	}

	// Start process
	if err := serverCmd.Start(); err != nil {
		spinner.Fail("Failed to start server")
		return nil, fmt.Errorf("failed to start server: %w", err)
	}

	// Wait for server to be healthy
	spinner.Update("Waiting for health check")
	if err := opts.waitForServerReady(); err != nil {
		spinner.Fail("Server health check failed")
		serverCmd.Process.Kill()
		return nil, err
	}

	spinner.Success("Server ready")
	return serverCmd, nil
}

// waitForServerReady waits for the server health check to pass
func (opts *ControlPlaneStartOptions) waitForServerReady() error {
	healthURL := fmt.Sprintf("http://%s:%d/api/health", opts.Host, opts.Port)
	client := &http.Client{Timeout: 2 * time.Second}

	for i := 0; i < 30; i++ {
		resp, err := client.Get(healthURL)
		if err == nil && resp.StatusCode == 200 {
			resp.Body.Close()
			return nil
		}
		if resp != nil {
			resp.Body.Close()
		}
		time.Sleep(1 * time.Second)
	}

	return fmt.Errorf(`server did not become healthy within 30 seconds

Check if port %d is already in use:
  lsof -i:%d

Or try a different port:
  kubiya control-plane start --port=8888`, opts.Port, opts.Port)
}

// buildEnvVars builds the environment variable list for server process
func (opts *ControlPlaneStartOptions) buildEnvVars() []string {
	env := os.Environ()

	// Database (required)
	if opts.DatabaseURL != "" {
		env = append(env, fmt.Sprintf("DATABASE_URL=%s", opts.DatabaseURL))
	}
	if opts.SupabaseURL != "" {
		env = append(env, fmt.Sprintf("SUPABASE_URL=%s", opts.SupabaseURL))
		env = append(env, fmt.Sprintf("SUPABASE_SERVICE_KEY=%s", opts.SupabaseKey))
	}

	// Server config
	env = append(env, fmt.Sprintf("API_HOST=%s", opts.Host))
	env = append(env, fmt.Sprintf("API_PORT=%d", opts.Port))

	// Optional services (with defaults)
	redisURL := opts.RedisURL
	if redisURL == "" {
		redisURL = getEnvOrDefault("REDIS_URL", "redis://localhost:6379/0")
	}
	env = append(env, fmt.Sprintf("REDIS_URL=%s", redisURL))

	temporalHost := opts.TemporalHost
	if temporalHost == "" {
		temporalHost = getEnvOrDefault("TEMPORAL_HOST", "localhost:7233")
	}
	env = append(env, fmt.Sprintf("TEMPORAL_HOST=%s", temporalHost))

	// Secret key (generate if not provided)
	secretKey := opts.SecretKey
	if secretKey == "" {
		secretKey = os.Getenv("SECRET_KEY")
		if secretKey == "" {
			secretKey = generateSecretKey()
			ptermpkg.Warning.Println("⚠️  Using auto-generated SECRET_KEY (not suitable for production)")
		}
	}
	env = append(env, fmt.Sprintf("SECRET_KEY=%s", secretKey))

	// LiteLLM (optional but recommended)
	if opts.LiteLLMAPIKey != "" {
		env = append(env, fmt.Sprintf("LITELLM_API_KEY=%s", opts.LiteLLMAPIKey))
	} else if key := os.Getenv("LITELLM_API_KEY"); key != "" {
		env = append(env, fmt.Sprintf("LITELLM_API_KEY=%s", key))
	} else {
		ptermpkg.Warning.Println("⚠️  LITELLM_API_KEY not set - LLM features unavailable")
	}

	// LiteLLM Gateway URL (optional)
	if opts.LiteLLMGatewayURL != "" {
		env = append(env, fmt.Sprintf("LITELLM_GATEWAY_URL=%s", opts.LiteLLMGatewayURL))
	}

	return env
}

// handleSignals handles graceful shutdown on Ctrl+C
func (opts *ControlPlaneStartOptions) handleSignals(ctx context.Context, serverCmd *exec.Cmd, webuiServer *webui.Server) error {
	sigChan := make(chan os.Signal, 1)
	signal.Notify(sigChan, getShutdownSignals()...)

	done := make(chan error, 1)
	go func() {
		done <- serverCmd.Wait()
	}()

	for {
		select {
		case <-sigChan:
			opts.progressManager.Println("\nShutting down...")

			// Stop WebUI
			if webuiServer != nil {
				webuiServer.Stop()
			}

			// Send SIGTERM to process group
			pgid := serverCmd.Process.Pid
			killProcessGroup(pgid, true) // graceful=true

			// Wait with timeout
			shutdownTimer := time.NewTimer(10 * time.Second)

			// Listen for second Ctrl+C
			forceShutdown := make(chan struct{}, 1)
			go func() {
				<-sigChan
				close(forceShutdown)
			}()

			select {
			case <-done:
				shutdownTimer.Stop()
				opts.progressManager.Success("Server stopped gracefully")
			case <-forceShutdown:
				killProcessGroup(pgid, false) // SIGKILL
				opts.progressManager.Info("Server force terminated")
			case <-shutdownTimer.C:
				killProcessGroup(pgid, false)
				opts.progressManager.Info("Server terminated")
			}

			return nil

		case err := <-done:
			// Server exited unexpectedly
			if webuiServer != nil {
				webuiServer.Stop()
			}

			if err != nil {
				ptermpkg.Error.Println("Server stopped unexpectedly")
				return fmt.Errorf("server exited with error: %w", err)
			}

			return nil
		}
	}
}

// Helper functions

func (opts *ControlPlaneStartOptions) displayStartupHeader() {
	headerContent := fmt.Sprintf(`  Host:       %s
  Port:       %d
  Database:   %s`,
		opts.Host,
		opts.Port,
		opts.getDatabaseURLForDisplay(),
	)

	ptermpkg.DefaultBox.
		WithTitle("🚀 Starting Kubiya Control Plane API Server").
		WithTitleTopCenter().
		WithBoxStyle(ptermpkg.NewStyle(ptermpkg.FgLightCyan)).
		Println(headerContent)

	ptermpkg.Println()
}

func (opts *ControlPlaneStartOptions) displaySuccessBanner(webuiURL string) {
	apiURL := fmt.Sprintf("http://%s:%d", opts.Host, opts.Port)
	docsURL := fmt.Sprintf("%s/docs", apiURL)

	successContent := fmt.Sprintf(`  🎯 Control Plane API is running

  Status:   Active ●
  API:      %s
  Docs:     %s`,
		apiURL,
		docsURL,
	)

	if webuiURL != "" {
		successContent += fmt.Sprintf("\n  WebUI:    %s", webuiURL)
	}

	successContent += "\n\n  Press Ctrl+C to stop gracefully"

	ptermpkg.DefaultBox.
		WithTitle("✓ Ready").
		WithTitleTopCenter().
		WithBoxStyle(ptermpkg.NewStyle(ptermpkg.FgGreen)).
		Println(successContent)
}

func (opts *ControlPlaneStartOptions) getDatabaseURLForDisplay() string {
	if opts.DatabaseURL != "" {
		// Mask password in URL
		return maskPasswordInURL(opts.DatabaseURL)
	}
	if opts.SupabaseURL != "" {
		return opts.SupabaseURL
	}
	return "not configured"
}

func maskPasswordInURL(url string) string {
	// Simple masking: postgresql://user:****@host:port/db
	if idx := strings.Index(url, "://"); idx != -1 {
		rest := url[idx+3:]
		if atIdx := strings.Index(rest, "@"); atIdx != -1 {
			userPass := rest[:atIdx]
			if colonIdx := strings.Index(userPass, ":"); colonIdx != -1 {
				user := userPass[:colonIdx]
				return fmt.Sprintf("%s://%s:****%s", url[:idx], user, rest[atIdx:])
			}
		}
	}
	return url
}

func generateSecretKey() string {
	bytes := make([]byte, 32)
	if _, err := rand.Read(bytes); err != nil {
		// Fallback to timestamp-based key
		return fmt.Sprintf("%d", time.Now().UnixNano())
	}
	return hex.EncodeToString(bytes)
}

func getEnvOrDefault(key, defaultValue string) string {
	if val := os.Getenv(key); val != "" {
		return val
	}
	return defaultValue
}

func pathExists(path string) bool {
	_, err := os.Stat(path)
	return err == nil
}
