package cli

import (
	"context"
	"fmt"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/kubiyabot/cli/internal/config"
	"github.com/kubiyabot/cli/internal/controlplane"
	"github.com/kubiyabot/cli/internal/controlplane/entities"
)

// OperatorController manages worker deployments in Kubernetes
type OperatorController struct {
	cfg                 *config.Config
	k8sClient           *K8sClient
	controlPlaneClient  *controlplane.Client
	namespace           string
	reconcileInterval   time.Duration
	enableAutoUpdate    bool
	updateCheckInterval string

	// State tracking
	managedQueues map[string]*entities.WorkerQueueConfig // queue_id -> config
}

// NewOperatorController creates a new operator controller
func NewOperatorController(
	cfg *config.Config,
	k8sClient *K8sClient,
	namespace string,
	reconcileIntervalStr string,
	enableAutoUpdate bool,
	updateCheckInterval string,
) (*OperatorController, error) {
	// Parse reconcile interval
	reconcileInterval, err := time.ParseDuration(reconcileIntervalStr)
	if err != nil {
		return nil, fmt.Errorf("invalid reconcile interval: %w", err)
	}

	// Create control plane client
	controlPlaneClient, err := controlplane.New(cfg.APIKey, false)
	if err != nil {
		return nil, fmt.Errorf("failed to create control plane client: %w", err)
	}

	return &OperatorController{
		cfg:                 cfg,
		k8sClient:           k8sClient,
		controlPlaneClient:  controlPlaneClient,
		namespace:           namespace,
		reconcileInterval:   reconcileInterval,
		enableAutoUpdate:    enableAutoUpdate,
		updateCheckInterval: updateCheckInterval,
		managedQueues:       make(map[string]*entities.WorkerQueueConfig),
	}, nil
}

// Run starts the operator reconciliation loop
func (c *OperatorController) Run(ctx context.Context) error {
	// Setup signal handling
	sigChan := make(chan os.Signal, 1)
	signal.Notify(sigChan, os.Interrupt, syscall.SIGTERM)

	// Create context that cancels on signal
	ctx, cancel := context.WithCancel(ctx)
	defer cancel()

	go func() {
		<-sigChan
		fmt.Println()
		fmt.Println("───────────────────────────────────────────────────────────────────────────")
		fmt.Println("🛑  SHUTTING DOWN OPERATOR")
		fmt.Println("───────────────────────────────────────────────────────────────────────────")
		cancel()
	}()

	// Initial reconciliation
	fmt.Println("🔄 Performing initial reconciliation...")
	if err := c.reconcile(ctx); err != nil {
		fmt.Printf("⚠️  Initial reconciliation failed: %v\n", err)
	} else {
		fmt.Println("✓ Initial reconciliation complete")
	}
	fmt.Println()

	// Start reconciliation loop
	ticker := time.NewTicker(c.reconcileInterval)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			fmt.Println("✓ Operator stopped gracefully")
			return nil
		case <-ticker.C:
			if err := c.reconcile(ctx); err != nil {
				fmt.Printf("⚠️  Reconciliation error: %v\n", err)
			}
		}
	}
}

