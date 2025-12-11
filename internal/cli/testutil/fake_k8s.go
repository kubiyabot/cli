package testutil

import (
	"context"
	"testing"

	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/client-go/kubernetes/fake"
)

// FakeK8sClientWrapper wraps a fake Kubernetes client for testing
type FakeK8sClientWrapper struct {
	Clientset *fake.Clientset
	Namespace string
}

// NewFakeK8sClientWrapper creates a new fake K8s client wrapper for testing
func NewFakeK8sClientWrapper(objects ...runtime.Object) *FakeK8sClientWrapper {
	return &FakeK8sClientWrapper{
		Clientset: fake.NewSimpleClientset(objects...),
		Namespace: "kubiya-test",
	}
}

// CreateTestNamespace creates a test namespace in the fake cluster
func (f *FakeK8sClientWrapper) CreateTestNamespace(t *testing.T, name string) {
	ns := &corev1.Namespace{
		ObjectMeta: metav1.ObjectMeta{
			Name: name,
			Labels: map[string]string{
				"test": "true",
			},
		},
	}
	_, err := f.Clientset.CoreV1().Namespaces().Create(context.Background(), ns, metav1.CreateOptions{})
	if err != nil {
		t.Fatalf("Failed to create test namespace: %v", err)
	}
}

// CreateTestDeployment creates a test deployment in the fake cluster
func (f *FakeK8sClientWrapper) CreateTestDeployment(t *testing.T, deployment *appsv1.Deployment) {
	if deployment.Namespace == "" {
		deployment.Namespace = f.Namespace
	}
	_, err := f.Clientset.AppsV1().Deployments(deployment.Namespace).Create(
		context.Background(), deployment, metav1.CreateOptions{})
	if err != nil {
		t.Fatalf("Failed to create test deployment: %v", err)
	}
}

// GetDeployment gets a deployment from the fake cluster
func (f *FakeK8sClientWrapper) GetDeployment(t *testing.T, namespace, name string) *appsv1.Deployment {
	deployment, err := f.Clientset.AppsV1().Deployments(namespace).Get(
		context.Background(), name, metav1.GetOptions{})
	if err != nil {
		t.Fatalf("Failed to get deployment: %v", err)
	}
	return deployment
}

// ListDeployments lists all deployments in a namespace
func (f *FakeK8sClientWrapper) ListDeployments(t *testing.T, namespace string) []appsv1.Deployment {
	list, err := f.Clientset.AppsV1().Deployments(namespace).List(
		context.Background(), metav1.ListOptions{})
	if err != nil {
		t.Fatalf("Failed to list deployments: %v", err)
	}
	return list.Items
}

// ListDeploymentsByLabel lists deployments matching a label selector
func (f *FakeK8sClientWrapper) ListDeploymentsByLabel(t *testing.T, namespace, labelKey, labelValue string) []appsv1.Deployment {
	selector := labelKey + "=" + labelValue
	list, err := f.Clientset.AppsV1().Deployments(namespace).List(
		context.Background(), metav1.ListOptions{LabelSelector: selector})
	if err != nil {
		t.Fatalf("Failed to list deployments by label: %v", err)
	}
	return list.Items
}

// DeleteDeployment deletes a deployment from the fake cluster
func (f *FakeK8sClientWrapper) DeleteDeployment(t *testing.T, namespace, name string) {
	err := f.Clientset.AppsV1().Deployments(namespace).Delete(
		context.Background(), name, metav1.DeleteOptions{})
	if err != nil {
		t.Fatalf("Failed to delete deployment: %v", err)
	}
}

// CreateTestConfigMap creates a test ConfigMap in the fake cluster
func (f *FakeK8sClientWrapper) CreateTestConfigMap(t *testing.T, cm *corev1.ConfigMap) {
	if cm.Namespace == "" {
		cm.Namespace = f.Namespace
	}
	_, err := f.Clientset.CoreV1().ConfigMaps(cm.Namespace).Create(
		context.Background(), cm, metav1.CreateOptions{})
	if err != nil {
		t.Fatalf("Failed to create test ConfigMap: %v", err)
	}
}

// CreateTestSecret creates a test Secret in the fake cluster
func (f *FakeK8sClientWrapper) CreateTestSecret(t *testing.T, secret *corev1.Secret) {
	if secret.Namespace == "" {
		secret.Namespace = f.Namespace
	}
	_, err := f.Clientset.CoreV1().Secrets(secret.Namespace).Create(
		context.Background(), secret, metav1.CreateOptions{})
	if err != nil {
		t.Fatalf("Failed to create test Secret: %v", err)
	}
}

// AssertDeploymentExists checks if a deployment exists
func (f *FakeK8sClientWrapper) AssertDeploymentExists(t *testing.T, namespace, name string) *appsv1.Deployment {
	deployment, err := f.Clientset.AppsV1().Deployments(namespace).Get(
		context.Background(), name, metav1.GetOptions{})
	if err != nil {
		t.Fatalf("Expected deployment %s/%s to exist, but got error: %v", namespace, name, err)
	}
	return deployment
}

