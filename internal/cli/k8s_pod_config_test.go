package cli

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/resource"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

func TestParsePodTemplateConfig_EmptySettings(t *testing.T) {
	// Test with nil settings
	config, err := ParsePodTemplateConfig(nil)
	require.NoError(t, err)
	assert.NotNil(t, config)
	assert.NotNil(t, config.Resources)

	// Test with empty settings
	config, err = ParsePodTemplateConfig(make(map[string]interface{}))
	require.NoError(t, err)
	assert.NotNil(t, config)
}

func TestParsePodTemplateConfig_NodeSelector(t *testing.T) {
	settings := map[string]interface{}{
		"kubernetes": map[string]interface{}{
			"nodeSelector": map[string]interface{}{
				"node-type": "gpu",
				"zone":      "us-west-2a",
			},
		},
	}

	config, err := ParsePodTemplateConfig(settings)
	require.NoError(t, err)
	assert.NotNil(t, config)
	assert.Equal(t, "gpu", config.NodeSelector["node-type"])
	assert.Equal(t, "us-west-2a", config.NodeSelector["zone"])
}

func TestParsePodTemplateConfig_Tolerations(t *testing.T) {
	settings := map[string]interface{}{
		"kubernetes": map[string]interface{}{
			"tolerations": []interface{}{
				map[string]interface{}{
					"key":      "dedicated",
					"operator": "Equal",
					"value":    "gpu",
					"effect":   "NoSchedule",
				},
			},
		},
	}

	config, err := ParsePodTemplateConfig(settings)
	require.NoError(t, err)
	assert.Len(t, config.Tolerations, 1)
	assert.Equal(t, "dedicated", config.Tolerations[0].Key)
	assert.Equal(t, corev1.TolerationOpEqual, config.Tolerations[0].Operator)
	assert.Equal(t, "gpu", config.Tolerations[0].Value)
	assert.Equal(t, corev1.TaintEffectNoSchedule, config.Tolerations[0].Effect)
}

func TestParsePodTemplateConfig_Resources(t *testing.T) {
	settings := map[string]interface{}{
		"kubernetes": map[string]interface{}{
			"resources": map[string]interface{}{
				"requests": map[string]interface{}{
					"cpu":    "2",
					"memory": "4Gi",
				},
				"limits": map[string]interface{}{
					"cpu":    "4",
					"memory": "8Gi",
				},
			},
		},
	}

	config, err := ParsePodTemplateConfig(settings)
	require.NoError(t, err)
	require.NotNil(t, config.Resources)

	// Check resource requests
	cpuReq := config.Resources.Requests[corev1.ResourceCPU]
	memReq := config.Resources.Requests[corev1.ResourceMemory]
	assert.Equal(t, "2", cpuReq.String())
	assert.Equal(t, "4Gi", memReq.String())

	// Check resource limits
	cpuLimit := config.Resources.Limits[corev1.ResourceCPU]
	memLimit := config.Resources.Limits[corev1.ResourceMemory]
	assert.Equal(t, "4", cpuLimit.String())
	assert.Equal(t, "8Gi", memLimit.String())
}

func TestParsePodTemplateConfig_PodAnnotations(t *testing.T) {
	settings := map[string]interface{}{
		"kubernetes": map[string]interface{}{
			"podAnnotations": map[string]interface{}{
				"prometheus.io/scrape": "true",
				"prometheus.io/port":   "8080",
			},
		},
	}

	config, err := ParsePodTemplateConfig(settings)
	require.NoError(t, err)
	assert.Equal(t, "true", config.PodAnnotations["prometheus.io/scrape"])
	assert.Equal(t, "8080", config.PodAnnotations["prometheus.io/port"])
}

func TestParsePodTemplateConfig_ImageConfiguration(t *testing.T) {
	settings := map[string]interface{}{
		"kubernetes": map[string]interface{}{
			"image":           "myregistry.io/worker:v1.2.3",
			"imagePullPolicy": "Always",
			"imagePullSecrets": []interface{}{
				map[string]interface{}{"name": "regcred"},
			},
		},
	}

	config, err := ParsePodTemplateConfig(settings)
	require.NoError(t, err)
	assert.Equal(t, "myregistry.io/worker:v1.2.3", config.Image)
	assert.Equal(t, corev1.PullAlways, config.ImagePullPolicy)
	assert.Len(t, config.ImagePullSecrets, 1)
	assert.Equal(t, "regcred", config.ImagePullSecrets[0].Name)
}

