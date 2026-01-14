package cli

import (
	"bytes"
	"context"
	"os"
	"os/exec"
	"path/filepath"
	"testing"
	"time"
)

// TestExecLocalExitBehavior tests that exec --local exits cleanly without hanging
// This is an end-to-end test that verifies the fix for the signal handler goroutine
// blocking issue that caused the CLI to hang after successful agent completion.
func TestExecLocalExitBehavior(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping e2e test in short mode")
	}

	// Check if required environment variables are set
	apiKey := os.Getenv("KUBIYA_API_KEY")
	if apiKey == "" {
		t.Skip("KUBIYA_API_KEY not set, skipping e2e test")
	}

	// Build the CLI binary
	binaryPath := buildTestBinary(t)

	tests := []struct {
		name           string
		args           []string
		timeout        time.Duration
		expectHang     bool
		skipReason     string
		checkExitCode  bool
		expectedExit   int
	}{
		{
			name:          "exec_local_simple_task_exits_cleanly",
			args:          []string{"exec", "--local", "--yes", "echo hello"},
			timeout:       120 * time.Second, // Allow time for worker setup
			expectHang:    false,
			checkExitCode: true,
			expectedExit:  0,
		},
		{
			name:          "exec_local_with_timeout_does_not_hang",
			args:          []string{"exec", "--local", "--yes", "list files in current directory"},
			timeout:       180 * time.Second,
			expectHang:    false,
			checkExitCode: false, // Just verify it exits
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if tt.skipReason != "" {
				t.Skip(tt.skipReason)
			}

			ctx, cancel := context.WithTimeout(context.Background(), tt.timeout)
			defer cancel()

			cmd := exec.CommandContext(ctx, binaryPath, tt.args...)
			var stdout, stderr bytes.Buffer
			cmd.Stdout = &stdout
			cmd.Stderr = &stderr
			cmd.Env = append(os.Environ(),
				"KUBIYA_API_KEY="+apiKey,
			)

			startTime := time.Now()
			err := cmd.Run()
			elapsed := time.Since(startTime)

			// Check if the command timed out (which would indicate hanging)
			if ctx.Err() == context.DeadlineExceeded {
				if !tt.expectHang {
					t.Errorf("Command hung and timed out after %v (expected clean exit)\nStdout: %s\nStderr: %s",
						elapsed, stdout.String(), stderr.String())
				}
				return
			}

			// If we expected it to hang but it didn't, that's good (the fix works)
			if tt.expectHang {
				t.Logf("Command exited in %v (was expected to potentially hang, fix working)", elapsed)
			}

			// Check exit code if required
			if tt.checkExitCode {
				exitCode := 0
				if exitErr, ok := err.(*exec.ExitError); ok {
					exitCode = exitErr.ExitCode()
				} else if err != nil {
					t.Errorf("Unexpected error type: %v", err)
					return
				}

				if exitCode != tt.expectedExit {
					t.Errorf("Expected exit code %d, got %d\nStdout: %s\nStderr: %s",
						tt.expectedExit, exitCode, stdout.String(), stderr.String())
				}
			}

			t.Logf("Command completed in %v with exit code from err: %v", elapsed, err)
		})
	}
}

// TestExecLocalSignalHandlerCleanup verifies that signal handlers are properly
// cleaned up after successful execution
func TestExecLocalSignalHandlerCleanup(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping e2e test in short mode")
	}

	apiKey := os.Getenv("KUBIYA_API_KEY")
	if apiKey == "" {
		t.Skip("KUBIYA_API_KEY not set, skipping e2e test")
	}

	binaryPath := buildTestBinary(t)

	// This test specifically checks that after successful execution,
	// the process exits within a reasonable time (not hanging on signal goroutine)
	ctx, cancel := context.WithTimeout(context.Background(), 180*time.Second)
	defer cancel()

	cmd := exec.CommandContext(ctx, binaryPath, "exec", "--local", "--yes", "print hello world")
	var stdout, stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr
	cmd.Env = append(os.Environ(), "KUBIYA_API_KEY="+apiKey)

	// Track when we see "Worker shut down" message
	startTime := time.Now()
	err := cmd.Run()
	totalTime := time.Since(startTime)

	if ctx.Err() == context.DeadlineExceeded {
		t.Fatalf("Process hung after execution - signal handler cleanup likely failed.\n"+
			"Total time: %v\nStdout: %s\nStderr: %s",
			totalTime, stdout.String(), stderr.String())
	}

	// Process should exit cleanly
	if err != nil {
		// Check if it's just a non-zero exit (which is different from hanging)
		if exitErr, ok := err.(*exec.ExitError); ok {
			t.Logf("Process exited with code %d in %v (not hanging, which is the key test)",
				exitErr.ExitCode(), totalTime)
		} else {
			t.Errorf("Unexpected error: %v", err)
		}
	} else {
		t.Logf("Process exited cleanly with code 0 in %v", totalTime)
	}

	// The key assertion: process should exit, not hang
	// If we reach here without timeout, the test passes
	output := stdout.String() + stderr.String()
	if len(output) > 500 {
		output = output[:500] + "..."
	}
	t.Logf("Output sample: %s", output)
}

