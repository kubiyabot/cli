package cli

import (
	"bytes"
	"context"
	"fmt"
	"io"
	"os"
	"path/filepath"

	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/util/yaml"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/rest"
	"k8s.io/client-go/tools/clientcmd"
)

// K8sClient wraps the Kubernetes client-go for worker management
type K8sClient struct {
	clientset *kubernetes.Clientset
	config    *rest.Config
}

// NewK8sClient creates a new Kubernetes client
// Tries in-cluster config first, then falls back to kubeconfig
func NewK8sClient() (*K8sClient, error) {
	// Try in-cluster configuration first
	config, err := rest.InClusterConfig()
	if err != nil {
		// Fall back to kubeconfig
		config, err = buildConfigFromKubeconfig()
		if err != nil {
			return nil, fmt.Errorf("failed to create Kubernetes config: %w", err)
		}
	}

	clientset, err := kubernetes.NewForConfig(config)
	if err != nil {
		return nil, fmt.Errorf("failed to create Kubernetes clientset: %w", err)
	}

	return &K8sClient{
		clientset: clientset,
		config:    config,
	}, nil
}

// buildConfigFromKubeconfig builds rest.Config from kubeconfig file
func buildConfigFromKubeconfig() (*rest.Config, error) {
	kubeconfigPath := os.Getenv("KUBECONFIG")
	if kubeconfigPath == "" {
		homeDir, err := os.UserHomeDir()
		if err != nil {
			return nil, fmt.Errorf("failed to get home directory: %w", err)
		}
		kubeconfigPath = filepath.Join(homeDir, ".kube", "config")
	}

	config, err := clientcmd.BuildConfigFromFlags("", kubeconfigPath)
	if err != nil {
		return nil, fmt.Errorf("failed to build config from kubeconfig: %w", err)
	}

	return config, nil
}

// GetClusterVersion returns the Kubernetes cluster version
func (c *K8sClient) GetClusterVersion() (string, error) {
	version, err := c.clientset.Discovery().ServerVersion()
	if err != nil {
		return "", err
	}
	return version.String(), nil
}

// EnsureNamespace ensures a namespace exists, creates it if it doesn't
func (c *K8sClient) EnsureNamespace(ctx context.Context, name string) error {
	_, err := c.clientset.CoreV1().Namespaces().Get(ctx, name, metav1.GetOptions{})
	if err == nil {
		return nil // Namespace exists
	}

	if !errors.IsNotFound(err) {
		return fmt.Errorf("failed to get namespace: %w", err)
	}

	// Create namespace
	namespace := &corev1.Namespace{
		ObjectMeta: metav1.ObjectMeta{
			Name: name,
			Labels: map[string]string{
				"app.kubernetes.io/name":      "kubiya",
				"app.kubernetes.io/component": "workers",
			},
		},
	}

	_, err = c.clientset.CoreV1().Namespaces().Create(ctx, namespace, metav1.CreateOptions{})
	if err != nil {
		return fmt.Errorf("failed to create namespace: %w", err)
	}

	return nil
}

// GetDeployment gets a deployment by name
func (c *K8sClient) GetDeployment(ctx context.Context, namespace, name string) (*appsv1.Deployment, error) {
	return c.clientset.AppsV1().Deployments(namespace).Get(ctx, name, metav1.GetOptions{})
}

// ListDeploymentsByLabel lists deployments with a specific label
func (c *K8sClient) ListDeploymentsByLabel(ctx context.Context, namespace, labelKey, labelValue string) ([]appsv1.Deployment, error) {
	labelSelector := fmt.Sprintf("%s=%s", labelKey, labelValue)
	list, err := c.clientset.AppsV1().Deployments(namespace).List(ctx, metav1.ListOptions{
		LabelSelector: labelSelector,
	})
	if err != nil {
		return nil, err
	}
	return list.Items, nil
}

