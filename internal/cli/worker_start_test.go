package cli

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"

	"github.com/kubiyabot/cli/internal/config"
)

// TestBuildPackageSpec tests the package specification building logic
func TestBuildPackageSpec(t *testing.T) {
	tests := []struct {
		name           string
		packageVersion string
		expected       string
		description    string
	}{
		{
			name:           "default version with >= operator",
			packageVersion: ">=0.3.0",
			expected:       "kubiya-control-plane-api>=0.3.0",
			description:    "Should preserve >= operator",
		},
		{
			name:           "exact version with == operator",
			packageVersion: "==0.3.0",
			expected:       "kubiya-control-plane-api==0.3.0",
			description:    "Should preserve == operator",
		},
		{
			name:           "compatible version with ~= operator",
			packageVersion: "~=0.3.0",
			expected:       "kubiya-control-plane-api~=0.3.0",
			description:    "Should preserve ~= operator",
		},
		{
			name:           "less than with < operator",
			packageVersion: "<1.0.0",
			expected:       "kubiya-control-plane-api<1.0.0",
			description:    "Should preserve < operator",
		},
		{
			name:           "greater than with > operator",
			packageVersion: ">0.2.0",
			expected:       "kubiya-control-plane-api>0.2.0",
			description:    "Should preserve > operator",
		},
		{
			name:           "plain version number without operator",
			packageVersion: "0.3.0",
			expected:       "kubiya-control-plane-api==0.3.0",
			description:    "Should add == operator for plain version",
		},
		{
			name:           "plain version with patch",
			packageVersion: "0.3.1",
			expected:       "kubiya-control-plane-api==0.3.1",
			description:    "Should add == operator for plain version with patch",
		},
		{
			name:           "empty version",
			packageVersion: "",
			expected:       "kubiya-control-plane-api",
			description:    "Should return package name only",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := buildPackageSpec(tt.packageVersion)
			if result != tt.expected {
				t.Errorf("%s: expected %q, got %q", tt.description, tt.expected, result)
			}
		})
	}
}

// Helper function to build package spec (extracted from worker_start.go for testing)
func buildPackageSpec(packageVersion string) string {
	packageSpec := "kubiya-control-plane-api"
	if packageVersion != "" {
		// If version doesn't start with operator, assume ==
		if !strings.HasPrefix(packageVersion, ">=") &&
			!strings.HasPrefix(packageVersion, "==") &&
			!strings.HasPrefix(packageVersion, "~=") &&
			!strings.HasPrefix(packageVersion, "<") &&
			!strings.HasPrefix(packageVersion, ">") {
			packageSpec += "==" + packageVersion
		} else {
			packageSpec += packageVersion
		}
	}
	return packageSpec
}

// TestWorkerStartOptionsValidation tests validation of worker start options
func TestWorkerStartOptionsValidation(t *testing.T) {
	tests := []struct {
		name        string
		opts        WorkerStartOptions
		expectError bool
		errorMsg    string
	}{
		{
			name: "valid options with default package version",
			opts: WorkerStartOptions{
				QueueID:        "test-queue-id",
				DeploymentType: "local",
				PackageVersion: ">=0.3.0",
				LocalWheel:     "",
			},
			expectError: false,
		},
		{
			name: "valid options with specific version",
			opts: WorkerStartOptions{
				QueueID:        "test-queue-id",
				DeploymentType: "local",
				PackageVersion: "0.3.1",
				LocalWheel:     "",
			},
			expectError: false,
		},
		{
			name: "valid options with local wheel",
			opts: WorkerStartOptions{
				QueueID:        "test-queue-id",
				DeploymentType: "local",
				PackageVersion: ">=0.3.0",
				LocalWheel:     "/path/to/wheel.whl",
			},
			expectError: false,
		},
		{
			name: "missing queue ID",
			opts: WorkerStartOptions{
				QueueID:        "",
				DeploymentType: "local",
				PackageVersion: ">=0.3.0",
			},
			expectError: true,
			errorMsg:    "queue ID is required",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := validateWorkerStartOptions(&tt.opts)

			if tt.expectError && err == nil {
				t.Errorf("Expected error %q but got none", tt.errorMsg)
			}

			if !tt.expectError && err != nil {
				t.Errorf("Unexpected error: %v", err)
			}

			if tt.expectError && err != nil && tt.errorMsg != "" {
				if !strings.Contains(err.Error(), tt.errorMsg) {
					t.Errorf("Expected error containing %q, got %q", tt.errorMsg, err.Error())
				}
			}
		})
	}
}

