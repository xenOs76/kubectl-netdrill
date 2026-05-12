package k8s

import (
	"context"
	"net/url"
	"testing"
	"time"

	"github.com/google/go-cmp/cmp"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/kubernetes/fake"
	"k8s.io/client-go/rest"
	"k8s.io/client-go/tools/remotecommand"
)

func TestCreatePodDefault(t *testing.T) {
	ctx := context.Background()
	client := fake.NewSimpleClientset()

	opts := PodOptions{
		Namespace: "default",
		PodName:   "test-pod",
		Image:     "netdrill:latest",
	}

	wantPod := &corev1.Pod{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "test-pod",
			Namespace: "default",
			Labels: map[string]string{
				"app": "kubectl-netdrill",
			},
		},
		Spec: corev1.PodSpec{
			Containers: []corev1.Container{
				{
					Name:            "netdrill",
					Image:           "netdrill:latest",
					ImagePullPolicy: corev1.PullIfNotPresent,
					Stdin:           true,
					TTY:             true,
				},
			},
			RestartPolicy: corev1.RestartPolicyNever,
		},
	}

	pod, err := CreatePod(ctx, client, opts)
	require.NoError(t, err)
	require.NotNil(t, pod)

	assert.Equal(t, wantPod.Name, pod.Name)
	assert.Equal(t, wantPod.Namespace, pod.Namespace)

	if diff := cmp.Diff(wantPod.Spec, pod.Spec); diff != "" {
		t.Errorf("CreatePod() spec mismatch (-want +got):\n%s", diff)
	}
}

func TestCreatePodWithHostNetwork(t *testing.T) {
	ctx := context.Background()
	client := fake.NewSimpleClientset()

	opts := PodOptions{
		Namespace:   "default",
		PodName:     "test-pod-network",
		Image:       "netdrill:latest",
		HostNetwork: true,
	}

	wantPod := &corev1.Pod{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "test-pod-network",
			Namespace: "default",
			Labels: map[string]string{
				"app": "kubectl-netdrill",
			},
		},
		Spec: corev1.PodSpec{
			Containers: []corev1.Container{
				{
					Name:            "netdrill",
					Image:           "netdrill:latest",
					ImagePullPolicy: corev1.PullIfNotPresent,
					Stdin:           true,
					TTY:             true,
				},
			},
			RestartPolicy: corev1.RestartPolicyNever,
			HostNetwork:   true,
		},
	}

	pod, err := CreatePod(ctx, client, opts)
	require.NoError(t, err)
	require.NotNil(t, pod)

	assert.Equal(t, wantPod.Name, pod.Name)

	if diff := cmp.Diff(wantPod.Spec, pod.Spec); diff != "" {
		t.Errorf("CreatePod() spec mismatch (-want +got):\n%s", diff)
	}
}

func TestCreatePodWithCommandAndArgs(t *testing.T) {
	ctx := context.Background()
	client := fake.NewSimpleClientset()

	opts := PodOptions{
		Namespace: "default",
		PodName:   "test-pod-cmd",
		Image:     "netdrill:latest",
		Command:   []string{"/bin/bash", "-c", "--"},
		Args:      []string{"while true; do sleep 30; done;"},
	}

	wantPod := &corev1.Pod{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "test-pod-cmd",
			Namespace: "default",
			Labels: map[string]string{
				"app": "kubectl-netdrill",
			},
		},
		Spec: corev1.PodSpec{
			Containers: []corev1.Container{
				{
					Name:            "netdrill",
					Image:           "netdrill:latest",
					ImagePullPolicy: corev1.PullIfNotPresent,
					Stdin:           true,
					TTY:             true,
					Command:         []string{"/bin/bash", "-c", "--"},
					Args:            []string{"while true; do sleep 30; done;"},
				},
			},
			RestartPolicy: corev1.RestartPolicyNever,
		},
	}

	pod, err := CreatePod(ctx, client, opts)
	require.NoError(t, err)
	require.NotNil(t, pod)

	assert.Equal(t, wantPod.Name, pod.Name)

	if diff := cmp.Diff(wantPod.Spec, pod.Spec); diff != "" {
		t.Errorf("CreatePod() spec mismatch (-want +got):\n%s", diff)
	}
}

