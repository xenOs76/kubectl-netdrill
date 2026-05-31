package mcp

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/xenos76/kubectl-netdrill/internal/netdrill"
	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

func TestGuardAuthorizePod(t *testing.T) {
	t.Parallel()

	g := Guard{Owner: "alice"}

	pod := &corev1.Pod{
		ObjectMeta: metav1.ObjectMeta{
			Labels: netdrill.PodLabels("alice", "INC-1"),
		},
	}

	require.NoError(t, g.AuthorizePod(pod, "INC-1"))
	require.Error(t, g.AuthorizePod(pod, "INC-2"))
	require.Error(t, g.AuthorizePod(pod, ""))

	podBob := &corev1.Pod{
		ObjectMeta: metav1.ObjectMeta{
			Labels: netdrill.PodLabels("bob", ""),
		},
	}
	require.Error(t, g.AuthorizePod(podBob, ""))

	podNoTicket := &corev1.Pod{
		ObjectMeta: metav1.ObjectMeta{
			Labels: netdrill.PodLabels("alice", ""),
		},
	}
	require.NoError(t, g.AuthorizePod(podNoTicket, ""))
}

func TestGuardAuthorizePod_MissingManaged(t *testing.T) {
	t.Parallel()

	g := Guard{Owner: "alice"}
	pod := &corev1.Pod{
		ObjectMeta: metav1.ObjectMeta{
			Labels: map[string]string{netdrill.LabelOwner: "alice"},
		},
	}

	require.Error(t, g.AuthorizePod(pod, ""))
}

func TestGuardAuthorizePod_AllowAnyPod(t *testing.T) {
	t.Parallel()

	g := Guard{Owner: "alice", AllowAnyPod: true}
	pod := &corev1.Pod{ObjectMeta: metav1.ObjectMeta{Labels: map[string]string{}}}

	require.NoError(t, g.AuthorizePod(pod, ""))
}

func TestGuardAuthorizeDeployment(t *testing.T) {
	t.Parallel()

	g := Guard{Owner: "alice"}

	dep := &appsv1.Deployment{
		ObjectMeta: metav1.ObjectMeta{
			Labels: netdrill.PodLabels("alice", "T-1"),
		},
	}

	require.NoError(t, g.AuthorizeDeployment(dep, "T-1"))
	require.Error(t, g.AuthorizeDeployment(dep, "T-2"))

	depBob := &appsv1.Deployment{
		ObjectMeta: metav1.ObjectMeta{
			Labels: netdrill.PodLabels("bob", ""),
		},
	}
	require.Error(t, g.AuthorizeDeployment(depBob, ""))
}

func TestGuardAuthorizeDeployment_MissingManaged(t *testing.T) {
	t.Parallel()

	g := Guard{Owner: "alice"}
	dep := &appsv1.Deployment{
		ObjectMeta: metav1.ObjectMeta{Name: "x"},
	}

	require.Error(t, g.AuthorizeDeployment(dep, ""))
}

func TestGuardValidateContainer(t *testing.T) {
	t.Parallel()

	g := Guard{}
	require.NoError(t, g.ValidateContainer(netdrill.ContainerNetdrill))
	require.Error(t, g.ValidateContainer("sidecar"))
}

func TestGuardTruncate(t *testing.T) {
	t.Parallel()

	g := Guard{MaxOutput: 5}
	out, trunc := g.Truncate("hello world")
	assert.True(t, trunc)
	assert.Contains(t, out, "truncated")

	gZero := Guard{MaxOutput: 0}
	out2, trunc2 := gZero.Truncate("hello world")
	assert.False(t, trunc2)
	assert.Equal(t, "hello world", out2)
}