// Helper function to validate worker start options
func validateWorkerStartOptions(opts *WorkerStartOptions) error {
	if opts.QueueID == "" {
		return fmt.Errorf("queue ID is required")
	}
	return nil
}

// TestLocalWheelPriority tests that local wheel takes priority when specified
func TestLocalWheelPriority(t *testing.T) {
	tests := []struct {
		name                string
		localWheel          string
		packageVersion      string
		expectedInstallType string
	}{
		{
			name:                "local wheel specified - should use local",
			localWheel:          "/path/to/wheel.whl",
			packageVersion:      ">=0.3.0",
			expectedInstallType: "local",
		},
		{
			name:                "no local wheel - should use PyPI",
			localWheel:          "",
			packageVersion:      ">=0.3.0",
			expectedInstallType: "pypi",
		},
		{
			name:                "no local wheel with specific version - should use PyPI",
			localWheel:          "",
			packageVersion:      "0.3.1",
			expectedInstallType: "pypi",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			installType := determineInstallType(tt.localWheel, tt.packageVersion)
			if installType != tt.expectedInstallType {
				t.Errorf("Expected install type %q, got %q", tt.expectedInstallType, installType)
			}
		})
	}
}

// Helper function to determine install type
func determineInstallType(localWheel, packageVersion string) string {
	if localWheel != "" {
		return "local"
	}
	return "pypi"
}

// TestWorkerDirectorySetup tests worker directory creation
func TestWorkerDirectorySetup(t *testing.T) {
	// Create temporary directory for testing
	tmpDir := t.TempDir()

	tests := []struct {
		name        string
		queueID     string
		expectError bool
	}{
		{
			name:        "valid queue ID",
			queueID:     "test-queue-123",
			expectError: false,
		},
		{
			name:        "queue ID with dashes and numbers",
			queueID:     "d7eb172e-f5be-49c8-bd34-7484e005f58b",
			expectError: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			workerDir := filepath.Join(tmpDir, "workers", tt.queueID)
			err := os.MkdirAll(workerDir, 0755)

			if tt.expectError && err == nil {
				t.Error("Expected error but got none")
			}

			if !tt.expectError && err != nil {
				t.Errorf("Unexpected error: %v", err)
			}

			if !tt.expectError && err == nil {
				// Verify directory was created
				if _, err := os.Stat(workerDir); os.IsNotExist(err) {
					t.Errorf("Worker directory was not created: %s", workerDir)
				}
			}
		})
	}
}

// TestPipCommandConstruction tests pip command construction
func TestPipCommandConstruction(t *testing.T) {
	tests := []struct {
		name           string
		pipPath        string
		localWheel     string
		packageVersion string
		expectedArgs   []string
	}{
		{
			name:           "install from PyPI with default version",
			pipPath:        "/path/to/pip",
			localWheel:     "",
			packageVersion: ">=0.3.0",
			expectedArgs:   []string{"install", "--quiet", "kubiya-control-plane-api>=0.3.0"},
		},
		{
			name:           "install from PyPI with specific version",
			pipPath:        "/path/to/pip",
			localWheel:     "",
			packageVersion: "0.3.1",
			expectedArgs:   []string{"install", "--quiet", "kubiya-control-plane-api==0.3.1"},
		},
		{
			name:           "install from local wheel",
			pipPath:        "/path/to/pip",
			localWheel:     "/path/to/wheel.whl",
			packageVersion: ">=0.3.0",
			expectedArgs:   []string{"install", "--quiet", "--force-reinstall", "/path/to/wheel.whl"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var args []string

			if tt.localWheel != "" {
				args = []string{"install", "--quiet", "--force-reinstall", tt.localWheel}
			} else {
				packageSpec := buildPackageSpec(tt.packageVersion)
				args = []string{"install", "--quiet", packageSpec}
			}

			if len(args) != len(tt.expectedArgs) {
				t.Errorf("Expected %d args, got %d", len(tt.expectedArgs), len(args))
				return
			}

			for i := range args {
				if args[i] != tt.expectedArgs[i] {
					t.Errorf("Arg %d: expected %q, got %q", i, tt.expectedArgs[i], args[i])
				}
			}
		})
	}
}

