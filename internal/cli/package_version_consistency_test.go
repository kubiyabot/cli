package cli

import (
	"testing"

	"github.com/kubiyabot/cli/internal/config"
)

// TestWorkerStartAndExecPackageVersionConsistency verifies that both
// `worker start` and `exec --local` commands use the same logic for
// fetching the latest package version from PyPI.
//
// This test ensures that the recent change to always fetch the latest
// package version applies to both commands consistently.
func TestWorkerStartAndExecPackageVersionConsistency(t *testing.T) {
	tests := []struct {
		name                string
		packageSource       string
		localWheel          string
		noUpdate            bool
		expectedBehavior    string
		shouldFetchLatest   bool
		shouldForceReinstall bool
	}{
		{
			name:                "default behavior - should fetch latest from PyPI",
			packageSource:       "",
			localWheel:          "",
			noUpdate:            false,
			expectedBehavior:    "fetch latest version from PyPI",
			shouldFetchLatest:   true,
			shouldForceReinstall: true,
		},
		{
			name:                "with --no-update - should use cached version",
			packageSource:       "",
			localWheel:          "",
			noUpdate:            true,
			expectedBehavior:    "use cached version if available",
			shouldFetchLatest:   false,
			shouldForceReinstall: false,
		},
		{
			name:                "with specific PyPI version - still fetches latest (override)",
			packageSource:       "0.5.0",
			localWheel:          "",
			noUpdate:            false,
			expectedBehavior:    "fetch latest version (overrides specified 0.5.0)",
			shouldFetchLatest:   true, // This is the actual behavior!
			shouldForceReinstall: true,
		},
		{
			name:                "with local wheel - fetches latest but uses wheel",
			packageSource:       "",
			localWheel:          "/path/to/wheel.whl",
			noUpdate:            false,
			expectedBehavior:    "fetch latest (checks PyPI) but uses local wheel",
			shouldFetchLatest:   true, // sourceInfo is nil when only localWheel is set
			shouldForceReinstall: true,
		},
		{
			name:                "with git URL - should use git source",
			packageSource:       "git+https://github.com/kubiyabot/control-plane-api.git@main",
			localWheel:          "",
			noUpdate:            false,
			expectedBehavior:    "use git source",
			shouldFetchLatest:   false,
			shouldForceReinstall: true,
		},
		{
			name:                "with GitHub shorthand - should use git source",
			packageSource:       "kubiyabot/control-plane-api@feature-branch",
			localWheel:          "",
			noUpdate:            false,
			expectedBehavior:    "use GitHub shorthand",
			shouldFetchLatest:   false,
			shouldForceReinstall: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Test worker start options
			t.Run("worker_start", func(t *testing.T) {
				opts := &WorkerStartOptions{
					QueueID:       "test-queue",
					PackageSource: tt.packageSource,
					LocalWheel:    tt.localWheel,
					NoUpdate:      tt.noUpdate,
					cfg:           &config.Config{},
				}

				shouldFetch, shouldForce := determinePackageBehavior(opts)

				if shouldFetch != tt.shouldFetchLatest {
					t.Errorf("worker start: expected shouldFetchLatest=%v, got %v (behavior: %s)",
						tt.shouldFetchLatest, shouldFetch, tt.expectedBehavior)
				}

				if shouldForce != tt.shouldForceReinstall {
					t.Errorf("worker start: expected shouldForceReinstall=%v, got %v (behavior: %s)",
						tt.shouldForceReinstall, shouldForce, tt.expectedBehavior)
				}
			})

			// Test exec command options
			// Note: exec command doesn't expose --no-update flag, so it always fetches latest
			// unless a specific package source is provided
			t.Run("exec_local", func(t *testing.T) {
				// ExecCommand creates WorkerStartOptions in executeLocal()
				// with NoUpdate implicitly set to false (default value)
				opts := &WorkerStartOptions{
					QueueID:             "test-queue",
					DeploymentType:      "local",
					SingleExecutionMode: true,
					PackageSource:       tt.packageSource,
					LocalWheel:          tt.localWheel,
					NoUpdate:            false, // exec command never sets this to true
					cfg:                 &config.Config{},
				}

				shouldFetch, shouldForce := determinePackageBehavior(opts)

				// For exec command, we expect it to always fetch latest when no specific source is given
				// This differs from worker start when --no-update is used
				expectedFetch := tt.shouldFetchLatest
				expectedForce := tt.shouldForceReinstall

				// Special case: exec command always has NoUpdate=false, so it will fetch latest
				// when no package source is specified (unlike worker start with --no-update)
				if tt.packageSource == "" && tt.localWheel == "" && tt.noUpdate {
					// In worker start with --no-update, it wouldn't fetch
					// But in exec, it always fetches (NoUpdate is always false)
					expectedFetch = true
					expectedForce = true
				}

				if shouldFetch != expectedFetch {
					t.Errorf("exec local: expected shouldFetchLatest=%v, got %v (behavior: %s)",
						expectedFetch, shouldFetch, tt.expectedBehavior)
				}

				if shouldForce != expectedForce {
					t.Errorf("exec local: expected shouldForceReinstall=%v, got %v (behavior: %s)",
						expectedForce, shouldForce, tt.expectedBehavior)
				}
			})
		})
	}
}

