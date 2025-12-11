package cli

import (
	"encoding/json"
	"fmt"

	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/resource"
)

// PodTemplateConfig holds all customizable pod template options
// that can be configured through the control plane queue settings
type PodTemplateConfig struct {
	// Scheduling
	NodeSelector      map[string]string   `json:"nodeSelector,omitempty"`
	Affinity          *corev1.Affinity    `json:"affinity,omitempty"`
	Tolerations       []corev1.Toleration `json:"tolerations,omitempty"`
	PriorityClassName string              `json:"priorityClassName,omitempty"`

	// Resources
	Resources *ResourceConfig `json:"resources,omitempty"`

	// Security
	PodSecurityContext       *corev1.PodSecurityContext       `json:"podSecurityContext,omitempty"`
	ContainerSecurityContext *corev1.SecurityContext          `json:"containerSecurityContext,omitempty"`
	ServiceAccountName       string                           `json:"serviceAccountName,omitempty"`
	ImagePullSecrets         []corev1.LocalObjectReference    `json:"imagePullSecrets,omitempty"`

	// Image
	Image           string              `json:"image,omitempty"`
	ImagePullPolicy corev1.PullPolicy   `json:"imagePullPolicy,omitempty"`

	// Metadata
	PodAnnotations map[string]string `json:"podAnnotations,omitempty"`
	PodLabels      map[string]string `json:"podLabels,omitempty"`

	// Storage
	Volumes      []corev1.Volume      `json:"volumes,omitempty"`
	VolumeMounts []corev1.VolumeMount `json:"volumeMounts,omitempty"`

	// Additional Containers
	InitContainers []corev1.Container `json:"initContainers,omitempty"`
	Sidecars       []corev1.Container `json:"sidecars,omitempty"`

	// Environment
	ExtraEnv []corev1.EnvVar `json:"extraEnv,omitempty"`

	// Networking
	DNSPolicy corev1.DNSPolicy `json:"dnsPolicy,omitempty"`
	DNSConfig *corev1.PodDNSConfig `json:"dnsConfig,omitempty"`
	HostNetwork bool `json:"hostNetwork,omitempty"`

	// Lifecycle
	TerminationGracePeriodSeconds *int64 `json:"terminationGracePeriodSeconds,omitempty"`
}

// ResourceConfig holds resource requests and limits
type ResourceConfig struct {
	Requests corev1.ResourceList `json:"requests,omitempty"`
	Limits   corev1.ResourceList `json:"limits,omitempty"`
}

// ParsePodTemplateConfig extracts pod configuration from queue settings
// Expected structure: settings["kubernetes"] = { ... pod config ... }
func ParsePodTemplateConfig(settings map[string]interface{}) (*PodTemplateConfig, error) {
	if settings == nil {
		return DefaultPodTemplateConfig(), nil
	}

	// Extract kubernetes settings
	k8sSettings, ok := settings["kubernetes"]
	if !ok {
		return DefaultPodTemplateConfig(), nil
	}

	// Convert to JSON and back to struct for type safety
	jsonData, err := json.Marshal(k8sSettings)
	if err != nil {
		return nil, fmt.Errorf("failed to marshal kubernetes settings: %w", err)
	}

	config := &PodTemplateConfig{}
	if err := json.Unmarshal(jsonData, config); err != nil {
		return nil, fmt.Errorf("failed to unmarshal pod template config: %w", err)
	}

	// Validate the configuration
	if err := config.Validate(); err != nil {
		return nil, fmt.Errorf("invalid pod template config: %w", err)
	}

	return config, nil
}

// ParsePodTemplateConfigSafe is like ParsePodTemplateConfig but returns defaults on error
// and logs warnings instead of returning errors
func ParsePodTemplateConfigSafe(settings map[string]interface{}, logger func(string, ...interface{})) *PodTemplateConfig {
	config, err := ParsePodTemplateConfig(settings)
	if err != nil {
		if logger != nil {
			logger("⚠️  Failed to parse pod template config: %v, using defaults", err)
		}
		return DefaultPodTemplateConfig()
	}
	return config
}

