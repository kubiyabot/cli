package cli

import (
	"context"
	"fmt"
	"os"

	"github.com/kubiyabot/cli/internal/config"
	"github.com/spf13/cobra"
)

type OperatorInstallOptions struct {
	Namespace           string
	ImageTag            string
	CreateNamespace     bool
	EnableAutoUpdate    bool
	UpdateCheckInterval string
	DryRun              bool
	OutputFile          string
	OpenShift           bool // Enable OpenShift-specific manifests (SCCs)

	cfg *config.Config
}

func newOperatorInstallCommand(cfg *config.Config) *cobra.Command {
	opts := &OperatorInstallOptions{
		cfg: cfg,
	}

	cmd := &cobra.Command{
		Use:   "install",
		Short: "Install the Kubiya worker operator to Kubernetes",
		Long: `Install the Kubiya worker operator to your Kubernetes cluster.

The operator will be deployed as a Deployment in the specified namespace with:
  • ServiceAccount with RBAC permissions
  • Deployment running the operator
  • ConfigMap for configuration
  • Secret for API key

Prerequisites:
  • kubectl configured and connected to cluster
  • Cluster admin permissions (for RBAC)
  • Valid KUBIYA_API_KEY

The operator will automatically:
  1. Watch control plane for worker queue changes
  2. Create/update worker Deployments
  3. Sync max_workers to replica count
  4. Handle auto-update configuration

Examples:
  # Install operator with default settings
  kubiya operator install

  # Install in custom namespace
  kubiya operator install --namespace=kubiya-system --create-namespace

  # Enable auto-update for all workers
  kubiya operator install --enable-auto-update

  # Generate YAML without applying (dry-run)
  kubiya operator install --dry-run

  # Save manifests to file
  kubiya operator install --output=operator.yaml --dry-run`,
		RunE: func(cmd *cobra.Command, args []string) error {
			return opts.installOperator(cmd.Context())
		},
	}

	cmd.Flags().StringVar(&opts.Namespace, "namespace", "kubiya", "Kubernetes namespace for operator")
	cmd.Flags().StringVar(&opts.ImageTag, "image-tag", "latest", "Operator image tag")
	cmd.Flags().BoolVar(&opts.CreateNamespace, "create-namespace", false, "Create namespace if it doesn't exist")
	cmd.Flags().BoolVar(&opts.EnableAutoUpdate, "enable-auto-update", false, "Enable auto-update for all managed workers")
	cmd.Flags().StringVar(&opts.UpdateCheckInterval, "update-check-interval", "5m", "Update check interval for workers")
	cmd.Flags().BoolVar(&opts.DryRun, "dry-run", false, "Generate manifests without applying")
	cmd.Flags().StringVarP(&opts.OutputFile, "output", "o", "", "Output manifests to file (implies --dry-run)")
	cmd.Flags().BoolVar(&opts.OpenShift, "openshift", false, "Generate OpenShift-specific manifests (SCCs). Auto-detected if not specified.")

	return cmd
}