func TestCreatePodWithNodeSelector(t *testing.T) {
	ctx := context.Background()
	client := fake.NewSimpleClientset()

	opts := PodOptions{
		Namespace: "default",
		PodName:   "test-pod-selector",
		Image:     "netdrill:latest",
		NodeSelector: map[string]string{
			"kubernetes.io/os": "linux",
		},
	}

	wantPod := &corev1.Pod{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "test-pod-selector",
			Namespace: "default",
			Labels: map[string]string{
				"app": "kubectl-netdrill",
			},
		},
		Spec: corev1.PodSpec{
			Containers: []corev1.Container{
				{
					Name:            "netdrill",
					Image:           "netdrill:latest",
					ImagePullPolicy: corev1.PullIfNotPresent,
					Stdin:           true,
					TTY:             true,
				},
			},
			RestartPolicy: corev1.RestartPolicyNever,
			NodeSelector: map[string]string{
				"kubernetes.io/os": "linux",
			},
		},
	}

	pod, err := CreatePod(ctx, client, opts)
	require.NoError(t, err)
	require.NotNil(t, pod)

	assert.Equal(t, wantPod.Name, pod.Name)

	if diff := cmp.Diff(wantPod.Spec, pod.Spec); diff != "" {
		t.Errorf("CreatePod() spec mismatch (-want +got):\n%s", diff)
	}
}

func TestCreatePodWithServiceAccount(t *testing.T) {
	ctx := context.Background()
	client := fake.NewSimpleClientset()

	opts := PodOptions{
		Namespace:      "default",
		PodName:        "test-pod-sa",
		Image:          "netdrill:latest",
		ServiceAccount: "my-service-account",
	}

	wantPod := &corev1.Pod{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "test-pod-sa",
			Namespace: "default",
			Labels: map[string]string{
				"app": "kubectl-netdrill",
			},
		},
		Spec: corev1.PodSpec{
			Containers: []corev1.Container{
				{
					Name:            "netdrill",
					Image:           "netdrill:latest",
					ImagePullPolicy: corev1.PullIfNotPresent,
					Stdin:           true,
					TTY:             true,
				},
			},
			RestartPolicy:      corev1.RestartPolicyNever,
			ServiceAccountName: "my-service-account",
		},
	}

	pod, err := CreatePod(ctx, client, opts)
	require.NoError(t, err)
	require.NotNil(t, pod)

	assert.Equal(t, wantPod.Name, pod.Name)

	if diff := cmp.Diff(wantPod.Spec, pod.Spec); diff != "" {
		t.Errorf("CreatePod() spec mismatch (-want +got):\n%s", diff)
	}
}

func TestCreatePodWithPorts(t *testing.T) {
	ctx := context.Background()
	client := fake.NewSimpleClientset()

	opts := PodOptions{
		Namespace: "default",
		PodName:   "test-pod-ports",
		Image:     "netdrill:latest",
		Ports: []corev1.ContainerPort{
			{ContainerPort: 80},
			{ContainerPort: 443},
		},
	}

	wantPod := &corev1.Pod{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "test-pod-ports",
			Namespace: "default",
			Labels: map[string]string{
				"app": "kubectl-netdrill",
			},
		},
		Spec: corev1.PodSpec{
			Containers: []corev1.Container{
				{
					Name:            "netdrill",
					Image:           "netdrill:latest",
					ImagePullPolicy: corev1.PullIfNotPresent,
					Stdin:           true,
					TTY:             true,
					Ports: []corev1.ContainerPort{
						{ContainerPort: 80},
						{ContainerPort: 443},
					},
				},
			},
			RestartPolicy: corev1.RestartPolicyNever,
		},
	}

	pod, err := CreatePod(ctx, client, opts)
	require.NoError(t, err)
	require.NotNil(t, pod)

	assert.Equal(t, wantPod.Name, pod.Name)

	if diff := cmp.Diff(wantPod.Spec, pod.Spec); diff != "" {
		t.Errorf("CreatePod() spec mismatch (-want +got):\n%s", diff)
	}
}