// AssertDeploymentNotExists checks if a deployment does not exist
func (f *FakeK8sClientWrapper) AssertDeploymentNotExists(t *testing.T, namespace, name string) {
	_, err := f.Clientset.AppsV1().Deployments(namespace).Get(
		context.Background(), name, metav1.GetOptions{})
	if err == nil {
		t.Fatalf("Expected deployment %s/%s to not exist, but it does", namespace, name)
	}
}

// AssertDeploymentReplicas checks if a deployment has the expected number of replicas
func (f *FakeK8sClientWrapper) AssertDeploymentReplicas(t *testing.T, namespace, name string, expectedReplicas int32) {
	deployment := f.GetDeployment(t, namespace, name)
	if deployment.Spec.Replicas == nil {
		t.Fatalf("Deployment %s/%s has nil replicas", namespace, name)
	}
	if *deployment.Spec.Replicas != expectedReplicas {
		t.Fatalf("Expected deployment %s/%s to have %d replicas, got %d",
			namespace, name, expectedReplicas, *deployment.Spec.Replicas)
	}
}

// AssertDeploymentLabel checks if a deployment has a specific label
func (f *FakeK8sClientWrapper) AssertDeploymentLabel(t *testing.T, namespace, name, labelKey, expectedValue string) {
	deployment := f.GetDeployment(t, namespace, name)
	actualValue, exists := deployment.Labels[labelKey]
	if !exists {
		t.Fatalf("Deployment %s/%s missing label %s", namespace, name, labelKey)
	}
	if actualValue != expectedValue {
		t.Fatalf("Deployment %s/%s label %s: expected %s, got %s",
			namespace, name, labelKey, expectedValue, actualValue)
	}
}

// AssertPodNodeSelector checks if a deployment's pod template has the expected node selector
func (f *FakeK8sClientWrapper) AssertPodNodeSelector(t *testing.T, namespace, name, key, expectedValue string) {
	deployment := f.GetDeployment(t, namespace, name)
	actualValue, exists := deployment.Spec.Template.Spec.NodeSelector[key]
	if !exists {
		t.Fatalf("Deployment %s/%s pod template missing nodeSelector key %s", namespace, name, key)
	}
	if actualValue != expectedValue {
		t.Fatalf("Deployment %s/%s nodeSelector %s: expected %s, got %s",
			namespace, name, key, expectedValue, actualValue)
	}
}

// AssertPodTolerations checks if a deployment's pod template has expected tolerations
func (f *FakeK8sClientWrapper) AssertPodTolerations(t *testing.T, namespace, name string, expectedCount int) {
	deployment := f.GetDeployment(t, namespace, name)
	actualCount := len(deployment.Spec.Template.Spec.Tolerations)
	if actualCount != expectedCount {
		t.Fatalf("Deployment %s/%s: expected %d tolerations, got %d",
			namespace, name, expectedCount, actualCount)
	}
}

// AssertContainerResources checks if a container has the expected resource requests/limits
func (f *FakeK8sClientWrapper) AssertContainerResources(t *testing.T, namespace, name, containerName, resourceName, requestValue, limitValue string) {
	deployment := f.GetDeployment(t, namespace, name)

	var container *corev1.Container
	for i := range deployment.Spec.Template.Spec.Containers {
		if deployment.Spec.Template.Spec.Containers[i].Name == containerName {
			container = &deployment.Spec.Template.Spec.Containers[i]
			break
		}
	}

	if container == nil {
		t.Fatalf("Deployment %s/%s: container %s not found", namespace, name, containerName)
	}

	if requestValue != "" {
		req := container.Resources.Requests[corev1.ResourceName(resourceName)]
		if req.String() != requestValue {
			t.Fatalf("Deployment %s/%s container %s resource %s request: expected %s, got %s",
				namespace, name, containerName, resourceName, requestValue, req.String())
		}
	}

	if limitValue != "" {
		limit := container.Resources.Limits[corev1.ResourceName(resourceName)]
		if limit.String() != limitValue {
			t.Fatalf("Deployment %s/%s container %s resource %s limit: expected %s, got %s",
				namespace, name, containerName, resourceName, limitValue, limit.String())
		}
	}
}

// CleanupTestResources deletes all test resources
func (f *FakeK8sClientWrapper) CleanupTestResources(t *testing.T) {
	// Delete all deployments in test namespace
	err := f.Clientset.AppsV1().Deployments(f.Namespace).DeleteCollection(
		context.Background(),
		metav1.DeleteOptions{},
		metav1.ListOptions{},
	)
	if err != nil {
		t.Logf("Warning: Failed to cleanup deployments: %v", err)
	}

	// Delete test namespace
	err = f.Clientset.CoreV1().Namespaces().Delete(
		context.Background(),
		f.Namespace,
		metav1.DeleteOptions{},
	)
	if err != nil {
		t.Logf("Warning: Failed to cleanup namespace: %v", err)
	}
}
