package cli

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/kubiyabot/cli/internal/config"
	"github.com/spf13/cobra"
)

type WorkerStatusOptions struct {
	QueueID string
	cfg     *config.Config
}

func newWorkerStatusCommand(cfg *config.Config) *cobra.Command {
	opts := &WorkerStatusOptions{
		cfg: cfg,
	}

	cmd := &cobra.Command{
		Use:   "status",
		Short: "📊 Check worker status",
		Long:  `Check the status of a running worker daemon`,
		Example: `  # Check worker status
  kubiya worker status --queue-id=<queue-id>`,
		RunE: func(cmd *cobra.Command, args []string) error {
			return opts.Run(cmd.Context())
		},
	}

	cmd.Flags().StringVar(&opts.QueueID, "queue-id", "", "Worker queue ID (required)")
	cmd.MarkFlagRequired("queue-id")

	return cmd
}

func (opts *WorkerStatusOptions) Run(ctx context.Context) error {
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
	running := IsProcessRunning(info.PID)

	// Print status banner
	fmt.Println()
	fmt.Println(strings.Repeat("═", 80))
	fmt.Println("📊  WORKER STATUS")
	fmt.Println(strings.Repeat("═", 80))
	fmt.Println()

	// Status indicator
	if running {
		fmt.Println("✅ Status:           RUNNING")
	} else {
		fmt.Println("❌ Status:           STOPPED (stale PID file)")
	}
	fmt.Println()

	// Worker information
	fmt.Println("📋 WORKER INFORMATION")
	fmt.Println(strings.Repeat("─", 80))
	fmt.Printf("   Queue ID:         %s\n", info.QueueID)
	fmt.Printf("   Process ID:       %d\n", info.PID)
	fmt.Printf("   Deployment Type:  %s\n", info.DeploymentType)
	fmt.Printf("   Worker Directory: %s\n", info.WorkerDir)
	fmt.Printf("   Log File:         %s\n", info.LogFile)
	fmt.Printf("   Started At:       %s\n", info.StartedAt.Format("2006-01-02 15:04:05"))

	// Calculate uptime
	if running {
		uptime := time.Since(info.StartedAt)
		fmt.Printf("   Uptime:           %s\n", formatDuration(uptime))
	}
	fmt.Println()

	// Log file information
	if logInfo, err := os.Stat(info.LogFile); err == nil {
		fmt.Println("📁 LOG FILE")
		fmt.Println(strings.Repeat("─", 80))
		fmt.Printf("   Size:             %s\n", formatBytes(logInfo.Size()))
		fmt.Printf("   Last Modified:    %s\n", logInfo.ModTime().Format("2006-01-02 15:04:05"))
		fmt.Println()
	}

	// Management commands
	fmt.Println("📊 MANAGEMENT COMMANDS")
	fmt.Println(strings.Repeat("─", 80))
	fmt.Printf("   View logs:        tail -f %s\n", info.LogFile)
	if running {
		fmt.Printf("   Stop worker:      kubiya worker stop --queue-id=%s\n", opts.QueueID)
	} else {
		fmt.Printf("   Clean PID file:   rm %s\n", pidFile)
	}
	fmt.Println()
	fmt.Println(strings.Repeat("═", 80))
	fmt.Println()

	return nil
}

func formatDuration(d time.Duration) string {
	days := int(d.Hours() / 24)
	hours := int(d.Hours()) % 24
	minutes := int(d.Minutes()) % 60
	seconds := int(d.Seconds()) % 60

	if days > 0 {
		return fmt.Sprintf("%dd %dh %dm %ds", days, hours, minutes, seconds)
	} else if hours > 0 {
		return fmt.Sprintf("%dh %dm %ds", hours, minutes, seconds)
	} else if minutes > 0 {
		return fmt.Sprintf("%dm %ds", minutes, seconds)
	}
	return fmt.Sprintf("%ds", seconds)
}

func formatBytes(bytes int64) string {
	const unit = 1024
	if bytes < unit {
		return fmt.Sprintf("%d B", bytes)
	}
	div, exp := int64(unit), 0
	for n := bytes / unit; n >= unit; n /= unit {
		div *= unit
		exp++
	}
	return fmt.Sprintf("%.1f %cB", float64(bytes)/float64(div), "KMGTPE"[exp])
}
