package netdrill

// PodConfig holds inputs for building a troubleshooting pod.
type PodConfig struct {
	// Namespace is the Kubernetes namespace for the pod.
	Namespace string
	// PodName is the pod metadata name.
	PodName string
	// Image is the container image for the netdrill container.
	Image string
	// HostNetwork requests hostNetwork on the pod spec.
	HostNetwork bool
	// NodeSelector constrains scheduling to matching nodes.
	NodeSelector map[string]string
	// ServiceAccount is the ServiceAccount name (e.g. for EKS IRSA).
	ServiceAccount string
	// Ports are container ports to expose on the pod.
	Ports []int32
	// EnvVars are environment variables for the netdrill container.
	EnvVars map[string]string
	// Command overrides the container entrypoint when non-empty.
	Command []string
	// Args are container args appended after Command.
	Args []string
	// Owner stamps the kubectl-netdrill.io/owner label.
	Owner string
	// Ticket stamps the kubectl-netdrill.io/ticket label when non-empty.
	Ticket string
}

// DeploymentConfig holds inputs for building a troubleshooting deployment.
type DeploymentConfig struct {
	// PodConfig holds shared pod create settings.
	PodConfig
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
