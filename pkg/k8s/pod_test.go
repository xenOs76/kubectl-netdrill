package k8s

import (
	"context"
	"testing"

	"github.com/google/go-cmp/cmp"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes/fake"
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
	t.Skip("WaitForPodReady requires real Kubernetes API for watch events")
}

func TestDeletePod(t *testing.T) {
	ctx := context.Background()

	tests := []struct {
		name      string
		pods      *corev1.PodList
		namespace string
		podName   string
		wantErr   bool
	}{
		{
			name: "delete existing pod",
			pods: &corev1.PodList{
				Items: []corev1.Pod{
					{
						ObjectMeta: metav1.ObjectMeta{
							Name:      "test-pod",
							Namespace: "default",
						},
					},
				},
			},
			namespace: "default",
			podName:   "test-pod",
			wantErr:   false,
		},
		{
			name:      "delete non-existent pod",
			pods:      &corev1.PodList{Items: []corev1.Pod{}},
			namespace: "default",
			podName:   "non-existent-pod",
			wantErr:   true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			client := fake.NewSimpleClientset(tt.pods)

			err := DeletePod(ctx, client, tt.namespace, tt.podName)

			if tt.wantErr {
				require.Error(t, err)
			} else {
				require.NoError(t, err)
			}
		})
	}
}
