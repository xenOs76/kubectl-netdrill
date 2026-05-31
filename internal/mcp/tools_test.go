package mcp

import (
	"context"
	"net/url"
	"testing"
	"time"

	"github.com/modelcontextprotocol/go-sdk/mcp"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/xenos76/kubectl-netdrill/internal/k8s"
	"github.com/xenos76/kubectl-netdrill/internal/netdrill"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/kubernetes/fake"
	"k8s.io/client-go/rest"
	"k8s.io/client-go/tools/remotecommand"
)

func testDeps(client kubernetes.Interface) *Deps {
	return &Deps{
		Client: client,
		Config: &rest.Config{},
		Cfg: Config{
			Image:            "ghcr.io/xenos76/netdrill:latest",
			DefaultNamespace: "default",
			Owner:            "alice",
			ExecTimeout:      5 * time.Second,
			MaxOutputBytes:   1024,
		},
		Guard: Guard{
			Owner:     "alice",
			MaxOutput: 1024,
		},
	}
}

func authorizedPod(name, owner, ticket string) *corev1.Pod {
	labels := netdrill.PodLabels(owner, ticket)

	return &corev1.Pod{
		ObjectMeta: metav1.ObjectMeta{
			Name:      name,
			Namespace: "default",
			Labels:    labels,
		},
		Status: corev1.PodStatus{Phase: corev1.PodRunning},
	}
}

func TestResolveNamespace(t *testing.T) {
	t.Parallel()

	deps := testDeps(fake.NewSimpleClientset())
	assert.Equal(t, "default", resolveNamespace(deps, ""))
	assert.Equal(t, "other", resolveNamespace(deps, "other"))
}

func TestDefaultPodName(t *testing.T) {
	t.Parallel()

	assert.Equal(t, "netdrill", defaultPodName(""))
	assert.Equal(t, "custom", defaultPodName("custom"))
}

func TestHandlePodCreate(t *testing.T) {
	t.Parallel()

	ctx := t.Context()
	deps := testDeps(fake.NewSimpleClientset())

	_, out, err := handlePodCreate(ctx, deps, podCreateInput{
		namespaceInput: namespaceInput{TicketID: "INC-1"},
		PodName:        "my-pod",
	})
	require.NoError(t, err)
	assert.Equal(t, "default", out.Namespace)
	assert.Equal(t, "my-pod", out.PodName)
	assert.Equal(t, "alice", out.Owner)
	assert.Equal(t, "INC-1", out.Ticket)

	pod, err := deps.Client.CoreV1().Pods("default").Get(ctx, "my-pod", metav1.GetOptions{})
	require.NoError(t, err)
	assert.Equal(t, "alice", pod.Labels[netdrill.LabelOwner])
	assert.Equal(t, "INC-1", pod.Labels[netdrill.LabelTicket])
}

func TestHandlePodDelete(t *testing.T) {
	t.Parallel()

	ctx := t.Context()
	pod := authorizedPod("del-me", "alice", "")
	client := fake.NewSimpleClientset(pod)
	deps := testDeps(client)

	_, _, err := handlePodDelete(ctx, deps, podNameInput{PodName: "del-me"})
	require.NoError(t, err)

	_, err = client.CoreV1().Pods("default").Get(ctx, "del-me", metav1.GetOptions{})
	require.Error(t, err)
}

func TestHandlePodDelete_EmptyName(t *testing.T) {
	t.Parallel()

	ctx := t.Context()
	_, _, err := handlePodDelete(ctx, testDeps(fake.NewSimpleClientset()), podNameInput{})
	require.Error(t, err)
	assert.Contains(t, err.Error(), "pod_name")
}

func TestHandlePodDelete_OwnerMismatch(t *testing.T) {
	t.Parallel()

	ctx := t.Context()
	pod := authorizedPod("bob-pod", "bob", "")
	deps := testDeps(fake.NewSimpleClientset(pod))

	_, _, err := handlePodDelete(ctx, deps, podNameInput{PodName: "bob-pod"})
	require.Error(t, err)
}

func TestHandlePodWait(t *testing.T) {
	ctx := t.Context()
	pod := authorizedPod("wait-me", "alice", "")
	deps := testDeps(fake.NewSimpleClientset(pod))

	k8s.LockTestHooks()

	orig := k8s.WaitForPodReady
	k8s.WaitForPodReady = func(context.Context, kubernetes.Interface, string, string) error {
		return nil
	}

	k8s.UnlockTestHooks()

	t.Cleanup(func() {
		k8s.LockTestHooks()

		k8s.WaitForPodReady = orig

		k8s.UnlockTestHooks()
	})

	_, _, err := handlePodWait(ctx, deps, podNameInput{PodName: "wait-me"})
	require.NoError(t, err)
}

func TestHandlePodExec(t *testing.T) {
	ctx := t.Context()
	pod := authorizedPod("exec-me", "alice", "")
	deps := testDeps(fake.NewSimpleClientset(pod))

	k8s.LockTestHooks()

	origURL := k8s.ExecURLGetter
	origExec := k8s.SPDYExecutorCreator
	k8s.ExecURLGetter = func(_ kubernetes.Interface, _, _, _ string, _ []string) (*url.URL, error) {
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
		podNameInput: podNameInput{PodName: "exec-me"},
		Command:      []string{"echo", "hi"},
	})
	require.NoError(t, err)
	assert.Equal(t, "ok\n", out.Stdout)
	assert.Equal(t, 0, out.ExitCode)
}

