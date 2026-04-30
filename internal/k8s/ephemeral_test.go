package k8s

import (
	"context"
	"errors"
	"net/url"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/kubernetes/fake"
	"k8s.io/client-go/rest"
	k8stest "k8s.io/client-go/testing"
	"k8s.io/client-go/tools/remotecommand"
)

type mockExecutor struct{}

func (*mockExecutor) Stream(_ remotecommand.StreamOptions) error {
	return nil
}

func (*mockExecutor) StreamWithContext(_ context.Context, _ remotecommand.StreamOptions) error {
	return nil
}

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

func TestAddEphemeralContainer_UpdateError(t *testing.T) {
	ctx := context.Background()

	pods := &corev1.Pod{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "test-pod",
			Namespace: "default",
		},
	}

	opts := EphemeralOptions{
		Namespace:     "default",
		PodName:       "test-pod",
		ContainerName: "debugger",
		Image:         "netdrill:latest",
	}

	client := fake.NewSimpleClientset(pods)

	// Inject error into UpdateEphemeralContainers
	client.PrependReactor("update", "pods", func(action k8stest.Action) (
		handled bool, ret runtime.Object, err error,
	) {
		if action.GetSubresource() == "ephemeralcontainers" {
			return true, nil, errors.New("update failed")
		}

		return false, nil, nil
	})

	err := AddEphemeralContainer(ctx, client, opts)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "update failed")
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
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	podName := "test-pod"
	namespace := "default"
	containerName := "debugger"

	pod := &corev1.Pod{
		ObjectMeta: metav1.ObjectMeta{
			Name:      podName,
			Namespace: namespace,
		},
		Status: corev1.PodStatus{
			EphemeralContainerStatuses: []corev1.ContainerStatus{
				{
					Name: containerName,
					State: corev1.ContainerState{
						Waiting: &corev1.ContainerStateWaiting{Reason: "ContainerCreating"},
					},
				},
			},
		},
	}

	client := fake.NewSimpleClientset(pod)

	// Update pod to Running in a goroutine
	go func() {
		time.Sleep(100 * time.Millisecond)

		pod.Status.EphemeralContainerStatuses[0].State = corev1.ContainerState{
			Running: &corev1.ContainerStateRunning{},
		}
		_, _ = client.CoreV1().Pods(namespace).UpdateStatus(ctx, pod, metav1.UpdateOptions{})
	}()

	err := WaitForEphemeralContainerReady(ctx, client, namespace, podName, containerName)
	require.NoError(t, err)
}

func TestWaitForEphemeralContainerReady_Error(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	podName := "test-pod-fail"
	namespace := "default"
	containerName := "debugger"

	pod := &corev1.Pod{
		ObjectMeta: metav1.ObjectMeta{
			Name:      podName,
			Namespace: namespace,
		},
		Status: corev1.PodStatus{
			EphemeralContainerStatuses: []corev1.ContainerStatus{
				{
					Name: containerName,
					State: corev1.ContainerState{
						Waiting: &corev1.ContainerStateWaiting{Reason: "ContainerCreating"},
					},
				},
			},
		},
	}

	client := fake.NewSimpleClientset(pod)

	// Update pod to Terminated in a goroutine
	go func() {
		time.Sleep(100 * time.Millisecond)

		pod.Status.EphemeralContainerStatuses[0].State = corev1.ContainerState{
			Terminated: &corev1.ContainerStateTerminated{Reason: "Error"},
		}
		_, _ = client.CoreV1().Pods(namespace).UpdateStatus(ctx, pod, metav1.UpdateOptions{})
	}()

	err := WaitForEphemeralContainerReady(ctx, client, namespace, podName, containerName)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "terminated")
}

func TestAttachToEphemeralContainer(t *testing.T) {
	ctx := context.Background()
	client := fake.NewSimpleClientset()
	config := &rest.Config{}
	namespace := "default"
	podName := "test-pod"
	containerName := "test-container"

	// Mock AttachURLGetter
	originalURLGetter := AttachURLGetter

	defer func() { AttachURLGetter = originalURLGetter }()

	AttachURLGetter = func(_ kubernetes.Interface, _, _, _ string) (*url.URL, error) {
		return &url.URL{}, nil
	}

	// Mock SPDYExecutorCreator
	originalCreator := SPDYExecutorCreator

	defer func() { SPDYExecutorCreator = originalCreator }()

	SPDYExecutorCreator = func(_ *rest.Config, _ string, _ *url.URL) (remotecommand.Executor, error) {
		return &mockExecutor{}, nil
	}

	err := AttachToEphemeralContainer(ctx, client, config, namespace, podName, containerName, nil)
	assert.NoError(t, err)
}
