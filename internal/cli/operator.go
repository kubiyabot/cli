package cli

import (
	"context"
	"fmt"

	"github.com/kubiyabot/cli/internal/config"
	"github.com/spf13/cobra"
)

type OperatorOptions struct {
	// Operator configuration
	Namespace           string
	ReconcileInterval   string
	EnableAutoUpdate    bool
	UpdateCheckInterval string

	cfg *config.Config
}

func newWorkerOperatorCommand(cfg *config.Config) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "operator",
		Short: "🎛️  Manage Kubiya worker operator",
		Long: `Manage the Kubiya worker operator for Kubernetes.

The operator watches the control plane for worker queue changes and automatically:
  • Creates/updates worker Deployments in Kubernetes
  • Syncs max_workers to replica count
  • Applies auto-update configuration
  • Handles dynamic scaling

Commands:
  • start   - Run the operator (typically in-cluster)
  • install - Install operator to Kubernetes cluster
  • status  - Check operator status
  • logs    - View operator logs

Examples:
  # Install operator to cluster
  kubiya worker operator install

  # Check operator status
  kubiya worker operator status

  # View operator logs
  kubiya worker operator logs -f`,
	}

	// Add subcommands
	cmd.AddCommand(newOperatorStartCommand(cfg))
	cmd.AddCommand(newOperatorInstallCommand(cfg))
	cmd.AddCommand(newOperatorStatusCommand(cfg))
	cmd.AddCommand(newOperatorLogsCommand(cfg))

	return cmd
}

func newOperatorStartCommand(cfg *config.Config) *cobra.Command {
	opts := &OperatorOptions{
		cfg: cfg,
	}

	cmd := &cobra.Command{
		Use:   "start",
		Short: "Start the Kubiya worker operator",
		Long: `Start the Kubiya worker operator to manage workers in Kubernetes.

The operator runs continuously and:
  1. Watches control plane for worker queue changes
  2. Creates/updates Kubernetes Deployments for workers
  3. Syncs max_workers to replica count
  4. Handles auto-update configuration
  5. Monitors worker health

This command is typically run inside a Kubernetes pod, but can also run locally
for development with proper kubeconfig access.

Examples:
  # Run operator (in-cluster)
  kubiya operator start

  # Run with custom namespace
  kubiya operator start --namespace=kubiya-workers

  # Enable auto-update for all workers
  kubiya operator start --enable-auto-update

  # Custom reconciliation interval
  kubiya operator start --reconcile-interval=30s`,
		RunE: func(cmd *cobra.Command, args []string) error {
			return opts.runOperator(cmd.Context())
		},
	}

	cmd.Flags().StringVar(&opts.Namespace, "namespace", "kubiya", "Kubernetes namespace for workers")
	cmd.Flags().StringVar(&opts.ReconcileInterval, "reconcile-interval", "30s", "Reconciliation interval (e.g., 30s, 1m, 5m)")
	cmd.Flags().BoolVar(&opts.EnableAutoUpdate, "enable-auto-update", false, "Enable auto-update for all managed workers")
	cmd.Flags().StringVar(&opts.UpdateCheckInterval, "update-check-interval", "5m", "Update check interval for workers")

	return cmd
}

func (opts *OperatorOptions) runOperator(ctx context.Context) error {
	fmt.Println()
	fmt.Println("═══════════════════════════════════════════════════════════════════════════")
	fmt.Println("🎛️   KUBIYA WORKER OPERATOR")
	fmt.Println("═══════════════════════════════════════════════════════════════════════════")
	fmt.Println()

	// Configuration
	fmt.Println("📋 CONFIGURATION")
	fmt.Println("───────────────────────────────────────────────────────────────────────────")
	fmt.Printf("   Namespace:           %s\n", opts.Namespace)
	fmt.Printf("   Reconcile Interval:  %s\n", opts.ReconcileInterval)
	fmt.Printf("   Auto-Update:         %v\n", opts.EnableAutoUpdate)
	if opts.EnableAutoUpdate {
		fmt.Printf("   Update Interval:     %s\n", opts.UpdateCheckInterval)
	}
	fmt.Println()

	// Check API key
	if opts.cfg.APIKey == "" {
		return fmt.Errorf("❌ KUBIYA_API_KEY is required\nRun: kubiya login")
	}
	fmt.Println("✓ API Key authenticated")

	// Initialize Kubernetes client
	fmt.Println()
	fmt.Println("🔧 KUBERNETES CONNECTION")
	fmt.Println("───────────────────────────────────────────────────────────────────────────")

	k8sClient, err := NewK8sClient()
	if err != nil {
		return fmt.Errorf("❌ failed to create Kubernetes client: %w", err)
	}
	fmt.Println("✓ Connected to Kubernetes cluster")

	// Get cluster info
	clusterVersion, err := k8sClient.GetClusterVersion()
	if err != nil {
		fmt.Printf("⚠️  Could not determine cluster version: %v\n", err)
	} else {
		fmt.Printf("✓ Cluster version: %s\n", clusterVersion)
	}

	// Verify namespace exists or create it
	if err := k8sClient.EnsureNamespace(ctx, opts.Namespace); err != nil {
		return fmt.Errorf("❌ failed to ensure namespace: %w", err)
	}
	fmt.Printf("✓ Namespace ready: %s\n", opts.Namespace)

	// Initialize operator
	fmt.Println()
	fmt.Println("🚀 OPERATOR INITIALIZATION")
	fmt.Println("───────────────────────────────────────────────────────────────────────────")

	operator, err := NewOperatorController(
		opts.cfg,
		k8sClient,
		opts.Namespace,
		opts.ReconcileInterval,
		opts.EnableAutoUpdate,
		opts.UpdateCheckInterval,
	)
	if err != nil {
		return fmt.Errorf("❌ failed to create operator: %w", err)
	}
	fmt.Println("✓ Operator controller initialized")

	// Start operator
	fmt.Println()
	fmt.Println("═══════════════════════════════════════════════════════════════════════════")
	fmt.Println("✅  OPERATOR READY")
	fmt.Println("═══════════════════════════════════════════════════════════════════════════")
	fmt.Println()
	fmt.Println("   The operator is now watching for worker queue changes...")
	fmt.Println("   Press Ctrl+C to stop gracefully")
	fmt.Println()
	fmt.Println("───────────────────────────────────────────────────────────────────────────")
	fmt.Println()

	// Run operator (blocks until context cancelled)
	return operator.Run(ctx)
}