func (opts *OperatorInstallOptions) installOperator(ctx context.Context) error {
	fmt.Println()
	fmt.Println("═══════════════════════════════════════════════════════════════════════════")
	fmt.Println("📦  INSTALLING KUBIYA WORKER OPERATOR")
	fmt.Println("═══════════════════════════════════════════════════════════════════════════")
	fmt.Println()

	// Check API key
	if opts.cfg.APIKey == "" {
		return fmt.Errorf("❌ KUBIYA_API_KEY is required\nRun: kubiya login")
	}

	// If output file specified, enable dry-run
	if opts.OutputFile != "" {
		opts.DryRun = true
	}

	// Detect OpenShift if not explicitly set
	if !opts.DryRun && !opts.OpenShift {
		k8sClient, err := NewK8sClient()
		if err == nil {
			isOpenShift, err := IsOpenShift(k8sClient.config)
			if err == nil && isOpenShift {
				opts.OpenShift = true
				fmt.Println("✓ OpenShift detected - including SCCs in manifests")
			}
		}
	}

	// Generate manifests
	fmt.Println("📋 GENERATING MANIFESTS")
	fmt.Println("───────────────────────────────────────────────────────────────────────────")
	fmt.Printf("   Namespace:           %s\n", opts.Namespace)
	fmt.Printf("   Image Tag:           %s\n", opts.ImageTag)
	fmt.Printf("   Platform:            %s\n", map[bool]string{true: "OpenShift", false: "Kubernetes"}[opts.OpenShift])
	fmt.Printf("   Auto-Update:         %v\n", opts.EnableAutoUpdate)
	if opts.EnableAutoUpdate {
		fmt.Printf("   Update Interval:     %s\n", opts.UpdateCheckInterval)
	}
	fmt.Println()

	manifests := opts.generateManifests()
	fmt.Println("✓ Manifests generated")

	// Output to file or stdout
	if opts.OutputFile != "" {
		if err := os.WriteFile(opts.OutputFile, []byte(manifests), 0644); err != nil {
			return fmt.Errorf("failed to write manifests to file: %w", err)
		}
		fmt.Printf("✓ Manifests saved to: %s\n", opts.OutputFile)
		fmt.Println()
		fmt.Println("To install, run:")
		fmt.Printf("  kubectl apply -f %s\n", opts.OutputFile)
		fmt.Println()
		return nil
	}

	if opts.DryRun {
		fmt.Println()
		fmt.Println("Generated manifests:")
		fmt.Println("───────────────────────────────────────────────────────────────────────────")
		fmt.Println(manifests)
		fmt.Println("───────────────────────────────────────────────────────────────────────────")
		fmt.Println()
		return nil
	}

	// Apply manifests to cluster
	fmt.Println("🚀 INSTALLING TO CLUSTER")
	fmt.Println("───────────────────────────────────────────────────────────────────────────")

	k8sClient, err := NewK8sClient()
	if err != nil {
		return fmt.Errorf("❌ failed to create Kubernetes client: %w", err)
	}
	fmt.Println("✓ Connected to Kubernetes cluster")

	// Create namespace if requested
	if opts.CreateNamespace {
		if err := k8sClient.EnsureNamespace(ctx, opts.Namespace); err != nil {
			return fmt.Errorf("❌ failed to create namespace: %w", err)
		}
		fmt.Printf("✓ Namespace created: %s\n", opts.Namespace)
	}

	// Apply manifests
	fmt.Println()
	fmt.Print("   Applying manifests... ")
	if err := k8sClient.ApplyManifests(ctx, manifests); err != nil {
		fmt.Println("failed")
		return fmt.Errorf("❌ failed to apply manifests: %w", err)
	}
	fmt.Println("done")

	fmt.Println()
	fmt.Println("═══════════════════════════════════════════════════════════════════════════")
	fmt.Println("✅  OPERATOR INSTALLED SUCCESSFULLY")
	fmt.Println("═══════════════════════════════════════════════════════════════════════════")
	fmt.Println()
	fmt.Println("The operator is now running in your cluster!")
	fmt.Println()
	fmt.Println("Next steps:")
	fmt.Printf("  • Check status:  kubiya operator status --namespace=%s\n", opts.Namespace)
	fmt.Printf("  • View logs:     kubiya operator logs --namespace=%s -f\n", opts.Namespace)
	fmt.Println("  • Create worker queues in the control plane")
	fmt.Println("  • Workers will be automatically deployed!")
	fmt.Println()
	fmt.Println("═══════════════════════════════════════════════════════════════════════════")
	fmt.Println()

	return nil
}

