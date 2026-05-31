package netdrill

// PodConfig holds inputs for building a troubleshooting pod.
type PodConfig struct {
	Namespace      string
	PodName        string
	Image          string
	HostNetwork    bool
	NodeSelector   map[string]string
	ServiceAccount string
	Ports          []int32
	EnvVars        map[string]string
	Command        []string
	Args           []string
	Owner          string
	Ticket         string
}

// DeploymentConfig holds inputs for building a troubleshooting deployment.
type DeploymentConfig struct {
	PodConfig
	Replicas      *int32
	Labels        map[string]string
	AppLabel      string
	CPURequest    string
	MemoryRequest string
	CPULimit      string
	MemoryLimit   string
}
