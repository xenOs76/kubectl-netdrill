package k8s

import (
	"slices"

	"github.com/xenos76/kubectl-netdrill/internal/netdrill"
	corev1 "k8s.io/api/core/v1"
)

func mergeRawEnv(rawEnv []corev1.EnvVar, extraEnv map[string]string) []corev1.EnvVar {
	envVars := slices.Clone(rawEnv)

	for k, v := range extraEnv {
		found := false

		for i := range envVars {
			if envVars[i].Name == k {
				envVars[i] = corev1.EnvVar{Name: k, Value: v}
				found = true

				break
			}
		}

		if !found {
			envVars = append(envVars, corev1.EnvVar{Name: k, Value: v})
		}
	}

	return envVars
}

func buildEnvVars(rawEnv []corev1.EnvVar, extraEnv map[string]string) []corev1.EnvVar {
	if len(rawEnv) > 0 {
		return mergeRawEnv(rawEnv, extraEnv)
	}

	if len(extraEnv) == 0 {
		return nil
	}

	keys := make([]string, 0, len(extraEnv))
	for k := range extraEnv {
		keys = append(keys, k)
	}

	slices.Sort(keys)

	envVars := make([]corev1.EnvVar, 0, len(keys))
	for _, k := range keys {
		envVars = append(envVars, corev1.EnvVar{Name: k, Value: extraEnv[k]})
	}

	return envVars
}

// PodOptionsFromConfig converts netdrill.PodConfig into PodOptions.
func PodOptionsFromConfig(cfg netdrill.PodConfig) PodOptions {
	var containerPorts []corev1.ContainerPort
	for _, p := range cfg.Ports {
		containerPorts = append(containerPorts, corev1.ContainerPort{ContainerPort: p})
	}

	envVars := buildEnvVars(cfg.RawEnvVars, cfg.EnvVars)

	command := cfg.Command
	if len(command) == 0 {
		command = []string{"/bin/bash", "-c", "--"}
	}

	args := cfg.Args
	if len(args) == 0 {
		args = []string{"while true; do sleep 30; done;"}
	}

	return PodOptions{
		Namespace:                 cfg.Namespace,
		PodName:                   cfg.PodName,
		Image:                     cfg.Image,
		HostNetwork:               cfg.HostNetwork,
		NodeSelector:              cfg.NodeSelector,
		ServiceAccount:            cfg.ServiceAccount,
		Ports:                     containerPorts,
		EnvVars:                   envVars,
		EnvFrom:                   cfg.EnvFrom,
		Command:                   command,
		Args:                      args,
		Owner:                     cfg.Owner,
		Ticket:                    cfg.Ticket,
		Labels:                    cfg.Labels,
		Annotations:               cfg.Annotations,
		Affinity:                  cfg.Affinity,
		Tolerations:               cfg.Tolerations,
		TopologySpreadConstraints: cfg.TopologySpreadConstraints,
		DNSConfig:                 cfg.DNSConfig,
		DNSPolicy:                 cfg.DNSPolicy,
		SecurityContext:           cfg.SecurityContext,
		ContainerSecurityContext:  cfg.ContainerSecurityContext,
		PriorityClassName:         cfg.PriorityClassName,
		Volumes:                   cfg.Volumes,
		VolumeMounts:              cfg.VolumeMounts,
		CPURequest:                cfg.CPURequest,
		MemoryRequest:             cfg.MemoryRequest,
		CPULimit:                  cfg.CPULimit,
		MemoryLimit:               cfg.MemoryLimit,
	}
}

// DeploymentOptionsFromConfig converts netdrill.DeploymentConfig into DeploymentOptions.
func DeploymentOptionsFromConfig(cfg netdrill.DeploymentConfig) DeploymentOptions {
	labels := cfg.Labels
	if len(labels) == 0 && len(cfg.PodConfig.Labels) > 0 {
		labels = cfg.PodConfig.Labels
	}

	cpuReq := cfg.CPURequest
	if cpuReq == "" {
		cpuReq = cfg.PodConfig.CPURequest
	}

	memReq := cfg.MemoryRequest
	if memReq == "" {
		memReq = cfg.PodConfig.MemoryRequest
	}

	cpuLim := cfg.CPULimit
	if cpuLim == "" {
		cpuLim = cfg.PodConfig.CPULimit
	}

	memLim := cfg.MemoryLimit
	if memLim == "" {
		memLim = cfg.PodConfig.MemoryLimit
	}

	opts := DeploymentOptions{
		PodOptions:    PodOptionsFromConfig(cfg.PodConfig),
		Replicas:      cfg.Replicas,
		Labels:        labels,
		AppLabel:      cfg.AppLabel,
		CPURequest:    cpuReq,
		MemoryRequest: memReq,
		CPULimit:      cpuLim,
		MemoryLimit:   memLim,
	}
	opts.PodOptions.Labels = labels
	opts.PodOptions.CPURequest = cpuReq
	opts.PodOptions.MemoryRequest = memReq
	opts.PodOptions.CPULimit = cpuLim
	opts.PodOptions.MemoryLimit = memLim

	return opts
}
