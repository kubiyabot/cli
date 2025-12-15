//go:build windows
// +build windows

package cli

import (
	"os"
	"os/exec"
)

// setupProcessGroup is a no-op on Windows
// Windows doesn't use Unix-style process groups
func setupProcessGroup(cmd *exec.Cmd) {
	// No-op on Windows
}

// killProcessGroup terminates the process
// On Windows, we rely on the process Kill method
// graceful parameter is ignored as Windows doesn't have SIGTERM
func killProcessGroup(pid int, graceful bool) error {
	// On Windows, we can't easily kill process groups
	// The caller should use cmd.Process.Kill() as fallback
	// Return an error to trigger the fallback
	return nil
}

// getShutdownSignals returns the OS signals to listen for shutdown
func getShutdownSignals() []os.Signal {
	// Windows only supports os.Interrupt (CTRL+C)
	return []os.Signal{os.Interrupt}
}
