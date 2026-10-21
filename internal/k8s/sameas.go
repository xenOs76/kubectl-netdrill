package k8s

import (
	"context"
	"errors"
	"fmt"
	"maps"
	"slices"
	"strings"

	"github.com/spf13/cobra"
	"github.com/xenos76/kubectl-netdrill/internal/netdrill"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes"
)

// TargetInfo describes the parsed workload location and kind.
type TargetInfo struct {
	Namespace string
	Kind      string // "pod" or "deployment"
	Name      string
}

func normalizeKind(k string) (string, error) {
	lower := strings.ToLower(k)
	switch lower {
	case "pod", "pods", "po":
		return "pod", nil
	case "deployment", "deployments", "deploy":
		return "deployment", nil
	default:
		return "", fmt.Errorf("unsupported kind %q for --same-as (supported: pod, deployment)", k)
	}
}

func parseTwoPartTarget(first, second, defaultKind, defaultNamespace string) (TargetInfo, error) {
	if second == "" {
		return TargetInfo{}, errors.New("target name cannot be empty")
	}

	// Check if first is a recognized kind
	if kind, err := normalizeKind(first); err == nil {
		return TargetInfo{
			Namespace: defaultNamespace,
			Kind:      kind,
			Name:      second,
		}, nil
	}

	// Otherwise first is namespace
	kind, err := normalizeKind(defaultKind)
	if err != nil {
		return TargetInfo{}, err
	}

	return TargetInfo{
		Namespace: first,
		Kind:      kind,
		Name:      second,
	}, nil
}

func parseThreePartTarget(ns, rawKind, name string) (TargetInfo, error) {
	if ns == "" {
		return TargetInfo{}, errors.New("target namespace cannot be empty")
	}

	if name == "" {
		return TargetInfo{}, errors.New("target name cannot be empty")
	}

	kind, err := normalizeKind(rawKind)
	if err != nil {
		return TargetInfo{}, err
	}

	return TargetInfo{
		Namespace: ns,
		Kind:      kind,
		Name:      name,
	}, nil
}

// ParseSameAsTarget parses a target string into TargetInfo.
// Supported syntax:
// - "<name>" -> defaultKind in defaultNamespace
// - "<kind>/<name>" -> specified kind in defaultNamespace
// - "<namespace>/<name>" -> defaultKind in specified namespace
// - "<namespace>/<kind>/<name>" -> specified kind in specified namespace
func ParseSameAsTarget(target, defaultKind, defaultNamespace string) (TargetInfo, error) {
	trimmed := strings.TrimSpace(target)
	if trimmed == "" {
		return TargetInfo{}, errors.New("same-as target cannot be empty")
	}

	if defaultNamespace == "" {
		defaultNamespace = "default"
	}

	parts := strings.Split(trimmed, "/")
	switch len(parts) {
	case 1:
		name := strings.TrimSpace(parts[0])
		if name == "" {
			return TargetInfo{}, errors.New("target name cannot be empty")
		}

		kind, err := normalizeKind(defaultKind)
		if err != nil {
			return TargetInfo{}, err
		}

		return TargetInfo{
			Namespace: defaultNamespace,
			Kind:      kind,
			Name:      name,
		}, nil

	case 2:
		return parseTwoPartTarget(
			strings.TrimSpace(parts[0]),
			strings.TrimSpace(parts[1]),
			defaultKind,
			defaultNamespace,
		)

	case 3:
		return parseThreePartTarget(
			strings.TrimSpace(parts[0]),
			strings.TrimSpace(parts[1]),
			strings.TrimSpace(parts[2]),
		)

	default:
		return TargetInfo{}, fmt.Errorf("invalid target format %q: expected [[namespace/]kind/]name", target)
	}
}

// SameAsWorkload holds extracted workload metadata and pod spec.
type SameAsWorkload struct {
	Target      TargetInfo
	Labels      map[string]string
	Annotations map[string]string
	PodSpec     corev1.PodSpec
	Replicas    *int32
}

// FetchSameAsWorkload fetches the target Pod or Deployment from the Kubernetes cluster.
func FetchSameAsWorkload(ctx context.Context, client kubernetes.Interface, target TargetInfo) (*SameAsWorkload, error) {
	if client == nil {
		return nil, errors.New("kubernetes client is nil")
	}

	switch target.Kind {
	case "pod":
		pod, err := client.CoreV1().Pods(target.Namespace).Get(ctx, target.Name, metav1.GetOptions{})
		if err != nil {
			return nil, fmt.Errorf("fetching pod %s/%s: %w", target.Namespace, target.Name, err)
		}

		return &SameAsWorkload{
			Target:      target,
			Labels:      pod.Labels,
			Annotations: pod.Annotations,
			PodSpec:     pod.Spec,
			Replicas:    nil,
		}, nil

	case "deployment":
		deploy, err := client.AppsV1().Deployments(target.Namespace).Get(ctx, target.Name, metav1.GetOptions{})
		if err != nil {
			return nil, fmt.Errorf("fetching deployment %s/%s: %w", target.Namespace, target.Name, err)
		}

		return &SameAsWorkload{
			Target:      target,
			Labels:      deploy.Spec.Template.Labels,
			Annotations: deploy.Spec.Template.Annotations,
			PodSpec:     deploy.Spec.Template.Spec,
			Replicas:    deploy.Spec.Replicas,
		}, nil

	default:
		return nil, fmt.Errorf("unsupported workload kind %q", target.Kind)
	}
}