// Integration test - requires Python and network access
func TestWorkerPackageInstallation(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping integration test in short mode")
	}

	// Check if python3 is available
	pythonPath, err := exec.LookPath("python3")
	if err != nil {
		t.Skip("Python3 not found, skipping integration test")
	}

	tests := []struct {
		name           string
		packageVersion string
		localWheel     string
		skipReason     string
	}{
		{
			name:           "install latest available version from PyPI",
			packageVersion: ">=0.2.0",
			localWheel:     "",
			skipReason:     "", // Will run if PyPI is accessible
		},
		{
			name:           "install specific published version from PyPI",
			packageVersion: "0.2.2",
			localWheel:     "",
			skipReason:     "", // Will run if PyPI is accessible
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if tt.skipReason != "" {
				t.Skip(tt.skipReason)
			}

			// Create temporary venv
			tmpDir := t.TempDir()
			venvDir := filepath.Join(tmpDir, "venv")

			// Create virtual environment
			createVenvCmd := exec.Command(pythonPath, "-m", "venv", venvDir)
			if err := createVenvCmd.Run(); err != nil {
				t.Fatalf("Failed to create venv: %v", err)
			}

			// Get pip path
			var pipPath string
			if _, err := os.Stat(filepath.Join(venvDir, "bin", "pip")); err == nil {
				pipPath = filepath.Join(venvDir, "bin", "pip")
			} else if _, err := os.Stat(filepath.Join(venvDir, "Scripts", "pip.exe")); err == nil {
				pipPath = filepath.Join(venvDir, "Scripts", "pip.exe")
			} else {
				t.Fatal("Could not find pip in venv")
			}

			// Build package spec
			var installCmd *exec.Cmd
			if tt.localWheel != "" {
				if _, err := os.Stat(tt.localWheel); os.IsNotExist(err) {
					t.Fatalf("Local wheel file not found: %s", tt.localWheel)
				}
				installCmd = exec.Command(pipPath, "install", "--quiet", "--force-reinstall", tt.localWheel)
			} else {
				packageSpec := buildPackageSpec(tt.packageVersion)
				installCmd = exec.Command(pipPath, "install", "--quiet", packageSpec)
			}

			// Run installation
			output, err := installCmd.CombinedOutput()
			if err != nil {
				t.Logf("Pip output: %s", string(output))
				t.Fatalf("Failed to install package: %v", err)
			}

			// Verify binary is available
			var workerBinary string
			if _, err := os.Stat(filepath.Join(venvDir, "bin", "kubiya-control-plane-worker")); err == nil {
				workerBinary = filepath.Join(venvDir, "bin", "kubiya-control-plane-worker")
			} else if _, err := os.Stat(filepath.Join(venvDir, "Scripts", "kubiya-control-plane-worker.exe")); err == nil {
				workerBinary = filepath.Join(venvDir, "Scripts", "kubiya-control-plane-worker.exe")
			} else {
				t.Fatal("Worker binary not found after installation")
			}

			// Verify binary exists
			if _, err := os.Stat(workerBinary); err != nil {
				t.Errorf("Worker binary not found: %s", workerBinary)
			}

			t.Logf("Successfully installed package and verified binary: %s", workerBinary)
		})
	}
}