// DefaultPodTemplateConfig returns default pod template configuration
// This configuration is compatible with both Kubernetes and OpenShift
func DefaultPodTemplateConfig() *PodTemplateConfig {
	return &PodTemplateConfig{
		Resources: &ResourceConfig{
			Requests: corev1.ResourceList{
				corev1.ResourceMemory: resource.MustParse("512Mi"),
				corev1.ResourceCPU:    resource.MustParse("250m"),
			},
			Limits: corev1.ResourceList{
				corev1.ResourceMemory: resource.MustParse("2Gi"),
				corev1.ResourceCPU:    resource.MustParse("1000m"),
			},
		},
		PodSecurityContext: &corev1.PodSecurityContext{
			// Note: Do not set runAsUser or fsGroup here to support OpenShift's
			// restricted SCC which assigns random UIDs. Users can override via settings.
		},
		ContainerSecurityContext: &corev1.SecurityContext{
			AllowPrivilegeEscalation: boolPtr(false),
			RunAsNonRoot:             boolPtr(true),
			// Note: Do not set runAsUser here to support OpenShift's restricted SCC
			// which assigns random UIDs. Users can override via settings.
			Capabilities: &corev1.Capabilities{
				Drop: []corev1.Capability{"ALL"},
			},
		},
		ImagePullPolicy: corev1.PullAlways,
	}
}

// Validate checks if the pod template configuration is valid
func (c *PodTemplateConfig) Validate() error {
	// Validate resources
	if c.Resources != nil {
		if err := validateResources(c.Resources); err != nil {
			return fmt.Errorf("resources validation failed: %w", err)
		}
	}

	// Validate labels and annotations
	if err := validateLabels(c.PodLabels); err != nil {
		return fmt.Errorf("pod labels validation failed: %w", err)
	}
	if err := validateAnnotations(c.PodAnnotations); err != nil {
		return fmt.Errorf("pod annotations validation failed: %w", err)
	}

	// Validate node selector
	if err := validateLabels(c.NodeSelector); err != nil {
		return fmt.Errorf("node selector validation failed: %w", err)
	}

	// Validate image pull policy
	if c.ImagePullPolicy != "" {
		validPolicies := map[corev1.PullPolicy]bool{
			corev1.PullAlways:       true,
			corev1.PullNever:        true,
			corev1.PullIfNotPresent: true,
		}
		if !validPolicies[c.ImagePullPolicy] {
			return fmt.Errorf("invalid image pull policy: %s", c.ImagePullPolicy)
		}
	}

	return nil
}