// SelectContainer finds the requested container by name, or defaults to the first container.
func SelectContainer(spec *corev1.PodSpec, containerName string) (*corev1.Container, error) {
	if len(spec.Containers) == 0 {
		return nil, errors.New("target workload has no containers")
	}

	if containerName == "" {
		return &spec.Containers[0], nil
	}

	for i := range spec.Containers {
		if spec.Containers[i].Name == containerName {
			return &spec.Containers[i], nil
		}
	}

	names := make([]string, len(spec.Containers))
	for i, c := range spec.Containers {
		names[i] = c.Name
	}

	return nil, fmt.Errorf(
		"container %q not found in target workload (available: %s)",
		containerName, strings.Join(names, ", "),
	)
}

// FilterLabels strips controller hashes, pod identifiers, and netdrill labels.
func FilterLabels(labels map[string]string) map[string]string {
	if len(labels) == 0 {
		return nil
	}

	filtered := make(map[string]string)

	for k, v := range labels {
		if k == "pod-template-hash" ||
			k == "controller-revision-hash" ||
			k == "statefulset.kubernetes.io/pod-name" ||
			k == "batch.kubernetes.io/job-name" ||
			k == "batch.kubernetes.io/controller-uid" ||
			strings.HasPrefix(k, "kubectl-netdrill.io/") {
			continue
		}

		filtered[k] = v
	}

	return filtered
}

// FilterAnnotations removes status, revision, and last-applied annotations.
func FilterAnnotations(annotations map[string]string) map[string]string {
	if len(annotations) == 0 {
		return nil
	}

	filtered := make(map[string]string)

	for k, v := range annotations {
		if k == "kubectl.kubernetes.io/last-applied-configuration" ||
			strings.HasPrefix(k, "deployment.kubernetes.io/") {
			continue
		}

		filtered[k] = v
	}

	return filtered
}

// FilterVolumesAndMounts extracts safe volumes (ConfigMaps, Secrets, EmptyDir, DownwardAPI)
// and container mounts matching those safe volumes, excluding PVCs and auto-injected service account tokens.
func FilterVolumesAndMounts(spec *corev1.PodSpec, container *corev1.Container) ([]corev1.Volume, []corev1.VolumeMount) {
	allowedVolumeNames := make(map[string]struct{})

	var volumes []corev1.Volume

	for _, v := range spec.Volumes {
		// Skip PVCs to avoid multi-attach conflicts
		if v.PersistentVolumeClaim != nil {
			continue
		}

		// Skip auto-injected tokens
		if v.Name == "aws-iam-token" || strings.HasPrefix(v.Name, "kube-api-access") {
			continue
		}

		if v.ConfigMap != nil || v.Secret != nil || v.EmptyDir != nil || v.DownwardAPI != nil || v.Projected != nil {
			volumes = append(volumes, v)
			allowedVolumeNames[v.Name] = struct{}{}
		}
	}

	var mounts []corev1.VolumeMount

	for _, m := range container.VolumeMounts {
		if _, ok := allowedVolumeNames[m.Name]; ok {
			mounts = append(mounts, m)
		}
	}

	return volumes, mounts
}

// SameAsOverrides records which configuration fields were explicitly set by the caller.
type SameAsOverrides struct {
	HasServiceAccount bool
	HasHostNetwork    bool
	HasNodeSelector   bool
	HasPorts          bool
	HasEnv            bool
	HasLabels         bool
	HasCPURequest     bool
	HasMemoryRequest  bool
	HasCPULimit       bool
	HasMemoryLimit    bool
	HasReplicas       bool
}

// OverridesFromCommand extracts override flags from a Cobra command.
func OverridesFromCommand(cmd *cobra.Command) SameAsOverrides {
	if cmd == nil {
		return SameAsOverrides{}
	}

	has := func(name string) bool {
		f := cmd.Flags().Lookup(name)
		return f != nil && f.Changed
	}

	return SameAsOverrides{
		HasServiceAccount: has("service-account"),
		HasHostNetwork:    has("host-network"),
		HasNodeSelector:   has("node-selector"),
		HasPorts:          has("port"),
		HasEnv:            has("env"),
		HasLabels:         has("labels"),
		HasCPURequest:     has("cpu-request"),
		HasMemoryRequest:  has("memory-request"),
		HasCPULimit:       has("cpu-limit"),
		HasMemoryLimit:    has("memory-limit"),
		HasReplicas:       has("replicas"),
	}
}

