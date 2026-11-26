package k8s

import (
	"errors"
	"fmt"
	"slices"

	"github.com/xenos76/kubectl-netdrill/internal/netdrill"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/resource"
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
	secureRootVolumeName = "root"
	secureRootMountPath  = "/root"
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
	ensureSecureEmptyDirMount(cfg, secureRootVolumeName, secureRootMountPath)
	ensureSecureEnv(cfg)
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
	sc.SeccompProfile = ensureSecureSeccompProfile(sc.SeccompProfile)

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

	if csc.Privileged != nil && *csc.Privileged {
		csc.Privileged = ptr.To(false)
	}

	if csc.RunAsUser != nil && *csc.RunAsUser == 0 {
		csc.RunAsUser = ptr.To(SecureDefaultUID)
	}

	csc.Capabilities = &corev1.Capabilities{
		Drop: []corev1.Capability{"ALL"},
	}

	csc.SeccompProfile = ensureSecureSeccompProfile(csc.SeccompProfile)

	cfg.ContainerSecurityContext = csc
}

func ensureSecureSeccompProfile(p *corev1.SeccompProfile) *corev1.SeccompProfile {
	if p != nil && (p.Type == corev1.SeccompProfileTypeRuntimeDefault || p.Type == corev1.SeccompProfileTypeLocalhost) {
		return p
	}

	return &corev1.SeccompProfile{
		Type: corev1.SeccompProfileTypeRuntimeDefault,
	}
}

func ensureSecureEmptyDirMount(cfg *netdrill.PodConfig, volumeName, mountPath string) {
	if slices.ContainsFunc(cfg.VolumeMounts, func(m corev1.VolumeMount) bool {
		return m.MountPath == mountPath
	}) {
		return
	}

	targetName := volumeName
	idx := slices.IndexFunc(cfg.Volumes, func(v corev1.Volume) bool {
		return v.Name == targetName
	})

	if idx >= 0 && cfg.Volumes[idx].EmptyDir != nil {
		cfg.VolumeMounts = append(cfg.VolumeMounts, corev1.VolumeMount{
			Name:      targetName,
			MountPath: mountPath,
		})

		return
	}

	if idx >= 0 {
		targetName = nextAvailableVolumeName(cfg.Volumes, "netdrill-"+volumeName)
	}

	cfg.Volumes = append(cfg.Volumes, corev1.Volume{
		Name: targetName,
		VolumeSource: corev1.VolumeSource{
			EmptyDir: &corev1.EmptyDirVolumeSource{},
		},
	})
	cfg.VolumeMounts = append(cfg.VolumeMounts, corev1.VolumeMount{
		Name:      targetName,
		MountPath: mountPath,
	})
}

func nextAvailableVolumeName(volumes []corev1.Volume, base string) string {
	hasName := func(name string) bool {
		return slices.ContainsFunc(volumes, func(v corev1.Volume) bool {
			return v.Name == name
		})
	}

	if !hasName(base) {
		return base
	}

	for i := 1; ; i++ {
		candidate := fmt.Sprintf("%s-%d", base, i)
		if !hasName(candidate) {
			return candidate
		}
	}
}

func ensureSecureEnv(cfg *netdrill.PodConfig) {
	if cfg.EnvVars == nil {
		cfg.EnvVars = make(map[string]string)
	}

	cfg.EnvVars["SECURE"] = "true"
	if _, ok := cfg.EnvVars["HOME"]; !ok {
		cfg.EnvVars["HOME"] = secureRootMountPath
	}
}

func applySecureResourceDefaults(cfg *netdrill.PodConfig) {
	applySecureResourcePair(&cfg.CPURequest, &cfg.CPULimit, SecureDefaultCPURequest, SecureDefaultCPULimit)
	applySecureResourcePair(&cfg.MemoryRequest, &cfg.MemoryLimit, SecureDefaultMemoryRequest, SecureDefaultMemoryLimit)
}

func applySecureResourcePair(req, lim *string, defaultReq, defaultLim string) {
	switch {
	case *req == "" && *lim == "":
		*req = defaultReq
		*lim = defaultLim
	case *req != "" && *lim == "":
		reqQty, err := resource.ParseQuantity(*req)
		if err != nil {
			*lim = defaultLim
			return
		}

		defLimQty := resource.MustParse(defaultLim)
		if reqQty.Cmp(defLimQty) > 0 {
			*lim = *req
		} else {
			*lim = defaultLim
		}
	case *req == "" && *lim != "":
		limQty, err := resource.ParseQuantity(*lim)
		if err != nil {
			*req = defaultReq
			return
		}

		defReqQty := resource.MustParse(defaultReq)
		if defReqQty.Cmp(limQty) > 0 {
			*req = *lim
		} else {
			*req = defaultReq
		}
	default:
	}
}
