package cmd

import (
	"bytes"
	"context"
	"testing"

	"github.com/stretchr/testify/require"
	"github.com/xenos76/kubectl-netdrill/internal/k8s"
	"k8s.io/cli-runtime/pkg/genericclioptions"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/kubernetes/fake"
	"k8s.io/client-go/rest"
)

func TestPodCmd(t *testing.T) {
	// Mock k8s.ClientProvider
	originalProvider := k8s.ClientProvider

	defer func() { k8s.ClientProvider = originalProvider }()

	originalSilenceErrors := rootCmd.SilenceErrors
	originalSilenceUsage := rootCmd.SilenceUsage

	defer func() {
		rootCmd.SilenceErrors = originalSilenceErrors
		rootCmd.SilenceUsage = originalSilenceUsage
	}()

	fakeClient := fake.NewSimpleClientset()
	k8s.ClientProvider = func(_ *genericclioptions.ConfigFlags) (kubernetes.Interface, *rest.Config, error) {
		return fakeClient, &rest.Config{}, nil
	}

	tests := []struct {
		name    string
		args    []string
		wantErr bool
		setup   func()
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
			rootCmd.SetArgs(append([]string{"pod"}, tt.args...))

			buf := new(bytes.Buffer)
			rootCmd.SetOut(buf)
			rootCmd.SetErr(buf)
			rootCmd.SilenceUsage = true
			rootCmd.SilenceErrors = true

			// Reset ClientProvider to default mock
			k8s.ClientProvider = func(_ *genericclioptions.ConfigFlags) (kubernetes.Interface, *rest.Config, error) {
				return fakeClient, &rest.Config{}, nil
			}

			if tt.setup != nil {
				tt.setup()
			}

			err := rootCmd.ExecuteContext(context.Background())
			if tt.wantErr {
				require.Error(t, err)
			} else {
				require.NoError(t, err)
			}

			rootCmd.SetArgs(nil)
		})
	}
}
