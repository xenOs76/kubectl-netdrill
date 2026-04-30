package k8s

import (
	"context"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"k8s.io/client-go/kubernetes/fake"
)

func TestCreateDeployment(t *testing.T) {
	client := fake.NewSimpleClientset()
	opts := DeploymentOptions{
		PodOptions: PodOptions{
			Namespace: "default",
			PodName:   "test-deploy",
			Image:     "netdrill",
		},
		CPURequest:  "100m",
		MemoryLimit: "256Mi",
	}

	deploy, err := CreateDeployment(context.Background(), client, opts)
	require.NoError(t, err)
	assert.NotNil(t, deploy)
	assert.Equal(t, "test-deploy", deploy.Name)
	assert.Equal(t, int32(1), *deploy.Spec.Replicas)
	assert.Equal(t, "netdrill", deploy.Spec.Template.Spec.Containers[0].Image)
	assert.Equal(t, "100m", deploy.Spec.Template.Spec.Containers[0].Resources.Requests.Cpu().String())
	assert.Equal(t, "256Mi", deploy.Spec.Template.Spec.Containers[0].Resources.Limits.Memory().String())
}

func TestCreateDeployment_DefaultLabel(t *testing.T) {
	client := fake.NewSimpleClientset()
	opts := DeploymentOptions{
		PodOptions: PodOptions{
			Namespace: "default",
			PodName:   "netdrill",
			Image:     "netdrill",
		},
	}

	deploy, err := CreateDeployment(context.Background(), client, opts)
	require.NoError(t, err)
	assert.Equal(t, "netdrill", deploy.Labels["app"], "should use deployment name as default label")
}

func TestCreateDeployment_LabelOverride(t *testing.T) {
	client := fake.NewSimpleClientset()
	opts := DeploymentOptions{
		PodOptions: PodOptions{
			Namespace: "default",
			PodName:   "netdrill",
			Image:     "netdrill",
		},
		Labels: map[string]string{
			"app": "SHOULD-NOT-OVERWRITE",
			"foo": "bar",
		},
	}

	deploy, err := CreateDeployment(context.Background(), client, opts)
	require.NoError(t, err)
	assert.Equal(t, "netdrill", deploy.Labels["app"], "selector label 'app' should NOT be overridable by user labels")
	assert.Equal(t, "bar", deploy.Labels["foo"], "other user labels should still be applied")
}

func TestCreateDeployment_InvalidResources(t *testing.T) {
	client := fake.NewSimpleClientset()
	opts := DeploymentOptions{
		PodOptions: PodOptions{
			Namespace: "default",
			PodName:   "netdrill",
			Image:     "netdrill",
		},
		CPURequest: "invalid",
	}

	_, err := CreateDeployment(context.Background(), client, opts)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "invalid cpu-request")
}
