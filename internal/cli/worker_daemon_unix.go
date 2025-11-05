//go:build !windows
// +build !windows

package cli

import (
	"fmt"
	"os"
	"os/exec"
	"syscall"
)

// Daemonize forks the current process into the background
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

	// Fork a new process
	cmd := exec.Command(executable, os.Args[1:]...)
	cmd.Env = append(os.Environ(), "KUBIYA_DAEMON_CHILD=1")

	// Detach from parent
	cmd.SysProcAttr = &syscall.SysProcAttr{
		Setsid: true, // Create new session
	}

	// Start the child process
	if err := cmd.Start(); err != nil {
		return fmt.Errorf("failed to fork daemon process: %w", err)
	}

	// Parent process exits
	os.Exit(0)
	return nil
}
