package cli

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"strings"
	"syscall"
	"time"

	"github.com/kubiyabot/cli/internal/controlplane"
	"github.com/kubiyabot/cli/internal/controlplane/entities"
)

// UpdateCoordinator coordinates rolling updates across multiple workers
type UpdateCoordinator struct {
	queueID            string
	workerID           string
	controlPlaneClient *controlplane.Client
	workerDir          string
	packageVersion     string
	debug              bool
}

// NewUpdateCoordinator creates a new update coordinator
func NewUpdateCoordinator(
	queueID string,
	workerID string,
	controlPlaneClient *controlplane.Client,
	workerDir string,
	debug bool,
) *UpdateCoordinator {
	return &UpdateCoordinator{
		queueID:            queueID,
		workerID:           workerID,
		controlPlaneClient: controlPlaneClient,
		workerDir:          workerDir,
		debug:              debug,
	}
}

// CoordinateUpdate coordinates an update with rolling update strategy
func (c *UpdateCoordinator) CoordinateUpdate(ctx context.Context, trigger UpdateTrigger) error {
	if c.debug {
		fmt.Printf("[UpdateCoordinator] Coordinating update (type: %d)\n", trigger.Type)
	}

	// Step 1: Try to acquire update lock
	lock, err := c.acquireUpdateLock(ctx)
	if err != nil {
		return fmt.Errorf("failed to acquire update lock: %w", err)
	}

	if c.debug {
		fmt.Printf("[UpdateCoordinator] Acquired update lock: %s\n", lock.LockID)
	}

	// Ensure lock is released on exit
	defer func() {
		if err := c.releaseUpdateLock(); err != nil && c.debug {
			fmt.Printf("[UpdateCoordinator] Failed to release lock: %v\n", err)
		}
	}()

	// Step 2: Perform the update
	if err := c.performUpdate(ctx, trigger); err != nil {
		return fmt.Errorf("update failed: %w", err)
	}

	if c.debug {
		fmt.Println("[UpdateCoordinator] Update completed successfully")
	}

	return nil
}

// acquireUpdateLock tries to acquire the update lock with retries
func (c *UpdateCoordinator) acquireUpdateLock(ctx context.Context) (*entities.UpdateLock, error) {
	maxRetries := 10
	retryDelay := 30 * time.Second // Wait 30s between retries
	lockDuration := 300              // 5 minutes

	for attempt := 1; attempt <= maxRetries; attempt++ {
		if c.debug {
			fmt.Printf("[UpdateCoordinator] Attempting to acquire lock (attempt %d/%d)\n",
				attempt, maxRetries)
		}

		lock, err := c.controlPlaneClient.AcquireUpdateLock(c.queueID, c.workerID, lockDuration)
		if err == nil {
			return lock, nil
		}

		// Check if it's a conflict (another worker holds the lock)
		if strings.Contains(err.Error(), "409") || strings.Contains(err.Error(), "conflict") {
			if c.debug {
				fmt.Printf("[UpdateCoordinator] Lock held by another worker, waiting %s\n", retryDelay)
			}

			// Wait before retrying
			select {
			case <-ctx.Done():
				return nil, fmt.Errorf("context cancelled while waiting for lock")
			case <-time.After(retryDelay):
				// Continue to next attempt
			}
		} else {
			// Other error, fail immediately
			return nil, fmt.Errorf("failed to acquire lock: %w", err)
		}
	}

	return nil, fmt.Errorf("failed to acquire lock after %d attempts", maxRetries)
}

// releaseUpdateLock releases the update lock
func (c *UpdateCoordinator) releaseUpdateLock() error {
	if c.debug {
		fmt.Println("[UpdateCoordinator] Releasing update lock")
	}

	return c.controlPlaneClient.ReleaseUpdateLock(c.queueID, c.workerID)
}

// performUpdate performs the actual update
func (c *UpdateCoordinator) performUpdate(ctx context.Context, trigger UpdateTrigger) error {
	switch trigger.Type {
	case UpdateTypeConfig:
		return c.performConfigUpdate(ctx)
	case UpdateTypePackage:
		return c.performPackageUpdate(ctx, trigger.NewPackageVersion)
	case UpdateTypeBoth:
		// Update package first, then config will be reloaded automatically
		return c.performPackageUpdate(ctx, trigger.NewPackageVersion)
	default:
		return fmt.Errorf("unknown update type: %d", trigger.Type)
	}
}

