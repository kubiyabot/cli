package webui

import (
	"context"
	"fmt"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strconv"
	"strings"
	"syscall"
	"time"
)

const (
	minPythonMajor = 3
	minPythonMinor = 8
)

// DiagnosticsRunner runs diagnostic checks and produces reports
type DiagnosticsRunner struct {
	workerDir       string
	controlPlaneURL string
	apiKey          string
	workerPID       int
}

// NewDiagnosticsRunner creates a new diagnostics runner
func NewDiagnosticsRunner(workerDir, controlPlaneURL, apiKey string, workerPID int) *DiagnosticsRunner {
	return &DiagnosticsRunner{
		workerDir:       workerDir,
		controlPlaneURL: controlPlaneURL,
		apiKey:          apiKey,
		workerPID:       workerPID,
	}
}

// RunAllChecks runs all diagnostic checks and returns a complete report
func (d *DiagnosticsRunner) RunAllChecks(ctx context.Context) *DiagnosticsReport {
	report := &DiagnosticsReport{
		Timestamp: time.Now(),
		Checks:    []DiagnosticCheck{},
	}

	// Run all check categories
	checks := []func(context.Context) DiagnosticCheck{
		d.checkPythonVersion,
		d.checkPipAvailable,
		d.checkVirtualEnv,
		d.checkWorkerPackage,
		d.checkLiteLLMPackage,
		d.checkLangfusePackage,
		d.checkControlPlaneConnectivity,
		d.checkAPIKeyValidity,
		d.checkWorkerProcess,
		d.checkWorkerDirectory,
	}

	for _, check := range checks {
		result := check(ctx)
		report.Checks = append(report.Checks, result)
	}

	// Calculate summary
	report.Summary = d.calculateSummary(report.Checks)
	report.Overall = d.determineOverallStatus(report.Summary)

	return report
}

// RunCategoryChecks runs checks for a specific category
func (d *DiagnosticsRunner) RunCategoryChecks(ctx context.Context, category DiagnosticCategory) *DiagnosticsReport {
	report := &DiagnosticsReport{
		Timestamp: time.Now(),
		Checks:    []DiagnosticCheck{},
	}

	var checks []func(context.Context) DiagnosticCheck

	switch category {
	case DiagnosticCategoryPython:
		checks = []func(context.Context) DiagnosticCheck{
			d.checkPythonVersion,
			d.checkPipAvailable,
			d.checkVirtualEnv,
		}
	case DiagnosticCategoryPackages:
		checks = []func(context.Context) DiagnosticCheck{
			d.checkWorkerPackage,
			d.checkLiteLLMPackage,
			d.checkLangfusePackage,
		}
	case DiagnosticCategoryConnectivity:
		checks = []func(context.Context) DiagnosticCheck{
			d.checkControlPlaneConnectivity,
			d.checkAPIKeyValidity,
		}
	case DiagnosticCategoryProcess:
		checks = []func(context.Context) DiagnosticCheck{
			d.checkWorkerProcess,
			d.checkWorkerDirectory,
		}
	case DiagnosticCategoryConfig:
		checks = []func(context.Context) DiagnosticCheck{
			d.checkAPIKeyValidity,
			d.checkWorkerDirectory,
		}
	default:
		return d.RunAllChecks(ctx)
	}

	for _, check := range checks {
		result := check(ctx)
		report.Checks = append(report.Checks, result)
	}

	report.Summary = d.calculateSummary(report.Checks)
	report.Overall = d.determineOverallStatus(report.Summary)

	return report
}

// ============================================================================
// Python Environment Checks
// ============================================================================

