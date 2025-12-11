package cli

import (
	"fmt"

	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/util/intstr"

	"github.com/kubiyabot/cli/internal/controlplane/entities"
)

// WorkerDeploymentTemplate generates a Kubernetes Deployment for a worker queue
func WorkerDeploymentTemplate(
	queue *entities.WorkerQueueConfig,
	namespace string,
	apiKey string,
	controlPlaneURL string,
	enableAutoUpdate bool,
	updateCheckInterval string,
) *appsv1.Deployment {
	// Generate deployment name from queue name
	deploymentName := fmt.Sprintf("kubiya-worker-%s", queue.Name)

	// Determine replicas from max_workers
	replicas := int32(1) // Default
	if queue.MaxWorkers != nil && *queue.MaxWorkers > 0 {
		replicas = int32(*queue.MaxWorkers)
	}

	// Parse pod template configuration from queue settings
	podConfig := ParsePodTemplateConfigSafe(queue.Settings, func(format string, args ...interface{}) {
		fmt.Printf(format+"\n", args...)
	})

	// Labels for the deployment (operator-managed labels)
	labels := map[string]string{
		"app.kubernetes.io/name":       "kubiya-worker",
		"app.kubernetes.io/component":  "worker",
		"app.kubernetes.io/managed-by": "kubiya-operator",
		"kubiya.ai/queue-id":           queue.QueueID,
		"kubiya.ai/queue-name":         queue.Name,
		"kubiya.ai/environment":        queue.EnvironmentName,
	}

	// Add custom tags as labels
	for _, tag := range queue.Tags {
		labels[fmt.Sprintf("kubiya.ai/tag-%s", tag)] = "true"
	}

	// Merge with custom pod labels from config
	if podConfig.PodLabels != nil {
		for k, v := range podConfig.PodLabels {
			// Don't allow overriding operator-managed labels
			if _, exists := labels[k]; !exists {
				labels[k] = v
			}
		}
	}

	// Environment variables
	env := []corev1.EnvVar{
		{
			Name:  "KUBIYA_API_KEY",
			Value: apiKey,
		},
		{
			Name:  "QUEUE_ID",
			Value: queue.QueueID,
		},
		{
			Name:  "CONTROL_PLANE_URL",
			Value: controlPlaneURL,
		},
	}

	// Add auto-update environment variables if enabled
	if enableAutoUpdate {
		env = append(env,
			corev1.EnvVar{
				Name:  "KUBIYA_WORKER_AUTOUPDATE_ENABLED",
				Value: "true",
			},
			corev1.EnvVar{
				Name:  "KUBIYA_WORKER_UPDATE_CHECK_INTERVAL",
				Value: updateCheckInterval,
			},
		)
	}

	deployment := &appsv1.Deployment{
		ObjectMeta: metav1.ObjectMeta{
			Name:      deploymentName,
			Namespace: namespace,
			Labels:    labels,
			Annotations: map[string]string{
				"kubiya.ai/config-version":            queue.ConfigVersion,
				"kubiya.ai/config-updated-at":         queue.ConfigUpdatedAt,
				"kubiya.ai/recommended-package-version": getStringOrDefault(queue.RecommendedPackageVersion, "latest"),
			},
		},
		Spec: appsv1.DeploymentSpec{
			Replicas: &replicas,
			Selector: &metav1.LabelSelector{
				MatchLabels: map[string]string{
					"app.kubernetes.io/name":      "kubiya-worker",
					"kubiya.ai/queue-id":          queue.QueueID,
				},
			},
			Strategy: appsv1.DeploymentStrategy{
				Type: appsv1.RollingUpdateDeploymentStrategyType,
				RollingUpdate: &appsv1.RollingUpdateDeployment{
					MaxUnavailable: func() *intstr.IntOrString {
						val := intstr.FromInt(0)
						return &val
					}(),
					MaxSurge: func() *intstr.IntOrString {
						val := intstr.FromInt(1)
						return &val
					}(),
				},
			},
			Template: corev1.PodTemplateSpec{
				ObjectMeta: metav1.ObjectMeta{
					Labels: labels,
					Annotations: map[string]string{
						"kubiya.ai/config-version": queue.ConfigVersion,
					},
				},
				Spec: corev1.PodSpec{
					Containers: []corev1.Container{
						{
							Name: "worker",
							// Image: default, will be overridden by ApplyToPodTemplate if custom image specified
							Image: "ghcr.io/kubiyabot/agent-worker:latest",
							Command: []string{
								"kubiya-control-plane-worker",
								"--queue-id", queue.QueueID,
							},
							Env: env, // Will be merged with ExtraEnv by ApplyToPodTemplate
							// Resources will be set by ApplyToPodTemplate
							Resources: corev1.ResourceRequirements{
								Requests: podConfig.Resources.Requests,
								Limits:   podConfig.Resources.Limits,
							},
							LivenessProbe: &corev1.Probe{
								ProbeHandler: corev1.ProbeHandler{
									HTTPGet: &corev1.HTTPGetAction{
										Path: "/health",
										Port: intstr.FromInt(8080),
									},
								},
								InitialDelaySeconds: 30,
								PeriodSeconds:       30,
								TimeoutSeconds:      10,
								FailureThreshold:    3,
							},
							ReadinessProbe: &corev1.Probe{
								ProbeHandler: corev1.ProbeHandler{
									HTTPGet: &corev1.HTTPGetAction{
										Path: "/health",
										Port: intstr.FromInt(8080),
									},
								},
								InitialDelaySeconds: 10,
								PeriodSeconds:       10,
								TimeoutSeconds:      5,
								FailureThreshold:    3,
							},
							// SecurityContext will be set by ApplyToPodTemplate
							SecurityContext: podConfig.ContainerSecurityContext,
						},
					},
					RestartPolicy: corev1.RestartPolicyAlways,
					// SecurityContext will be set by ApplyToPodTemplate
					SecurityContext: podConfig.PodSecurityContext,
				},
			},
		},
	}

	// Apply pod template customizations (node selector, affinity, tolerations, etc.)
	podConfig.ApplyToPodTemplate(&deployment.Spec.Template, env)

	return deployment
}

// WorkerConfigMapTemplate generates a ConfigMap for worker configuration
func WorkerConfigMapTemplate(
	queue *entities.WorkerQueueConfig,
	namespace string,
) *corev1.ConfigMap {
	configMapName := fmt.Sprintf("kubiya-worker-%s-config", queue.Name)

	return &corev1.ConfigMap{
		ObjectMeta: metav1.ObjectMeta{
			Name:      configMapName,
			Namespace: namespace,
			Labels: map[string]string{
				"app.kubernetes.io/name":       "kubiya-worker",
				"app.kubernetes.io/component":  "config",
				"app.kubernetes.io/managed-by": "kubiya-operator",
				"kubiya.ai/queue-id":           queue.QueueID,
			},
		},
		Data: map[string]string{
			"queue-id":            queue.QueueID,
			"queue-name":          queue.Name,
			"environment-name":    queue.EnvironmentName,
			"heartbeat-interval":  fmt.Sprintf("%d", queue.HeartbeatInterval),
			"config-version":      queue.ConfigVersion,
		},
	}
}

// Helper functions

func boolPtr(b bool) *bool {
	return &b
}

func int64Ptr(i int64) *int64 {
	return &i
}

func getStringOrDefault(s *string, defaultVal string) string {
	if s != nil && *s != "" {
		return *s
	}
	return defaultVal
}