// reconcile performs a single reconciliation cycle
func (c *OperatorController) reconcile(ctx context.Context) error {
	fmt.Printf("[%s] Reconciling...\n", time.Now().Format("15:04:05"))

	// Step 1: Fetch all worker queues from control plane
	queues, err := c.controlPlaneClient.ListWorkerQueues()
	if err != nil {
		return fmt.Errorf("failed to list worker queues: %w", err)
	}

	fmt.Printf("  • Found %d worker queues in control plane\n", len(queues))

	// Step 2: Fetch detailed config for each queue
	queueConfigs := make(map[string]*entities.WorkerQueueConfig)
	for _, queue := range queues {
		if queue.ID == "" {
			continue
		}

		config, err := c.controlPlaneClient.GetWorkerQueueConfig(queue.ID)
		if err != nil {
			fmt.Printf("  ⚠️  Failed to get config for queue %s: %v\n", queue.Name, err)
			continue
		}

		// Only manage active queues
		if config.Status == "active" {
			queueConfigs[config.QueueID] = config
		}
	}

	// Step 3: Get existing deployments managed by operator
	existingDeployments, err := c.k8sClient.ListDeploymentsByLabel(
		ctx,
		c.namespace,
		"app.kubernetes.io/managed-by",
		"kubiya-operator",
	)
	if err != nil {
		return fmt.Errorf("failed to list existing deployments: %w", err)
	}

	existingQueues := make(map[string]bool)
	for _, deployment := range existingDeployments {
		queueID := deployment.Labels["kubiya.ai/queue-id"]
		if queueID != "" {
			existingQueues[queueID] = true
		}
	}

	// Step 4: Reconcile each queue (create/update)
	created := 0
	updated := 0
	for queueID, config := range queueConfigs {
		if existingQueues[queueID] {
			// Update existing deployment
			if c.needsUpdate(queueID, config) {
				if err := c.updateWorkerDeployment(ctx, config); err != nil {
					fmt.Printf("  ⚠️  Failed to update deployment for %s: %v\n", config.Name, err)
				} else {
					fmt.Printf("  ✓ Updated deployment: %s\n", config.Name)
					updated++
					c.managedQueues[queueID] = config
				}
			}
		} else {
			// Create new deployment
			if err := c.createWorkerDeployment(ctx, config); err != nil {
				fmt.Printf("  ⚠️  Failed to create deployment for %s: %v\n", config.Name, err)
			} else {
				fmt.Printf("  ✓ Created deployment: %s\n", config.Name)
				created++
				c.managedQueues[queueID] = config
			}
		}
	}

	// Step 5: Delete deployments for queues that no longer exist
	deleted := 0
	for _, deployment := range existingDeployments {
		queueID := deployment.Labels["kubiya.ai/queue-id"]
		if queueID == "" {
			continue
		}

		// If queue doesn't exist in control plane, delete deployment
		if _, exists := queueConfigs[queueID]; !exists {
			if err := c.k8sClient.DeleteDeployment(ctx, c.namespace, deployment.Name); err != nil {
				fmt.Printf("  ⚠️  Failed to delete deployment %s: %v\n", deployment.Name, err)
			} else {
				fmt.Printf("  ✓ Deleted deployment: %s (queue removed)\n", deployment.Name)
				deleted++
				delete(c.managedQueues, queueID)
			}
		}
	}

	// Summary
	if created > 0 || updated > 0 || deleted > 0 {
		fmt.Printf("  Summary: %d created, %d updated, %d deleted\n", created, updated, deleted)
	}

	return nil
}

// needsUpdate checks if a deployment needs to be updated
func (c *OperatorController) needsUpdate(queueID string, newConfig *entities.WorkerQueueConfig) bool {
	oldConfig, exists := c.managedQueues[queueID]
	if !exists {
		return true // First time seeing this queue
	}

	// Check if config version changed
	if oldConfig.ConfigVersion != newConfig.ConfigVersion {
		return true
	}

	// Check if max_workers changed
	oldMaxWorkers := 1
	if oldConfig.MaxWorkers != nil {
		oldMaxWorkers = *oldConfig.MaxWorkers
	}
	newMaxWorkers := 1
	if newConfig.MaxWorkers != nil {
		newMaxWorkers = *newConfig.MaxWorkers
	}
	if oldMaxWorkers != newMaxWorkers {
		return true
	}

	return false
}

// createWorkerDeployment creates a new worker deployment
func (c *OperatorController) createWorkerDeployment(ctx context.Context, config *entities.WorkerQueueConfig) error {
	deployment := WorkerDeploymentTemplate(
		config,
		c.namespace,
		c.cfg.APIKey,
		c.cfg.BaseURL,
		c.enableAutoUpdate,
		c.updateCheckInterval,
	)

	return c.k8sClient.CreateDeployment(ctx, c.namespace, deployment)
}

// updateWorkerDeployment updates an existing worker deployment
func (c *OperatorController) updateWorkerDeployment(ctx context.Context, config *entities.WorkerQueueConfig) error {
	deployment := WorkerDeploymentTemplate(
		config,
		c.namespace,
		c.cfg.APIKey,
		c.cfg.BaseURL,
		c.enableAutoUpdate,
		c.updateCheckInterval,
	)

	return c.k8sClient.CreateOrUpdateDeployment(ctx, c.namespace, deployment)
}