func (opts *OperatorInstallOptions) generateManifests() string {
	controlPlaneURL := opts.cfg.BaseURL
	if controlPlaneURL == "" {
		controlPlaneURL = "https://control-plane.kubiya.ai"
	}

	manifests := fmt.Sprintf(`---
# Namespace (optional - only if --create-namespace)
apiVersion: v1
kind: Namespace
metadata:
  name: %s
  labels:
    app.kubernetes.io/name: kubiya
    app.kubernetes.io/component: operator

---
# ServiceAccount for operator
apiVersion: v1
kind: ServiceAccount
metadata:
  name: kubiya-operator
  namespace: %s
  labels:
    app.kubernetes.io/name: kubiya-operator
    app.kubernetes.io/component: operator

---
# ClusterRole for operator RBAC
apiVersion: rbac.authorization.k8s.io/v1
kind: ClusterRole
metadata:
  name: kubiya-operator
  labels:
    app.kubernetes.io/name: kubiya-operator
rules:
# Deployments management
- apiGroups: ["apps"]
  resources: ["deployments"]
  verbs: ["get", "list", "watch", "create", "update", "patch", "delete"]
# Pods for status checking
- apiGroups: [""]
  resources: ["pods"]
  verbs: ["get", "list", "watch"]
# ConfigMaps for worker configuration
- apiGroups: [""]
  resources: ["configmaps"]
  verbs: ["get", "list", "watch", "create", "update", "patch", "delete"]
# Secrets for API keys
- apiGroups: [""]
  resources: ["secrets"]
  verbs: ["get", "list", "watch", "create"]
# Services (optional, for future use)
- apiGroups: [""]
  resources: ["services"]
  verbs: ["get", "list", "watch", "create", "update", "patch", "delete"]
# Namespaces (read-only)
- apiGroups: [""]
  resources: ["namespaces"]
  verbs: ["get", "list", "watch"]

---
# ClusterRoleBinding
apiVersion: rbac.authorization.k8s.io/v1
kind: ClusterRoleBinding
metadata:
  name: kubiya-operator
  labels:
    app.kubernetes.io/name: kubiya-operator
roleRef:
  apiGroup: rbac.authorization.k8s.io
  kind: ClusterRole
  name: kubiya-operator
subjects:
- kind: ServiceAccount
  name: kubiya-operator
  namespace: %s

---
# Secret for API key
apiVersion: v1
kind: Secret
metadata:
  name: kubiya-operator-secret
  namespace: %s
  labels:
    app.kubernetes.io/name: kubiya-operator
type: Opaque
stringData:
  api-key: "%s"

---
# ConfigMap for operator configuration
apiVersion: v1
kind: ConfigMap
metadata:
  name: kubiya-operator-config
  namespace: %s
  labels:
    app.kubernetes.io/name: kubiya-operator
data:
  namespace: "%s"
  reconcile-interval: "30s"
  enable-auto-update: "%t"
  update-check-interval: "%s"
  control-plane-url: "%s"

---
# Deployment for operator
apiVersion: apps/v1
kind: Deployment
metadata:
  name: kubiya-operator
  namespace: %s
  labels:
    app.kubernetes.io/name: kubiya-operator
    app.kubernetes.io/component: operator
    app.kubernetes.io/version: "%s"
spec:
  replicas: 1
  strategy:
    type: Recreate  # Only one operator should run at a time
  selector:
    matchLabels:
      app.kubernetes.io/name: kubiya-operator
  template:
    metadata:
      labels:
        app.kubernetes.io/name: kubiya-operator
        app.kubernetes.io/component: operator
    spec:
      serviceAccountName: kubiya-operator
      containers:
      - name: operator
        image: ghcr.io/kubiyabot/cli:%s
        imagePullPolicy: Always
        command: ["kubiya", "operator", "start"]
        args:
          - "--namespace=$(WORKER_NAMESPACE)"
          - "--reconcile-interval=$(RECONCILE_INTERVAL)"
        env:
        - name: KUBIYA_API_KEY
          valueFrom:
            secretKeyRef:
              name: kubiya-operator-secret
              key: api-key
        - name: WORKER_NAMESPACE
          valueFrom:
            configMapKeyRef:
              name: kubiya-operator-config
              key: namespace
        - name: RECONCILE_INTERVAL
          valueFrom:
            configMapKeyRef:
              name: kubiya-operator-config
              key: reconcile-interval
        - name: ENABLE_AUTO_UPDATE
          valueFrom:
            configMapKeyRef:
              name: kubiya-operator-config
              key: enable-auto-update
        - name: UPDATE_CHECK_INTERVAL
          valueFrom:
            configMapKeyRef:
              name: kubiya-operator-config
              key: update-check-interval
        - name: CONTROL_PLANE_URL
          valueFrom:
            configMapKeyRef:
              name: kubiya-operator-config
              key: control-plane-url
        resources:
          requests:
            memory: "128Mi"
            cpu: "100m"
          limits:
            memory: "512Mi"
            cpu: "500m"
        securityContext:
          allowPrivilegeEscalation: false
          runAsNonRoot: true
          # Note: runAsUser not set for OpenShift compatibility (uses random UID assignment)
          capabilities:
            drop:
            - ALL
      restartPolicy: Always
`,
		opts.Namespace,             // Namespace metadata
		opts.Namespace,             // ServiceAccount namespace
		opts.Namespace,             // ClusterRoleBinding namespace
		opts.Namespace,             // Secret namespace
		opts.cfg.APIKey,            // API key in Secret
		opts.Namespace,             // ConfigMap namespace
		opts.Namespace,             // namespace in ConfigMap
		opts.EnableAutoUpdate,      // enable-auto-update in ConfigMap
		opts.UpdateCheckInterval,   // update-check-interval in ConfigMap
		controlPlaneURL,            // control-plane-url in ConfigMap
		opts.Namespace,             // Deployment namespace
		opts.ImageTag,              // version label
		opts.ImageTag,              // container image tag
	)

	// Add OpenShift-specific manifests (SCCs and RBAC bindings) if detected/requested
	if opts.OpenShift {
		manifests += "\n\n" + GenerateOperatorSCC(opts.Namespace)
		manifests += "\n\n" + GenerateWorkerSCC(opts.Namespace)
		manifests += "\n\n" + GetOpenShiftRBACManifests(opts.Namespace)
	}

	return manifests
}