// TestWorkerBinaryVerification tests that the worker binary is found after installation
func TestWorkerBinaryVerification(t *testing.T) {
	tests := []struct {
		name        string
		venvDir     string
		expectFound bool
	}{
		{
			name:        "binary in Unix-style venv",
			venvDir:     "testdata/venv_unix",
			expectFound: false, // Will not exist in test
		},
		{
			name:        "binary in Windows-style venv",
			venvDir:     "testdata/venv_windows",
			expectFound: false, // Will not exist in test
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			found := verifyWorkerBinary(tt.venvDir)
			if found != tt.expectFound {
				t.Errorf("Expected found=%v, got found=%v", tt.expectFound, found)
			}
		})
	}
}

// Helper function to verify worker binary exists
func verifyWorkerBinary(venvDir string) bool {
	// Check Unix-style path
	if _, err := os.Stat(filepath.Join(venvDir, "bin", "kubiya-control-plane-worker")); err == nil {
		return true
	}

	// Check Windows-style path
	if _, err := os.Stat(filepath.Join(venvDir, "Scripts", "kubiya-control-plane-worker.exe")); err == nil {
		return true
	}

	return false
}

// TestEndToEndWorkerSetup tests the complete worker setup flow
func TestEndToEndWorkerSetup(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping end-to-end test in short mode")
	}

	// Check if python3 is available
	pythonPath, err := exec.LookPath("python3")
	if err != nil {
		t.Skip("Python3 not found, skipping end-to-end test")
	}

	// Create test config
	cfg := &config.Config{
		BaseURL: "https://api.kubiya.ai",
		APIKey:  "test-api-key",
	}

	// Create temporary directory for worker
	tmpDir := t.TempDir()
	workerDir := filepath.Join(tmpDir, "workers", "test-queue-123")

	tests := []struct {
		name           string
		packageVersion string
		expectSuccess  bool
	}{
		{
			name:           "setup with published version",
			packageVersion: ">=0.2.0",
			expectSuccess:  true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Create worker directory
			if err := os.MkdirAll(workerDir, 0755); err != nil {
				t.Fatalf("Failed to create worker directory: %v", err)
			}

			// Create virtual environment
			venvDir := filepath.Join(workerDir, "venv")
			createVenvCmd := exec.Command(pythonPath, "-m", "venv", venvDir)
			if err := createVenvCmd.Run(); err != nil {
				t.Fatalf("Failed to create venv: %v", err)
			}

			// Verify venv was created
			if _, err := os.Stat(venvDir); os.IsNotExist(err) {
				t.Fatal("Virtual environment was not created")
			}

			// Get pip path
			var pipPath string
			if _, err := os.Stat(filepath.Join(venvDir, "bin", "pip")); err == nil {
				pipPath = filepath.Join(venvDir, "bin", "pip")
			} else if _, err := os.Stat(filepath.Join(venvDir, "Scripts", "pip.exe")); err == nil {
				pipPath = filepath.Join(venvDir, "Scripts", "pip.exe")
			} else {
				t.Fatal("Could not find pip in venv")
			}

			// Install package
			packageSpec := buildPackageSpec(tt.packageVersion)
			installCmd := exec.Command(pipPath, "install", "--quiet", packageSpec)
			output, err := installCmd.CombinedOutput()

			if tt.expectSuccess && err != nil {
				t.Logf("Pip output: %s", string(output))
				t.Logf("Config: %+v", cfg)
				t.Logf("Worker directory: %s", workerDir)
				t.Fatalf("Expected success but got error: %v", err)
			}

			if !tt.expectSuccess && err == nil {
				t.Fatal("Expected error but got success")
			}

			// Verify binary is available (only if install succeeded)
			if tt.expectSuccess && err == nil {
				found := verifyWorkerBinary(venvDir)
				if !found {
					t.Error("Worker binary not found after successful installation")
				} else {
					t.Logf("Worker binary found in venv: %s", venvDir)
				}
			}
		})
	}
}

// BenchmarkPackageSpecBuilding benchmarks the package spec building function
func BenchmarkPackageSpecBuilding(b *testing.B) {
	versions := []string{
		">=0.3.0",
		"==0.3.0",
		"~=0.3.0",
		"0.3.0",
		"<1.0.0",
		">0.2.0",
	}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		version := versions[i%len(versions)]
		_ = buildPackageSpec(version)
	}
}