func (d *DiagnosticsRunner) checkPythonVersion(ctx context.Context) DiagnosticCheck {
	start := time.Now()
	check := DiagnosticCheck{
		Name:     "Python Version",
		Category: DiagnosticCategoryPython,
	}

	pythonCmd, version, err := findPythonCommand()
	if err != nil {
		check.Status = DiagnosticStatusFail
		check.Message = fmt.Sprintf("Python %d.%d+ not found", minPythonMajor, minPythonMinor)
		check.Remediation = getPythonInstallInstructions()
		check.DurationMS = time.Since(start).Milliseconds()
		return check
	}

	// Parse version
	major, minor, err := parsePythonVersion(version)
	if err != nil {
		check.Status = DiagnosticStatusWarning
		check.Message = fmt.Sprintf("Could not parse Python version: %s", version)
		check.DurationMS = time.Since(start).Milliseconds()
		return check
	}

	// Check minimum version
	if major < minPythonMajor || (major == minPythonMajor && minor < minPythonMinor) {
		check.Status = DiagnosticStatusFail
		check.Message = fmt.Sprintf("Python %d.%d found, but %d.%d+ required", major, minor, minPythonMajor, minPythonMinor)
		check.Remediation = getPythonInstallInstructions()
		check.DurationMS = time.Since(start).Milliseconds()
		return check
	}

	check.Status = DiagnosticStatusPass
	check.Message = fmt.Sprintf("Python %d.%d found (%s)", major, minor, pythonCmd)
	check.Details = &PythonInfo{
		Version: version,
		Path:    pythonCmd,
	}
	check.DurationMS = time.Since(start).Milliseconds()
	return check
}

func (d *DiagnosticsRunner) checkPipAvailable(ctx context.Context) DiagnosticCheck {
	start := time.Now()
	check := DiagnosticCheck{
		Name:     "pip Available",
		Category: DiagnosticCategoryPython,
	}

	pythonCmd, _, err := findPythonCommand()
	if err != nil {
		check.Status = DiagnosticStatusSkip
		check.Message = "Skipped: Python not found"
		check.DurationMS = time.Since(start).Milliseconds()
		return check
	}

	cmd := exec.CommandContext(ctx, pythonCmd, "-m", "pip", "--version")
	output, err := cmd.CombinedOutput()
	if err != nil {
		check.Status = DiagnosticStatusFail
		check.Message = "pip is not available"
		check.Remediation = fmt.Sprintf("Run: %s -m ensurepip --upgrade", pythonCmd)
		check.DurationMS = time.Since(start).Milliseconds()
		return check
	}

	// Parse pip version
	pipVersion := parsePipVersion(string(output))

	check.Status = DiagnosticStatusPass
	check.Message = fmt.Sprintf("pip %s available", pipVersion)
	check.Details = map[string]string{"pip_version": pipVersion}
	check.DurationMS = time.Since(start).Milliseconds()
	return check
}

func (d *DiagnosticsRunner) checkVirtualEnv(ctx context.Context) DiagnosticCheck {
	start := time.Now()
	check := DiagnosticCheck{
		Name:     "Virtual Environment",
		Category: DiagnosticCategoryPython,
	}

	venvPath := filepath.Join(d.workerDir, "venv")
	if _, err := os.Stat(venvPath); os.IsNotExist(err) {
		check.Status = DiagnosticStatusWarning
		check.Message = "Virtual environment not found"
		check.Remediation = "Worker will create venv on first run"
		check.DurationMS = time.Since(start).Milliseconds()
		return check
	}

	// Check if venv is valid by looking for python binary
	pythonPath := getVenvPythonPath(venvPath)
	if _, err := os.Stat(pythonPath); os.IsNotExist(err) {
		check.Status = DiagnosticStatusFail
		check.Message = "Virtual environment exists but appears corrupted"
		check.Remediation = fmt.Sprintf("Remove and recreate: rm -rf %s", venvPath)
		check.DurationMS = time.Since(start).Milliseconds()
		return check
	}

	check.Status = DiagnosticStatusPass
	check.Message = "Virtual environment ready"
	check.Details = &PythonInfo{
		VenvPath:   venvPath,
		VenvActive: true,
	}
	check.DurationMS = time.Since(start).Milliseconds()
	return check
}