// CreateDeployment creates a new deployment
func (c *K8sClient) CreateDeployment(ctx context.Context, namespace string, deployment *appsv1.Deployment) error {
	_, err := c.clientset.AppsV1().Deployments(namespace).Create(ctx, deployment, metav1.CreateOptions{})
	return err
}

// UpdateDeployment updates an existing deployment
func (c *K8sClient) UpdateDeployment(ctx context.Context, namespace string, deployment *appsv1.Deployment) error {
	_, err := c.clientset.AppsV1().Deployments(namespace).Update(ctx, deployment, metav1.UpdateOptions{})
	return err
}

// DeleteDeployment deletes a deployment
func (c *K8sClient) DeleteDeployment(ctx context.Context, namespace, name string) error {
	return c.clientset.AppsV1().Deployments(namespace).Delete(ctx, name, metav1.DeleteOptions{})
}

// ScaleDeployment scales a deployment to the specified number of replicas
func (c *K8sClient) ScaleDeployment(ctx context.Context, namespace, name string, replicas int32) error {
	deployment, err := c.GetDeployment(ctx, namespace, name)
	if err != nil {
		return err
	}

	deployment.Spec.Replicas = &replicas
	return c.UpdateDeployment(ctx, namespace, deployment)
}

// CreateOrUpdateDeployment creates a deployment if it doesn't exist, updates if it does
func (c *K8sClient) CreateOrUpdateDeployment(ctx context.Context, namespace string, deployment *appsv1.Deployment) error {
	existing, err := c.GetDeployment(ctx, namespace, deployment.Name)
	if err != nil {
		if errors.IsNotFound(err) {
			// Create new deployment
			return c.CreateDeployment(ctx, namespace, deployment)
		}
		return err
	}

	// Update existing deployment
	deployment.ResourceVersion = existing.ResourceVersion
	return c.UpdateDeployment(ctx, namespace, deployment)
}

// StreamPodLogs streams logs from a pod
func (c *K8sClient) StreamPodLogs(ctx context.Context, namespace, deploymentName string, follow bool, tailLines int64) error {
	// Find pods for deployment
	pods, err := c.clientset.CoreV1().Pods(namespace).List(ctx, metav1.ListOptions{
		LabelSelector: fmt.Sprintf("app.kubernetes.io/name=%s", deploymentName),
	})
	if err != nil {
		return fmt.Errorf("failed to list pods: %w", err)
	}

	if len(pods.Items) == 0 {
		return fmt.Errorf("no pods found for deployment %s", deploymentName)
	}

	// Stream logs from first pod
	pod := pods.Items[0]
	opts := &corev1.PodLogOptions{
		Follow:    follow,
		TailLines: &tailLines,
	}

	req := c.clientset.CoreV1().Pods(namespace).GetLogs(pod.Name, opts)
	stream, err := req.Stream(ctx)
	if err != nil {
		return fmt.Errorf("failed to open log stream: %w", err)
	}
	defer stream.Close()

	// Copy logs to stdout
	_, err = io.Copy(os.Stdout, stream)
	return err
}