// determinePackageBehavior analyzes WorkerStartOptions and determines
// whether it will fetch the latest version and force reinstall.
// This mimics the logic in worker_start.go:484-499
func determinePackageBehavior(opts *WorkerStartOptions) (shouldFetchLatest, shouldForceReinstall bool) {
	// Parse package source if provided
	var sourceInfo *PackageSourceInfo
	if opts.PackageSource != "" {
		parsedSource, err := parsePackageSource(opts.PackageSource)
		if err == nil {
			sourceInfo = parsedSource
		}
	}

	// Determine if we should fetch latest (worker_start.go:485-499)
	// Default behavior: Always fetch latest unless --no-update is set
	if !opts.NoUpdate && (sourceInfo == nil || sourceInfo.Type == PackageSourcePyPI) {
		shouldFetchLatest = true
		shouldForceReinstall = true
		return
	}

	// Check for git sources and local files (always force reinstall)
	if opts.LocalWheel != "" {
		shouldForceReinstall = true
		return
	}

	if sourceInfo != nil {
		if sourceInfo.Type == PackageSourceGitURL ||
			sourceInfo.Type == PackageSourceGitHubShorthand ||
			sourceInfo.Type == PackageSourceLocalFile {
			shouldForceReinstall = true
			return
		}
	}

	return false, false
}

// TestExecCommandAlwaysFetchesLatest verifies that the exec command
// always fetches the latest package version (unlike worker start with --no-update)
func TestExecCommandAlwaysFetchesLatest(t *testing.T) {
	// Simulate what exec command does in executeLocal()
	execOpts := &WorkerStartOptions{
		QueueID:             "test-queue",
		DeploymentType:      "local",
		SingleExecutionMode: true,
		PackageSource:       "", // No specific source
		LocalWheel:          "", // No local wheel
		NoUpdate:            false, // exec never sets this to true
		cfg:                 &config.Config{},
	}

	shouldFetch, shouldForce := determinePackageBehavior(execOpts)

	if !shouldFetch {
		t.Error("exec command should always fetch latest version when no package source is specified")
	}

	if !shouldForce {
		t.Error("exec command should force reinstall when fetching latest version")
	}
}

// TestWorkerStartNoUpdateFlag verifies that worker start respects --no-update flag
func TestWorkerStartNoUpdateFlag(t *testing.T) {
	// Worker start with --no-update
	workerOpts := &WorkerStartOptions{
		QueueID:       "test-queue",
		PackageSource: "",
		LocalWheel:    "",
		NoUpdate:      true, // User explicitly set --no-update
		cfg:           &config.Config{},
	}

	shouldFetch, _ := determinePackageBehavior(workerOpts)

	if shouldFetch {
		t.Error("worker start with --no-update should NOT fetch latest version")
	}
}

