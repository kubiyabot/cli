//go:build windows
// +build windows

package cli

import (
	"fmt"
	"os"
	"os/exec"
	"syscall"
)

const (
	// Windows process creation flags
	CREATE_NEW_PROCESS_GROUP uint32 = 0x00000200
	DETACHED_PROCESS         uint32 = 0x00000008
)

// Daemonize forks the current process into the background
func Daemonize() error {
	// Check if we're already running as daemon child
	if IsDaemonChild() {
		return nil
	}

	// Get current executable path
	executable, err := os.Executable()
	if err != nil {
		return fmt.Errorf("failed to get executable path: %w", err)
	}

	// Create a new process detached from current console
	cmd := exec.Command(executable, os.Args[1:]...)
	cmd.Env = append(os.Environ(), "KUBIYA_DAEMON_CHILD=1")

	// Windows-specific detachment
	cmd.SysProcAttr = &syscall.SysProcAttr{
		CreationFlags: CREATE_NEW_PROCESS_GROUP | DETACHED_PROCESS,
	}

	// Start the child process
	if err := cmd.Start(); err != nil {
		return fmt.Errorf("failed to fork daemon process: %w", err)
	}

	// Parent process exits
	os.Exit(0)
	return nil
}
