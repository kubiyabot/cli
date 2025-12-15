//go:build !windows
// +build !windows

package cli

import (
	"os"
	"os/exec"
	"syscall"
)

// setupProcessGroup configures the command to create a new process group
// This allows killing all child processes together
func setupProcessGroup(cmd *exec.Cmd) {
	cmd.SysProcAttr = &syscall.SysProcAttr{
		Setpgid: true, // Create new process group
	}
}

// killProcessGroup terminates the process and all its children
// If graceful is true, sends SIGTERM; otherwise sends SIGKILL
func killProcessGroup(pid int, graceful bool) error {
	signal := syscall.SIGKILL
	if graceful {
		signal = syscall.SIGTERM
	}
	// Negative PID kills the entire process group
	return syscall.Kill(-pid, signal)
}

// getShutdownSignals returns the OS signals to listen for shutdown
func getShutdownSignals() []os.Signal {
	return []os.Signal{os.Interrupt, syscall.SIGTERM}
}