func TestHandlePodExec_TicketRequired(t *testing.T) {
	t.Parallel()

	ctx := t.Context()
	pod := authorizedPod("t-pod", "alice", "INC-1")
	deps := testDeps(fake.NewSimpleClientset(pod))

	_, _, err := handlePodExec(ctx, deps, podExecInput{
		podNameInput: podNameInput{PodName: "t-pod"},
		Command:      []string{"true"},
	})
	require.Error(t, err)
}

func TestHandleRunCreate_WithCommand(t *testing.T) {
	t.Parallel()

	ctx := t.Context()
	deps := testDeps(fake.NewSimpleClientset())

	_, out, err := handleRunCreate(ctx, deps, runCreateInput{
		podCreateInput: podCreateInput{PodName: "run-once"},
		Command:        []string{"dig", "example.com"},
	})
	require.NoError(t, err)
	assert.Equal(t, "run-once", out.PodName)

	pod, err := deps.Client.CoreV1().Pods("default").Get(ctx, "run-once", metav1.GetOptions{})
	require.NoError(t, err)
	assert.Equal(t, []string{"dig", "example.com"}, pod.Spec.Containers[0].Command)
}

func TestHandleListPods(t *testing.T) {
	t.Parallel()

	ctx := t.Context()
	p1 := authorizedPod("p1", "alice", "T1")
	p2 := authorizedPod("p2", "alice", "")
	p3 := authorizedPod("p3", "bob", "")
	deps := testDeps(fake.NewSimpleClientset(p1, p2, p3))

	_, out, err := handleListPods(ctx, deps, listPodsInput{})
	require.NoError(t, err)
	require.Len(t, out.Pods, 2)

	_, outFiltered, err := handleListPods(ctx, deps, listPodsInput{
		namespaceInput: namespaceInput{TicketID: "T1"},
	})
	require.NoError(t, err)
	require.Len(t, outFiltered.Pods, 1)
	assert.Equal(t, "p1", outFiltered.Pods[0].Name)
}

func TestHandleDeploymentCreateAndDelete(t *testing.T) {
	t.Parallel()

	ctx := t.Context()
	client := fake.NewSimpleClientset()
	deps := testDeps(client)

	_, out, err := handleDeploymentCreate(ctx, deps, deploymentCreateInput{Name: "dep1"})
	require.NoError(t, err)
	assert.Equal(t, "dep1", out.PodName)

	dep, err := client.AppsV1().Deployments("default").Get(ctx, "dep1", metav1.GetOptions{})
	require.NoError(t, err)
	assert.Equal(t, "alice", dep.Labels[netdrill.LabelOwner])

	_, _, err = handleDeploymentDelete(ctx, deps, deploymentNameInput{Name: "dep1"})
	require.NoError(t, err)

	_, err = client.AppsV1().Deployments("default").Get(ctx, "dep1", metav1.GetOptions{})
	require.Error(t, err)
}

func TestHandleDeploymentDelete_EmptyName(t *testing.T) {
	t.Parallel()

	ctx := t.Context()
	_, _, err := handleDeploymentDelete(ctx, testDeps(fake.NewSimpleClientset()), deploymentNameInput{})
	require.Error(t, err)
}

func TestHandleDebugAdd_EmptyPodName(t *testing.T) {
	t.Parallel()

	ctx := t.Context()
	_, _, err := handleDebugAdd(ctx, testDeps(fake.NewSimpleClientset()), debugAddInput{})
	require.Error(t, err)
}

func TestGetAuthorizedPod_NotFound(t *testing.T) {
	t.Parallel()

	ctx := t.Context()
	deps := testDeps(fake.NewSimpleClientset())

	_, err := deps.getAuthorizedPod(ctx, "default", "missing", "")
	require.Error(t, err)
}

func TestRegisterToolsDoesNotPanic(t *testing.T) {
	t.Parallel()

	srv := mcp.NewServer(&mcp.Implementation{Name: "test", Version: "v0"}, nil)

	require.NotPanics(t, func() {
		registerTools(srv, testDeps(fake.NewSimpleClientset()))
	})
}

type mockMCPExecExecutor struct {
	stdout []byte
	stderr []byte
	err    error
}

func (m *mockMCPExecExecutor) Stream(opts remotecommand.StreamOptions) error {
	return m.write(opts)
}

func (m *mockMCPExecExecutor) StreamWithContext(_ context.Context, opts remotecommand.StreamOptions) error {
	return m.write(opts)
}

func (m *mockMCPExecExecutor) write(opts remotecommand.StreamOptions) error {
	if opts.Stdout != nil && len(m.stdout) > 0 {
		_, _ = opts.Stdout.Write(m.stdout)
	}

	if opts.Stderr != nil && len(m.stderr) > 0 {
		_, _ = opts.Stderr.Write(m.stderr)
	}

	return m.err
}