func TestCreatePodWithEnvVars(t *testing.T) {
	ctx := context.Background()
	client := fake.NewSimpleClientset()

	opts := PodOptions{
		Namespace: "default",
		PodName:   "test-pod-env",
		Image:     "netdrill:latest",
		EnvVars: []corev1.EnvVar{
			{Name: "FOO", Value: "bar"},
			{Name: "DEBUG", Value: "true"},
		},
	}

	wantPod := &corev1.Pod{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "test-pod-env",
			Namespace: "default",
			Labels: map[string]string{
				"app": "kubectl-netdrill",
			},
		},
		Spec: corev1.PodSpec{
			Containers: []corev1.Container{
				{
					Name:            "netdrill",
					Image:           "netdrill:latest",
					ImagePullPolicy: corev1.PullIfNotPresent,
					Stdin:           true,
					TTY:             true,
					Env: []corev1.EnvVar{
						{Name: "FOO", Value: "bar"},
						{Name: "DEBUG", Value: "true"},
					},
				},
			},
			RestartPolicy: corev1.RestartPolicyNever,
		},
	}

	pod, err := CreatePod(ctx, client, opts)
	require.NoError(t, err)
	require.NotNil(t, pod)

	assert.Equal(t, wantPod.Name, pod.Name)

	if diff := cmp.Diff(wantPod.Spec, pod.Spec); diff != "" {
		t.Errorf("CreatePod() spec mismatch (-want +got):\n%s", diff)
	}
}

func TestWaitForPodReady(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	podName := "test-pod"
	namespace := "default"
	pod := &corev1.Pod{
		ObjectMeta: metav1.ObjectMeta{
			Name:      podName,
			Namespace: namespace,
		},
		Status: corev1.PodStatus{
			Phase: corev1.PodPending,
		},
	}

	client := fake.NewSimpleClientset(pod)

	// Update pod to Running in a goroutine
	go func() {
		// Wait a bit for the watch to start
		time.Sleep(100 * time.Millisecond)

		pod.Status.Phase = corev1.PodRunning
		_, _ = client.CoreV1().Pods(namespace).UpdateStatus(ctx, pod, metav1.UpdateOptions{})
	}()

	err := WaitForPodReady(ctx, client, namespace, podName)
	require.NoError(t, err)
}

func TestWaitForPodReady_Error(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	podName := "test-pod-fail"
	namespace := "default"
	pod := &corev1.Pod{
		ObjectMeta: metav1.ObjectMeta{
			Name:      podName,
			Namespace: namespace,
		},
		Status: corev1.PodStatus{
			Phase: corev1.PodPending,
		},
	}

	client := fake.NewSimpleClientset(pod)

	// Update pod to Failed in a goroutine
	go func() {
		time.Sleep(100 * time.Millisecond)

		pod.Status.Phase = corev1.PodFailed
		_, _ = client.CoreV1().Pods(namespace).UpdateStatus(ctx, pod, metav1.UpdateOptions{})
	}()

	err := WaitForPodReady(ctx, client, namespace, podName)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "terminated early")
}

func TestWaitForPodReady_Timeout(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), 100*time.Millisecond)
	defer cancel()

	podName := "test-pod-timeout"
	namespace := "default"
	pod := &corev1.Pod{
		ObjectMeta: metav1.ObjectMeta{
			Name:      podName,
			Namespace: namespace,
		},
		Status: corev1.PodStatus{
			Phase: corev1.PodPending,
		},
	}

	client := fake.NewSimpleClientset(pod)

	err := WaitForPodReady(ctx, client, namespace, podName)
	require.Error(t, err)
	assert.Equal(t, context.DeadlineExceeded, err)
}

func TestMonitorPodStatus_Error(t *testing.T) {
	fakeClient := fake.NewSimpleClientset()

	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()

	pod := &corev1.Pod{
		ObjectMeta: metav1.ObjectMeta{Name: "test-pod", Namespace: "default"},
		Status:     corev1.PodStatus{Phase: corev1.PodFailed},
	}
	_, _ = fakeClient.CoreV1().Pods("default").Create(ctx, pod, metav1.CreateOptions{})

	err := MonitorPodStatus(ctx, fakeClient, "default", "test-pod")
	require.Error(t, err)
	assert.Contains(t, err.Error(), "terminated early with phase Failed")
}