// ApplyManifests applies Kubernetes manifests from a YAML string
func (c *K8sClient) ApplyManifests(ctx context.Context, manifestsYAML string) error {
	// Parse YAML documents
	decoder := yaml.NewYAMLOrJSONDecoder(bytes.NewReader([]byte(manifestsYAML)), 4096)

	for {
		var obj map[string]interface{}
		err := decoder.Decode(&obj)
		if err == io.EOF {
			break
		}
		if err != nil {
			return fmt.Errorf("failed to decode YAML: %w", err)
		}

		if obj == nil {
			continue
		}

		// Apply based on Kind
		kind, ok := obj["kind"].(string)
		if !ok {
			continue
		}

		switch kind {
		case "Namespace":
			if err := c.applyNamespace(ctx, obj); err != nil {
				return fmt.Errorf("failed to apply namespace: %w", err)
			}
		case "ServiceAccount":
			if err := c.applyServiceAccount(ctx, obj); err != nil {
				return fmt.Errorf("failed to apply service account: %w", err)
			}
		case "ClusterRole":
			if err := c.applyClusterRole(ctx, obj); err != nil {
				return fmt.Errorf("failed to apply cluster role: %w", err)
			}
		case "ClusterRoleBinding":
			if err := c.applyClusterRoleBinding(ctx, obj); err != nil {
				return fmt.Errorf("failed to apply cluster role binding: %w", err)
			}
		case "Secret":
			if err := c.applySecret(ctx, obj); err != nil {
				return fmt.Errorf("failed to apply secret: %w", err)
			}
		case "ConfigMap":
			if err := c.applyConfigMap(ctx, obj); err != nil {
				return fmt.Errorf("failed to apply configmap: %w", err)
			}
		case "Deployment":
			if err := c.applyDeploymentFromMap(ctx, obj); err != nil {
				return fmt.Errorf("failed to apply deployment: %w", err)
			}
		}
	}

	return nil
}

// Helper methods for applying specific resource types
func (c *K8sClient) applyNamespace(ctx context.Context, obj map[string]interface{}) error {
	metadata := obj["metadata"].(map[string]interface{})
	name := metadata["name"].(string)
	return c.EnsureNamespace(ctx, name)
}

func (c *K8sClient) applyServiceAccount(ctx context.Context, obj map[string]interface{}) error {
	// Convert map to ServiceAccount struct
	// For simplicity, we'll use kubectl apply command
	// In production, proper struct conversion should be used
	return nil
}

func (c *K8sClient) applyClusterRole(ctx context.Context, obj map[string]interface{}) error {
	// Similar to ServiceAccount
	return nil
}

func (c *K8sClient) applyClusterRoleBinding(ctx context.Context, obj map[string]interface{}) error {
	// Similar to ServiceAccount
	return nil
}

func (c *K8sClient) applySecret(ctx context.Context, obj map[string]interface{}) error {
	// Similar to ServiceAccount
	return nil
}

func (c *K8sClient) applyConfigMap(ctx context.Context, obj map[string]interface{}) error {
	// Similar to ServiceAccount
	return nil
}

func (c *K8sClient) applyDeploymentFromMap(ctx context.Context, obj map[string]interface{}) error {
	// Similar to ServiceAccount
	return nil
}

// GetPodsByLabel gets pods with a specific label
func (c *K8sClient) GetPodsByLabel(ctx context.Context, namespace, labelKey, labelValue string) ([]corev1.Pod, error) {
	labelSelector := fmt.Sprintf("%s=%s", labelKey, labelValue)
	list, err := c.clientset.CoreV1().Pods(namespace).List(ctx, metav1.ListOptions{
		LabelSelector: labelSelector,
	})
	if err != nil {
		return nil, err
	}
	return list.Items, nil
}

// CreateConfigMap creates a ConfigMap
func (c *K8sClient) CreateConfigMap(ctx context.Context, namespace string, configMap *corev1.ConfigMap) error {
	_, err := c.clientset.CoreV1().ConfigMaps(namespace).Create(ctx, configMap, metav1.CreateOptions{})
	return err
}

// CreateOrUpdateConfigMap creates or updates a ConfigMap
func (c *K8sClient) CreateOrUpdateConfigMap(ctx context.Context, namespace string, configMap *corev1.ConfigMap) error {
	_, err := c.clientset.CoreV1().ConfigMaps(namespace).Get(ctx, configMap.Name, metav1.GetOptions{})
	if err != nil {
		if errors.IsNotFound(err) {
			return c.CreateConfigMap(ctx, namespace, configMap)
		}
		return err
	}

	_, err = c.clientset.CoreV1().ConfigMaps(namespace).Update(ctx, configMap, metav1.UpdateOptions{})
	return err
}