func applyNetworkAndNode(spec *corev1.PodSpec, overrides SameAsOverrides, cfg *netdrill.PodConfig) {
	if !overrides.HasServiceAccount && spec.ServiceAccountName != "" {
		cfg.ServiceAccount = spec.ServiceAccountName
	}

	if !overrides.HasHostNetwork {
		cfg.HostNetwork = spec.HostNetwork
	}

	clonedNodeSelector := maps.Clone(spec.NodeSelector)
	if clonedNodeSelector == nil {
		clonedNodeSelector = make(map[string]string)
	}

	if overrides.HasNodeSelector {
		for k, v := range cfg.NodeSelector {
			clonedNodeSelector[k] = v
		}
	}

	if len(clonedNodeSelector) > 0 {
		cfg.NodeSelector = clonedNodeSelector
	}
}

func applyLabelsAndAnnotations(workload *SameAsWorkload, overrides SameAsOverrides, cfg *netdrill.PodConfig) {
	// Workload labels are not inherited by default to prevent cloned pods
	// from matching production Service selectors and receiving live traffic.
	// Explicitly supplied cfg.Labels / --labels are preserved unchanged.
	if !overrides.HasLabels {
		cfg.Labels = nil
	}

	cfg.Annotations = FilterAnnotations(workload.Annotations)
}

func applyContainerPorts(container *corev1.Container, overrides SameAsOverrides, cfg *netdrill.PodConfig) {
	if !overrides.HasPorts {
		var ports []int32

		for _, p := range container.Ports {
			if p.ContainerPort > 0 {
				ports = append(ports, p.ContainerPort)
			}
		}

		cfg.Ports = ports
	}
}

func applyContainerEnv(container *corev1.Container, overrides SameAsOverrides, cfg *netdrill.PodConfig) {
	rawEnv := slices.Clone(container.Env)

	if overrides.HasEnv {
		rawEnv = mergeRawEnv(rawEnv, cfg.EnvVars)
	}

	cfg.RawEnvVars = rawEnv
	cfg.EnvFrom = slices.Clone(container.EnvFrom)
}

func applyContainerRequests(res corev1.ResourceList, overrides SameAsOverrides, cfg *netdrill.PodConfig) {
	if res == nil {
		return
	}

	if !overrides.HasCPURequest {
		if cpu := res.Cpu(); cpu != nil && !cpu.IsZero() {
			cfg.CPURequest = cpu.String()
		}
	}

	if !overrides.HasMemoryRequest {
		if mem := res.Memory(); mem != nil && !mem.IsZero() {
			cfg.MemoryRequest = mem.String()
		}
	}
}

func applyContainerLimits(res corev1.ResourceList, overrides SameAsOverrides, cfg *netdrill.PodConfig) {
	if res == nil {
		return
	}

	if !overrides.HasCPULimit {
		if cpu := res.Cpu(); cpu != nil && !cpu.IsZero() {
			cfg.CPULimit = cpu.String()
		}
	}

	if !overrides.HasMemoryLimit {
		if mem := res.Memory(); mem != nil && !mem.IsZero() {
			cfg.MemoryLimit = mem.String()
		}
	}
}

func applyContainerResources(container *corev1.Container, overrides SameAsOverrides, cfg *netdrill.PodConfig) {
	applyContainerRequests(container.Resources.Requests, overrides, cfg)
	applyContainerLimits(container.Resources.Limits, overrides, cfg)
}

func applySchedulingAndSecurity(spec *corev1.PodSpec, container *corev1.Container, cfg *netdrill.PodConfig) {
	if spec.Affinity != nil {
		cfg.Affinity = spec.Affinity.DeepCopy()
	}

	if len(spec.Tolerations) > 0 {
		cfg.Tolerations = slices.Clone(spec.Tolerations)
	}

	if len(spec.TopologySpreadConstraints) > 0 {
		cfg.TopologySpreadConstraints = slices.Clone(spec.TopologySpreadConstraints)
	}

	if spec.DNSConfig != nil {
		cfg.DNSConfig = spec.DNSConfig.DeepCopy()
	}

	if spec.DNSPolicy != "" {
		cfg.DNSPolicy = spec.DNSPolicy
	}

	if spec.PriorityClassName != "" {
		cfg.PriorityClassName = spec.PriorityClassName
	}

	if spec.SecurityContext != nil {
		cfg.SecurityContext = spec.SecurityContext.DeepCopy()
	}

	if container.SecurityContext != nil {
		cfg.ContainerSecurityContext = container.SecurityContext.DeepCopy()
	}
}