func TestDeletePod(t *testing.T) {
	ctx := context.Background()
	namespace := "default"
	podName := "test-pod"
	pod := &corev1.Pod{
		ObjectMeta: metav1.ObjectMeta{
			Name:      podName,
			Namespace: namespace,
		},
	}

	tests := []struct {
		name    string
		pods    []runtime.Object
		target  string
		wantErr bool
	}{
		{
			name:    "delete existing pod",
			pods:    []runtime.Object{pod},
			target:  podName,
			wantErr: false,
		},
		{
			name:    "delete non-existent pod",
			pods:    []runtime.Object{},
			target:  "non-existent",
			wantErr: false, // k8s Delete returns success if not found
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			client := fake.NewSimpleClientset(tt.pods...)
			err := DeletePod(ctx, client, namespace, tt.target)
			assert.NoError(t, err)
		})
	}
}

func TestAttachToPod(t *testing.T) {
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

	err := AttachToPod(ctx, client, config, namespace, podName, containerName, nil)
	assert.NoError(t, err)
}

func TestCreatePodWithEKSToken(t *testing.T) {
	ctx := context.Background()
	client := fake.NewSimpleClientset()

	opts := PodOptions{
		Namespace: "default",
		PodName:   "test-pod-eks",
		Image:     "netdrill:latest",
		EnvVars: []corev1.EnvVar{
			{Name: "AWS_WEB_IDENTITY_TOKEN_FILE", Value: "/custom/token/path"},
		},
	}

	pod, err := CreatePod(ctx, client, opts)
	require.NoError(t, err)
	require.NotNil(t, pod)

	// Verify volume was added
	assert.Len(t, pod.Spec.Volumes, 1)
	assert.Equal(t, "aws-iam-token", pod.Spec.Volumes[0].Name)
	assert.NotNil(t, pod.Spec.Volumes[0].Projected)
	assert.Equal(t, "sts.amazonaws.com", pod.Spec.Volumes[0].Projected.Sources[0].ServiceAccountToken.Audience)
	assert.Equal(t, "path", pod.Spec.Volumes[0].Projected.Sources[0].ServiceAccountToken.Path)

	// Verify volume mount was added
	assert.Len(t, pod.Spec.Containers[0].VolumeMounts, 1)
	assert.Equal(t, "aws-iam-token", pod.Spec.Containers[0].VolumeMounts[0].Name)
	assert.Equal(t, "/custom/token", pod.Spec.Containers[0].VolumeMounts[0].MountPath)
	assert.Empty(t, pod.Spec.Containers[0].VolumeMounts[0].SubPath)
}

func TestCreatePodWithEKSRole(t *testing.T) {
	ctx := context.Background()
	client := fake.NewSimpleClientset()

	opts := PodOptions{
		Namespace: "default",
		PodName:   "test-pod-eks-role",
		Image:     "netdrill:latest",
		EnvVars: []corev1.EnvVar{
			{Name: "AWS_ROLE_ARN", Value: "arn:aws:iam::123456789012:role/my-role"},
		},
	}

	pod, err := CreatePod(ctx, client, opts)
	require.NoError(t, err)
	require.NotNil(t, pod)

	// Verify volume was added with default path components
	assert.Len(t, pod.Spec.Volumes, 1)
	assert.Equal(t, "token", pod.Spec.Volumes[0].Projected.Sources[0].ServiceAccountToken.Path)

	assert.Len(t, pod.Spec.Containers[0].VolumeMounts, 1)

	wantPath := "/var/run/secrets/eks.amazonaws.com/serviceaccount"
	assert.Equal(t, wantPath, pod.Spec.Containers[0].VolumeMounts[0].MountPath)
	assert.Empty(t, pod.Spec.Containers[0].VolumeMounts[0].SubPath)
}