func TestParsePodTemplateConfig_SecurityContext(t *testing.T) {
	settings := map[string]interface{}{
		"kubernetes": map[string]interface{}{
			"podSecurityContext": map[string]interface{}{
				"runAsUser":    2000,
				"runAsNonRoot": true,
				"fsGroup":      2000,
			},
			"containerSecurityContext": map[string]interface{}{
				"allowPrivilegeEscalation": false,
				"readOnlyRootFilesystem":   true,
				"runAsNonRoot":             true,
			},
		},
	}

	config, err := ParsePodTemplateConfig(settings)
	require.NoError(t, err)
	require.NotNil(t, config.PodSecurityContext)
	assert.Equal(t, int64(2000), *config.PodSecurityContext.RunAsUser)
	assert.True(t, *config.PodSecurityContext.RunAsNonRoot)
	assert.Equal(t, int64(2000), *config.PodSecurityContext.FSGroup)

	require.NotNil(t, config.ContainerSecurityContext)
	assert.False(t, *config.ContainerSecurityContext.AllowPrivilegeEscalation)
	assert.True(t, *config.ContainerSecurityContext.ReadOnlyRootFilesystem)
}

func TestParsePodTemplateConfig_InvalidConfig(t *testing.T) {
	tests := []struct {
		name     string
		settings map[string]interface{}
		wantErr  bool
	}{
		{
			name: "invalid resource limits less than requests",
			settings: map[string]interface{}{
				"kubernetes": map[string]interface{}{
					"resources": map[string]interface{}{
						"requests": map[string]interface{}{
							"cpu": "4",
						},
						"limits": map[string]interface{}{
							"cpu": "2",
						},
					},
				},
			},
			wantErr: true,
		},
		{
			name: "label key too long",
			settings: map[string]interface{}{
				"kubernetes": map[string]interface{}{
					"podLabels": map[string]interface{}{
						string(make([]byte, 300)): "value",
					},
				},
			},
			wantErr: true,
		},
		{
			name: "invalid image pull policy",
			settings: map[string]interface{}{
				"kubernetes": map[string]interface{}{
					"imagePullPolicy": "InvalidPolicy",
				},
			},
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			_, err := ParsePodTemplateConfig(tt.settings)
			if tt.wantErr {
				assert.Error(t, err)
			} else {
				assert.NoError(t, err)
			}
		})
	}
}

func TestDefaultPodTemplateConfig(t *testing.T) {
	config := DefaultPodTemplateConfig()

	assert.NotNil(t, config)
	assert.NotNil(t, config.Resources)
	assert.NotNil(t, config.PodSecurityContext)
	assert.NotNil(t, config.ContainerSecurityContext)
	assert.Equal(t, corev1.PullAlways, config.ImagePullPolicy)

	// Check default resource requests
	memReq := config.Resources.Requests[corev1.ResourceMemory]
	cpuReq := config.Resources.Requests[corev1.ResourceCPU]
	assert.Equal(t, "512Mi", memReq.String())
	assert.Equal(t, "250m", cpuReq.String())

	// Check default resource limits
	memLimit := config.Resources.Limits[corev1.ResourceMemory]
	cpuLimit := config.Resources.Limits[corev1.ResourceCPU]
	assert.Equal(t, "2Gi", memLimit.String())
	assert.Equal(t, "1", cpuLimit.String())

	// Check OpenShift compatibility - no fixed UIDs
	assert.Nil(t, config.PodSecurityContext.RunAsUser, "Should not set runAsUser for OpenShift compatibility")
	assert.Nil(t, config.PodSecurityContext.FSGroup, "Should not set fsGroup for OpenShift compatibility")
	assert.Nil(t, config.ContainerSecurityContext.RunAsUser, "Should not set runAsUser for OpenShift compatibility")

	// Check security settings that should be set
	assert.NotNil(t, config.ContainerSecurityContext.RunAsNonRoot)
	assert.True(t, *config.ContainerSecurityContext.RunAsNonRoot)
	assert.NotNil(t, config.ContainerSecurityContext.AllowPrivilegeEscalation)
	assert.False(t, *config.ContainerSecurityContext.AllowPrivilegeEscalation)
}

func TestValidateResources(t *testing.T) {
	tests := []struct {
		name    string
		res     *ResourceConfig
		wantErr bool
	}{
		{
			name: "valid resources",
			res: &ResourceConfig{
				Requests: corev1.ResourceList{
					corev1.ResourceCPU:    resource.MustParse("1"),
					corev1.ResourceMemory: resource.MustParse("1Gi"),
				},
				Limits: corev1.ResourceList{
					corev1.ResourceCPU:    resource.MustParse("2"),
					corev1.ResourceMemory: resource.MustParse("2Gi"),
				},
			},
			wantErr: false,
		},
		{
			name: "limits less than requests",
			res: &ResourceConfig{
				Requests: corev1.ResourceList{
					corev1.ResourceCPU: resource.MustParse("2"),
				},
				Limits: corev1.ResourceList{
					corev1.ResourceCPU: resource.MustParse("1"),
				},
			},
			wantErr: true,
		},
		{
			name: "limits equal to requests (valid)",
			res: &ResourceConfig{
				Requests: corev1.ResourceList{
					corev1.ResourceCPU: resource.MustParse("2"),
				},
				Limits: corev1.ResourceList{
					corev1.ResourceCPU: resource.MustParse("2"),
				},
			},
			wantErr: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := validateResources(tt.res)
			if tt.wantErr {
				assert.Error(t, err)
			} else {
				assert.NoError(t, err)
			}
		})
	}
}

