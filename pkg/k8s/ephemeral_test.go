package k8s

import (
	"context"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes/fake"
)

func TestAddEphemeralContainer(t *testing.T) {
	ctx := context.Background()

	pods := &corev1.PodList{
		Items: []corev1.Pod{
			{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "test-pod",
					Namespace: "default",
				},
			},
		},
	}

	opts := EphemeralOptions{
		Namespace:     "default",
		PodName:       "test-pod",
		ContainerName: "debugger",
		Image:         "netdrill:latest",
	}

	client := fake.NewSimpleClientset(pods)
	err := AddEphemeralContainer(ctx, client, opts)
	require.NoError(t, err)

	pod, err := client.CoreV1().Pods(opts.Namespace).Get(ctx, opts.PodName, metav1.GetOptions{})
	require.NoError(t, err)

	found := false

	for _, ec := range pod.Spec.EphemeralContainers {
		if ec.Name == opts.ContainerName {
			found = true

			assert.Equal(t, opts.Image, ec.Image)
			assert.Equal(t, corev1.PullIfNotPresent, ec.ImagePullPolicy)
		}
	}

	assert.True(t, found, "ephemeral container not found in pod spec")
}

func TestAddEphemeralContainerWithTargetProcess(t *testing.T) {
	ctx := context.Background()

	pods := &corev1.PodList{
		Items: []corev1.Pod{
			{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "test-pod-target",
					Namespace: "default",
				},
			},
		},
	}

	opts := EphemeralOptions{
		Namespace:     "default",
		PodName:       "test-pod-target",
		ContainerName: "debugger",
		Image:         "netdrill:latest",
		TargetProcess: "app",
	}

	client := fake.NewSimpleClientset(pods)
	err := AddEphemeralContainer(ctx, client, opts)
	require.NoError(t, err)

	pod, err := client.CoreV1().Pods(opts.Namespace).Get(ctx, opts.PodName, metav1.GetOptions{})
	require.NoError(t, err)

	found := false

	for _, ec := range pod.Spec.EphemeralContainers {
		if ec.Name == opts.ContainerName {
			found = true

			assert.Equal(t, opts.Image, ec.Image)
			assert.Equal(t, corev1.PullIfNotPresent, ec.ImagePullPolicy)
			assert.Equal(t, opts.TargetProcess, ec.TargetContainerName)
		}
	}

	assert.True(t, found, "ephemeral container not found in pod spec")
}

func TestAddEphemeralContainer_GetError(t *testing.T) {
	ctx := context.Background()
	client := fake.NewSimpleClientset()

	opts := EphemeralOptions{
		Namespace:     "default",
		PodName:       "non-existent-pod",
		ContainerName: "debugger",
		Image:         "netdrill:latest",
	}

	err := AddEphemeralContainer(ctx, client, opts)
	require.Error(t, err)
}

func TestCheckEphemeralContainerRunning(t *testing.T) {
	pod := &corev1.Pod{
		Status: corev1.PodStatus{
			EphemeralContainerStatuses: []corev1.ContainerStatus{
				{
					Name: "debugger",
					State: corev1.ContainerState{
						Running: &corev1.ContainerStateRunning{},
					},
				},
			},
		},
	}

	result, err := checkEphemeralContainer(pod, "debugger")
	require.NoError(t, err)
	assert.True(t, result)
}

func TestCheckEphemeralContainerTerminated(t *testing.T) {
	pod := &corev1.Pod{
		Status: corev1.PodStatus{
			EphemeralContainerStatuses: []corev1.ContainerStatus{
				{
					Name: "debugger",
					State: corev1.ContainerState{
						Terminated: &corev1.ContainerStateTerminated{
							Reason: "Error",
						},
					},
				},
			},
		},
	}

	result, err := checkEphemeralContainer(pod, "debugger")
	require.Error(t, err)
	assert.Contains(t, err.Error(), "terminated")
	assert.False(t, result)
}

func TestCheckEphemeralContainerNotFound(t *testing.T) {
	pod := &corev1.Pod{
		Status: corev1.PodStatus{
			EphemeralContainerStatuses: []corev1.ContainerStatus{},
		},
	}

	result, err := checkEphemeralContainer(pod, "debugger")
	require.NoError(t, err)
	assert.False(t, result)
}

func TestWaitForEphemeralContainerReady(t *testing.T) {
	t.Skip("WaitForEphemeralContainerReady requires real Kubernetes API for watch events")
}

func TestAttachToEphemeralContainer(t *testing.T) {
	t.Skip("AttachToEphemeralContainer requires streaming which is not testable with unit tests")
}
