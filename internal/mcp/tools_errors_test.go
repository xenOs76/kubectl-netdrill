package mcp

import (
	"context"
	"errors"
	"net/url"
	"sync"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/xenos76/kubectl-netdrill/internal/k8s"
	"github.com/xenos76/kubectl-netdrill/internal/netdrill"
	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/kubernetes/fake"
	"k8s.io/client-go/rest"
	k8stest "k8s.io/client-go/testing"
	"k8s.io/client-go/tools/remotecommand"
)

type mcpExitError struct {
	code int
}

func (mcpExitError) Error() string { return "exit" }

func (e mcpExitError) ExitStatus() int { return e.code }

func TestHandlePodCreate_CreateError(t *testing.T) {
	t.Parallel()

	ctx := t.Context()
	existing := authorizedPod("dup", "alice", "")
	deps := testDeps(fake.NewSimpleClientset(existing))

	_, _, err := handlePodCreate(ctx, deps, podCreateInput{PodName: "dup"})
	require.Error(t, err)
	assert.Contains(t, err.Error(), "creating pod")
}

func TestHandleRunCreate_CreateError(t *testing.T) {
	t.Parallel()

	ctx := t.Context()
	existing := authorizedPod("run-dup", "alice", "")
	deps := testDeps(fake.NewSimpleClientset(existing))

	_, _, err := handleRunCreate(ctx, deps, runCreateInput{
		podCreateInput: podCreateInput{PodName: "run-dup"},
	})
	require.Error(t, err)
	assert.Contains(t, err.Error(), "creating pod")
}

func TestHandlePodDelete_DeleteError(t *testing.T) {
	t.Parallel()

	ctx := t.Context()
	pod := authorizedPod("del-fail", "alice", "")
	client := fake.NewSimpleClientset(pod)
	client.PrependReactor("delete", "pods", func(k8stest.Action) (bool, runtime.Object, error) {
		return true, nil, errors.New("delete boom")
	})
	deps := testDeps(client)

	_, _, err := handlePodDelete(ctx, deps, podNameInput{PodName: "del-fail"})
	require.Error(t, err)
	assert.Contains(t, err.Error(), "deleting pod")
}

func TestHandlePodWait_EmptyName(t *testing.T) {
	t.Parallel()

	_, _, err := handlePodWait(t.Context(), testDeps(fake.NewSimpleClientset()), podNameInput{})
	require.Error(t, err)
	assert.Contains(t, err.Error(), "podName")
}

func TestHandlePodWait_AuthorizeFail(t *testing.T) {
	t.Parallel()

	ctx := t.Context()
	pod := authorizedPod("bob-wait", "bob", "")
	deps := testDeps(fake.NewSimpleClientset(pod))

	_, _, err := handlePodWait(ctx, deps, podNameInput{PodName: "bob-wait"})
	require.Error(t, err)
	require.ErrorIs(t, err, ErrForbidden)
}

func TestHandlePodWait_WaitFail(t *testing.T) {
	ctx := t.Context()
	pod := authorizedPod("wait-fail", "alice", "")
	deps := testDeps(fake.NewSimpleClientset(pod))

	k8s.LockTestHooks()

	orig := k8s.WaitForPodReady
	k8s.WaitForPodReady = func(context.Context, kubernetes.Interface, string, string) error {
		return errors.New("not ready")
	}

	k8s.UnlockTestHooks()

	t.Cleanup(func() {
		k8s.LockTestHooks()

		k8s.WaitForPodReady = orig

		k8s.UnlockTestHooks()
	})

	_, _, err := handlePodWait(ctx, deps, podNameInput{PodName: "wait-fail"})
	require.Error(t, err)
	assert.Contains(t, err.Error(), "waiting for pod")
}

func TestHandlePodExec_EmptyNameAndCommand(t *testing.T) {
	t.Parallel()

	ctx := t.Context()
	deps := testDeps(fake.NewSimpleClientset())

	_, _, err := handlePodExec(ctx, deps, podExecInput{Command: []string{"true"}})
	require.Error(t, err)
	assert.Contains(t, err.Error(), "podName")

	_, _, err = handlePodExec(ctx, deps, podExecInput{podNameInput: podNameInput{PodName: "x"}})
	require.Error(t, err)
	assert.Contains(t, err.Error(), "command")
}

func TestHandlePodExec_BadContainer(t *testing.T) {
	t.Parallel()

	ctx := t.Context()
	pod := authorizedPod("exec-side", "alice", "")
	deps := testDeps(fake.NewSimpleClientset(pod))

	_, _, err := handlePodExec(ctx, deps, podExecInput{
		podNameInput:  podNameInput{PodName: "exec-side"},
		Command:       []string{"true"},
		ContainerName: "sidecar",
	})
	require.Error(t, err)
}