// ============================================================================
// Package Checks
// ============================================================================

func (d *DiagnosticsRunner) checkWorkerPackage(ctx context.Context) DiagnosticCheck {
	return d.checkPackage(ctx, "kubiya-control-plane-api", "worker", true)
}

func (d *DiagnosticsRunner) checkLiteLLMPackage(ctx context.Context) DiagnosticCheck {
	return d.checkPackage(ctx, "litellm", "LiteLLM proxy", false)
}

func (d *DiagnosticsRunner) checkLangfusePackage(ctx context.Context) DiagnosticCheck {
	return d.checkPackage(ctx, "langfuse", "observability", false)
}

func (d *DiagnosticsRunner) checkPackage(ctx context.Context, packageName, description string, required bool) DiagnosticCheck {
	start := time.Now()
	check := DiagnosticCheck{
		Name:     fmt.Sprintf("Package: %s", packageName),
		Category: DiagnosticCategoryPackages,
	}

	venvPath := filepath.Join(d.workerDir, "venv")
	pipPath := getVenvPipPath(venvPath)

	// Check if pip exists in venv
	if _, err := os.Stat(pipPath); os.IsNotExist(err) {
		if required {
			check.Status = DiagnosticStatusFail
			check.Message = "Virtual environment not set up"
			check.Remediation = "Run worker to initialize environment"
		} else {
			check.Status = DiagnosticStatusSkip
			check.Message = "Virtual environment not set up"
		}
		check.DurationMS = time.Since(start).Milliseconds()
		return check
	}

	// Check package version
	cmd := exec.CommandContext(ctx, pipPath, "show", packageName)
	output, err := cmd.CombinedOutput()
	if err != nil {
		if required {
			check.Status = DiagnosticStatusFail
			check.Message = fmt.Sprintf("%s not installed (required for %s)", packageName, description)
			check.Remediation = fmt.Sprintf("Run: %s install %s", pipPath, packageName)
		} else {
			check.Status = DiagnosticStatusWarning
			check.Message = fmt.Sprintf("%s not installed (optional for %s)", packageName, description)
			check.Remediation = fmt.Sprintf("Install with: %s install %s", pipPath, packageName)
		}
		check.DurationMS = time.Since(start).Milliseconds()
		return check
	}

	// Parse version from output
	installedVersion := parsePackageVersionFromPipShow(string(output))

	pkgInfo := &PackageInfo{
		Name:             packageName,
		InstalledVersion: installedVersion,
		IsInstalled:      true,
	}

	check.Status = DiagnosticStatusPass
	check.Message = fmt.Sprintf("%s v%s installed", packageName, installedVersion)
	check.Details = pkgInfo
	check.DurationMS = time.Since(start).Milliseconds()
	return check
}

// ============================================================================
// Connectivity Checks
// ============================================================================