func TestApplyToPodTemplate(t *testing.T) {
	// Create a base pod template
	template := &corev1.PodTemplateSpec{
		ObjectMeta: metav1.ObjectMeta{
			Labels: map[string]string{
				"app": "test",
			},
			Annotations: map[string]string{
				"existing": "annotation",
			},
		},
		Spec: corev1.PodSpec{
			Containers: []corev1.Container{
				{
					Name:  "main",
					Image: "base:latest",
					Resources: corev1.ResourceRequirements{
						Requests: corev1.ResourceList{
							corev1.ResourceCPU: resource.MustParse("100m"),
						},
					},
				},
			},
		},
	}

	// Create pod config with customizations
	config := &PodTemplateConfig{
		NodeSelector: map[string]string{
			"node-type": "gpu",
		},
		Tolerations: []corev1.Toleration{
			{
				Key:      "dedicated",
				Operator: corev1.TolerationOpEqual,
				Value:    "gpu",
				Effect:   corev1.TaintEffectNoSchedule,
			},
		},
		PodAnnotations: map[string]string{
			"custom": "annotation",
		},
		PodLabels: map[string]string{
			"custom": "label",
		},
		Resources: &ResourceConfig{
			Requests: corev1.ResourceList{
				corev1.ResourceMemory: resource.MustParse("4Gi"),
			},
			Limits: corev1.ResourceList{
				corev1.ResourceMemory: resource.MustParse("8Gi"),
			},
		},
		Image: "custom:v1.0.0",
		ServiceAccountName: "custom-sa",
	}

	baseEnv := []corev1.EnvVar{
		{Name: "BASE_ENV", Value: "value"},
	}

	// Apply the config
	config.ApplyToPodTemplate(template, baseEnv)

	// Verify node selector was applied
	assert.Equal(t, "gpu", template.Spec.NodeSelector["node-type"])

	// Verify tolerations were applied
	assert.Len(t, template.Spec.Tolerations, 1)
	assert.Equal(t, "dedicated", template.Spec.Tolerations[0].Key)

	// Verify annotations were merged (not overwriting existing)
	assert.Equal(t, "annotation", template.ObjectMeta.Annotations["existing"])
	assert.Equal(t, "annotation", template.ObjectMeta.Annotations["custom"])

	// Verify labels were merged (not overwriting existing)
	assert.Equal(t, "test", template.ObjectMeta.Labels["app"])
	assert.Equal(t, "label", template.ObjectMeta.Labels["custom"])

	// Verify resources were applied
	container := template.Spec.Containers[0]
	memReq := container.Resources.Requests[corev1.ResourceMemory]
	memLimit := container.Resources.Limits[corev1.ResourceMemory]
	assert.Equal(t, "4Gi", memReq.String())
	assert.Equal(t, "8Gi", memLimit.String())

	// Verify image was updated
	assert.Equal(t, "custom:v1.0.0", container.Image)

	// Verify service account was applied
	assert.Equal(t, "custom-sa", template.Spec.ServiceAccountName)
}

func TestParsePodTemplateConfigSafe(t *testing.T) {
	tests := []struct {
		name           string
		settings       map[string]interface{}
		expectDefaults bool
	}{
		{
			name:           "nil settings",
			settings:       nil,
			expectDefaults: true,
		},
		{
			name: "invalid JSON",
			settings: map[string]interface{}{
				"kubernetes": "not a map",
			},
			expectDefaults: true,
		},
		{
			name: "invalid config",
			settings: map[string]interface{}{
				"kubernetes": map[string]interface{}{
					"resources": map[string]interface{}{
						"requests": map[string]interface{}{"cpu": "4"},
						"limits":   map[string]interface{}{"cpu": "2"},
					},
				},
			},
			expectDefaults: true,
		},
		{
			name: "valid config",
			settings: map[string]interface{}{
				"kubernetes": map[string]interface{}{
					"nodeSelector": map[string]interface{}{"type": "gpu"},
				},
			},
			expectDefaults: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var loggedWarning bool
			logger := func(format string, args ...interface{}) {
				loggedWarning = true
			}

			config := ParsePodTemplateConfigSafe(tt.settings, logger)

			// Should always return a valid config
			assert.NotNil(t, config)

			if tt.expectDefaults {
				// Should have logged a warning for invalid configs
				if tt.settings != nil && tt.name != "nil settings" {
					assert.True(t, loggedWarning, "Expected warning to be logged")
				}
				// Should have default resources
				assert.NotNil(t, config.Resources)
				memReq := config.Resources.Requests[corev1.ResourceMemory]
				assert.Equal(t, "512Mi", memReq.String())
			} else {
				// Should not have logged a warning
				assert.False(t, loggedWarning, "Should not have logged warning for valid config")
			}
		})
	}
}
