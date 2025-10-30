package cli

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/kubiyabot/cli/internal/config"
	"github.com/spf13/cobra"
)

type WorkerStopOptions struct {
	QueueID string
	Force   bool
	cfg     *config.Config
}

func newWorkerStopCommand(cfg *config.Config) *cobra.Command {
	opts := &WorkerStopOptions{
		cfg: cfg,
	}

	cmd := &cobra.Command{
		Use:   "stop",
		Short: "🛑 Stop a running worker",
		Long:  `Stop a running worker daemon gracefully`,
		Example: `  # Stop worker gracefully
  kubiya worker stop --queue-id=<queue-id>

  # Force stop worker
  kubiya worker stop --queue-id=<queue-id> --force`,
		RunE: func(cmd *cobra.Command, args []string) error {
			return opts.Run(cmd.Context())
		},
	}

	cmd.Flags().StringVar(&opts.QueueID, "queue-id", "", "Worker queue ID (required)")
	cmd.Flags().BoolVarP(&opts.Force, "force", "f", false, "Force stop the worker (SIGKILL)")
	cmd.MarkFlagRequired("queue-id")

	return cmd
}

func (opts *WorkerStopOptions) Run(ctx context.Context) error {
	homeDir, err := os.UserHomeDir()
	if err != nil {
		return fmt.Errorf("failed to get home directory: %w", err)
	}

	workerDir := filepath.Join(homeDir, ".kubiya", "workers", opts.QueueID)
	pidFile := filepath.Join(workerDir, "worker.pid")

	// Check if PID file exists
	if _, err := os.Stat(pidFile); os.IsNotExist(err) {
		return fmt.Errorf("❌ No worker found for queue ID: %s\n   (PID file not found: %s)", opts.QueueID, pidFile)
	}

	// Read daemon info
	info, err := ReadDaemonInfo(pidFile)
	if err != nil {
		return fmt.Errorf("❌ Failed to read worker info: %w", err)
	}

	// Check if process is running
	if !IsProcessRunning(info.PID) {
		fmt.Printf("⚠️  Worker process (PID %d) is not running\n", info.PID)
		fmt.Printf("   Cleaning up stale PID file: %s\n", pidFile)
		os.Remove(pidFile)
		return nil
	}

	// Print stopping banner
	fmt.Println()
	fmt.Println(strings.Repeat("═", 80))
	fmt.Println("🛑  STOPPING WORKER")
	fmt.Println(strings.Repeat("═", 80))
	fmt.Println()
	fmt.Printf("   Queue ID:    %s\n", info.QueueID)
	fmt.Printf("   Process ID:  %d\n", info.PID)
	if opts.Force {
		fmt.Printf("   Method:      Force stop (SIGKILL)\n")
	} else {
		fmt.Printf("   Method:      Graceful shutdown (SIGTERM)\n")
	}
	fmt.Println()
	fmt.Println(strings.Repeat("─", 80))
	fmt.Println()

	// Stop the daemon
	if opts.Force {
		// Force kill
		process, err := os.FindProcess(info.PID)
		if err != nil {
			return fmt.Errorf("failed to find process: %w", err)
		}

		if err := process.Kill(); err != nil {
			return fmt.Errorf("failed to kill process: %w", err)
		}

		// Clean up PID file
		os.Remove(pidFile)
		fmt.Println("✅ Worker force stopped successfully")
	} else {
		// Graceful stop
		if err := StopDaemon(pidFile); err != nil {
			return fmt.Errorf("❌ Failed to stop worker: %w", err)
		}
		fmt.Println("✅ Worker stopped gracefully")
	}

	fmt.Println()
	fmt.Println(strings.Repeat("═", 80))
	fmt.Println()

	return nil
}
