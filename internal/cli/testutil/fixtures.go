package testutil

import (
	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/resource"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	"github.com/kubiyabot/cli/internal/controlplane/entities"
)

// SampleQueueConfig creates a sample worker queue configuration for testing
func SampleQueueConfig(name string, maxWorkers int) *entities.WorkerQueueConfig {
	maxWorkersPtr := &maxWorkers
	return &entities.WorkerQueueConfig{
		QueueID:           "test-queue-" + name,
		Name:              name,
		DisplayName:       stringPtr("Test Queue " + name),
		Description:       stringPtr("Test queue for " + name),
		Status:            "active",
		MaxWorkers:        maxWorkersPtr,
		HeartbeatInterval: 30,
		Tags:              []string{"test", "queue"},
		Settings:          make(map[string]interface{}),
		ConfigVersion:     "abc123",
		ConfigUpdatedAt:   "2024-01-01T00:00:00Z",
		EnvironmentID:     "env-test",
		EnvironmentName:   "test-environment",
	}
}

// SampleQueueConfigWithSettings creates a queue config with custom Kubernetes settings
func SampleQueueConfigWithSettings(name string, maxWorkers int, k8sSettings map[string]interface{}) *entities.WorkerQueueConfig {
	config := SampleQueueConfig(name, maxWorkers)
	config.Settings = map[string]interface{}{
		"kubernetes": k8sSettings,
	}
	return config
}

// SampleQueueConfigWithNodeSelector creates a queue config with node selector
func SampleQueueConfigWithNodeSelector(name string, maxWorkers int, nodeSelector map[string]string) *entities.WorkerQueueConfig {
	k8sSettings := map[string]interface{}{
		"nodeSelector": nodeSelector,
	}
	return SampleQueueConfigWithSettings(name, maxWorkers, k8sSettings)
}

// SampleQueueConfigWithResources creates a queue config with custom resource limits
func SampleQueueConfigWithResources(name string, maxWorkers int, requestCPU, requestMemory, limitCPU, limitMemory string) *entities.WorkerQueueConfig {
	k8sSettings := map[string]interface{}{
		"resources": map[string]interface{}{
			"requests": map[string]string{
				"cpu":    requestCPU,
				"memory": requestMemory,
			},
			"limits": map[string]string{
				"cpu":    limitCPU,
				"memory": limitMemory,
			},
		},
	}
	return SampleQueueConfigWithSettings(name, maxWorkers, k8sSettings)
}

// SampleQueueConfigWithTolerations creates a queue config with tolerations
func SampleQueueConfigWithTolerations(name string, maxWorkers int, tolerations []interface{}) *entities.WorkerQueueConfig {
	k8sSettings := map[string]interface{}{
		"tolerations": tolerations,
	}
	return SampleQueueConfigWithSettings(name, maxWorkers, k8sSettings)
}

// SampleDeployment creates a sample Kubernetes Deployment for testing
func SampleDeployment(namespace, name, queueID string, replicas int32) *appsv1.Deployment {
	return &appsv1.Deployment{
		ObjectMeta: metav1.ObjectMeta{
			Name:      name,
			Namespace: namespace,
			Labels: map[string]string{
				"app.kubernetes.io/name":       "kubiya-worker",
				"app.kubernetes.io/component":  "worker",
				"app.kubernetes.io/managed-by": "kubiya-operator",
				"kubiya.ai/queue-id":           queueID,
				"kubiya.ai/queue-name":         name,
			},
		},
		Spec: appsv1.DeploymentSpec{
			Replicas: &replicas,
			Selector: &metav1.LabelSelector{
				MatchLabels: map[string]string{
					"app.kubernetes.io/name": "kubiya-worker",
					"kubiya.ai/queue-id":     queueID,
				},
			},
			Template: corev1.PodTemplateSpec{
				ObjectMeta: metav1.ObjectMeta{
					Labels: map[string]string{
						"app.kubernetes.io/name":       "kubiya-worker",
						"app.kubernetes.io/component":  "worker",
						"app.kubernetes.io/managed-by": "kubiya-operator",
						"kubiya.ai/queue-id":           queueID,
						"kubiya.ai/queue-name":         name,
					},
				},
				Spec: corev1.PodSpec{
					Containers: []corev1.Container{
						{
							Name:  "worker",
							Image: "ghcr.io/kubiyabot/agent-worker:latest",
							Resources: corev1.ResourceRequirements{
								Requests: corev1.ResourceList{
									corev1.ResourceMemory: resource.MustParse("512Mi"),
									corev1.ResourceCPU:    resource.MustParse("250m"),
								},
								Limits: corev1.ResourceList{
									corev1.ResourceMemory: resource.MustParse("2Gi"),
									corev1.ResourceCPU:    resource.MustParse("1000m"),
								},
							},
						},
					},
				},
			},
		},
	}
}

