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
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/kubernetes/fake"
	"k8s.io/client-go/rest"
	"k8s.io/client-go/tools/remotecommand"
)

func patchMCPExecHooks(t *testing.T) {
	t.Helper()

	k8s.LockTestHooks()

	origWait := k8s.WaitForPodReady
	origEph := k8s.WaitForEphemeralContainerReady
	origURL := k8s.ExecURLGetter
	origSPDY := k8s.SPDYExecutorCreator
	k8s.WaitForPodReady = func(context.Context, kubernetes.Interface, string, string) error { return nil }
	k8s.WaitForEphemeralContainerReady = func(context.Context, kubernetes.Interface, string, string, string) error {
		return nil
	}
	k8s.ExecURLGetter = func(kubernetes.Interface, string, string, string, []string) (*url.URL, error) {
		return &url.URL{}, nil
	}
	k8s.SPDYExecutorCreator = func(*rest.Config, string, *url.URL) (remotecommand.Executor, error) {
		return &mockMCPExecExecutor{stdout: []byte("ok\n")}, nil
	}

	k8s.UnlockTestHooks()

	t.Cleanup(func() {
		k8s.LockTestHooks()

		k8s.WaitForPodReady = origWait
		k8s.WaitForEphemeralContainerReady = origEph
		k8s.ExecURLGetter = origURL
		k8s.SPDYExecutorCreator = origSPDY

		k8s.UnlockTestHooks()
	})
}

func startInMemoryMCP(t *testing.T, deps *Deps) (*mcp.ClientSession, context.CancelFunc, <-chan error) {
	t.Helper()

	ctx, cancel := context.WithCancel(t.Context())
	serverTransport, clientTransport := mcp.NewInMemoryTransports()
	errCh := make(chan error, 1)

	go func() {
		errCh <- RunTransport(ctx, "test", deps, serverTransport)
	}()

	client := mcp.NewClient(&mcp.Implementation{Name: "test-client", Version: "v0"}, nil)
	session, err := client.Connect(ctx, clientTransport, nil)
	require.NoError(t, err)

	t.Cleanup(func() {
		_ = session.Close()

		cancel()
	})

	return session, cancel, errCh
}

func TestRunTransport_InMemory(t *testing.T) {
	patchMCPExecHooks(t)

	deps := testDeps(fake.NewSimpleClientset())
	session, cancel, errCh := startInMemoryMCP(t, deps)

	res, err := session.ReadResource(t.Context(), &mcp.ReadResourceParams{URI: containerToolsResourceURI})
	require.NoError(t, err)
	require.NotEmpty(t, res.Contents)
	assert.Contains(t, res.Contents[0].Text, "https-wrench")

	prompt, err := session.GetPrompt(t.Context(), &mcp.GetPromptParams{Name: "netdrill_prompt_aws_in_pod"})
	require.NoError(t, err)
	require.NotEmpty(t, prompt.Messages)

	prompt, err = session.GetPrompt(t.Context(), &mcp.GetPromptParams{Name: "netdrill_prompt_https_in_pod"})
	require.NoError(t, err)
	require.NotEmpty(t, prompt.Messages)

	prompt, err = session.GetPrompt(t.Context(), &mcp.GetPromptParams{
		Name:      "netdrill_prompt_network_check",
		Arguments: map[string]string{"target": "8.8.8.8"},
	})
	require.NoError(t, err)

	text, ok := prompt.Messages[0].Content.(*mcp.TextContent)
	require.True(t, ok)
	assert.Contains(t, text.Text, "8.8.8.8")

	prompt, err = session.GetPrompt(t.Context(), &mcp.GetPromptParams{Name: "netdrill_prompt_network_check"})
	require.NoError(t, err)

	textDefault, ok := prompt.Messages[0].Content.(*mcp.TextContent)
	require.True(t, ok)
	assert.Contains(t, textDefault.Text, "1.1.1.1")

	_, err = session.CallTool(t.Context(), &mcp.CallToolParams{
		Name:      "netdrill_pod_create",
		Arguments: map[string]any{"podName": "mcp-wire"},
	})
	require.NoError(t, err)

	pod, err := deps.Client.CoreV1().Pods("default").Get(t.Context(), "mcp-wire", metav1.GetOptions{})
	require.NoError(t, err)
	assert.Equal(t, "alice", pod.Labels[netdrill.LabelOwner])

	cancel()

	select {
	case err := <-errCh:
		require.Error(t, err)
	case <-time.After(5 * time.Second):
		t.Fatal("RunTransport did not return after cancel")
	}
}

func TestRunTransport_ToolClosures(t *testing.T) {
	patchMCPExecHooks(t)

	deps := testDeps(fake.NewSimpleClientset())
	session, cancel, errCh := startInMemoryMCP(t, deps)

	_, err := session.CallTool(t.Context(), &mcp.CallToolParams{
		Name:      "netdrill_pod_create",
		Arguments: map[string]any{"podName": "mcp-wire"},
	})
	require.NoError(t, err)

	for _, call := range []struct {
		name string
		args map[string]any
	}{
		{"netdrill_pod_wait", map[string]any{"podName": "mcp-wire"}},
		{"netdrill_pod_exec", map[string]any{"podName": "mcp-wire", "command": []string{"true"}}},
		{"netdrill_run_create", map[string]any{"podName": "mcp-run"}},
		{"netdrill_run_cleanup", map[string]any{"podName": "mcp-run"}},
		{"netdrill_debug_add", map[string]any{"podName": "mcp-wire"}},
		{"netdrill_list_managed_pods", map[string]any{}},
		{"netdrill_deployment_create", map[string]any{"name": "mcp-dep"}},
		{"netdrill_deployment_delete", map[string]any{"name": "mcp-dep"}},
		{"netdrill_pod_delete", map[string]any{"podName": "mcp-wire"}},
	} {
		_, callErr := session.CallTool(t.Context(), &mcp.CallToolParams{Name: call.name, Arguments: call.args})
		require.NoError(t, callErr, call.name)
	}

	toolRes, err := session.CallTool(t.Context(), &mcp.CallToolParams{
		Name: "netdrill_debug_exec",
		Arguments: map[string]any{
			"podName": "missing",
			"command": []string{"true"},
		},
	})
	require.NoError(t, err)
	require.NotNil(t, toolRes)
	assert.True(t, toolRes.IsError)

	cancel()

	select {
	case <-errCh:
	case <-time.After(5 * time.Second):
		t.Fatal("RunTransport did not return after cancel")
	}
}

func TestRun_Cancel(t *testing.T) {
	ctx, cancel := context.WithCancel(t.Context())
	deps := testDeps(fake.NewSimpleClientset())

	errCh := make(chan error, 1)

	go func() {
		errCh <- Run(ctx, "test", deps)
	}()

	time.Sleep(50 * time.Millisecond)
	cancel()

	select {
	case <-errCh:
	case <-time.After(5 * time.Second):
		t.Fatal("Run did not return after cancel")
	}
}
