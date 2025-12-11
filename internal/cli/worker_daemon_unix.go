//go:build !windows
// +build !windows

package cli

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"syscall"

	"github.com/kubiyabot/cli/internal/output"
)

// Daemonize forks the current process into the background
// The parent process waits for the child to signal readiness before exiting
func Daemonize() error {
	// Check if we're already a daemon (detached from parent)
	if os.Getppid() == 1 {
		return nil // Already a daemon
	}

	// Get current executable path
	executable, err := os.Executable()
	if err != nil {
		return fmt.Errorf("failed to get executable path: %w", err)
	}

	// Create temporary socket for readiness signaling
	socketPath := filepath.Join(os.TempDir(), fmt.Sprintf("kubiya-daemon-%d.sock", os.Getpid()))

	// Create readiness client to wait for child
	client := NewReadinessClient(socketPath)
	defer client.Close()

	// Fork a new process
	cmd := exec.Command(executable, os.Args[1:]...)
	cmd.Env = append(os.Environ(),
		"KUBIYA_DAEMON_CHILD=1",
		fmt.Sprintf("KUBIYA_READINESS_SOCKET=%s", socketPath),
	)

	// Detach from parent
	cmd.SysProcAttr = &syscall.SysProcAttr{
		Setsid: true, // Create new session
	}

	// Start the child process
	if err := cmd.Start(); err != nil {
		return fmt.Errorf("failed to fork daemon process: %w", err)
	}

	// Create progress manager for status display
	pm := output.NewProgressManager()

	// Wait for child to signal readiness
	spinner := pm.Spinner("Waiting for daemon to start")
	spinner.Start()

	info, err := client.WaitForReady(ReadinessSocketTimeout)
	spinner.Stop()

	if err != nil {
		pm.Error(fmt.Sprintf("Daemon failed to start: %v", err))
		pm.Info(fmt.Sprintf("Child PID: %d (may have failed during startup)", cmd.Process.Pid))
		os.Exit(1)
	}

	// Success! Display daemon info
	pm.Success("Daemon started successfully")
	pm.Println("")
	pm.Printf("  PID:          %d\n", info.PID)
	pm.Printf("  Queue ID:     %s\n", info.QueueID)
	pm.Printf("  Worker Dir:   %s\n", info.WorkerDir)
	pm.Printf("  Started:      %s\n", info.StartTime.Format("2006-01-02 15:04:05"))
	pm.Println("")
	pm.Info(fmt.Sprintf("View logs: tail -f %s/worker.log", info.WorkerDir))
	pm.Println("")

	// Parent process exits successfully
	os.Exit(0)
	return nil
}