// SampleDeploymentWithNodeSelector creates a deployment with node selector
func SampleDeploymentWithNodeSelector(namespace, name, queueID string, replicas int32, nodeSelector map[string]string) *appsv1.Deployment {
	deployment := SampleDeployment(namespace, name, queueID, replicas)
	deployment.Spec.Template.Spec.NodeSelector = nodeSelector
	return deployment
}

// SampleDeploymentWithTolerations creates a deployment with tolerations
func SampleDeploymentWithTolerations(namespace, name, queueID string, replicas int32, tolerations []corev1.Toleration) *appsv1.Deployment {
	deployment := SampleDeployment(namespace, name, queueID, replicas)
	deployment.Spec.Template.Spec.Tolerations = tolerations
	return deployment
}

// SampleConfigMap creates a sample ConfigMap for testing
func SampleConfigMap(namespace, name string, data map[string]string) *corev1.ConfigMap {
	return &corev1.ConfigMap{
		ObjectMeta: metav1.ObjectMeta{
			Name:      name,
			Namespace: namespace,
		},
		Data: data,
	}
}

// SampleSecret creates a sample Secret for testing
func SampleSecret(namespace, name string, data map[string][]byte) *corev1.Secret {
	return &corev1.Secret{
		ObjectMeta: metav1.ObjectMeta{
			Name:      name,
			Namespace: namespace,
		},
		Data: data,
	}
}

// SampleNamespace creates a sample Namespace for testing
func SampleNamespace(name string) *corev1.Namespace {
	return &corev1.Namespace{
		ObjectMeta: metav1.ObjectMeta{
			Name: name,
			Labels: map[string]string{
				"app.kubernetes.io/name": "kubiya",
			},
		},
	}
}

// SampleToleration creates a sample toleration
func SampleToleration(key, value, effect string) corev1.Toleration {
	return corev1.Toleration{
		Key:      key,
		Operator: corev1.TolerationOpEqual,
		Value:    value,
		Effect:   corev1.TaintEffect(effect),
	}
}

// SampleNodeSelectorRequirement creates a sample node selector requirement
func SampleNodeSelectorRequirement(key, operator string, values []string) corev1.NodeSelectorRequirement {
	return corev1.NodeSelectorRequirement{
		Key:      key,
		Operator: corev1.NodeSelectorOperator(operator),
		Values:   values,
	}
}

// SampleNodeAffinity creates a sample node affinity
func SampleNodeAffinity(matchExpressions []corev1.NodeSelectorRequirement) *corev1.Affinity {
	return &corev1.Affinity{
		NodeAffinity: &corev1.NodeAffinity{
			RequiredDuringSchedulingIgnoredDuringExecution: &corev1.NodeSelector{
				NodeSelectorTerms: []corev1.NodeSelectorTerm{
					{
						MatchExpressions: matchExpressions,
					},
				},
			},
		},
	}
}

