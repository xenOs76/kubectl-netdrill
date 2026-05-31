package k8s

import (
	"context"
	"fmt"

	"github.com/xenos76/kubectl-netdrill/internal/netdrill"
	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/errors"
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
	appLabel := opts.PodName
	if opts.AppLabel != "" {
		appLabel = opts.AppLabel
	}

	selectorLabels := map[string]string{
		"app": appLabel,
	}

	// Merge netdrill base labels, user labels, and selector labels.
	labels := netdrill.PodLabels(opts.Owner, opts.Ticket)
	for k, v := range opts.Labels {
		labels[k] = v
	}

	for k, v := range selectorLabels {
		labels[k] = v
	}

	resources, err := buildResources(opts)
	if err != nil {
		return nil, err
	}

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
							Name:            netdrill.ContainerNetdrill,
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

	ensureEKSToken(&deployment.Spec.Template.Spec)

	return client.AppsV1().Deployments(opts.Namespace).Create(ctx, deployment, metav1.CreateOptions{})
}

// buildResources creates ResourceRequirements from DeploymentOptions.
func buildResources(opts DeploymentOptions) (corev1.ResourceRequirements, error) {
	var err error

	resources := corev1.ResourceRequirements{}

	resources.Requests, err = parseResource(opts.CPURequest, "cpu-request", resources.Requests, corev1.ResourceCPU)
	if err != nil {
		return resources, err
	}

	resources.Requests, err = parseResource(opts.MemoryRequest, "memory-request",
		resources.Requests, corev1.ResourceMemory)
	if err != nil {
		return resources, err
	}

	resources.Limits, err = parseResource(opts.CPULimit, "cpu-limit", resources.Limits, corev1.ResourceCPU)
	if err != nil {
		return resources, err
	}

	resources.Limits, err = parseResource(opts.MemoryLimit, "memory-limit", resources.Limits, corev1.ResourceMemory)
	if err != nil {
		return resources, err
	}

	return resources, nil
}

// parseResource parses a string value into a Quantity and adds it to the ResourceList.
func parseResource(
	val, name string,
	list corev1.ResourceList,
	resName corev1.ResourceName,
) (corev1.ResourceList, error) {
	if val == "" {
		return list, nil
	}

	q, err := resource.ParseQuantity(val)
	if err != nil {
		return list, fmt.Errorf("invalid %s %q: %w", name, val, err)
	}

	if list == nil {
		list = make(corev1.ResourceList)
	}

	list[resName] = q

	return list, nil
}

// DeleteDeployment deletes the specified Deployment.
func DeleteDeployment(ctx context.Context, client kubernetes.Interface, namespace, name string) error {
	err := client.AppsV1().Deployments(namespace).Delete(ctx, name, metav1.DeleteOptions{})
	if err != nil && errors.IsNotFound(err) {
		return nil
	}

	return err
}
