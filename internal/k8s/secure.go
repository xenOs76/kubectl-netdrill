package k8s

import (
	"errors"
	"slices"

	"github.com/xenos76/kubectl-netdrill/internal/netdrill"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/utils/ptr"
)

const (
	// SecureDefaultUID is the non-root UID/GID/FSGroup applied by --secure.
	SecureDefaultUID int64 = 65532
	// SecureDefaultCPURequest is the default CPU request applied by --secure when unset.
	SecureDefaultCPURequest = "100m"
	// SecureDefaultMemoryRequest is the default memory request applied by --secure when unset.
	SecureDefaultMemoryRequest = "128Mi"
	// SecureDefaultCPULimit is the default CPU limit applied by --secure when unset.
	SecureDefaultCPULimit = "500m"
	// SecureDefaultMemoryLimit is the default memory limit applied by --secure when unset.
	SecureDefaultMemoryLimit = "256Mi"

	secureTmpVolumeName  = "tmp"
	secureTmpMountPath   = "/tmp"
	secureHomeVolumeName = "home"
	secureHomeMountPath  = "/root"
)

// ApplySecurePodConfig applies Kubernetes Restricted Pod Security Standards and
// hardening defaults to cfg. It fails if HostNetwork is enabled.
func ApplySecurePodConfig(cfg *netdrill.PodConfig) error {
	if cfg == nil {
		return errors.New("pod config is nil")
	}

	if cfg.HostNetwork {
		return errors.New("--secure cannot be used with --host-network")
	}

	applySecurePodSecurityContext(cfg)
	applySecureContainerSecurityContext(cfg)

	if cfg.ServiceAccount == "" && cfg.AutomountServiceAccountToken == nil {
		cfg.AutomountServiceAccountToken = ptr.To(false)
	}

	ensureSecureEmptyDirMount(cfg, secureTmpVolumeName, secureTmpMountPath)
	ensureSecureEmptyDirMount(cfg, secureHomeVolumeName, secureHomeMountPath)
	ensureSecureHomeEnv(cfg)
	applySecureResourceDefaults(cfg)

	return nil
}

// ApplySecureDeploymentConfig applies --secure hardening defaults to a DeploymentConfig.
func ApplySecureDeploymentConfig(cfg *netdrill.DeploymentConfig) error {
	if cfg == nil {
		return errors.New("deployment config is nil")
	}

	if cfg.CPURequest != "" && cfg.PodConfig.CPURequest == "" {
		cfg.PodConfig.CPURequest = cfg.CPURequest
	}

	if cfg.MemoryRequest != "" && cfg.PodConfig.MemoryRequest == "" {
		cfg.PodConfig.MemoryRequest = cfg.MemoryRequest
	}

	if cfg.CPULimit != "" && cfg.PodConfig.CPULimit == "" {
		cfg.PodConfig.CPULimit = cfg.CPULimit
	}

	if cfg.MemoryLimit != "" && cfg.PodConfig.MemoryLimit == "" {
		cfg.PodConfig.MemoryLimit = cfg.MemoryLimit
	}

	if err := ApplySecurePodConfig(&cfg.PodConfig); err != nil {
		return err
	}

	cfg.CPURequest = cfg.PodConfig.CPURequest
	cfg.MemoryRequest = cfg.PodConfig.MemoryRequest
	cfg.CPULimit = cfg.PodConfig.CPULimit
	cfg.MemoryLimit = cfg.PodConfig.MemoryLimit

	return nil
}

func applySecurePodSecurityContext(cfg *netdrill.PodConfig) {
	sc := &corev1.PodSecurityContext{}
	if cfg.SecurityContext != nil {
		sc = cfg.SecurityContext.DeepCopy()
	}

	sc.RunAsNonRoot = ptr.To(true)
	if sc.SeccompProfile == nil {
		sc.SeccompProfile = &corev1.SeccompProfile{
			Type: corev1.SeccompProfileTypeRuntimeDefault,
		}
	}

	if sc.RunAsUser == nil || *sc.RunAsUser == 0 {
		sc.RunAsUser = ptr.To(SecureDefaultUID)
	}

	if sc.RunAsGroup == nil || *sc.RunAsGroup == 0 {
		sc.RunAsGroup = ptr.To(SecureDefaultUID)
	}

	if sc.FSGroup == nil || *sc.FSGroup == 0 {
		sc.FSGroup = ptr.To(SecureDefaultUID)
	}

	cfg.SecurityContext = sc
}

func applySecureContainerSecurityContext(cfg *netdrill.PodConfig) {
	csc := &corev1.SecurityContext{}
	if cfg.ContainerSecurityContext != nil {
		csc = cfg.ContainerSecurityContext.DeepCopy()
	}

	csc.AllowPrivilegeEscalation = ptr.To(false)
	csc.ReadOnlyRootFilesystem = ptr.To(true)
	csc.RunAsNonRoot = ptr.To(true)
	csc.Capabilities = &corev1.Capabilities{
		Drop: []corev1.Capability{"ALL"},
	}

	if csc.SeccompProfile == nil {
		csc.SeccompProfile = &corev1.SeccompProfile{
			Type: corev1.SeccompProfileTypeRuntimeDefault,
		}
	}

	cfg.ContainerSecurityContext = csc
}

func ensureSecureEmptyDirMount(cfg *netdrill.PodConfig, volumeName, mountPath string) {
	hasMount := slices.ContainsFunc(cfg.VolumeMounts, func(m corev1.VolumeMount) bool {
		return m.MountPath == mountPath
	})
	if hasMount {
		return
	}

	hasVol := slices.ContainsFunc(cfg.Volumes, func(v corev1.Volume) bool {
		return v.Name == volumeName
	})
	if !hasVol {
		cfg.Volumes = append(cfg.Volumes, corev1.Volume{
			Name: volumeName,
			VolumeSource: corev1.VolumeSource{
				EmptyDir: &corev1.EmptyDirVolumeSource{},
			},
		})
	}

	cfg.VolumeMounts = append(cfg.VolumeMounts, corev1.VolumeMount{
		Name:      volumeName,
		MountPath: mountPath,
	})
}

func ensureSecureHomeEnv(cfg *netdrill.PodConfig) {
	if _, ok := cfg.EnvVars["HOME"]; ok {
		return
	}

	hasRawHome := slices.ContainsFunc(cfg.RawEnvVars, func(e corev1.EnvVar) bool {
		return e.Name == "HOME"
	})
	if hasRawHome {
		return
	}

	if cfg.EnvVars == nil {
		cfg.EnvVars = make(map[string]string)
	}

	cfg.EnvVars["HOME"] = secureHomeMountPath
}

func applySecureResourceDefaults(cfg *netdrill.PodConfig) {
	if cfg.CPURequest == "" {
		cfg.CPURequest = SecureDefaultCPURequest
	}

	if cfg.MemoryRequest == "" {
		cfg.MemoryRequest = SecureDefaultMemoryRequest
	}

	if cfg.CPULimit == "" {
		cfg.CPULimit = SecureDefaultCPULimit
	}

	if cfg.MemoryLimit == "" {
		cfg.MemoryLimit = SecureDefaultMemoryLimit
	}
}
