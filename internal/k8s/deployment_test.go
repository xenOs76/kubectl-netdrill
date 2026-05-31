package k8s

import (
	"context"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/xenos76/kubectl-netdrill/internal/netdrill"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
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

func TestCreateDeployment_ProtectedLabelsNotOverridable(t *testing.T) {
	client := fake.NewSimpleClientset()
	opts := DeploymentOptions{
		PodOptions: PodOptions{
			Namespace: "default",
			PodName:   "netdrill",
			Image:     "netdrill",
			Owner:     "alice",
			Ticket:    "INC-1",
		},
		Labels: map[string]string{
			netdrill.LabelManaged: "false",
			netdrill.LabelOwner:   "bob",
			netdrill.LabelTicket:  "WRONG",
		},
	}

	deploy, err := CreateDeployment(context.Background(), client, opts)
	require.NoError(t, err)
	assert.Equal(t, netdrill.LabelManagedValue, deploy.Labels[netdrill.LabelManaged])
	assert.Equal(t, "alice", deploy.Labels[netdrill.LabelOwner])
	assert.Equal(t, "INC-1", deploy.Labels[netdrill.LabelTicket])
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

func TestCreateDeploymentWithEKSToken(t *testing.T) {
	client := fake.NewSimpleClientset()
	opts := DeploymentOptions{
		PodOptions: PodOptions{
			Namespace: "default",
			PodName:   "test-deploy-eks",
			Image:     "netdrill",
			EnvVars: []corev1.EnvVar{
				{Name: "AWS_ROLE_ARN", Value: "arn:aws:iam::123456789012:role/my-role"},
			},
		},
	}

	deploy, err := CreateDeployment(context.Background(), client, opts)
	require.NoError(t, err)
	require.NotNil(t, deploy)

	// Verify volume was added to template
	assert.Len(t, deploy.Spec.Template.Spec.Volumes, 1)
	assert.Equal(t, "aws-iam-token", deploy.Spec.Template.Spec.Volumes[0].Name)
	assert.Equal(t, "token", deploy.Spec.Template.Spec.Volumes[0].Projected.Sources[0].ServiceAccountToken.Path)

	// Verify volume mount was added to container
	assert.Len(t, deploy.Spec.Template.Spec.Containers[0].VolumeMounts, 1)
	assert.Equal(t, "aws-iam-token", deploy.Spec.Template.Spec.Containers[0].VolumeMounts[0].Name)

	wantPath := "/var/run/secrets/eks.amazonaws.com/serviceaccount"
	assert.Equal(t, wantPath, deploy.Spec.Template.Spec.Containers[0].VolumeMounts[0].MountPath)
	assert.Empty(t, deploy.Spec.Template.Spec.Containers[0].VolumeMounts[0].SubPath)
}

func TestDeleteDeployment(t *testing.T) {
	t.Parallel()

	ctx := t.Context()
	client := fake.NewSimpleClientset()

	opts := DeploymentOptions{
		PodOptions: PodOptions{
			Namespace: "default",
			PodName:   "to-delete",
			Image:     "netdrill",
		},
	}

	_, err := CreateDeployment(ctx, client, opts)
	require.NoError(t, err)

	err = DeleteDeployment(ctx, client, "default", "to-delete")
	require.NoError(t, err)

	_, err = client.AppsV1().Deployments("default").Get(ctx, "to-delete", metav1.GetOptions{})
	require.Error(t, err)
}

func TestDeleteDeployment_NotFound(t *testing.T) {
	t.Parallel()

	ctx := t.Context()
	client := fake.NewSimpleClientset()

	err := DeleteDeployment(ctx, client, "default", "missing")
	require.NoError(t, err)
}
