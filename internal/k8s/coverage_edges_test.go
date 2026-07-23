package k8s

import (
	"context"
	"errors"
	"net/url"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/xenos76/kubectl-netdrill/internal/netdrill"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/kubernetes/fake"
	"k8s.io/client-go/rest"
	k8stest "k8s.io/client-go/testing"
	"k8s.io/client-go/tools/remotecommand"
)

func TestEnsureEKSToken_EmptyContainers(t *testing.T) {
	t.Parallel()

	spec := &corev1.PodSpec{}
	ensureEKSToken(spec)
	assert.Empty(t, spec.Volumes)
}

func TestAddEKSTokenVolume_Idempotent(t *testing.T) {
	t.Parallel()

	t.Run("volumeExists", func(t *testing.T) {
		t.Parallel()

		spec := &corev1.PodSpec{
			Containers: []corev1.Container{{Name: "netdrill"}},
			Volumes: []corev1.Volume{{
				Name: "aws-iam-token",
				VolumeSource: corev1.VolumeSource{
					EmptyDir: &corev1.EmptyDirVolumeSource{},
				},
			}},
		}
		addEKSTokenVolume(spec, "/var/run/secrets/eks.amazonaws.com/serviceaccount/token")
		assert.Len(t, spec.Volumes, 1)
		require.Len(t, spec.Containers[0].VolumeMounts, 1)
		assert.Equal(t, "aws-iam-token", spec.Containers[0].VolumeMounts[0].Name)
	})

	t.Run("mountExists", func(t *testing.T) {
		t.Parallel()

		spec := &corev1.PodSpec{
			Containers: []corev1.Container{{
				Name: "netdrill",
				VolumeMounts: []corev1.VolumeMount{{
					Name:      "aws-iam-token",
					MountPath: "/var/run/secrets/eks.amazonaws.com/serviceaccount",
				}},
			}},
		}
		addEKSTokenVolume(spec, "/var/run/secrets/eks.amazonaws.com/serviceaccount/token")
		assert.Len(t, spec.Volumes, 1)
		assert.Len(t, spec.Containers[0].VolumeMounts, 1)
	})
}

func TestAttachToPod_Errors(t *testing.T) {
	ctx := t.Context()
	client := fake.NewSimpleClientset()
	config := &rest.Config{}

	t.Run("url", func(t *testing.T) {
		LockTestHooks()

		origURL := AttachURLGetter
		AttachURLGetter = func(kubernetes.Interface, string, string, string) (*url.URL, error) {
			return nil, errors.New("attach url fail")
		}

		UnlockTestHooks()

		t.Cleanup(func() {
			LockTestHooks()

			AttachURLGetter = origURL

			UnlockTestHooks()
		})

		err := AttachToPod(ctx, client, config, "default", "p", "c", nil)
		require.Error(t, err)
		assert.Contains(t, err.Error(), "attach url fail")
	})

	t.Run("spdy", func(t *testing.T) {
		LockTestHooks()

		origURL := AttachURLGetter
		origSPDY := SPDYExecutorCreator
		AttachURLGetter = func(kubernetes.Interface, string, string, string) (*url.URL, error) {
			return &url.URL{}, nil
		}
		SPDYExecutorCreator = func(*rest.Config, string, *url.URL) (remotecommand.Executor, error) {
			return nil, errors.New("spdy fail")
		}

		UnlockTestHooks()

		t.Cleanup(func() {
			LockTestHooks()

			AttachURLGetter = origURL
			SPDYExecutorCreator = origSPDY

			UnlockTestHooks()
		})

		err := AttachToPod(ctx, client, config, "default", "p", "c", nil)
		require.Error(t, err)
		assert.Contains(t, err.Error(), "spdy fail")
	})
}

func TestAttachToEphemeralContainer_Errors(t *testing.T) {
	ctx := t.Context()
	client := fake.NewSimpleClientset()
	config := &rest.Config{}

	LockTestHooks()

	origURL := AttachURLGetter
	origSPDY := SPDYExecutorCreator
	AttachURLGetter = func(kubernetes.Interface, string, string, string) (*url.URL, error) {
		return nil, errors.New("ephemeral attach url")
	}

	UnlockTestHooks()

	t.Cleanup(func() {
		LockTestHooks()

		AttachURLGetter = origURL
		SPDYExecutorCreator = origSPDY

		UnlockTestHooks()
	})

	err := AttachToEphemeralContainer(ctx, client, config, "default", "p", "c", nil)
	require.Error(t, err)

	LockTestHooks()

	AttachURLGetter = func(kubernetes.Interface, string, string, string) (*url.URL, error) {
		return &url.URL{}, nil
	}
	SPDYExecutorCreator = func(*rest.Config, string, *url.URL) (remotecommand.Executor, error) {
		return nil, errors.New("ephemeral spdy")
	}

	UnlockTestHooks()

	err = AttachToEphemeralContainer(ctx, client, config, "default", "p", "c", nil)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "ephemeral spdy")
}