func newOperatorStatusCommand(cfg *config.Config) *cobra.Command {
	return &cobra.Command{
		Use:   "status",
		Short: "Check operator status",
		Long: `Check the status of the Kubiya worker operator.

Shows:
  • Operator deployment status
  • Number of managed worker queues
  • Total worker replicas
  • Recent reconciliation events

Examples:
  # Check operator status
  kubiya operator status

  # Check status in specific namespace
  kubiya operator status --namespace=kubiya-system`,
		RunE: func(cmd *cobra.Command, args []string) error {
			namespace := cmd.Flag("namespace").Value.String()
			if namespace == "" {
				namespace = "kubiya"
			}
			return checkOperatorStatus(cmd.Context(), namespace)
		},
	}
}

func newOperatorLogsCommand(cfg *config.Config) *cobra.Command {
	var follow bool
	var tail int

	cmd := &cobra.Command{
		Use:   "logs",
		Short: "View operator logs",
		Long: `View logs from the Kubiya worker operator.

Examples:
  # View recent logs
  kubiya operator logs

  # Follow logs in real-time
  kubiya operator logs --follow

  # Show last 100 lines
  kubiya operator logs --tail=100`,
		RunE: func(cmd *cobra.Command, args []string) error {
			namespace := cmd.Flag("namespace").Value.String()
			if namespace == "" {
				namespace = "kubiya"
			}
			return streamOperatorLogs(cmd.Context(), namespace, follow, tail)
		},
	}

	cmd.Flags().BoolVarP(&follow, "follow", "f", false, "Follow log output")
	cmd.Flags().IntVar(&tail, "tail", 50, "Number of lines to show from the end of the logs")
	cmd.Flags().String("namespace", "kubiya", "Operator namespace")

	return cmd
}

func checkOperatorStatus(ctx context.Context, namespace string) error {
	k8sClient, err := NewK8sClient()
	if err != nil {
		return fmt.Errorf("failed to create Kubernetes client: %w", err)
	}

	fmt.Println()
	fmt.Println("🎛️  OPERATOR STATUS")
	fmt.Println("═══════════════════════════════════════════════════════════════════════════")
	fmt.Println()

	// Check if operator deployment exists
	deployment, err := k8sClient.GetDeployment(ctx, namespace, "kubiya-operator")
	if err != nil {
		fmt.Println("❌ Operator not found")
		fmt.Println()
		fmt.Println("To install the operator, run:")
		fmt.Println("  kubiya operator install")
		fmt.Println()
		return nil
	}

	// Show deployment status
	fmt.Printf("✓ Operator: %s/%s\n", namespace, deployment.Name)
	fmt.Printf("  Replicas:   %d/%d ready\n", deployment.Status.ReadyReplicas, deployment.Status.Replicas)
	fmt.Printf("  Image:      %s\n", deployment.Spec.Template.Spec.Containers[0].Image)
	fmt.Println()

	// Count managed workers
	workers, err := k8sClient.ListDeploymentsByLabel(ctx, namespace, "app.kubernetes.io/managed-by", "kubiya-operator")
	if err != nil {
		fmt.Printf("⚠️  Could not list managed workers: %v\n", err)
	} else {
		totalReplicas := int32(0)
		for _, w := range workers {
			totalReplicas += w.Status.Replicas
		}
		fmt.Printf("📊 Managed Workers: %d queues, %d total replicas\n", len(workers), totalReplicas)
		fmt.Println()

		if len(workers) > 0 {
			fmt.Println("Workers:")
			for _, w := range workers {
				queueID := w.Labels["kubiya.ai/queue-id"]
				fmt.Printf("  • %s (queue: %s) - %d/%d replicas\n",
					w.Name, queueID, w.Status.ReadyReplicas, w.Status.Replicas)
			}
			fmt.Println()
		}
	}

	fmt.Println("═══════════════════════════════════════════════════════════════════════════")
	fmt.Println()

	return nil
}

func streamOperatorLogs(ctx context.Context, namespace string, follow bool, tail int) error {
	k8sClient, err := NewK8sClient()
	if err != nil {
		return fmt.Errorf("failed to create Kubernetes client: %w", err)
	}

	fmt.Printf("📜 Streaming logs from kubiya-operator in %s...\n\n", namespace)

	return k8sClient.StreamPodLogs(ctx, namespace, "kubiya-operator", follow, int64(tail))
}