// ApplyToPodTemplate applies the configuration to a pod template spec
func (c *PodTemplateConfig) ApplyToPodTemplate(template *corev1.PodTemplateSpec, baseEnv []corev1.EnvVar) {
	if template == nil {
		return
	}

	// Apply scheduling configuration
	if c.NodeSelector != nil {
		template.Spec.NodeSelector = mergeMaps(template.Spec.NodeSelector, c.NodeSelector)
	}
	if c.Affinity != nil {
		template.Spec.Affinity = c.Affinity
	}
	if c.Tolerations != nil {
		template.Spec.Tolerations = append(template.Spec.Tolerations, c.Tolerations...)
	}
	if c.PriorityClassName != "" {
		template.Spec.PriorityClassName = c.PriorityClassName
	}

	// Apply security configuration
	if c.PodSecurityContext != nil {
		template.Spec.SecurityContext = c.PodSecurityContext
	}
	if c.ServiceAccountName != "" {
		template.Spec.ServiceAccountName = c.ServiceAccountName
	}
	if c.ImagePullSecrets != nil {
		template.Spec.ImagePullSecrets = append(template.Spec.ImagePullSecrets, c.ImagePullSecrets...)
	}

	// Apply metadata
	if template.ObjectMeta.Annotations == nil {
		template.ObjectMeta.Annotations = make(map[string]string)
	}
	if template.ObjectMeta.Labels == nil {
		template.ObjectMeta.Labels = make(map[string]string)
	}
	for k, v := range c.PodAnnotations {
		// Don't overwrite operator-managed annotations
		if _, exists := template.ObjectMeta.Annotations[k]; !exists {
			template.ObjectMeta.Annotations[k] = v
		}
	}
	for k, v := range c.PodLabels {
		// Don't overwrite operator-managed labels
		if _, exists := template.ObjectMeta.Labels[k]; !exists {
			template.ObjectMeta.Labels[k] = v
		}
	}

	// Apply networking
	if c.DNSPolicy != "" {
		template.Spec.DNSPolicy = c.DNSPolicy
	}
	if c.DNSConfig != nil {
		template.Spec.DNSConfig = c.DNSConfig
	}
	if c.HostNetwork {
		template.Spec.HostNetwork = true
	}

	// Apply lifecycle
	if c.TerminationGracePeriodSeconds != nil {
		template.Spec.TerminationGracePeriodSeconds = c.TerminationGracePeriodSeconds
	}

	// Apply storage
	if c.Volumes != nil {
		template.Spec.Volumes = append(template.Spec.Volumes, c.Volumes...)
	}

	// Apply init containers
	if c.InitContainers != nil {
		template.Spec.InitContainers = append(template.Spec.InitContainers, c.InitContainers...)
	}

	// Apply to main container (assumed to be first container)
	if len(template.Spec.Containers) > 0 {
		container := &template.Spec.Containers[0]

		// Apply image configuration
		if c.Image != "" {
			container.Image = c.Image
		}
		if c.ImagePullPolicy != "" {
			container.ImagePullPolicy = c.ImagePullPolicy
		}

		// Apply resources
		if c.Resources != nil {
			if c.Resources.Requests != nil {
				if container.Resources.Requests == nil {
					container.Resources.Requests = corev1.ResourceList{}
				}
				for k, v := range c.Resources.Requests {
					container.Resources.Requests[k] = v
				}
			}
			if c.Resources.Limits != nil {
				if container.Resources.Limits == nil {
					container.Resources.Limits = corev1.ResourceList{}
				}
				for k, v := range c.Resources.Limits {
					container.Resources.Limits[k] = v
				}
			}
		}

		// Apply security context
		if c.ContainerSecurityContext != nil {
			container.SecurityContext = c.ContainerSecurityContext
		}

		// Apply volume mounts
		if c.VolumeMounts != nil {
			container.VolumeMounts = append(container.VolumeMounts, c.VolumeMounts...)
		}

		// Apply extra environment variables
		if c.ExtraEnv != nil {
			// Merge with existing env vars, custom vars take precedence
			envMap := make(map[string]corev1.EnvVar)
			for _, env := range baseEnv {
				envMap[env.Name] = env
			}
			for _, env := range c.ExtraEnv {
				envMap[env.Name] = env
			}

			// Convert back to slice
			newEnv := make([]corev1.EnvVar, 0, len(envMap))
			for _, env := range envMap {
				newEnv = append(newEnv, env)
			}
			container.Env = newEnv
		}
	}

	// Apply sidecar containers
	if c.Sidecars != nil {
		template.Spec.Containers = append(template.Spec.Containers, c.Sidecars...)
	}
}

// validateResources validates resource requests and limits
func validateResources(res *ResourceConfig) error {
	if res == nil {
		return nil
	}

	// Check that limits are greater than or equal to requests
	if res.Requests != nil && res.Limits != nil {
		for resourceName, requestQty := range res.Requests {
			if limitQty, exists := res.Limits[resourceName]; exists {
				if limitQty.Cmp(requestQty) < 0 {
					return fmt.Errorf("resource limit for %s (%s) is less than request (%s)",
						resourceName, limitQty.String(), requestQty.String())
				}
			}
		}
	}

	return nil
}

// validateLabels validates label keys and values
func validateLabels(labels map[string]string) error {
	// Basic validation - Kubernetes has complex rules but we'll do basic checks
	for key, value := range labels {
		if len(key) == 0 {
			return fmt.Errorf("label key cannot be empty")
		}
		if len(key) > 253 {
			return fmt.Errorf("label key too long: %s", key)
		}
		if len(value) > 63 {
			return fmt.Errorf("label value too long for key %s: %s", key, value)
		}
	}
	return nil
}

// validateAnnotations validates annotation keys and values
func validateAnnotations(annotations map[string]string) error {
	// Annotations have similar rules to labels but more relaxed
	for key := range annotations {
		if len(key) == 0 {
			return fmt.Errorf("annotation key cannot be empty")
		}
		if len(key) > 253 {
			return fmt.Errorf("annotation key too long: %s", key)
		}
	}
	return nil
}

// mergeMaps merges two string maps, with values from the second map taking precedence
func mergeMaps(base, override map[string]string) map[string]string {
	result := make(map[string]string)
	for k, v := range base {
		result[k] = v
	}
	for k, v := range override {
		result[k] = v
	}
	return result
}
