package k8s

import (
	"testing"

	"github.com/google/go-cmp/cmp"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/xenos76/kubectl-netdrill/internal/netdrill"
)

func TestPodOptionsFromConfig(t *testing.T) {
	t.Parallel()

	cfg := netdrill.PodConfig{
		Namespace:      "ns1",
		PodName:        "p1",
		Image:          "img:tag",
		HostNetwork:    true,
		NodeSelector:   map[string]string{"os": "linux"},
		ServiceAccount: "sa",
		Ports:          []int32{80, 443},
		EnvVars:        map[string]string{"K": "V"},
		Owner:          "alice",
		Ticket:         "INC-1",
	}

	opts := PodOptionsFromConfig(cfg)

	assert.Equal(t, "ns1", opts.Namespace)
	assert.Equal(t, "p1", opts.PodName)
	assert.Equal(t, "img:tag", opts.Image)
	assert.True(t, opts.HostNetwork)
	assert.Equal(t, "sa", opts.ServiceAccount)
	assert.Equal(t, "alice", opts.Owner)
	assert.Equal(t, "INC-1", opts.Ticket)
	assert.Equal(t, []string{"/bin/bash", "-c", "--"}, opts.Command)
	assert.Equal(t, []string{"while true; do sleep 30; done;"}, opts.Args)

	require.Len(t, opts.Ports, 2)
	assert.Equal(t, int32(80), opts.Ports[0].ContainerPort)
	require.Len(t, opts.EnvVars, 1)
	assert.Equal(t, "K", opts.EnvVars[0].Name)
}

func TestPodOptionsFromConfig_CustomCommand(t *testing.T) {
	t.Parallel()

	cfg := netdrill.PodConfig{
		Namespace: "default",
		PodName:   "run1",
		Image:     "img",
		Command:   []string{"dig", "example.com"},
		Args:      []string{"-short"},
	}

	opts := PodOptionsFromConfig(cfg)
	assert.Equal(t, []string{"dig", "example.com"}, opts.Command)
	assert.Equal(t, []string{"-short"}, opts.Args)
}

func TestDeploymentOptionsFromConfig(t *testing.T) {
	t.Parallel()

	replicas := int32(3)
	cfg := netdrill.DeploymentConfig{
		PodConfig: netdrill.PodConfig{
			Namespace: "ns",
			PodName:   "dep",
			Image:     "img",
			Owner:     "alice",
		},
		Replicas:      &replicas,
		Labels:        map[string]string{"team": "net"},
		AppLabel:      "myapp",
		CPURequest:    "100m",
		MemoryRequest: "128Mi",
		CPULimit:      "200m",
		MemoryLimit:   "256Mi",
	}

	opts := DeploymentOptionsFromConfig(cfg)

	assert.Equal(t, &replicas, opts.Replicas)
	assert.Equal(t, "myapp", opts.AppLabel)
	assert.Equal(t, "100m", opts.CPURequest)
	assert.Equal(t, "alice", opts.PodOptions.Owner)

	if diff := cmp.Diff(map[string]string{"team": "net"}, opts.Labels); diff != "" {
		t.Errorf("Labels mismatch (-want +got):\n%s", diff)
	}

	assert.Equal(t, "img", opts.PodOptions.Image)
}
