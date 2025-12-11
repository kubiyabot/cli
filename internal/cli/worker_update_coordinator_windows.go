//go:build windows

package cli

import (
	"fmt"
	"os"
)

// sendTermSignal sends a termination signal on Windows
// Windows doesn't support SIGTERM, so we kill the process
func sendTermSignal(pid int) error {
	process, err := os.FindProcess(pid)
	if err != nil {
		return fmt.Errorf("failed to find process: %w", err)
	}

	if err := process.Kill(); err != nil {
		return fmt.Errorf("failed to kill process: %w", err)
	}
	return nil
}

// checkProcessExists checks if a process exists on Windows
func checkProcessExists(process *os.Process) error {
	// On Windows, we try to get the process exit code
	// If the process is still running, this will return an error
	_, err := process.Wait()
	if err == nil {
		return fmt.Errorf("worker process has exited")
	}
	// If we get an error, the process might still be running
	// This is a limitation of the Windows API through Go
	return nil
}
