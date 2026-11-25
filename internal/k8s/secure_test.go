package k8s

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/xenos76/kubectl-netdrill/internal/netdrill"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/utils/ptr"
)

func TestApplySecurePodConfig_Defaults(t *testing.T) {
	cfg := netdrill.PodConfig{
		Namespace: "default",
		PodName:   "sec-pod",
		Image:     "ghcr.io/xenos76/netdrill:latest",
	}

	require.NoError(t, ApplySecurePodConfig(&cfg))

	pod, err := BuildPod(PodOptionsFromConfig(cfg))
	require.NoError(t, err)

	require.NotNil(t, pod.Spec.AutomountServiceAccountToken)
	assert.False(t, *pod.Spec.AutomountServiceAccountToken)

	require.NotNil(t, pod.Spec.SecurityContext)
	assert.Equal(t, ptr.To(true), pod.Spec.SecurityContext.RunAsNonRoot)
	assert.Equal(t, ptr.To(SecureDefaultUID), pod.Spec.SecurityContext.RunAsUser)
	assert.Equal(t, ptr.To(SecureDefaultUID), pod.Spec.SecurityContext.RunAsGroup)
	assert.Equal(t, ptr.To(SecureDefaultUID), pod.Spec.SecurityContext.FSGroup)
	require.NotNil(t, pod.Spec.SecurityContext.SeccompProfile)
	assert.Equal(t, corev1.SeccompProfileTypeRuntimeDefault, pod.Spec.SecurityContext.SeccompProfile.Type)

	require.Len(t, pod.Spec.Containers, 1)
	c := pod.Spec.Containers[0]
	require.NotNil(t, c.SecurityContext)
	assert.Equal(t, ptr.To(false), c.SecurityContext.AllowPrivilegeEscalation)
	assert.Equal(t, ptr.To(true), c.SecurityContext.ReadOnlyRootFilesystem)
	assert.Equal(t, ptr.To(true), c.SecurityContext.RunAsNonRoot)
	require.NotNil(t, c.SecurityContext.Capabilities)
	assert.Equal(t, []corev1.Capability{"ALL"}, c.SecurityContext.Capabilities.Drop)

	assert.Contains(t, c.Env, corev1.EnvVar{Name: "HOME", Value: "/root"})
	assert.Contains(t, c.VolumeMounts, corev1.VolumeMount{Name: "tmp", MountPath: "/tmp"})
	assert.Contains(t, c.VolumeMounts, corev1.VolumeMount{Name: "home", MountPath: "/root"})
	assert.Len(t, pod.Spec.Volumes, 2)
	assert.NotNil(t, pod.Spec.Volumes[0].EmptyDir)
	assert.NotNil(t, pod.Spec.Volumes[1].EmptyDir)

	assert.Equal(t, SecureDefaultCPURequest, c.Resources.Requests.Cpu().String())
	assert.Equal(t, SecureDefaultMemoryRequest, c.Resources.Requests.Memory().String())
	assert.Equal(t, SecureDefaultCPULimit, c.Resources.Limits.Cpu().String())
	assert.Equal(t, SecureDefaultMemoryLimit, c.Resources.Limits.Memory().String())
}

func TestApplySecurePodConfig_EdgeCases(t *testing.T) {
	t.Run("hostNetwork rejected", func(t *testing.T) {
		cfg := netdrill.PodConfig{HostNetwork: true}
		err := ApplySecurePodConfig(&cfg)
		require.Error(t, err)
		assert.Contains(t, err.Error(), "--host-network")
	})

	t.Run("serviceAccount preserves automount for IRSA", func(t *testing.T) {
		cfg := netdrill.PodConfig{ServiceAccount: "irsa-sa"}
		require.NoError(t, ApplySecurePodConfig(&cfg))
		assert.Nil(t, cfg.AutomountServiceAccountToken)
	})

	t.Run("explicit resources and HOME preserved", func(t *testing.T) {
		cfg := netdrill.PodConfig{
			CPURequest:    "250m",
			MemoryRequest: "512Mi",
			CPULimit:      "1",
			MemoryLimit:   "1Gi",
			EnvVars:       map[string]string{"HOME": "/custom"},
		}
		require.NoError(t, ApplySecurePodConfig(&cfg))
		assert.Equal(t, "250m", cfg.CPURequest)
		assert.Equal(t, "512Mi", cfg.MemoryRequest)
		assert.Equal(t, "1", cfg.CPULimit)
		assert.Equal(t, "1Gi", cfg.MemoryLimit)
		assert.Equal(t, "/custom", cfg.EnvVars["HOME"])
	})
}

func TestApplySecureDeploymentConfig(t *testing.T) {
	cfg := netdrill.DeploymentConfig{
		PodConfig: netdrill.PodConfig{
			Namespace: "default",
			PodName:   "sec-deploy",
			Image:     "ghcr.io/xenos76/netdrill:latest",
		},
		AppLabel: "sec-deploy",
	}

	require.NoError(t, ApplySecureDeploymentConfig(&cfg))

	deploy, err := BuildDeployment(DeploymentOptionsFromConfig(cfg))
	require.NoError(t, err)

	spec := deploy.Spec.Template.Spec
	require.NotNil(t, spec.AutomountServiceAccountToken)
	assert.False(t, *spec.AutomountServiceAccountToken)
	require.NotNil(t, spec.SecurityContext)
	assert.Equal(t, ptr.To(true), spec.SecurityContext.RunAsNonRoot)
	require.Len(t, spec.Containers, 1)
	assert.Equal(t, ptr.To(true), spec.Containers[0].SecurityContext.ReadOnlyRootFilesystem)
}
