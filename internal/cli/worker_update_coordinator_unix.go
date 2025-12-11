//go:build unix

package cli

import (
	"fmt"
	"os"
	"syscall"
)

// sendTermSignal sends a SIGTERM signal to the given PID for graceful shutdown
func sendTermSignal(pid int) error {
	if err := syscall.Kill(pid, syscall.SIGTERM); err != nil {
		return fmt.Errorf("failed to send SIGTERM: %w", err)
	}
	return nil
}

// checkProcessExists checks if a process exists by sending signal 0
func checkProcessExists(process *os.Process) error {
	if err := process.Signal(syscall.Signal(0)); err != nil {
		return fmt.Errorf("worker process not running: %w", err)
	}
	return nil
}