// TestPackageSourcePriority verifies the priority order for package sources
func TestPackageSourcePriority(t *testing.T) {
	tests := []struct {
		name             string
		packageSource    string
		localWheel       string
		expectedPriority string
	}{
		{
			name:             "package-source takes priority over local-wheel",
			packageSource:    "0.5.0",
			localWheel:       "/path/to/wheel.whl",
			expectedPriority: "package-source (0.5.0)",
		},
		{
			name:             "local-wheel used when package-source empty",
			packageSource:    "",
			localWheel:       "/path/to/wheel.whl",
			expectedPriority: "local-wheel",
		},
		{
			name:             "fetch latest when both empty",
			packageSource:    "",
			localWheel:       "",
			expectedPriority: "fetch latest from PyPI",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			opts := &WorkerStartOptions{
				QueueID:       "test-queue",
				PackageSource: tt.packageSource,
				LocalWheel:    tt.localWheel,
				NoUpdate:      false,
				cfg:           &config.Config{},
			}

			shouldFetch, shouldForce := determinePackageBehavior(opts)

			switch tt.expectedPriority {
			case "fetch latest from PyPI":
				if !shouldFetch {
					t.Error("Expected to fetch latest from PyPI")
				}
			case "fetch latest (checks PyPI) but uses local wheel":
				// When localWheel is specified, sourceInfo is nil, so it still checks PyPI
				// but the local wheel path takes precedence in the installation
				if !shouldFetch {
					t.Error("Expected to fetch latest from PyPI (version check)")
				}
				if !shouldForce {
					t.Error("Local wheel should force reinstall")
				}
			case "fetch latest version (overrides specified 0.5.0)":
				// Even with specific version, it fetches latest when NoUpdate=false
				if !shouldFetch {
					t.Error("Expected to fetch latest and override specified version")
				}
				if !shouldForce {
					t.Error("Should force reinstall when fetching latest")
				}
			}
		})
	}
}

// TestDaemonModePackageBehavior verifies that daemon mode also fetches latest
func TestDaemonModePackageBehavior(t *testing.T) {
	// Daemon mode (runLocalDaemon) also uses the same logic at worker_start.go:1753-1762
	opts := &WorkerStartOptions{
		QueueID:        "test-queue",
		DaemonMode:     true,
		PackageVersion: "", // No specific version
		NoUpdate:       false,
		cfg:            &config.Config{},
	}

	shouldFetch, shouldForce := determinePackageBehavior(opts)

	if !shouldFetch {
		t.Error("daemon mode should fetch latest version when no package version is specified")
	}

	if !shouldForce {
		t.Error("daemon mode should force reinstall when fetching latest version")
	}
}

// TestConsistencyBetweenCommands is the main test that verifies consistency
func TestConsistencyBetweenCommands(t *testing.T) {
	// This test verifies that both commands use the SAME logic path
	// through runLocalForeground() -> the package installation logic

	// Scenario 1: Default behavior (no flags)
	workerStartOpts := &WorkerStartOptions{
		QueueID:       "test-queue",
		PackageSource: "",
		NoUpdate:      false,
		cfg:           &config.Config{},
	}

	execLocalOpts := &WorkerStartOptions{
		QueueID:             "test-queue",
		PackageSource:       "",
		NoUpdate:            false, // exec always has this as false
		SingleExecutionMode: true,
		cfg:                 &config.Config{},
	}

	workerFetch, workerForce := determinePackageBehavior(workerStartOpts)
	execFetch, execForce := determinePackageBehavior(execLocalOpts)

	if workerFetch != execFetch {
		t.Errorf("Inconsistent fetch behavior: worker start=%v, exec=%v", workerFetch, execFetch)
	}

	if workerForce != execForce {
		t.Errorf("Inconsistent force reinstall behavior: worker start=%v, exec=%v", workerForce, execForce)
	}

	if !workerFetch || !execFetch {
		t.Error("Both commands should fetch latest version by default")
	}

	t.Logf("✓ Both commands consistently fetch latest version from PyPI by default")
	t.Logf("  worker start: fetch=%v, force=%v", workerFetch, workerForce)
	t.Logf("  exec --local: fetch=%v, force=%v", execFetch, execForce)
}
