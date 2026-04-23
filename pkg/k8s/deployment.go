package k8s

import (
	"context"

	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/resource"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes"
)

// DeploymentOptions defines options for creating a troubleshooting Deployment.
type DeploymentOptions struct {
	PodOptions
	Replicas      *int32
	Labels        map[string]string
	AppLabel      string
	CPURequest    string
	MemoryRequest string
	CPULimit      string
	MemoryLimit   string
}

// CreateDeployment creates a new Deployment with the specified options.
func CreateDeployment(
	ctx context.Context,
	client kubernetes.Interface,
	opts DeploymentOptions,
) (*appsv1.Deployment, error) {
	replicas := int32(1)
	if opts.Replicas != nil {
		replicas = *opts.Replicas
	}

	// Base labels required for the selector
	appLabel := "kubectl-netdrill"
	if opts.AppLabel != "" {
		appLabel = opts.AppLabel
	}

	selectorLabels := map[string]string{
		"app": appLabel,
	}

	// Merge with user-provided labels
	labels := make(map[string]string)
	for k, v := range selectorLabels {
		labels[k] = v
	}

	for k, v := range opts.Labels {
		labels[k] = v
	}

	resources := buildResources(opts)

	deployment := &appsv1.Deployment{
		ObjectMeta: metav1.ObjectMeta{
			Name:      opts.PodName, // Reuse PodName field for Deployment name
			Namespace: opts.Namespace,
			Labels:    labels,
		},
		Spec: appsv1.DeploymentSpec{
			Replicas: &replicas,
			Selector: &metav1.LabelSelector{
				MatchLabels: selectorLabels,
			},
			Template: corev1.PodTemplateSpec{
				ObjectMeta: metav1.ObjectMeta{
					Labels: labels,
				},
				Spec: corev1.PodSpec{
					Containers: []corev1.Container{
						{
							Name:            "netdrill",
							Image:           opts.Image,
							ImagePullPolicy: corev1.PullIfNotPresent,
							Stdin:           true,
							TTY:             true,
							Command:         opts.Command,
							Args:            opts.Args,
							Ports:           opts.Ports,
							Env:             opts.EnvVars,
							Resources:       resources,
						},
					},
					HostNetwork:        opts.HostNetwork,
					NodeSelector:       opts.NodeSelector,
					ServiceAccountName: opts.ServiceAccount,
				},
			},
		},
	}

	return client.AppsV1().Deployments(opts.Namespace).Create(ctx, deployment, metav1.CreateOptions{})
}

func buildResources(opts DeploymentOptions) corev1.ResourceRequirements {
	resources := corev1.ResourceRequirements{
		Requests: make(corev1.ResourceList),
		Limits:   make(corev1.ResourceList),
	}

	if opts.CPURequest != "" {
		resources.Requests[corev1.ResourceCPU] = resource.MustParse(opts.CPURequest)
	}

	if opts.MemoryRequest != "" {
		resources.Requests[corev1.ResourceMemory] = resource.MustParse(opts.MemoryRequest)
	}

	if opts.CPULimit != "" {
		resources.Limits[corev1.ResourceCPU] = resource.MustParse(opts.CPULimit)
	}

	if opts.MemoryLimit != "" {
		resources.Limits[corev1.ResourceMemory] = resource.MustParse(opts.MemoryLimit)
	}

	return resources
}