func (d *DiagnosticsRunner) checkControlPlaneConnectivity(ctx context.Context) DiagnosticCheck {
	start := time.Now()
	check := DiagnosticCheck{
		Name:     "Control Plane Connection",
		Category: DiagnosticCategoryConnectivity,
	}

	if d.controlPlaneURL == "" {
		check.Status = DiagnosticStatusSkip
		check.Message = "Control plane URL not configured"
		check.DurationMS = time.Since(start).Milliseconds()
		return check
	}

	// Create HTTP client with timeout
	client := &http.Client{
		Timeout: 10 * time.Second,
	}

	// Check health endpoint
	healthURL := strings.TrimSuffix(d.controlPlaneURL, "/") + "/api/health"
	req, err := http.NewRequestWithContext(ctx, "GET", healthURL, nil)
	if err != nil {
		check.Status = DiagnosticStatusFail
		check.Message = fmt.Sprintf("Failed to create request: %v", err)
		check.DurationMS = time.Since(start).Milliseconds()
		return check
	}

	// Add auth header if we have an API key
	if d.apiKey != "" {
		req.Header.Set("Authorization", fmt.Sprintf("UserKey %s", d.apiKey))
	}

	requestStart := time.Now()
	resp, err := client.Do(req)
	latency := time.Since(requestStart)

	if err != nil {
		check.Status = DiagnosticStatusFail
		check.Message = fmt.Sprintf("Connection failed: %v", err)
		check.Remediation = "Check network connectivity and firewall settings"
		check.DurationMS = time.Since(start).Milliseconds()
		return check
	}
	defer resp.Body.Close()

	if resp.StatusCode == http.StatusUnauthorized || resp.StatusCode == http.StatusForbidden {
		check.Status = DiagnosticStatusWarning
		check.Message = "Connected but authentication failed"
		check.Remediation = "Check your API key: kubiya login"
		check.Details = map[string]interface{}{
			"status_code": resp.StatusCode,
			"latency_ms":  latency.Milliseconds(),
		}
		check.DurationMS = time.Since(start).Milliseconds()
		return check
	}

	if resp.StatusCode != http.StatusOK {
		check.Status = DiagnosticStatusWarning
		check.Message = fmt.Sprintf("Control plane returned status %d", resp.StatusCode)
		check.Details = map[string]interface{}{
			"status_code": resp.StatusCode,
			"latency_ms":  latency.Milliseconds(),
		}
		check.DurationMS = time.Since(start).Milliseconds()
		return check
	}

	check.Status = DiagnosticStatusPass
	check.Message = fmt.Sprintf("Connected to control plane (%dms)", latency.Milliseconds())
	check.Details = map[string]interface{}{
		"url":        d.controlPlaneURL,
		"latency_ms": latency.Milliseconds(),
	}
	check.DurationMS = time.Since(start).Milliseconds()
	return check
}

func (d *DiagnosticsRunner) checkAPIKeyValidity(ctx context.Context) DiagnosticCheck {
	start := time.Now()
	check := DiagnosticCheck{
		Name:     "API Key Configuration",
		Category: DiagnosticCategoryConfig,
	}

	if d.apiKey == "" {
		check.Status = DiagnosticStatusFail
		check.Message = "API key not configured"
		check.Remediation = "Run: kubiya login"
		check.DurationMS = time.Since(start).Milliseconds()
		return check
	}

	// Basic format validation (JWT-like structure)
	parts := strings.Split(d.apiKey, ".")
	if len(parts) != 3 {
		check.Status = DiagnosticStatusWarning
		check.Message = "API key format appears invalid"
		check.Remediation = "Regenerate API key: kubiya login"
		check.DurationMS = time.Since(start).Milliseconds()
		return check
	}

	// Mask the key for display
	maskedKey := maskAPIKey(d.apiKey)

	check.Status = DiagnosticStatusPass
	check.Message = "API key configured"
	check.Details = map[string]string{
		"key_preview": maskedKey,
	}
	check.DurationMS = time.Since(start).Milliseconds()
	return check
}

// ============================================================================
// Process Checks
// ============================================================================

