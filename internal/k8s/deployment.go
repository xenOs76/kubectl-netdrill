package k8s

import (
	"context"
	"fmt"
	"maps"

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
	// PodOptions holds shared pod create settings.
	PodOptions
	// Replicas is the desired replica count; nil uses the API default.
	Replicas *int32
	// Labels are extra labels merged onto the Deployment and pods.
	Labels map[string]string
	// AppLabel overrides the app label used in selectors when non-empty.
	AppLabel string
	// CPURequest is the container CPU request (e.g. "100m").
	CPURequest string
	// MemoryRequest is the container memory request (e.g. "128Mi").
	MemoryRequest string
	// CPULimit is the container CPU limit (e.g. "500m").
	CPULimit string
	// MemoryLimit is the container memory limit (e.g. "256Mi").
	MemoryLimit string
}

// deploymentLabels builds pod/deployment labels and selector labels from opts.
func deploymentLabels(opts DeploymentOptions) (labels, selectorLabels map[string]string) {
	appLabel := opts.PodName
	if opts.AppLabel != "" {
		appLabel = opts.AppLabel
	}

	selectorLabels = map[string]string{"app": appLabel}

	protected := netdrill.PodLabels(opts.Owner, opts.Ticket)

	labels = maps.Clone(protected)
	for k, v := range opts.Labels {
		labels[k] = v
	}

	for k, v := range selectorLabels {
		labels[k] = v
	}

	labels[netdrill.LabelManaged] = protected[netdrill.LabelManaged]
	if v, ok := protected[netdrill.LabelOwner]; ok {
		labels[netdrill.LabelOwner] = v
	}

	if v, ok := protected[netdrill.LabelTicket]; ok {
		labels[netdrill.LabelTicket] = v
	}

	return labels, selectorLabels
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

	labels, selectorLabels := deploymentLabels(opts)

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
