package k8s

import (
	"errors"
	"fmt"
	"maps"
	"slices"
	"strings"

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

	appArmorBetaAnnotationPrefix = "container.apparmor.security.beta.kubernetes.io/"
	appArmorBetaRuntimeDefault   = "runtime/default"
	appArmorBetaLocalhostPrefix  = "localhost/"
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

	if err := applySecurePodSecurityContext(cfg); err != nil {
		return err
	}

	if err := applySecureContainerSecurityContext(cfg); err != nil {
		return err
	}

	normalizeSecureAppArmorAnnotations(cfg)

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

func applySecurePodSecurityContext(cfg *netdrill.PodConfig) error {
	sc := &corev1.PodSecurityContext{}
	if cfg.SecurityContext != nil {
		sc = cfg.SecurityContext.DeepCopy()
	}

	if err := validateSecureSysctls(sc.Sysctls); err != nil {
		return err
	}

	if err := validateSecureSELinuxOptions(sc.SELinuxOptions); err != nil {
		return err
	}

	sc.RunAsNonRoot = ptr.To(true)
	sc.SeccompProfile = ensureSecureSeccompProfile(sc.SeccompProfile)
	sc.AppArmorProfile = ensureSecureAppArmorProfile(sc.AppArmorProfile)

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

	return nil
}

func applySecureContainerSecurityContext(cfg *netdrill.PodConfig) error {
	csc := &corev1.SecurityContext{}
	if cfg.ContainerSecurityContext != nil {
		csc = cfg.ContainerSecurityContext.DeepCopy()
	}

	if err := validateSecureSELinuxOptions(csc.SELinuxOptions); err != nil {
		return err
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
	csc.AppArmorProfile = ensureSecureAppArmorProfile(csc.AppArmorProfile)

	cfg.ContainerSecurityContext = csc

	return nil
}

func ensureSecureSeccompProfile(p *corev1.SeccompProfile) *corev1.SeccompProfile {
	if p != nil && (p.Type == corev1.SeccompProfileTypeRuntimeDefault || p.Type == corev1.SeccompProfileTypeLocalhost) {
		return p
	}

	return &corev1.SeccompProfile{
		Type: corev1.SeccompProfileTypeRuntimeDefault,
	}
}

func ensureSecureAppArmorProfile(p *corev1.AppArmorProfile) *corev1.AppArmorProfile {
	if p == nil {
		return nil
	}

	if p.Type == corev1.AppArmorProfileTypeRuntimeDefault || p.Type == corev1.AppArmorProfileTypeLocalhost {
		return p
	}

	return &corev1.AppArmorProfile{
		Type: corev1.AppArmorProfileTypeRuntimeDefault,
	}
}

func normalizeSecureAppArmorAnnotations(cfg *netdrill.PodConfig) {
	if len(cfg.Annotations) == 0 {
		return
	}

	var cloned map[string]string

	for k, v := range cfg.Annotations {
		if !strings.HasPrefix(k, appArmorBetaAnnotationPrefix) {
			continue
		}

		if isPermittedAppArmorAnnotation(v) {
			continue
		}

		if cloned == nil {
			cloned = maps.Clone(cfg.Annotations)
		}

		cloned[k] = appArmorBetaRuntimeDefault
	}

	if cloned != nil {
		cfg.Annotations = cloned
	}
}

func isPermittedAppArmorAnnotation(v string) bool {
	if v == appArmorBetaRuntimeDefault {
		return true
	}

	return strings.HasPrefix(v, appArmorBetaLocalhostPrefix) &&
		len(v) > len(appArmorBetaLocalhostPrefix)
}

func validateSecureSysctls(sysctls []corev1.Sysctl) error {
	for _, s := range sysctls {
		switch s.Name {
		case "kernel.shm_rmid_forced",
			"net.ipv4.ip_local_port_range",
			"net.ipv4.tcp_syncookies",
			"net.ipv4.ping_group_range",
			"net.ipv4.ip_unprivileged_port_start",
			"net.ipv4.ip_local_reserved_ports",
			"net.unix.max_dgram_qlen",
			"net.ipv4.tcp_keepalive_time",
			"net.ipv4.tcp_fin_timeout",
			"net.ipv4.tcp_keepalive_intvl",
			"net.ipv4.tcp_keepalive_probes":
		default:
			return fmt.Errorf("--secure does not allow sysctl %q", s.Name)
		}
	}

	return nil
}

func validateSecureSELinuxOptions(opts *corev1.SELinuxOptions) error {
	if opts == nil {
		return nil
	}

	if opts.User != "" {
		return fmt.Errorf("--secure does not allow seLinuxOptions.user %q", opts.User)
	}

	if opts.Role != "" {
		return fmt.Errorf("--secure does not allow seLinuxOptions.role %q", opts.Role)
	}

	switch opts.Type {
	case "", "container_t", "container_init_t", "container_kvm_t", "container_engine_t":
		return nil
	default:
		return fmt.Errorf("--secure does not allow seLinuxOptions.type %q", opts.Type)
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
