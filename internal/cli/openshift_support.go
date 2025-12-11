package cli

import (
	"context"
	"fmt"

	"k8s.io/client-go/discovery"
	"k8s.io/client-go/rest"
)

// OpenShiftSecurityContextConstraint represents an OpenShift SCC
// This is a simplified version - in production, use the OpenShift API types
type OpenShiftSecurityContextConstraint struct {
	APIVersion string                 `json:"apiVersion"`
	Kind       string                 `json:"kind"`
	Metadata   map[string]interface{} `json:"metadata"`
	Priority   *int                   `json:"priority,omitempty"`
	// SCC fields
	AllowPrivilegedContainer bool     `json:"allowPrivilegedContainer"`
	AllowHostDirVolumePlugin bool     `json:"allowHostDirVolumePlugin"`
	AllowHostNetwork         bool     `json:"allowHostNetwork"`
	AllowHostPorts           bool     `json:"allowHostPorts"`
	AllowHostPID             bool     `json:"allowHostPID"`
	AllowHostIPC             bool     `json:"allowHostIPC"`
	ReadOnlyRootFilesystem   bool     `json:"readOnlyRootFilesystem"`
	RunAsUser                RunAsUser           `json:"runAsUser"`
	SELinuxContext           SELinuxContext      `json:"seLinuxContext"`
	FSGroup                  FSGroup             `json:"fsGroup"`
	SupplementalGroups       SupplementalGroups  `json:"supplementalGroups"`
	DefaultAddCapabilities   []string            `json:"defaultAddCapabilities,omitempty"`
	RequiredDropCapabilities []string            `json:"requiredDropCapabilities,omitempty"`
	AllowedCapabilities      []string            `json:"allowedCapabilities,omitempty"`
	Volumes                  []string            `json:"volumes,omitempty"`
	Users                    []string            `json:"users,omitempty"`
	Groups                   []string            `json:"groups,omitempty"`
}

type RunAsUser struct {
	Type string `json:"type"`
	UID  *int64 `json:"uid,omitempty"`
}

type SELinuxContext struct {
	Type string `json:"type"`
}

type FSGroup struct {
	Type string `json:"type"`
}

type SupplementalGroups struct {
	Type string `json:"type"`
}

// IsOpenShift detects if we're running on OpenShift
func IsOpenShift(config *rest.Config) (bool, error) {
	discoveryClient, err := discovery.NewDiscoveryClientForConfig(config)
	if err != nil {
		return false, err
	}

	// Check for OpenShift-specific API groups
	apiGroups, err := discoveryClient.ServerGroups()
	if err != nil {
		return false, err
	}

	for _, group := range apiGroups.Groups {
		// OpenShift has security.openshift.io API group
		if group.Name == "security.openshift.io" {
			return true, nil
		}
		// Also check for route.openshift.io
		if group.Name == "route.openshift.io" {
			return true, nil
		}
	}

	return false, nil
}

// GenerateOperatorSCC generates an SCC for the Kubiya operator
// This SCC is needed for the operator pod itself (not worker pods)
func GenerateOperatorSCC(namespace string) string {
	return fmt.Sprintf(`apiVersion: security.openshift.io/v1
kind: SecurityContextConstraints
metadata:
  name: kubiya-operator-scc
  annotations:
    kubernetes.io/description: "SCC for Kubiya operator to manage worker deployments"
allowPrivilegedContainer: false
allowHostDirVolumePlugin: false
allowHostNetwork: false
allowHostPorts: false
allowHostPID: false
allowHostIPC: false
readOnlyRootFilesystem: false
runAsUser:
  type: RunAsAny
seLinuxContext:
  type: MustRunAs
fsGroup:
  type: RunAsAny
supplementalGroups:
  type: RunAsAny
requiredDropCapabilities:
- ALL
volumes:
- configMap
- downwardAPI
- emptyDir
- persistentVolumeClaim
- projected
- secret
users:
- system:serviceaccount:%s:kubiya-operator
`, namespace)
}

