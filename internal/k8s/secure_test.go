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

	assert.Contains(t, c.Env, corev1.EnvVar{Name: "SECURE", Value: "true"})
	assert.Contains(t, c.Env, corev1.EnvVar{Name: "HOME", Value: "/root"})
	assert.Contains(t, c.VolumeMounts, corev1.VolumeMount{Name: "tmp", MountPath: "/tmp"})
	assert.Contains(t, c.VolumeMounts, corev1.VolumeMount{Name: "root", MountPath: "/root"})
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

	t.Run("explicit resources and SECURE=true", func(t *testing.T) {
		cfg := netdrill.PodConfig{
			CPURequest:    "250m",
			MemoryRequest: "512Mi",
			CPULimit:      "1",
			MemoryLimit:   "1Gi",
		}
		require.NoError(t, ApplySecurePodConfig(&cfg))
		assert.Equal(t, "250m", cfg.CPURequest)
		assert.Equal(t, "512Mi", cfg.MemoryRequest)
		assert.Equal(t, "1", cfg.CPULimit)
		assert.Equal(t, "1Gi", cfg.MemoryLimit)
		assert.Equal(t, "true", cfg.EnvVars["SECURE"])
		assert.Equal(t, "/root", cfg.EnvVars["HOME"])
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

func TestApplySecurePodConfig_HardensClonedFields(t *testing.T) {
	t.Run("replaces privileged, container runAsUser=0, and unconfined seccomp", func(t *testing.T) {
		cfg := netdrill.PodConfig{
			SecurityContext: &corev1.PodSecurityContext{
				RunAsUser:      ptr.To(int64(0)),
				SeccompProfile: &corev1.SeccompProfile{Type: corev1.SeccompProfileTypeUnconfined},
			},
			ContainerSecurityContext: &corev1.SecurityContext{
				Privileged:     ptr.To(true),
				RunAsUser:      ptr.To(int64(0)),
				SeccompProfile: &corev1.SeccompProfile{Type: corev1.SeccompProfileTypeUnconfined},
				Capabilities: &corev1.Capabilities{
					Add: []corev1.Capability{"NET_ADMIN"},
				},
			},
		}

		require.NoError(t, ApplySecurePodConfig(&cfg))

		assert.Equal(t, ptr.To(SecureDefaultUID), cfg.SecurityContext.RunAsUser)
		assert.Equal(t, corev1.SeccompProfileTypeRuntimeDefault, cfg.SecurityContext.SeccompProfile.Type)

		assert.Equal(t, ptr.To(false), cfg.ContainerSecurityContext.Privileged)
		assert.Equal(t, ptr.To(SecureDefaultUID), cfg.ContainerSecurityContext.RunAsUser)
		assert.Equal(t, corev1.SeccompProfileTypeRuntimeDefault, cfg.ContainerSecurityContext.SeccompProfile.Type)
		assert.Equal(t, []corev1.Capability{"ALL"}, cfg.ContainerSecurityContext.Capabilities.Drop)
		assert.Empty(t, cfg.ContainerSecurityContext.Capabilities.Add)
	})

	t.Run("preserves valid localhost seccomp and non-zero runAsUser", func(t *testing.T) {
		localhost := "profiles/audit.json"
		cfg := netdrill.PodConfig{
			SecurityContext: &corev1.PodSecurityContext{
				RunAsUser: ptr.To(int64(1000)),
				SeccompProfile: &corev1.SeccompProfile{
					Type:             corev1.SeccompProfileTypeLocalhost,
					LocalhostProfile: &localhost,
				},
			},
			ContainerSecurityContext: &corev1.SecurityContext{
				RunAsUser: ptr.To(int64(1001)),
				SeccompProfile: &corev1.SeccompProfile{
					Type:             corev1.SeccompProfileTypeLocalhost,
					LocalhostProfile: &localhost,
				},
			},
		}

		require.NoError(t, ApplySecurePodConfig(&cfg))

		assert.Equal(t, ptr.To(int64(1000)), cfg.SecurityContext.RunAsUser)
		assert.Equal(t, corev1.SeccompProfileTypeLocalhost, cfg.SecurityContext.SeccompProfile.Type)
		assert.Equal(t, ptr.To(int64(1001)), cfg.ContainerSecurityContext.RunAsUser)
		assert.Equal(t, corev1.SeccompProfileTypeLocalhost, cfg.ContainerSecurityContext.SeccompProfile.Type)
	})
}

func TestApplySecurePodConfig_VolumeCollision(t *testing.T) {
	t.Run("reuses matching emptyDir volume", func(t *testing.T) {
		cfg := netdrill.PodConfig{
			Volumes: []corev1.Volume{
				{Name: "tmp", VolumeSource: corev1.VolumeSource{EmptyDir: &corev1.EmptyDirVolumeSource{}}},
			},
		}

		require.NoError(t, ApplySecurePodConfig(&cfg))
		assert.Len(t, cfg.Volumes, 2) // tmp (reused) + root
		assert.Contains(t, cfg.VolumeMounts, corev1.VolumeMount{Name: "tmp", MountPath: "/tmp"})
	})

	t.Run("allocates collision-free emptyDir when matching name is not emptyDir", func(t *testing.T) {
		cfg := netdrill.PodConfig{
			Volumes: []corev1.Volume{
				{Name: "tmp", VolumeSource: corev1.VolumeSource{ConfigMap: &corev1.ConfigMapVolumeSource{}}},
				{Name: "netdrill-tmp", VolumeSource: corev1.VolumeSource{ConfigMap: &corev1.ConfigMapVolumeSource{}}},
			},
			VolumeMounts: []corev1.VolumeMount{
				{Name: "tmp", MountPath: "/etc/tmp-cfg"},
			},
		}

		require.NoError(t, ApplySecurePodConfig(&cfg))
		assert.Contains(t, cfg.VolumeMounts, corev1.VolumeMount{Name: "tmp", MountPath: "/etc/tmp-cfg"})
		assert.Contains(t, cfg.VolumeMounts, corev1.VolumeMount{Name: "netdrill-tmp-1", MountPath: "/tmp"})
		assert.Contains(t, cfg.Volumes, corev1.Volume{
			Name:         "netdrill-tmp-1",
			VolumeSource: corev1.VolumeSource{EmptyDir: &corev1.EmptyDirVolumeSource{}},
		})
	})
}

func TestApplySecurePodConfig_ResourcePairs(t *testing.T) {
	t.Run("explicit request above default limit raises limit", func(t *testing.T) {
		cfg := netdrill.PodConfig{
			CPURequest:    "1",
			MemoryRequest: "512Mi",
		}
		require.NoError(t, ApplySecurePodConfig(&cfg))
		assert.Equal(t, "1", cfg.CPURequest)
		assert.Equal(t, "1", cfg.CPULimit)
		assert.Equal(t, "512Mi", cfg.MemoryRequest)
		assert.Equal(t, "512Mi", cfg.MemoryLimit)
	})

	t.Run("explicit limit below default request lowers request", func(t *testing.T) {
		cfg := netdrill.PodConfig{
			CPULimit:    "50m",
			MemoryLimit: "64Mi",
		}
		require.NoError(t, ApplySecurePodConfig(&cfg))
		assert.Equal(t, "50m", cfg.CPURequest)
		assert.Equal(t, "50m", cfg.CPULimit)
		assert.Equal(t, "64Mi", cfg.MemoryRequest)
		assert.Equal(t, "64Mi", cfg.MemoryLimit)
	})
}

func TestApplySecurePodConfig_Sysctls(t *testing.T) {
	okCfg := netdrill.PodConfig{
		SecurityContext: &corev1.PodSecurityContext{
			Sysctls: []corev1.Sysctl{
				{Name: "net.ipv4.ip_local_port_range", Value: "1024 65535"},
			},
		},
	}
	require.NoError(t, ApplySecurePodConfig(&okCfg))
	require.Len(t, okCfg.SecurityContext.Sysctls, 1)
	assert.Equal(t, "net.ipv4.ip_local_port_range", okCfg.SecurityContext.Sysctls[0].Name)

	badCfg := netdrill.PodConfig{
		SecurityContext: &corev1.PodSecurityContext{
			Sysctls: []corev1.Sysctl{
				{Name: "kernel.msgmax", Value: "65536"},
			},
		},
	}
	err := ApplySecurePodConfig(&badCfg)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "sysctl")
}

func TestApplySecurePodConfig_AppArmorProfile(t *testing.T) {
	localhost := "k8s-app/custom"
	cfg := netdrill.PodConfig{
		SecurityContext: &corev1.PodSecurityContext{
			AppArmorProfile: &corev1.AppArmorProfile{Type: corev1.AppArmorProfileTypeUnconfined},
		},
		ContainerSecurityContext: &corev1.SecurityContext{
			AppArmorProfile: &corev1.AppArmorProfile{
				Type:             corev1.AppArmorProfileTypeLocalhost,
				LocalhostProfile: &localhost,
			},
		},
	}
	require.NoError(t, ApplySecurePodConfig(&cfg))
	require.NotNil(t, cfg.SecurityContext.AppArmorProfile)
	assert.Equal(t, corev1.AppArmorProfileTypeRuntimeDefault, cfg.SecurityContext.AppArmorProfile.Type)
	require.NotNil(t, cfg.ContainerSecurityContext.AppArmorProfile)
	assert.Equal(t, corev1.AppArmorProfileTypeLocalhost, cfg.ContainerSecurityContext.AppArmorProfile.Type)

	containerUnconfined := netdrill.PodConfig{
		ContainerSecurityContext: &corev1.SecurityContext{
			AppArmorProfile: &corev1.AppArmorProfile{Type: corev1.AppArmorProfileTypeUnconfined},
		},
	}
	require.NoError(t, ApplySecurePodConfig(&containerUnconfined))
	assert.Nil(t, containerUnconfined.SecurityContext.AppArmorProfile)
	aa := containerUnconfined.ContainerSecurityContext.AppArmorProfile
	require.NotNil(t, aa)
	assert.Equal(t, corev1.AppArmorProfileTypeRuntimeDefault, aa.Type)
}

func TestApplySecurePodConfig_SELinuxOptions(t *testing.T) {
	okCfg := netdrill.PodConfig{
		SecurityContext: &corev1.PodSecurityContext{
			SELinuxOptions: &corev1.SELinuxOptions{
				Type:  "container_t",
				Level: "s0:c123,c456",
			},
		},
		ContainerSecurityContext: &corev1.SecurityContext{
			SELinuxOptions: &corev1.SELinuxOptions{
				Level: "s0:c1,c2",
			},
		},
	}
	require.NoError(t, ApplySecurePodConfig(&okCfg))
	assert.Equal(t, "container_t", okCfg.SecurityContext.SELinuxOptions.Type)
	assert.Equal(t, "s0:c123,c456", okCfg.SecurityContext.SELinuxOptions.Level)
	assert.Equal(t, "s0:c1,c2", okCfg.ContainerSecurityContext.SELinuxOptions.Level)

	badPodUser := netdrill.PodConfig{
		SecurityContext: &corev1.PodSecurityContext{
			SELinuxOptions: &corev1.SELinuxOptions{User: "system_u"},
		},
	}
	require.Error(t, ApplySecurePodConfig(&badPodUser))

	badContainerRole := netdrill.PodConfig{
		ContainerSecurityContext: &corev1.SecurityContext{
			SELinuxOptions: &corev1.SELinuxOptions{Role: "system_r"},
		},
	}
	require.Error(t, ApplySecurePodConfig(&badContainerRole))

	badContainerType := netdrill.PodConfig{
		ContainerSecurityContext: &corev1.SecurityContext{
			SELinuxOptions: &corev1.SELinuxOptions{Type: "spc_t"},
		},
	}
	require.Error(t, ApplySecurePodConfig(&badContainerType))
}
