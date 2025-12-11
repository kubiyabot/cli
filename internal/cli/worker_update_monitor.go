package cli

import (
	"context"
	"fmt"
	"os"
	"os/exec"
	"strings"
	"time"

	"github.com/kubiyabot/cli/internal/controlplane"
	"github.com/kubiyabot/cli/internal/controlplane/entities"
)

// UpdateMonitor monitors for configuration and package updates
type UpdateMonitor struct {
	queueID                string
	workerID               string
	controlPlaneClient     *controlplane.Client
	currentConfigVersion   string
	currentPackageVersion  string
	checkInterval          time.Duration
	autoUpdateEnabled      bool
	updateChan             chan UpdateTrigger
	stopChan               chan struct{}
	debug                  bool
}

// UpdateTrigger represents an update event
type UpdateTrigger struct {
	Type              UpdateType
	NewConfigVersion  string
	NewPackageVersion string
	Config            *entities.WorkerQueueConfig
}

// UpdateType represents the type of update
type UpdateType int

const (
	UpdateTypeConfig  UpdateType = iota // Configuration changed
	UpdateTypePackage                   // Package version changed
	UpdateTypeBoth                      // Both changed
)

// NewUpdateMonitor creates a new update monitor
func NewUpdateMonitor(
	queueID string,
	workerID string,
	controlPlaneClient *controlplane.Client,
	autoUpdateEnabled bool,
	checkInterval time.Duration,
	debug bool,
) *UpdateMonitor {
	return &UpdateMonitor{
		queueID:            queueID,
		workerID:           workerID,
		controlPlaneClient: controlPlaneClient,
		checkInterval:      checkInterval,
		autoUpdateEnabled:  autoUpdateEnabled,
		updateChan:         make(chan UpdateTrigger, 1),
		stopChan:           make(chan struct{}),
		debug:              debug,
	}
}

// Start starts the update monitor goroutine
func (m *UpdateMonitor) Start(ctx context.Context) {
	if m.debug {
		fmt.Printf("[UpdateMonitor] Starting update monitor (interval: %s, auto-update: %v)\n",
			m.checkInterval, m.autoUpdateEnabled)
	}

	// Initial check to set current versions
	if err := m.performInitialCheck(); err != nil {
		if m.debug {
			fmt.Printf("[UpdateMonitor] Initial check failed: %v\n", err)
		}
	}

	go m.monitorLoop(ctx)
}

// Stop stops the update monitor
func (m *UpdateMonitor) Stop() {
	close(m.stopChan)
}

// UpdateChan returns the channel that receives update triggers
func (m *UpdateMonitor) UpdateChan() <-chan UpdateTrigger {
	return m.updateChan
}

// performInitialCheck performs initial version check to set baseline
func (m *UpdateMonitor) performInitialCheck() error {
	// Get current config and versions
	config, err := m.controlPlaneClient.GetWorkerQueueConfig(m.queueID)
	if err != nil {
		return fmt.Errorf("failed to fetch config: %w", err)
	}

	m.currentConfigVersion = config.ConfigVersion

	if config.RecommendedPackageVersion != nil && *config.RecommendedPackageVersion != "" {
		m.currentPackageVersion = *config.RecommendedPackageVersion
	}

	// Get currently installed package version
	installedVersion, err := m.getCurrentInstalledVersion()
	if err != nil {
		if m.debug {
			fmt.Printf("[UpdateMonitor] Could not determine installed version: %v\n", err)
		}
	} else {
		if m.debug {
			fmt.Printf("[UpdateMonitor] Installed package version: %s\n", installedVersion)
		}
	}

	if m.debug {
		fmt.Printf("[UpdateMonitor] Initial config version: %s\n", m.currentConfigVersion[:8])
		fmt.Printf("[UpdateMonitor] Recommended package version: %s\n", m.currentPackageVersion)
	}

	return nil
}

// monitorLoop is the main monitoring loop
func (m *UpdateMonitor) monitorLoop(ctx context.Context) {
	ticker := time.NewTicker(m.checkInterval)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			if m.debug {
				fmt.Println("[UpdateMonitor] Context cancelled, stopping monitor")
			}
			return
		case <-m.stopChan:
			if m.debug {
				fmt.Println("[UpdateMonitor] Stop signal received, stopping monitor")
			}
			return
		case <-ticker.C:
			if err := m.checkForUpdates(); err != nil {
				if m.debug {
					fmt.Printf("[UpdateMonitor] Update check failed: %v\n", err)
				}
			}
		}
	}
}