func (d *DiagnosticsRunner) checkWorkerProcess(ctx context.Context) DiagnosticCheck {
	start := time.Now()
	check := DiagnosticCheck{
		Name:     "Worker Process",
		Category: DiagnosticCategoryProcess,
	}

	if d.workerPID == 0 {
		check.Status = DiagnosticStatusWarning
		check.Message = "Worker PID not available"
		check.DurationMS = time.Since(start).Milliseconds()
		return check
	}

	// Check if process exists
	process, err := os.FindProcess(d.workerPID)
	if err != nil {
		check.Status = DiagnosticStatusFail
		check.Message = fmt.Sprintf("Cannot find worker process (PID: %d)", d.workerPID)
		check.DurationMS = time.Since(start).Milliseconds()
		return check
	}

	// On Unix, FindProcess always succeeds, so we need to check if process is running
	// by sending signal 0 (which doesn't actually send a signal, just checks if process exists)
	if runtime.GOOS != "windows" {
		// Use syscall.Signal(0) to check if process is alive
		err := process.Signal(syscall.Signal(0))
		if err != nil {
			// Check if it's a permission error (process exists but we can't signal it)
			// This is still a valid running process
			if err == syscall.EPERM {
				check.Status = DiagnosticStatusPass
				check.Message = fmt.Sprintf("Worker process running (PID: %d)", d.workerPID)
				check.Details = map[string]int{"pid": d.workerPID}
				check.DurationMS = time.Since(start).Milliseconds()
				return check
			}
			check.Status = DiagnosticStatusFail
			check.Message = fmt.Sprintf("Worker process not running (PID: %d)", d.workerPID)
			check.DurationMS = time.Since(start).Milliseconds()
			return check
		}
	}

	check.Status = DiagnosticStatusPass
	check.Message = fmt.Sprintf("Worker process running (PID: %d)", d.workerPID)
	check.Details = map[string]int{"pid": d.workerPID}
	check.DurationMS = time.Since(start).Milliseconds()
	return check
}

func (d *DiagnosticsRunner) checkWorkerDirectory(ctx context.Context) DiagnosticCheck {
	start := time.Now()
	check := DiagnosticCheck{
		Name:     "Worker Directory",
		Category: DiagnosticCategoryConfig,
	}

	if d.workerDir == "" {
		check.Status = DiagnosticStatusFail
		check.Message = "Worker directory not configured"
		check.DurationMS = time.Since(start).Milliseconds()
		return check
	}

	info, err := os.Stat(d.workerDir)
	if os.IsNotExist(err) {
		check.Status = DiagnosticStatusWarning
		check.Message = "Worker directory does not exist (will be created on start)"
		check.Details = map[string]string{"path": d.workerDir}
		check.DurationMS = time.Since(start).Milliseconds()
		return check
	}
	if err != nil {
		check.Status = DiagnosticStatusFail
		check.Message = fmt.Sprintf("Cannot access worker directory: %v", err)
		check.DurationMS = time.Since(start).Milliseconds()
		return check
	}

	if !info.IsDir() {
		check.Status = DiagnosticStatusFail
		check.Message = "Worker path exists but is not a directory"
		check.DurationMS = time.Since(start).Milliseconds()
		return check
	}

	// Check if writable
	testFile := filepath.Join(d.workerDir, ".write_test")
	if err := os.WriteFile(testFile, []byte("test"), 0644); err != nil {
		check.Status = DiagnosticStatusWarning
		check.Message = "Worker directory exists but may not be writable"
		check.DurationMS = time.Since(start).Milliseconds()
		return check
	}
	os.Remove(testFile)

	// Count files
	fileCount := 0
	var totalSize int64
	filepath.Walk(d.workerDir, func(path string, info os.FileInfo, err error) error {
		if err == nil && !info.IsDir() {
			fileCount++
			totalSize += info.Size()
		}
		return nil
	})

	check.Status = DiagnosticStatusPass
	check.Message = fmt.Sprintf("Worker directory ready (%d files, %s)", fileCount, formatBytes(totalSize))
	check.Details = &DirectoryInfo{
		Path:      d.workerDir,
		Exists:    true,
		Writable:  true,
		Size:      totalSize,
		FileCount: fileCount,
	}
	check.DurationMS = time.Since(start).Milliseconds()
	return check
}

// ============================================================================
// Helper Functions
// ============================================================================

func (d *DiagnosticsRunner) calculateSummary(checks []DiagnosticCheck) DiagnosticSummary {
	summary := DiagnosticSummary{
		Total: len(checks),
	}

	for _, check := range checks {
		switch check.Status {
		case DiagnosticStatusPass:
			summary.Passed++
		case DiagnosticStatusFail:
			summary.Failed++
		case DiagnosticStatusWarning:
			summary.Warnings++
		case DiagnosticStatusSkip:
			summary.Skipped++
		}
	}

	return summary
}

