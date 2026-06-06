package k8s

import (
	"slices"

	"github.com/xenos76/kubectl-netdrill/internal/netdrill"
	corev1 "k8s.io/api/core/v1"
)

// PodOptionsFromConfig converts netdrill.PodConfig into PodOptions.
func PodOptionsFromConfig(cfg netdrill.PodConfig) PodOptions {
	var containerPorts []corev1.ContainerPort
	for _, p := range cfg.Ports {
		containerPorts = append(containerPorts, corev1.ContainerPort{ContainerPort: p})
	}

	var envVars []corev1.EnvVar

	if len(cfg.EnvVars) > 0 {
		keys := make([]string, 0, len(cfg.EnvVars))
		for k := range cfg.EnvVars {
			keys = append(keys, k)
		}

		slices.Sort(keys)

		for _, k := range keys {
			envVars = append(envVars, corev1.EnvVar{Name: k, Value: cfg.EnvVars[k]})
		}
	}

	command := cfg.Command
	if len(command) == 0 {
		command = []string{"/bin/bash", "-c", "--"}
	}

	args := cfg.Args
	if len(args) == 0 {
		args = []string{"while true; do sleep 30; done;"}
	}

	return PodOptions{
		Namespace:      cfg.Namespace,
		PodName:        cfg.PodName,
		Image:          cfg.Image,
		HostNetwork:    cfg.HostNetwork,
		NodeSelector:   cfg.NodeSelector,
		ServiceAccount: cfg.ServiceAccount,
		Ports:          containerPorts,
		EnvVars:        envVars,
		Command:        command,
		Args:           args,
		Owner:          cfg.Owner,
		Ticket:         cfg.Ticket,
	}
}

// DeploymentOptionsFromConfig converts netdrill.DeploymentConfig into DeploymentOptions.
func DeploymentOptionsFromConfig(cfg netdrill.DeploymentConfig) DeploymentOptions {
	return DeploymentOptions{
		PodOptions:    PodOptionsFromConfig(cfg.PodConfig),
		Replicas:      cfg.Replicas,
		Labels:        cfg.Labels,
		AppLabel:      cfg.AppLabel,
		CPURequest:    cfg.CPURequest,
		MemoryRequest: cfg.MemoryRequest,
		CPULimit:      cfg.CPULimit,
		MemoryLimit:   cfg.MemoryLimit,
	}
}