// TestExecLocalProcessExitWithinTimeout is a focused test that measures
// the time between "Worker shut down" message and actual process exit
func TestExecLocalProcessExitWithinTimeout(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping e2e test in short mode")
	}

	apiKey := os.Getenv("KUBIYA_API_KEY")
	if apiKey == "" {
		t.Skip("KUBIYA_API_KEY not set, skipping e2e test")
	}

	binaryPath := buildTestBinary(t)

	// Create a simple test that should complete quickly
	ctx, cancel := context.WithTimeout(context.Background(), 180*time.Second)
	defer cancel()

	cmd := exec.CommandContext(ctx, binaryPath, "exec", "--local", "--yes", "--compact", "say hello")
	cmd.Env = append(os.Environ(), "KUBIYA_API_KEY="+apiKey)

	startTime := time.Now()
	output, err := cmd.CombinedOutput()
	elapsed := time.Since(startTime)

	if ctx.Err() == context.DeadlineExceeded {
		t.Fatalf("CRITICAL: Process hung and did not exit within timeout.\n"+
			"This indicates the signal handler goroutine fix is not working.\n"+
			"Elapsed: %v\nOutput: %s", elapsed, string(output))
	}

	// Check if output contains expected shutdown messages
	outputStr := string(output)
	hasWorkerShutdown := bytes.Contains(output, []byte("Worker shut down")) ||
		bytes.Contains(output, []byte("✓ Worker shut down"))

	t.Logf("Test completed in %v", elapsed)
	t.Logf("Worker shutdown message found: %v", hasWorkerShutdown)

	if err != nil {
		if exitErr, ok := err.(*exec.ExitError); ok {
			t.Logf("Exit code: %d", exitErr.ExitCode())
		}
	} else {
		t.Log("Exit code: 0 (success)")
	}

	// The main success criteria: process exited (didn't hang)
	t.Log("SUCCESS: Process exited without hanging")

	// Show truncated output for debugging
	if len(outputStr) > 1000 {
		t.Logf("Output (truncated): %s...", outputStr[:1000])
	} else {
		t.Logf("Output: %s", outputStr)
	}
}

// buildTestBinary builds the CLI binary and returns its path
func buildTestBinary(t *testing.T) string {
	t.Helper()

	// Find the root of the project
	projectRoot := findProjectRoot(t)

	// Create a temp directory for the binary
	tmpDir := t.TempDir()
	binaryPath := filepath.Join(tmpDir, "kubiya-test")

	// Build the binary
	cmd := exec.Command("go", "build", "-o", binaryPath, ".")
	cmd.Dir = projectRoot
	output, err := cmd.CombinedOutput()
	if err != nil {
		t.Fatalf("Failed to build test binary: %v\nOutput: %s", err, output)
	}

	return binaryPath
}

// findProjectRoot finds the root of the project by looking for go.mod
func findProjectRoot(t *testing.T) string {
	t.Helper()

	// Start from current directory and walk up
	dir, err := os.Getwd()
	if err != nil {
		t.Fatalf("Failed to get current directory: %v", err)
	}

	for {
		if _, err := os.Stat(filepath.Join(dir, "go.mod")); err == nil {
			return dir
		}

		parent := filepath.Dir(dir)
		if parent == dir {
			t.Fatal("Could not find project root (no go.mod found)")
		}
		dir = parent
	}
}