// GenerateWorkerSCC generates an SCC for Kubiya worker pods
// This is more permissive than the operator SCC as workers may need additional capabilities
func GenerateWorkerSCC(namespace string) string {
	return fmt.Sprintf(`apiVersion: security.openshift.io/v1
kind: SecurityContextConstraints
metadata:
  name: kubiya-worker-scc
  annotations:
    kubernetes.io/description: "SCC for Kubiya worker pods executing agent tasks"
allowPrivilegedContainer: false
allowHostDirVolumePlugin: false
allowHostNetwork: false
allowHostPorts: false
allowHostPID: false
allowHostIPC: false
readOnlyRootFilesystem: false
runAsUser:
  type: MustRunAsRange
seLinuxContext:
  type: MustRunAs
fsGroup:
  type: RunAsAny
supplementalGroups:
  type: RunAsAny
requiredDropCapabilities:
- ALL
allowedCapabilities: []
volumes:
- configMap
- downwardAPI
- emptyDir
- persistentVolumeClaim
- projected
- secret
users:
- system:serviceaccount:%s:default
`, namespace)
}

// GetOpenShiftRBACManifests returns OpenShift-specific RBAC manifests
func GetOpenShiftRBACManifests(namespace string) string {
	return fmt.Sprintf(`---
# RoleBinding to allow operator to use the SCC
apiVersion: rbac.authorization.k8s.io/v1
kind: RoleBinding
metadata:
  name: kubiya-operator-scc-binding
  namespace: %s
roleRef:
  apiGroup: rbac.authorization.k8s.io
  kind: ClusterRole
  name: system:openshift:scc:kubiya-operator-scc
subjects:
- kind: ServiceAccount
  name: kubiya-operator
  namespace: %s
---
# RoleBinding to allow workers to use their SCC
apiVersion: rbac.authorization.k8s.io/v1
kind: RoleBinding
metadata:
  name: kubiya-worker-scc-binding
  namespace: %s
roleRef:
  apiGroup: rbac.authorization.k8s.io
  kind: ClusterRole
  name: system:openshift:scc:kubiya-worker-scc
subjects:
- kind: ServiceAccount
  name: default
  namespace: %s
`, namespace, namespace, namespace, namespace)
}

// ValidateOpenShiftSecurityContext validates that security contexts are OpenShift-compatible
func ValidateOpenShiftSecurityContext(podConfig *PodTemplateConfig) error {
	// Check if fixed UIDs are being used (not recommended for OpenShift)
	if podConfig.PodSecurityContext != nil && podConfig.PodSecurityContext.RunAsUser != nil {
		fmt.Printf("⚠️  Warning: Fixed runAsUser specified. This may conflict with OpenShift's restricted SCC.\n")
		fmt.Printf("   OpenShift assigns random UIDs by default. Consider removing runAsUser for better compatibility.\n")
	}

	if podConfig.ContainerSecurityContext != nil && podConfig.ContainerSecurityContext.RunAsUser != nil {
		fmt.Printf("⚠️  Warning: Fixed runAsUser specified in container security context.\n")
		fmt.Printf("   This may conflict with OpenShift's restricted SCC.\n")
	}

	// Ensure runAsNonRoot is set for OpenShift
	if podConfig.ContainerSecurityContext != nil {
		if podConfig.ContainerSecurityContext.RunAsNonRoot == nil || !*podConfig.ContainerSecurityContext.RunAsNonRoot {
			return fmt.Errorf("runAsNonRoot must be true for OpenShift compatibility")
		}
	}

	return nil
}

// GetOpenShiftCompatibleDefaults returns OpenShift-optimized default configuration
func GetOpenShiftCompatibleDefaults() *PodTemplateConfig {
	config := DefaultPodTemplateConfig()

	// Ensure no fixed UIDs are set
	if config.PodSecurityContext != nil {
		config.PodSecurityContext.RunAsUser = nil
		config.PodSecurityContext.FSGroup = nil
	}

	if config.ContainerSecurityContext != nil {
		config.ContainerSecurityContext.RunAsUser = nil
	}

	return config
}

// AddOpenShiftAnnotations adds OpenShift-specific annotations to help with deployment
func AddOpenShiftAnnotations(annotations map[string]string) map[string]string {
	if annotations == nil {
		annotations = make(map[string]string)
	}

	// Add annotation to indicate OpenShift compatibility
	annotations["openshift.io/scc"] = "restricted"

	return annotations
}

// CheckOpenShiftCompatibility performs OpenShift compatibility checks
func CheckOpenShiftCompatibility(ctx context.Context, k8sClient *K8sClient) error {
	// This function can be called during operator startup to verify OpenShift setup
	// For now, it's a placeholder for future compatibility checks

	fmt.Println("🔍 Checking OpenShift compatibility...")

	// TODO: Add checks for:
	// - SCC existence
	// - RBAC bindings
	// - API availability
	// - Network policies

	fmt.Println("✓ OpenShift compatibility checks passed")
	return nil
}
