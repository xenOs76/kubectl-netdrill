package cmd

import (
	"bytes"
	"context"
	"net/url"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/xenos76/kubectl-netdrill/internal/k8s"
	"github.com/xenos76/kubectl-netdrill/internal/term"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/cli-runtime/pkg/genericclioptions"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/kubernetes/fake"
	"k8s.io/client-go/rest"
	"k8s.io/client-go/tools/remotecommand"
)

//nolint:revive
func TestDebugCmd(t *testing.T) {
	// Mock k8s.ClientProvider
	originalProvider := k8s.ClientProvider

	defer func() { k8s.ClientProvider = originalProvider }()

	fakeClient := fake.NewSimpleClientset()
	_, err := fakeClient.CoreV1().Pods("default").Create(context.Background(), &corev1.Pod{
		ObjectMeta: metav1.ObjectMeta{Name: "netdrill", Namespace: "default"},
	}, metav1.CreateOptions{})
	require.NoError(t, err)

	_, err = fakeClient.CoreV1().Pods("default").Create(context.Background(), &corev1.Pod{
		ObjectMeta: metav1.ObjectMeta{Name: "test-pod", Namespace: "default"},
	}, metav1.CreateOptions{})
	require.NoError(t, err)

	k8s.ClientProvider = func(_ *genericclioptions.ConfigFlags) (kubernetes.Interface, *rest.Config, error) {
		return fakeClient, &rest.Config{}, nil
	}

	// Mock term.RawModeSetter
	originalRawModeSetter := term.RawModeSetter

	defer func() { term.RawModeSetter = originalRawModeSetter }()

	term.RawModeSetter = func() (func(), error) {
		return func() {}, nil
	}

	// Mock k8s.AttachURLGetter
	originalURLGetter := k8s.AttachURLGetter

	defer func() { k8s.AttachURLGetter = originalURLGetter }()

	k8s.AttachURLGetter = func(_ kubernetes.Interface, _, _, _ string) (*url.URL, error) {
		return &url.URL{}, nil
	}

	// Mock k8s.SPDYExecutorCreator
	originalCreator := k8s.SPDYExecutorCreator

	defer func() { k8s.SPDYExecutorCreator = originalCreator }()

	k8s.SPDYExecutorCreator = func(_ *rest.Config, _ string, _ *url.URL) (remotecommand.Executor, error) {
		return &mockExecutor{}, nil
	}

	// Mock k8s.MonitorPodStatus
	originalMonitor := k8s.MonitorPodStatus

	defer func() { k8s.MonitorPodStatus = originalMonitor }()

	k8s.MonitorPodStatus = func(_ context.Context, _ kubernetes.Interface, _, _ string) error {
		return nil
	}

	// Mock k8s.WaitForEphemeralContainerReady
	originalWait := k8s.WaitForEphemeralContainerReady

	defer func() { k8s.WaitForEphemeralContainerReady = originalWait }()

	k8s.WaitForEphemeralContainerReady = func(_ context.Context, _ kubernetes.Interface, _, _, _ string) error {
		return nil
	}

	tests := []struct {
		name    string
		args    []string
		wantErr bool
	}{
		{
			name:    "no args",
			args:    []string{},
			wantErr: false,
		},
		{
			name:    "help",
			args:    []string{"--help"},
			wantErr: false,
		},
		{
			name:    "success",
			args:    []string{"test-pod"},
			wantErr: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			buf := new(bytes.Buffer)
			rootCmd.SetOut(buf)
			rootCmd.SetErr(buf)
			rootCmd.SetArgs(append([]string{"debug"}, tt.args...))

			err := rootCmd.ExecuteContext(context.Background())
			if tt.wantErr {
				assert.Error(t, err)
			} else {
				assert.NoError(t, err)
			}
		})
	}
}