func TestHandlePodExec_HardFail(t *testing.T) {
	ctx := t.Context()
	pod := authorizedPod("exec-hard", "alice", "")
	deps := testDeps(fake.NewSimpleClientset(pod))

	k8s.LockTestHooks()

	origURL := k8s.ExecURLGetter
	k8s.ExecURLGetter = func(_ kubernetes.Interface, _, _, _ string, _ []string) (*url.URL, error) {
		return nil, errors.New("url boom")
	}

	k8s.UnlockTestHooks()

	t.Cleanup(func() {
		k8s.LockTestHooks()

		k8s.ExecURLGetter = origURL

		k8s.UnlockTestHooks()
	})

	_, _, err := handlePodExec(ctx, deps, podExecInput{
		podNameInput: podNameInput{PodName: "exec-hard"},
		Command:      []string{"true"},
	})
	require.Error(t, err)
	assert.Contains(t, err.Error(), "url boom")
}

func TestHandlePodExec_SoftFail(t *testing.T) {
	ctx := t.Context()
	pod := authorizedPod("exec-soft", "alice", "")
	deps := testDeps(fake.NewSimpleClientset(pod))

	k8s.LockTestHooks()

	origURL := k8s.ExecURLGetter
	origExec := k8s.SPDYExecutorCreator
	k8s.ExecURLGetter = func(_ kubernetes.Interface, _, _, _ string, _ []string) (*url.URL, error) {
		return &url.URL{}, nil
	}
	k8s.SPDYExecutorCreator = func(_ *rest.Config, _ string, _ *url.URL) (remotecommand.Executor, error) {
		return &mockMCPExecExecutor{
			stdout: []byte("partial\n"),
			stderr: []byte("oops\n"),
			err:    mcpExitError{code: 7},
		}, nil
	}

	k8s.UnlockTestHooks()

	t.Cleanup(func() {
		k8s.LockTestHooks()

		k8s.ExecURLGetter = origURL
		k8s.SPDYExecutorCreator = origExec

		k8s.UnlockTestHooks()
	})

	_, out, err := handlePodExec(ctx, deps, podExecInput{
		podNameInput: podNameInput{PodName: "exec-soft"},
		Command:      []string{"false"},
	})
	require.Error(t, err)
	assert.Contains(t, err.Error(), "exec failed")
	assert.Equal(t, 7, out.ExitCode)
	assert.Equal(t, "partial\n", out.Stdout)
	assert.Equal(t, "oops\n", out.Stderr)
}

func TestHandleDeploymentCreate_CreateError(t *testing.T) {
	t.Parallel()

	ctx := t.Context()
	deps := testDeps(fake.NewSimpleClientset())

	_, _, err := handleDeploymentCreate(ctx, deps, deploymentCreateInput{Name: "dep-dup"})
	require.NoError(t, err)

	_, _, err = handleDeploymentCreate(ctx, deps, deploymentCreateInput{Name: "dep-dup"})
	require.Error(t, err)
	assert.Contains(t, err.Error(), "creating deployment")
}

func TestHandleDeploymentDelete_NotFound(t *testing.T) {
	t.Parallel()

	_, _, err := handleDeploymentDelete(
		t.Context(),
		testDeps(fake.NewSimpleClientset()),
		deploymentNameInput{Name: "missing"},
	)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "getting deployment")
}

func TestHandleDeploymentDelete_AuthorizeFail(t *testing.T) {
	t.Parallel()

	ctx := t.Context()
	dep := &appsv1.Deployment{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "bob-dep",
			Namespace: "default",
			Labels:    netdrill.PodLabels("bob", ""),
		},
	}
	deps := testDeps(fake.NewSimpleClientset(dep))

	_, _, err := handleDeploymentDelete(ctx, deps, deploymentNameInput{Name: "bob-dep"})
	require.Error(t, err)
	require.ErrorIs(t, err, ErrForbidden)
}

func TestHandleDeploymentDelete_DeleteError(t *testing.T) {
	t.Parallel()

	ctx := t.Context()
	dep := &appsv1.Deployment{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "del-dep",
			Namespace: "default",
			Labels:    netdrill.PodLabels("alice", ""),
		},
	}
	client := fake.NewSimpleClientset(dep)
	client.PrependReactor("delete", "deployments", func(k8stest.Action) (bool, runtime.Object, error) {
		return true, nil, errors.New("delete dep boom")
	})
	deps := testDeps(client)

	_, _, err := handleDeploymentDelete(ctx, deps, deploymentNameInput{Name: "del-dep"})
	require.Error(t, err)
	assert.Contains(t, err.Error(), "deleting deployment")
}