func (d *DiagnosticsRunner) determineOverallStatus(summary DiagnosticSummary) string {
	if summary.Failed > 0 {
		return "unhealthy"
	}
	if summary.Warnings > 0 {
		return "degraded"
	}
	return "healthy"
}

// findPythonCommand finds an available Python command
func findPythonCommand() (string, string, error) {
	commands := []string{"python3", "python"}

	for _, cmd := range commands {
		checkCmd := exec.Command(cmd, "--version")
		output, err := checkCmd.CombinedOutput()
		if err == nil {
			version := strings.TrimSpace(string(output))
			return cmd, version, nil
		}
	}

	return "", "", fmt.Errorf("no Python installation found")
}

// parsePythonVersion parses "Python X.Y.Z" into major, minor components
func parsePythonVersion(version string) (int, int, error) {
	// Remove "Python " prefix
	version = strings.TrimPrefix(version, "Python ")
	parts := strings.Split(version, ".")

	if len(parts) < 2 {
		return 0, 0, fmt.Errorf("invalid version format")
	}

	major, err := strconv.Atoi(parts[0])
	if err != nil {
		return 0, 0, err
	}

	minor, err := strconv.Atoi(parts[1])
	if err != nil {
		return 0, 0, err
	}

	return major, minor, nil
}

// parsePipVersion parses pip version from "pip X.Y.Z from ..." output
func parsePipVersion(output string) string {
	parts := strings.Fields(output)
	if len(parts) >= 2 {
		return parts[1]
	}
	return "unknown"
}

// parsePackageVersionFromPipShow extracts version from pip show output
func parsePackageVersionFromPipShow(output string) string {
	lines := strings.Split(output, "\n")
	for _, line := range lines {
		if strings.HasPrefix(line, "Version:") {
			return strings.TrimSpace(strings.TrimPrefix(line, "Version:"))
		}
	}
	return "unknown"
}

// getVenvPythonPath returns the Python path within a venv
func getVenvPythonPath(venvDir string) string {
	if runtime.GOOS == "windows" {
		return filepath.Join(venvDir, "Scripts", "python.exe")
	}
	return filepath.Join(venvDir, "bin", "python")
}

// getVenvPipPath returns the pip path within a venv
func getVenvPipPath(venvDir string) string {
	if runtime.GOOS == "windows" {
		return filepath.Join(venvDir, "Scripts", "pip.exe")
	}
	return filepath.Join(venvDir, "bin", "pip")
}

// getPythonInstallInstructions returns OS-specific Python installation instructions
func getPythonInstallInstructions() string {
	switch runtime.GOOS {
	case "darwin":
		return "Install Python: brew install python@3.11\nOr download from https://www.python.org/downloads/"
	case "linux":
		return "Install Python:\n  Ubuntu/Debian: sudo apt install python3.11 python3.11-venv\n  Fedora: sudo dnf install python3.11\n  CentOS: sudo yum install python311"
	case "windows":
		return "Install Python: winget install Python.Python.3.11\nOr download from https://www.python.org/downloads/"
	default:
		return "Download Python from https://www.python.org/downloads/"
	}
}

// maskAPIKey masks an API key for safe display
func maskAPIKey(key string) string {
	if len(key) < 20 {
		return "***"
	}
	return key[:10] + "..." + key[len(key)-6:]
}

// formatBytes formats bytes into human-readable format
func formatBytes(bytes int64) string {
	const unit = 1024
	if bytes < unit {
		return fmt.Sprintf("%d B", bytes)
	}
	div, exp := int64(unit), 0
	for n := bytes / unit; n >= unit; n /= unit {
		div *= unit
		exp++
	}
	return fmt.Sprintf("%.1f %cB", float64(bytes)/float64(div), "KMGTPE"[exp])
}
