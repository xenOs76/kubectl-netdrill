package mcp

import (
	"encoding/base64"
	"net/url"
	"sync"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/xenos76/kubectl-netdrill/internal/k8s"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/kubernetes/fake"
	"k8s.io/client-go/rest"
	"k8s.io/client-go/tools/remotecommand"
)

func TestFormatContainerExecLog(t *testing.T) {
	t.Parallel()

	got := formatContainerExecLog(
		[]string{"https-wrench", "certinfo", "--tls-endpoint", "example.com:443"},
		"notAfter=...\n",
		"warn\n",
		0,
	)

	assert.Contains(t, got, "--- netdrill-mcp exec ---")
	assert.Contains(t, got, `"https-wrench"`)
	assert.Contains(t, got, "exit: 0")
	assert.Contains(t, got, "--- stdout ---\nnotAfter=...")
	assert.Contains(t, got, "--- stderr ---\nwarn")
	assert.Contains(t, got, "--- end ---")
}

func TestFormatContainerExecLog_AddsTrailingNewline(t *testing.T) {
	t.Parallel()

	got := formatContainerExecLog([]string{"true"}, "ok", "", 0)
	assert.Contains(t, got, "--- stdout ---\nok\n--- end ---")
}

func TestFormatContainerExecLog_AddsStderrTrailingNewline(t *testing.T) {
	t.Parallel()

	got := formatContainerExecLog([]string{"false"}, "", "warn", 1)
	assert.Contains(t, got, "--- stderr ---\nwarn\n--- end ---")
}

func TestHandlePodExec_MirrorsToContainerLog(t *testing.T) {
	ctx := t.Context()
	pod := authorizedPod("exec-log", "alice", "")
	deps := testDeps(fake.NewSimpleClientset(pod))
	deps.Cfg.MirrorExecToLogs = true

	var mu sync.Mutex

	var commands [][]string

	k8s.LockTestHooks()

	origURL := k8s.ExecURLGetter
	origExec := k8s.SPDYExecutorCreator
	k8s.ExecURLGetter = func(
		_ kubernetes.Interface, _, _, _ string, command []string,
	) (*url.URL, error) {
		mu.Lock()

		commands = append(commands, append([]string(nil), command...))
		mu.Unlock()

		return &url.URL{}, nil
	}
	k8s.SPDYExecutorCreator = func(_ *rest.Config, _ string, _ *url.URL) (remotecommand.Executor, error) {
		return &mockMCPExecExecutor{stdout: []byte("probe-ok\n")}, nil
	}

	k8s.UnlockTestHooks()

	t.Cleanup(func() {
		k8s.LockTestHooks()

		k8s.ExecURLGetter = origURL
		k8s.SPDYExecutorCreator = origExec

		k8s.UnlockTestHooks()
	})

	_, out, err := handlePodExec(ctx, deps, podExecInput{
		podNameInput: podNameInput{PodName: "exec-log"},
		Command:      []string{"https-wrench", "certinfo"},
	})
	require.NoError(t, err)
	assert.Equal(t, "probe-ok\n", out.Stdout)

	mu.Lock()
	defer mu.Unlock()

	require.Len(t, commands, 2)
	assert.Equal(t, []string{"https-wrench", "certinfo"}, commands[0])
	require.GreaterOrEqual(t, len(commands[1]), 4)
	assert.Equal(t, "sh", commands[1][0])
	assert.Contains(t, commands[1][2], "base64 -d > /proc/1/fd/1")

	decoded, err := base64.StdEncoding.DecodeString(commands[1][4])
	require.NoError(t, err)
	assert.Contains(t, string(decoded), `"https-wrench"`)
	assert.Contains(t, string(decoded), "probe-ok")
	assert.Contains(t, string(decoded), "exit: 0")
}

func TestHandlePodExec_NoMirrorByDefault(t *testing.T) {
	ctx := t.Context()
	pod := authorizedPod("exec-no-log", "alice", "")
	deps := testDeps(fake.NewSimpleClientset(pod))

	var mu sync.Mutex

	var commands [][]string

	k8s.LockTestHooks()

	origURL := k8s.ExecURLGetter
	origExec := k8s.SPDYExecutorCreator
	k8s.ExecURLGetter = func(
		_ kubernetes.Interface, _, _, _ string, command []string,
	) (*url.URL, error) {
		mu.Lock()

		commands = append(commands, append([]string(nil), command...))
		mu.Unlock()

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
		podNameInput: podNameInput{PodName: "exec-no-log"},
		Command:      []string{"true"},
	})
	require.NoError(t, err)
	assert.Equal(t, "ok\n", out.Stdout)

	mu.Lock()
	defer mu.Unlock()

	require.Len(t, commands, 1)
	assert.Equal(t, []string{"true"}, commands[0])
}