// performConfigUpdate performs a configuration update (graceful restart)
func (c *UpdateCoordinator) performConfigUpdate(ctx context.Context) error {
	if c.debug {
		fmt.Println("[UpdateCoordinator] Performing configuration update (graceful restart)")
	}

	// For configuration updates, we just need to restart the worker process
	// Send SIGTERM to self (current process) for graceful shutdown
	// The supervisor/daemon will restart us automatically
	pid := os.Getpid()

	if c.debug {
		fmt.Printf("[UpdateCoordinator] Sending SIGTERM to PID %d for graceful restart\n", pid)
	}

	// Signal the process to shutdown gracefully
	if err := syscall.Kill(pid, syscall.SIGTERM); err != nil {
		return fmt.Errorf("failed to send SIGTERM: %w", err)
	}

	return nil
}

// performPackageUpdate performs a package update (upgrade Python package)
func (c *UpdateCoordinator) performPackageUpdate(ctx context.Context, newVersion string) error {
	if c.debug {
		fmt.Printf("[UpdateCoordinator] Performing package update to version: %s\n", newVersion)
	}

	// Step 1: Upgrade the package in the virtual environment
	venvPath := fmt.Sprintf("%s/venv", c.workerDir)
	pipPath := fmt.Sprintf("%s/bin/pip", venvPath)

	// Upgrade package
	packageSpec := fmt.Sprintf("kubiya-control-plane-api[worker]==%s", newVersion)
	cmd := exec.CommandContext(ctx, pipPath, "install", "--upgrade", packageSpec)
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr

	if c.debug {
		fmt.Printf("[UpdateCoordinator] Running: %s\n", cmd.String())
	}

	if err := cmd.Run(); err != nil {
		return fmt.Errorf("failed to upgrade package: %w", err)
	}

	if c.debug {
		fmt.Printf("[UpdateCoordinator] Package upgraded to %s successfully\n", newVersion)
	}

	// Step 2: Restart worker process (same as config update)
	return c.performConfigUpdate(ctx)
}

// CheckHealth checks if the worker is healthy after update
func (c *UpdateCoordinator) CheckHealth(ctx context.Context, maxWait time.Duration) error {
	if c.debug {
		fmt.Printf("[UpdateCoordinator] Checking worker health (max wait: %s)\n", maxWait)
	}

	// Read PID file to get worker PID
	pidFilePath := fmt.Sprintf("%s/worker.pid", c.workerDir)
	pidFile, err := os.ReadFile(pidFilePath)
	if err != nil {
		return fmt.Errorf("failed to read PID file: %w", err)
	}

	// Parse PID
	var daemonInfo DaemonInfo
	if err := json.Unmarshal(pidFile, &daemonInfo); err != nil {
		return fmt.Errorf("failed to parse PID file: %w", err)
	}

	// Check if process is running
	process, err := os.FindProcess(daemonInfo.PID)
	if err != nil {
		return fmt.Errorf("failed to find process: %w", err)
	}

	// Send signal 0 to check if process exists
	if err := process.Signal(syscall.Signal(0)); err != nil {
		return fmt.Errorf("worker process not running: %w", err)
	}

	// Check control plane heartbeat
	// Workers should send heartbeats within 60 seconds of starting
	ticker := time.NewTicker(5 * time.Second)
	defer ticker.Stop()

	deadline := time.Now().Add(maxWait)

	for {
		select {
		case <-ctx.Done():
			return fmt.Errorf("context cancelled during health check")
		case <-ticker.C:
			if time.Now().After(deadline) {
				return fmt.Errorf("health check timeout after %s", maxWait)
			}

			// Check if worker is sending heartbeats by querying control plane
			workers, err := c.controlPlaneClient.ListQueueWorkers(c.queueID)
			if err != nil {
				if c.debug {
					fmt.Printf("[UpdateCoordinator] Failed to list workers: %v\n", err)
				}
				continue
			}

			// Find our worker in the list
			for _, worker := range workers {
				if worker.ID == c.workerID && worker.Status == "active" {
					if c.debug {
						fmt.Printf("[UpdateCoordinator] Worker is healthy (status: %s)\n", worker.Status)
					}
					return nil
				}
			}
		}
	}
}