func TestHandleDebugAdd_Success(t *testing.T) {
	ctx := t.Context()
	pod := authorizedPod("dbg-ok", "alice", "")
	pod.Spec.Containers = []corev1.Container{{Name: netdrill.ContainerNetdrill}}
	deps := testDeps(fake.NewSimpleClientset(pod))

	k8s.LockTestHooks()

	orig := k8s.WaitForEphemeralContainerReady
	k8s.WaitForEphemeralContainerReady = func(
		context.Context, kubernetes.Interface, string, string, string,
	) error {
		return nil
	}

	k8s.UnlockTestHooks()

	t.Cleanup(func() {
		k8s.LockTestHooks()

		k8s.WaitForEphemeralContainerReady = orig

		k8s.UnlockTestHooks()
	})

	_, out, err := handleDebugAdd(ctx, deps, debugAddInput{
		podNameInput:    podNameInput{PodName: "dbg-ok"},
		TargetContainer: netdrill.ContainerNetdrill,
	})
	require.NoError(t, err)

	m, ok := out.(map[string]string)
	require.True(t, ok)
	assert.Equal(t, "ready", m["status"])
	assert.Equal(t, netdrill.ContainerDebug, m["container_name"])
}

func TestHandleDebugAdd_WaitFail(t *testing.T) {
	ctx := t.Context()
	pod := authorizedPod("dbg-wait", "alice", "")
	pod.Spec.Containers = []corev1.Container{{Name: netdrill.ContainerNetdrill}}
	deps := testDeps(fake.NewSimpleClientset(pod))

	k8s.LockTestHooks()

	orig := k8s.WaitForEphemeralContainerReady
	k8s.WaitForEphemeralContainerReady = func(
		context.Context, kubernetes.Interface, string, string, string,
	) error {
		return errors.New("ephemeral not ready")
	}

	k8s.UnlockTestHooks()

	t.Cleanup(func() {
		k8s.LockTestHooks()

		k8s.WaitForEphemeralContainerReady = orig

		k8s.UnlockTestHooks()
	})

	_, _, err := handleDebugAdd(ctx, deps, debugAddInput{podNameInput: podNameInput{PodName: "dbg-wait"}})
	require.Error(t, err)
	assert.Contains(t, err.Error(), "waiting for ephemeral")
}

func TestHandleListPods_AllowAny(t *testing.T) {
	t.Parallel()

	ctx := t.Context()
	p1 := authorizedPod("p1", "alice", "")
	p2 := authorizedPod("p2", "bob", "")
	deps := testDeps(fake.NewSimpleClientset(p1, p2))
	deps.Guard.AllowAnyPod = true
	deps.Cfg.Owner = "alice"

	_, out, err := handleListPods(ctx, deps, listPodsInput{})
	require.NoError(t, err)
	require.Len(t, out.Pods, 2)
}

func TestHandleListPods_ListError(t *testing.T) {
	t.Parallel()

	client := fake.NewSimpleClientset()
	client.PrependReactor("list", "pods", func(k8stest.Action) (bool, runtime.Object, error) {
		return true, nil, errors.New("list boom")
	})
	deps := testDeps(client)

	_, _, err := handleListPods(t.Context(), deps, listPodsInput{})
	require.Error(t, err)
	assert.Contains(t, err.Error(), "listing pods")
}

func TestHandlePodExec_MirrorWarnStillOK(t *testing.T) {
	ctx := t.Context()
	pod := authorizedPod("exec-mirror-fail", "alice", "")
	deps := testDeps(fake.NewSimpleClientset(pod))
	deps.Cfg.MirrorExecToLogs = true

	var mu sync.Mutex

	var n int

	k8s.LockTestHooks()

	origURL := k8s.ExecURLGetter
	origExec := k8s.SPDYExecutorCreator
	k8s.ExecURLGetter = func(_ kubernetes.Interface, _, _, _ string, command []string) (*url.URL, error) {
		mu.Lock()

		n++
		call := n
		mu.Unlock()

		if call > 1 {
			return nil, errors.New("mirror url fail")
		}

		_ = command

		return &url.URL{}, nil
	}
	k8s.SPDYExecutorCreator = func(_ *rest.Config, _ string, _ *url.URL) (remotecommand.Executor, error) {
		return &mockMCPExecExecutor{stdout: []byte("ok\n")}, nil
	}

	k8s.UnlockTestHooks()

	t.Cleanup(func() {
		k8s.LockTestHooks()

		k8s.ExecURLGetter = origURL
		k8s.SPDYExecutorCreator = origExec

		k8s.UnlockTestHooks()
	})

	_, out, err := handlePodExec(ctx, deps, podExecInput{
		podNameInput: podNameInput{PodName: "exec-mirror-fail"},
		Command:      []string{"echo"},
	})
	require.NoError(t, err)
	assert.Equal(t, "ok\n", out.Stdout)
}

func TestHandleDebugAdd_AddError(t *testing.T) {
	t.Parallel()

	ctx := t.Context()
	pod := authorizedPod("dbg-add-fail", "alice", "")
	client := fake.NewSimpleClientset(pod)
	client.PrependReactor("update", "pods", func(k8stest.Action) (bool, runtime.Object, error) {
		return true, nil, errors.New("update ephemeral boom")
	})
	deps := testDeps(client)

	_, _, err := handleDebugAdd(ctx, deps, debugAddInput{podNameInput: podNameInput{PodName: "dbg-add-fail"}})
	require.Error(t, err)
	assert.Contains(t, err.Error(), "adding ephemeral")
}