// SamplePodAntiAffinity creates a sample pod anti-affinity for spreading across zones
func SamplePodAntiAffinity(labelKey, labelValue string) *corev1.Affinity {
	return &corev1.Affinity{
		PodAntiAffinity: &corev1.PodAntiAffinity{
			PreferredDuringSchedulingIgnoredDuringExecution: []corev1.WeightedPodAffinityTerm{
				{
					Weight: 100,
					PodAffinityTerm: corev1.PodAffinityTerm{
						LabelSelector: &metav1.LabelSelector{
							MatchLabels: map[string]string{
								labelKey: labelValue,
							},
						},
						TopologyKey: "topology.kubernetes.io/zone",
					},
				},
			},
		},
	}
}

// SampleResourceRequirements creates sample resource requirements
func SampleResourceRequirements(requestCPU, requestMemory, limitCPU, limitMemory string) corev1.ResourceRequirements {
	return corev1.ResourceRequirements{
		Requests: corev1.ResourceList{
			corev1.ResourceCPU:    resource.MustParse(requestCPU),
			corev1.ResourceMemory: resource.MustParse(requestMemory),
		},
		Limits: corev1.ResourceList{
			corev1.ResourceCPU:    resource.MustParse(limitCPU),
			corev1.ResourceMemory: resource.MustParse(limitMemory),
		},
	}
}

// SamplePodSecurityContext creates a sample pod security context
func SamplePodSecurityContext(runAsUser, fsGroup int64) *corev1.PodSecurityContext {
	return &corev1.PodSecurityContext{
		RunAsUser:    &runAsUser,
		RunAsNonRoot: boolPtr(true),
		FSGroup:      &fsGroup,
	}
}

// SampleContainerSecurityContext creates a sample container security context
func SampleContainerSecurityContext(runAsUser int64, readOnlyRootFilesystem bool) *corev1.SecurityContext {
	return &corev1.SecurityContext{
		RunAsUser:                &runAsUser,
		RunAsNonRoot:             boolPtr(true),
		ReadOnlyRootFilesystem:   &readOnlyRootFilesystem,
		AllowPrivilegeEscalation: boolPtr(false),
		Capabilities: &corev1.Capabilities{
			Drop: []corev1.Capability{"ALL"},
		},
	}
}

// SampleVolume creates a sample volume
func SampleVolume(name, pvcName string) corev1.Volume {
	return corev1.Volume{
		Name: name,
		VolumeSource: corev1.VolumeSource{
			PersistentVolumeClaim: &corev1.PersistentVolumeClaimVolumeSource{
				ClaimName: pvcName,
			},
		},
	}
}

// SampleVolumeMount creates a sample volume mount
func SampleVolumeMount(name, mountPath string) corev1.VolumeMount {
	return corev1.VolumeMount{
		Name:      name,
		MountPath: mountPath,
	}
}

// SampleImagePullSecret creates a sample image pull secret reference
func SampleImagePullSecret(name string) corev1.LocalObjectReference {
	return corev1.LocalObjectReference{
		Name: name,
	}
}

// SampleEnvVar creates a sample environment variable
func SampleEnvVar(name, value string) corev1.EnvVar {
	return corev1.EnvVar{
		Name:  name,
		Value: value,
	}
}

// SampleEnvVarFromSecret creates a sample environment variable from a secret
func SampleEnvVarFromSecret(name, secretName, secretKey string) corev1.EnvVar {
	return corev1.EnvVar{
		Name: name,
		ValueFrom: &corev1.EnvVarSource{
			SecretKeyRef: &corev1.SecretKeySelector{
				LocalObjectReference: corev1.LocalObjectReference{
					Name: secretName,
				},
				Key: secretKey,
			},
		},
	}
}

// Helper functions

func stringPtr(s string) *string {
	return &s
}

func boolPtr(b bool) *bool {
	return &b
}

func int32Ptr(i int32) *int32 {
	return &i
}

func int64Ptr(i int64) *int64 {
	return &i
}