func TestExecCommand_DefaultContainerAndErrors(t *testing.T) {
	ctx := t.Context()
	client := fake.NewSimpleClientset()
	config := &rest.Config{}

	var sawContainer string

	patchExecHooks(t,
		func(_ kubernetes.Interface, _, _, container string, _ []string) (*url.URL, error) {
			sawContainer = container

			return &url.URL{}, nil
		},
		func(*rest.Config, string, *url.URL) (remotecommand.Executor, error) {
			return &mockExecExecutor{stdout: []byte("x")}, nil
		},
	)

	_, err := ExecCommand(ctx, client, config, "default", "p", "", []string{"true"}, 0)
	require.NoError(t, err)
	assert.Equal(t, netdrill.ContainerNetdrill, sawContainer)

	patchExecHooks(t,
		func(kubernetes.Interface, string, string, string, []string) (*url.URL, error) {
			return nil, errors.New("exec url fail")
		},
		func(*rest.Config, string, *url.URL) (remotecommand.Executor, error) {
			return nil, nil
		},
	)

	_, err = ExecCommand(ctx, client, config, "default", "p", "netdrill", []string{"true"}, 0)
	require.Error(t, err)

	patchExecHooks(t,
		func(kubernetes.Interface, string, string, string, []string) (*url.URL, error) {
			return &url.URL{}, nil
		},
		func(*rest.Config, string, *url.URL) (remotecommand.Executor, error) {
			return nil, errors.New("exec spdy fail")
		},
	)

	_, err = ExecCommand(ctx, client, config, "default", "p", "netdrill", []string{"true"}, 0)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "creating executor")
}

func TestTruncateBytes_IncompleteUTF8(t *testing.T) {
	t.Parallel()

	assert.Empty(t, truncateBytes("日", 2))
}

func TestCreateDeployment_AppLabelReplicasAndResourceErrors(t *testing.T) {
	t.Parallel()

	ctx := t.Context()
	replicas := int32(2)

	dep, err := CreateDeployment(ctx, fake.NewSimpleClientset(), DeploymentOptions{
		PodOptions: PodOptions{Namespace: "default", PodName: "p", Image: "img"},
		AppLabel:   "custom-app",
		Replicas:   &replicas,
	})
	require.NoError(t, err)
	assert.Equal(t, int32(2), *dep.Spec.Replicas)
	assert.Equal(t, "custom-app", dep.Spec.Selector.MatchLabels["app"])

	cases := []struct {
		name string
		opts DeploymentOptions
		want string
	}{
		{
			name: "memRequest",
			opts: DeploymentOptions{
				PodOptions:    PodOptions{Namespace: "default", PodName: "p", Image: "i"},
				MemoryRequest: "nope",
			},
			want: "invalid memory-request",
		},
		{
			name: "cpuLimit",
			opts: DeploymentOptions{
				PodOptions: PodOptions{Namespace: "default", PodName: "p", Image: "i"},
				CPULimit:   "nope",
			},
			want: "invalid cpu-limit",
		},
		{
			name: "memLimit",
			opts: DeploymentOptions{
				PodOptions:  PodOptions{Namespace: "default", PodName: "p", Image: "i"},
				MemoryLimit: "nope",
			},
			want: "invalid memory-limit",
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			_, err := CreateDeployment(ctx, fake.NewSimpleClientset(), tc.opts)
			require.Error(t, err)
			assert.Contains(t, err.Error(), tc.want)
		})
	}
}

func TestMonitorPodStatus_RunningNotFoundCancel(t *testing.T) {
	t.Parallel()

	ctx := t.Context()
	pod := &corev1.Pod{
		ObjectMeta: metav1.ObjectMeta{Name: "run-me", Namespace: "default"},
		Status:     corev1.PodStatus{Phase: corev1.PodRunning},
	}
	client := fake.NewSimpleClientset(pod)

	err := MonitorPodStatus(ctx, client, "default", "run-me")
	require.NoError(t, err)

	err = MonitorPodStatus(ctx, fake.NewSimpleClientset(), "default", "missing")
	require.Error(t, err)

	cancelCtx, cancel := context.WithCancel(ctx)
	cancel()

	pending := &corev1.Pod{
		ObjectMeta: metav1.ObjectMeta{Name: "pending", Namespace: "default"},
		Status:     corev1.PodStatus{Phase: corev1.PodPending},
	}
	err = MonitorPodStatus(cancelCtx, fake.NewSimpleClientset(pending), "default", "pending")
	require.Error(t, err)
}

func TestWaitForEphemeralContainerReady_WatchError(t *testing.T) {
	client := fake.NewSimpleClientset()
	client.PrependReactor("watch", "pods", func(k8stest.Action) (bool, runtime.Object, error) {
		return true, nil, errors.New("watch boom")
	})

	ctx, cancel := context.WithTimeout(t.Context(), 2*time.Second)
	defer cancel()

	err := WaitForEphemeralContainerReady(ctx, client, "default", "p", "c")
	require.Error(t, err)
}

func TestWaitForEphemeralContainerReady_Timeout(t *testing.T) {
	pending := &corev1.Pod{
		ObjectMeta: metav1.ObjectMeta{Name: "pending-eph", Namespace: "default"},
	}

	ctx, cancel := context.WithTimeout(t.Context(), 50*time.Millisecond)
	defer cancel()

	err := WaitForEphemeralContainerReady(ctx, fake.NewSimpleClientset(pending), "default", "pending-eph", "c")
	require.Error(t, err)
}

func TestWaitForPodReady_WatchError(t *testing.T) {
	client := fake.NewSimpleClientset()
	client.PrependReactor("watch", "pods", func(k8stest.Action) (bool, runtime.Object, error) {
		return true, nil, errors.New("watch boom")
	})

	ctx, cancel := context.WithTimeout(t.Context(), 2*time.Second)
	defer cancel()

	err := WaitForPodReady(ctx, client, "default", "x")
	require.Error(t, err)
}