// ApplySameAsWorkload merges workload settings into PodConfig, respecting explicit overrides.
func ApplySameAsWorkload(
	workload *SameAsWorkload,
	containerName string,
	overrides SameAsOverrides,
	cfg *netdrill.PodConfig,
) error {
	if workload == nil {
		return errors.New("workload is nil")
	}

	if cfg == nil {
		return errors.New("pod config is nil")
	}

	container, err := SelectContainer(&workload.PodSpec, containerName)
	if err != nil {
		return err
	}

	applyNetworkAndNode(&workload.PodSpec, overrides, cfg)
	applyLabelsAndAnnotations(workload, overrides, cfg)
	applyContainerPorts(container, overrides, cfg)
	applyContainerEnv(container, overrides, cfg)
	applyContainerResources(container, overrides, cfg)
	applySchedulingAndSecurity(&workload.PodSpec, container, cfg)

	vols, mounts := FilterVolumesAndMounts(&workload.PodSpec, container)
	cfg.Volumes = vols
	cfg.VolumeMounts = mounts

	return nil
}

// ApplySameAsPodConfig resolves the target workload and applies its settings to PodConfig.
func ApplySameAsPodConfig(
	ctx context.Context,
	client kubernetes.Interface,
	cmd *cobra.Command,
	targetStr, containerName, defaultNamespace string,
	cfg *netdrill.PodConfig,
) (*TargetInfo, error) {
	target, err := ParseSameAsTarget(targetStr, "pod", defaultNamespace)
	if err != nil {
		return nil, err
	}

	if cmd != nil {
		nsFlag := cmd.Flags().Lookup("namespace")
		if (nsFlag == nil || !nsFlag.Changed) && target.Namespace != "" {
			cfg.Namespace = target.Namespace
		}
	}

	workload, err := FetchSameAsWorkload(ctx, client, target)
	if err != nil {
		return nil, err
	}

	overrides := OverridesFromCommand(cmd)

	if err := ApplySameAsWorkload(workload, containerName, overrides, cfg); err != nil {
		return nil, err
	}

	return &target, nil
}

// SyncDeploymentFromPodConfig synchronizes DeploymentConfig fields from its embedded PodConfig and the target workload.
func SyncDeploymentFromPodConfig(workload *SameAsWorkload, overrides SameAsOverrides, cfg *netdrill.DeploymentConfig) {
	if !overrides.HasReplicas && workload.Replicas != nil {
		cfg.Replicas = workload.Replicas
	}

	if !overrides.HasCPURequest {
		cfg.CPURequest = cfg.PodConfig.CPURequest
	}

	if !overrides.HasMemoryRequest {
		cfg.MemoryRequest = cfg.PodConfig.MemoryRequest
	}

	if !overrides.HasCPULimit {
		cfg.CPULimit = cfg.PodConfig.CPULimit
	}

	if !overrides.HasMemoryLimit {
		cfg.MemoryLimit = cfg.PodConfig.MemoryLimit
	}

	if len(cfg.PodConfig.Labels) > 0 {
		cfg.Labels = cfg.PodConfig.Labels
	}
}

// ApplySameAsDeploymentConfig resolves the target workload and applies its settings to DeploymentConfig.
func ApplySameAsDeploymentConfig(
	ctx context.Context,
	client kubernetes.Interface,
	cmd *cobra.Command,
	targetStr, containerName, defaultNamespace string,
	cfg *netdrill.DeploymentConfig,
) (*TargetInfo, error) {
	target, err := ParseSameAsTarget(targetStr, "deployment", defaultNamespace)
	if err != nil {
		return nil, err
	}

	if cmd != nil {
		nsFlag := cmd.Flags().Lookup("namespace")
		if (nsFlag == nil || !nsFlag.Changed) && target.Namespace != "" {
			cfg.Namespace = target.Namespace
			cfg.PodConfig.Namespace = target.Namespace
		}
	}

	workload, err := FetchSameAsWorkload(ctx, client, target)
	if err != nil {
		return nil, err
	}

	overrides := OverridesFromCommand(cmd)

	if overrides.HasLabels && len(cfg.PodConfig.Labels) == 0 && len(cfg.Labels) > 0 {
		cfg.PodConfig.Labels = maps.Clone(cfg.Labels)
	}

	if err := ApplySameAsWorkload(workload, containerName, overrides, &cfg.PodConfig); err != nil {
		return nil, err
	}

	SyncDeploymentFromPodConfig(workload, overrides, cfg)

	return &target, nil
}