// checkForUpdates checks for configuration and package updates
func (m *UpdateMonitor) checkForUpdates() error {
	if m.debug {
		fmt.Println("[UpdateMonitor] Checking for updates...")
	}

	// Fetch latest configuration
	config, err := m.controlPlaneClient.GetWorkerQueueConfig(m.queueID)
	if err != nil {
		return fmt.Errorf("failed to fetch config: %w", err)
	}

	var updateType UpdateType
	configChanged := false
	packageChanged := false

	// Check if configuration changed
	if config.ConfigVersion != m.currentConfigVersion {
		if m.debug {
			fmt.Printf("[UpdateMonitor] Configuration changed: %s -> %s\n",
				m.currentConfigVersion[:8], config.ConfigVersion[:8])
		}
		configChanged = true
	}

	// Check if package version changed
	if config.RecommendedPackageVersion != nil && *config.RecommendedPackageVersion != "" {
		recommendedVersion := *config.RecommendedPackageVersion
		if recommendedVersion != m.currentPackageVersion {
			if m.debug {
				fmt.Printf("[UpdateMonitor] Package version changed: %s -> %s\n",
					m.currentPackageVersion, recommendedVersion)
			}
			packageChanged = true
		}
	}

	// Determine update type
	if configChanged && packageChanged {
		updateType = UpdateTypeBoth
	} else if configChanged {
		updateType = UpdateTypeConfig
	} else if packageChanged {
		updateType = UpdateTypePackage
	} else {
		// No changes
		if m.debug {
			fmt.Println("[UpdateMonitor] No updates available")
		}
		return nil
	}

	// Only trigger update if auto-update is enabled
	if !m.autoUpdateEnabled {
		if m.debug {
			fmt.Println("[UpdateMonitor] Updates available but auto-update is disabled")
		}
		return nil
	}

	// Send update trigger
	trigger := UpdateTrigger{
		Type:              updateType,
		NewConfigVersion:  config.ConfigVersion,
		NewPackageVersion: "",
		Config:            config,
	}

	if config.RecommendedPackageVersion != nil {
		trigger.NewPackageVersion = *config.RecommendedPackageVersion
	}

	select {
	case m.updateChan <- trigger:
		if m.debug {
			fmt.Printf("[UpdateMonitor] Update trigger sent (type: %d)\n", updateType)
		}
		// Update current versions after triggering
		m.currentConfigVersion = config.ConfigVersion
		if trigger.NewPackageVersion != "" {
			m.currentPackageVersion = trigger.NewPackageVersion
		}
	default:
		// Channel full, update already in progress
		if m.debug {
			fmt.Println("[UpdateMonitor] Update already in progress, skipping")
		}
	}

	return nil
}

// getCurrentInstalledVersion attempts to get the currently installed package version
func (m *UpdateMonitor) getCurrentInstalledVersion() (string, error) {
	// Try to get version from pip show
	cmd := exec.Command("pip", "show", "kubiya-control-plane-api")
	output, err := cmd.Output()
	if err != nil {
		return "", fmt.Errorf("pip show failed: %w", err)
	}

	// Parse version from output
	lines := strings.Split(string(output), "\n")
	for _, line := range lines {
		if strings.HasPrefix(line, "Version:") {
			version := strings.TrimSpace(strings.TrimPrefix(line, "Version:"))
			return version, nil
		}
	}

	return "", fmt.Errorf("version not found in pip show output")
}

// GetUpdateCheckIntervalFromEnv gets the update check interval from environment
func GetUpdateCheckIntervalFromEnv() time.Duration {
	intervalStr := os.Getenv("KUBIYA_WORKER_UPDATE_CHECK_INTERVAL")
	if intervalStr == "" {
		return 5 * time.Minute // Default: 5 minutes
	}

	duration, err := time.ParseDuration(intervalStr)
	if err != nil {
		fmt.Printf("[WARN] Invalid KUBIYA_WORKER_UPDATE_CHECK_INTERVAL: %v, using default\n", err)
		return 5 * time.Minute
	}

	// Minimum 1 minute, maximum 1 hour
	if duration < time.Minute {
		duration = time.Minute
	} else if duration > time.Hour {
		duration = time.Hour
	}

	return duration
}

// IsAutoUpdateEnabled checks if auto-update is enabled via environment or flag
func IsAutoUpdateEnabled() bool {
	// Check environment variable
	env := os.Getenv("KUBIYA_WORKER_AUTOUPDATE_ENABLED")
	if env != "" {
		return env == "true" || env == "1" || env == "yes"
	}

	return false // Default: disabled
}
