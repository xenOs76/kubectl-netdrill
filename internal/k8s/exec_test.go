package k8s

import (
	"context"
	"net/url"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/kubernetes/fake"
	"k8s.io/client-go/rest"
	"k8s.io/client-go/tools/remotecommand"
)

type mockExecExecutor struct {
	stdout []byte
	stderr []byte
	err    error
}

func (m *mockExecExecutor) Stream(opts remotecommand.StreamOptions) error {
	return m.write(opts)
}

func (m *mockExecExecutor) StreamWithContext(_ context.Context, opts remotecommand.StreamOptions) error {
	return m.write(opts)
}

func (m *mockExecExecutor) write(opts remotecommand.StreamOptions) error {
	if opts.Stdout != nil && len(m.stdout) > 0 {
		_, _ = opts.Stdout.Write(m.stdout)
	}

	if opts.Stderr != nil && len(m.stderr) > 0 {
		_, _ = opts.Stderr.Write(m.stderr)
	}

	return m.err
}

type testExitError struct {
	code int
}

func (testExitError) Error() string {
	return "command exited with non-zero status"
}

func (e testExitError) ExitStatus() int {
	return e.code
}

func patchExecHooks(
	t *testing.T,
	urlGetter func(kubernetes.Interface, string, string, string, []string) (*url.URL, error),
	spdyCreator func(*rest.Config, string, *url.URL) (remotecommand.Executor, error),
) {
	t.Helper()

	LockTestHooks()

	origURL := ExecURLGetter
	origSPDY := SPDYExecutorCreator
	ExecURLGetter = urlGetter
	SPDYExecutorCreator = spdyCreator

	UnlockTestHooks()

	t.Cleanup(func() {
		LockTestHooks()

		ExecURLGetter = origURL
		SPDYExecutorCreator = origSPDY

		UnlockTestHooks()
	})
}

func TestExecCommand(t *testing.T) {
	ctx := t.Context()
	client := fake.NewSimpleClientset()
	config := &rest.Config{}

	patchExecHooks(t,
		func(_ kubernetes.Interface, _, _, _ string, _ []string) (*url.URL, error) {
			return &url.URL{}, nil
		},
		func(_ *rest.Config, _ string, _ *url.URL) (remotecommand.Executor, error) {
			return &mockExecExecutor{stdout: []byte("hello\n")}, nil
		},
	)

	result, err := ExecCommand(ctx, client, config, "default", "p1", "netdrill", []string{"echo", "hi"}, 0)
	require.NoError(t, err)
	assert.Equal(t, "hello\n", result.Stdout)
	assert.Empty(t, result.Stderr)
	assert.Equal(t, 0, result.ExitCode)
}

func TestExecCommandExitError(t *testing.T) {
	ctx := t.Context()
	client := fake.NewSimpleClientset()
	config := &rest.Config{}

	patchExecHooks(t,
		func(_ kubernetes.Interface, _, _, _ string, _ []string) (*url.URL, error) {
			return &url.URL{}, nil
		},
		func(_ *rest.Config, _ string, _ *url.URL) (remotecommand.Executor, error) {
			return &mockExecExecutor{
				stderr: []byte("fail"),
				err:    testExitError{code: 2},
			}, nil
		},
	)

	result, err := ExecCommand(ctx, client, config, "default", "p1", "netdrill", []string{"false"}, 0)
	require.Error(t, err)
	assert.Equal(t, 2, result.ExitCode)
	assert.Equal(t, "fail", result.Stderr)
}

func TestTruncateBytes(t *testing.T) {
	t.Parallel()

	assert.Equal(t, "hello", truncateBytes("hello", 0))
	assert.Equal(t, "hello", truncateBytes("hello", -1))
	assert.Equal(t, "hello", truncateBytes("hello", 5))
	assert.Equal(t, "hel", truncateBytes("hello", 3))
}
